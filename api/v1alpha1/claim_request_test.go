// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

type claimRequestFixtureManifest struct {
	Cases []claimRequestFixtureCase `json:"cases"`
}

type claimRequestFixtureCase struct {
	ID       string                      `json:"id"`
	Subject  string                      `json:"subject"`
	Input    string                      `json:"input"`
	Format   string                      `json:"format"`
	Expected claimRequestFixtureExpected `json:"expected"`
}

type claimRequestFixtureExpected struct {
	Outcome  string             `json:"outcome"`
	Category ValidationCategory `json:"category,omitempty"`
}

func claimRequestFixtureRoot() string {
	return filepath.Join("..", "..", "harness", "fixtures", "contract", "v0")
}

func loadClaimRequestCases(t *testing.T) []claimRequestFixtureCase {
	t.Helper()

	manifestData, err := os.ReadFile(filepath.Join(claimRequestFixtureRoot(), "manifest.json"))
	if err != nil {
		t.Fatalf("read shared fixture manifest: %v", err)
	}
	var manifest claimRequestFixtureManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("decode shared fixture manifest: %v", err)
	}

	var cases []claimRequestFixtureCase
	for _, fixture := range manifest.Cases {
		if fixture.Subject == ClaimRequestKind {
			cases = append(cases, fixture)
		}
	}
	return cases
}

func parseClaimRequestFixture(t *testing.T, fixture claimRequestFixtureCase) (*ClaimRequest, *ValidationError) {
	t.Helper()

	input, err := os.ReadFile(filepath.Join(claimRequestFixtureRoot(), filepath.FromSlash(fixture.Input)))
	if err != nil {
		t.Fatalf("read shared fixture input: %v", err)
	}
	switch fixture.Format {
	case "yaml":
		return ParseClaimRequestYAML(input)
	case "json":
		return ParseClaimRequestJSON(input)
	default:
		t.Fatalf("fixture %s has unsupported format %q", fixture.ID, fixture.Format)
		return nil, nil
	}
}

func TestClaimRequestFixtures(t *testing.T) {
	cases := loadClaimRequestCases(t)

	for _, fixture := range cases {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			request, validationErr := parseClaimRequestFixture(t, fixture)
			if fixture.Expected.Outcome == "valid" {
				if validationErr != nil {
					t.Fatalf("parse error = %#v", validationErr)
				}
				assertTeamARequest(t, request)
				return
			}
			if validationErr == nil {
				t.Fatalf("parse error = nil, want category %q", fixture.Expected.Category)
			}
			if validationErr.Category != fixture.Expected.Category {
				t.Fatalf("validation category = %q, want %q (path %q)", validationErr.Category, fixture.Expected.Category, validationErr.FieldPath)
			}
			if strings.TrimSpace(validationErr.FieldPath) == "" {
				t.Fatal("validation field path is blank")
			}
		})
	}

	if len(cases) != 7 {
		t.Fatalf("ClaimRequest fixture count = %d, want 7", len(cases))
	}
}

func TestClaimRequestSurfacesAreSemanticallyEquivalent(t *testing.T) {
	var yamlRequest, jsonRequest *ClaimRequest

	for _, fixture := range loadClaimRequestCases(t) {
		if fixture.Expected.Outcome != "valid" {
			continue
		}
		request, validationErr := parseClaimRequestFixture(t, fixture)
		if validationErr != nil {
			t.Fatalf("parse %s: %#v", fixture.ID, validationErr)
		}
		switch fixture.Format {
		case "yaml":
			yamlRequest = request
		case "json":
			jsonRequest = request
		}
	}

	if yamlRequest == nil || jsonRequest == nil {
		t.Fatal("expected one canonical YAML and one API JSON valid ClaimRequest fixture")
	}
	if !reflect.DeepEqual(yamlRequest, jsonRequest) {
		t.Fatalf("canonical YAML and API JSON surfaces are not semantically equivalent:\nYAML: %#v\nJSON: %#v", yamlRequest, jsonRequest)
	}
}

