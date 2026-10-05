// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Decision is the engine-neutral outcome returned by a Policy evaluator.
type Decision string

const (
	DecisionAllow            Decision = "Allow"
	DecisionDeny             Decision = "Deny"
	DecisionApprovalRequired Decision = "ApprovalRequired"
)

// PrincipalContext is the trusted identity projection made available to Policy.
type PrincipalContext struct {
	Subject               string
	Team                  string
	AuthenticationContext string
}

// ResourceContext identifies the backend-neutral object being authorized.
type ResourceContext struct {
	RequestRef  string
	Project     string
	TemplateRef string
}

// EnvironmentFact is one bounded, trusted input supplied by the application.
// Evaluators must not interpret caller-authored metadata as an environment fact.
type EnvironmentFact struct {
	Name  string
	Value string
}

// EvaluationContext is the complete trusted input to a Policy decision.
type EvaluationContext struct {
	Principal   PrincipalContext
	Action      string
	Resource    ResourceContext
	Environment []EnvironmentFact
}

// PolicyReference identifies the immutable Policy snapshot that decided.
type PolicyReference struct {
	ID      string
	Version string
}

// Reason is stable machine-readable decision provenance plus safe explanation.
type Reason struct {
	Code    string
	Message string
}

// StringSetConstraint distinguishes an explicitly empty cap from an absent cap.
type StringSetConstraint struct {
	Values []string
}

// AuthorityConstraints are optional caps carried with an evaluation. They do
// not grant authority; consumers may only intersect them with other ceilings.
type AuthorityConstraints struct {
	Tools           *StringSetConstraint
	ResourceScopes  *StringSetConstraint
	ModelProfiles   *StringSetConstraint
	MemoryScopes    *StringSetConstraint
	RuntimeProfiles *StringSetConstraint
	MaxTimeout      *time.Duration
}

// Evaluation is an evaluator result bound to the exact context it evaluated.
// Gateways and public contracts do not consume this type directly.
type Evaluation struct {
	Context     EvaluationContext
	Decision    Decision
	PolicyRef   PolicyReference
	Reasons     []Reason
	Constraints *AuthorityConstraints
}

// Evaluator is the replaceable backend-neutral Policy boundary.
type Evaluator interface {
	Evaluate(EvaluationContext) (Evaluation, error)
}

// SnapshotSource returns one immutable evaluator snapshot for an admission.
type SnapshotSource interface {
	Snapshot() (Evaluator, bool)
}

// ContextEqual compares the complete trusted input, including ordered facts.
func ContextEqual(left, right EvaluationContext) bool {
	return left.Principal == right.Principal && left.Action == right.Action &&
		left.Resource == right.Resource && slices.Equal(left.Environment, right.Environment)
}

// CloneEvaluation returns a deep copy safe to retain as an admission snapshot.
func CloneEvaluation(evaluation Evaluation) Evaluation {
	copy := evaluation
	copy.Context.Environment = append([]EnvironmentFact(nil), evaluation.Context.Environment...)
	copy.Reasons = append([]Reason(nil), evaluation.Reasons...)
	copy.Constraints = cloneConstraints(evaluation.Constraints)
	return copy
}

// ValidateEvaluation rejects malformed or context-replayed evaluator output.
func ValidateEvaluation(expected EvaluationContext, evaluation Evaluation) error {
	if err := ValidateContext(expected); err != nil {
		return err
	}
	if !ContextEqual(expected, evaluation.Context) {
		return errors.New("evaluation context does not match the trusted input")
	}
	if evaluation.Decision != DecisionAllow && evaluation.Decision != DecisionDeny && evaluation.Decision != DecisionApprovalRequired {
		return fmt.Errorf("invalid evaluation decision %q", evaluation.Decision)
	}
	policyID := strings.TrimSpace(evaluation.PolicyRef.ID)
	policyVersion := strings.TrimSpace(evaluation.PolicyRef.Version)
	if (policyID == "") != (policyVersion == "") {
		return errors.New("evaluation policy reference requires both ID and version")
	}
	if evaluation.Decision == DecisionAllow && policyID == "" {
		return errors.New("allowed evaluation requires a policy reference")
	}
	if len(evaluation.Reasons) == 0 {
		return errors.New("evaluation requires at least one reason")
	}
	for index, reason := range evaluation.Reasons {
		if strings.TrimSpace(reason.Code) == "" || strings.TrimSpace(reason.Message) == "" {
			return fmt.Errorf("evaluation reason %d requires code and message", index)
		}
	}
	return validateConstraints(evaluation.Constraints)
}

