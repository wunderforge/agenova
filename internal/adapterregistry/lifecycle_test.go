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