// assertTeamARequest verifies the parsed canonical fixture preserves every
// declared field, and that task input stays distinct from requested resource
// scopes.
func assertTeamARequest(t *testing.T, request *ClaimRequest) {
	t.Helper()

	if request.APIVersion != ClaimRequestAPIVersion || request.Kind != ClaimRequestKind {
		t.Fatalf("identity = %s/%s, want %s/%s", request.APIVersion, request.Kind, ClaimRequestAPIVersion, ClaimRequestKind)
	}
	if request.Metadata.Name != "fix-payment-timeout" {
		t.Errorf("metadata.name = %q, want fix-payment-timeout", request.Metadata.Name)
	}
	if request.Spec.TemplateRef != "engineer" {
		t.Errorf("templateRef = %q, want engineer", request.Spec.TemplateRef)
	}
	if request.Spec.ProjectRef != "payments" {
		t.Errorf("projectRef = %q, want payments", request.Spec.ProjectRef)
	}
	if request.Spec.Task == nil || request.Spec.Task.Type != "repository-change" {
		t.Fatalf("task = %+v, want type repository-change", request.Spec.Task)
	}
	if request.Spec.Task.Input["repository"] != "acme/payments" || request.Spec.Task.Input["objective"] == "" {
		t.Errorf("task input = %v, want repository and objective work data", request.Spec.Task.Input)
	}

	access := request.Spec.RequestedAccess
	if !reflect.DeepEqual(access.Tools, []string{"git.read", "git.write", "github.pull-request"}) {
		t.Errorf("requested tools = %v", access.Tools)
	}
	if !reflect.DeepEqual(access.ResourceScopes, []string{"repo:acme/payments"}) {
		t.Errorf("requested resourceScopes = %v", access.ResourceScopes)
	}
	if access.ModelProfile != "approved-coding-model" {
		t.Errorf("requested modelProfile = %q", access.ModelProfile)
	}
	if !reflect.DeepEqual(access.MemoryScopes, []string{"team-docs"}) {
		t.Errorf("requested memoryScopes = %v", access.MemoryScopes)
	}

	// Task input is work data; resource scopes are access intent. The task's
	// repository value must not leak into scopes or vice versa.
	if access.ResourceScopes[0] == request.Spec.Task.Input["repository"] {
		t.Error("resource scope and task input share one value; the surfaces must stay distinct")
	}

	if request.Spec.Runtime == nil || request.Spec.Runtime.ProfileRef != "standard-isolated" {
		t.Fatalf("runtime = %+v, want profileRef standard-isolated", request.Spec.Runtime)
	}
	if request.Spec.Runtime.Timeout == nil || time.Duration(*request.Spec.Runtime.Timeout) != 20*time.Minute {
		t.Errorf("runtime timeout = %v, want 20m", request.Spec.Runtime.Timeout)
	}
}

func validClaimRequest() *ClaimRequest {
	timeout := Duration(20 * time.Minute)
	return &ClaimRequest{
		APIVersion: ClaimRequestAPIVersion,
		Kind:       ClaimRequestKind,
		Metadata:   ObjectMeta{Name: "fix-payment-timeout"},
		Spec: ClaimRequestSpec{
			TemplateRef: "engineer",
			ProjectRef:  "payments",
			Task: &ClaimRequestTask{
				Type:  "repository-change",
				Input: map[string]any{"repository": "acme/payments"},
			},
			RequestedAccess: ClaimRequestedAccess{Tools: []string{"git.read"}},
			Runtime:         &ClaimRuntimeRequirements{ProfileRef: "standard-isolated", Timeout: &timeout},
		},
	}
}

func assertClaimRequestValidationError(t *testing.T, err *ValidationError, category ValidationCategory, fieldPath string) {
	t.Helper()
	if err == nil {
		t.Fatalf("validation error = nil, want category %q at %q", category, fieldPath)
	}
	if err.Category != category || err.FieldPath != fieldPath {
		t.Fatalf("validation error = (%q, %q), want (%q, %q)", err.Category, err.FieldPath, category, fieldPath)
	}
}

