// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package contracttest

import (
	"bytes"
	"errors"
	"path"
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

// FilesystemFixture supplies reference-model probes for the filesystem
// contract. These probes are test controls, not RuntimeBackend operations and
// not a filesystem gateway.
type FilesystemFixture struct {
	Backend           runtime.RuntimeBackend
	TemplateRef       string
	PrepareTaskFile   func(v1alpha1.SandboxClaimBackendIdentity, string, []byte) error
	WriteTaskFile     func(v1alpha1.SandboxClaimBackendIdentity, string, []byte) error
	ReadTaskFile      func(v1alpha1.SandboxClaimBackendIdentity, string) ([]byte, error)
	ExportTaskFile    func(v1alpha1.SandboxClaimBackendIdentity, string) ([]byte, error)
	ReadRuntimeFile   func(v1alpha1.SandboxClaimBackendIdentity, string) ([]byte, error)
	WriteRuntimeFile  func(v1alpha1.SandboxClaimBackendIdentity, string, []byte) error
	OutsideSentinel   func() []byte
	RuntimeSentinel   func() []byte
	FailNextTerminate func(v1alpha1.SandboxClaimBackendIdentity, error)
}

// RunFilesystem exercises the backend-neutral filesystem description and
// reference semantics. It proves a simulated model only; hostile native
// process isolation requires separate real-backend evidence.
func RunFilesystem(t *testing.T, newFixture func(t *testing.T) FilesystemFixture) {
	t.Helper()
	cases := []struct {
		name string
		fn   func(*testing.T, FilesystemFixture)
	}{
		{"FS-P1 reports one simulated ephemeral task directory", testFilesystemDescription},
		{"FS-N0 denies task access before explicit Start", testFilesystemBeforeStart},
		{"FS-P2 reads and writes task data after explicit start", testFilesystemTaskData},
		{"FS-P4 reads but cannot mutate the runtime fixture", testFilesystemRuntimeReadOnly},
		{"FS-N1 rejects outside and traversal writes", testFilesystemOutsideBoundary},
		{"FS-N4 replacement claim receives fresh data", testFilesystemFreshReplacement},
		{"FS-N7 rejects export after successful termination", testFilesystemLifecycle},
		{"FS-N8 cleanup stops while termination remains incomplete", testFilesystemTerminationFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			requireFilesystemFixture(t, f)
			tc.fn(t, f)
		})
	}
}

func requireFilesystemFixture(t *testing.T, f FilesystemFixture) {
	t.Helper()
	if f.Backend == nil || f.TemplateRef == "" || f.PrepareTaskFile == nil || f.WriteTaskFile == nil || f.ReadTaskFile == nil || f.ExportTaskFile == nil ||
		f.ReadRuntimeFile == nil || f.WriteRuntimeFile == nil || f.OutsideSentinel == nil || f.RuntimeSentinel == nil || f.FailNextTerminate == nil {
		t.Fatal("filesystem fixture requires backend, task/export/runtime probes, sentinels and termination failure control")
	}
}

func testFilesystemBeforeStart(t *testing.T, f FilesystemFixture) {
	alloc := allocateFilesystem(t, f, "fs-before-start")
	prepared := []byte("prepared-before-start")
	if err := f.PrepareTaskFile(alloc.Identity, "repo/task.txt", prepared); err != nil {
		t.Fatalf("prepare task file: %v", err)
	}
	if got, err := f.ReadTaskFile(alloc.Identity, "repo/task.txt"); err == nil || got != nil {
		t.Fatalf("pre-Start read = %q, error = %v, want nil bytes and denial", got, err)
	}
	if err := f.WriteTaskFile(alloc.Identity, "repo/task.txt", []byte("mutated-before-start")); err == nil {
		t.Fatal("task write before Start succeeded")
	}
	if err := f.Backend.Start(alloc.Identity); err != nil {
		t.Fatalf("start after pre-Start probes: %v", err)
	}
	got, err := f.ReadTaskFile(alloc.Identity, "repo/task.txt")
	if err != nil || !bytes.Equal(got, prepared) {
		t.Fatalf("prepared task file after Start = %q, error = %v, want %q", got, err, prepared)
	}
}

