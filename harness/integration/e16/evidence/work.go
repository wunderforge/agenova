// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/wunderforge/agenova/internal/evidence"
)

// workCase states what one production Work must show in its own evidence and
// in the server log. Files are relative to the fixture dataset.
type workCase struct {
	status       string   // Work outcome status
	scope        string   // the resource scope the Work requests (work-<case>.yaml)
	failedReason string   // the single failed tool outcome's reason code, if any
	failedFile   string   // the file that failed call asked for
	files        []string // every file any attempted call may ask for
	required     []string // files that must each have a successful correlated read
	truncated    string   // file whose successful call must be truncated
	denied       bool     // admission denial: no claim, no tool activity
	path         string   // endpoint path of every server entry the Work's calls cause
	auth         string   // credential class of every receipt the Work's calls cause
	credential   string   // Slice 4 token case: one invocation of exactly this shape
}

const (
	fixtureScope = "repo:agenova/e16-fixture"
	faultsScope  = "repo:agenova/e16-faults"
)

// The fixture serves the credential-free backend on /mcp and the
// token-required one on /mcp-token. A receipt on /mcp must have no token
// ("missing"): a token sent to the credential-free backend is a leak.
const (
	freePath  = "/mcp"
	tokenPath = "/mcp-token"
)

// The token cases (Slice 4) each make exactly one call.
const (
	tokenValid       = "valid"       // read README.md; every request carries the token
	tokenUnavailable = "unavailable" // the token cannot be resolved; no request is sent
	tokenRejected    = "rejected"    // the server answers initialize 401; nothing else is sent
)

// configuredTool is the reason code of every configured provider attempt and
// of its successful outcome (internal/console/tool_provider.go).
const configuredTool = "configured-tool"

var workCases = map[string]workCase{
	"positive": {status: "Succeeded", scope: fixtureScope, files: []string{"README.md", "logs/timeout.log", "src/retry.txt"},
		required: []string{"logs/timeout.log", "src/retry.txt"}, path: freePath, auth: "missing"},
	"n6-timeout":     {status: "Failed", scope: faultsScope, failedReason: "tool-timeout", failedFile: "logs/slow.log", files: []string{"logs/slow.log"}, path: freePath, auth: "missing"},
	"n7-oversize":    {status: "Failed", scope: faultsScope, failedReason: "tool-response-too-large", failedFile: "logs/full-trace.log", files: []string{"logs/full-trace.log"}, path: freePath, auth: "missing"},
	"n8-truncation":  {status: "Succeeded", scope: faultsScope, truncated: "notes/incident-timeline.md", files: []string{"notes/incident-timeline.md"}, path: freePath, auth: "missing"},
	"admission-deny": {status: "Deny", denied: true},
	"token-valid": {status: "Succeeded", scope: "repo:agenova/e16-token", credential: tokenValid, files: []string{"README.md"},
		required: []string{"README.md"}, path: tokenPath, auth: "ok"},
	// Nothing may reach the server, so no path or credential class applies.
	"token-missing": {status: "Failed", scope: "repo:agenova/e16-token-missing", credential: tokenUnavailable, failedReason: "tool-credential-unavailable"},
	"token-wrong": {status: "Failed", scope: "repo:agenova/e16-token-wrong", credential: tokenRejected, failedReason: "tool-credential-rejected",
		path: tokenPath, auth: "invalid"},
}

// Answer facts are judged from a fixed facts block the Work objective asks
// the agent to end its answer with ("key: value" lines). Free prose is not
// judged: no pattern can prove what a sentence means, while the block can be
// compared exactly. Each required key must appear exactly once with the value
// derived from the fixture dataset; a missing, repeated or wrong value fails.
// An agent that ignores the format fails and the run is inspected and rerun.

// observationCap is max-observation-bytes in platform.yaml (checked by
// harness/integration/e16/inputs_test.go).
const observationCap = 4096

// The facts block is the answer's final lines, each exactly "key: value" in
// plain text. Required keys must appear there exactly once and nowhere else
// in the answer, in any case or quoting; a fenced or indented block is not a
// facts block.
var finalFactLine = regexp.MustCompile(`^([a-z_]+): *(\S.*?)\s*$`)