func TestValidateClaimRequestRequiredFields(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(r *ClaimRequest)
		category ValidationCategory
		path     string
	}{
		{"blank metadata name", func(r *ClaimRequest) { r.Metadata.Name = "   " }, ValidationCategoryRequiredField, "metadata.name"},
		{"blank templateRef", func(r *ClaimRequest) { r.Spec.TemplateRef = "" }, ValidationCategoryRequiredField, "spec.templateRef"},
		{"missing task", func(r *ClaimRequest) { r.Spec.Task = nil }, ValidationCategoryRequiredField, "spec.task"},
		{"blank task type", func(r *ClaimRequest) { r.Spec.Task.Type = " " }, ValidationCategoryRequiredField, "spec.task.type"},
		{"missing runtime", func(r *ClaimRequest) { r.Spec.Runtime = nil }, ValidationCategoryRequiredField, "spec.runtime"},
		{"blank runtime profileRef", func(r *ClaimRequest) { r.Spec.Runtime.ProfileRef = "" }, ValidationCategoryRequiredField, "spec.runtime.profileRef"},
		{"missing runtime timeout", func(r *ClaimRequest) { r.Spec.Runtime.Timeout = nil }, ValidationCategoryRequiredField, "spec.runtime.timeout"},
		{"wrong apiVersion", func(r *ClaimRequest) { r.APIVersion = "agenova.io/v9" }, ValidationCategoryInvalidValue, "apiVersion"},
		{"wrong kind", func(r *ClaimRequest) { r.Kind = "AgentTemplate" }, ValidationCategoryInvalidValue, "kind"},
		{"blank requested tool", func(r *ClaimRequest) { r.Spec.RequestedAccess.Tools = []string{" "} }, ValidationCategoryInvalidValue, "spec.requestedAccess.tools[0]"},
		{"duplicate requested tool", func(r *ClaimRequest) { r.Spec.RequestedAccess.Tools = []string{"git.read", "git.read"} }, ValidationCategoryInvalidValue, "spec.requestedAccess.tools[1]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := validClaimRequest()
			tc.mutate(request)
			assertClaimRequestValidationError(t, ValidateClaimRequest(request), tc.category, tc.path)
		})
	}
}

// Task input shape is task-specific and requested access may be default-deny,
// so neither is a required field.
func TestValidateClaimRequestOptionalSurfaces(t *testing.T) {
	t.Run("absent task input", func(t *testing.T) {
		request := validClaimRequest()
		request.Spec.Task.Input = nil
		if err := ValidateClaimRequest(request); err != nil {
			t.Fatalf("absent task input must be valid, got %#v", err)
		}
	})
	t.Run("empty task input", func(t *testing.T) {
		request := validClaimRequest()
		request.Spec.Task.Input = map[string]any{}
		if err := ValidateClaimRequest(request); err != nil {
			t.Fatalf("empty task input must be valid, got %#v", err)
		}
	})
	t.Run("empty requested access is default-deny", func(t *testing.T) {
		request := validClaimRequest()
		request.Spec.RequestedAccess = ClaimRequestedAccess{}
		if err := ValidateClaimRequest(request); err != nil {
			t.Fatalf("empty requestedAccess must be valid (default-deny), got %#v", err)
		}
	})
	t.Run("explicitly empty surfaces parse from YAML", func(t *testing.T) {
		input := []byte("apiVersion: agenova.io/v1alpha1\nkind: ClaimRequest\nmetadata:\n  name: fix-payment-timeout\nspec:\n  templateRef: engineer\n  task:\n    type: repository-change\n    input: {}\n  requestedAccess: {}\n  runtime:\n    profileRef: standard-isolated\n    timeout: 20m\n")
		if _, err := ParseClaimRequestYAML(input); err != nil {
			t.Fatalf("explicitly empty input/requestedAccess must parse, got %#v", err)
		}
	})
}

// A request serialized by any consumer must be re-parseable by this contract,
// otherwise the "equivalent API JSON surface" only works one way.
func TestClaimRequestJSONRoundTrip(t *testing.T) {
	for _, fixture := range loadClaimRequestCases(t) {
		if fixture.Expected.Outcome != "valid" || fixture.Format != "json" {
			continue
		}
		original, validationErr := parseClaimRequestFixture(t, fixture)
		if validationErr != nil {
			t.Fatalf("parse %s: %#v", fixture.ID, validationErr)
		}

		encoded, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal %s: %v", fixture.ID, err)
		}
		reparsed, validationErr := ParseClaimRequestJSON(encoded)
		if validationErr != nil {
			t.Fatalf("re-parsing our own JSON failed: %#v\nencoded: %s", validationErr, encoded)
		}
		if !reflect.DeepEqual(original, reparsed) {
			t.Errorf("round trip changed the request:\nbefore: %#v\nafter:  %#v", original, reparsed)
		}
	}
}

