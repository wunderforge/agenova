// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package modelgateway

import (
	"testing"
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/workloadidentity"
)

func TestAuthenticatedModelInvocationCannotBorrowAnotherClaim(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	identity, err := workloadidentity.New([]byte("0123456789abcdef0123456789abcdef"), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	gw, claims, store, _, adapter := fixture(t, WithWorkloadIdentity(identity))
	req := teamARequest(t)
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)
	claims.Put("claim-b", v1alpha1.ClaimPhaseRunning)
	token, err := identity.Issue(req.ClaimID, "worker:"+req.ClaimID, "model-gateway", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := gw.Invoke(req); d.Result != gateway.ResultDeny || len(adapter.calls) != 0 {
		t.Fatalf("bare claim accepted: %+v", d)
	}
	foreign := req
	foreign.ClaimID = "claim-b"
	if d, _ := gw.InvokeAuthenticated(foreign, token); d.Result != gateway.ResultDeny || len(adapter.calls) != 0 || len(store.ModelInvocations("claim-b")) != 0 {
		t.Fatalf("cross-claim identity accepted or attributed: %+v", d)
	}
	if got := store.ModelInvocations(req.ClaimID); len(got) != 1 || got[0].Result != gateway.ResultDeny {
		t.Fatalf("cross-claim denial not attributed to verified claim: %+v", got)
	}
	if d, _ := gw.InvokeAuthenticated(req, "invalid"); d.Result != gateway.ResultDeny || len(adapter.calls) != 0 {
		t.Fatalf("invalid identity accepted: %+v", d)
	}
	wrongWorker, _ := identity.Issue(req.ClaimID, "worker:other", "model-gateway", now.Add(time.Minute))
	if d, _ := gw.InvokeAuthenticated(req, wrongWorker); d.Result != gateway.ResultDeny || len(adapter.calls) != 0 {
		t.Fatalf("wrong worker accepted: %+v", d)
	}
	wrongAudience, _ := identity.Issue(req.ClaimID, "worker:"+req.ClaimID, "tool-gateway", now.Add(time.Minute))
	if d, _ := gw.InvokeAuthenticated(req, wrongAudience); d.Result != gateway.ResultDeny || len(adapter.calls) != 0 {
		t.Fatalf("wrong audience accepted: %+v", d)
	}
	if d, err := gw.InvokeAuthenticated(req, token); err != nil || d.Result != gateway.ResultAllow || len(adapter.calls) != 1 {
		t.Fatalf("valid identity rejected: %+v, %v", d, err)
	}
	if err := identity.Revoke(token, "model-gateway"); err != nil {
		t.Fatal(err)
	}
	if d, _ := gw.InvokeAuthenticated(req, token); d.Result != gateway.ResultDeny || len(adapter.calls) != 1 {
		t.Fatalf("revoked credential invoked provider: %+v", d)
	}
	token, err = identity.Issue(req.ClaimID, "worker:"+req.ClaimID, "model-gateway", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	claim := claims.Items[req.ClaimID]
	claim.Phase = v1alpha1.ClaimPhaseSucceeded
	claims.Items[req.ClaimID] = claim
	if d, _ := gw.InvokeAuthenticated(req, token); d.Result != gateway.ResultDeny || len(adapter.calls) != 1 {
		t.Fatalf("terminal claim invoked provider: %+v", d)
	}
}
