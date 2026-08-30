// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package policy provides the static, backend-neutral reference policy bundle.
package policy

import (
	"errors"
	"fmt"
	"slices"
	"sync"
)

// PolicyBundle is one immutable version of the reference control-plane policy.
// Rules are exact matches; absence of a matching rule denies access.
type PolicyBundle struct {
	ID      string
	Version string
	Rules   []Rule
}

// Rule grants one action for one project and agent template.
type Rule struct {
	Action      string
	Project     string
	TemplateRef string
}

// Loader owns the last successfully loaded policy bundle.
type Loader struct {
	mu      sync.RWMutex
	current *PolicyBundle
}

// Load validates the complete bundle before replacing the current version.
func (l *Loader) Load(bundle PolicyBundle) error {
	if err := validate(bundle); err != nil {
		return err
	}

	copy := clone(bundle)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.current != nil && l.current.ID == bundle.ID && l.current.Version == bundle.Version {
		if !slices.Equal(l.current.Rules, bundle.Rules) {
			return fmt.Errorf("policy bundle %s@%s cannot change content", bundle.ID, bundle.Version)
		}
		return nil
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

// Allows reports whether an exact action, project, and template rule exists.
func (b PolicyBundle) Allows(action, project, templateRef string) bool {
	if action == "" || project == "" || templateRef == "" {
		return false
	}

	for _, rule := range b.Rules {
		if rule.Action == action && rule.Project == project && rule.TemplateRef == templateRef {
			return true
		}
	}
	return false
}

func validate(bundle PolicyBundle) error {
	if bundle.ID == "" {
		return errors.New("policy bundle ID is required")
	}
	if bundle.Version == "" {
		return errors.New("policy bundle version is required")
	}

	seen := make(map[Rule]int, len(bundle.Rules))
	for index, rule := range bundle.Rules {
		if rule.Action == "" || rule.Project == "" || rule.TemplateRef == "" {
			return fmt.Errorf("policy rule %d requires action, project, and templateRef", index)
		}
		if first, ok := seen[rule]; ok {
			return fmt.Errorf("policy rule %d duplicates rule %d", index, first)
		}
		seen[rule] = index
	}
	return nil
}

func clone(bundle PolicyBundle) PolicyBundle {
	copy := bundle
	copy.Rules = append([]Rule(nil), bundle.Rules...)
	return copy
}