// YAML null is the idiomatic spelling of "empty" for an optional field, and it
// must decode to the zero value rather than a shape error. For a required
// field it must reach the semantic check as a missing value.
func TestParseClaimRequestTreatsNullAsEmpty(t *testing.T) {
	head := "apiVersion: agenova.io/v1alpha1\nkind: ClaimRequest\nmetadata:\n  name: fix-payment-timeout\nspec:\n  templateRef: engineer\n"
	task := "  task:\n    type: repository-change\n"
	runtime := "  runtime:\n    profileRef: standard-isolated\n    timeout: 20m\n"

	t.Run("null requestedAccess is default-deny", func(t *testing.T) {
		request, err := ParseClaimRequestYAML([]byte(head + task + "  requestedAccess:\n" + runtime))
		if err != nil {
			t.Fatalf("null requestedAccess must be valid, got %#v", err)
		}
		if len(request.Spec.RequestedAccess.Tools) != 0 || request.Spec.RequestedAccess.ModelProfile != "" {
			t.Errorf("null requestedAccess must grant nothing, got %+v", request.Spec.RequestedAccess)
		}
	})
	t.Run("null task input", func(t *testing.T) {
		if _, err := ParseClaimRequestYAML([]byte(head + "  task:\n    type: repository-change\n    input:\n" + runtime)); err != nil {
			t.Fatalf("null task input must be valid, got %#v", err)
		}
	})
	t.Run("null requested list", func(t *testing.T) {
		if _, err := ParseClaimRequestYAML([]byte(head + task + "  requestedAccess:\n    tools:\n" + runtime)); err != nil {
			t.Fatalf("null tools list must be valid, got %#v", err)
		}
	})
	t.Run("null reserved paths keep their category", func(t *testing.T) {
		_, err := ParseClaimRequestYAML([]byte(head + task + "  principal:\n" + runtime))
		assertClaimRequestValidationError(t, err, ValidationCategorySelfAssertedPrincipal, "spec.principal")

		_, err = ParseClaimRequestYAML([]byte(head + task + "  secrets:\n" + runtime))
		assertClaimRequestValidationError(t, err, ValidationCategorySecretValue, "spec.secrets")
	})

	t.Run("null optional scalar is empty", func(t *testing.T) {
		request, err := ParseClaimRequestYAML([]byte(head + task + "  requestedAccess:\n    modelProfile:\n" + runtime))
		if err != nil {
			t.Fatalf("null modelProfile must be valid, got %#v", err)
		}
		if request.Spec.RequestedAccess.ModelProfile != "" {
			t.Errorf("modelProfile = %q, want empty", request.Spec.RequestedAccess.ModelProfile)
		}
	})

	// A null-valued required field is semantically missing, so it must reach
	// the semantic check as required-field rather than fail as a shape error.
	t.Run("null required fields report required-field", func(t *testing.T) {
		cases := map[string]struct {
			doc  string
			path string
		}{
			"whole task":     {head + "  task:\n" + runtime, "spec.task"},
			"whole runtime":  {head + task + "  runtime:\n", "spec.runtime"},
			"metadata name":  {"apiVersion: agenova.io/v1alpha1\nkind: ClaimRequest\nmetadata:\n  name:\nspec:\n  templateRef: engineer\n" + task + runtime, "metadata.name"},
			"templateRef":    {"apiVersion: agenova.io/v1alpha1\nkind: ClaimRequest\nmetadata:\n  name: fix-payment-timeout\nspec:\n  templateRef:\n" + task + runtime, "spec.templateRef"},
			"task type":      {head + "  task:\n    type:\n" + runtime, "spec.task.type"},
			"profileRef":     {head + task + "  runtime:\n    profileRef:\n    timeout: 20m\n", "spec.runtime.profileRef"},
			"timeout":        {head + task + "  runtime:\n    profileRef: standard-isolated\n    timeout:\n", "spec.runtime.timeout"},
			"apiVersion":     {"apiVersion:\nkind: ClaimRequest\nmetadata:\n  name: fix-payment-timeout\nspec:\n  templateRef: engineer\n" + task + runtime, "apiVersion"},
			"kind":           {"apiVersion: agenova.io/v1alpha1\nkind:\nmetadata:\n  name: fix-payment-timeout\nspec:\n  templateRef: engineer\n" + task + runtime, "kind"},
			"whole metadata": {"apiVersion: agenova.io/v1alpha1\nkind: ClaimRequest\nmetadata:\nspec:\n  templateRef: engineer\n" + task + runtime, "metadata.name"},
			"whole spec":     {"apiVersion: agenova.io/v1alpha1\nkind: ClaimRequest\nmetadata:\n  name: fix-payment-timeout\nspec:\n", "spec.templateRef"},
		}
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := ParseClaimRequestYAML([]byte(tc.doc))
				assertClaimRequestValidationError(t, err, ValidationCategoryRequiredField, tc.path)
			})
		}
	})
}