func factsBlock(answer string, wanted []string) (map[string]string, error) {
	want := map[string]bool{}
	for _, key := range wanted {
		want[key] = true
	}
	lines := strings.Split(strings.TrimRight(answer, " \t\r\n"), "\n")
	start := len(lines)
	for start > 0 && finalFactLine.MatchString(strings.TrimRight(lines[start-1], " \t\r")) {
		start--
	}
	values, counts := map[string]string{}, map[string]int{}
	for _, line := range lines[start:] {
		m := finalFactLine.FindStringSubmatch(strings.TrimRight(line, " \t\r"))
		if key := m[1]; want[key] {
			counts[key]++
			values[key] = strings.ToLower(m[2])
		}
	}
	var problems []string
	if openFence(lines[:start]) {
		problems = append(problems, "the final lines are inside an unclosed code fence")
	}
	before := strings.ToLower(strings.Join(lines[:start], "\n"))
	for _, key := range wanted {
		if counts[key] != 1 {
			problems = append(problems, fmt.Sprintf("%s appears %d times in the final block", key, counts[key]))
		}
		if strings.Contains(before, key) {
			problems = append(problems, fmt.Sprintf("%s also appears before the final block", key))
		}
	}
	if len(problems) > 0 {
		return nil, errors.New("facts block: " + strings.Join(problems, ", "))
	}
	return values, nil
}

// openFence reports whether the text ends inside a fenced code block. A fence
// opens with three or more backticks or tildes and closes only with a line of
// the same character, at least as long, and nothing else.
func openFence(lines []string) bool {
	var marker byte
	length := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		run := 0
		for run < len(trimmed) && (trimmed[run] == '`' || trimmed[run] == '~') && trimmed[run] == trimmed[0] {
			run++
		}
		if run < 3 {
			continue
		}
		switch {
		case length == 0:
			marker, length = trimmed[0], run
		case trimmed[0] == marker && run >= length && run == len(trimmed):
			length = 0
		}
	}
	return length != 0
}

var number = regexp.MustCompile(`^(\d+(\.\d+)?)\s*(s|sec|secs|seconds?)?$`)

func sameFact(got, want string) bool {
	if m := number.FindStringSubmatch(got); m != nil {
		var a, b float64
		_, err1 := fmt.Sscan(m[1], &a)
		_, err2 := fmt.Sscan(want, &b)
		return err1 == nil && err2 == nil && a == b
	}
	return got == want
}

// expectedFacts derives the required facts block from the dataset itself.
func expectedFacts(name, dataDir string) (map[string]string, error) {
	read := func(file string) (string, error) {
		data, err := os.ReadFile(filepath.Join(dataDir, file))
		return string(data), err
	}
	find := func(text, pattern string) (string, error) {
		m := regexp.MustCompile(pattern).FindStringSubmatch(text)
		if m == nil {
			return "", fmt.Errorf("dataset no longer contains %q", pattern)
		}
		return m[1], nil
	}
	switch name {
	case "positive":
		timeout, err := read("logs/timeout.log")
		if err != nil {
			return nil, err
		}
		retry, err := read("src/retry.txt")
		if err != nil {
			return nil, err
		}
		latency, err1 := find(timeout, `attempt=1 upstream_latency=([0-9.]+)s`)
		backoff, err2 := find(timeout, `retry_backoff=([0-9.]+)s`)
		deadline, err3 := find(timeout, `client_deadline=([0-9.]+)s`)
		key, err4 := find(timeout, `idempotency_key=(\S+)`)
		if err := errors.Join(err1, err2, err3, err4); err != nil {
			return nil, err
		}
		// The retry summary must state the deadline behaviour in one of the
		// two supported forms; anything else is an unsupported dataset.
		fresh := regexp.MustCompile(`(?m)^- Each attempt starts a fresh \d+ second deadline instead of using the time\b`).MatchString(retry)
		shared := regexp.MustCompile(`(?m)^- All attempts share one total deadline\.?$`).MatchString(retry)
		if fresh == shared {
			return nil, errors.New("dataset form unsupported: src/retry.txt must state either a fresh per-attempt deadline or one shared deadline")
		}
		var l, b, d float64
		fmt.Sscan(latency, &l)
		fmt.Sscan(backoff, &b)
		fmt.Sscan(deadline, &d)
		yesNo := map[bool]string{true: "yes", false: "no"}
		return map[string]string{
			"deadline_resets_each_attempt":  yesNo[fresh],
			"first_attempt_seconds":         latency,
			"backoff_seconds":               backoff,
			"total_deadline_seconds":        deadline,
			"budget_exceeded":               yesNo[l+b > d],
			"stable_idempotency_key_needed": yesNo[key == "none"],
			// The requested fix, in structured form: share one deadline across
			// attempts when each resets it, and reuse one idempotency key
			// across retries when none is sent.
			"fix_shares_one_deadline_across_attempts":       yesNo[fresh],
			"fix_reuses_one_idempotency_key_across_retries": yesNo[key == "none"],
		}, nil
	case "n8-truncation":
		info, err := os.Stat(filepath.Join(dataDir, "notes/incident-timeline.md"))
		if err != nil {
			return nil, err
		}
		return map[string]string{"timeline_complete": map[bool]string{true: "no", false: "yes"}[info.Size() > observationCap]}, nil
	}
	return nil, nil
}

