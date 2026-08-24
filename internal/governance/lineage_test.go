// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"testing"
)

func TestLineage_RegisterAndQuery(t *testing.T) {
	l := NewLineage()

	if err := l.RegisterChild("parent-1", "child-a"); err != nil {
		t.Fatalf("RegisterChild: %v", err)
	}
	if err := l.RegisterChild("parent-1", "child-b"); err != nil {
		t.Fatalf("RegisterChild: %v", err)
	}

	p, ok := l.Parent("child-a")
	if !ok || p != "parent-1" {
		t.Fatalf("Parent(child-a) = %q, %v; want parent-1, true", p, ok)
	}

	_, ok = l.Parent("parent-1")
	if ok {
		t.Error("parent-1 should have no parent")
	}

	children := l.Children("parent-1")
	if len(children) != 2 {
		t.Fatalf("Children(parent-1) = %d, want 2", len(children))
	}

	if !l.IsChildOf("parent-1", "child-a") {
		t.Error("child-a should be a child of parent-1")
	}
	if !l.IsChildOf("parent-1", "child-b") {
		t.Error("child-b should be a child of parent-1")
	}
	if l.IsChildOf("parent-1", "child-c") {
		t.Error("child-c is not registered, should not be a child")
	}
}

func TestLineage_IdempotentReregistration(t *testing.T) {
	l := NewLineage()

	if err := l.RegisterChild("parent-1", "child-a"); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := l.RegisterChild("parent-1", "child-a"); err != nil {
		t.Fatalf("idempotent re-register: %v", err)
	}

	// Should not duplicate child in the list.
	if len(l.Children("parent-1")) != 1 {
		t.Errorf("re-registration should not duplicate child, got %d", len(l.Children("parent-1")))
	}
}

func TestLineage_ConflictingParent(t *testing.T) {
	l := NewLineage()

	if err := l.RegisterChild("parent-1", "child-a"); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := l.RegisterChild("parent-2", "child-a"); err == nil {
		t.Fatal("registering child under a different parent should fail")
	}
}

func TestLineage_SelfParent(t *testing.T) {
	l := NewLineage()

	if err := l.RegisterChild("claim-x", "claim-x"); err == nil {
		t.Fatal("a claim cannot be its own parent")
	}
}

func TestLineage_EmptyIDs(t *testing.T) {
	l := NewLineage()

	if err := l.RegisterChild("", "child-a"); err == nil {
		t.Fatal("empty parent ID should be rejected")
	}
	if err := l.RegisterChild("parent-1", ""); err == nil {
		t.Fatal("empty child ID should be rejected")
	}
}

func TestLineage_UnregisteredClaimHasNoChildren(t *testing.T) {
	l := NewLineage()

	if children := l.Children("nonexistent"); children != nil {
		t.Errorf("nonexistent claim should have nil children, got %v", children)
	}
}