// The canonical YAML surface must survive a marshal-and-reparse cycle too,
// otherwise Duration's YAML encoding could regress unnoticed.
func TestClaimRequestYAMLRoundTrip(t *testing.T) {
	for _, fixture := range loadClaimRequestCases(t) {
		if fixture.Expected.Outcome != "valid" || fixture.Format != "yaml" {
			continue
		}
		original, validationErr := parseClaimRequestFixture(t, fixture)
		if validationErr != nil {
			t.Fatalf("parse %s: %#v", fixture.ID, validationErr)
		}

		encoded, err := yaml.Marshal(original)
		if err != nil {
			t.Fatalf("marshal %s: %v", fixture.ID, err)
		}
		reparsed, validationErr := ParseClaimRequestYAML(encoded)
		if validationErr != nil {
			t.Fatalf("re-parsing our own YAML failed: %#v\nencoded:\n%s", validationErr, encoded)
		}
		if !reflect.DeepEqual(original, reparsed) {
			t.Errorf("round trip changed the request:\nbefore: %#v\nafter:  %#v", original, reparsed)
		}
	}
}

// Task input is JSON-compatible structured data. The paired documents author
// numbers with identical spellings on both surfaces, and equality is direct
// value comparison — a numeric-tolerant comparison could hide precision loss.
func TestClaimRequestStructuredTaskInput(t *testing.T) {
	yamlDoc := []byte(`apiVersion: agenova.io/v1alpha1
kind: ClaimRequest
metadata:
  name: fix-payment-timeout
spec:
  templateRef: engineer
  task:
    type: repository-change
    input:
      repository: acme/payments
      retries: 3
      threshold: 0.5
      dryRun: true
      note: null
      steps:
        - build
        - test
      limits:
        memoryMi: 512
  runtime:
    profileRef: standard-isolated
    timeout: 20m
`)
	jsonDoc := []byte(`{
  "apiVersion": "agenova.io/v1alpha1",
  "kind": "ClaimRequest",
  "metadata": {"name": "fix-payment-timeout"},
  "spec": {
    "templateRef": "engineer",
    "task": {
      "type": "repository-change",
      "input": {
        "repository": "acme/payments",
        "retries": 3,
        "threshold": 0.5,
        "dryRun": true,
        "note": null,
        "steps": ["build", "test"],
        "limits": {"memoryMi": 512}
      }
    },
    "runtime": {"profileRef": "standard-isolated", "timeout": "20m"}
  }
}`)

	fromYAML, validationErr := ParseClaimRequestYAML(yamlDoc)
	if validationErr != nil {
		t.Fatalf("parse YAML surface: %#v", validationErr)
	}
	fromJSON, validationErr := ParseClaimRequestJSON(jsonDoc)
	if validationErr != nil {
		t.Fatalf("parse JSON surface: %#v", validationErr)
	}
	if !reflect.DeepEqual(fromYAML.Spec.Task.Input, fromJSON.Spec.Task.Input) {
		t.Fatalf("structured task input is not semantically equal across surfaces:\nYAML: %#v\nJSON: %#v", fromYAML.Spec.Task.Input, fromJSON.Spec.Task.Input)
	}

	want := map[string]any{
		"repository": "acme/payments",
		"retries":    3,
		"threshold":  0.5,
		"dryRun":     true,
		"note":       nil,
		"steps":      []any{"build", "test"},
		"limits":     map[string]any{"memoryMi": 512},
	}
	if !reflect.DeepEqual(fromYAML.Spec.Task.Input, want) {
		t.Fatalf("decoded task input = %#v, want %#v", fromYAML.Spec.Task.Input, want)
	}
}

