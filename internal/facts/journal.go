// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

// Fact is an immutable observation, not a source of permission. Sequence owns
// ordering; timestamps alone do not order concurrent or same-time events.
type Fact struct {
	ID              string                          `json:"id"`
	Sequence        uint64                          `json:"sequence"`
	Timestamp       time.Time                       `json:"timestamp"`
	Kind            string                          `json:"kind"`
	RequestRef      string                          `json:"requestRef"`
	ClaimID         string                          `json:"claimId,omitempty"`
	InvocationID    string                          `json:"invocationId,omitempty"`
	Decision        *v0.Decision                    `json:"decision,omitempty"`
	Result          v0.DecisionResult               `json:"result,omitempty"`
	ReasonCode      string                          `json:"reasonCode,omitempty"`
	Reason          string                          `json:"reason,omitempty"`
	PolicyRef       *v0.PolicyReference             `json:"policyRef,omitempty"`
	Authority       *v0.EffectiveAuthority          `json:"effectiveAuthority,omitempty"`
	BackendIdentity *v0.SandboxClaimBackendIdentity `json:"backendIdentity,omitempty"`
	Operation       string                          `json:"operation,omitempty"`
	Target          string                          `json:"target,omitempty"`
	ProviderStatus  string                          `json:"providerStatus,omitempty"`
}

// Journal supplies the strict correlation spine for the application path.
// Legacy Store remains compatible; it is not an alternate authority owner.
type Journal struct {
	mu               sync.RWMutex
	requests         map[string]v0.Principal
	claims           map[string]string
	requestClaims    map[string]string
	invocations      map[string]string
	invocationStates map[string]invocationState
	authorities      map[string]string
	backends         map[string]v0.SandboxClaimBackendIdentity
	backendOwners    map[v0.SandboxClaimBackendIdentity]string
	ids              map[string]struct{}
	facts            []Fact
	now              func() time.Time
}

type invocationState struct {
	result               v0.DecisionResult
	attempted, completed bool
}

func NewJournal() *Journal {
	return &Journal{requests: map[string]v0.Principal{}, claims: map[string]string{}, requestClaims: map[string]string{}, invocations: map[string]string{}, invocationStates: map[string]invocationState{}, authorities: map[string]string{}, backends: map[string]v0.SandboxClaimBackendIdentity{}, backendOwners: map[v0.SandboxClaimBackendIdentity]string{}, ids: map[string]struct{}{}, facts: []Fact{}, now: time.Now}
}

func (j *Journal) RegisterRequest(ref string, principal v0.Principal) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if ref == "" || principal.Subject == "" {
		return fmt.Errorf("request reference and trusted principal are required")
	}
	if _, exists := j.requests[ref]; exists {
		return fmt.Errorf("request already registered")
	}
	j.requests[ref] = principal
	return nil
}

func (j *Journal) BindClaim(ref string, claim v0.SandboxClaim) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, ok := j.requests[ref]; !ok {
		return fmt.Errorf("request is not registered")
	}
	if claim.ID == "" || claim.RequestRef != ref {
		return fmt.Errorf("claim/request correlation mismatch")
	}
	if owner, ok := j.claims[claim.ID]; ok {
		if owner == ref && j.requestClaims[ref] == claim.ID {
			return nil
		}
		return fmt.Errorf("claim already belongs to another request")
	}
	if _, ok := j.requestClaims[ref]; ok {
		return fmt.Errorf("request already has a claim")
	}
	j.claims[claim.ID] = ref
	j.requestClaims[ref] = claim.ID
	j.authorities[claim.ID] = claim.AuthorityRef
	return nil
}

