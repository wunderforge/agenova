// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package authorization evaluates assignment creation before any authority,
// claim, or runtime side effect is allowed to occur.
package authorization

import (
	"crypto/sha256"
	"fmt"
	"strings"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/policy"
)

const assignmentCreateAction = "claim.create"

// Request is the trusted, backend-neutral context for assignment admission.
// Principal is supplied out-of-band; Action is derived from validated request
// references and does not itself grant authority.
type Request struct {
	RequestRef string
	Principal  v1alpha1.Principal
	Action     v1alpha1.Action
}

// Evaluation binds one evidence-ready decision to the complete request that
// was actually evaluated. Its fields are private so a public Decision cannot
// be replayed as admission proof for a different assignment.
type Evaluation struct {
	request          Request
	policyEvaluation policy.Evaluation
	decision         v1alpha1.Decision
}

// Decision returns the public, evidence-ready result of this evaluation.
func (e Evaluation) Decision() v1alpha1.Decision {
	return e.decision
}

func (e Evaluation) matches(input Request) bool {
	return e.request == input && policy.ContextEqual(e.policyEvaluation.Context, policyContext(input))
}

// Evaluator allows Gate to enforce the pre-side-effect ordering boundary.
type Evaluator interface {
	Evaluate(Request) (Evaluation, error)
}

// Admission is an internal capability proving that Gate admitted one exact
// assignment request. Its binding fields are intentionally private so callers
// cannot manufacture an Allow token from a public Decision value.
type Admission struct {
	evaluation Evaluation
}

// Matches reports whether this admission belongs to the request context that
// a downstream authority resolver is about to process.
func (a Admission) Matches(requestRef, projectRef, templateRef string) bool {
	return a.evaluation.decision.Result == v1alpha1.DecisionResultAllow &&
		a.evaluation.request.RequestRef == requestRef &&
		a.evaluation.request.Action.Name == assignmentCreateAction &&
		a.evaluation.request.Action.Project == projectRef &&
		a.evaluation.request.Action.TemplateRef == templateRef
}

// MatchesContext checks the complete trusted context and decision bound by
// Gate. Issuance uses this instead of accepting a separately supplied public
// Allow decision as proof of admission. It does not reevaluate policy.
func (a Admission) MatchesContext(input Request, decision v1alpha1.Decision) bool {
	return a.evaluation.decision.Result == v1alpha1.DecisionResultAllow &&
		a.evaluation.request == input &&
		a.evaluation.decision == decision
}

// Decision returns the evidence-ready decision that produced this admission.
func (a Admission) Decision() v1alpha1.Decision {
	return a.evaluation.Decision()
}

// Constraints returns a defensive copy of the Policy caps bound to admission.
// The returned value is not a grant and may only narrow downstream authority.
func (a Admission) Constraints() *policy.AuthorityConstraints {
	return policy.CloneEvaluation(a.evaluation.policyEvaluation).Constraints
}

// Authorizer performs exact-match, default-deny assignment admission.
type Authorizer struct {
	Policies policy.SnapshotSource
}

// Evaluate returns one evidence-ready decision. Invalid trusted/request
// context is rejected before policy evaluation; absence or mismatch is Deny.
func (a Authorizer) Evaluate(input Request) (Evaluation, error) {
	if err := validate(input); err != nil {
		return Evaluation{}, err
	}

	context := policyContext(input)
	if a.Policies == nil {
		return completePolicyEvaluation(input, noActivePolicy(context)), nil
	}
	evaluator, ok := a.Policies.Snapshot()
	if !ok {
		return completePolicyEvaluation(input, noActivePolicy(context)), nil
	}
	if evaluator == nil {
		return Evaluation{}, required("evaluator.snapshot")
	}
	evaluation, err := evaluator.Evaluate(context)
	if err != nil {
		return Evaluation{}, err
	}
	return completePolicyEvaluation(input, evaluation), nil
}

// Gate invokes the authorized continuation exactly once only for Allow.
type Gate struct {
	Evaluator Evaluator
}

// Admit evaluates input and stops all downstream work unless the result is
// exactly Allow. ApprovalRequired is intentionally not treated as authority.
func (g Gate) Admit(input Request, onAllowed func(Admission) error) (v1alpha1.Decision, error) {
	if err := validate(input); err != nil {
		return v1alpha1.Decision{}, err
	}
	if g.Evaluator == nil {
		return v1alpha1.Decision{}, required("evaluator")
	}
	evaluation, err := g.Evaluator.Evaluate(input)
	decision := evaluation.Decision()
	if err != nil {
		return decision, err
	}
	if !evaluation.matches(input) {
		return decision, invalid("evaluation.request", "must match the request evaluated for this decision")
	}
	if err := validateDecision(input, decision); err != nil {
		return decision, err
	}
	if err := policy.ValidateEvaluation(policyContext(input), evaluation.policyEvaluation); err != nil {
		return decision, invalid("evaluation", err.Error())
	}
	if decision.Result != v1alpha1.DecisionResultAllow {
		return decision, nil
	}
	if onAllowed == nil {
		return decision, required("onAllowed")
	}
	if err := onAllowed(Admission{evaluation: evaluation}); err != nil {
		return decision, err
	}
	return decision, nil
}

