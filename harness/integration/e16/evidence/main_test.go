// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

type fixture struct {
	receipts []Receipt
	log      []Entry
	clock    time.Time
}

func (f *fixture) tick() string {
	f.clock = f.clock.Add(10 * time.Millisecond)
	return f.clock.Format(time.RFC3339Nano)
}

// session appends one complete, successful MCP session for id on the
// credential-free path, as the fixture logs it.
func (f *fixture) session(id string) {
	f.log = append(f.log,
		Entry{Time: f.tick(), Event: "receipt", HTTPMethod: "POST", RPCMethod: "initialize", Correlation: id, Path: "/mcp", Auth: "missing"},
		Entry{Time: f.tick(), Event: "response", RPCMethod: "initialize", Correlation: id, Path: "/mcp"},
		Entry{Time: f.tick(), Event: "receipt", HTTPMethod: "POST", RPCMethod: "notifications/initialized", Correlation: id, Path: "/mcp", Auth: "missing"},
		Entry{Time: f.tick(), Event: "receipt", HTTPMethod: "POST", RPCMethod: "tools/call", Correlation: id, Path: "/mcp", Auth: "missing"},
		Entry{Time: f.tick(), Event: "tool", Correlation: id, Outcome: "ok", File: "README.md", Path: "/mcp"},
		Entry{Time: f.tick(), Event: "response", RPCMethod: "tools/call", Correlation: id, Path: "/mcp"},
		Entry{Time: f.tick(), Event: "receipt", HTTPMethod: "DELETE", Correlation: id, Path: "/mcp", Auth: "missing"},
	)
}

// complete builds a passing campaign in a fixed order. Zero-call steps that
// reach the Gateway report an invocation ID the server must never see.
func complete() *fixture {
	f := &fixture{clock: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)}
	keys := []string{}
	for key := range Expected() {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for n, key := range keys {
		parts := strings.SplitN(key, "/", 2)
		r := Receipt{Case: parts[0], Step: parts[1], Provider: "mcp", Pass: true, StartedAt: f.tick(), InvocationIDs: []string{}}
		id := fmt.Sprintf("inv-%d", n)
		switch {
		case Expected()[key] == 1:
			r.InvocationIDs = []string{id}
			f.session(id)
		case parts[0] == "N1" || parts[0] == "N2a" || parts[0] == "N5b":
			r.InvocationIDs = []string{id}
		}
		r.EndedAt = f.tick()
		f.tick() // gap between steps
		f.receipts = append(f.receipts, r)
	}
	return f
}

func (f *fixture) find(t *testing.T, key string) *Receipt {
	t.Helper()
	for i := range f.receipts {
		if f.receipts[i].Case+"/"+f.receipts[i].Step == key {
			return &f.receipts[i]
		}
	}
	t.Fatalf("no %s", key)
	return nil
}

func within(r *Receipt) string {
	start, _ := parseTime(r.StartedAt)
	return start.Add(time.Millisecond).Format(time.RFC3339Nano)
}

func TestCompleteCampaignPasses(t *testing.T) {
	f := complete()
	// Traffic outside the probe campaign (for example production Works) is
	// not the checker's concern.
	f.log = append(f.log, Entry{Time: "2026-10-02T23:00:00Z", Event: "receipt", RPCMethod: "tools/call", Correlation: "production-work"})
	if _, err := Check(f.receipts, f.log, nil); err != nil {
		t.Fatal(err)
	}
}

