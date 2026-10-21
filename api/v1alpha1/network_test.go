// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNetworkProfilesCanonicalRequestAndTemplate(t *testing.T) {
	data, err := os.ReadFile("../../harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.yaml")
	if err != nil {
		t.Fatal(err)
	}
	request, parseErr := ParseClaimRequestYAML(data)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if len(request.Spec.RequestedAccess.NetworkProfiles) != 0 {
		t.Fatal("legacy request acquired networking")
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "networkProfiles") {
		t.Fatal("legacy request wire representation changed")
	}
	request.Spec.RequestedAccess.NetworkProfiles = []string{NetworkProfilePublicWeb}
	for _, format := range []string{"yaml", "json"} {
		t.Run(format, func(t *testing.T) {
			var parsed *ClaimRequest
			var validationErr *ValidationError
			if format == "yaml" {
				encoded, err = yaml.Marshal(request)
				if err != nil {
					t.Fatal(err)
				}
				parsed, validationErr = ParseClaimRequestYAML(encoded)
			} else {
				encoded, err = json.Marshal(request)
				if err != nil {
					t.Fatal(err)
				}
				parsed, validationErr = ParseClaimRequestJSON(encoded)
			}
			if validationErr != nil || !reflect.DeepEqual(parsed.Spec.RequestedAccess.NetworkProfiles, []string{NetworkProfilePublicWeb}) {
				t.Fatalf("network request did not round trip: %+v / %v", parsed, validationErr)
			}
		})
	}
	template := validAgentTemplate()
	template.Spec.CapabilityCeiling.NetworkProfiles = []string{NetworkProfilePublicWeb}
	encoded, err = yaml.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}
	parsed, validationErr := ParseAgentTemplateYAML(encoded)
	if validationErr != nil || !reflect.DeepEqual(parsed.Spec.CapabilityCeiling.NetworkProfiles, []string{NetworkProfilePublicWeb}) {
		t.Fatalf("network ceiling did not round trip: %+v / %v", parsed, validationErr)
	}
	for _, profiles := range [][]string{{""}, {"*"}, {"private"}, {"public-web", "public-web"}} {
		request.Spec.RequestedAccess.NetworkProfiles = profiles
		if err := ValidateClaimRequest(request); err == nil || !strings.HasPrefix(err.FieldPath, "spec.requestedAccess.networkProfiles[") {
			t.Fatalf("invalid request profiles accepted: %q / %v", profiles, err)
		}
		template.Spec.CapabilityCeiling.NetworkProfiles = profiles
		if err := ValidateAgentTemplate(template); err == nil || !strings.HasPrefix(err.FieldPath, "spec.capabilityCeiling.networkProfiles[") {
			t.Fatalf("invalid ceiling profiles accepted: %q / %v", profiles, err)
		}
	}
}

func TestNetworkProfilesIssuedStateValidationAndLegacyCompatibility(t *testing.T) {
	data, err := os.ReadFile("../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json")
	if err != nil {
		t.Fatal(err)
	}
	state, validationErr := ParseSystemIssuedState(data)
	if validationErr != nil {
		t.Fatal(validationErr)
	}
	if len(state.EffectiveAuthority.NetworkProfiles) != 0 {
		t.Fatal("historical state acquired network grant")
	}
	state.EffectiveAuthority.NetworkProfiles = []string{NetworkProfilePublicWeb}
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	parsed, validationErr := ParseSystemIssuedState(encoded)
	if validationErr != nil || !reflect.DeepEqual(parsed.EffectiveAuthority.NetworkProfiles, []string{NetworkProfilePublicWeb}) {
		t.Fatalf("issued grant roundtrip failed: %+v / %v", parsed, validationErr)
	}
	for _, profiles := range [][]string{{"unknown"}, {"public-web", "public-web"}} {
		state.EffectiveAuthority.NetworkProfiles = profiles
		if err := ValidateIssuedState(state); err == nil || !strings.HasPrefix(err.FieldPath, "effectiveAuthority.networkProfiles[") {
			t.Fatalf("invalid issued profiles accepted: %q / %v", profiles, err)
		}
	}
}

func TestNetworkProfilesStrictWireShape(t *testing.T) {
	data, err := os.ReadFile("../../harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.yaml")
	if err != nil {
		t.Fatal(err)
	}
	request, validationErr := ParseClaimRequestYAML(data)
	if validationErr != nil {
		t.Fatal(validationErr)
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	access := wire["spec"].(map[string]any)["requestedAccess"].(map[string]any)
	for _, test := range []struct {
		name  string
		value any
		valid bool
	}{
		{"null", nil, true},
		{"empty", []any{}, true},
		{"valid", []any{"public-web"}, true},
		{"scalar", "public-web", false},
		{"object", map[string]any{"values": []any{"public-web"}}, false},
		{"non-string", []any{true}, false},
		{"unknown", []any{"unknown"}, false},
		{"duplicate", []any{"public-web", "public-web"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			access["networkProfiles"] = test.value
			encoded, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			parsed, validationErr := ParseClaimRequestJSON(encoded)
			if (validationErr == nil) != test.valid {
				t.Fatalf("wire shape accepted=%v, want %v: %v", validationErr == nil, test.valid, validationErr)
			}
			if test.valid && (test.value == nil || test.name == "empty") && len(parsed.Spec.RequestedAccess.NetworkProfiles) != 0 {
				t.Fatal("empty input manufactured a network request")
			}
		})
	}
}

func TestBackendAllocationIdentityIsOpaqueOptionalAndStrictlyParsed(t *testing.T) {
	data, err := os.ReadFile("../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json")
	if err != nil {
		t.Fatal(err)
	}
	state, validationErr := ParseSystemIssuedState(data)
	if validationErr != nil {
		t.Fatal(validationErr)
	}
	state.Claim.Phase = ClaimPhaseBound
	state.Claim.BackendIdentity = &SandboxClaimBackendIdentity{Backend: "reference-backend", WorkerID: "worker-one"}
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "allocationID") {
		t.Fatal("legacy identity wire representation changed")
	}
	for _, id := range []string{"opaque:allocation:one", "任意后端:一", strings.Repeat("a", 256)} {
		state.Claim.BackendIdentity.AllocationID = id
		encoded, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		parsed, validationErr := ParseSystemIssuedState(encoded)
		if validationErr != nil || *parsed.Claim.BackendIdentity != *state.Claim.BackendIdentity {
			t.Fatalf("opaque allocation identity lost: %+v / %v", parsed, validationErr)
		}
	}
	for _, id := range []string{" ", "\t", " leading", "trailing ", strings.Repeat("a", 257), string([]byte{0xff})} {
		state.Claim.BackendIdentity.AllocationID = id
		assertValidationError(t, ValidateIssuedState(state), ValidationCategoryInvalidValue, "claim.backendIdentity.allocationID")
	}
}
