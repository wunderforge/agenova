// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMemoryOperationsStrictRoundTripAndDefaultDeny(t *testing.T) {
	data, err := os.ReadFile("../../harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.json")
	if err != nil {
		t.Fatal(err)
	}
	request, validation := ParseClaimRequestJSON(data)
	if validation != nil {
		t.Fatal(validation)
	}
	if len(request.Spec.RequestedAccess.MemoryOperations) != 0 {
		t.Fatal("legacy scopes acquired operations")
	}
	request.Spec.RequestedAccess.MemoryOperations = []string{MemoryRead, MemoryWrite}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	got, validation := ParseClaimRequestJSON(encoded)
	if validation != nil || !reflect.DeepEqual(got.Spec.RequestedAccess.MemoryOperations, request.Spec.RequestedAccess.MemoryOperations) {
		t.Fatalf("JSON round trip: %v", validation)
	}
	encoded, err = yaml.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	got, validation = ParseClaimRequestYAML(encoded)
	if validation != nil || !reflect.DeepEqual(got.Spec.RequestedAccess.MemoryOperations, request.Spec.RequestedAccess.MemoryOperations) {
		t.Fatalf("YAML round trip: %v", validation)
	}
	for _, operations := range [][]string{{"admin"}, {""}, {"READ"}, {MemoryRead, MemoryRead}} {
		request.Spec.RequestedAccess.MemoryOperations = operations
		if validation := ValidateClaimRequest(request); validation == nil || validation.Category != ValidationCategoryInvalidValue {
			t.Fatalf("accepted invalid request operations %v", operations)
		}
	}
}

func TestTemplateAndIssuedMemoryOperationsFailClosed(t *testing.T) {
	data, err := os.ReadFile("../../harness/fixtures/contract/v0/inputs/agent-template/valid-engineer.yaml")
	if err != nil {
		t.Fatal(err)
	}
	template, validation := ParseAgentTemplateYAML(data)
	if validation != nil {
		t.Fatal(validation)
	}
	template.Spec.CapabilityCeiling.MemoryOperations = []string{MemoryRead, MemoryWrite}
	encoded, err := yaml.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}
	if _, validation = ParseAgentTemplateYAML(encoded); validation != nil {
		t.Fatal(validation)
	}
	for _, operations := range [][]string{{"reset"}, {" "}, {MemoryWrite, MemoryWrite}} {
		template.Spec.CapabilityCeiling.MemoryOperations = operations
		if validation := ValidateAgentTemplate(template); validation == nil || validation.Category != ValidationCategoryInvalidCapabilityCeiling {
			t.Fatalf("accepted invalid ceiling %v", operations)
		}
	}
	data, err = os.ReadFile("../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json")
	if err != nil {
		t.Fatal(err)
	}
	state, validation := ParseSystemIssuedState(data)
	if validation != nil {
		t.Fatal(validation)
	}
	state.EffectiveAuthority.MemoryOperations = []string{MemoryRead}
	if validation = ValidateIssuedState(state); validation != nil {
		t.Fatal(validation)
	}
	state.EffectiveAuthority.MemoryOperations = []string{"reset"}
	if validation = ValidateIssuedState(state); validation == nil || validation.FieldPath != "effectiveAuthority.memoryOperations[0]" {
		t.Fatalf("invalid issued operation: %v", validation)
	}
}

func TestMemoryOperationWireShapesRejectNonLists(t *testing.T) {
	data, err := os.ReadFile("../../harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.json")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	access := document["spec"].(map[string]any)["requestedAccess"].(map[string]any)
	for _, value := range []any{"read", []any{42}, map[string]any{"read": true}} {
		access["memoryOperations"] = value
		encoded, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if _, validation := ParseClaimRequestJSON(encoded); validation == nil {
			t.Fatalf("JSON accepted %v", value)
		}
		encoded, err = yaml.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if _, validation := ParseClaimRequestYAML(encoded); validation == nil {
			t.Fatalf("YAML accepted %v", value)
		}
	}
}
