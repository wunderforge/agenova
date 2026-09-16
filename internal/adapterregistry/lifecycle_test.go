// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package adapterregistry

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/platform"
)

func TestLifecycleFirstAndIdempotentInstallThenVersionConflict(t *testing.T) {
	id := "example.com/deployment/demo"
	registry, err := New(testRegistration(id, "0.1.0"), testRegistration(id, "0.2.0"))
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	lifecycle, err := NewLifecycle(registry, store)
	if err != nil {
		t.Fatal(err)
	}
	first, err := lifecycle.Install(id + "@0.1.0")
	if err != nil || !first.Changed {
		t.Fatalf("first install = %#v, %v", first, err)
	}
	second, err := lifecycle.Install(id + "@0.1.0")
	if err != nil || second.Changed {
		t.Fatalf("second install = %#v, %v", second, err)
	}
	before, _ := lifecycle.List()
	if _, err := lifecycle.Install(id + "@0.2.0"); err == nil || !strings.Contains(err.Error(), "active version 0.1.0") {
		t.Fatalf("conflict error = %v", err)
	}
	after, _ := lifecycle.List()
	if !reflect.DeepEqual(before, after) || len(after.Adapters) != 1 {
		t.Fatalf("conflict mutated lock: before=%#v after=%#v", before, after)
	}
	inspection, err := lifecycle.Inspect(id + "@0.1.0")
	if err != nil || !inspection.Installed {
		t.Fatalf("Inspect() = %#v, %v", inspection, err)
	}
}

func TestFileStorePersistsIndependentEntriesAndRejectsCorruption(t *testing.T) {
	directory := t.TempDir()
	store, err := NewFileStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	one := installedFromManifest(testRegistration("example.com/deployment/one", "0.1.0").Manifest)
	two := installedFromManifest(testRegistration("example.com/deployment/two", "0.1.0").Manifest)
	if changed, err := store.Install(two); err != nil || !changed {
		t.Fatalf("install two = %t, %v", changed, err)
	}
	if changed, err := store.Install(one); err != nil || !changed {
		t.Fatalf("install one = %t, %v", changed, err)
	}
	reopened, err := NewFileStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := reopened.List()
	if err != nil || len(lock.Adapters) != 2 || lock.Adapters[0].ID != one.ID || lock.Adapters[1].ID != two.ID {
		t.Fatalf("List() = %#v, %v", lock, err)
	}
	if changed, err := reopened.Install(one); err != nil || changed {
		t.Fatalf("idempotent file install = %t, %v", changed, err)
	}
	if err := os.WriteFile(filepath.Join(directory, "adapters", "corrupt.json"), []byte(`{"id":"leaked"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.List(); err == nil || !strings.Contains(err.Error(), "decode adapter lock entry") && !strings.Contains(err.Error(), "invalid qualified adapter ID") {
		t.Fatalf("corrupt List() error = %v", err)
	}
}

func TestInitRejectsAdapterEmittedCredentialConfiguration(t *testing.T) {
	registration := testRegistration("example.com/deployment/unsafe", "0.1.0")
	registration.Descriptor.CanonicalizeInstance = func(platform.Capability, map[string]any) (map[string]any, error) {
		return map[string]any{"apiKey": "should-never-be-emitted"}, nil
	}
	registry, err := New(registration)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, _ := NewLifecycle(registry, NewMemoryStore())
	fragment, err := lifecycle.Init(registration.Manifest.ID, "unsafe")
	if err == nil || !strings.Contains(err.Error(), "forbidden configuration") || !reflect.DeepEqual(fragment, PlatformFragment{}) {
		t.Fatalf("Init() = %#v, %v", fragment, err)
	}
}

func TestLifecycleConstructUsesRegisteredCapabilityFactory(t *testing.T) {
	implementation := &struct{ Name string }{Name: "deployment"}
	registration := testRegistration("example.com/deployment/reference", "1.0.0")
	registration.Factories[platform.CapabilityDeployment] = func() (any, error) { return implementation, nil }
	registry, err := New(registration)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := NewLifecycle(registry, NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}

	got, err := lifecycle.Construct(registration.Manifest.ID, registration.Manifest.Version, platform.CapabilityDeployment)
	if err != nil {
		t.Fatal(err)
	}
	if got != implementation {
		t.Fatalf("Construct() = %#v, want registered implementation %#v", got, implementation)
	}
}

func TestInitRejectsGeneratedNamesBeyondPlatformLimit(t *testing.T) {
	registration := runtimeTestRegistration("example.com/runtime/names", "1.0.0")
	registry, err := New(registration)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, _ := NewLifecycle(registry, NewMemoryStore())
	name := strings.Repeat("a", 57)
	if fragment, err := lifecycle.Init(registration.Manifest.ID, name); err == nil || !strings.Contains(err.Error(), "profile name") || !reflect.DeepEqual(fragment, PlatformFragment{}) {
		t.Fatalf("Init() = %#v, %v", fragment, err)
	}
}

func TestInitPreservesIntegerDefaultsAndIsolatesBackendConfig(t *testing.T) {
	registration := runtimeTestRegistration("example.com/runtime/safe", "1.0.0")
	registration.Manifest.InstanceSchema.Fields = append(registration.Manifest.InstanceSchema.Fields, Field{Path: "workers", Kind: ValueInteger, Default: int64(9007199254740993)})
	registration.Descriptor.CanonicalizeInstance = func(_ platform.Capability, config map[string]any) (map[string]any, error) {
		if _, ok := config["workers"].(int64); !ok {
			t.Fatalf("integer default type = %T, want int64", config["workers"])
		}
		return config, nil
	}
	registration.Descriptor.CanonicalizeProfile = func(_ platform.Capability, backend, profile map[string]any) (map[string]any, error) {
		backend["apiKey"] = "must-not-leak"
		return profile, nil
	}
	registry, err := New(registration)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, _ := NewLifecycle(registry, NewMemoryStore())
	fragment, err := lifecycle.Init(registration.Manifest.ID, "safe")
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := fragment.Spec.Infrastructure.RuntimeBackends[0].Config["apiKey"]; leaked {
		t.Fatalf("profile canonicalizer mutated emitted backend config: %#v", fragment)
	}
}

func runtimeTestRegistration(id, version string) Registration {
	manifest := Manifest{ID: id, Version: version, Protocol: ProtocolVersion, Capabilities: []platform.Capability{platform.CapabilityRuntime}, InstanceSchema: ConfigSchema{Fields: []Field{{Path: "namespace", Kind: ValueString, Default: "workers"}}}, ProfileSchema: ConfigSchema{Fields: []Field{{Path: "isolation", Kind: ValueString, Default: "dedicated"}}}}
	return Registration{Manifest: manifest, Descriptor: platform.Descriptor{ID: id, Version: version, Capabilities: manifest.Capabilities, CanonicalizeInstance: func(_ platform.Capability, config map[string]any) (map[string]any, error) { return config, nil }, CanonicalizeProfile: func(_ platform.Capability, _, config map[string]any) (map[string]any, error) { return config, nil }}, Factories: map[platform.Capability]Factory{platform.CapabilityRuntime: func() (any, error) { return &struct{}{}, nil }}}
}
