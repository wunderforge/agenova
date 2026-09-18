// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package platformapply

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/platform"
)

func testResolvedState(t *testing.T, name string) (*platform.ResolvedPlatform, *platform.PlatformLock) {
	t.Helper()
	resolved := &platform.ResolvedPlatform{PlatformName: name}
	payload := struct {
		PlatformName     string                           `json:"platformName"`
		Adapters         []platform.ResolvedAdapter       `json:"adapters"`
		Instances        []platform.ResolvedInstance      `json:"instances"`
		Profiles         []platform.ResolvedProfile       `json:"profiles"`
		ModelRoutes      []platform.ModelRoute            `json:"modelRoutes"`
		InitialPolicyRef v1alpha1.PlatformPolicyReference `json:"initialPolicyRef"`
	}{PlatformName: name}
	encoded, err := platform.CanonicalJSON(payload)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(encoded)
	resolved.Revision = "sha256:" + hex.EncodeToString(sum[:])
	return resolved, &platform.PlatformLock{PlatformName: name, Revision: resolved.Revision}
}

func TestAppliedStateRoundTripAndRevisionGuard(t *testing.T) {
	store, err := NewFileState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("missing applied Platform pointer was accepted")
	}
	resolved, lock := testResolvedState(t, "reference")
	if err := store.Save(resolved, lock); err != nil {
		t.Fatal(err)
	}
	first, err := store.Load()
	if err != nil || first.Platform.Revision != resolved.Revision {
		t.Fatalf("Load = %#v, %v", first, err)
	}
	resolved, lock = testResolvedState(t, "replacement")
	if err := store.Save(resolved, lock); err != nil {
		t.Fatalf("replace pointer: %v", err)
	}
	second, err := store.Load()
	if err != nil || second.Platform.Revision != resolved.Revision {
		t.Fatalf("replacement = %#v, %v", second, err)
	}
	lock.Revision = "sha256:wrong"
	if err := store.Save(resolved, lock); err == nil || !strings.Contains(err.Error(), "matching") {
		t.Fatalf("mismatched save = %v", err)
	}
}

func TestAppliedStateLoadRejectsChangedContentWithMatchingStoredRevisions(t *testing.T) {
	store, err := NewFileState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	resolved, lock := testResolvedState(t, "reference")
	if err := store.Save(resolved, lock); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.directory, currentPlatformFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state AppliedState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	state.Platform.PlatformName = "other-target"
	changed, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "content revision mismatch") {
		t.Fatalf("changed target with matching stored revisions accepted: %v", err)
	}
	state.Platform.PlatformName = "reference"
	state.Lock.PlatformName = "other-target"
	changed, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "lock content mismatch") {
		t.Fatalf("changed lock with matching stored revisions accepted: %v", err)
	}
}
