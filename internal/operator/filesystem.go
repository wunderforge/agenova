// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

var errTaskFileNotFound = errors.New("reference filesystem: task file not found")

const referenceRuntimeFile = "/runtime/agent"

// prepareTaskFile models backend preparation before the worker is allowed to
// start. It lets the contract suite prove that an existing task file is not
// readable or mutable while the allocation is only Bound.
func (r *Runtime) prepareTaskFile(id v1alpha1.SandboxClaimBackendIdentity, target string, data []byte) error {
	a, err := r.allocationFor(id)
	if err != nil {
		return err
	}
	if a.started || a.terminated || a.released {
		return fmt.Errorf("prepare filesystem %s: allocation is not awaiting Start", a.claimID)
	}
	relative, err := taskRelativePath(a.filesystem.WorkingDirectory, target)
	if err != nil {
		return err
	}
	a.taskFiles[relative] = append([]byte(nil), data...)
	return nil
}

// writeTaskFile is a reference-model probe used by the reusable filesystem
// contract suite. It is deliberately not part of RuntimeBackend: production
// agents use ordinary filesystem APIs inside the boundary established by the
// selected backend.
func (r *Runtime) writeTaskFile(id v1alpha1.SandboxClaimBackendIdentity, target string, data []byte) error {
	a, err := r.activeFilesystem(id)
	if err != nil {
		return err
	}
	relative, err := taskRelativePath(a.filesystem.WorkingDirectory, target)
	if err != nil {
		return err
	}
	a.taskFiles[relative] = append([]byte(nil), data...)
	return nil
}

func (r *Runtime) readTaskFile(id v1alpha1.SandboxClaimBackendIdentity, target string) ([]byte, error) {
	a, err := r.activeFilesystem(id)
	if err != nil {
		return nil, err
	}
	relative, err := taskRelativePath(a.filesystem.WorkingDirectory, target)
	if err != nil {
		return nil, err
	}
	data, ok := a.taskFiles[relative]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errTaskFileNotFound, relative)
	}
	return append([]byte(nil), data...), nil
}

// exportTaskFile is the reference model's test-only stand-in for an approved
// external output operation. It deliberately shares the active-work guard:
// output bytes can leave the task directory only before termination.
func (r *Runtime) exportTaskFile(id v1alpha1.SandboxClaimBackendIdentity, target string) ([]byte, error) {
	return r.readTaskFile(id, target)
}

func (r *Runtime) readRuntimeFile(id v1alpha1.SandboxClaimBackendIdentity, target string) ([]byte, error) {
	if _, err := r.activeFilesystem(id); err != nil {
		return nil, err
	}
	if path.Clean(target) != referenceRuntimeFile {
		return nil, fmt.Errorf("%w: %s", runtime.ErrFilesystemBoundary, target)
	}
	return append([]byte(nil), r.filesystemRuntimeSentinel...), nil
}

func (r *Runtime) writeRuntimeFile(id v1alpha1.SandboxClaimBackendIdentity, target string, _ []byte) error {
	if _, err := r.activeFilesystem(id); err != nil {
		return err
	}
	return fmt.Errorf("%w: runtime file is read-only: %s", runtime.ErrFilesystemBoundary, target)
}

func (r *Runtime) outsideFilesystemSentinel() []byte {
	return append([]byte(nil), r.filesystemOutsideSentinel...)
}

func (r *Runtime) runtimeFilesystemSentinel() []byte {
	return append([]byte(nil), r.filesystemRuntimeSentinel...)
}

func (r *Runtime) activeFilesystem(id v1alpha1.SandboxClaimBackendIdentity) (*allocation, error) {
	a, err := r.allocationFor(id)
	if err != nil {
		return nil, err
	}
	switch {
	case a.released:
		return nil, fmt.Errorf("filesystem %s: %w", a.claimID, runtime.ErrReleased)
	case a.terminated:
		return nil, fmt.Errorf("filesystem %s: %w", a.claimID, runtime.ErrTerminated)
	case !a.started:
		return nil, fmt.Errorf("filesystem %s: work has not started", a.claimID)
	}
	return a, nil
}

func taskRelativePath(root, target string) (string, error) {
	if strings.TrimSpace(target) == "" || strings.Contains(target, "\\") {
		return "", fmt.Errorf("%w: invalid worker path %q", runtime.ErrFilesystemBoundary, target)
	}
	cleaned := path.Clean(target)
	if !path.IsAbs(cleaned) {
		cleaned = path.Join(root, cleaned)
	}
	root = path.Clean(root)
	if cleaned == root || !strings.HasPrefix(cleaned, root+"/") {
		return "", fmt.Errorf("%w: %s", runtime.ErrFilesystemBoundary, target)
	}
	return strings.TrimPrefix(cleaned, root+"/"), nil
}
