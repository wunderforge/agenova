// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
)

const dataset = "../../mcpfixture/data"

const goodBlock = `
deadline_resets_each_attempt: yes
first_attempt_seconds: 4.0s
backoff_seconds: 2
total_deadline_seconds: 5
budget_exceeded: yes
stable_idempotency_key_needed: yes
fix_shares_one_deadline_across_attempts: yes
fix_reuses_one_idempotency_key_across_retries: yes`

const goodAnswer = "Each retry starts a fresh 5 second deadline, so the 4s attempt plus the 2s backoff overruns the budget. " +
	"Keep one deadline across attempts and reuse a stable idempotency key." + goodBlock

const goodN8 = "The timeline was cut off after the first part, so later events are not summarised.\ntimeline_complete: no"

// The resource scopes the Work inputs request (work-<case>.yaml). The MCP
// client returns resultRef as the scope, "/" and the file it asked the server
// for (internal/adapters/bundled/mcp_client.go).
const (
	positiveScope = "repo:agenova/e16-fixture"
	faultScope    = "repo:agenova/e16-faults"
)

type workBuilder struct {
	view     evidence.View
	log      []Entry
	clock    time.Time
	scope    string
	sessions int
}

func newWork(status, answer, scope string) *workBuilder {
	w := &workBuilder{clock: time.Date(2026, 10, 3, 1, 0, 0, 0, time.UTC), scope: scope}
	w.view = evidence.View{RequestRef: "e16-case", State: &v0.IssuedState{Decision: v0.Decision{Result: "Allow"}, Claim: &v0.SandboxClaim{ID: "claim"}},
		Outcome: &evidence.Outcome{Status: status, Text: answer}}
	w.fact(facts.Fact{Kind: "RequestReceived"})
	return w
}

func (w *workBuilder) now() time.Time { w.clock = w.clock.Add(10 * time.Millisecond); return w.clock }

func (w *workBuilder) fact(f facts.Fact) {
	f.Timestamp = w.now()
	w.view.Facts = append(w.view.Facts, f)
}

// server adds an entry the fixture would not write on its own, for tests
// that inject unexpected traffic.
func (w *workBuilder) server(e Entry) {
	e.Time = w.now().Format(time.RFC3339Nano)
	w.log = append(w.log, e)
}

// fixture adds one line in the fixture's own log format (every field it
// writes, harness/integration/mcpfixture/main.go), read back through ReadLog
// as the runner reads the real log.
func (w *workBuilder) fixture(fields map[string]any) {
	fields["time"] = w.now().Format(time.RFC3339Nano)
	fields["pod"] = "e16-mcp-7d9c5b8f6-x2k4q"
	line, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}
	entries, err := ReadLog(bytes.NewReader(line))
	if err != nil || len(entries) != 1 {
		panic(fmt.Sprintf("fixture line %s: %v", line, err))
	}
	w.log = append(w.log, entries[0])
}

// session logs one MCP client session for an invocation as the fixture
// does: initialize, the initialized notification, tools/call and its handler
// for the file, then the closing DELETE, each request with its response.
func (w *workBuilder) session(id, file string) {
	w.sessions++
	s := fmt.Sprintf("%012x", w.sessions)
	type m = map[string]any
	w.fixture(m{"event": "receipt", "httpMethod": "POST", "rpcMethod": "initialize", "rpcId": "1", "correlation": id})
	w.fixture(m{"event": "response", "httpMethod": "POST", "rpcMethod": "initialize", "rpcId": "1", "correlation": id, "status": 200, "bytes": 210, "durationMs": 1})
	w.fixture(m{"event": "receipt", "httpMethod": "POST", "rpcMethod": "notifications/initialized", "correlation": id, "session": s})
	w.fixture(m{"event": "response", "httpMethod": "POST", "rpcMethod": "notifications/initialized", "correlation": id, "status": 202})
	w.fixture(m{"event": "receipt", "httpMethod": "POST", "rpcMethod": "tools/call", "rpcId": "2", "tool": "read_file", "file": file, "correlation": id, "session": s})
	w.fixture(m{"event": "tool", "tool": "read_file", "file": file, "correlation": id, "outcome": "ok", "bytes": 490})
	w.fixture(m{"event": "response", "httpMethod": "POST", "rpcMethod": "tools/call", "rpcId": "2", "correlation": id, "status": 200, "bytes": 598})
	w.fixture(m{"event": "receipt", "httpMethod": "DELETE", "correlation": id, "session": s})
	w.fixture(m{"event": "response", "httpMethod": "DELETE", "correlation": id, "status": 204})
}

