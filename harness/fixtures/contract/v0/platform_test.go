// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package contractv0

import (
	"os"
	"reflect"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	platformresolver "github.com/wunderforge/agenova/internal/platform"
)

type platformFixtureLookup map[string]platformresolver.Descriptor

func (l platformFixtureLookup) Lookup(id, version string) (platformresolver.Descriptor, bool) {
	descriptor, ok := l[id+"@"+version]
	return descriptor, ok
}

func TestPlatformFixturesRoundTripAndResolve(t *testing.T) {
	yamlRaw := readPlatformFixture(t, "inputs/platform/valid-reference.yaml")
	jsonRaw := readPlatformFixture(t, "inputs/platform/valid-reference.json")
	fromYAML, yamlErr := v1alpha1.ParsePlatformYAML(yamlRaw)
	fromJSON, jsonErr := v1alpha1.ParsePlatformJSON(jsonRaw)
	if yamlErr != nil || jsonErr != nil {
		t.Fatalf("parse errors: YAML=%v JSON=%v", yamlErr, jsonErr)
	}
	if !reflect.DeepEqual(fromYAML, fromJSON) {
		t.Fatalf("equivalent fixtures differ:\nYAML=%#v\nJSON=%#v", fromYAML, fromJSON)
	}
	resolved, lock, resolveErr := platformresolver.Resolve(fromYAML, platformFixtureDescriptors())
	if resolveErr != nil {
		t.Fatalf("Resolve() error = %v", resolveErr)
	}
	const expectedRevision = "sha256:9e5a26c12cae8d966794f27daafff70d7a770f62935e50879a5892964bdc1090"
	if resolved.Revision != expectedRevision || lock.Revision != expectedRevision {
		t.Fatalf("revisions = %q, %q; want %q", resolved.Revision, lock.Revision, expectedRevision)
	}
}

func TestPlatformFixturesRejectNamedNegativeCases(t *testing.T) {
	t.Run("credential reference", func(t *testing.T) {
		_, err := v1alpha1.ParsePlatformYAML(readPlatformFixture(t, "inputs/platform/invalid-credential-ref.yaml"))
		if err == nil || err.Category != v1alpha1.ValidationCategorySecretValue {
			t.Fatalf("error = %#v", err)
		}
	})
	t.Run("unknown backend", func(t *testing.T) {
		_, err := v1alpha1.ParsePlatformYAML(readPlatformFixture(t, "inputs/platform/invalid-unknown-backend.yaml"))
		if err == nil || err.Category != v1alpha1.ValidationCategoryInvalidValue {
			t.Fatalf("error = %#v", err)
		}
	})
	t.Run("invalid runtime pair", func(t *testing.T) {
		input, parseErr := v1alpha1.ParsePlatformYAML(readPlatformFixture(t, "inputs/platform/invalid-runtime-pair.yaml"))
		if parseErr != nil {
			t.Fatalf("parse error = %v", parseErr)
		}
		resolved, lock, err := platformresolver.Resolve(input, platformFixtureDescriptors())
		if resolved != nil || lock != nil || err == nil || err.Category != platformresolver.ErrorInvalidConfig {
			t.Fatalf("Resolve() = %#v, %#v, %#v", resolved, lock, err)
		}
	})
}

func readPlatformFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func platformFixtureDescriptors() platformFixtureLookup {
	identityInstance := func(_ platformresolver.Capability, config map[string]any) (map[string]any, error) { return config, nil }
	identityProfile := func(_ platformresolver.Capability, _ map[string]any, config map[string]any) (map[string]any, error) {
		return config, nil
	}
	runtimeProfile := func(_ platformresolver.Capability, backend, profile map[string]any) (map[string]any, error) {
		connection, _ := backend["connection"].(map[string]any)
		if connection["mode"] != "in-cluster" {
			return nil, platformresolver.NewAdapterConfigError("unsupported-connection-mode", "connection.mode")
		}
		isolation, _ := profile["isolation"].(string)
		if isolation != "dedicated" {
			return nil, platformresolver.NewAdapterConfigError("unsupported-isolation", "isolation")
		}
		return map[string]any{"isolation": isolation}, nil
	}
	return platformFixtureLookup{
		"agenova.io/deployment/kubernetes@0.1.0":   {ID: "agenova.io/deployment/kubernetes", Version: "0.1.0", Capabilities: []platformresolver.Capability{platformresolver.CapabilityDeployment}, CanonicalizeInstance: identityInstance, CanonicalizeProfile: identityProfile},
		"agenova.io/runtime/agent-sandbox@0.1.0":   {ID: "agenova.io/runtime/agent-sandbox", Version: "0.1.0", Capabilities: []platformresolver.Capability{platformresolver.CapabilityRuntime}, CanonicalizeInstance: identityInstance, CanonicalizeProfile: runtimeProfile},
		"agenova.io/model/openai-compatible@0.1.0": {ID: "agenova.io/model/openai-compatible", Version: "0.1.0", Capabilities: []platformresolver.Capability{platformresolver.CapabilityModel}, CanonicalizeInstance: identityInstance, CanonicalizeProfile: identityProfile},
	}
}
