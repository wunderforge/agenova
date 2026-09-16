// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package adapterregistry

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/platform"
)

type testDeployment struct{ version string }
type typedNilDeployment struct{}

func TestRegistryCatalogLookupAndCapabilityFactory(t *testing.T) {
	second := testRegistration("example.com/deployment/second", "0.1.0")
	first := testRegistration("example.com/deployment/first", "0.1.0")
	registry, err := New(second, first)
	if err != nil {
		t.Fatal(err)
	}
	catalog := registry.Catalog()
	if len(catalog) != 2 || catalog[0].ID != first.Manifest.ID || catalog[1].ID != second.Manifest.ID {
		t.Fatalf("catalog = %#v", catalog)
	}
	descriptor, ok := registry.Lookup(first.Manifest.ID, first.Manifest.Version)
	if !ok || descriptor.ID != first.Manifest.ID {
		t.Fatalf("descriptor = %#v, %t", descriptor, ok)
	}
	implementation, err := registry.Construct(first.Manifest.ID, first.Manifest.Version, platform.CapabilityDeployment)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := implementation.(*testDeployment); !ok || got.version != "0.1.0" {
		t.Fatalf("implementation = %#v", implementation)
	}
	if _, err := registry.Construct(first.Manifest.ID, first.Manifest.Version, platform.CapabilityModel); err == nil || !strings.Contains(err.Error(), "does not provide model") {
		t.Fatalf("capability error = %v", err)
	}

	// Returned metadata is a snapshot, not a mutation path into the registry.
	catalog[0].Capabilities[0] = platform.CapabilityModel
	again := registry.Catalog()
	if again[0].Capabilities[0] != platform.CapabilityDeployment {
		t.Fatalf("catalog mutation leaked: %#v", again)
	}
}

func TestRegistryRejectsInvalidRegistrationWithoutPartialRegistry(t *testing.T) {
	valid := testRegistration("example.com/deployment/valid", "0.1.0")
	tests := []struct {
		name   string
		mutate func(*Registration)
		want   string
	}{
		{"identity", func(r *Registration) { r.Manifest.ID = "not-qualified" }, "qualified adapter ID"},
		{"version", func(r *Registration) { r.Manifest.Version = "latest" }, "adapter version"},
		{"protocol", func(r *Registration) { r.Manifest.Protocol = "other/v1" }, "unsupported protocol"},
		{"descriptor identity", func(r *Registration) { r.Descriptor.ID = "example.com/deployment/other" }, "descriptor identity"},
		{"capability mismatch", func(r *Registration) { r.Descriptor.Capabilities = []platform.Capability{platform.CapabilityModel} }, "descriptor capabilities"},
		{"missing factory", func(r *Registration) { r.Factories = nil }, "no factory"},
		{"extra factory", func(r *Registration) {
			r.Factories[platform.CapabilityModel] = func() (any, error) { return struct{}{}, nil }
		}, "outside"},
		{"secret schema", func(r *Registration) {
			r.Manifest.InstanceSchema.Fields = append(r.Manifest.InstanceSchema.Fields, Field{Path: "token", Kind: ValueString, Default: "forbidden"})
		}, "forbidden config"},
		{"optional secret schema", func(r *Registration) {
			r.Manifest.InstanceSchema.Fields = append(r.Manifest.InstanceSchema.Fields, Field{Path: "password", Kind: ValueString})
		}, "forbidden field"},
		{"bad default", func(r *Registration) { r.Manifest.InstanceSchema.Fields[0].Default = true }, "does not match"},
		{"factory nil", func(r *Registration) { r.Factories[platform.CapabilityDeployment] = nil }, "no factory"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneRegistration(valid)
			test.mutate(&candidate)
			registry, err := New(valid, candidate)
			if registry != nil || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("New() = %#v, %v; want %q", registry, err, test.want)
			}
		})
	}
	registry, err := New(valid, valid)
	if registry != nil || err == nil || !strings.Contains(err.Error(), "duplicate adapter") {
		t.Fatalf("duplicate New() = %#v, %v", registry, err)
	}
}

