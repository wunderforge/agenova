// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

func TestCanonicalFixtures(t *testing.T) {
	rows, err := fixtures("../..")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 12 {
		t.Fatalf("expected 12 manifest cases, got %d; review fixture coverage", len(rows))
	}
	byID := map[string]fixtureRow{}
	for _, row := range rows {
		byID[row.ID] = row
		t.Run(row.ID, func(t *testing.T) {
			if row.Expected.Outcome == "invalid" && (row.Data != nil || row.Diagnostic == nil) {
				t.Fatal("invalid data escaped into bundle")
			}
		})
	}
	for _, row := range rows {
		if row.EquivalentTo != "" {
			a, _ := json.Marshal(row.Data)
			b, _ := json.Marshal(byID[row.EquivalentTo].Data)
			if !bytes.Equal(a, b) {
				t.Fatal("YAML/JSON canonical parity drift")
			}
		}
	}
}

func TestBindingsTrackEnumsAndDuration(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "api/v1alpha1")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"types.go", "sandbox_claim.go"} {
		b, err := os.ReadFile(filepath.Join("../../api/v1alpha1", name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "types.go" {
			b = bytes.Replace(b, []byte("ClaimPhasePending   ClaimPhase = \"Pending\""), []byte("ClaimPhasePending   ClaimPhase = \"FuturePending\""), 1)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	before, err := bindings("../..")
	if err != nil {
		t.Fatal(err)
	}
	after, err := bindings(root)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, after) || !bytes.Contains(after, []byte("FuturePending")) {
		t.Fatal("canonical enum drift was missed")
	}
	wire, err := json.Marshal(v0.Duration(1))
	if err != nil || len(wire) < 2 || wire[0] != '"' {
		t.Fatal("Duration custom wire encoding changed")
	}
}