func testFilesystemDescription(t *testing.T, f FilesystemFixture) {
	alloc := allocateFilesystem(t, f, "fs-description")
	want := runtime.FilesystemBoundary{
		WorkingDirectory: "/workspace",
		OutsideBoundary:  runtime.FilesystemOutsideRuntimeReadOnlyOtherUnavailable,
		Ephemeral:        true,
		EvidenceLevel:    runtime.FilesystemEvidenceSimulated,
	}
	if alloc.Filesystem != want {
		t.Fatalf("filesystem = %+v, want %+v", alloc.Filesystem, want)
	}
	obs, err := f.Backend.Observe(alloc.Identity)
	if err != nil || obs.Filesystem != alloc.Filesystem {
		t.Fatalf("observed filesystem = %+v, error = %v", obs.Filesystem, err)
	}
}

func testFilesystemTaskData(t *testing.T, f FilesystemFixture) {
	alloc := startFilesystem(t, f, "fs-task-data")
	want := []byte("claim-scoped output")
	if err := f.WriteTaskFile(alloc.Identity, "repo/result.txt", want); err != nil {
		t.Fatal(err)
	}
	got, err := f.ReadTaskFile(alloc.Identity, path.Join(alloc.Filesystem.WorkingDirectory, "repo/result.txt"))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("read = %q, error = %v", got, err)
	}
	got[0] = 'X'
	again, _ := f.ReadTaskFile(alloc.Identity, "repo/result.txt")
	if !bytes.Equal(again, want) {
		t.Fatalf("caller mutated stored task data: %q", again)
	}
}

func testFilesystemRuntimeReadOnly(t *testing.T, f FilesystemFixture) {
	alloc := startFilesystem(t, f, "fs-runtime-readonly")
	want := f.RuntimeSentinel()
	got, err := f.ReadRuntimeFile(alloc.Identity, "/runtime/agent")
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("runtime read = %q, error = %v, want %q", got, err, want)
	}
	if err := f.WriteRuntimeFile(alloc.Identity, "/runtime/agent", []byte("mutated")); !errors.Is(err, runtime.ErrFilesystemBoundary) {
		t.Fatalf("runtime write error = %v, want ErrFilesystemBoundary", err)
	}
	if after := f.RuntimeSentinel(); !bytes.Equal(after, want) {
		t.Fatalf("runtime sentinel changed: before %q after %q", want, after)
	}
}

func testFilesystemOutsideBoundary(t *testing.T, f FilesystemFixture) {
	alloc := startFilesystem(t, f, "fs-outside")
	before := f.OutsideSentinel()
	for _, target := range []string{"/outside/sentinel", "../sentinel", "/workspace-sibling/sentinel", `C:\\host\\secret`} {
		if err := f.WriteTaskFile(alloc.Identity, target, []byte("mutated")); !errors.Is(err, runtime.ErrFilesystemBoundary) {
			t.Fatalf("target %q error = %v, want ErrFilesystemBoundary", target, err)
		}
	}
	if _, err := f.ReadTaskFile(alloc.Identity, "/outside/sentinel"); !errors.Is(err, runtime.ErrFilesystemBoundary) {
		t.Fatalf("outside read error = %v, want ErrFilesystemBoundary", err)
	}
	if after := f.OutsideSentinel(); !bytes.Equal(after, before) {
		t.Fatalf("outside sentinel changed: before %q after %q", before, after)
	}
}

