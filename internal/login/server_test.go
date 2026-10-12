// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package login

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSavedServer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "current-server")
	if got, err := readServer(path); err != nil || got != "" {
		t.Fatalf("missing: %q %v", got, err)
	}
	if err := writeServer(path, "https://app.example"); err != nil {
		t.Fatal(err)
	}
	if got, err := readServer(path); err != nil || got != "https://app.example" {
		t.Fatalf("saved: %q %v", got, err)
	}
	if err := writeServer(path, "http://invalid.example"); err == nil {
		t.Fatal("accepted insecure server")
	}
	if got, _ := readServer(path); got != "https://app.example" {
		t.Fatal("failed save changed endpoint")
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readServer(path); err == nil {
		t.Fatal("corrupt profile silently fell back")
	}
}
