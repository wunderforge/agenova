// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

type diagnostic struct {
	Category  string `json:"category"`
	FieldPath string `json:"fieldPath"`
}
type fixtureCase struct {
	ID           string `json:"id"`
	Subject      string `json:"subject"`
	Input        string `json:"input"`
	Format       string `json:"format"`
	EquivalentTo string `json:"equivalentTo,omitempty"`
	Expected     struct {
		Outcome  string `json:"outcome"`
		Category string `json:"category,omitempty"`
	} `json:"expected"`
}
type fixtureRow struct {
	fixtureCase
	Data       any         `json:"data,omitempty"`
	Diagnostic *diagnostic `json:"diagnostic,omitempty"`
}

// The canonical parser is the sole semantic validator. Only normalized valid
// data or sanitized category/path diagnostics enter the browser bundle.
func fixtures(root string) ([]fixtureRow, error) {
	dir := filepath.Join(root, "harness/fixtures/contract/v0")
	contents, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var manifest struct {
		SchemaVersion string        `json:"schemaVersion"`
		Cases         []fixtureCase `json:"cases"`
	}
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return nil, err
	}
	if manifest.SchemaVersion != "agenova.contract-fixtures/v0" {
		return nil, fmt.Errorf("unsupported fixture version")
	}
	rows := []fixtureRow{}
	for _, c := range manifest.Cases {
		if c.Subject != "ClaimRequest" && c.Subject != "IssuedState" {
			continue
		}
		if !filepath.IsLocal(c.Input) || !strings.HasPrefix(filepath.ToSlash(c.Input), "inputs/") {
			return nil, fmt.Errorf("invalid fixture path for %s", c.ID)
		}
		input, err := os.ReadFile(filepath.Join(dir, c.Input))
		if err != nil {
			return nil, err
		}
		var data any
		var failure *v0.ValidationError
		switch c.Subject {
		case "ClaimRequest":
			switch c.Format {
			case "json":
				data, failure = v0.ParseClaimRequestJSON(input)
			case "yaml":
				data, failure = v0.ParseClaimRequestYAML(input)
			default:
				return nil, fmt.Errorf("unsupported request fixture format")
			}
		case "IssuedState":
			if c.Format != "json" {
				return nil, fmt.Errorf("unsupported issued fixture format")
			}
			// Trust selection is code-owned: the manifest's expectation or any
			// payload source flag cannot turn a caller case into system state.
			switch {
			case strings.HasPrefix(c.ID, "issued-state.invalid.caller-"):
				data, failure = v0.ParseCallerIssuedState(input)
			case strings.HasPrefix(c.ID, "issued-state.valid."):
				data, failure = v0.ParseSystemIssuedState(input)
			default:
				return nil, fmt.Errorf("unclassified trust boundary for %s", c.ID)
			}
		}
		row := fixtureRow{fixtureCase: c}
		if failure != nil {
			if c.Expected.Outcome != "invalid" || c.Expected.Category != string(failure.Category) {
				return nil, fmt.Errorf("fixture expectation drift: %s", c.ID)
			}
			row.Diagnostic = &diagnostic{string(failure.Category), failure.FieldPath}
		} else {
			if c.Expected.Outcome != "valid" {
				return nil, fmt.Errorf("fixture unexpectedly accepted: %s", c.ID)
			}
			row.Data = data
			// Canonical normalization is preserved, but absence must not look
			// like observed empty arrays to a reviewer of the original source.
			if c.Subject == "IssuedState" {
				var raw struct {
					Evidence map[string]json.RawMessage `json:"evidence"`
				}
				if err := json.Unmarshal(input, &raw); err != nil {
					return nil, err
				}
				for _, k := range []string{"runtimeEvents", "toolInvocations", "modelInvocations"} {
					v := raw.Evidence[k]
					if len(v) == 0 || string(v) == "null" {
						row.Data = nil
						row.Diagnostic = &diagnostic{"missing-source-field", "evidence." + k}
						break
					}
				}
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}
