// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

func TestConsoleNarrowing(t *testing.T) {
	canonical, err := fixtures("../..")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := consoleFixtures("../..")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(canonical)+1 {
		t.Fatal("expected one derived scenario")
	}
	derived := rows[len(rows)-1]
	if derived.ID != "derived.console.narrowed" || derived.DerivedFrom != "issued-state.valid.team-a-engineer" {
		t.Fatal("derivation provenance missing")
	}
	state := derived.Data.(*v0.IssuedState)
	if !reflect.DeepEqual(state.EffectiveAuthority.Tools, []string{"git.read", "git.write"}) {
		t.Fatal("unexpected narrowed tools")
	}
	if failure := v0.ValidateIssuedState(state); failure != nil {
		t.Fatal(failure)
	}
	for i, row := range canonical {
		a, _ := json.Marshal(row)
		b, _ := json.Marshal(rows[i])
		if !bytes.Equal(a, b) {
			t.Fatal("canonical fixture was mutated")
		}
		if row.ID == derived.DerivedFrom {
			original := row.Data.(*v0.IssuedState)
			state.EffectiveAuthority.Tools = original.EffectiveAuthority.Tools
			if !reflect.DeepEqual(state, original) {
				t.Fatal("narrowing changed more than effective tools")
			}
		}
	}
}

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