func testFilesystemFreshReplacement(t *testing.T, f FilesystemFixture) {
	first := startFilesystem(t, f, "fs-first")
	if err := f.WriteTaskFile(first.Identity, "repo/private.txt", []byte("first claim")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Backend.Cleanup(first.Identity); err != nil {
		t.Fatal(err)
	}
	second := startFilesystem(t, f, "fs-second")
	if _, err := f.ReadTaskFile(second.Identity, "repo/private.txt"); err == nil {
		t.Fatal("replacement claim inherited the previous claim's task file")
	}
}

func testFilesystemLifecycle(t *testing.T, f FilesystemFixture) {
	alloc := startFilesystem(t, f, "fs-lifecycle")
	exportedWant := []byte("exported-before-termination")
	if err := f.WriteTaskFile(alloc.Identity, "repo/exported.txt", exportedWant); err != nil {
		t.Fatal(err)
	}
	exported, err := f.ExportTaskFile(alloc.Identity, "repo/exported.txt")
	if err != nil || !bytes.Equal(exported, exportedWant) {
		t.Fatalf("pre-termination export = %q, error = %v", exported, err)
	}
	if err := f.WriteTaskFile(alloc.Identity, "repo/unexported.txt", []byte("must-not-export")); err != nil {
		t.Fatal(err)
	}
	if err := f.Backend.Terminate(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ReadTaskFile(alloc.Identity, "repo/unexported.txt"); !errors.Is(err, runtime.ErrTerminated) {
		t.Fatalf("read after termination = %v, want ErrTerminated", err)
	}
	if late, err := f.ExportTaskFile(alloc.Identity, "repo/unexported.txt"); !errors.Is(err, runtime.ErrTerminated) || late != nil {
		t.Fatalf("post-termination export = %q, error = %v, want nil/ErrTerminated", late, err)
	}
	if !bytes.Equal(exported, exportedWant) {
		t.Fatalf("acknowledged export changed after termination: %q", exported)
	}
	if _, err := f.Backend.Cleanup(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteTaskFile(alloc.Identity, "repo/late.txt", []byte("late")); !errors.Is(err, runtime.ErrReleased) {
		t.Fatalf("write after cleanup = %v, want ErrReleased", err)
	}
}

func testFilesystemTerminationFailure(t *testing.T, f FilesystemFixture) {
	alloc := startFilesystem(t, f, "fs-termination-failure")
	if err := f.WriteTaskFile(alloc.Identity, "repo/live.txt", []byte("still-live")); err != nil {
		t.Fatal(err)
	}
	first := errors.New("injected explicit termination failure")
	f.FailNextTerminate(alloc.Identity, first)
	if err := f.Backend.Terminate(alloc.Identity); !errors.Is(err, first) {
		t.Fatalf("termination error = %v, want injected failure", err)
	}
	second := errors.New("injected cleanup termination failure")
	f.FailNextTerminate(alloc.Identity, second)
	result, err := f.Backend.Cleanup(alloc.Identity)
	if !errors.Is(err, second) || result.Released || result.Replaced {
		t.Fatalf("cleanup while termination incomplete = %+v, %v", result, err)
	}
	obs, err := f.Backend.Observe(alloc.Identity)
	if err != nil || obs.Released || obs.Replaced {
		t.Fatalf("failed termination fabricated cleanup: %+v, %v", obs, err)
	}
	if got, err := f.ReadTaskFile(alloc.Identity, "repo/live.txt"); err != nil || !bytes.Equal(got, []byte("still-live")) {
		t.Fatalf("failed termination/cleanup released task data: %q, %v", got, err)
	}
	if _, err := f.Backend.Allocate(runtime.AllocateRequest{ClaimID: "fs-premature-reuse", TemplateRef: f.TemplateRef}); err == nil {
		t.Fatal("failed termination returned the only worker to reusable capacity")
	}
	if _, err := f.Backend.Cleanup(alloc.Identity); err != nil {
		t.Fatalf("cleanup retry after termination succeeds: %v", err)
	}
	replacement := startFilesystem(t, f, "fs-after-termination-retry")
	if _, err := f.ReadTaskFile(replacement.Identity, "repo/live.txt"); err == nil {
		t.Fatal("replacement after termination retry inherited prior task data")
	}
}

func allocateFilesystem(t *testing.T, f FilesystemFixture, claimID string) runtime.Allocation {
	t.Helper()
	alloc, err := f.Backend.Allocate(runtime.AllocateRequest{ClaimID: claimID, TemplateRef: f.TemplateRef})
	if err != nil {
		t.Fatalf("allocate %s: %v", claimID, err)
	}
	return alloc
}

func startFilesystem(t *testing.T, f FilesystemFixture, claimID string) runtime.Allocation {
	t.Helper()
	alloc := allocateFilesystem(t, f, claimID)
	if err := f.Backend.Start(alloc.Identity); err != nil {
		t.Fatalf("start %s: %v", claimID, err)
	}
	return alloc
}
