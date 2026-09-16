// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/adapters/bundled"
	"github.com/wunderforge/agenova/internal/platformapply"
)

// NewAdapterLifecycle composes the bundled catalog with the selected local
// installation state. Platform orchestration can inject the same lifecycle
// service rather than reimplementing adapter lookup or activation.
func NewAdapterLifecycle(stateDirectory string) (*adapterregistry.Lifecycle, error) {
	registry, err := bundled.NewRegistry()
	if err != nil {
		return nil, fmt.Errorf("construct bundled adapter registry: %w", err)
	}
	stateDirectory = strings.TrimSpace(stateDirectory)
	if stateDirectory == "" {
		root, configErr := os.UserConfigDir()
		if configErr != nil {
			return nil, fmt.Errorf("resolve user configuration directory: %w", configErr)
		}
		stateDirectory = filepath.Join(root, "agenova")
	}
	store, err := adapterregistry.NewFileStore(stateDirectory)
	if err != nil {
		return nil, err
	}
	return adapterregistry.NewLifecycle(registry, store)
}

// NewPlatformService composes Platform orchestration over the exact same
// registry and installation state used by the lower-level adapter commands.
func NewPlatformService(stateDirectory string) (platformapply.Service, error) {
	lifecycle, err := NewAdapterLifecycle(stateDirectory)
	if err != nil {
		return platformapply.Service{}, err
	}
	return platformapply.Service{Adapters: lifecycle, Policies: bundled.ReferencePolicyCatalog{}}, nil
}
