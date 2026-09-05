// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestIssuedStateFixtures(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "harness", "fixtures", "contract", "v0")
	manifestData, err := os.ReadFile(filepath.Join(fixtureRoot, "manifest.json"))
	if err != nil {
		t.Fatalf("read shared fixture manifest: %v", err)
	}

	var manifest fixtureManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("decode shared fixture manifest: %v", err)
	}

	count := 0
	for _, fixture := range manifest.Cases {
		if fixture.Subject != "IssuedState" {
			continue
		}
		count++
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join(fixtureRoot, filepath.FromSlash(fixture.Input)))
			if err != nil {
				t.Fatalf("read shared fixture input: %v", err)
			}

			var (
				state         *IssuedState
				validationErr *ValidationError
			)
			if strings.Contains(fixture.ID, "invalid.caller-") {
				state, validationErr = ParseCallerIssuedState(input)
			} else {
				state, validationErr = ParseSystemIssuedState(input)
			}

			if fixture.Expected.Outcome == "valid" {
				if validationErr != nil {
					t.Fatalf("Parse(%s) error = %#v", fixture.ID, validationErr)
				}
				assertIssuedStateFixture(t, fixture.ID, state)
				return
			}
			if validationErr == nil {
				t.Fatalf("Parse(%s) error = nil, want category %q", fixture.ID, fixture.Expected.Category)
			}
			if validationErr.Category != fixture.Expected.Category {
				t.Fatalf("validation category = %q, want %q (path %q)", validationErr.Category, fixture.Expected.Category, validationErr.FieldPath)
			}
			if strings.TrimSpace(validationErr.FieldPath) == "" {
				t.Fatal("validation field path is blank")
			}
		})
	}

	if count != 5 {
		t.Fatalf("IssuedState fixture count = %d, want 5", count)
	}
}

func assertIssuedStateFixture(t *testing.T, id string, state *IssuedState) {
	t.Helper()
	switch id {
	case "issued-state.valid.team-a-engineer":
		if state.Decision.Result != DecisionResultAllow {
			t.Fatalf("decision.result = %s, want Allow", state.Decision.Result)
		}
		if state.Claim == nil || state.EffectiveAuthority == nil {
			t.Fatal("Allow decision must carry claim and effectiveAuthority")
		}
		if state.Claim.AuthorityRef != state.EffectiveAuthority.ID {
			t.Fatalf("claim.authorityRef = %q, want %q", state.Claim.AuthorityRef, state.EffectiveAuthority.ID)
		}
		if state.Evidence.ClaimID != state.Claim.ID {
			t.Fatalf("evidence.claimId = %q, want %q", state.Evidence.ClaimID, state.Claim.ID)
		}
	case "issued-state.valid.team-b-denial":
		if state.Decision.Result != DecisionResultDeny {
			t.Fatalf("decision.result = %s, want Deny", state.Decision.Result)
		}
		if state.Claim != nil {
			t.Fatal("Deny decision must not carry a claim")
		}
		if state.EffectiveAuthority != nil {
			t.Fatal("Deny decision must not carry effectiveAuthority")
		}
		if state.Evidence.ClaimID != "" {
			t.Fatalf("evidence.claimId = %q, want empty (pre-claim denial)", state.Evidence.ClaimID)
		}
	}
}

// --- hand-built cases not covered by the shared fixtures ---