// ValidateContext validates trusted context before an evaluator is called.
func ValidateContext(context EvaluationContext) error {
	fields := []struct {
		name  string
		value string
	}{
		{"principal.subject", context.Principal.Subject},
		{"principal.team", context.Principal.Team},
		{"principal.authenticationContext", context.Principal.AuthenticationContext},
		{"action", context.Action},
		{"resource.requestRef", context.Resource.RequestRef},
		{"resource.project", context.Resource.Project},
		{"resource.templateRef", context.Resource.TemplateRef},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("evaluation context %s is required", field.name)
		}
	}
	seen := make(map[string]struct{}, len(context.Environment))
	for index, fact := range context.Environment {
		if strings.TrimSpace(fact.Name) == "" || strings.TrimSpace(fact.Value) == "" {
			return fmt.Errorf("evaluation context environment fact %d requires name and value", index)
		}
		if _, duplicate := seen[fact.Name]; duplicate {
			return fmt.Errorf("evaluation context environment fact %q is duplicated", fact.Name)
		}
		seen[fact.Name] = struct{}{}
	}
	return nil
}

func validateConstraints(constraints *AuthorityConstraints) error {
	if constraints == nil {
		return nil
	}
	sets := []struct {
		name string
		set  *StringSetConstraint
	}{
		{"tools", constraints.Tools},
		{"resourceScopes", constraints.ResourceScopes},
		{"modelProfiles", constraints.ModelProfiles},
		{"memoryScopes", constraints.MemoryScopes},
		{"runtimeProfiles", constraints.RuntimeProfiles},
	}
	for _, item := range sets {
		if item.set == nil {
			continue
		}
		seen := make(map[string]struct{}, len(item.set.Values))
		for index, value := range item.set.Values {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("authority constraint %s[%d] is blank", item.name, index)
			}
			if _, duplicate := seen[value]; duplicate {
				return fmt.Errorf("authority constraint %s[%d] is duplicated", item.name, index)
			}
			seen[value] = struct{}{}
		}
	}
	if constraints.MaxTimeout != nil && *constraints.MaxTimeout <= 0 {
		return errors.New("authority constraint maxTimeout must be positive")
	}
	return nil
}

func cloneConstraints(constraints *AuthorityConstraints) *AuthorityConstraints {
	if constraints == nil {
		return nil
	}
	copy := *constraints
	copy.Tools = cloneSet(constraints.Tools)
	copy.ResourceScopes = cloneSet(constraints.ResourceScopes)
	copy.ModelProfiles = cloneSet(constraints.ModelProfiles)
	copy.MemoryScopes = cloneSet(constraints.MemoryScopes)
	copy.RuntimeProfiles = cloneSet(constraints.RuntimeProfiles)
	if constraints.MaxTimeout != nil {
		value := *constraints.MaxTimeout
		copy.MaxTimeout = &value
	}
	return &copy
}

func cloneSet(set *StringSetConstraint) *StringSetConstraint {
	if set == nil {
		return nil
	}
	return &StringSetConstraint{Values: append([]string(nil), set.Values...)}
}

type referenceEvaluator struct {
	bundle PolicyBundle
}

// NewReferenceEvaluator snapshots one validated exact-match PolicyBundle.
func NewReferenceEvaluator(bundle PolicyBundle) (Evaluator, error) {
	if err := validate(bundle); err != nil {
		return nil, err
	}
	return referenceEvaluator{bundle: clone(bundle)}, nil
}

func (r referenceEvaluator) Evaluate(context EvaluationContext) (Evaluation, error) {
	if err := ValidateContext(context); err != nil {
		return Evaluation{}, err
	}
	evaluation := Evaluation{
		Context:   cloneContext(context),
		Decision:  DecisionDeny,
		PolicyRef: PolicyReference{ID: r.bundle.ID, Version: r.bundle.Version},
		Reasons: []Reason{{
			Code:    "reference-no-exact-match",
			Message: "no exact policy rule matched the trusted principal and requested assignment",
		}},
	}
	if r.bundle.Allows(Match{
		Team:        context.Principal.Team,
		Action:      context.Action,
		Project:     context.Resource.Project,
		TemplateRef: context.Resource.TemplateRef,
	}) {
		evaluation.Decision = DecisionAllow
		evaluation.Reasons = []Reason{{
			Code:    "reference-exact-match",
			Message: "exact policy rule matched the trusted principal and requested assignment",
		}}
	}
	return evaluation, nil
}

func cloneContext(context EvaluationContext) EvaluationContext {
	copy := context
	copy.Environment = append([]EnvironmentFact(nil), context.Environment...)
	return copy
}
