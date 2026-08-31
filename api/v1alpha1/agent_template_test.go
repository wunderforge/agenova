// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fixtureManifest struct {
	Cases []fixtureCase `json:"cases"`
}

type fixtureCase struct {
	ID       string          `json:"id"`
	Subject  string          `json:"subject"`
	Input    string          `json:"input"`
	Expected fixtureExpected `json:"expected"`
}

type fixtureExpected struct {
	Outcome  string             `json:"outcome"`
	Category ValidationCategory `json:"category,omitempty"`
}

func TestAgentTemplateFixtures(t *testing.T) {
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
		if fixture.Subject != AgentTemplateKind {
			continue
		}
		count++
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join(fixtureRoot, filepath.FromSlash(fixture.Input)))
			if err != nil {
				t.Fatalf("read shared fixture input: %v", err)
			}

			template, validationErr := ParseAgentTemplateYAML(input)
			if fixture.Expected.Outcome == "valid" {
				if validationErr != nil {
					t.Fatalf("ParseAgentTemplateYAML() error = %#v", validationErr)
				}
				assertEngineerTemplate(t, template)
				return
			}
			if validationErr == nil {
				t.Fatalf("ParseAgentTemplateYAML() error = nil, want category %q", fixture.Expected.Category)
			}
			if validationErr.Category != fixture.Expected.Category {
				t.Fatalf("validation category = %q, want %q (path %q)", validationErr.Category, fixture.Expected.Category, validationErr.FieldPath)
			}
			if strings.TrimSpace(validationErr.FieldPath) == "" {
				t.Fatal("validation field path is blank")
			}
		})
	}

	if count != 6 {
		t.Fatalf("AgentTemplate fixture count = %d, want 6", count)
	}
}

func TestValidateAgentTemplateRequiresNonBlankName(t *testing.T) {
	for _, name := range []string{"", "   "} {
		t.Run("name="+name, func(t *testing.T) {
			template := validAgentTemplate()
			template.Metadata.Name = name
			assertValidationError(t, ValidateAgentTemplate(template), ValidationCategoryRequiredField, "metadata.name")
		})
	}
}

func TestValidateAgentTemplateRequiresRunnableArtifactAndEntrypoint(t *testing.T) {
	tests := []struct {
		name string
		edit func(*AgentTemplate)
		path string
	}{
		{
			name: "blank artifact image",
			edit: func(template *AgentTemplate) {
				template.Spec.Artifact.Image = "   "
			},
			path: "spec.artifact.image",
		},
		{
			name: "empty command",
			edit: func(template *AgentTemplate) {
				template.Spec.Entrypoint.Command = nil
			},
			path: "spec.entrypoint.command",
		},
		{
			name: "blank command element",
			edit: func(template *AgentTemplate) {
				template.Spec.Entrypoint.Command = []string{"run", " "}
			},
			path: "spec.entrypoint.command[1]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			template := validAgentTemplate()
			test.edit(template)
			assertValidationError(t, ValidateAgentTemplate(template), ValidationCategoryRequiredField, test.path)
		})
	}
}

func TestValidateAgentTemplateCapabilityCeilingPresenceAndDefaultDeny(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		template := validAgentTemplate()
		template.Spec.CapabilityCeiling = nil
		assertValidationError(t, ValidateAgentTemplate(template), ValidationCategoryRequiredField, "spec.capabilityCeiling")
	})

	t.Run("explicitly empty", func(t *testing.T) {
		template := validAgentTemplate()
		template.Spec.Defaults = AgentTemplateDefaults{}
		template.Spec.CapabilityCeiling = &AgentTemplateCapabilityCeiling{}
		if err := ValidateAgentTemplate(template); err != nil {
			t.Fatalf("ValidateAgentTemplate() error = %#v, want nil", err)
		}
	})
}