func validAllowIssuedState() *IssuedState {
	return &IssuedState{
		RequestRef: "req-1",
		Principal:  Principal{Subject: "user:a", Team: "team-a", AuthenticationContext: "upstream:test"},
		Action:     Action{Name: "claim.create", Project: "payments", TemplateRef: "engineer"},
		PolicyRef:  PolicyReference{ID: "policy-1", Version: "1"},
		EffectiveAuthority: &EffectiveAuthority{
			ID:      "authority-1",
			Runtime: EffectiveAuthorityRuntime{ProfileRef: "standard", Timeout: Duration(20 * time.Minute)},
		},
		Claim: &SandboxClaim{
			ID:              "claim-1",
			RequestRef:      "req-1",
			TemplateRef:     "engineer",
			AuthorityRef:    "authority-1",
			Phase:           ClaimPhaseRunning,
			BackendIdentity: &SandboxClaimBackendIdentity{Backend: "reference", WorkerID: "worker-1"},
		},
		Decision: Decision{
			ID:           "decision-1",
			PrincipalRef: "user:a",
			Action:       "claim.create",
			Result:       DecisionResultAllow,
			PolicyRef:    PolicyReference{ID: "policy-1", Version: "1"},
			Reason:       "user:a may create the engineer assignment for the payments project",
		},
		Evidence: Evidence{
			RequestRef:  "req-1",
			ClaimID:     "claim-1",
			DecisionIDs: []string{"decision-1"},
		},
	}
}

func TestValidateIssuedState_AllowRequiresClaimAndAuthority(t *testing.T) {
	state := validAllowIssuedState()
	state.Claim = nil
	assertValidationError(t, ValidateIssuedState(state), ValidationCategoryInvalidValue, "claim")
}

func TestValidateIssuedState_AllowRequiresEffectiveAuthority(t *testing.T) {
	state := validAllowIssuedState()
	state.EffectiveAuthority = nil
	assertValidationError(t, ValidateIssuedState(state), ValidationCategoryInvalidValue, "effectiveAuthority")
}

func TestValidateIssuedState_DenyMustNotCarryClaimOrAuthority(t *testing.T) {
	state := validAllowIssuedState()
	state.Decision.Result = DecisionResultDeny
	assertValidationError(t, ValidateIssuedState(state), ValidationCategoryInvalidValue, "claim")
}

func TestValidateIssuedState_EvidenceDecisionIdsMustContainDecision(t *testing.T) {
	state := validAllowIssuedState()
	state.Evidence.DecisionIDs = []string{"some-other-decision"}
	assertValidationError(t, ValidateIssuedState(state), ValidationCategoryInvalidValue, "evidence.decisionIds")
}

func TestValidateIssuedState_EvidenceClaimIdRequiresAllow(t *testing.T) {
	state := validAllowIssuedState()
	state.Decision.Result = DecisionResultDeny
	state.Claim = nil
	state.EffectiveAuthority = nil
	state.Evidence.ClaimID = "claim-1" // fabricated: no claim was ever issued
	assertValidationError(t, ValidateIssuedState(state), ValidationCategoryInvalidValue, "evidence.claimId")
}

func TestValidateIssuedState_PendingClaimWithoutBackendIdentityIsValid(t *testing.T) {
	state := validAllowIssuedState()
	state.Claim.Phase = ClaimPhasePending
	state.Claim.BackendIdentity = nil
	if err := ValidateIssuedState(state); err != nil {
		t.Fatalf("Pending claim without backendIdentity should validate: %#v", err)
	}
}

func TestValidateIssuedState_PendingClaimRejectsBackendIdentity(t *testing.T) {
	state := validAllowIssuedState()
	state.Claim.Phase = ClaimPhasePending
	assertValidationError(t, ValidateIssuedState(state), ValidationCategoryInvalidValue, "claim.backendIdentity")
}

func TestValidateIssuedState_ApprovalRequiredIsNotGrantedAuthority(t *testing.T) {
	state := validAllowIssuedState()
	state.Decision.Result = DecisionResultApprovalRequired
	assertValidationError(t, ValidateIssuedState(state), ValidationCategoryInvalidValue, "claim")
}

