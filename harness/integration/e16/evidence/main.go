// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Command evidence checks E16 probe receipts against the MCP fixture's
// complete server log. It decides nothing from the probe's own claims about
// calls: every expected or forbidden call is read from the server log.
//
//	go run ./harness/integration/e16/evidence -receipts probes.jsonl -server-log fixture.jsonl
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// Receipt is the subset of an E16_PROBE line the checker relies on.
type Receipt struct {
	Case          string   `json:"case"`
	Step          string   `json:"step"`
	Provider      string   `json:"provider"`
	InvocationIDs []string `json:"invocationIds"`
	StartedAt     string   `json:"startedAt"`
	EndedAt       string   `json:"endedAt"`
	ExpectedCalls int      `json:"expectedToolsCalls"`
	Pass          bool     `json:"pass"`
}

// Entry is one fixture log line.
type Entry struct {
	Time        string `json:"time"`
	Event       string `json:"event"`
	RPCMethod   string `json:"rpcMethod"`
	Correlation string `json:"correlation"`
	Outcome     string `json:"outcome"`
	File        string `json:"file"`
}

// Expected lists every case and step one probe run must report.
func Expected() map[string]bool {
	out := map[string]bool{}
	for _, c := range []string{"N1", "N2a", "N2b", "N5a", "N5b", "N4-Succeeded", "N4-Failed", "N4-Expired"} {
		for _, s := range []string{"control-before", "probe", "control-after"} {
			out[c+"/"+s] = true
		}
	}
	for _, s := range []string{"control-before", "probe", "control-matched-a", "control-matched-b", "control-after"} {
		out["N3/"+s] = true
	}
	out["N11/probe"] = true
	return out
}

// Result is one per-step verdict.
type Result struct {
	Key    string `json:"key"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

// ReadReceipts accepts raw Job output and keeps only E16_PROBE lines.
func ReadReceipts(r io.Reader) ([]Receipt, error) {
	var out []Receipt
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		payload, ok := strings.CutPrefix(line, "E16_PROBE ")
		if !ok {
			continue
		}
		var receipt Receipt
		if err := json.Unmarshal([]byte(payload), &receipt); err != nil {
			return nil, fmt.Errorf("malformed receipt: %w", err)
		}
		out = append(out, receipt)
	}
	return out, scanner.Err()
}

// ReadLog requires every non-empty line to be a fixture entry.
func ReadLog(r io.Reader) ([]Entry, error) {
	var out []Entry
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil || entry.Event == "" {
			return nil, fmt.Errorf("unrecognised server log line: %q", line)
		}
		if _, err := parseTime(entry.Time); err != nil {
			return nil, fmt.Errorf("server log line without time: %q", line)
		}
		out = append(out, entry)
	}
	return out, scanner.Err()
}

// toolTraffic is any server activity that belongs to a tool call session.
func toolTraffic(e Entry) bool {
	return e.Event == "tool" || (e.Event == "receipt" && (e.RPCMethod == "initialize" || e.RPCMethod == "tools/call"))
}

// Check returns per-step results and an overall error.
func Check(receipts []Receipt, log []Entry) ([]Result, error) {
	var results []Result
	var problems []string
	add := func(key string, pass bool, format string, args ...any) {
		results = append(results, Result{Key: key, Pass: pass, Detail: fmt.Sprintf(format, args...)})
		if !pass {
			problems = append(problems, key+": "+fmt.Sprintf(format, args...))
		}
	}
	expected := Expected()
	seen := map[string]bool{}
	type window struct{ start, end time.Time }
	windows := []window{}
	attributed := map[int]bool{}
	for _, r := range receipts {
		key := r.Case + "/" + r.Step
		if !expected[key] {
			add(key, false, "unexpected case or step")
			continue
		}
		if seen[key] {
			add(key, false, "duplicate receipt")
			continue
		}
		seen[key] = true
		if r.Provider != "mcp" {
			add(key, false, "provider %q is not the real MCP server", r.Provider)
			continue
		}
		if !r.Pass {
			add(key, false, "probe reported failure")
			continue
		}
		start, err1 := parseTime(r.StartedAt)
		end, err2 := parseTime(r.EndedAt)
		if err1 != nil || err2 != nil || end.Before(start) {
			add(key, false, "invalid window")
			continue
		}
		windows = append(windows, window{start, end})
		ids := map[string]bool{}
		for _, id := range r.InvocationIDs {
			ids[id] = true
		}
		inWindow, calls, handled := 0, 0, 0
		for i, e := range log {
			at, _ := parseTime(e.Time)
			correlated := e.Correlation != "" && ids[e.Correlation]
			if !correlated && (at.Before(start) || at.After(end)) {
				continue
			}
			if !toolTraffic(e) {
				continue
			}
			attributed[i] = true
			inWindow++
			if e.Event == "receipt" && e.RPCMethod == "tools/call" && correlated {
				calls++
			}
			if e.Event == "tool" && correlated && e.Outcome == "ok" {
				handled++
			}
		}
		switch r.ExpectedCalls {
		case 0:
			add(key, inWindow == 0, "zero-call window: %d tool-session entries", inWindow)
		case 1:
			add(key, len(ids) == 1 && calls == 1 && handled == 1, "one call expected: %d correlated tools/call, %d successful handler results, %d invocation IDs", calls, handled, len(ids))
		default:
			add(key, false, "unsupported expected call count %d", r.ExpectedCalls)
		}
	}
	for key := range expected {
		if !seen[key] {
			add(key, false, "missing receipt")
		}
	}
	// Tool traffic between the first and last probe step that no step
	// accounts for is an unknown request.
	if len(windows) > 0 {
		first, last := windows[0].start, windows[0].end
		for _, w := range windows {
			if w.start.Before(first) {
				first = w.start
			}
			if w.end.After(last) {
				last = w.end
			}
		}
		unknown := 0
		for i, e := range log {
			at, _ := parseTime(e.Time)
			if toolTraffic(e) && !attributed[i] && !at.Before(first) && !at.After(last) {
				unknown++
			}
		}
		add("campaign/unattributed", unknown == 0, "%d tool-session entries outside every probe step", unknown)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Key < results[j].Key })
	if len(problems) > 0 {
		return results, errors.New(strings.Join(problems, "; "))
	}
	return results, nil
}

func main() {
	receiptsPath := flag.String("receipts", "", "probe Job output containing E16_PROBE lines")
	logPath := flag.String("server-log", "", "complete MCP fixture log for the probe Pod lifetime")
	flag.Parse()
	if *receiptsPath == "" || *logPath == "" {
		fmt.Fprintln(os.Stderr, "usage: evidence -receipts <file> -server-log <file>")
		os.Exit(2)
	}
	open := func(path string) *os.File {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return f
	}
	rf, lf := open(*receiptsPath), open(*logPath)
	defer rf.Close()
	defer lf.Close()
	receipts, err := ReadReceipts(rf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	log, err := ReadLog(lf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	results, checkErr := Check(receipts, log)
	encoder := json.NewEncoder(os.Stdout)
	for _, r := range results {
		_ = encoder.Encode(r)
	}
	if checkErr != nil {
		fmt.Fprintln(os.Stderr, "[fail]", checkErr)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "[pass] every probe step matches the server log")
}
