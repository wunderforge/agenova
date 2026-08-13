// Package governance records parent/child claim relationships for multi-agent governance.
// It does not implement DAG orchestration or workflow scheduling; it only tracks claim
// ownership lineage so gateways can enforce scope.
package governance

import (
	"errors"
	"fmt"
	"sync"
)

// Lineage tracks parent/child claim relationships.
// A claim can have at most one parent and any number of direct children.
// It is safe for concurrent use.
type Lineage struct {
	mu       sync.Mutex
	parents  map[string]string   // child -> parent
	children map[string][]string // parent -> children
}

func NewLineage() *Lineage {
	return &Lineage{
		parents:  make(map[string]string),
		children: make(map[string][]string),
	}
}

// RegisterChild records that childClaimID runs under parentClaimID.
// Returns an error if the child is already registered under a different parent,
// or if either ID is empty, or if a claim tries to be its own parent.
func (l *Lineage) RegisterChild(parentClaimID, childClaimID string) error {
	if parentClaimID == "" {
		return errors.New("parent claim id is required")
	}
	if childClaimID == "" {
		return errors.New("child claim id is required")
	}
	if parentClaimID == childClaimID {
		return errors.New("a claim cannot be its own parent")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if existing, ok := l.parents[childClaimID]; ok && existing != parentClaimID {
		return fmt.Errorf("child claim %q already registered under %q", childClaimID, existing)
	}
	l.parents[childClaimID] = parentClaimID
	for _, c := range l.children[parentClaimID] {
		if c == childClaimID {
			return nil // idempotent re-registration
		}
	}
	l.children[parentClaimID] = append(l.children[parentClaimID], childClaimID)
	return nil
}

// Parent returns the parent claim ID for a child claim, or ("", false) if none.
func (l *Lineage) Parent(childClaimID string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	p, ok := l.parents[childClaimID]
	return p, ok
}

// Children returns all direct child claim IDs for a parent claim.
// Returns nil if the claim has no registered children.
func (l *Lineage) Children(parentClaimID string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	src := l.children[parentClaimID]
	if len(src) == 0 {
		return nil
	}
	result := make([]string, len(src))
	copy(result, src)
	return result
}

// IsChildOf reports whether childClaimID is a direct child of parentClaimID.
func (l *Lineage) IsChildOf(parentClaimID, childClaimID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.parents[childClaimID] == parentClaimID
}
