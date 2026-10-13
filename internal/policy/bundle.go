// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package policy provides the static, backend-neutral reference policy bundle.
package policy

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// PolicyBundle is one immutable version of the reference control-plane policy.
// Rules are exact matches; absence of a matching rule denies access.
type PolicyBundle struct {
	ID      string
	Version string
	Rules   []Rule
}

// Rule grants one action for one trusted team, project, and agent template.
type Rule struct {
	Subject     string                `json:"subject,omitempty" yaml:"subject,omitempty"`
	Team        string                `json:"team" yaml:"team"`
	Action      string                `json:"action" yaml:"action"`
	Project     string                `json:"project" yaml:"project"`
	TemplateRef string                `json:"templateRef" yaml:"templateRef"`
	Constraints *AuthorityConstraints `json:"constraints,omitempty" yaml:"constraints,omitempty"`
}

// Match is the trusted assignment context evaluated against a policy bundle.
type Match struct {
	Subject     string
	Team        string
	Action      string
	Project     string
	TemplateRef string
}

type bundleIdentity struct {
	id      string
	version string
}

// ReferenceBundle is the versioned MVP bootstrap baseline. Its one explicit
// allow rule leaves every other trusted assignment denied by default.
func ReferenceBundle() PolicyBundle {
	return PolicyBundle{
		ID: "reference-default-deny", Version: "1",
		Rules: []Rule{{Team: "team-a", Action: "claim.create", Project: "payments", TemplateRef: "engineer"}},
	}
}

// Loader owns the last successfully loaded policy bundle.
type Loader struct {
	mu      sync.RWMutex
	current *PolicyBundle
	seen    map[bundleIdentity][]Rule
}

// Load validates the complete bundle before replacing the current version.
func (l *Loader) Load(bundle PolicyBundle) error {
	if err := validate(bundle); err != nil {
		return err
	}

	copy := clone(bundle)
	l.mu.Lock()
	defer l.mu.Unlock()
	identity := bundleIdentity{id: bundle.ID, version: bundle.Version}
	if rules, ok := l.seen[identity]; ok {
		if !reflect.DeepEqual(rules, bundle.Rules) {
			return fmt.Errorf("policy bundle %s@%s cannot change content", bundle.ID, bundle.Version)
		}
	} else {
		if l.seen == nil {
			l.seen = make(map[bundleIdentity][]Rule)
		}
		l.seen[identity] = clone(bundle).Rules
	}
	l.current = &copy
	return nil
}

// Current returns a copy of the last successfully loaded bundle.
func (l *Loader) Current() (PolicyBundle, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.current == nil {
		return PolicyBundle{}, false
	}

	return clone(*l.current), true
}

// Snapshot returns an evaluator bound to the currently loaded immutable bundle.
// Later Loader updates cannot change the returned evaluator's decisions.
func (l *Loader) Snapshot() (Evaluator, bool) {
	bundle, ok := l.Current()
	if !ok {
		return nil, false
	}
	evaluator, err := NewReferenceEvaluator(bundle)
	if err != nil {
		return nil, false
	}
	return evaluator, true
}

// Allows reports whether an exact trusted team, action, project, and template rule exists.
func (b PolicyBundle) Allows(match Match) bool {
	if strings.TrimSpace(match.Team) == "" || strings.TrimSpace(match.Action) == "" || strings.TrimSpace(match.Project) == "" || strings.TrimSpace(match.TemplateRef) == "" {
		return false
	}

	for _, rule := range b.Rules {
		if (rule.Subject == "" || rule.Subject == match.Subject) && rule.Team == match.Team && rule.Action == match.Action && rule.Project == match.Project && rule.TemplateRef == match.TemplateRef {
			return true
		}
	}
	return false
}

func validate(bundle PolicyBundle) error {
	if strings.TrimSpace(bundle.ID) == "" {
		return errors.New("policy bundle ID is required")
	}
	if strings.TrimSpace(bundle.Version) == "" {
		return errors.New("policy bundle version is required")
	}
	// These identifiers are repeated in issued state and multiple evidence
	// facts. Bound their encoded size before an operator can register them.
	if len(bundle.ID) > 128 || len(bundle.Version) > 64 {
		return errors.New("policy bundle ID or version exceeds the evidence-safe limit")
	}

	type ruleKey struct{ subject, team, action, project, template string }
	seen := make(map[ruleKey]int, len(bundle.Rules))
	for index, rule := range bundle.Rules {
		if strings.TrimSpace(rule.Team) == "" || strings.TrimSpace(rule.Action) == "" || strings.TrimSpace(rule.Project) == "" || strings.TrimSpace(rule.TemplateRef) == "" {
			return fmt.Errorf("policy rule %d requires team, action, project, and templateRef", index)
		}
		if err := validateConstraints(rule.Constraints); err != nil {
			return fmt.Errorf("policy rule %d: %w", index, err)
		}
		key := ruleKey{rule.Subject, rule.Team, rule.Action, rule.Project, rule.TemplateRef}
		if first, ok := seen[key]; ok {
			return fmt.Errorf("policy rule %d duplicates rule %d", index, first)
		}
		seen[key] = index
	}
	return nil
}

// ValidateBundle is shared by operator registration and the in-process
// reference loader; it performs no write or activation.
func ValidateBundle(bundle PolicyBundle) error { return validate(bundle) }

func clone(bundle PolicyBundle) PolicyBundle {
	copy := bundle
	if bundle.Rules != nil {
		copy.Rules = append([]Rule{}, bundle.Rules...)
	}
	for index := range copy.Rules {
		copy.Rules[index].Constraints = cloneConstraints(bundle.Rules[index].Constraints)
	}
	return copy
}
