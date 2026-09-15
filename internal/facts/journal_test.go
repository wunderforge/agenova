// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"sync"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

func newRegisteredJournal(t *testing.T) *Journal {
	t.Helper()
	j := NewJournal()
	for _, ref := range []string{"a", "b"} {
		if err := j.RegisterRequest(ref, v0.Principal{Subject: "user:" + ref}); err != nil {
			t.Fatal(err)
		}
		if err := j.BindClaim(ref, v0.SandboxClaim{ID: "claim:" + ref, RequestRef: ref, AuthorityRef: "authority:" + ref}); err != nil {
			t.Fatal(err)
		}
	}
	return j
}

func TestJournalInvocationAttemptOutcomeOrdering(t *testing.T) {
	j := newRegisteredJournal(t)
	p := &v0.PolicyReference{ID: "policy", Version: "1"}
	base := Fact{RequestRef: "a", ClaimID: "claim:a", InvocationID: "inv:model", PolicyRef: p}
	attempt := base
	attempt.Kind = "ProviderAttempt"
	if _, err := j.Append(attempt); err == nil {
		t.Fatal("attempt without Allow accepted")
	}
	d := base
	d.Kind = "ModelDecision"
	d.Result = v0.DecisionResultAllow
	d.ReasonCode = "allowed"
	if _, err := j.Append(d); err != nil {
		t.Fatal(err)
	}
	outcome := base
	outcome.Kind = "ProviderOutcome"
	outcome.ProviderStatus = "Succeeded"
	if _, err := j.Append(outcome); err == nil {
		t.Fatal("outcome without attempt accepted")
	}
	if _, err := j.Append(attempt); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append(attempt); err == nil {
		t.Fatal("duplicate attempt accepted")
	}
	if _, err := j.Append(outcome); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append(outcome); err == nil {
		t.Fatal("duplicate outcome accepted")
	}
	d.InvocationID = "inv:denied"
	d.Result = v0.DecisionResultDeny
	if _, err := j.Append(d); err != nil {
		t.Fatal(err)
	}
	attempt.InvocationID = d.InvocationID
	if _, err := j.Append(attempt); err == nil {
		t.Fatal("denied provider attempt accepted")
	}
}

func TestJournalAuthorityAndBackendCannotBeReassigned(t *testing.T) {
	j := newRegisteredJournal(t)
	if _, err := j.Append(Fact{Kind: "AuthorityResolved", RequestRef: "a", ClaimID: "claim:a", Authority: &v0.EffectiveAuthority{ID: "authority:b"}}); err == nil {
		t.Fatal("foreign authority accepted")
	}
	id := &v0.SandboxClaimBackendIdentity{Backend: "kind", WorkerID: "one"}
	if _, err := j.Append(Fact{Kind: "Runtime", RequestRef: "a", ClaimID: "claim:a", BackendIdentity: id}); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append(Fact{Kind: "Runtime", RequestRef: "b", ClaimID: "claim:b", BackendIdentity: id}); err == nil {
		t.Fatal("foreign worker accepted")
	}
}

func TestJournalDefensiveAppendAndQuery(t *testing.T) {
	j := newRegisteredJournal(t)
	a := &v0.EffectiveAuthority{ID: "authority:a", Tools: []string{"git.read"}}
	f, err := j.Append(Fact{RequestRef: "a", ClaimID: "claim:a", Kind: "AuthorityResolved", Authority: a})
	if err != nil {
		t.Fatal(err)
	}
	a.Tools[0] = "shell.exec"
	f.Authority.Tools[0] = "shell.exec"
	got := j.ForRequest("a")
	got[0].Authority.Tools[0] = "shell.exec"
	if j.ForRequest("a")[0].Authority.Tools[0] != "git.read" {
		t.Fatal("fact mutated through a caller snapshot")
	}
	if _, err = j.Append(Fact{ID: f.ID, RequestRef: "a", Kind: "Rewrite"}); err == nil {
		t.Fatal("duplicate fact ID accepted")
	}
}

func TestJournalRejectsCrossClaimAndInvocation(t *testing.T) {
	j := newRegisteredJournal(t)
	if _, err := j.Append(Fact{RequestRef: "a", ClaimID: "claim:b", Kind: "Runtime"}); err == nil {
		t.Fatal("cross-claim fact accepted")
	}
	if _, err := j.Append(Fact{RequestRef: "a", ClaimID: "claim:a", InvocationID: "inv:1", Kind: "Tool", Result: v0.DecisionResultAllow}); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append(Fact{RequestRef: "b", ClaimID: "claim:b", InvocationID: "inv:1", Kind: "ProviderOutcome"}); err == nil {
		t.Fatal("invocation reassigned")
	}
	if len(j.ForClaim("claim:b")) != 0 {
		t.Fatal("rejected operation fabricated another claim's fact")
	}
}

func TestJournalPreClaimTypedDecisionAndOrdering(t *testing.T) {
	j := NewJournal()
	if err := j.RegisterRequest("denied", v0.Principal{Subject: "user:b"}); err != nil {
		t.Fatal(err)
	}
	for _, result := range []v0.DecisionResult{v0.DecisionResultAllow, v0.DecisionResultDeny, v0.DecisionResultApprovalRequired} {
		d := &v0.Decision{ID: "decision:" + string(result), PrincipalRef: "user:b", Action: "claim.create", Result: result, PolicyRef: v0.PolicyReference{ID: "policy", Version: "1"}, Reason: "recorded reason"}
		if _, err := j.Append(Fact{RequestRef: "denied", Kind: "Decision", Decision: d, ReasonCode: "recorded-code"}); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := j.Append(Fact{RequestRef: "denied", Kind: "Observed"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got := j.ForRequest("denied")
	for i, f := range got {
		if f.Sequence != uint64(i+1) || f.ClaimID != "" || f.Timestamp.IsZero() {
			t.Fatalf("invalid order/correlation: %+v", f)
		}
	}
	if got[2].Decision.Result != v0.DecisionResultApprovalRequired {
		t.Fatal("typed approval result collapsed")
	}
	if _, err := j.Append(Fact{RequestRef: "denied", Kind: "Decision", Decision: &v0.Decision{Result: "true"}}); err == nil {
		t.Fatal("untyped decision accepted")
	}
}