// YAML constructs with no consistent JSON representation fail closed on the
// document tree, before decoding can erase them.
func TestParseClaimRequestRejectsNonJSONTaskInput(t *testing.T) {
	docWithInput := func(inputLines string) []byte {
		return []byte("apiVersion: agenova.io/v1alpha1\nkind: ClaimRequest\nmetadata:\n  name: x\nspec:\n  templateRef: engineer\n  task:\n    type: repository-change\n    input:\n" + inputLines + "  runtime:\n    profileRef: standard-isolated\n    timeout: 20m\n")
	}

	cases := map[string]struct {
		doc  []byte
		path string
	}{
		"non-string key":  {docWithInput("      1: numeric-key\n"), "spec.task.input"},
		"duplicate key":   {docWithInput("      repo: a\n      repo: b\n"), "spec.task.input.repo"},
		"alias value":     {docWithInput("      a: &anchor x\n      b: *anchor\n"), "spec.task.input.b"},
		"merge key":       {docWithInput("      base: &anchor {x: 1}\n      <<: *anchor\n"), "spec.task.input"},
		"binary tag":      {docWithInput("      blob: !!binary aGk=\n"), "spec.task.input.blob"},
		"timestamp tag":   {docWithInput("      at: !!timestamp 2026-01-01T00:00:00Z\n"), "spec.task.input.at"},
		"custom tag":      {docWithInput("      custom: !mytag value\n"), "spec.task.input.custom"},
		"nan float":       {docWithInput("      bad: .nan\n"), "spec.task.input.bad"},
		"infinite float":  {docWithInput("      bad: .inf\n"), "spec.task.input.bad"},
		"nested bad item": {docWithInput("      steps:\n        - ok\n        - !!binary aGk=\n"), "spec.task.input.steps[1]"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseClaimRequestYAML(tc.doc)
			assertClaimRequestValidationError(t, err, ValidationCategoryInvalidDocument, tc.path)
		})
	}
}

// The same JSON-compatibility invariants protect directly constructed
// requests: the public contract fails closed, not only the parser.
func TestValidateClaimRequestRejectsNonJSONTaskInputValues(t *testing.T) {
	cyclicSlice := make([]any, 1)
	cyclicSlice[0] = cyclicSlice
	cyclicMap := map[string]any{}
	cyclicMap["self"] = cyclicMap

	cases := map[string]struct {
		input map[string]any
		path  string
	}{
		"NaN float":          {map[string]any{"bad": math.NaN()}, "spec.task.input.bad"},
		"infinite float":     {map[string]any{"bad": math.Inf(1)}, "spec.task.input.bad"},
		"time value":         {map[string]any{"at": time.Now()}, "spec.task.input.at"},
		"binary value":       {map[string]any{"blob": []byte("hi")}, "spec.task.input.blob"},
		"nested bad item":    {map[string]any{"steps": []any{"ok", time.Now()}}, "spec.task.input.steps[1]"},
		"nested bad map":     {map[string]any{"limits": map[string]any{"cpu": math.NaN()}}, "spec.task.input.limits.cpu"},
		"non-string-key map": {map[string]any{"byID": map[int]string{1: "x"}}, "spec.task.input.byID"},
		"cyclic slice":       {map[string]any{"loop": cyclicSlice}, "spec.task.input.loop[0]"},
		"cyclic map":         {map[string]any{"loop": cyclicMap}, "spec.task.input.loop.self"},
		"typed bad element":  {map[string]any{"times": []time.Time{time.Now()}}, "spec.task.input.times[0]"},
		"pointer value":      {map[string]any{"ref": new(int)}, "spec.task.input.ref"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			request := validClaimRequest()
			request.Spec.Task.Input = tc.input
			assertClaimRequestValidationError(t, ValidateClaimRequest(request), ValidationCategoryInvalidValue, tc.path)
		})
	}
}