// A slow handler may finish after the client gave up; only a timed-out call
// may have server-side completion entries long after its outcome record.
const lateCompletion = time.Minute

// Session cleanup (the DELETE request and any response entry) may complete
// shortly after the outcome record: the client closes with a 1s deadline.
const cleanupCompletion = 2 * time.Second

// completion reports entries that finish work already started rather than
// start new work.
func completion(e Entry) bool {
	return e.Event == "response" || e.Event == "tool" || (e.Event == "receipt" && e.HTTPMethod == "DELETE")
}

// CheckWork decides one production Work from its CLI evidence and the
// complete server log. dataDir is the fixture dataset used by the oracle;
// prior holds the invocations of every earlier attempt in the campaign, failed
// ones included, whose late completion entries may appear during this Work.
func CheckWork(name string, view evidence.View, log []Entry, dataDir string, prior *priorRecords) error {
	c, ok := workCases[name]
	if !ok {
		return fmt.Errorf("unknown Work case %q", name)
	}
	var problems []string
	fail := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	if view.Outcome == nil || view.Outcome.Status != c.status {
		fail("outcome %v, want %s", view.Outcome, c.status)
	}
	type invocation struct {
		decision, outcome, reason, resultRef, attemptReason string
		decisions, attempts, outcomes                       int
		attempted                                           bool
		attemptAt, outcomeAt                                time.Time
		truncated                                           bool
	}
	calls := map[string]*invocation{}
	var first, last time.Time
	for _, f := range view.Facts {
		if first.IsZero() || f.Timestamp.Before(first) {
			first = f.Timestamp
		}
		if f.Timestamp.After(last) {
			last = f.Timestamp
		}
		if f.Operation != "tool.invoke" || f.InvocationID == "" {
			continue
		}
		in := calls[f.InvocationID]
		if in == nil {
			in = &invocation{}
			calls[f.InvocationID] = in
		}
		switch f.Kind {
		case "ToolDecision":
			in.decision = string(f.Result)
			in.decisions++
		case "ProviderAttempt":
			in.attempted, in.attemptAt, in.attemptReason = true, f.Timestamp, f.ReasonCode
			in.attempts++
		case "ProviderOutcome":
			in.outcome, in.reason, in.resultRef, in.truncated, in.outcomeAt = f.ProviderStatus, f.ReasonCode, f.ResultRef, f.Truncated, f.Timestamp
			in.outcomes++
		}
	}
	if c.denied {
		if view.State == nil || view.State.Decision.Result != "Deny" || view.State.Claim != nil || len(calls) != 0 {
			fail("admission denial must have a Deny decision, no claim and no tool activity")
		}
	} else if len(calls) == 0 {
		fail("no tool invocation was recorded")
	}
	if c.credential != "" && len(calls) > 1 {
		fail("a token case makes exactly one tool invocation, got %d", len(calls))
	}

	// Server side, per invocation: everything it caused falls inside its own
	// attempt-to-outcome window (a timed-out call may complete later on the
	// server) and on the case's endpoint path, with the case's credential
	// class on every receipt; denied invocations caused nothing; a successful
	// read has one session, one tools/call and one successful handler for the
	// same file.
	type session struct {
		entries, initialize, calls, handled int
		files, handledFiles                 []string
		log                                 []Entry
	}
	seen := map[string]*session{}
	unknown := 0
	for _, e := range log {
		at, _ := parseTime(e.Time)
		in := calls[e.Correlation]
		if e.Correlation == "" || in == nil {
			if prior.known(e.Correlation) {
				if err := prior.admit(e, at); err != nil {
					fail("%v", err)
				}
				continue
			}
			if !first.IsZero() && !at.Before(first) && !at.After(last) {
				unknown++
			}
			continue
		}
		ss := seen[e.Correlation]
		if ss == nil {
			ss = &session{}
			seen[e.Correlation] = ss
		}
		ss.entries++
		ss.log = append(ss.log, e)
		if !in.attempted {
			continue // reported below as traffic from a call that was never attempted
		}
		latest := in.outcomeAt
		switch {
		case completion(e) && in.reason == "tool-timeout":
			latest = latest.Add(lateCompletion)
		case completion(e):
			latest = latest.Add(cleanupCompletion)
		}
		if at.Before(in.attemptAt) || in.outcomeAt.IsZero() || at.After(latest) {
			fail("server entry for invocation %s at %s is outside its attempt window", e.Correlation, e.Time)
		}
		// Any entry at all fails a call that had no token to send (below).
		if c.credential != tokenUnavailable {
			if e.Path != c.path {
				fail("server %s entry for invocation %s at %s has path %q, want %q", e.Event, e.Correlation, e.Time, e.Path, c.path)
			}
			if e.Event == "receipt" && e.Auth != c.auth {
				fail("receipt for invocation %s at %s has auth %q, want %q", e.Correlation, e.Time, e.Auth, c.auth)
			}
		}
		switch {
		case e.Event == "receipt" && e.RPCMethod == "initialize":
			ss.initialize++
		case e.Event == "receipt" && e.RPCMethod == "tools/call":
			ss.calls++
			ss.files = append(ss.files, e.File)
		case e.Event == "tool" && e.Outcome == "ok":
			ss.handled++
			ss.handledFiles = append(ss.handledFiles, e.File)
		}
	}
	if unknown != 0 {
		fail("%d server entries during the Work belong to none of its invocations", unknown)
	}
	failed, truncatedOK := 0, c.truncated == ""
	read := map[string]bool{} // files with a successful correlated read
	ids := make([]string, 0, len(calls))
	for id := range calls {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		in, ss := calls[id], seen[id]
		if ss == nil {
			ss = &session{}
		}
		// A token case's one invocation has exactly these facts and reason
		// codes, so a denial or a different failure never stands in for it.
		if c.credential != "" {
			outcome := "Succeeded"
			want := configuredTool
			if c.failedReason != "" {
				outcome, want = "Failed", c.failedReason
			}
			if in.decisions != 1 || in.decision != "Allow" || in.attempts != 1 || in.attemptReason != configuredTool ||
				in.outcomes != 1 || in.outcome != outcome || in.reason != want {
				fail("invocation %s must have one ToolDecision Allow, one ProviderAttempt %q and one ProviderOutcome %s %q; got %d decisions (%s), %d attempts (%q), %d outcomes (%s %q)",
					id, configuredTool, outcome, want, in.decisions, in.decision, in.attempts, in.attemptReason, in.outcomes, in.outcome, in.reason)
			}
		}
		if !in.attempted {
			if ss.entries != 0 {
				fail("invocation %s was not attempted but caused %d server entries", id, ss.entries)
			}
			continue
		}
		// A call that failed on its credential never reached tools/call, so
		// it is judged by its exact server shape before the session rules.
		if c.credential == tokenUnavailable || c.credential == tokenRejected {
			if c.credential == tokenUnavailable && ss.entries != 0 {
				fail("invocation %s had no token to send but caused %s", id, describe(ss.log))
			}
			if c.credential == tokenRejected && !rejectedOnce(ss.log) {
				fail("invocation %s must show one initialize answered 401 and nothing else, got %s", id, describe(ss.log))
			}
			if in.outcome == "Failed" {
				failed++
			}
			if in.resultRef != "" || in.truncated {
				fail("failed invocation %s carries a result", id)
			}
			continue
		}
		if ss.calls != 1 || ss.initialize != 1 {
			fail("invocation %s made %d tools/call and %d initialize requests, want 1 each", id, ss.calls, ss.initialize)
			continue
		}
		file := ss.files[0]
		if !contains(c.files, file) {
			fail("invocation %s read %q, outside this case's files %v", id, file, c.files)
		}
		switch in.outcome {
		case "Failed":
			failed++
			if in.reason != c.failedReason || file != c.failedFile {
				fail("invocation %s failed with %q on %q, want %q on %q", id, in.reason, file, c.failedReason, c.failedFile)
			}
			if in.resultRef != "" || in.truncated {
				fail("failed invocation %s carries a result", id)
			}
		case "Succeeded":
			handled := ss.handled == 1 && ss.handledFiles[0] == file
			if !handled {
				fail("successful invocation %s has %d successful server handler entries for %q", id, ss.handled, file)
			}
			// The result must name the file the server actually read, not
			// just any file the route allows.
			want := c.scope + "/" + file
			if in.resultRef != want {
				fail("successful invocation %s has resultRef %q, but its tools/call read %q (want %q)", id, in.resultRef, file, want)
			}
			if handled && in.resultRef == want {
				read[file] = true
			}
			if in.truncated && file == c.truncated {
				truncatedOK = true
			}
		default:
			fail("invocation %s has outcome %q", id, in.outcome)
		}
	}
	if c.failedReason != "" && failed != 1 {
		fail("want exactly one failed tool call, got %d", failed)
	}
	if c.failedReason == "" && failed != 0 {
		fail("unexpected failed tool calls: %d", failed)
	}
	if !truncatedOK {
		fail("no successful call on %s was marked truncated", c.truncated)
	}
	// Correct answers alone do not show the investigation used the files.
	for _, file := range c.required {
		if !read[file] {
			fail("no successful correlated read of %s", file)
		}
	}

	answer := ""
	if view.Outcome != nil {
		answer = view.Outcome.Text
	}
	// A token case proves the credential path; its answer has no facts block
	// and is not judged.
	var want map[string]string
	var err error
	if c.credential == "" {
		want, err = expectedFacts(name, dataDir)
	}
	if err != nil {
		fail("%v", err)
	} else if want != nil {
		keys := make([]string, 0, len(want))
		for key := range want {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		got, err := factsBlock(answer, keys)
		if err != nil {
			fail("answer %v", err)
		}
		for _, key := range keys {
			if got != nil && !sameFact(got[key], want[key]) {
				fail("answer states %s: %q, the dataset gives %q", key, got[key], want[key])
			}
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// rejectedOnce reports whether an invocation's server entries are exactly
// one initialize request and its 401 response: same HTTP method, RPC method
// and id, so nothing was retried or sent after the rejection.
func rejectedOnce(log []Entry) bool {
	if len(log) != 2 {
		return false
	}
	req, resp := log[0], log[1]
	return req.Event == "receipt" && req.HTTPMethod == "POST" && req.RPCMethod == "initialize" && req.RPCID != "" && req.Error == "" &&
		resp.Event == "response" && resp.HTTPMethod == req.HTTPMethod && resp.RPCMethod == req.RPCMethod && resp.RPCID == req.RPCID && resp.Status == 401
}

// describe summarises server entries for a failure message.
func describe(log []Entry) string {
	parts := make([]string, 0, len(log))
	for _, e := range log {
		part := strings.Join(strings.Fields(e.Event+" "+e.HTTPMethod+" "+e.RPCMethod), " ")
		if e.Status != 0 {
			part += fmt.Sprintf(" %d", e.Status)
		}
		parts = append(parts, part)
	}
	return fmt.Sprintf("%d server entries [%s]", len(log), strings.Join(parts, ", "))
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