// Session cleanup may finish just after a step; an earlier Work's timed-out
// call may complete during the campaign but never start anything new.
func TestCheckerAllowsLegitimateLateCompletion(t *testing.T) {
	f := complete()
	r := f.find(t, "N11/probe")
	end, _ := parseTime(r.EndedAt)
	f.log = append(f.log, Entry{Time: end.Add(time.Millisecond).Format(time.RFC3339Nano), Event: "response", HTTPMethod: "DELETE", Correlation: r.InvocationIDs[0], Path: "/mcp"})
	mid := within(f.find(t, "N3/control-before"))
	f.log = append(f.log, Entry{Time: mid, Event: "tool", Correlation: "earlier-timeout", Outcome: "ok"})
	before, _ := parseTime(f.receipts[0].StartedAt)
	prior := func() *priorRecords { return priorOf("earlier-timeout", before.Add(-time.Second), "tool-timeout") }
	if _, err := Check(f.receipts, f.log, prior()); err != nil {
		t.Fatal(err)
	}
	f.log = append(f.log, Entry{Time: mid, Event: "receipt", RPCMethod: "tools/call", Correlation: "earlier-timeout"})
	if _, err := Check(f.receipts, f.log, prior()); err == nil || !strings.Contains(err.Error(), "caused a new server entry") {
		t.Fatalf("an earlier invocation's new request: %v", err)
	}
	g := complete()
	r = g.find(t, "N11/probe")
	end, _ = parseTime(r.EndedAt)
	g.log = append(g.log, Entry{Time: end.Add(time.Millisecond).Format(time.RFC3339Nano), Event: "receipt", HTTPMethod: "POST", RPCMethod: "tools/call", Correlation: r.InvocationIDs[0]})
	if _, err := Check(g.receipts, g.log, nil); err == nil {
		t.Fatal("a new request after the step was accepted")
	}
}

func TestCheckerRejects(t *testing.T) {
	for name, tc := range map[string]struct {
		want   string
		mutate func(*testing.T, *fixture)
	}{
		"unknown tools/call inside a positive control": {"belong to no step", func(t *testing.T, f *fixture) {
			r := f.find(t, "N1/control-before")
			f.log = append(f.log, Entry{Time: within(r), Event: "receipt", RPCMethod: "tools/call"})
		}},
		"unparseable request inside a zero-call window": {"belong to no step", func(t *testing.T, f *fixture) {
			r := f.find(t, "N5a/probe")
			f.log = append(f.log, Entry{Time: within(r), Event: "receipt", HTTPMethod: "POST", Error: "unparseable"})
		}},
		"empty server log": {"one call expected: 0 tools/call", func(t *testing.T, f *fixture) { f.log = nil }},
		"session reused by a later step": {"is empty or reused from N1/control-", func(t *testing.T, f *fixture) {
			f.find(t, "N1/control-after").InvocationIDs = f.find(t, "N1/control-before").InvocationIDs
		}},
		"session outside its step window": {"outside its window", func(t *testing.T, f *fixture) {
			id := f.find(t, "N11/probe").InvocationIDs[0]
			for i := range f.log {
				if f.log[i].Correlation == id {
					f.log[i].Time = "2026-10-02T23:00:00Z"
				}
			}
		}},
		"server saw a denied invocation": {"N2a/probe: zero-call step: 1 server entries (1 tools/call", func(t *testing.T, f *fixture) {
			r := f.find(t, "N2a/probe")
			f.log = append(f.log, Entry{Time: within(r), Event: "receipt", RPCMethod: "tools/call", Correlation: r.InvocationIDs[0]})
		}},
		"denied invocation opened a stream": {"N1/probe: zero-call step: 1 server entries", func(t *testing.T, f *fixture) {
			r := f.find(t, "N1/probe")
			f.log = append(f.log, Entry{Time: within(r), Event: "receipt", HTTPMethod: "GET", Correlation: r.InvocationIDs[0]})
		}},
		"replayed call": {"N11/probe: one call expected: 2 tools/call", func(t *testing.T, f *fixture) {
			r := f.find(t, "N11/probe")
			end, _ := parseTime(r.EndedAt)
			f.log = append(f.log, Entry{Time: end.Add(-time.Millisecond).Format(time.RFC3339Nano), Event: "receipt", RPCMethod: "tools/call", Correlation: r.InvocationIDs[0]})
		}},
		"handler failure in a control": {"1 bad entries", func(t *testing.T, f *fixture) {
			id := f.find(t, "N3/control-matched-a").InvocationIDs[0]
			for i := range f.log {
				if f.log[i].Correlation == id && f.log[i].Event == "tool" {
					f.log[i].Outcome = "error"
				}
			}
		}},
		"control without its invocation": {"exactly one invocation, got 0", func(t *testing.T, f *fixture) {
			f.find(t, "N4-Failed/control-before").InvocationIDs = []string{}
		}},
		"unattributed traffic between steps": {"belong to no step", func(t *testing.T, f *fixture) {
			end, _ := parseTime(f.receipts[3].EndedAt)
			f.log = append(f.log, Entry{Time: end.Add(5 * time.Millisecond).Format(time.RFC3339Nano), Event: "response"})
		}},
		"overlapping windows": {"window overlaps", func(t *testing.T, f *fixture) {
			f.receipts[5].StartedAt = f.receipts[4].StartedAt
		}},
		"missing receipt":   {"missing receipt", func(t *testing.T, f *fixture) { f.receipts = f.receipts[1:] }},
		"duplicate receipt": {"duplicate receipt", func(t *testing.T, f *fixture) { f.receipts = append(f.receipts, f.receipts[0]) }},
		"double provider":   {"not the real MCP server", func(t *testing.T, f *fixture) { f.find(t, "N2a/probe").Provider = "double" }},
		"probe failure":     {"probe reported failure", func(t *testing.T, f *fixture) { f.find(t, "N5a/probe").Pass = false }},
		"control sent a token": {`has path "/mcp" and auth "ok"`, func(t *testing.T, f *fixture) {
			id := f.find(t, "N1/control-before").InvocationIDs[0]
			for i := range f.log {
				if f.log[i].Correlation == id && f.log[i].RPCMethod == "tools/call" && f.log[i].Event == "receipt" {
					f.log[i].Auth = "ok"
				}
			}
		}},
		"control on the token path": {`has path "/mcp-token"`, func(t *testing.T, f *fixture) {
			id := f.find(t, "N11/probe").InvocationIDs[0]
			for i := range f.log {
				if f.log[i].Correlation == id && f.log[i].Event == "tool" {
					f.log[i].Path = "/mcp-token"
				}
			}
		}},
		"entry without a path": {`has path ""`, func(t *testing.T, f *fixture) {
			id := f.find(t, "N3/control-matched-a").InvocationIDs[0]
			for i := range f.log {
				if f.log[i].Correlation == id && f.log[i].Event == "response" {
					f.log[i].Path = ""
				}
			}
		}},
	} {
		t.Run(name, func(t *testing.T) {
			f := complete()
			tc.mutate(t, f)
			_, err := Check(f.receipts, f.log, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want rejection containing %q, got %v", tc.want, err)
			}
		})
	}
}