func TestValidateIssuedState_FailsClosedOnBlankOrInvalidFields(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*IssuedState)
		category ValidationCategory
		path     string
	}{
		{"blank policyRef.id", func(s *IssuedState) { s.PolicyRef.ID = ""; s.Decision.PolicyRef.ID = "" }, ValidationCategoryRequiredField, "policyRef.id"},
		{"blank policyRef.version", func(s *IssuedState) { s.PolicyRef.Version = ""; s.Decision.PolicyRef.Version = "" }, ValidationCategoryRequiredField, "policyRef.version"},
		{"blank decision.reason", func(s *IssuedState) { s.Decision.Reason = "" }, ValidationCategoryRequiredField, "decision.reason"},
		{"blank action.project", func(s *IssuedState) { s.Action.Project = "" }, ValidationCategoryRequiredField, "action.project"},
		{"blank action.templateRef and claim.templateRef", func(s *IssuedState) { s.Action.TemplateRef = ""; s.Claim.TemplateRef = "" }, ValidationCategoryRequiredField, "action.templateRef"},
		{"blank claim.templateRef alone", func(s *IssuedState) { s.Claim.TemplateRef = "" }, ValidationCategoryInvalidValue, "claim.templateRef"},
		{"blank claim.id", func(s *IssuedState) { s.Claim.ID = "" }, ValidationCategoryRequiredField, "claim.id"},
		{"blank effectiveAuthority.id and claim.authorityRef", func(s *IssuedState) { s.EffectiveAuthority.ID = ""; s.Claim.AuthorityRef = "" }, ValidationCategoryRequiredField, "effectiveAuthority.id"},
		{"blank claim.authorityRef alone", func(s *IssuedState) { s.Claim.AuthorityRef = "" }, ValidationCategoryInvalidValue, "claim.authorityRef"},
		{"blank effectiveAuthority.runtime.profileRef", func(s *IssuedState) { s.EffectiveAuthority.Runtime.ProfileRef = "" }, ValidationCategoryRequiredField, "effectiveAuthority.runtime.profileRef"},
		{"unknown claim.phase", func(s *IssuedState) { s.Claim.Phase = ClaimPhase("Deleted") }, ValidationCategoryInvalidValue, "claim.phase"},
		{"blank claim.phase", func(s *IssuedState) { s.Claim.Phase = ClaimPhase("") }, ValidationCategoryInvalidValue, "claim.phase"},
		{"claim.templateRef mismatched with action.templateRef", func(s *IssuedState) { s.Claim.TemplateRef = "other-template" }, ValidationCategoryInvalidValue, "claim.templateRef"},
		{"blank backendIdentity.backend", func(s *IssuedState) { s.Claim.BackendIdentity.Backend = "" }, ValidationCategoryRequiredField, "claim.backendIdentity.backend"},
		{"blank backendIdentity.workerId", func(s *IssuedState) { s.Claim.BackendIdentity.WorkerID = "" }, ValidationCategoryRequiredField, "claim.backendIdentity.workerId"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := validAllowIssuedState()
			tc.mutate(state)
			assertValidationError(t, ValidateIssuedState(state), tc.category, tc.path)
		})
	}
}

func TestParseSystemIssuedState_RejectsMalformedTimeoutSyntax(t *testing.T) {
	// A malformed duration string cannot be represented by the typed Duration
	// field at all; it can only be exercised at JSON-decode time, where the
	// Duration.UnmarshalJSON parse failure surfaces as an invalid-document error.
	data, err := json.Marshal(validAllowIssuedState())
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	corrupted := strings.Replace(string(data), `"20m0s"`, `"banana"`, 1)
	if corrupted == string(data) {
		t.Fatal("fixture did not contain the expected timeout literal to corrupt")
	}
	_, validationErr := ParseSystemIssuedState([]byte(corrupted))
	assertValidationError(t, validationErr, ValidationCategoryInvalidDocument, "$")
}

func TestValidateIssuedState_RejectsNonPositiveTimeout(t *testing.T) {
	state := validAllowIssuedState()
	state.EffectiveAuthority.Runtime.Timeout = Duration(-5 * time.Minute)
	assertValidationError(t, ValidateIssuedState(state), ValidationCategoryInvalidValue, "effectiveAuthority.runtime.timeout")
}

