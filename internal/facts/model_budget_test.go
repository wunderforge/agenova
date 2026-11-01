// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package facts

import (
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"testing"
)

func TestBudgetReservationRejectsReplayLimitChangesAndCrossWork(t *testing.T) {
	j := NewJournal()
	_ = j.RegisterRequest("a", v0.Principal{Subject: "alice"})
	_ = j.RegisterRequest("b", v0.Principal{Subject: "bob"})
	_ = j.BindClaim("a", v0.SandboxClaim{ID: "claim-a", RequestRef: "a"})
	_ = j.BindClaim("b", v0.SandboxClaim{ID: "claim-b", RequestRef: "b"})
	if ok, err := j.ReserveModelCall("a", "claim-a", "call-1", 2); !ok || err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		ref, claim, id string
		limit          int
	}{{"a", "claim-a", "call-1", 2}, {"a", "claim-a", "call-2", 20}, {"a", "claim-b", "call-2", 2}, {"a", "claim-a", "call-2", 0}} {
		if ok, err := j.ReserveModelCall(tc.ref, tc.claim, tc.id, tc.limit); ok || err == nil {
			t.Fatal("invalid/replayed reservation accepted", tc)
		}
	}
	if ok, err := j.ReserveModelCall("a", "claim-a", "call-2", 2); !ok || err != nil {
		t.Fatal(err)
	}
	if ok, err := j.ReserveModelCall("a", "claim-a", "call-3", 2); ok || err != nil {
		t.Fatal("exhausted quota not refused", ok, err)
	}
	if len(j.ForRequest("a")) != 2 || len(j.ForRequest("b")) != 0 {
		t.Fatal("failed attempts changed accounting")
	}
}

func TestFailedBudgetCommitNeverPublishesOrConsumesReservation(t *testing.T) {
	failing := true
	j, err := NewDurableJournal(0, func(Fact) error {
		if failing {
			return errors.New("disk unavailable")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = j.RegisterRequest("a", v0.Principal{Subject: "alice"})
	_ = j.BindClaim("a", v0.SandboxClaim{ID: "claim-a", RequestRef: "a"})
	if ok, err := j.ReserveModelCall("a", "claim-a", "call-1", 2); ok || err == nil {
		t.Fatal("failed durable reservation accepted")
	}
	if len(j.ForRequest("a")) != 0 {
		t.Fatal("failed commit published")
	}
	failing = false
	if ok, err := j.ReserveModelCall("a", "claim-a", "call-1", 2); !ok || err != nil {
		t.Fatal(err)
	}
	if j.ForRequest("a")[0].ModelCallCount != 1 {
		t.Fatal("failed attempt consumed quota")
	}
}