func validateDecision(input Request, decision v1alpha1.Decision) error {
	if strings.TrimSpace(decision.ID) == "" {
		return required("decision.id")
	}
	if decision.PrincipalRef != input.Principal.Subject {
		return invalid("decision.principalRef", "must match the admitted principal")
	}
	if decision.Action != input.Action.Name {
		return invalid("decision.action", "must match the admitted action")
	}
	if decision.Result != v1alpha1.DecisionResultAllow &&
		decision.Result != v1alpha1.DecisionResultDeny &&
		decision.Result != v1alpha1.DecisionResultApprovalRequired {
		return invalid("decision.result", "must be Allow, Deny, or ApprovalRequired")
	}
	policyID := strings.TrimSpace(decision.PolicyRef.ID)
	policyVersion := strings.TrimSpace(decision.PolicyRef.Version)
	if (policyID == "") != (policyVersion == "") {
		return invalid("decision.policyRef", "ID and version must be provided together")
	}
	if decision.Result == v1alpha1.DecisionResultAllow && policyID == "" {
		return required("decision.policyRef")
	}
	if strings.TrimSpace(decision.Reason) == "" {
		return required("decision.reason")
	}
	return nil
}

func completeEvaluation(input Request, decision v1alpha1.Decision) Evaluation {
	decision.ID = decisionID(input, decision)
	policyEvaluation := policy.Evaluation{
		Context:   policyContext(input),
		Decision:  policy.Decision(decision.Result),
		PolicyRef: policy.PolicyReference{ID: decision.PolicyRef.ID, Version: decision.PolicyRef.Version},
		Reasons:   []policy.Reason{{Code: "authorization-compatibility", Message: decision.Reason}},
	}
	return Evaluation{request: input, policyEvaluation: policyEvaluation, decision: decision}
}

func completePolicyEvaluation(input Request, evaluation policy.Evaluation) Evaluation {
	copy := policy.CloneEvaluation(evaluation)
	reason := ""
	if len(copy.Reasons) > 0 {
		reason = copy.Reasons[0].Message
	}
	decision := v1alpha1.Decision{
		PrincipalRef: input.Principal.Subject,
		Action:       input.Action.Name,
		Result:       v1alpha1.DecisionResult(copy.Decision),
		PolicyRef:    v1alpha1.PolicyReference{ID: copy.PolicyRef.ID, Version: copy.PolicyRef.Version},
		Reason:       reason,
	}
	decision.ID = decisionID(input, decision)
	return Evaluation{request: input, policyEvaluation: copy, decision: decision}
}

func noActivePolicy(context policy.EvaluationContext) policy.Evaluation {
	return policy.Evaluation{
		Context:  context,
		Decision: policy.DecisionDeny,
		Reasons: []policy.Reason{{
			Code:    "no-active-policy",
			Message: "no active policy bundle",
		}},
	}
}

func policyContext(input Request) policy.EvaluationContext {
	return policy.EvaluationContext{
		Principal: policy.PrincipalContext{
			Subject:               input.Principal.Subject,
			Team:                  input.Principal.Team,
			AuthenticationContext: input.Principal.AuthenticationContext,
		},
		Action: input.Action.Name,
		Resource: policy.ResourceContext{
			RequestRef:  input.RequestRef,
			Project:     input.Action.Project,
			TemplateRef: input.Action.TemplateRef,
		},
	}
}

func decisionID(input Request, decision v1alpha1.Decision) string {
	parts := []string{
		input.RequestRef,
		input.Principal.Subject,
		input.Principal.Team,
		input.Principal.AuthenticationContext,
		input.Action.Name,
		input.Action.Project,
		input.Action.TemplateRef,
		decision.PolicyRef.ID,
		decision.PolicyRef.Version,
		string(decision.Result),
		decision.Reason,
	}
	var payload strings.Builder
	for _, part := range parts {
		fmt.Fprintf(&payload, "%d:", len(part))
		payload.WriteString(part)
	}
	sum := sha256.Sum256([]byte(payload.String()))
	return fmt.Sprintf("decision:%s:authorization:%x", input.RequestRef, sum[:16])
}

func validate(input Request) error {
	fields := []struct {
		path  string
		value string
	}{
		{"requestRef", input.RequestRef},
		{"principal.subject", input.Principal.Subject},
		{"principal.team", input.Principal.Team},
		{"principal.authenticationContext", input.Principal.AuthenticationContext},
		{"action.name", input.Action.Name},
		{"action.project", input.Action.Project},
		{"action.templateRef", input.Action.TemplateRef},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return required(field.path)
		}
	}
	if input.Action.Name != assignmentCreateAction {
		return invalid("action.name", "assignment admission only accepts claim.create")
	}
	return nil
}

func required(path string) *v1alpha1.ValidationError {
	return &v1alpha1.ValidationError{
		Category:  v1alpha1.ValidationCategoryRequiredField,
		FieldPath: path,
		Detail:    "value is required",
	}
}

func invalid(path, detail string) *v1alpha1.ValidationError {
	return &v1alpha1.ValidationError{
		Category:  v1alpha1.ValidationCategoryInvalidValue,
		FieldPath: path,
		Detail:    detail,
	}
}
