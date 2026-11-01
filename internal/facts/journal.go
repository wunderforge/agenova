// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authority"
)

// Fact is an immutable observation, not a source of permission. Sequence owns
// ordering; timestamps alone do not order concurrent or same-time events.
type Fact struct {
	Turn             int                             `json:"turn,omitempty"`
	ActionRef        string                          `json:"actionRef,omitempty"`
	ModelCallCount   int                             `json:"modelCallCount,omitempty"`
	ModelCallLimit   int                             `json:"modelCallLimit,omitempty"`
	ActionKind       string                          `json:"actionKind,omitempty"`
	ActionTool       string                          `json:"actionTool,omitempty"`
	ActionResource   string                          `json:"actionResource,omitempty"`
	ContentRef       string                          `json:"contentRef,omitempty"`
	ID               string                          `json:"id"`
	Sequence         uint64                          `json:"sequence"`
	Timestamp        time.Time                       `json:"timestamp"`
	Kind             string                          `json:"kind"`
	RequestRef       string                          `json:"requestRef"`
	ClaimID          string                          `json:"claimId,omitempty"`
	InvocationID     string                          `json:"invocationId,omitempty"`
	Decision         *v0.Decision                    `json:"decision,omitempty"`
	Result           v0.DecisionResult               `json:"result,omitempty"`
	ReasonCode       string                          `json:"reasonCode,omitempty"`
	Reason           string                          `json:"reason,omitempty"`
	PolicyRef        *v0.PolicyReference             `json:"policyRef,omitempty"`
	Authority        *v0.EffectiveAuthority          `json:"effectiveAuthority,omitempty"`
	AuthorityChanges []authority.Change              `json:"authorityChanges,omitempty"`
	BackendIdentity  *v0.SandboxClaimBackendIdentity `json:"backendIdentity,omitempty"`
	Operation        string                          `json:"operation,omitempty"`
	Target           string                          `json:"target,omitempty"`
	ProviderStatus   string                          `json:"providerStatus,omitempty"`
	// ResultRef names the external result of a successful tool call. It is
	// separate from Target, which stays equal across attempt and outcome.
	ResultRef string `json:"resultRef,omitempty"`
	// Truncated marks a successful tool outcome whose observation was cut to
	// the configured budget. Absence is not proof that an observation was
	// complete: facts recorded before this field existed never carry it.
	Truncated bool `json:"truncated,omitempty"`
}

// ValidResultRef accepts a bounded logical reference, never a fetchable URL.
// A reference may start with a scope kind such as "repo:", but not with "//",
// a URI scheme followed by "/" (file:/etc/passwd) or a scheme that clients
// dereference without a slash (data:text/plain,..., ws:host/x).
func ValidResultRef(ref string) bool {
	if ref == "" || len(ref) > 256 || !utf8.ValidString(ref) || strings.ContainsAny(ref, " \t\r\n\x00?#@") || strings.Contains(ref, "://") || strings.HasPrefix(ref, "//") {
		return false
	}
	scheme, rest, found := strings.Cut(ref, ":")
	return !found || !uriScheme(scheme) || (!strings.HasPrefix(rest, "/") && !fetchableSchemes[strings.ToLower(scheme)])
}

// The WHATWG special schemes plus data and javascript.
var fetchableSchemes = map[string]bool{"data": true, "file": true, "ftp": true, "http": true, "https": true, "javascript": true, "ws": true, "wss": true}