// Typed Go containers are JSON-compatible when their shape is: the validator
// walks slices, arrays, and string-keyed maps by kind, so direct callers are
// not forced to convert everything to []any / map[string]any first. A shared
// (non-cyclic) container referenced from two sibling paths stays valid.
func TestValidateClaimRequestAcceptsTypedTaskInputContainers(t *testing.T) {
	shared := []string{"build", "test"}
	request := validClaimRequest()
	request.Spec.Task.Input = map[string]any{
		"steps":   shared,
		"again":   shared,
		"labels":  map[string]string{"team": "a"},
		"matrix":  [2]int{1, 2},
		"weights": []float64{0.5, 1},
		"nested":  map[string]any{"inner": []any{map[string]string{"k": "v"}}},
	}
	if err := ValidateClaimRequest(request); err != nil {
		t.Fatalf("typed JSON-compatible containers must be valid, got %#v", err)
	}
}

func TestValidateClaimRequestRejectsNonPositiveTimeout(t *testing.T) {
	for _, d := range []time.Duration{0, -5 * time.Minute} {
		t.Run(d.String(), func(t *testing.T) {
			request := validClaimRequest()
			timeout := Duration(d)
			request.Spec.Runtime.Timeout = &timeout
			assertClaimRequestValidationError(t, ValidateClaimRequest(request), ValidationCategoryInvalidValue, "spec.runtime.timeout")
		})
	}
}

func TestParseClaimRequestFailsClosed(t *testing.T) {
	t.Run("multiple YAML documents", func(t *testing.T) {
		_, err := ParseClaimRequestYAML([]byte("apiVersion: agenova.io/v1alpha1\n---\napiVersion: agenova.io/v1alpha1\n"))
		if err == nil || err.Category != ValidationCategoryInvalidDocument {
			t.Fatalf("error = %#v, want invalid-document", err)
		}
	})
	t.Run("empty document", func(t *testing.T) {
		_, err := ParseClaimRequestYAML(nil)
		if err == nil || err.Category != ValidationCategoryInvalidDocument {
			t.Fatalf("error = %#v, want invalid-document", err)
		}
	})
	t.Run("JSON surface rejects non-JSON input", func(t *testing.T) {
		_, err := ParseClaimRequestJSON([]byte("apiVersion: agenova.io/v1alpha1\nkind: ClaimRequest\n"))
		if err == nil || err.Category != ValidationCategoryInvalidDocument {
			t.Fatalf("error = %#v, want invalid-document", err)
		}
	})
	t.Run("unknown field fails closed", func(t *testing.T) {
		_, err := ParseClaimRequestYAML([]byte("apiVersion: agenova.io/v1alpha1\nkind: ClaimRequest\nmetadata:\n  name: x\nspec:\n  templateRef: engineer\n  workflow: dag\n"))
		if err == nil || err.Category != ValidationCategoryUnknownField || err.FieldPath != "spec.workflow" {
			t.Fatalf("error = %#v, want unknown-field at spec.workflow", err)
		}
	})
	t.Run("reserved principal path on the YAML surface", func(t *testing.T) {
		_, err := ParseClaimRequestYAML([]byte("apiVersion: agenova.io/v1alpha1\nkind: ClaimRequest\nmetadata:\n  name: x\nspec:\n  principal:\n    subject: user:attacker\n"))
		if err == nil || err.Category != ValidationCategorySelfAssertedPrincipal || err.FieldPath != "spec.principal" {
			t.Fatalf("error = %#v, want self-asserted-principal at spec.principal", err)
		}
	})
	t.Run("reserved secrets path on the YAML surface", func(t *testing.T) {
		_, err := ParseClaimRequestYAML([]byte("apiVersion: agenova.io/v1alpha1\nkind: ClaimRequest\nmetadata:\n  name: x\nspec:\n  secrets:\n    githubToken: value\n"))
		if err == nil || err.Category != ValidationCategorySecretValue || err.FieldPath != "spec.secrets" {
			t.Fatalf("error = %#v, want secret-value at spec.secrets", err)
		}
	})
	t.Run("non-positive timeout", func(t *testing.T) {
		_, err := ParseClaimRequestYAML([]byte("apiVersion: agenova.io/v1alpha1\nkind: ClaimRequest\nmetadata:\n  name: x\nspec:\n  runtime:\n    profileRef: standard-isolated\n    timeout: -5m\n"))
		if err == nil || err.Category != ValidationCategoryInvalidValue || err.FieldPath != "spec.runtime.timeout" {
			t.Fatalf("error = %#v, want invalid-value at spec.runtime.timeout", err)
		}
	})
}