// Append assigns identity/time/order and rejects attempts to rewrite or
// cross-attribute facts. Unresolved operations never get a fabricated claim.
func (j *Journal) Append(fact Fact) (Fact, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	principal, ok := j.requests[fact.RequestRef]
	if !ok || fact.Kind == "" {
		return Fact{}, fmt.Errorf("registered request and fact kind are required")
	}
	if fact.ClaimID != "" && j.claims[fact.ClaimID] != fact.RequestRef {
		return Fact{}, fmt.Errorf("claim/request correlation mismatch")
	}
	if fact.Decision != nil {
		d := fact.Decision
		if d.ID == "" || d.PrincipalRef != principal.Subject || d.Action == "" || d.Reason == "" || d.PolicyRef.ID == "" || d.PolicyRef.Version == "" || fact.ReasonCode == "" {
			return Fact{}, fmt.Errorf("decision correlation and stable reason are required")
		}
		if !validDecision(d.Result) {
			return Fact{}, fmt.Errorf("invalid decision result")
		}
		if fact.Result != "" && fact.Result != d.Result {
			return Fact{}, fmt.Errorf("decision result mismatch")
		}
		if d.Result != v0.DecisionResultAllow && fact.ClaimID != "" && fact.InvocationID == "" {
			return Fact{}, fmt.Errorf("pre-claim decision cannot carry a claim")
		}
		if fact.PolicyRef != nil && *fact.PolicyRef != d.PolicyRef {
			return Fact{}, fmt.Errorf("decision policy correlation mismatch")
		}
	}
	if fact.Authority != nil && (fact.ClaimID == "" || fact.Authority.ID != j.authorities[fact.ClaimID]) {
		return Fact{}, fmt.Errorf("authority/claim correlation mismatch")
	}
	if fact.BackendIdentity != nil {
		id := *fact.BackendIdentity
		if fact.ClaimID == "" || id.Backend == "" || id.WorkerID == "" {
			return Fact{}, fmt.Errorf("backend identity requires a resolved claim")
		}
		if previous, exists := j.backends[fact.ClaimID]; exists && previous != id {
			return Fact{}, fmt.Errorf("claim backend identity changed")
		}
		if owner, exists := j.backendOwners[id]; exists && owner != fact.ClaimID {
			return Fact{}, fmt.Errorf("backend identity belongs to another claim")
		}
	}
	if fact.Result != "" && !validDecision(fact.Result) {
		return Fact{}, fmt.Errorf("invalid result")
	}
	if fact.InvocationID != "" {
		if fact.ClaimID == "" {
			return Fact{}, fmt.Errorf("invocation requires a resolved claim")
		}
		if owner, exists := j.invocations[fact.InvocationID]; exists && owner != fact.ClaimID {
			return Fact{}, fmt.Errorf("invocation belongs to another claim")
		}
	}
	stage := j.invocationStates[fact.InvocationID]
	if fact.Kind == "ModelDecision" || fact.Kind == "ToolDecision" {
		if fact.InvocationID == "" || !validDecision(fact.Result) || fact.ReasonCode == "" || fact.PolicyRef == nil || fact.PolicyRef.ID == "" || fact.PolicyRef.Version == "" || stage.result != "" {
			return Fact{}, fmt.Errorf("invocation decision is incomplete or already recorded")
		}
		stage.result = fact.Result
	}
	if fact.Kind == "ProviderAttempt" {
		if fact.InvocationID == "" || stage.result != v0.DecisionResultAllow || stage.attempted {
			return Fact{}, fmt.Errorf("provider attempt requires one recorded Allow")
		}
		stage.attempted = true
	}
	if fact.Kind == "ProviderOutcome" {
		if fact.InvocationID == "" || !stage.attempted || stage.completed || (fact.ProviderStatus != "Succeeded" && fact.ProviderStatus != "Failed" && fact.ProviderStatus != "Cancelled") {
			return Fact{}, fmt.Errorf("provider outcome requires one correlated attempt")
		}
		stage.completed = true
	}
	sequence := uint64(len(j.facts) + 1)
	if fact.ID == "" {
		fact.ID = fmt.Sprintf("fact:%d", sequence)
	}
	if _, exists := j.ids[fact.ID]; exists {
		return Fact{}, fmt.Errorf("fact ID already exists")
	}
	fact.Sequence = sequence
	if fact.Timestamp.IsZero() {
		fact.Timestamp = j.now().UTC()
	}
	copy, err := cloneFact(fact)
	if err != nil {
		return Fact{}, err
	}
	j.ids[copy.ID] = struct{}{}
	if copy.InvocationID != "" {
		j.invocations[copy.InvocationID] = copy.ClaimID
		j.invocationStates[copy.InvocationID] = stage
	}
	if copy.BackendIdentity != nil {
		j.backends[copy.ClaimID] = *copy.BackendIdentity
		j.backendOwners[*copy.BackendIdentity] = copy.ClaimID
	}
	j.facts = append(j.facts, copy)
	return cloneFact(copy)
}

func (j *Journal) ForRequest(ref string) []Fact {
	j.mu.RLock()
	defer j.mu.RUnlock()
	result := []Fact{}
	for _, f := range j.facts {
		if f.RequestRef == ref {
			copy, _ := cloneFact(f)
			result = append(result, copy)
		}
	}
	return result
}

func (j *Journal) ForClaim(id string) []Fact {
	j.mu.RLock()
	defer j.mu.RUnlock()
	result := []Fact{}
	for _, f := range j.facts {
		if f.ClaimID == id {
			copy, _ := cloneFact(f)
			result = append(result, copy)
		}
	}
	return result
}

func validDecision(value v0.DecisionResult) bool {
	return value == v0.DecisionResultAllow || value == v0.DecisionResultDeny || value == v0.DecisionResultApprovalRequired
}
func cloneFact(value Fact) (Fact, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return Fact{}, fmt.Errorf("fact is not serializable")
	}
	var copy Fact
	if err = json.Unmarshal(data, &copy); err != nil {
		return Fact{}, fmt.Errorf("fact could not be copied")
	}
	return copy, nil
}
