// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/platformapply"
	"github.com/wunderforge/agenova/internal/registration"
)

// AppliedPlatform locates the selected install without accepting an ad-hoc
// namespace or backend override on individual management commands.
func AppliedPlatform(stateDirectory string) (*platformapply.AppliedState, error) {
	if strings.TrimSpace(stateDirectory) == "" {
		root, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("resolve user configuration directory: %w", err)
		}
		stateDirectory = filepath.Join(root, "agenova")
	}
	state, err := platformapply.NewFileState(stateDirectory)
	if err != nil {
		return nil, err
	}
	return state.Load()
}

func NewRegistrationService(stateDirectory string) (registration.Service, error) {
	state, err := AppliedPlatform(stateDirectory)
	if err != nil {
		return registration.Service{}, fmt.Errorf("apply a Platform before registering resources: %w", err)
	}
	contextName, namespace, err := DeploymentCoordinates(&state.Platform)
	if err != nil {
		return registration.Service{}, err
	}
	return registration.Service{Store: registration.KubernetesStore{Context: contextName, Namespace: namespace}}, nil
}

func DeploymentCoordinates(resolved *platform.ResolvedPlatform) (string, string, error) {
	if resolved == nil {
		return "", "", fmt.Errorf("effective Platform is required")
	}
	for _, instance := range resolved.Instances {
		if instance.Category != platform.CapabilityDeployment {
			continue
		}
		contextName, _ := instance.Config["context"].(string)
		namespace, _ := instance.Config["namespace"].(string)
		if contextName == "" || namespace == "" || namespace == "default" {
			return "", "", fmt.Errorf("effective Platform has no safe Kubernetes target")
		}
		return contextName, namespace, nil
	}
	return "", "", fmt.Errorf("effective Platform has no deployment target")
}
