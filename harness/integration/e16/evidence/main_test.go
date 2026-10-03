// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
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

// session appends one complete MCP session for a successful call.
func (f *fixture) session(id string) {
	f.log = append(f.log,
		Entry{Time: f.tick(), Event: "receipt", RPCMethod: "initialize", Correlation: id},
		Entry{Time: f.tick(), Event: "receipt", RPCMethod: "notifications/initialized", Correlation: id},
		Entry{Time: f.tick(), Event: "receipt", RPCMethod: "tools/call", Correlation: id},
		Entry{Time: f.tick(), Event: "tool", Correlation: id, Outcome: "ok", File: "README.md"},
		Entry{Time: f.tick(), Event: "response", RPCMethod: "tools/call", Correlation: id},
	)
}

func complete() *fixture {
	f := &fixture{clock: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)}
	n := 0
	for key := range Expected() {
		parts := strings.SplitN(key, "/", 2)
		r := Receipt{Case: parts[0], Step: parts[1], Provider: "mcp", Pass: true, StartedAt: f.tick()}
		if parts[1] != "probe" || parts[0] == "N11" {
			n++
			id := fmt.Sprintf("inv-%d", n)
			r.ExpectedCalls, r.InvocationIDs = 1, []string{id}
			f.session(id)
		}
		r.EndedAt = f.tick()
		f.receipts = append(f.receipts, r)
	}
	return f
}

func TestCompleteCampaignPasses(t *testing.T) {
	f := complete()
	if _, err := Check(f.receipts, f.log); err != nil {
		t.Fatal(err)
	}
}

func TestCheckerRejects(t *testing.T) {
	probe := func(f *fixture, name string) int {
		for i, r := range f.receipts {
			if r.Case+"/"+r.Step == name {
				return i
			}
		}
		t.Fatalf("no %s", name)
		return -1
	}
	for name, mutate := range map[string]func(*fixture){
		"call inside a zero-call window": func(f *fixture) {
			r := f.receipts[probe(f, "N1/probe")]
			f.log = append(f.log, Entry{Time: r.StartedAt, Event: "receipt", RPCMethod: "tools/call"})
		},
		"initialize inside a zero-call window": func(f *fixture) {
			r := f.receipts[probe(f, "N3/probe")]
			f.log = append(f.log, Entry{Time: r.EndedAt, Event: "receipt", RPCMethod: "initialize"})
		},
		"missing receipt":   func(f *fixture) { f.receipts = f.receipts[1:] },
		"duplicate receipt": func(f *fixture) { f.receipts = append(f.receipts, f.receipts[0]) },
		"double provider":   func(f *fixture) { f.receipts[probe(f, "N2a/probe")].Provider = "double" },
		"probe failure":     func(f *fixture) { f.receipts[probe(f, "N5a/probe")].Pass = false },
		"replayed call": func(f *fixture) {
			r := f.receipts[probe(f, "N11/probe")]
			f.log = append(f.log, Entry{Time: r.EndedAt, Event: "receipt", RPCMethod: "tools/call", Correlation: r.InvocationIDs[0]})
		},
		"control without a server call": func(f *fixture) {
			r := &f.receipts[probe(f, "N1/control-before")]
			r.InvocationIDs = []string{"never-seen"}
		},
		"unattributed traffic": func(f *fixture) {
			// Between two steps: after one step ends and before the next starts.
			end, _ := parseTime(f.receipts[0].EndedAt)
			f.log = append(f.log, Entry{Time: end.Add(5 * time.Millisecond).Format(time.RFC3339Nano), Event: "receipt", RPCMethod: "tools/call"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := complete()
			mutate(f)
			if _, err := Check(f.receipts, f.log); err == nil {
				t.Fatal("accepted")
			}
		})
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
	if _, err := Check(receipts, nil); err == nil {
		t.Fatal("a skipped probe run with no receipts passed")
	}
}