// call records one invocation and, when attempted, its server session.
func (w *workBuilder) call(id, decision, file, outcome, reason string, truncated bool) {
	w.fact(facts.Fact{Kind: "ToolDecision", InvocationID: id, Operation: "tool.invoke", Result: v0.DecisionResult(decision)})
	if decision != "Allow" {
		return
	}
	w.fact(facts.Fact{Kind: "ProviderAttempt", InvocationID: id, Operation: "tool.invoke", ProviderStatus: "Attempted"})
	w.session(id, file)
	ref := ""
	if outcome == "Succeeded" {
		ref = w.scope + "/" + file
	}
	w.fact(facts.Fact{Kind: "ProviderOutcome", InvocationID: id, Operation: "tool.invoke", ProviderStatus: outcome, ReasonCode: reason, ResultRef: ref, Truncated: truncated})
}

func (w *workBuilder) end() { w.fact(facts.Fact{Kind: "RunOutcome"}) }

// setResultRef replaces the recorded resultRef of every successful outcome.
func (w *workBuilder) setResultRef(ref string) {
	for i, f := range w.view.Facts {
		if f.Kind == "ProviderOutcome" && f.ProviderStatus == "Succeeded" {
			w.view.Facts[i].ResultRef = ref
		}
	}
}

func positive() *workBuilder {
	w := newWork("Succeeded", goodAnswer, positiveScope)
	w.call("a", "Allow", "logs/timeout.log", "Succeeded", "configured-tool", false)
	w.call("b", "Allow", "src/retry.txt", "Succeeded", "configured-tool", false)
	w.end()
	return w
}

func timeoutWork(file string) *workBuilder {
	w := newWork("Failed", "", faultScope)
	w.call("a", "Allow", file, "Failed", "tool-timeout", false)
	w.end()
	return w
}

func truncationWork(answer string, truncated bool) *workBuilder {
	w := newWork("Succeeded", answer, faultScope)
	w.call("a", "Allow", "notes/incident-timeline.md", "Succeeded", "configured-tool", truncated)
	w.end()
	return w
}