func TestParseCallerIssuedState_IgnoresSpoofedSourceField(t *testing.T) {
	// A caller-supplied "source": "system" field must not grant trust; only
	// which parse function is invoked determines the trust boundary.
	doc := []byte(`{"source":"system","requestRef":"req-1","claim":{"phase":"Running"}}`)
	_, err := ParseCallerIssuedState(doc)
	assertValidationError(t, err, ValidationCategorySystemManagedField, "claim.phase")
}

func TestParseCallerIssuedState_RejectsEffectiveAuthority(t *testing.T) {
	doc := []byte(`{"requestRef":"req-1","effectiveAuthority":{"tools":["git.write"]}}`)
	_, err := ParseCallerIssuedState(doc)
	assertValidationError(t, err, ValidationCategorySystemManagedField, "effectiveAuthority")
}

func TestParseCallerIssuedState_RejectsBackendIdentity(t *testing.T) {
	doc := []byte(`{"requestRef":"req-1","claim":{"backendIdentity":{"backend":"reference","workerId":"caller-chosen"}}}`)
	_, err := ParseCallerIssuedState(doc)
	assertValidationError(t, err, ValidationCategorySystemManagedField, "claim.backendIdentity")
}

func TestParseSystemIssuedState_AcceptsSystemManagedFields(t *testing.T) {
	data, err := json.Marshal(validAllowIssuedState())
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if _, validationErr := ParseSystemIssuedState(data); validationErr != nil {
		t.Fatalf("ParseSystemIssuedState() error = %#v", validationErr)
	}
}

func TestParseIssuedState_RejectsDecisionResultOutsideVocabulary(t *testing.T) {
	data, err := json.Marshal(validAllowIssuedState())
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	// Corrupt the decoded document's result to a value outside the typed
	// vocabulary; encoding/json accepts any string into DecisionResult since
	// it is not decode-time validated, so this exercises ValidateIssuedState.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	raw["decision"] = json.RawMessage(strings.Replace(string(raw["decision"]), `"Allow"`, `"Approved"`, 1))
	corrupted, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal corrupted fixture: %v", err)
	}

	_, validationErr := ParseSystemIssuedState(corrupted)
	assertValidationError(t, validationErr, ValidationCategoryInvalidValue, "decision.result")
}

// TestIssuedStateFixtures_RoundTrip carries the shared-fixture round-trip
// requirement: parse -> marshal -> parse must reach the same semantic value,
// not just decode without error. Before decodeIssuedState normalized
// evidence-item slices, an explicit `[]` in the fixture decoded to a non-nil
// empty slice but re-encoded (with `omitempty`) as an omitted field, which
// then reparsed as nil — a silent drift across the round trip.
func TestIssuedStateFixtures_RoundTrip(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "harness", "fixtures", "contract", "v0")
	manifestData, err := os.ReadFile(filepath.Join(fixtureRoot, "manifest.json"))
	if err != nil {
		t.Fatalf("read shared fixture manifest: %v", err)
	}

	var manifest fixtureManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("decode shared fixture manifest: %v", err)
	}

	count := 0
	for _, fixture := range manifest.Cases {
		if fixture.Subject != "IssuedState" || fixture.Expected.Outcome != "valid" {
			continue
		}
		count++
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join(fixtureRoot, filepath.FromSlash(fixture.Input)))
			if err != nil {
				t.Fatalf("read shared fixture input: %v", err)
			}

			first, validationErr := ParseSystemIssuedState(input)
			if validationErr != nil {
				t.Fatalf("Parse(%s) error = %#v", fixture.ID, validationErr)
			}

			marshaled, err := json.Marshal(first)
			if err != nil {
				t.Fatalf("marshal parsed state: %v", err)
			}

			second, validationErr := ParseSystemIssuedState(marshaled)
			if validationErr != nil {
				t.Fatalf("re-parse marshaled state: %#v", validationErr)
			}

			if !reflect.DeepEqual(first, second) {
				t.Fatalf("round trip not semantically equivalent:\nfirst  = %#v\nsecond = %#v", first, second)
			}
		})
	}

	if count != 2 {
		t.Fatalf("valid IssuedState fixture count = %d, want 2", count)
	}
}
