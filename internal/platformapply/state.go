// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package platformapply

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/wunderforge/agenova/internal/platform"
)

const currentPlatformFile = "current-platform.json"

// AppliedState is a local, secret-free pointer to the last successfully
// applied effective Platform. It does not substitute for cluster observation.
type AppliedState struct {
	Platform platform.ResolvedPlatform `json:"platform"`
	Lock     platform.PlatformLock     `json:"lock"`
}

type FileState struct{ directory string }

func NewFileState(directory string) (*FileState, error) {
	if directory == "" {
		return nil, fmt.Errorf("Platform state directory is required")
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve Platform state directory: %w", err)
	}
	return &FileState{directory: absolute}, nil
}

func (s *FileState) Save(resolved *platform.ResolvedPlatform, lock *platform.PlatformLock) error {
	if s == nil || resolved == nil || lock == nil || resolved.Revision == "" || resolved.Revision != lock.Revision {
		return fmt.Errorf("matching effective Platform and lock are required")
	}
	if err := os.MkdirAll(s.directory, 0o700); err != nil {
		return fmt.Errorf("create Platform state directory: %w", err)
	}
	data, err := json.MarshalIndent(AppliedState{Platform: *resolved, Lock: *lock}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode applied Platform pointer: %w", err)
	}
	temporary, err := os.CreateTemp(s.directory, ".current-platform-*.tmp")
	if err != nil {
		return fmt.Errorf("create applied Platform pointer: %w", err)
	}
	defer os.Remove(temporary.Name())
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary.Name(), filepath.Join(s.directory, currentPlatformFile)); err != nil {
		return fmt.Errorf("publish applied Platform pointer: %w", err)
	}
	return nil
}

func (s *FileState) Load() (*AppliedState, error) {
	if s == nil {
		return nil, fmt.Errorf("Platform state store is required")
	}
	data, err := os.ReadFile(filepath.Join(s.directory, currentPlatformFile))
	if err != nil {
		return nil, fmt.Errorf("read applied Platform pointer: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var state AppliedState
	if err := decoder.Decode(&state); err != nil {
		return nil, fmt.Errorf("decode applied Platform pointer: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("applied Platform pointer has trailing data")
	}
	if state.Platform.Revision == "" || state.Platform.Revision != state.Lock.Revision {
		return nil, fmt.Errorf("applied Platform pointer revision mismatch")
	}
	return &state, nil
}
