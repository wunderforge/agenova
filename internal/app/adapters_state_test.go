// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewOperatorUsesOwnDefaultStateWithoutSharedDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	service, err := NewPlatformService("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.State.Load(); err == nil {
		t.Fatal("fresh operator unexpectedly has an applied installation")
	}
	resolved, lock, err := service.ValidateFile("../../deploy/reference/platform.kind.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// Remember is called after successful reconciliation. No remote mutation in this test.
	if err = service.Remember(resolved, lock); err != nil {
		t.Fatal(err)
	}
	next, err := NewPlatformService("")
	if err != nil {
		t.Fatal(err)
	}
	saved, err := next.State.Load()
	if err != nil || saved.Platform.Revision != resolved.Revision {
		t.Fatalf("default state not retained: %v", err)
	}
	root, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, "agenova", "current-platform.json")); err != nil {
		t.Fatal(err)
	}
}
