// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package v1alpha1

import (
	"bytes"
	"encoding/json"
	"gopkg.in/yaml.v3"
	"testing"
)

func TestOrdinaryTaskNeedsNoCategoryAndCanonicalizesEmptyType(t *testing.T) {
	original := validClaimRequest()
	typed, _ := json.Marshal(original)
	original.Spec.Task.Type = ""
	generic, _ := json.Marshal(original)
	if bytes.Contains(generic, []byte(`"type"`)) {
		t.Fatal("generic category was invented")
	}
	for _, raw := range [][]byte{generic, bytes.Replace(generic, []byte(`"task":{`), []byte(`"task":{"type":"",`), 1), bytes.Replace(generic, []byte(`"task":{`), []byte(`"task":{"type":null,`), 1)} {
		parsed, err := ParseClaimRequestJSON(raw)
		if err != nil {
			t.Fatal(err)
		}
		canonical, _ := json.Marshal(parsed)
		if !bytes.Equal(canonical, generic) {
			t.Fatal("generic forms drifted")
		}
	}
	legacy, err := ParseClaimRequestJSON(typed)
	if err != nil || legacy.Spec.Task.Type != "repository-change" {
		t.Fatal(legacy, err)
	}
	again, _ := json.Marshal(legacy)
	if !bytes.Equal(again, typed) {
		t.Fatal("legacy serialized content changed")
	}
	for _, r := range []*ClaimRequest{original, legacy} {
		encoded, err := yaml.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		parsed, invalid := ParseClaimRequestYAML(encoded)
		if invalid != nil {
			t.Fatal(invalid)
		}
		got, _ := json.Marshal(parsed)
		want, _ := json.Marshal(r)
		if !bytes.Equal(got, want) {
			t.Fatal("CLI YAML parser changed explicit or omitted task metadata")
		}
	}
}
