// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A named pipe is refused from its type, never opened: opening it would block
// until a writer appeared.
func TestScanRefusesANamedPipe(t *testing.T) {
	valid, wrong := scanTokens()
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o600); err != nil {
		t.Skip("cannot create a named pipe here:", err)
	}
	var out, errOut bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- scanMain([]string{dir}, strings.NewReader(tokenLines(valid, valid, wrong)), &out, &errOut)
	}()
	select {
	case code := <-done:
		checkNoSecrets(t, out.String()+errOut.String(), nil)
		if code != 2 || !strings.Contains(errOut.String(), "is not a regular file") || strings.Contains(out.String(), "scanned ") {
			t.Fatalf("exited %d with stdout %q and stderr %q", code, out.String(), errOut.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the scan blocked on a named pipe")
	}
}
