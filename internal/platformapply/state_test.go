// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package platformapply

import (
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/platform"
)

func TestAppliedStateRoundTripAndRevisionGuard(t *testing.T) {
	store, err := NewFileState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("missing applied Platform pointer was accepted")
	}
	resolved := &platform.ResolvedPlatform{PlatformName: "reference", Revision: "sha256:first"}
	lock := &platform.PlatformLock{PlatformName: "reference", Revision: resolved.Revision}
	if err := store.Save(resolved, lock); err != nil {
		t.Fatal(err)
	}
	first, err := store.Load()
	if err != nil || first.Platform.Revision != resolved.Revision {
		t.Fatalf("Load = %#v, %v", first, err)
	}
	resolved.Revision, lock.Revision = "sha256:second", "sha256:second"
	if err := store.Save(resolved, lock); err != nil {
		t.Fatalf("replace pointer: %v", err)
	}
	second, err := store.Load()
	if err != nil || second.Platform.Revision != "sha256:second" {
		t.Fatalf("replacement = %#v, %v", second, err)
	}
	lock.Revision = "sha256:wrong"
	if err := store.Save(resolved, lock); err == nil || !strings.Contains(err.Error(), "matching") {
		t.Fatalf("mismatched save = %v", err)
	}
}
