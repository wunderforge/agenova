// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package workloadidentity

import (
	"errors"
	"testing"
	"time"
)

func TestIssueVerifyBoundedAndFailClosed(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	service, err := New([]byte("0123456789abcdef0123456789abcdef"), clock)
	if err != nil {
		t.Fatal(err)
	}
	token, err := service.Issue("claim-a", "worker-a", "model-gateway", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	identity, err := service.Verify(token, "model-gateway")
	if err != nil || identity.ClaimID != "claim-a" || identity.WorkerID != "worker-a" || identity.Expires != now.Add(time.Minute).Unix() {
		t.Fatalf("verified identity = %+v, %v", identity, err)
	}
	for _, candidate := range []string{"", token + "x", "claim-a", token[:len(token)-1]} {
		if _, err := service.Verify(candidate, "model-gateway"); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid credential accepted: %v", err)
		}
	}
	if _, err := service.Verify(token, "tool-gateway"); !errors.Is(err, ErrInvalid) {
		t.Fatal("wrong audience accepted")
	}
	if err := service.Revoke(token, "model-gateway"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Verify(token, "model-gateway"); !errors.Is(err, ErrInvalid) {
		t.Fatal("revoked credential accepted")
	}
	token, err = service.Issue("claim-a", "worker-a", "model-gateway", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if _, err := service.Verify(token, "model-gateway"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expired credential accepted")
	}
}
