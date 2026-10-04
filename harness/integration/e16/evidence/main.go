// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Command evidence checks E16 probe receipts against the MCP fixture's
// complete server log. It decides nothing from the probe's own claims about
// calls: every expected or forbidden call is read from the server log.
//
//	go run ./harness/integration/e16/evidence -receipts probes.jsonl -server-log fixture-full.jsonl
//
// -receipts accepts either probes.jsonl or the raw probe Job log.
//
// A production Work is checked with:
//
//	go run ./harness/integration/e16/evidence work -case positive -ref e16-positive-a1 \
//	  -view work-show.json -server-log fixture-full.jsonl -fixture-data harness/integration/mcpfixture/data
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

	"github.com/wunderforge/agenova/internal/evidence"
)

// Receipt is the subset of a probe receipt the checker relies on. Its own
// expected call count is informational only; expectations come from the
// fixed case/step table below.
type Receipt struct {
	Case          string   `json:"case"`
	Step          string   `json:"step"`
	Provider      string   `json:"provider"`
	InvocationIDs []string `json:"invocationIds"`
	StartedAt     string   `json:"startedAt"`
	EndedAt       string   `json:"endedAt"`
	Pass          bool     `json:"pass"`
}

// Entry is one fixture log line.
type Entry struct {
	Time        string `json:"time"`
	Event       string `json:"event"`
	HTTPMethod  string `json:"httpMethod"`
	RPCMethod   string `json:"rpcMethod"`
	Correlation string `json:"correlation"`
	Outcome     string `json:"outcome"`
	File        string `json:"file"`
	Error       string `json:"error"`
}

// Expected maps every case/step one probe run must report to the number of
// tools/call requests the server must have received for it.
func Expected() map[string]int {
	out := map[string]int{}
	for _, c := range []string{"N1", "N2a", "N2b", "N5a", "N5b", "N4-Succeeded", "N4-Failed", "N4-Expired"} {
		out[c+"/control-before"], out[c+"/probe"], out[c+"/control-after"] = 1, 0, 1
	}
	for _, s := range []string{"control-before", "control-matched-a", "control-matched-b", "control-after"} {
		out["N3/"+s] = 1
	}
	out["N3/probe"] = 0
	out["N11/probe"] = 1 // the call happens; only its outcome record fails
	return out
}

