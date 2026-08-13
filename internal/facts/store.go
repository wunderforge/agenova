// Package facts provides an in-memory append-only store for claim-scoped
// governance facts. ToolInvocation and ModelInvocation are emitted by the
// gateway MVPs. RuntimeEvent is reserved for runtime lifecycle events; gateways
// do not emit it.
package facts

import (
	"sync"
	"time"

	"github.com/donozhang1992/agenova/api/v1alpha1"
)

// Store is an in-memory append-only fact store. It is safe for concurrent use.
type Store struct {
	mu               sync.Mutex
	toolInvocations  []v1alpha1.ToolInvocation
	modelInvocations []v1alpha1.ModelInvocation
	runtimeEvents    []v1alpha1.RuntimeEvent
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) RecordToolInvocation(claimID, toolName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolInvocations = append(s.toolInvocations, v1alpha1.ToolInvocation{
		ClaimID:   claimID,
		ToolName:  toolName,
		Timestamp: time.Now(),
	})
}

func (s *Store) RecordModelInvocation(claimID, modelName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modelInvocations = append(s.modelInvocations, v1alpha1.ModelInvocation{
		ClaimID:   claimID,
		ModelName: modelName,
		Timestamp: time.Now(),
	})
}

func (s *Store) RecordRuntimeEvent(claimID, kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtimeEvents = append(s.runtimeEvents, v1alpha1.RuntimeEvent{
		ClaimID:   claimID,
		Kind:      kind,
		Timestamp: time.Now(),
	})
}

// ToolInvocations returns all ToolInvocation facts recorded for the given claim.
func (s *Store) ToolInvocations(claimID string) []v1alpha1.ToolInvocation {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []v1alpha1.ToolInvocation
	for _, inv := range s.toolInvocations {
		if inv.ClaimID == claimID {
			result = append(result, inv)
		}
	}
	return result
}

// ModelInvocations returns all ModelInvocation facts recorded for the given claim.
func (s *Store) ModelInvocations(claimID string) []v1alpha1.ModelInvocation {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []v1alpha1.ModelInvocation
	for _, inv := range s.modelInvocations {
		if inv.ClaimID == claimID {
			result = append(result, inv)
		}
	}
	return result
}

// RuntimeEvents returns all RuntimeEvent facts recorded for the given claim.
func (s *Store) RuntimeEvents(claimID string) []v1alpha1.RuntimeEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []v1alpha1.RuntimeEvent
	for _, ev := range s.runtimeEvents {
		if ev.ClaimID == claimID {
			result = append(result, ev)
		}
	}
	return result
}
