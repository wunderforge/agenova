// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package contractv0

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const fixtureSchemaVersion = "agenova.contract-fixtures/v0"

var (
	caseIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:[.-][a-z0-9]+)*$`)
	providerTerms = regexp.MustCompile(`(?i)(kubernetes|agents\.x-k8s\.io|\bpod\b|\bnamespace\b|runtimeclass|\be2b\b|daytona|fargate)`)
	secretTerms   = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bghp_[a-z0-9]{20,}\b`),
		regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`),
		regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----`),
	}
)

type fixtureManifest struct {
	SchemaVersion string        `json:"schemaVersion"`
	Cases         []fixtureCase `json:"cases"`
}

type fixtureCase struct {
	ID           string          `json:"id"`
	Subject      string          `json:"subject"`
	Purpose      string          `json:"purpose"`
	Input        string          `json:"input"`
	Format       string          `json:"format"`
	Coverage     []string        `json:"coverage"`
	EquivalentTo string          `json:"equivalentTo,omitempty"`
	Expected     fixtureExpected `json:"expected"`
}

type fixtureExpected struct {
	Outcome  string `json:"outcome"`
	Category string `json:"category,omitempty"`
}

func TestFixtureInventory(t *testing.T) {
	t.Parallel()

	if err := validateFixtureTree("."); err != nil {
		t.Fatal(err)
	}
}

func TestFixtureInventoryRejectsBrokenContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		want   string
		mutate func(*testing.T, string)
	}{
		{
			name: "duplicate case ID",
			want: "duplicate case ID",
			mutate: func(t *testing.T, root string) {
				manifest := readManifestForMutation(t, root)
				manifest.Cases = append(manifest.Cases, manifest.Cases[0])
				writeManifest(t, root, manifest)
			},
		},
		{
			name: "missing expected outcome",
			want: "expected outcome",
			mutate: func(t *testing.T, root string) {
				manifest := readManifestForMutation(t, root)
				manifest.Cases[0].Expected.Outcome = ""
				writeManifest(t, root, manifest)
			},
		},
		{
			name: "unreadable input",
			want: "parse fixture",
			mutate: func(t *testing.T, root string) {
				writeFixture(t, root, "inputs/agent-template/valid-engineer.yaml", []byte("spec: [broken"))
			},
		},
		{
			name: "missing required coverage",
			want: "required coverage Decision",
			mutate: func(t *testing.T, root string) {
				manifest := readManifestForMutation(t, root)
				for i := range manifest.Cases {
					manifest.Cases[i].Coverage = removeString(manifest.Cases[i].Coverage, "Decision")
				}
				writeManifest(t, root, manifest)
			},
		},
		{
			name: "canonical encoding drift",
			want: "equivalent fixtures differ",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, filepath.FromSlash("inputs/claim-request/valid-team-a-engineer.yaml"))
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				raw = bytes.Replace(raw, []byte("Fix the payment timeout bug"), []byte("Change an unrelated behavior"), 1)
				if err := os.WriteFile(path, raw, 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "provider vocabulary",
			want: "provider vocabulary",
			mutate: func(t *testing.T, root string) {
				writeFixture(t, root, "inputs/agent-template/valid-engineer.yaml", []byte("provider: kubernetes\n"))
			},
		},
		{
			name: "secret-like value",
			want: "secret-like value",
			mutate: func(t *testing.T, root string) {
				writeFixture(t, root, "inputs/agent-template/valid-engineer.yaml", []byte("token: ghp_01234567890123456789\n"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			copyFixtureTree(t, root)
			tt.mutate(t, root)

			err := validateFixtureTree(root)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateFixtureTree() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func validateFixtureTree(root string) error {
	manifestPath := filepath.Join(root, "manifest.json")
	rawManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read fixture manifest: %w", err)
	}

	var manifest fixtureManifest
	if err := decodeJSON(rawManifest, &manifest); err != nil {
		return fmt.Errorf("parse fixture manifest: %w", err)
	}
	if manifest.SchemaVersion != fixtureSchemaVersion {
		return fmt.Errorf("schemaVersion = %q, want %q", manifest.SchemaVersion, fixtureSchemaVersion)
	}
	if len(manifest.Cases) == 0 {
		return errors.New("fixture manifest has no cases")
	}

	allowedSubjects := stringSet("AgentTemplate", "ClaimRequest", "IssuedState")
	allowedCoverage := stringSet("AgentTemplate", "Principal", "Action", "ClaimRequest", "PolicyReference", "EffectiveAuthority", "SandboxClaim", "Decision", "Evidence")
	allowedCategories := stringSet("required-field", "invalid-capability-ceiling", "system-managed-field", "secret-value", "self-asserted-principal")
	seenIDs := map[string]struct{}{}
	seenInputs := map[string]string{}
	caseByID := map[string]fixtureCase{}
	decodedByID := map[string]any{}
	coverage := map[string]struct{}{}

	for _, fixture := range manifest.Cases {
		if !caseIDPattern.MatchString(fixture.ID) {
			return fmt.Errorf("invalid case ID %q", fixture.ID)
		}
		if _, ok := seenIDs[fixture.ID]; ok {
			return fmt.Errorf("duplicate case ID %q", fixture.ID)
		}
		seenIDs[fixture.ID] = struct{}{}
		caseByID[fixture.ID] = fixture

		if _, ok := allowedSubjects[fixture.Subject]; !ok {
			return fmt.Errorf("case %s has unsupported subject %q", fixture.ID, fixture.Subject)
		}
		if strings.TrimSpace(fixture.Purpose) == "" {
			return fmt.Errorf("case %s has empty purpose", fixture.ID)
		}
		if len(fixture.Coverage) == 0 {
			return fmt.Errorf("case %s has empty coverage", fixture.ID)
		}
		localCoverage := map[string]struct{}{}
		for _, shape := range fixture.Coverage {
			if _, ok := allowedCoverage[shape]; !ok {
				return fmt.Errorf("case %s has unsupported coverage %q", fixture.ID, shape)
			}
			if _, ok := localCoverage[shape]; ok {
				return fmt.Errorf("case %s repeats coverage %q", fixture.ID, shape)
			}
			localCoverage[shape] = struct{}{}
			coverage[shape] = struct{}{}
		}

		if err := validateExpected(fixture); err != nil {
			return err
		}
		if fixture.Expected.Category != "" {
			if _, ok := allowedCategories[fixture.Expected.Category]; !ok {
				return fmt.Errorf("case %s has unsupported error category %q", fixture.ID, fixture.Expected.Category)
			}
		}

		cleanInput, err := cleanRelativeInput(fixture.Input)
		if err != nil {
			return fmt.Errorf("case %s: %w", fixture.ID, err)
		}
		if previous, ok := seenInputs[cleanInput]; ok {
			return fmt.Errorf("cases %s and %s share input path %q", previous, fixture.ID, cleanInput)
		}
		seenInputs[cleanInput] = fixture.ID
		if err := validateFormatExtension(fixture.Format, cleanInput); err != nil {
			return fmt.Errorf("case %s: %w", fixture.ID, err)
		}

		inputPath := filepath.Join(root, filepath.FromSlash(cleanInput))
		rawInput, err := os.ReadFile(inputPath)
		if err != nil {
			return fmt.Errorf("read fixture %s: %w", fixture.ID, err)
		}
		if err := validateFixtureBytes(fixture.ID, rawInput); err != nil {
			return err
		}
		decoded, err := decodeFixture(fixture.Format, rawInput)
		if err != nil {
			return fmt.Errorf("parse fixture %s: %w", fixture.ID, err)
		}
		if _, ok := decoded.(map[string]any); !ok {
			return fmt.Errorf("fixture %s must decode to an object", fixture.ID)
		}
		decodedByID[fixture.ID] = decoded
	}

	for _, id := range requiredCaseIDs() {
		if _, ok := seenIDs[id]; !ok {
			return fmt.Errorf("required case %s is missing", id)
		}
	}
	for shape := range allowedCoverage {
		if _, ok := coverage[shape]; !ok {
			return fmt.Errorf("required coverage %s is missing", shape)
		}
	}
	for _, fixture := range manifest.Cases {
		if fixture.EquivalentTo == "" {
			continue
		}
		other, ok := caseByID[fixture.EquivalentTo]
		if !ok {
			return fmt.Errorf("case %s references unknown equivalent case %s", fixture.ID, fixture.EquivalentTo)
		}
		if fixture.Expected.Outcome != "valid" || other.Expected.Outcome != "valid" {
			return fmt.Errorf("equivalent cases %s and %s must both be valid", fixture.ID, fixture.EquivalentTo)
		}
		if fixture.Format == other.Format {
			return fmt.Errorf("equivalent cases %s and %s must use different formats", fixture.ID, fixture.EquivalentTo)
		}
		if !reflect.DeepEqual(decodedByID[fixture.ID], decodedByID[fixture.EquivalentTo]) {
			return fmt.Errorf("equivalent fixtures differ: %s and %s", fixture.ID, fixture.EquivalentTo)
		}
	}

	return validateScenarioInvariants(decodedByID)
}

func validateExpected(fixture fixtureCase) error {
	switch fixture.Expected.Outcome {
	case "valid":
		if fixture.Expected.Category != "" {
			return fmt.Errorf("valid case %s must not have an error category", fixture.ID)
		}
	case "invalid":
		if strings.TrimSpace(fixture.Expected.Category) == "" {
			return fmt.Errorf("invalid case %s has no expected error category", fixture.ID)
		}
	default:
		return fmt.Errorf("case %s has invalid or missing expected outcome %q", fixture.ID, fixture.Expected.Outcome)
	}
	return nil
}

func cleanRelativeInput(input string) (string, error) {
	if input == "" {
		return "", errors.New("input path is empty")
	}
	if filepath.IsAbs(input) {
		return "", fmt.Errorf("input path %q must be relative", input)
	}
	clean := filepath.Clean(filepath.FromSlash(input))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("input path %q escapes the fixture root", input)
	}
	return filepath.ToSlash(clean), nil
}

func validateFormatExtension(format, input string) error {
	ext := strings.ToLower(filepath.Ext(input))
	switch format {
	case "json":
		if ext != ".json" {
			return fmt.Errorf("json input %q must use .json", input)
		}
	case "yaml":
		if ext != ".yaml" && ext != ".yml" {
			return fmt.Errorf("yaml input %q must use .yaml or .yml", input)
		}
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
	return nil
}

func validateFixtureBytes(caseID string, raw []byte) error {
	if match := providerTerms.Find(raw); match != nil {
		return fmt.Errorf("fixture %s contains provider vocabulary %q", caseID, match)
	}
	for _, pattern := range secretTerms {
		if pattern.Match(raw) {
			return fmt.Errorf("fixture %s contains a secret-like value", caseID)
		}
	}
	return nil
}

func decodeFixture(format string, raw []byte) (any, error) {
	var decoded any
	switch format {
	case "json":
		if err := decodeJSON(raw, &decoded); err != nil {
			return nil, err
		}
	case "yaml":
		if err := yaml.Unmarshal(raw, &decoded); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}

	normalized, err := json.Marshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("normalize %s: %w", format, err)
	}
	var result any
	if err := decodeJSON(normalized, &result); err != nil {
		return nil, fmt.Errorf("decode normalized %s: %w", format, err)
	}
	return result, nil
}

func decodeJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func validateScenarioInvariants(decoded map[string]any) error {
	request := decoded["claim-request.valid.team-a-engineer-json"].(map[string]any)
	spec := request["spec"].(map[string]any)
	for _, forbidden := range []string{"principal", "effectiveAuthority", "claim", "backendIdentity", "secrets"} {
		if _, ok := spec[forbidden]; ok {
			return fmt.Errorf("canonical ClaimRequest contains forbidden field %s", forbidden)
		}
	}

	issued := decoded["issued-state.valid.team-a-engineer"].(map[string]any)
	if err := requireDecisionResult(issued, "Allow"); err != nil {
		return fmt.Errorf("Team A issued state: %w", err)
	}

	denial := decoded["issued-state.valid.team-b-denial"].(map[string]any)
	for _, forbidden := range []string{"claim", "effectiveAuthority", "backendIdentity"} {
		if _, ok := denial[forbidden]; ok {
			return fmt.Errorf("pre-claim denial contains fabricated field %s", forbidden)
		}
	}
	if err := requireDecisionResult(denial, "Deny"); err != nil {
		return fmt.Errorf("pre-claim denial: %w", err)
	}
	evidence := denial["evidence"].(map[string]any)
	for _, forbidden := range []string{"claimId", "backendIdentity"} {
		if _, ok := evidence[forbidden]; ok {
			return fmt.Errorf("pre-claim denial evidence contains fabricated field %s", forbidden)
		}
	}

	return nil
}

func requireDecisionResult(state map[string]any, want string) error {
	decision, ok := state["decision"].(map[string]any)
	if !ok {
		return errors.New("decision must be an object")
	}
	if _, ok := decision["allowed"]; ok {
		return errors.New("decision must use typed result instead of allowed boolean")
	}
	result, ok := decision["result"].(string)
	if !ok || result != want {
		return fmt.Errorf("decision result = %v, want %s", decision["result"], want)
	}
	return nil
}

func requiredCaseIDs() []string {
	ids := []string{
		"agent-template.valid.engineer",
		"agent-template.invalid.missing-artifact",
		"agent-template.invalid.missing-entrypoint",
		"agent-template.invalid.capability-ceiling",
		"agent-template.invalid.issued-authority",
		"agent-template.invalid.secret-value",
		"claim-request.valid.team-a-engineer-json",
		"claim-request.valid.team-a-engineer-yaml",
		"claim-request.invalid.missing-template",
		"claim-request.invalid.missing-task",
		"claim-request.invalid.missing-runtime",
		"claim-request.invalid.self-asserted-principal",
		"claim-request.invalid.secret-value",
		"issued-state.valid.team-a-engineer",
		"issued-state.valid.team-b-denial",
		"issued-state.invalid.caller-effective-authority",
		"issued-state.invalid.caller-claim-phase",
		"issued-state.invalid.caller-backend-identity",
	}
	sort.Strings(ids)
	return ids
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func removeString(values []string, remove string) []string {
	result := values[:0]
	for _, value := range values {
		if value != remove {
			result = append(result, value)
		}
	}
	return result
}

func copyFixtureTree(t *testing.T, destination string) {
	t.Helper()

	rawManifest, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "manifest.json"), rawManifest, 0o600); err != nil {
		t.Fatal(err)
	}

	err = filepath.WalkDir("inputs", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		target := filepath.Join(destination, path)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func readManifestForMutation(t *testing.T, root string) fixtureManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest fixtureManifest
	if err := decodeJSON(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func writeManifest(t *testing.T, root string, manifest fixtureManifest) {
	t.Helper()
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeFixture(t *testing.T, root, relative string, raw []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