// Result is one per-step verdict.
type Result struct {
	Key    string `json:"key"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

// ReadReceipts accepts raw Job output (E16_PROBE lines) or probes.jsonl.
func ReadReceipts(r io.Reader) ([]Receipt, error) {
	var out []Receipt
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		payload, ok := strings.CutPrefix(line, "E16_PROBE ")
		if !ok && !strings.HasPrefix(line, "{") {
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

type step struct {
	key        string
	calls      int
	start, end time.Time
	ids        map[string]bool
}

// Check returns per-step results and an overall error. Every server entry
// inside the probe campaign must belong to exactly one step's own session,
// inside that step's window; anything else is an unknown request.
func Check(receipts []Receipt, log []Entry, prior *priorRecords) ([]Result, error) {
	var results []Result
	var problems []string
	add := func(key string, pass bool, format string, args ...any) {
		results = append(results, Result{Key: key, Pass: pass, Detail: fmt.Sprintf(format, args...)})
		if !pass {
			problems = append(problems, key+": "+fmt.Sprintf(format, args...))
		}
	}
	expected := Expected()
	steps := map[string]*step{}
	owner := map[string]string{} // invocation ID -> step key
	for _, r := range receipts {
		key := r.Case + "/" + r.Step
		calls, known := expected[key]
		switch {
		case !known:
			add(key, false, "unexpected case or step")
			continue
		case steps[key] != nil:
			add(key, false, "duplicate receipt")
			continue
		case r.Provider != "mcp":
			add(key, false, "provider %q is not the real MCP server", r.Provider)
			continue
		case !r.Pass:
			add(key, false, "probe reported failure")
			continue
		}
		start, err1 := parseTime(r.StartedAt)
		end, err2 := parseTime(r.EndedAt)
		if err1 != nil || err2 != nil || end.Before(start) {
			add(key, false, "invalid window")
			continue
		}
		st := &step{key: key, calls: calls, start: start, end: end, ids: map[string]bool{}}
		for _, id := range r.InvocationIDs {
			if id == "" || owner[id] != "" {
				add(key, false, "invocation ID %q is empty or reused from %s", id, owner[id])
				continue
			}
			owner[id] = key
			st.ids[id] = true
		}
		if calls == 1 && len(st.ids) != 1 {
			add(key, false, "a step with one call must report exactly one invocation, got %d", len(st.ids))
		}
		steps[key] = st
	}
	for key := range expected {
		if steps[key] == nil {
			add(key, false, "missing receipt")
		}
	}
	if len(steps) == 0 {
		return results, errors.New("no probe receipts")
	}
	ordered := make([]*step, 0, len(steps))
	for _, st := range steps {
		ordered = append(ordered, st)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].start.Before(ordered[j].start) })
	for i := 1; i < len(ordered); i++ {
		if ordered[i].start.Before(ordered[i-1].end) {
			add(ordered[i].key, false, "window overlaps %s", ordered[i-1].key)
		}
	}
	first, last := ordered[0].start, ordered[len(ordered)-1].end

	type tally struct{ total, calls, handled, initialize, bad int }
	counts := map[string]*tally{}
	unknown := 0
	for _, e := range log {
		at, _ := parseTime(e.Time)
		if owner[e.Correlation] == "" && prior.known(e.Correlation) {
			if err := prior.admit(e, at); err != nil {
				add("campaign/prior", false, "%v", err)
			}
			continue
		}
		if key := owner[e.Correlation]; e.Correlation != "" && key != "" {
			st := steps[key]
			late := completion(e) && at.After(st.end) && !at.After(st.end.Add(cleanupCompletion))
			if at.Before(st.start) || (at.After(st.end) && !late) {
				add(key, false, "server entry for this step's invocation at %s is outside its window", e.Time)
				continue
			}
			c := counts[key]
			if c == nil {
				c = &tally{}
				counts[key] = c
			}
			c.total++
			switch {
			case e.Error != "" || (e.Event == "tool" && e.Outcome != "ok"):
				c.bad++
			case e.Event == "receipt" && e.RPCMethod == "tools/call":
				c.calls++
			case e.Event == "receipt" && e.RPCMethod == "initialize":
				c.initialize++
			case e.Event == "tool":
				c.handled++
			}
			continue
		}
		if !at.Before(first) && !at.After(last) {
			unknown++
		}
	}
	for _, st := range ordered {
		c := counts[st.key]
		if c == nil {
			c = &tally{}
		}
		switch st.calls {
		case 0:
			// Any downstream entry at all (GET, DELETE, notification,
			// response) means the rejected operation reached the server.
			add(st.key, c.total == 0, "zero-call step: %d server entries (%d tools/call, %d handler, %d initialize)", c.total, c.calls, c.handled, c.initialize)
		case 1:
			add(st.key, c.calls == 1 && c.handled == 1 && c.initialize == 1 && c.bad == 0, "one call expected: %d tools/call, %d successful handler, %d initialize, %d bad entries", c.calls, c.handled, c.initialize, c.bad)
		}
	}
	add("campaign/unattributed", unknown == 0, "%d server entries inside the probe campaign belong to no step", unknown)
	sort.Slice(results, func(i, j int) bool { return results[i].Key < results[j].Key })
	if len(problems) > 0 {
		return results, errors.New(strings.Join(problems, "; "))
	}
	return results, nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "work" {
		os.Exit(workMain(os.Args[2:]))
	}
	receiptsPath := flag.String("receipts", "", "probe Job output containing E16_PROBE lines")
	logPath := flag.String("server-log", "", "complete MCP fixture log for the probe Pod lifetime")
	priorPath := flag.String("prior", "", "invocation IDs of production Works recorded earlier, one per line")
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
	prior, err := readPrior(*priorPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	results, checkErr := Check(receipts, log, prior)
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

func workMain(args []string) int {
	flags := flag.NewFlagSet("work", flag.ContinueOnError)
	name := flags.String("case", "", "Work case: positive, n6-timeout, n7-oversize, n8-truncation, admission-deny")
	ref := flags.String("ref", "", "request name of this attempt of the Work")
	viewPath := flags.String("view", "", "agenova work show --json output for the Work")
	logPath := flags.String("server-log", "", "complete MCP fixture log covering the Work")
	dataDir := flags.String("fixture-data", "", "fixture dataset directory")
	priorPath := flags.String("prior", "", "invocation records of every earlier attempt in this campaign, failed ones included")
	if err := flags.Parse(args); err != nil || *name == "" || *ref == "" || *viewPath == "" || *logPath == "" || *dataDir == "" {
		fmt.Fprintln(os.Stderr, "usage: evidence work -case <name> -ref <request name> -view <file> -server-log <file> -fixture-data <dir>")
		return 2
	}
	data, err := os.ReadFile(*viewPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	var view evidence.View
	if err := json.Unmarshal(data, &view); err != nil {
		fmt.Fprintln(os.Stderr, "unreadable Work evidence:", err)
		return 1
	}
	// A case can have several attempts; the evidence must be this one's.
	if view.RequestRef != *ref {
		fmt.Fprintf(os.Stderr, "[fail] %s: the evidence is for %q, not this attempt %q\n", *name, view.RequestRef, *ref)
		return 1
	}
	lf, err := os.Open(*logPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer lf.Close()
	log, err := ReadLog(lf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	prior, err := readPrior(*priorPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := CheckWork(*name, view, log, *dataDir, prior); err != nil {
		fmt.Fprintln(os.Stderr, "[fail]", *name+":", err)
		return 1
	}
	fmt.Fprintln(os.Stderr, "[pass]", *name+": Work evidence, server log and answer agree")
	return 0
}

// priorCall is an invocation recorded by an earlier Work: when its evidence
// ended and its final state ("denied" when it never reached a provider,
// "no-outcome" when the attempt has no outcome, otherwise the outcome's
// reason code).
type priorCall struct {
	end   time.Time
	state string
}

// priorRecords counts the completion entries admitted per invocation after
// its end, so no extra handler or response can be explained away.
type priorRecords struct {
	calls map[string]priorCall
	late  map[string]int
}

// lateKind names the completion entries a finished invocation may still log
// after its end; anything else is new activity.
func lateKind(e Entry) string {
	switch {
	case e.Event == "tool":
		return "handler"
	case e.Event == "response" && e.RPCMethod == "tools/call":
		return "tools/call response"
	case e.HTTPMethod == "DELETE" && (e.Event == "response" || e.Event == "receipt"):
		return "session close " + e.Event
	}
	return ""
}

// admit decides one server entry carrying an earlier Work's invocation ID.
// Entries up to its end are history, already judged with that Work. After
// it, a call that never reached a provider may log nothing; any other call
// may log each completion kind at most once: its session close within 2s,
// and, for a timed-out call only, its handler, tools/call response and
// session close within a minute while the slow handler finishes.
func (p *priorRecords) admit(e Entry, at time.Time) error {
	call := p.calls[e.Correlation]
	if !at.After(call.end) {
		return nil
	}
	kind := lateKind(e)
	timeout := call.state == "tool-timeout"
	allowed := call.state != "denied" && kind != "" && ((timeout && !at.After(call.end.Add(lateCompletion))) ||
		(strings.HasPrefix(kind, "session close") && !at.After(call.end.Add(cleanupCompletion))))
	if !allowed {
		return fmt.Errorf("earlier invocation %s (%s) caused a new server entry (%s %s %s) at %s", e.Correlation, call.state, e.Event, e.HTTPMethod, e.RPCMethod, e.Time)
	}
	key := e.Correlation + "/" + kind
	p.late[key]++
	if p.late[key] > 1 {
		return fmt.Errorf("earlier invocation %s logged a second %s at %s", e.Correlation, kind, e.Time)
	}
	return nil
}

func (p *priorRecords) known(id string) bool {
	if p == nil || id == "" {
		return false
	}
	_, ok := p.calls[id]
	return ok
}

// readPrior reads "<invocation ID> <end RFC3339> <state>" lines written by
// the campaign runner; a missing file means none.
func readPrior(path string) (*priorRecords, error) {
	prior := &priorRecords{calls: map[string]priorCall{}, late: map[string]int{}}
	if path == "" {
		return prior, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return prior, nil
	}
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 3 {
			return nil, fmt.Errorf("prior invocation line %q must be: id end state", line)
		}
		end, err := parseTime(fields[1])
		if err != nil {
			return nil, fmt.Errorf("prior invocation line %q: %w", line, err)
		}
		prior.calls[fields[0]] = priorCall{end: end, state: fields[2]}
	}
	return prior, nil
}