func TestValidateAgentTemplateRejectsDefaultsOutsideCeiling(t *testing.T) {
	tests := []struct {
		name string
		edit func(*AgentTemplate)
		path string
	}{
		{
			name: "model profile",
			edit: func(template *AgentTemplate) {
				template.Spec.Defaults.ModelProfile = "unapproved-model"
			},
			path: "spec.defaults.modelProfile",
		},
		{
			name: "memory scope",
			edit: func(template *AgentTemplate) {
				template.Spec.Defaults.MemoryScopes = []string{"private-memory"}
			},
			path: "spec.defaults.memoryScopes[0]",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			template := validAgentTemplate()
			test.edit(template)
			assertValidationError(t, ValidateAgentTemplate(template), ValidationCategoryInvalidCapabilityCeiling, test.path)
		})
	}
}

func TestValidateAgentTemplateRejectsInvalidCapabilityCeilingValues(t *testing.T) {
	tests := []struct {
		name string
		edit func(*AgentTemplate)
		path string
	}{
		{
			name: "blank list value",
			edit: func(template *AgentTemplate) {
				template.Spec.CapabilityCeiling.Tools = []string{"git.read", " "}
			},
			path: "spec.capabilityCeiling.tools[1]",
		},
		{
			name: "duplicate list value",
			edit: func(template *AgentTemplate) {
				template.Spec.CapabilityCeiling.Tools = []string{"git.read", "git.read"}
			},
			path: "spec.capabilityCeiling.tools[1]",
		},
		{
			name: "non-positive timeout",
			edit: func(template *AgentTemplate) {
				zero := Duration(0)
				template.Spec.CapabilityCeiling.MaxTimeout = &zero
			},
			path: "spec.capabilityCeiling.maxTimeout",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			template := validAgentTemplate()
			test.edit(template)
			assertValidationError(t, ValidateAgentTemplate(template), ValidationCategoryInvalidCapabilityCeiling, test.path)
		})
	}
}

func TestParseAgentTemplateYAMLClassifiesReservedFieldsByExactPath(t *testing.T) {
	tests := []struct {
		name     string
		extra    string
		category ValidationCategory
		path     string
	}{
		{
			name:     "issued authority",
			extra:    "  effectiveAuthority: {}\n",
			category: ValidationCategorySystemManagedField,
			path:     "spec.effectiveAuthority",
		},
		{
			name:     "credential-bearing environment",
			extra:    "  environment: {}\n",
			category: ValidationCategorySecretValue,
			path:     "spec.environment",
		},
		{
			name:     "other credential-like field",
			extra:    "  credentialToken: value\n",
			category: ValidationCategoryUnknownField,
			path:     "spec.credentialToken",
		},
		{
			name:     "similar authority field",
			extra:    "  effectiveAuthorityHint: value\n",
			category: ValidationCategoryUnknownField,
			path:     "spec.effectiveAuthorityHint",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseAgentTemplateYAML(minimalAgentTemplateYAML(test.extra))
			assertValidationError(t, err, test.category, test.path)
		})
	}
}

func TestParseAgentTemplateYAMLDoesNotScanValuesForSecrets(t *testing.T) {
	input := strings.Replace(string(minimalAgentTemplateYAML("")), "example/agent:v0", "example/token-agent:v0", 1)
	if _, err := ParseAgentTemplateYAML([]byte(input)); err != nil {
		t.Fatalf("ParseAgentTemplateYAML() error = %#v, want nil", err)
	}
}

func TestParseAgentTemplateYAMLRejectsUnknownNestedField(t *testing.T) {
	input := `apiVersion: agenova.io/v1alpha1
kind: AgentTemplate
metadata:
  name: engineer
spec:
  artifact:
    image: example/agent:v0
  entrypoint:
    command: [run]
  defaults:
    environment: token
  capabilityCeiling: {}
`
	_, err := ParseAgentTemplateYAML([]byte(input))
	assertValidationError(t, err, ValidationCategoryUnknownField, "spec.defaults.environment")
}

func TestParseAgentTemplateYAMLRejectsMultipleDocuments(t *testing.T) {
	input := append(minimalAgentTemplateYAML(""), []byte("---\n")...)
	input = append(input, minimalAgentTemplateYAML("")...)
	_, err := ParseAgentTemplateYAML(input)
	assertValidationError(t, err, ValidationCategoryInvalidDocument, "$")
}