// A receipt's own expected call count cannot lower the bar.
func TestReceiptExpectationsAreIgnored(t *testing.T) {
	f := complete()
	var raw []map[string]any
	for _, r := range f.receipts {
		data, _ := json.Marshal(r)
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		m["expectedToolsCalls"] = 0
		raw = append(raw, m)
	}
	var lines []string
	for _, m := range raw {
		data, _ := json.Marshal(m)
		lines = append(lines, string(data))
	}
	receipts, err := ReadReceipts(strings.NewReader(strings.Join(lines, "\n")))
	if err != nil || len(receipts) != len(f.receipts) {
		t.Fatalf("plain JSONL receipts: %d, %v", len(receipts), err)
	}
	if _, err := Check(receipts, nil, nil); err == nil {
		t.Fatal("receipts claiming zero calls passed against an empty server log")
	}
}

func TestReadersRejectMalformedInput(t *testing.T) {
	if _, err := ReadLog(strings.NewReader("not json\n")); err == nil {
		t.Fatal("malformed server log accepted")
	}
	if _, err := ReadReceipts(strings.NewReader("E16_PROBE {broken\n")); err == nil {
		t.Fatal("malformed receipt accepted")
	}
	receipts, err := ReadReceipts(strings.NewReader("=== RUN TestE16Probes\n--- SKIP: TestE16Probes\nPASS\n"))
	if err != nil || len(receipts) != 0 {
		t.Fatalf("receipts=%v err=%v", receipts, err)
	}
	if _, err := Check(receipts, nil, nil); err == nil {
		t.Fatal("a skipped probe run with no receipts passed")
	}
}

func priorOf(id string, end time.Time, state string) *priorRecords {
	return &priorRecords{calls: map[string]priorCall{id: {end: end, state: state}}, late: map[string]int{}}
}

