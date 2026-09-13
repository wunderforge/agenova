// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"testing"
)

func TestStore_RecordAndQueryToolInvocations(t *testing.T) {
	s := NewStore()

	s.RecordToolInvocation("claim-a", "web-search", "inv-1", "Allow")
	s.RecordToolInvocation("claim-a", "code-exec", "inv-2", "Allow")
	s.RecordToolInvocation("claim-b", "web-search", "inv-3", "Allow")

	got := s.ToolInvocations("claim-a")
	if len(got) != 2 {
		t.Fatalf("ToolInvocations(claim-a) = %d, want 2", len(got))
	}
	if got[0].ToolName != "web-search" {
		t.Errorf("got[0].ToolName = %q, want %q", got[0].ToolName, "web-search")
	}
	if got[1].ToolName != "code-exec" {
		t.Errorf("got[1].ToolName = %q, want %q", got[1].ToolName, "code-exec")
	}
	for _, inv := range got {
		if inv.ClaimID != "claim-a" {
			t.Errorf("ToolInvocation.ClaimID = %q, want claim-a", inv.ClaimID)
		}
		if inv.Timestamp.IsZero() {
			t.Error("ToolInvocation.Timestamp must not be zero")
		}
	}

	if len(s.ToolInvocations("claim-b")) != 1 {
		t.Error("claim-b should have exactly 1 tool invocation")
	}
	if len(s.ToolInvocations("claim-unknown")) != 0 {
		t.Error("unknown claim should return empty slice")
	}
}

func TestStore_RecordAndQueryModelInvocations(t *testing.T) {
	s := NewStore()

	s.RecordModelInvocation("claim-a", "claude-sonnet-4-6", "inv-4", "Allow")
	s.RecordModelInvocation("claim-b", "claude-haiku-4-5", "inv-5", "Allow")

	got := s.ModelInvocations("claim-a")
	if len(got) != 1 {
		t.Fatalf("ModelInvocations(claim-a) = %d, want 1", len(got))
	}
	if got[0].ModelName != "claude-sonnet-4-6" {
		t.Errorf("ModelName = %q, want claude-sonnet-4-6", got[0].ModelName)
	}
	if got[0].Timestamp.IsZero() {
		t.Error("ModelInvocation.Timestamp must not be zero")
	}
}

func TestStore_RecordAndQueryRuntimeEvents(t *testing.T) {
	s := NewStore()

	s.RecordRuntimeEvent("claim-a", "started")
	s.RecordRuntimeEvent("claim-a", "tool-authorized")
	s.RecordRuntimeEvent("claim-b", "started")

	got := s.RuntimeEvents("claim-a")
	if len(got) != 2 {
		t.Fatalf("RuntimeEvents(claim-a) = %d, want 2", len(got))
	}
	if got[0].Kind != "started" {
		t.Errorf("got[0].Kind = %q, want started", got[0].Kind)
	}
	if got[1].Kind != "tool-authorized" {
		t.Errorf("got[1].Kind = %q, want tool-authorized", got[1].Kind)
	}
}

func TestStore_IsolatesByClaimID(t *testing.T) {
	s := NewStore()

	s.RecordToolInvocation("parent", "p-tool", "inv-p1", "Allow")
	s.RecordToolInvocation("child", "c-tool", "inv-c1", "Allow")
	s.RecordModelInvocation("parent", "p-model", "inv-p2", "Allow")
	s.RecordModelInvocation("child", "c-model", "inv-c2", "Allow")

	if len(s.ToolInvocations("parent")) != 1 {
		t.Error("parent should have exactly 1 tool invocation")
	}
	if len(s.ToolInvocations("child")) != 1 {
		t.Error("child should have exactly 1 tool invocation")
	}
	if len(s.ModelInvocations("parent")) != 1 {
		t.Error("parent should have exactly 1 model invocation")
	}
	if len(s.ModelInvocations("child")) != 1 {
		t.Error("child should have exactly 1 model invocation")
	}
}