// uriScheme matches RFC 3986 scheme syntax: ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ).
func uriScheme(s string) bool {
	for i, c := range s {
		letter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if !letter && (i == 0 || !(c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.')) {
			return false
		}
	}
	return s != ""
}

// Journal supplies the strict correlation spine for the application path.
// Legacy Store remains compatible; it is not an alternate authority owner.
type Journal struct {
	budgetMu         sync.Mutex
	startSequence    uint64
	commit           func(Fact) error
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

// NewDurableJournal starts after the persisted global sequence. The commit
// callback runs under the journal lock, after validation and before publication;
// failure prevents the fact from authorizing a later external side effect.
func NewDurableJournal(lastSequence uint64, commit func(Fact) error) (*Journal, error) {
	if commit == nil {
		return nil, fmt.Errorf("durable fact commit is required")
	}
	j := NewJournal()
	j.startSequence = lastSequence
	j.commit = commit
	return j, nil
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
	return j.append(fact, nil)
}

// AppendWithCommit lets the lifecycle owner commit a snapshot together with its
// fact. The override has the same validation/failure boundary as the default sink.
// Neither commit callback may call back into this journal.
func (j *Journal) AppendWithCommit(fact Fact, commit func(Fact) error) (Fact, error) {
	if commit == nil {
		return Fact{}, fmt.Errorf("atomic fact commit is required")
	}
	return j.append(fact, commit)
}
func (j *Journal) append(fact Fact, commit func(Fact) error) (Fact, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if fact.ContentRef != "" {
		if len(fact.ContentRef) != 40 || !strings.HasPrefix(fact.ContentRef, "content:") || strings.Trim(fact.ContentRef[8:], "0123456789abcdef") != "" {
			return Fact{}, fmt.Errorf("invalid activity content reference")
		}
	}
	principal, ok := j.requests[fact.RequestRef]
	if !ok || fact.Kind == "" {
		return Fact{}, fmt.Errorf("registered request and fact kind are required")
	}
	if fact.ClaimID != "" && j.claims[fact.ClaimID] != fact.RequestRef {
		return Fact{}, fmt.Errorf("claim/request correlation mismatch")
	}
	if fact.Turn < 0 || fact.Turn > 1000 {
		return Fact{}, fmt.Errorf("invalid action turn")
	}
	if fact.ActionKind != "" || fact.ActionTool != "" || fact.ActionResource != "" {
		if fact.Kind != "WorkerActivity" || fact.Operation != "ActionValidated" || fact.Turn < 1 || !strings.HasPrefix(fact.ReasonCode, "agent-action-") {
			return Fact{}, fmt.Errorf("public action requires a validated model action")
		}
		if fact.ActionKind != "tool" && fact.ActionKind != "finish" && fact.ActionKind != "native_https" {
			return Fact{}, fmt.Errorf("invalid public action")
		}
		if len(fact.ActionTool) > 128 || len(fact.ActionResource) > 256 || strings.ContainsAny(fact.ActionTool+fact.ActionResource, "\r\n\x00") {
			return Fact{}, fmt.Errorf("unbounded public action")
		}
		if fact.ActionKind == "tool" && (fact.ActionTool == "" || fact.ActionResource == "") {
			return Fact{}, fmt.Errorf("tool action requires bounded target")
		}
		if fact.ActionKind != "tool" && (fact.ActionTool != "" || fact.ActionResource != "") {
			return Fact{}, fmt.Errorf("non-tool action cannot declare tool target")
		}
	}
	if fact.ActionRef != "" {
		matched := false
		for _, prior := range j.facts {
			if prior.ID == fact.ActionRef && prior.RequestRef == fact.RequestRef && prior.ClaimID == fact.ClaimID && prior.Operation == "ActionValidated" && prior.ActionKind == "tool" && prior.ActionTool == fact.Target && prior.Turn == fact.Turn {
				matched = true
				break
			}
		}
		if fact.Kind != "ToolDecision" || !matched {
			return Fact{}, fmt.Errorf("tool decision/action correlation mismatch")
		}
	}
	if fact.Kind == "ModelBudgetReservation" {
		if fact.ClaimID == "" || fact.InvocationID == "" || fact.Operation != "model.invoke" || fact.ReasonCode != "model-call-reserved" || fact.ModelCallLimit < 1 || fact.ModelCallLimit > 1000 || fact.ModelCallCount < 1 || fact.ModelCallCount > fact.ModelCallLimit {
			return Fact{}, fmt.Errorf("invalid budget fact")
		}
		count := 0
		for _, prior := range j.facts {
			if prior.Kind == "ModelBudgetReservation" && prior.RequestRef == fact.RequestRef {
				count++
				if prior.InvocationID == fact.InvocationID || prior.ModelCallLimit != fact.ModelCallLimit {
					return Fact{}, fmt.Errorf("duplicate or changed budget fact")
				}
			}
		}
		if fact.ModelCallCount != count+1 {
			return Fact{}, fmt.Errorf("budget count is not monotonic")
		}
	} else if fact.ModelCallCount != 0 || fact.ModelCallLimit != 0 {
		return Fact{}, fmt.Errorf("budget fields require reservation")
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
	if fact.ResultRef != "" && (fact.Kind != "ProviderOutcome" || fact.Operation != "tool.invoke" || fact.ProviderStatus != "Succeeded" || !ValidResultRef(fact.ResultRef)) {
		return Fact{}, fmt.Errorf("result reference is only valid on a successful tool outcome")
	}
	if fact.Truncated && (fact.Kind != "ProviderOutcome" || fact.Operation != "tool.invoke" || fact.ProviderStatus != "Succeeded") {
		return Fact{}, fmt.Errorf("truncation is only valid on a successful tool outcome")
	}
	if fact.Kind == "ProviderOutcome" {
		if fact.InvocationID == "" || !stage.attempted || stage.completed || (fact.ProviderStatus != "Succeeded" && fact.ProviderStatus != "Failed" && fact.ProviderStatus != "Cancelled") {
			return Fact{}, fmt.Errorf("provider outcome requires one correlated attempt")
		}
		stage.completed = true
	}
	sequence := j.startSequence + uint64(len(j.facts)) + 1
	if sequence <= j.startSequence {
		return Fact{}, fmt.Errorf("fact sequence exhausted")
	}
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
	if commit == nil {
		commit = j.commit
	}
	if commit != nil {
		committed, err := cloneFact(copy)
		if err != nil {
			return Fact{}, err
		}
		if err := commit(committed); err != nil {
			return Fact{}, err
		}
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