// An earlier Work's whole session, logged before it ended, is history; after
// it ends only bounded completions may follow, once each.
func TestPriorRecordsSeparateHistoryFromNewEntries(t *testing.T) {
	end := time.Date(2026, 10, 3, 1, 0, 0, 0, time.UTC)
	check := func(p *priorRecords, d time.Duration, event, method, rpc string) error {
		at := end.Add(d)
		return p.admit(Entry{Time: at.Format(time.RFC3339Nano), Correlation: "x", Event: event, HTTPMethod: method, RPCMethod: rpc}, at)
	}
	rejection := func(p *priorRecords, d time.Duration, status int) error {
		at := end.Add(d)
		return p.admit(Entry{Time: at.Format(time.RFC3339Nano), Correlation: "x", Event: "response", HTTPMethod: "POST", RPCMethod: "initialize", Status: status, Path: "/mcp-token"}, at)
	}
	// The fixture logs its 401 after answering, so it may follow the
	// recorded outcome; once, within the cleanup window, and nothing else.
	rejected := priorOf("x", end, "tool-credential-rejected")
	if err := rejection(rejected, time.Millisecond, 401); err != nil {
		t.Fatalf("late 401 of a rejected token: %v", err)
	}
	if err := rejection(rejected, 2*time.Millisecond, 401); err == nil {
		t.Fatal("a second 401 of a rejected token was accepted")
	}
	for name, tc := range map[string]struct {
		p      *priorRecords
		at     time.Duration
		status int
	}{
		"401 after the cleanup window":      {priorOf("x", end, "tool-credential-rejected"), 3 * time.Second, 401},
		"initialize answered 200 after end": {priorOf("x", end, "tool-credential-rejected"), time.Millisecond, 200},
		"401 for an unresolved token":       {priorOf("x", end, "tool-credential-unavailable"), time.Millisecond, 401},
		"401 for a completed call":          {priorOf("x", end, "configured-tool"), time.Millisecond, 401},
		"401 for a denied call":             {priorOf("x", end, "denied"), time.Millisecond, 401},
	} {
		if err := rejection(tc.p, tc.at, tc.status); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	ok := priorOf("x", end, "configured-tool")
	for _, entry := range [][3]string{{"receipt", "POST", "initialize"}, {"receipt", "POST", "tools/call"}, {"tool", "", ""}, {"response", "POST", "tools/call"}} {
		if err := check(ok, 0, entry[0], entry[1], entry[2]); err != nil {
			t.Fatalf("historical %v: %v", entry, err)
		}
	}
	if err := check(ok, 2*time.Second, "response", "DELETE", ""); err != nil {
		t.Fatalf("session close completion: %v", err)
	}
	for name, tc := range map[string]struct {
		p                  *priorRecords
		at                 time.Duration
		event, method, rpc string
	}{
		"handler right after a completed call":    {priorOf("x", end, "configured-tool"), time.Nanosecond, "tool", "", ""},
		"handler a second after a completed call": {priorOf("x", end, "configured-tool"), time.Second, "tool", "", ""},
		"new request after the end":               {priorOf("x", end, "configured-tool"), time.Nanosecond, "receipt", "POST", "tools/call"},
		"session close after the cleanup window":  {priorOf("x", end, "configured-tool"), 3 * time.Second, "response", "DELETE", ""},
		"handler for a denied call":               {priorOf("x", end, "denied"), time.Nanosecond, "tool", "", ""},
		"timed-out call after a minute":           {priorOf("x", end, "tool-timeout"), 2 * time.Minute, "response", "POST", "tools/call"},
		"session close of an unresolved token":    {priorOf("x", end, "tool-credential-unavailable"), time.Nanosecond, "receipt", "DELETE", ""},
		"session close of a rejected token":       {priorOf("x", end, "tool-credential-rejected"), time.Nanosecond, "response", "DELETE", ""},
	} {
		if err := check(tc.p, tc.at, tc.event, tc.method, tc.rpc); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	// A timed-out call finishes on the server once: handler, tools/call
	// response and session close, each at most once.
	slow := priorOf("x", end, "tool-timeout")
	for _, entry := range [][3]string{{"tool", "", ""}, {"response", "POST", "tools/call"}, {"response", "DELETE", ""}} {
		if err := check(slow, 29*time.Second, entry[0], entry[1], entry[2]); err != nil {
			t.Fatalf("late completion %v: %v", entry, err)
		}
	}
	twice := priorOf("x", end, "tool-timeout")
	if err := check(twice, time.Second, "tool", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := check(twice, 30*time.Second, "tool", "", ""); err == nil {
		t.Fatal("a second late handler entry was accepted")
	}
}
