// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package adapterregistry

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wunderforge/agenova/internal/platform"
)

type MemoryStore struct {
	mu   sync.Mutex
	lock InstallationLock
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{lock: emptyLock()} }

func (s *MemoryStore) List() (InstallationLock, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneLock(s.lock), nil
}

func (s *MemoryStore) Install(adapter InstalledAdapter) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := normalizeLock(s.lock)
	if err != nil {
		return false, err
	}
	for _, existing := range lock.Adapters {
		if existing.ID != adapter.ID {
			continue
		}
		if existing.Version == adapter.Version && equalInstalled(existing, adapter) {
			return false, nil
		}
		return false, fmt.Errorf("adapter %s already has active version %s", adapter.ID, existing.Version)
	}
	lock.Adapters = append(lock.Adapters, cloneInstalled(adapter))
	lock, err = normalizeLock(lock)
	if err != nil {
		return false, err
	}
	s.lock = lock
	return true, nil
}

// FileStore records each exact installed adapter as one immutable file. A
// first install is an exclusive file creation, so a failed write cannot
// replace an already valid lock entry and idempotent installs need no write.
type FileStore struct {
	directory string
}

func NewFileStore(stateDirectory string) (*FileStore, error) {
	stateDirectory = strings.TrimSpace(stateDirectory)
	if stateDirectory == "" {
		return nil, fmt.Errorf("adapter state directory is required")
	}
	absolute, err := filepath.Abs(stateDirectory)
	if err != nil {
		return nil, fmt.Errorf("resolve adapter state directory: %w", err)
	}
	return &FileStore{directory: filepath.Join(absolute, "adapters")}, nil
}

func (s *FileStore) List() (InstallationLock, error) {
	entries, err := os.ReadDir(s.directory)
	if errors.Is(err, os.ErrNotExist) {
		return emptyLock(), nil
	}
	if err != nil {
		return InstallationLock{}, fmt.Errorf("read %s: %w", s.directory, err)
	}
	lock := emptyLock()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.directory, entry.Name())
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return InstallationLock{}, fmt.Errorf("read adapter lock entry %s: %w", entry.Name(), readErr)
		}
		var adapter InstalledAdapter
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if decodeErr := decoder.Decode(&adapter); decodeErr != nil {
			return InstallationLock{}, fmt.Errorf("decode adapter lock entry %s: %w", entry.Name(), decodeErr)
		}
		if decodeErr := decoder.Decode(&struct{}{}); decodeErr != io.EOF {
			return InstallationLock{}, fmt.Errorf("decode adapter lock entry %s: trailing content", entry.Name())
		}
		lock.Adapters = append(lock.Adapters, adapter)
	}
	return normalizeLock(lock)
}

func (s *FileStore) Install(adapter InstalledAdapter) (bool, error) {
	validated, err := normalizeLock(InstallationLock{APIVersion: LockAPIVersion, Kind: LockKind, Adapters: []InstalledAdapter{adapter}})
	if err != nil {
		return false, err
	}
	adapter = validated.Adapters[0]
	lock, err := s.List()
	if err != nil {
		return false, err
	}
	for _, existing := range lock.Adapters {
		if existing.ID != adapter.ID {
			continue
		}
		if existing.Version == adapter.Version && equalInstalled(existing, adapter) {
			return false, nil
		}
		return false, fmt.Errorf("adapter %s already has active version %s", adapter.ID, existing.Version)
	}
	if err := os.MkdirAll(s.directory, 0o700); err != nil {
		return false, fmt.Errorf("create adapter state directory: %w", err)
	}
	data, err := json.MarshalIndent(adapter, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode adapter lock entry: %w", err)
	}
	data = append(data, '\n')
	filePath := filepath.Join(s.directory, adapterFileName(adapter))
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		current, listErr := s.List()
		if listErr != nil {
			return false, listErr
		}
		for _, existing := range current.Adapters {
			if equalInstalled(existing, adapter) {
				return false, nil
			}
		}
		return false, fmt.Errorf("adapter lock entry already exists with different content")
	}
	if err != nil {
		return false, fmt.Errorf("create adapter lock entry: %w", err)
	}
	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(filePath)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return false, fmt.Errorf("write adapter lock entry: %w", err)
	}
	if err := file.Sync(); err != nil {
		return false, fmt.Errorf("sync adapter lock entry: %w", err)
	}
	if err := file.Close(); err != nil {
		return false, fmt.Errorf("close adapter lock entry: %w", err)
	}
	success = true
	return true, nil
}

func adapterFileName(adapter InstalledAdapter) string {
	id := base64.RawURLEncoding.EncodeToString([]byte(adapter.ID))
	return id + "@" + adapter.Version + ".json"
}

func cloneLock(input InstallationLock) InstallationLock {
	result := InstallationLock{APIVersion: input.APIVersion, Kind: input.Kind, Adapters: make([]InstalledAdapter, len(input.Adapters))}
	for i, adapter := range input.Adapters {
		result.Adapters[i] = cloneInstalled(adapter)
	}
	return result
}

func cloneInstalled(input InstalledAdapter) InstalledAdapter {
	input.Capabilities = append([]platform.Capability(nil), input.Capabilities...)
	return input
}

func equalInstalled(left, right InstalledAdapter) bool {
	if left.ID != right.ID || left.Version != right.Version || left.Protocol != right.Protocol {
		return false
	}
	left.Capabilities = sortedCapabilities(left.Capabilities)
	right.Capabilities = sortedCapabilities(right.Capabilities)
	if len(left.Capabilities) != len(right.Capabilities) {
		return false
	}
	for i := range left.Capabilities {
		if left.Capabilities[i] != right.Capabilities[i] {
			return false
		}
	}
	return true
}
