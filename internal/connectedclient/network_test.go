// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package connectedclient

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authority"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
)

func networkView(t *testing.T, failed bool, releaseBeforeTerminal bool) evidence.View {
	t.Helper()
	status := "Succeeded"
	if failed {
		status = "Failed"
	}
	var view evidence.View
	if err := json.Unmarshal([]byte(installedEvidenceJSON("network", status)), &view); err != nil {
		t.Fatal(err)
	}
	view.State.Claim.BackendIdentity.AllocationID = "original-incarnation"
	var next []facts.Fact
	for _, f := range view.Facts {
		if f.BackendIdentity != nil {
			id := *view.State.Claim.BackendIdentity
			f.BackendIdentity = &id
		}
		if f.Kind == "Runtime" && (f.Operation == "Succeeded" || f.Operation == "Failed") {
			next = append(next, networkFact(view, "NetworkActivationAttempt"))
			if failed {
				f = networkFact(view, "NetworkActivationFailed")
			} else {
				next = append(next, networkFact(view, "NetworkActive"))
			}
			if releaseBeforeTerminal {
				next = append(next, networkFact(view, "NetworkRevocationAttempt"), networkFact(view, "NetworkRevoked"))
			}
			next = append(next, f)
			if !releaseBeforeTerminal {
				next = append(next, networkFact(view, "NetworkRevocationAttempt"), networkFact(view, "NetworkRevoked"))
			}
			continue
		}
		next = append(next, f)
	}
	view.Facts = next
	refreshNetworkView(&view)
	return view
}

func networkFact(v evidence.View, operation string) facts.Fact {
	id, p := *v.State.Claim.BackendIdentity, v.State.PolicyRef
	return facts.Fact{Kind: "Runtime", RequestRef: v.RequestRef, ClaimID: v.State.Claim.ID, BackendIdentity: &id,
		PolicyRef: &p, Operation: operation, Target: "work-network", ReasonCode: "runtime-" + operation,
		Reason: "Simulated lifecycle configuration only; not a packet-level enforcement test."}
}

func refreshNetworkView(v *evidence.View) {
	v.State.Evidence.RuntimeEvents = nil
	for i := range v.Facts {
		v.Facts[i].ID = fmt.Sprintf("network-fact:%d", i)
		v.Facts[i].Sequence = uint64(i + 1)
		v.Facts[i].Timestamp = time.Date(2026, 10, 20, 0, 0, 0, i, time.UTC)
		if v.Facts[i].Kind == "Runtime" && v.Facts[i].Operation != "Pending" {
			v.State.Evidence.RuntimeEvents = append(v.State.Evidence.RuntimeEvents, v0.EvidenceRuntimeEvent{Kind: v.Facts[i].Operation})
		}
	}
}

func changeNetworkFact(v *evidence.View, operation string, mutate func(*facts.Fact)) {
	for i := range v.Facts {
		if v.Facts[i].Operation == operation {
			mutate(&v.Facts[i])
			return
		}
	}
	panic("test fact missing: " + operation)
}

func TestNetworkLifecycleEvidenceAndHistoricalCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name           string
		failed, before bool
	}{
		{"normal release", false, false}, {"cancel ordering release before terminal", false, true}, {"activation failure", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := networkView(t, tc.failed, tc.before)
			if !validEvidenceView(v, v.RequestRef) {
				t.Fatal("valid network lifecycle rejected")
			}
		})
	}
	if _, err := decodeView([]byte(installedEvidenceJSON("historical", "Succeeded")), "historical"); err != nil {
		t.Fatal("historical evidence was retroactively required to prove network enforcement", err)
	}
}

