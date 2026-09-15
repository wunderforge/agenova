// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

func TestControlClaimValidation(t *testing.T) {
	valid := "sha256:" + strings.Repeat("a", 64)
	if !validControlClaim(valid) {
		t.Fatalf("valid claim token rejected: %q", valid)
	}
	for _, claim := range []string{"", "claim-51", "sha256:xyz", "sha256:" + strings.Repeat("a", 62), "sha256:" + strings.Repeat("a", 66)} {
		if validControlClaim(claim) {
			t.Fatalf("invalid claim token accepted: %q", claim)
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
	claimA := "sha256:" + strings.Repeat("a", 64)
	claimB := "sha256:" + strings.Repeat("b", 64)
	if res := w.handle(request{Action: "status", Claim: claimA}); res != (response{State: "idle", Claim: claimA, Result: "none"}) {
		t.Fatalf("idle status must not imply task start: %+v", res)
	}
	if w.claim != "" || w.state != "idle" {
		t.Fatalf("status unexpectedly reserved the worker: %+v", w)
	}
	if res := w.handle(request{Action: "status", Claim: claimB}); res != (response{State: "idle", Claim: claimB, Result: "none"}) {
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
	claimA := "sha256:" + strings.Repeat("a", 64)
	claimB := "sha256:" + strings.Repeat("b", 64)
	if res := w.handle(request{Action: "stop", Claim: claimA}); res != (response{State: "stopped", Claim: claimA, Result: "none"}) {
		t.Fatalf("pre-start cancellation must be confirmed: %+v", res)
	}
	if w.command != nil || w.done != nil {
		t.Fatalf("cancellation started a child: %+v", w)
	}
	for _, action := range []string{"start", "stop", "status"} {
		if res := w.handle(request{Action: action, Claim: claimB}); res.Error == "" {
			t.Fatalf("%s accepted a different claim: %+v", action, res)
		}
	}
	if res := w.handle(request{Action: "start", Claim: claimA}); res.Error == "" {
		t.Fatalf("start after cancellation accepted: %+v", res)
	}
	if res := w.handle(request{Action: "status", Claim: claimA}); res != (response{State: "stopped", Claim: claimA, Result: "none"}) {
		t.Fatalf("status lost ownership or stop state: %+v", res)
	}
	if res := w.handle(request{Action: "stop", Claim: claimA}); res != (response{State: "stopped", Claim: claimA, Result: "none"}) {
		t.Fatalf("repeat stop must preserve confirmed cancellation: %+v", res)
	}
}