// The runner names each attempt of a case with its own request name. Evidence
// of another attempt, or no named attempt at all, is never judged.
func TestWorkModeJudgesOnlyTheNamedAttempt(t *testing.T) {
	denied := newWork("Deny", "", "")
	denied.view.State = &v0.IssuedState{Decision: v0.Decision{Result: "Deny"}}
	denied.end()
	dir := t.TempDir()
	view, log := filepath.Join(dir, "work-show.json"), filepath.Join(dir, "fixture-full.jsonl")
	data, err := json.Marshal(denied.view)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(view, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(log, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-case", "admission-deny", "-view", view, "-server-log", log, "-fixture-data", dataset}
	for ref, want := range map[string]int{"e16-case": 0, "e16-case-a2": 1, "": 2} {
		if got := workMain(append([]string{"-ref", ref}, args...)); got != want {
			t.Errorf("-ref %q exited %d, want %d", ref, got, want)
		}
	}
	if got := workMain(args); got != 2 {
		t.Errorf("no -ref exited %d, want 2", got)
	}
}

func TestWorkCasesPass(t *testing.T) {
	oversize := newWork("Failed", "", faultScope)
	oversize.call("a", "Allow", "logs/full-trace.log", "Failed", "tool-response-too-large", false)
	oversize.end()
	denied := newWork("Deny", "", "")
	denied.view.State = &v0.IssuedState{Decision: v0.Decision{Result: "Deny"}}
	denied.end()
	for name, w := range map[string]*workBuilder{
		"positive":       positive(),
		"n6-timeout":     timeoutWork("logs/slow.log"),
		"n7-oversize":    oversize,
		"n8-truncation":  truncationWork(goodN8, true),
		"admission-deny": denied,
	} {
		if err := CheckWork(name, w.view, w.log, dataset, nil); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestWorkCasesReject(t *testing.T) {
	for name, tc := range map[string]struct {
		work, want string
		build      func() *workBuilder
	}{
		"wrong answer": {"positive", "facts block", func() *workBuilder {
			w := positive()
			w.view.Outcome.Text = "The upstream API is slow; add more retries."
			return w
		}},
		"read a fault file": {"positive", "outside this case's files", func() *workBuilder {
			w := positive()
			w.call("c", "Allow", "logs/slow.log", "Succeeded", "configured-tool", false)
			return w
		}},
		"replayed call": {"positive", "made 2 tools/call", func() *workBuilder {
			w := positive()
			w.server(Entry{Event: "receipt", RPCMethod: "tools/call", Correlation: "a", File: "logs/timeout.log"})
			w.end()
			return w
		}},
		"unknown traffic during the Work": {"positive", "belong to none of its invocations", func() *workBuilder {
			w := positive()
			w.server(Entry{Event: "receipt", RPCMethod: "tools/call", File: "README.md"})
			w.end()
			return w
		}},
		"denied call reached the server": {"positive", "was not attempted but caused", func() *workBuilder {
			w := positive()
			w.call("d", "Deny", "", "", "", false)
			w.server(Entry{Event: "receipt", RPCMethod: "tools/call", Correlation: "d", File: "README.md"})
			w.end()
			return w
		}},
		"timeout on the wrong file": {"n6-timeout", "failed with \"tool-timeout\" on \"logs/full-trace.log\"", func() *workBuilder {
			return timeoutWork("logs/full-trace.log")
		}},
		"truncation not marked": {"n8-truncation", "marked truncated", func() *workBuilder {
			return truncationWork(goodN8, false)
		}},
		"answer claims a complete timeline": {"n8-truncation", "timeline_complete", func() *workBuilder {
			return truncationWork("Summary.\ntimeline_complete: yes", true)
		}},
	} {
		t.Run(name, func(t *testing.T) {
			w := tc.build()
			err := CheckWork(tc.work, w.view, w.log, dataset, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want rejection containing %q, got %v", tc.want, err)
			}
		})
	}
}

// Correct answers do not prove the investigation read the diagnostic files,
// and a result reference must name the file the server actually read.
func TestWorkReadsAreCorrelatedWithTheServer(t *testing.T) {
	for name, tc := range map[string]struct {
		work  string
		want  []string
		build func() *workBuilder
	}{
		// Codex counterexample 1: README.md reads only, with a correct answer.
		"README-only reads with correct answers": {"positive", []string{"no successful correlated read of logs/timeout.log", "no successful correlated read of src/retry.txt"}, func() *workBuilder {
			w := newWork("Succeeded", goodAnswer, positiveScope)
			w.call("a", "Allow", "README.md", "Succeeded", "configured-tool", false)
			w.call("b", "Allow", "README.md", "Succeeded", "configured-tool", false)
			w.end()
			return w
		}},
		// Codex counterexample 2: real timeout/retry reads whose results name README.md.
		"README resultRefs on real reads": {"positive", []string{
			`invocation a has resultRef "repo:agenova/e16-fixture/README.md", but its tools/call read "logs/timeout.log"`,
			`invocation b has resultRef "repo:agenova/e16-fixture/README.md", but its tools/call read "src/retry.txt"`,
			"no successful correlated read of logs/timeout.log",
		}, func() *workBuilder {
			w := positive()
			w.setResultRef(positiveScope + "/README.md")
			return w
		}},
		"result in another scope": {"positive", []string{`want "repo:agenova/e16-fixture/logs/timeout.log"`}, func() *workBuilder {
			w := positive()
			w.setResultRef("repo:agenova/e16/logs/timeout.log")
			return w
		}},
		"successful result without a reference": {"positive", []string{`invocation a has resultRef ""`}, func() *workBuilder {
			w := positive()
			w.setResultRef("")
			return w
		}},
		"retry file never read": {"positive", []string{"no successful correlated read of src/retry.txt"}, func() *workBuilder {
			w := newWork("Succeeded", goodAnswer, positiveScope)
			w.call("a", "Allow", "logs/timeout.log", "Succeeded", "configured-tool", false)
			w.call("b", "Allow", "README.md", "Succeeded", "configured-tool", false)
			w.end()
			return w
		}},
		"failed read does not count": {"positive", []string{"no successful correlated read of src/retry.txt"}, func() *workBuilder {
			w := newWork("Succeeded", goodAnswer, positiveScope)
			w.call("a", "Allow", "logs/timeout.log", "Succeeded", "configured-tool", false)
			w.call("b", "Allow", "src/retry.txt", "Failed", "tool-timeout", false)
			w.end()
			return w
		}},
		"read without a server handler does not count": {"positive", []string{"no successful correlated read of src/retry.txt"}, func() *workBuilder {
			w := positive()
			kept := w.log[:0]
			for _, e := range w.log {
				if e.Event != "tool" || e.File != "src/retry.txt" {
					kept = append(kept, e)
				}
			}
			w.log = kept
			return w
		}},
		"truncated read names the fixture scope": {"n8-truncation", []string{`want "repo:agenova/e16-faults/notes/incident-timeline.md"`}, func() *workBuilder {
			w := truncationWork(goodN8, true)
			w.setResultRef(positiveScope + "/notes/incident-timeline.md")
			return w
		}},
	} {
		t.Run(name, func(t *testing.T) {
			w := tc.build()
			err := CheckWork(tc.work, w.view, w.log, dataset, nil)
			if err == nil {
				t.Fatal("accepted")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("want rejection containing %q, got %v", want, err)
				}
			}
		})
	}
	// README.md reads beside both diagnostic reads are allowed.
	w := newWork("Succeeded", goodAnswer, positiveScope)
	w.call("r", "Allow", "README.md", "Succeeded", "configured-tool", false)
	w.call("a", "Allow", "logs/timeout.log", "Succeeded", "configured-tool", false)
	w.call("b", "Allow", "src/retry.txt", "Succeeded", "configured-tool", false)
	w.end()
	if err := CheckWork("positive", w.view, w.log, dataset, nil); err != nil {
		t.Fatalf("an extra README.md read: %v", err)
	}
}

// The checker's scopes must be the ones the Work inputs request.
func TestWorkCaseScopesMatchWorkInputs(t *testing.T) {
	for name, c := range workCases {
		if c.denied {
			continue
		}
		data, err := os.ReadFile(filepath.Join("..", "work-"+name+".yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if want := "resourceScopes: [" + c.scope + "]"; !strings.Contains(string(data), want) {
			t.Errorf("work-%s.yaml does not request %q", name, want)
		}
	}
	if workCases["positive"].scope != positiveScope || workCases["n8-truncation"].scope != faultScope {
		t.Fatal("the checker's scopes differ from the MCP client's result references used in these tests")
	}
}

func TestPositiveOracleFollowsTheDataset(t *testing.T) {
	dir := t.TempDir()
	for _, file := range []string{"logs/timeout.log", "src/retry.txt"} {
		data, err := os.ReadFile(filepath.Join(dataset, file))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, file)), 0o755); err != nil {
			t.Fatal(err)
		}
		if file == "logs/timeout.log" {
			data = []byte(strings.ReplaceAll(string(data), "idempotency_key=none", "idempotency_key=pay_7Q2"))
		}
		if err := os.WriteFile(filepath.Join(dir, file), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w := positive()
	if err := CheckWork("positive", w.view, w.log, dir, nil); err == nil || !strings.Contains(err.Error(), `the dataset gives "no"`) {
		t.Fatalf("a changed dataset must fail the oracle, got %v", err)
	}
}

// The facts block decides; prose alone, wrong values, repeats and missing
// keys fail. Text outside the block is not judged.
func TestFactsBlockOracle(t *testing.T) {
	block := func(change func(string) string) string { return "Analysis." + change(goodBlock) }
	for name, tc := range map[string]struct{ work, answer, want string }{
		"prose without a block": {"positive", "The 4s attempt plus 2s backoff fits within the 5s budget. Generate a new idempotency key for every retry.", "appears 0 times"},
		"budget not exceeded":   {"positive", block(func(b string) string { return strings.Replace(b, "budget_exceeded: yes", "budget_exceeded: no", 1) }), `budget_exceeded: "no"`},
		"new key per retry": {"positive", block(func(b string) string {
			return strings.Replace(b, "stable_idempotency_key_needed: yes", "stable_idempotency_key_needed: no", 1)
		}), "stable_idempotency_key_needed"},
		"contradictory repeat": {"positive", block(func(b string) string { return b + "\nbudget_exceeded: no" }), "budget_exceeded appears 2 times"},
		"missing key":          {"positive", block(func(b string) string { return strings.Replace(b, "backoff_seconds: 2\n", "", 1) }), "backoff_seconds appears 0 times"},
		"wrong number": {"positive", block(func(b string) string {
			return strings.Replace(b, "first_attempt_seconds: 4.0s", "first_attempt_seconds: 6", 1)
		}), "first_attempt_seconds"},
		"spelled-out number": {"positive", block(func(b string) string { return strings.Replace(b, "backoff_seconds: 2", "backoff_seconds: two", 1) }), "backoff_seconds"},
		"n8 prose only":      {"n8-truncation", "If the timeline is truncated, I will say so. The timeline is complete.", "timeline_complete appears 0 times"},
		"n8 claims complete": {"n8-truncation", "The fix is not complete yet.\ntimeline_complete: yes", `timeline_complete: "yes"`},
	} {
		t.Run(name, func(t *testing.T) {
			var w *workBuilder
			if tc.work == "positive" {
				w = positive()
			} else {
				w = truncationWork("", true)
			}
			w.view.Outcome.Text = tc.answer
			err := CheckWork(tc.work, w.view, w.log, dataset, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want rejection containing %q, got %v", tc.want, err)
			}
		})
	}
	// A closed fence that merely contains another delimiter is closed.
	w := positive()
	w.view.Outcome.Text = "Example:\n```\n~~~\n```\n" + goodAnswer
	if err := CheckWork("positive", w.view, w.log, dataset, nil); err != nil {
		t.Fatalf("a closed fence before the block: %v", err)
	}
	// Prose before the final block is not judged.
	w = positive()
	w.view.Outcome.Text = "Some prose the checker does not judge.\n" + goodAnswer
	if err := CheckWork("positive", w.view, w.log, dataset, nil); err != nil {
		t.Fatalf("prose before the final block must not change the verdict: %v", err)
	}
	for name, tc := range map[string]struct{ answer, want string }{
		"text after the block":            {goodAnswer + "\nThese statements are false.", "appears 0 times in the final block"},
		"quoted example then refusal":     {"For example:" + goodBlock + "\nI cannot identify a cause or recommend a fix.", "appears 0 times in the final block"},
		"contradiction in other case":     {"BUDGET_EXCEEDED: no\n" + goodAnswer, "budget_exceeded also appears before the final block"},
		"fenced block":                    {"Analysis.\n```" + goodBlock + "\n```", "appears 0 times in the final block"},
		"unclosed fence":                  {"Analysis.\n```text" + goodBlock, "unclosed code fence"},
		"open backtick fence, tilde line": {"Analysis.\n```text\n~~~" + goodBlock, "unclosed code fence"},
		"diagnosis without the fix":       {"Analysis." + strings.Join(strings.Split(goodBlock, "\n")[:7], "\n"), "fix_shares_one_deadline_across_attempts appears 0 times"},
		"a new key for every retry":       {"Analysis." + strings.Replace(goodBlock, "fix_reuses_one_idempotency_key_across_retries: yes", "fix_reuses_one_idempotency_key_across_retries: no", 1), "fix_reuses_one_idempotency_key_across_retries"},
	} {
		t.Run(name, func(t *testing.T) {
			w := positive()
			w.view.Outcome.Text = tc.answer
			err := CheckWork("positive", w.view, w.log, dataset, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want rejection containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestWorkServerProofIsComplete(t *testing.T) {
	for name, tc := range map[string]struct {
		want  string
		build func() *workBuilder
	}{
		"denied call opened a session": {"was not attempted but caused 1 server entries", func() *workBuilder {
			w := positive()
			w.call("d", "Deny", "", "", "", false)
			w.server(Entry{Event: "receipt", RPCMethod: "initialize", Correlation: "d"})
			w.end()
			return w
		}},
		"no handler proof for a successful read": {"successful server handler entries", func() *workBuilder {
			w := positive()
			kept := w.log[:0]
			for _, e := range w.log {
				if e.Event != "tool" {
					kept = append(kept, e)
				}
			}
			w.log = kept
			return w
		}},
		"sessions outside the Work": {"outside its attempt window", func() *workBuilder {
			w := positive()
			for i := range w.log {
				w.log[i].Time = "2026-10-03T00:00:00Z"
			}
			return w
		}},
	} {
		t.Run(name, func(t *testing.T) {
			w := tc.build()
			err := CheckWork("positive", w.view, w.log, dataset, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want rejection containing %q, got %v", tc.want, err)
			}
		})
	}
	// Session cleanup may complete just after a successful outcome; a new
	// request after the outcome may not.
	w0 := positive()
	outcome := w0.view.Facts[len(w0.view.Facts)-2].Timestamp
	w0.log = append(w0.log, Entry{Time: outcome.Add(time.Millisecond).Format(time.RFC3339Nano), Event: "response", HTTPMethod: "DELETE", Correlation: "b"})
	if err := CheckWork("positive", w0.view, w0.log, dataset, nil); err != nil {
		t.Fatalf("delayed session cleanup: %v", err)
	}
	w0.log = append(w0.log, Entry{Time: outcome.Add(time.Millisecond).Format(time.RFC3339Nano), Event: "receipt", HTTPMethod: "POST", RPCMethod: "tools/call", Correlation: "b"})
	if err := CheckWork("positive", w0.view, w0.log, dataset, nil); err == nil {
		t.Fatal("a new request after the outcome was accepted")
	}
	// A previous Work's timed-out call may complete during this Work, but may
	// not start anything new.
	w1 := positive()
	during := w1.view.Facts[1].Timestamp.Add(time.Millisecond).Format(time.RFC3339Nano)
	w1.log = append(w1.log, Entry{Time: during, Event: "tool", Correlation: "earlier-timeout", Outcome: "ok"}, Entry{Time: during, Event: "response", HTTPMethod: "POST", RPCMethod: "tools/call", Correlation: "earlier-timeout"})
	prior := priorOf("earlier-timeout", w1.view.Facts[0].Timestamp.Add(-time.Second), "tool-timeout")
	if err := CheckWork("positive", w1.view, w1.log, dataset, nil); err == nil {
		t.Fatal("unreconciled late entries were accepted")
	}
	if err := CheckWork("positive", w1.view, w1.log, dataset, prior); err != nil {
		t.Fatalf("reconciled late completion: %v", err)
	}
	w1.log = append(w1.log, Entry{Time: during, Event: "receipt", RPCMethod: "tools/call", Correlation: "earlier-timeout"})
	if err := CheckWork("positive", w1.view, w1.log, dataset, priorOf("earlier-timeout", w1.view.Facts[0].Timestamp.Add(-time.Second), "tool-timeout")); err == nil || !strings.Contains(err.Error(), "caused a new server entry") {
		t.Fatalf("an earlier invocation's new request: %v", err)
	}
	// A timed-out call's server handler may finish after the client gave up.
	w := timeoutWork("logs/slow.log")
	late := w.view.Facts[len(w.view.Facts)-1].Timestamp.Add(30 * time.Second)
	w.log = append(w.log, Entry{Time: late.Format(time.RFC3339Nano), Event: "tool", Correlation: "a", Outcome: "ok", File: "logs/slow.log"})
	if err := CheckWork("n6-timeout", w.view, w.log, dataset, nil); err != nil {
		t.Fatalf("late completion of a timed-out call: %v", err)
	}
}

// The deadline fact must come from a supported statement in the dataset.
func TestUnsupportedDatasetFormIsRejected(t *testing.T) {
	dir := t.TempDir()
	for _, file := range []string{"logs/timeout.log", "src/retry.txt"} {
		data, err := os.ReadFile(filepath.Join(dataset, file))
		if err != nil {
			t.Fatal(err)
		}
		if file == "src/retry.txt" {
			data = []byte(strings.Replace(string(data), "Each attempt starts a fresh 5 second deadline instead of using the time", "Each attempt starts a fresh cache entry. All attempts share one total deadline", 1))
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, file)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w := positive()
	if err := CheckWork("positive", w.view, w.log, dir, nil); err == nil || !strings.Contains(err.Error(), "dataset form unsupported") {
		t.Fatalf("got %v", err)
	}
}
