// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

func TestClaimValidation(t *testing.T) {
	for _, claim := range []string{"claim-51", "claim:invoice.retry_1"} {
		if !validClaim(claim) {
			t.Fatalf("valid claim rejected: %q", claim)
		}
	}
	for _, claim := range []string{"", "claim other", "claim/other", "claim\nother", strings.Repeat("a", 129)} {
		if validClaim(claim) {
			t.Fatalf("invalid claim accepted: %q", claim)
		}
	}
}

func TestProbeResultIsDeterministicAndClaimBound(t *testing.T) {
	a := probeResult("claim-a")
	if a != probeResult("claim-a") || a == probeResult("claim-b") || !strings.HasPrefix(a, "probe-") {
		t.Fatalf("probe result is not stable and claim-bound: %q", a)
	}
}

func TestIdleStatusDoesNotClaimWorker(t *testing.T) {
	w := &worker{state: "idle"}
	if res := w.handle(request{Action: "status", Claim: "claim-a"}); res != (response{State: "idle", Claim: "claim-a", Result: "none"}) {
		t.Fatalf("idle status must not imply task start: %+v", res)
	}
	if w.claim != "" || w.state != "idle" {
		t.Fatalf("status unexpectedly reserved the worker: %+v", w)
	}
	if res := w.handle(request{Action: "status", Claim: "claim-b"}); res != (response{State: "idle", Claim: "claim-b", Result: "none"}) {
		t.Fatalf("unbound worker should still be idle: %+v", res)
	}
	if res := w.handle(request{Action: "start", Claim: "claim/invalid"}); res.Error == "" {
		t.Fatalf("invalid claim accepted: %+v", res)
	}
	if w.claim != "" || w.state != "idle" {
		t.Fatalf("invalid start unexpectedly mutated the worker: %+v", w)
	}
}

func TestStopBeforeStartCancelsAndBindsClaim(t *testing.T) {
	w := &worker{state: "idle"}
	if res := w.handle(request{Action: "stop", Claim: "claim-a"}); res != (response{State: "stopped", Claim: "claim-a", Result: "none"}) {
		t.Fatalf("pre-start cancellation must be confirmed: %+v", res)
	}
	if w.command != nil || w.done != nil {
		t.Fatalf("cancellation started a child: %+v", w)
	}
	for _, action := range []string{"start", "stop", "status"} {
		if res := w.handle(request{Action: action, Claim: "claim-b"}); res.Error == "" {
			t.Fatalf("%s accepted a different claim: %+v", action, res)
		}
	}
	if res := w.handle(request{Action: "start", Claim: "claim-a"}); res.Error == "" {
		t.Fatalf("start after cancellation accepted: %+v", res)
	}
	if res := w.handle(request{Action: "status", Claim: "claim-a"}); res != (response{State: "stopped", Claim: "claim-a", Result: "none"}) {
		t.Fatalf("status lost ownership or stop state: %+v", res)
	}
	if res := w.handle(request{Action: "stop", Claim: "claim-a"}); res != (response{State: "stopped", Claim: "claim-a", Result: "none"}) {
		t.Fatalf("repeat stop must preserve confirmed cancellation: %+v", res)
	}
}