func assertEngineerTemplate(t *testing.T, template *AgentTemplate) {
	t.Helper()
	if template.APIVersion != AgentTemplateAPIVersion || template.Kind != AgentTemplateKind {
		t.Fatalf("type metadata = %q/%q", template.APIVersion, template.Kind)
	}
	if template.Metadata.Name != "engineer" {
		t.Errorf("metadata.name = %q, want engineer", template.Metadata.Name)
	}
	if template.Spec.Artifact == nil || template.Spec.Artifact.Image != "ghcr.io/wunderforge/agenova-engineer:v0" {
		t.Errorf("artifact = %#v", template.Spec.Artifact)
	}
	if template.Spec.Entrypoint == nil || strings.Join(template.Spec.Entrypoint.Command, " ") != "/agenova-agent run" {
		t.Errorf("entrypoint = %#v", template.Spec.Entrypoint)
	}
	if template.Spec.Defaults.ModelProfile != "approved-coding-model" || strings.Join(template.Spec.Defaults.MemoryScopes, ",") != "team-docs" {
		t.Errorf("defaults = %#v", template.Spec.Defaults)
	}
	ceiling := template.Spec.CapabilityCeiling
	if ceiling == nil {
		t.Fatal("capability ceiling is nil")
	}
	if strings.Join(ceiling.Tools, ",") != "git.read,git.write,github.pull-request" {
		t.Errorf("tools = %#v", ceiling.Tools)
	}
	if strings.Join(ceiling.ResourceScopes, ",") != "repo:acme/*" {
		t.Errorf("resource scopes = %#v", ceiling.ResourceScopes)
	}
	if strings.Join(ceiling.ModelProfiles, ",") != "approved-coding-model" {
		t.Errorf("model profiles = %#v", ceiling.ModelProfiles)
	}
	if strings.Join(ceiling.MemoryScopes, ",") != "team-docs" {
		t.Errorf("memory scopes = %#v", ceiling.MemoryScopes)
	}
	if strings.Join(ceiling.RuntimeProfiles, ",") != "standard-isolated" {
		t.Errorf("runtime profiles = %#v", ceiling.RuntimeProfiles)
	}
	if ceiling.MaxTimeout == nil || time.Duration(*ceiling.MaxTimeout) != 30*time.Minute {
		t.Errorf("max timeout = %#v", ceiling.MaxTimeout)
	}
}

func assertValidationError(t *testing.T, err *ValidationError, category ValidationCategory, path string) {
	t.Helper()
	if err == nil {
		t.Fatalf("validation error = nil, want category %q at %q", category, path)
	}
	if err.Category != category || err.FieldPath != path {
		t.Fatalf("validation error = %#v, want category %q at %q", err, category, path)
	}
}

func validAgentTemplate() *AgentTemplate {
	timeout := Duration(30 * time.Minute)
	return &AgentTemplate{
		APIVersion: AgentTemplateAPIVersion,
		Kind:       AgentTemplateKind,
		Metadata:   ObjectMeta{Name: "engineer"},
		Spec: AgentTemplateSpec{
			Artifact:   &AgentTemplateArtifact{Image: "example/agent:v0"},
			Entrypoint: &AgentTemplateEntrypoint{Command: []string{"run"}},
			Defaults: AgentTemplateDefaults{
				ModelProfile: "approved-model",
				MemoryScopes: []string{"team-memory"},
			},
			CapabilityCeiling: &AgentTemplateCapabilityCeiling{
				ModelProfiles: []string{"approved-model"},
				MemoryScopes:  []string{"team-memory"},
				MaxTimeout:    &timeout,
			},
		},
	}
}

func minimalAgentTemplateYAML(specExtra string) []byte {
	return []byte(`apiVersion: agenova.io/v1alpha1
kind: AgentTemplate
metadata:
  name: engineer
spec:
  artifact:
    image: example/agent:v0
  entrypoint:
    command: [run]
  capabilityCeiling: {}
` + specExtra)
}