func TestNetworkLifecycleRejectsForgedFacts(t *testing.T) {
	for name, mutate := range map[string]func(*evidence.View){
		"wrong incarnation": func(v *evidence.View) {
			changeNetworkFact(v, "NetworkActive", func(f *facts.Fact) { f.BackendIdentity.AllocationID = "replacement" })
		},
		"missing identity": func(v *evidence.View) {
			changeNetworkFact(v, "NetworkActive", func(f *facts.Fact) { f.BackendIdentity = nil })
		},
		"wrong policy": func(v *evidence.View) {
			changeNetworkFact(v, "NetworkActive", func(f *facts.Fact) { f.PolicyRef.Version = "other" })
		},
		"unknown event": func(v *evidence.View) {
			changeNetworkFact(v, "NetworkActive", func(f *facts.Fact) { f.Operation = "NetworkPacketProof" })
		},
		"invented release": func(v *evidence.View) {
			changeNetworkFact(v, "NetworkRevocationAttempt", func(f *facts.Fact) { f.Operation = "NetworkRevoked"; f.ReasonCode = "runtime-NetworkRevoked" })
		},
		"missing activation attempt": func(v *evidence.View) {
			changeNetworkFact(v, "NetworkActivationAttempt", func(f *facts.Fact) { f.Operation = "NetworkActive"; f.ReasonCode = "runtime-NetworkActive" })
		},
		"bad target": func(v *evidence.View) {
			changeNetworkFact(v, "NetworkActive", func(f *facts.Fact) { f.Target = "another-work" })
		},
		"unbounded reason": func(v *evidence.View) {
			changeNetworkFact(v, "NetworkActive", func(f *facts.Fact) { f.Reason = string(make([]byte, 4097)) })
		},
		"forged invocation": func(v *evidence.View) {
			changeNetworkFact(v, "NetworkActive", func(f *facts.Fact) { f.InvocationID = "fake" })
		},
		"missing release result": func(v *evidence.View) {
			changeNetworkFact(v, "NetworkRevoked", func(f *facts.Fact) { f.Kind = "WorkerActivity"; f.Operation = "FinalAnswer" })
		},
		"activity after release": func(v *evidence.View) {
			f := facts.Fact{Kind: "WorkerActivity", RequestRef: v.RequestRef, ClaimID: v.State.Claim.ID, Operation: "TurnStarted"}
			for i, g := range v.Facts {
				if g.Operation == "Succeeded" {
					v.Facts = append(v.Facts[:i], append([]facts.Fact{f}, v.Facts[i:]...)...)
					return
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			v := networkView(t, false, name == "activity after release")
			mutate(&v)
			refreshNetworkView(&v)
			if validEvidenceView(v, v.RequestRef) {
				t.Fatal("accepted malformed network evidence")
			}
		})
	}
}

func TestNetworkReleaseFailureRetryAndRecovery(t *testing.T) {
	for _, afterTermination := range []bool{false, true} {
		v := networkView(t, false, false)
		var next []facts.Fact
		for _, f := range v.Facts {
			next = append(next, f)
			if f.Operation == "NetworkRevoked" && !afterTermination || f.Operation == "TerminateSucceeded" && afterTermination {
				for _, op := range []string{"NetworkRevocationAttempt", "NetworkRevoked"} {
					r := networkFact(v, op)
					r.ReasonCode = "recovery-" + op
					next = append(next, r)
				}
			}
		}
		v.Facts = next
		refreshNetworkView(&v)
		if !validEvidenceView(v, v.RequestRef) {
			t.Fatal("explicit recovery release retry rejected", afterTermination)
		}
	}
	v := networkView(t, false, false)
	changeNetworkFact(&v, "NetworkRevoked", func(f *facts.Fact) {
		f.Operation = "NetworkRevocationFailed"
		f.ReasonCode = "runtime-NetworkRevocationFailed"
	})
	refreshNetworkView(&v)
	if !validEvidenceView(v, v.RequestRef) {
		t.Fatal("failed release evidence must remain readable, not rewritten as success")
	}
}

func TestCancelledProviderMayFinishAfterNetworkRevocation(t *testing.T) {
	v := networkView(t, false, false)
	grantPublicNetwork(&v)
	v.State.Claim.Phase = v0.ClaimPhaseFailed
	v.Outcome = &evidence.Outcome{Status: "Cancelled", Failure: "synthetic cancellation"}
	v.Request.Spec.RequestedAccess.ModelProfile = "coding-standard"
	v.State.EffectiveAuthority.ModelProfile = "coding-standard"
	var next []facts.Fact
	for _, f := range v.Facts {
		if f.Authority != nil {
			f.Authority.ModelProfile = "coding-standard"
		}
		if f.Operation == "Succeeded" {
			f.Operation = "Cancelled"
		}
		if f.Kind == "RunOutcome" {
			f.Operation, f.Reason = "Cancelled", "synthetic cancellation"
		}
		next = append(next, f)
		if f.Operation == "NetworkActive" {
			p := v.State.PolicyRef
			next = append(next,
				facts.Fact{Kind: "ModelDecision", RequestRef: v.RequestRef, ClaimID: v.State.Claim.ID, InvocationID: "inflight", PolicyRef: &p, Operation: "model.invoke", Target: "coding-standard", Result: v0.DecisionResultAllow},
				facts.Fact{Kind: "ProviderAttempt", RequestRef: v.RequestRef, ClaimID: v.State.Claim.ID, InvocationID: "inflight", Operation: "model.invoke", Target: "coding-standard", ProviderStatus: "Attempted"})
		}
		if f.Operation == "NetworkRevoked" {
			next = append(next, facts.Fact{Kind: "ProviderOutcome", RequestRef: v.RequestRef, ClaimID: v.State.Claim.ID, InvocationID: "inflight", Operation: "model.invoke", Target: "coding-standard", ProviderStatus: "Cancelled"})
		}
	}
	v.Facts = next
	refreshNetworkView(&v)
	if !validEvidenceView(v, v.RequestRef) {
		t.Fatal("legitimate cancelled provider completion after network release rejected")
	}
	for i := range v.Facts {
		if v.Facts[i].Kind == "ProviderOutcome" {
			v.Facts[i].ProviderStatus = "Succeeded"
		}
	}
	if validEvidenceView(v, v.RequestRef) {
		t.Fatal("forged successful provider completion after cancellation accepted")
	}
}

func TestNetworkAuthorityCannotExpandOrDisagree(t *testing.T) {
	v := networkView(t, false, false)
	v.State.EffectiveAuthority.NetworkProfiles = []string{"public-web"}
	for i := range v.Facts {
		if v.Facts[i].Authority != nil {
			v.Facts[i].Authority.NetworkProfiles = []string{"public-web"}
		}
	}
	if validEvidenceView(v, v.RequestRef) {
		t.Fatal("unsolicited public network grant accepted")
	}
	v.Request.Spec.RequestedAccess.NetworkProfiles = []string{"public-web"}
	if !validEvidenceView(v, v.RequestRef) {
		t.Fatal("correlated requested and effective network grant rejected")
	}
	for i := range v.Facts {
		if v.Facts[i].Authority != nil {
			v.Facts[i].Authority.NetworkProfiles = nil
		}
	}
	if validEvidenceView(v, v.RequestRef) {
		t.Fatal("authority snapshot mismatch accepted")
	}
	request := v.Request
	grant := *v.State.EffectiveAuthority
	grant.NetworkProfiles = nil
	if !validAuthorityChanges(request, grant, []authority.Change{{Field: "networkProfiles", Requested: "public-web", ReasonCode: "outside-policy-constraint"}}) {
		t.Fatal("network narrowing provenance omitted")
	}
	if validAuthorityChanges(request, grant, nil) {
		t.Fatal("network narrowing was silently hidden")
	}
}

func TestToolArgumentRejectionIsNotAnInvocation(t *testing.T) {
	base := networkView(t, false, false)
	rejected := facts.Fact{Kind: "WorkerActivity", RequestRef: base.RequestRef, ClaimID: base.State.Claim.ID, Operation: "ToolRequestRejected", Target: "tool.invoke", ReasonCode: "tool-arguments-rejected", Reason: "Tool parameters do not match an installed route; no provider call was made."}
	for _, bad := range []string{"", "invocation", "provider", "reason", "after terminal"} {
		v := networkView(t, false, false)
		f := rejected
		switch bad {
		case "invocation":
			f.InvocationID = "invented"
		case "provider":
			f.ProviderStatus = "Attempted"
		case "reason":
			f.Reason = "arbitrary model refusal"
		}
		for i, g := range v.Facts {
			if g.Operation == "Succeeded" {
				if bad == "after terminal" {
					i++
				}
				v.Facts = append(v.Facts[:i], append([]facts.Fact{f}, v.Facts[i:]...)...)
				break
			}
		}
		refreshNetworkView(&v)
		if validEvidenceView(v, v.RequestRef) != (bad == "") {
			t.Fatalf("rejection shape validity wrong: %q", bad)
		}
	}
}

func grantPublicNetwork(v *evidence.View) {
	v.Request.Spec.RequestedAccess.NetworkProfiles = []string{"public-web"}
	v.State.EffectiveAuthority.NetworkProfiles = []string{"public-web"}
	for i := range v.Facts {
		if v.Facts[i].Authority != nil {
			v.Facts[i].Authority.NetworkProfiles = []string{"public-web"}
		}
	}
}

func withoutNetworkFacts(v evidence.View) evidence.View {
	var kept []facts.Fact
	for _, f := range v.Facts {
		if !isNetworkOperation(f.Operation) {
			kept = append(kept, f)
		}
	}
	v.Facts = kept
	refreshNetworkView(&v)
	return v
}

func TestExplicitNetworkGrantCannotOmitWholeLifecycle(t *testing.T) {
	for _, terminalOutcome := range []bool{false, true} {
		v := networkView(t, false, false)
		grantPublicNetwork(&v)
		v = withoutNetworkFacts(v)
		if !terminalOutcome {
			v.Outcome = nil
			v.Facts = v.Facts[:len(v.Facts)-1]
			refreshNetworkView(&v)
		}
		if validEvidenceView(v, v.RequestRef) {
			t.Fatal("public-web success accepted with the entire network lifecycle omitted", terminalOutcome)
		}
	}
	// Empty/omitted networking predates this extension. Do not retroactively
	// label it isolated, or make real historical records impossible to inspect.
	v := withoutNetworkFacts(networkView(t, false, false))
	if !validEvidenceView(v, v.RequestRef) {
		t.Fatal("historical evidence compatibility regressed")
	}
}

func TestExplicitNetworkGrantRequiresActivationBeforeDispatch(t *testing.T) {
	for _, kind := range []string{"WorkerActivity", "ModelDecision", "ProviderAttempt"} {
		v := networkView(t, false, false)
		grantPublicNetwork(&v)
		v = withoutNetworkFacts(v)
		v.Outcome = nil
		v.State.Claim.Phase = v0.ClaimPhaseRunning
		var prefix []facts.Fact
		for _, f := range v.Facts {
			prefix = append(prefix, f)
			if f.Operation == "Running" {
				break
			}
		}
		v.Facts = prefix
		v.Request.Spec.RequestedAccess.ModelProfile = "coding-standard"
		v.State.EffectiveAuthority.ModelProfile = "coding-standard"
		for i := range v.Facts {
			if v.Facts[i].Authority != nil {
				v.Facts[i].Authority.ModelProfile = "coding-standard"
			}
		}
		refreshNetworkView(&v)
		if !validEvidenceView(v, v.RequestRef) {
			t.Fatal("Running-before-activation poll must be readable")
		}
		p := v.State.PolicyRef
		decision := facts.Fact{Kind: "ModelDecision", RequestRef: v.RequestRef, ClaimID: v.State.Claim.ID, InvocationID: "early", PolicyRef: &p, Operation: "model.invoke", Target: "coding-standard", Result: v0.DecisionResultAllow}
		if kind == "WorkerActivity" {
			v.Facts = append(v.Facts, facts.Fact{Kind: kind, RequestRef: v.RequestRef, ClaimID: v.State.Claim.ID, Operation: "TurnStarted"})
		} else {
			v.Facts = append(v.Facts, decision)
			if kind == "ProviderAttempt" {
				v.Facts = append(v.Facts, facts.Fact{Kind: kind, RequestRef: v.RequestRef, ClaimID: v.State.Claim.ID, InvocationID: "early", Operation: "model.invoke", Target: "coding-standard", ProviderStatus: "Attempted"})
			}
		}
		refreshNetworkView(&v)
		if validEvidenceView(v, v.RequestRef) {
			t.Fatal("dispatch/activity without network activation accepted", kind)
		}
	}
}

func TestExplicitNetworkGrantRejectsIncompleteTerminalLifecycle(t *testing.T) {
	for _, omitted := range []string{"NetworkActivationAttempt", "NetworkActive", "NetworkRevocationAttempt", "NetworkRevoked", "release pair"} {
		t.Run(omitted, func(t *testing.T) {
			v := networkView(t, false, false)
			grantPublicNetwork(&v)
			var kept []facts.Fact
			for _, f := range v.Facts {
				if f.Operation != omitted && !(omitted == "release pair" && isNetworkRevocationOperation(f.Operation)) {
					kept = append(kept, f)
				}
			}
			v.Facts = kept
			refreshNetworkView(&v)
			if validEvidenceView(v, v.RequestRef) {
				t.Fatal("incomplete explicit network lifecycle accepted")
			}
		})
	}
}

func TestExplicitNetworkGrantAllowsLegitimatePartialPollingAndEarlyFailure(t *testing.T) {
	for _, stop := range []string{"AuthorityResolved", "Bound", "BackendReady", "Running", "NetworkActivationAttempt", "NetworkActive"} {
		t.Run(stop, func(t *testing.T) {
			v := networkView(t, false, false)
			grantPublicNetwork(&v)
			for i, f := range v.Facts {
				if f.Kind == stop || f.Operation == stop {
					v.Facts = v.Facts[:i+1]
					break
				}
			}
			v.Outcome = nil
			v.State.Claim.Phase = v0.ClaimPhaseRunning
			if stop == "AuthorityResolved" {
				v.State.Claim.Phase = v0.ClaimPhasePending
				v.State.Claim.BackendIdentity = nil
			}
			if stop == "Bound" || stop == "BackendReady" {
				v.State.Claim.Phase = v0.ClaimPhaseBound
			}
			refreshNetworkView(&v)
			if !validEvidenceView(v, v.RequestRef) {
				t.Fatal("legitimate queued/startup/activation snapshot rejected")
			}
		})
	}
	v := networkView(t, true, false)
	grantPublicNetwork(&v)
	if !validEvidenceView(v, v.RequestRef) {
		t.Fatal("terminal activation failure falsely required NetworkActive")
	}
	for i, f := range v.Facts {
		if f.Operation == "NetworkActivationFailed" {
			v.Facts = v.Facts[:i+1]
			break
		}
	}
	v.Outcome = nil
	refreshNetworkView(&v)
	if !validEvidenceView(v, v.RequestRef) {
		t.Fatal("activation failure crash window unreadable")
	}
	v.Recovery = &evidence.Recovery{Code: "service-restarted", Cleanup: "Unknown"}
	if !validEvidenceView(v, v.RequestRef) {
		t.Fatal("activation failure recovery snapshot unreadable")
	}
	// Cancellation can legitimately win after Running but before any network
	// grant or worker dispatch. Release facts must not be mistaken for activation.
	v = networkView(t, true, false)
	grantPublicNetwork(&v)
	v.Outcome.Status = "Cancelled"
	var cancelled []facts.Fact
	for _, f := range v.Facts {
		if f.Operation == "NetworkActivationAttempt" {
			continue
		}
		if f.Operation == "NetworkActivationFailed" || f.Kind == "RunOutcome" {
			f.Operation = "Cancelled"
		}
		cancelled = append(cancelled, f)
	}
	v.Facts = cancelled
	refreshNetworkView(&v)
	if !validEvidenceView(v, v.RequestRef) {
		t.Fatal("pre-activation cancellation falsely required a successful grant")
	}
}
