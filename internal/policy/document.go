// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"bytes"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

const (
	DocumentAPIVersion = "agenova.io/v1alpha1"
	DocumentKind       = "PolicyBundle"
)

// Document is the operator-authored representation of one immutable policy
// version. It does not contain credentials or an asserted caller identity.
type Document struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
	Metadata   struct {
		Name    string `json:"name" yaml:"name"`
		Version string `json:"version" yaml:"version"`
	} `json:"metadata" yaml:"metadata"`
	Spec struct {
		Rules []Rule `json:"rules" yaml:"rules"`
	} `json:"spec" yaml:"spec"`
}

func ParseDocumentYAML(data []byte) (PolicyBundle, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var document Document
	if err := decoder.Decode(&document); err != nil {
		return PolicyBundle{}, fmt.Errorf("decode PolicyBundle: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return PolicyBundle{}, fmt.Errorf("PolicyBundle must contain exactly one document")
	}
	if document.APIVersion != DocumentAPIVersion || document.Kind != DocumentKind {
		return PolicyBundle{}, fmt.Errorf("PolicyBundle requires apiVersion %s and kind %s", DocumentAPIVersion, DocumentKind)
	}
	bundle := PolicyBundle{ID: document.Metadata.Name, Version: document.Metadata.Version, Rules: append([]Rule{}, document.Spec.Rules...)}
	if err := ValidateBundle(bundle); err != nil {
		return PolicyBundle{}, err
	}
	return bundle, nil
}
