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
	AuthenticationContext string
	Attributes            []Attribute
}

// ResourceContext identifies the backend-neutral object being authorized.
type ResourceContext struct {
	Type       string
	ID         string
	Attributes []Attribute
}

// Attribute carries one trusted, evaluator-neutral named value set. Attribute
// names and their meaning belong to the producer/evaluator composition.
type Attribute struct {
	Name   string
	Values []string
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
	return left.Principal.Subject == right.Principal.Subject &&
		left.Principal.AuthenticationContext == right.Principal.AuthenticationContext &&
		attributesEqual(left.Principal.Attributes, right.Principal.Attributes) &&
		left.Action == right.Action && left.Resource.Type == right.Resource.Type &&
		left.Resource.ID == right.Resource.ID &&
		attributesEqual(left.Resource.Attributes, right.Resource.Attributes) &&
		slices.Equal(left.Environment, right.Environment)
}

// CloneEvaluation returns a deep copy safe to retain as an admission snapshot.
func CloneEvaluation(evaluation Evaluation) Evaluation {
	copy := evaluation
	copy.Context = cloneContext(evaluation.Context)
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
	if policyID == "" || policyVersion == "" {
		return errors.New("evaluation requires a complete policy reference")
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
		{"principal.authenticationContext", context.Principal.AuthenticationContext},
		{"action", context.Action},
		{"resource.type", context.Resource.Type},
		{"resource.id", context.Resource.ID},
	}
	if err := validateAttributes("principal.attributes", context.Principal.Attributes); err != nil {
		return err
	}
	if err := validateAttributes("resource.attributes", context.Resource.Attributes); err != nil {
		return err
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

func validateAttributes(path string, attributes []Attribute) error {
	seen := make(map[string]struct{}, len(attributes))
	for index, attribute := range attributes {
		if strings.TrimSpace(attribute.Name) == "" {
			return fmt.Errorf("evaluation context %s[%d] requires a name", path, index)
		}
		if _, duplicate := seen[attribute.Name]; duplicate {
			return fmt.Errorf("evaluation context %s attribute %q is duplicated", path, attribute.Name)
		}
		seen[attribute.Name] = struct{}{}
		if len(attribute.Values) == 0 {
			return fmt.Errorf("evaluation context %s[%d] requires at least one value", path, index)
		}
		values := make(map[string]struct{}, len(attribute.Values))
		for valueIndex, value := range attribute.Values {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("evaluation context %s[%d].values[%d] is blank", path, index, valueIndex)
			}
			if _, duplicate := values[value]; duplicate {
				return fmt.Errorf("evaluation context %s[%d].values[%d] is duplicated", path, index, valueIndex)
			}
			values[value] = struct{}{}
		}
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
	team, teamOK := singleAttribute(context.Principal.Attributes, "team")
	project, projectOK := singleAttribute(context.Resource.Attributes, "project")
	templateRef, templateOK := singleAttribute(context.Resource.Attributes, "templateRef")
	if teamOK && projectOK && templateOK && r.bundle.Allows(Match{
		Team:        team,
		Action:      context.Action,
		Project:     project,
		TemplateRef: templateRef,
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
	copy.Principal.Attributes = cloneAttributes(context.Principal.Attributes)
	copy.Resource.Attributes = cloneAttributes(context.Resource.Attributes)
	copy.Environment = append([]EnvironmentFact(nil), context.Environment...)
	return copy
}

func cloneAttributes(attributes []Attribute) []Attribute {
	copy := make([]Attribute, len(attributes))
	for index, attribute := range attributes {
		copy[index] = Attribute{Name: attribute.Name, Values: append([]string(nil), attribute.Values...)}
	}
	return copy
}

func attributesEqual(left, right []Attribute) bool {
	return slices.EqualFunc(left, right, func(left, right Attribute) bool {
		return left.Name == right.Name && slices.Equal(left.Values, right.Values)
	})
}

func singleAttribute(attributes []Attribute, name string) (string, bool) {
	for _, attribute := range attributes {
		if attribute.Name == name && len(attribute.Values) == 1 {
			return attribute.Values[0], true
		}
	}
	return "", false
}