func TestRegistryResolveRequiresExactVersionWhenAmbiguous(t *testing.T) {
	first := testRegistration("example.com/deployment/demo", "0.1.0")
	second := testRegistration("example.com/deployment/demo", "0.2.0")
	registry, err := New(first, second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Resolve(first.Manifest.ID); err == nil || !strings.Contains(err.Error(), "multiple available versions") {
		t.Fatalf("ambiguous error = %v", err)
	}
	resolved, err := registry.Resolve(first.Manifest.ID + "@" + first.Manifest.Version)
	if err != nil || !reflect.DeepEqual(resolved.Manifest, first.Manifest) {
		t.Fatalf("Resolve() = %#v, %v", resolved.Manifest, err)
	}
	if _, err := registry.Resolve("example.com/deployment/missing@0.1.0"); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("unknown error = %v", err)
	}
}

func TestRegistryFactoryFailureAndNilAreActionable(t *testing.T) {
	failing := testRegistration("example.com/deployment/failing", "0.1.0")
	failing.Factories[platform.CapabilityDeployment] = func() (any, error) { return nil, errors.New("unavailable") }
	nilResult := testRegistration("example.com/deployment/nil", "0.1.0")
	nilResult.Factories[platform.CapabilityDeployment] = func() (any, error) { return nil, nil }
	typedNil := testRegistration("example.com/deployment/typed-nil", "0.1.0")
	typedNil.Factories[platform.CapabilityDeployment] = func() (any, error) { return (*typedNilDeployment)(nil), nil }
	registry, err := New(failing, nilResult, typedNil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Construct(failing.Manifest.ID, failing.Manifest.Version, platform.CapabilityDeployment); err == nil || !strings.Contains(err.Error(), "factory failed") || strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("factory error = %v", err)
	}
	if _, err := registry.Construct(nilResult.Manifest.ID, nilResult.Manifest.Version, platform.CapabilityDeployment); err == nil || !strings.Contains(err.Error(), "no implementation") {
		t.Fatalf("nil factory error = %v", err)
	}
	if _, err := registry.Construct(typedNil.Manifest.ID, typedNil.Manifest.Version, platform.CapabilityDeployment); err == nil || !strings.Contains(err.Error(), "no implementation") {
		t.Fatalf("typed nil factory error = %v", err)
	}
}

func TestRegistryRejectsUnusableProfileSchemaAndOversizedIdentity(t *testing.T) {
	withProfile := testRegistration("example.com/deployment/profile", "0.1.0")
	withProfile.Manifest.ProfileSchema.Fields = []Field{{Path: "apiKey", Kind: ValueString, Default: "forbidden"}}
	if registry, err := New(withProfile); registry != nil || err == nil || !strings.Contains(err.Error(), "cannot declare a profile schema") {
		t.Fatalf("deployment profile schema = %#v, %v", registry, err)
	}
	tooLongID := "example.com/deployment/" + strings.Repeat("a", 240)
	oversized := testRegistration(tooLongID, "0.1.0")
	if registry, err := New(oversized); registry != nil || err == nil || !strings.Contains(err.Error(), "256") {
		t.Fatalf("oversized ID = %#v, %v", registry, err)
	}
	longVersion := "1.0.0-" + strings.Repeat("a", 60)
	oversized = testRegistration("example.com/deployment/version", longVersion)
	if registry, err := New(oversized); registry != nil || err == nil || !strings.Contains(err.Error(), "64") {
		t.Fatalf("oversized version = %#v, %v", registry, err)
	}
}

func testRegistration(id, version string) Registration {
	manifest := Manifest{
		ID: id, Version: version, Protocol: ProtocolVersion,
		Capabilities:   []platform.Capability{platform.CapabilityDeployment},
		InstanceSchema: ConfigSchema{Fields: []Field{{Path: "namespace", Kind: ValueString, Required: true, Default: "agenova-system"}}},
	}
	return Registration{
		Manifest: manifest,
		Descriptor: platform.Descriptor{
			ID: id, Version: version, Capabilities: manifest.Capabilities,
			CanonicalizeInstance: func(_ platform.Capability, config map[string]any) (map[string]any, error) { return config, nil },
		},
		Factories: map[platform.Capability]Factory{
			platform.CapabilityDeployment: func() (any, error) { return &testDeployment{version: version}, nil },
		},
	}
}
