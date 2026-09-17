// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"encoding/json"
	"fmt"
	"sync"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/policy"
)

// MemoryStore is the contract-test oracle, not connected installation state.
type MemoryStore struct {
	mu        sync.RWMutex
	policies  map[PolicyReference]policy.PolicyBundle
	active    PolicyReference
	templates map[string]*v0.AgentTemplate
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{policies: map[PolicyReference]policy.PolicyBundle{}, templates: map[string]*v0.AgentTemplate{}}
}

func (s *MemoryStore) PutPolicy(bundle policy.PolicyBundle) (bool, error) {
	if err := policy.ValidateBundle(bundle); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ref := PolicyReference{ID: bundle.ID, Version: bundle.Version}
	if existing, ok := s.policies[ref]; ok {
		if !EqualJSON(existing, bundle) {
			return false, ErrConflict
		}
		return false, nil
	}
	s.policies[ref] = cloneBundle(bundle)
	return true, nil
}

func (s *MemoryStore) ActivatePolicy(ref PolicyReference) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.policies[ref]; !ok {
		return fmt.Errorf("PolicyBundle %s@%s is not registered", ref.ID, ref.Version)
	}
	s.active = ref
	return nil
}

func (s *MemoryStore) ActivePolicy() (policy.PolicyBundle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bundle, ok := s.policies[s.active]
	if !ok {
		return policy.PolicyBundle{}, fmt.Errorf("no active PolicyBundle")
	}
	return cloneBundle(bundle), nil
}

func (s *MemoryStore) PutTemplate(template *v0.AgentTemplate) (bool, error) {
	if err := v0.ValidateAgentTemplate(template); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.templates[template.Metadata.Name]; ok {
		if !EqualJSON(existing, template) {
			return false, ErrConflict
		}
		return false, nil
	}
	s.templates[template.Metadata.Name] = cloneTemplate(template)
	return true, nil
}

func (s *MemoryStore) Template(name string) (*v0.AgentTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	template, ok := s.templates[name]
	if !ok {
		return nil, fmt.Errorf("AgentTemplate %q is not registered", name)
	}
	return cloneTemplate(template), nil
}

func cloneBundle(bundle policy.PolicyBundle) policy.PolicyBundle {
	bundle.Rules = append([]policy.Rule(nil), bundle.Rules...)
	return bundle
}

func cloneTemplate(template *v0.AgentTemplate) *v0.AgentTemplate {
	data, _ := json.Marshal(template)
	var result v0.AgentTemplate
	_ = json.Unmarshal(data, &result)
	return &result
}
