// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package registration owns the narrow, immutable operator registration
// contract. Backends decide where records live and how writes are authorized.
package registration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/policy"
)

var ErrConflict = errors.New("registered identity has different content")

type PolicyReference struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type Result struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Changed bool   `json:"changed"`
	Active  bool   `json:"active,omitempty"`
}

// Store must implement atomic create-or-equal semantics. Implementations may
// return ErrConflict for the same identity with different canonical content.
type Store interface {
	CanActivatePolicy(PolicyReference) error
	PutPolicy(policy.PolicyBundle) (changed bool, err error)
	ActivatePolicy(PolicyReference) error
	ActivePolicy() (policy.PolicyBundle, error)
	PutTemplate(*v0.AgentTemplate) (changed bool, err error)
	Template(name string) (*v0.AgentTemplate, error)
}

type Service struct{ Store Store }

func (s Service) ApplyPolicyFile(path string) (Result, error) {
	data, err := readDocument(path)
	if err != nil {
		return Result{}, fmt.Errorf("read PolicyBundle file: %w", err)
	}
	return s.ApplyPolicy(data)
}

func (s Service) ApplyPolicy(data []byte) (Result, error) {
	if s.Store == nil {
		return Result{}, errors.New("registration store is unavailable")
	}
	bundle, err := policy.ParseDocumentYAML(data)
	if err != nil {
		return Result{}, err
	}
	ref := PolicyReference{ID: bundle.ID, Version: bundle.Version}
	if err := s.Store.CanActivatePolicy(ref); err != nil {
		return Result{}, fmt.Errorf("PolicyBundle activation is not authorized: %w", err)
	}
	changed, err := s.Store.PutPolicy(bundle)
	if err != nil {
		return Result{}, err
	}
	if err := s.Store.ActivatePolicy(ref); err != nil {
		return Result{}, fmt.Errorf("PolicyBundle registered but not activated: %w", err)
	}
	return Result{Kind: policy.DocumentKind, Name: bundle.ID, Version: bundle.Version, Changed: changed, Active: true}, nil
}

func (s Service) ApplyTemplateFile(path string) (Result, error) {
	data, err := readDocument(path)
	if err != nil {
		return Result{}, fmt.Errorf("read AgentTemplate file: %w", err)
	}
	return s.ApplyTemplate(data)
}

func readDocument(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	const maxDocument = 1 << 20
	data, err := io.ReadAll(io.LimitReader(file, maxDocument+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDocument {
		return nil, fmt.Errorf("registration document exceeds 1 MiB")
	}
	return data, nil
}

func (s Service) ApplyTemplate(data []byte) (Result, error) {
	if s.Store == nil {
		return Result{}, errors.New("registration store is unavailable")
	}
	template, validationErr := v0.ParseAgentTemplateYAML(data)
	if validationErr != nil {
		return Result{}, validationErr
	}
	changed, err := s.Store.PutTemplate(template)
	if err != nil {
		return Result{}, err
	}
	return Result{Kind: v0.AgentTemplateKind, Name: template.Metadata.Name, Changed: changed}, nil
}

// EqualJSON compares canonical typed records, not their formatting or YAML
// key order. Backends use this after reading an existing immutable record.
func EqualJSON(left, right any) bool {
	a, aErr := json.Marshal(left)
	b, bErr := json.Marshal(right)
	return aErr == nil && bErr == nil && bytes.Equal(a, b)
}
