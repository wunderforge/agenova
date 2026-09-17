// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
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
	return registration.Service{
		Store: registration.KubernetesStore{Context: contextName, Namespace: namespace},
		ValidateTemplate: func(template *v0.AgentTemplate) error {
			return validateReferenceTemplate(template, &state.Platform)
		},
	}, nil
}

// Registration must reject a template that this installed reference runtime
// cannot launch before the create-only registry takes its singleton slot.
func validateReferenceTemplate(template *v0.AgentTemplate, resolved *platform.ResolvedPlatform) error {
	if template == nil || template.Spec.Artifact == nil || template.Spec.Entrypoint == nil || resolved == nil {
		return fmt.Errorf("registered AgentTemplate or installed Platform is incomplete")
	}
	workerImage := ""
	runtimes := 0
	for _, instance := range resolved.Instances {
		if instance.Category == platform.CapabilityRuntime {
			runtimes++
			workerImage, _ = instance.Config["compatible-worker-image"].(string)
		}
	}
	if runtimes != 1 || workerImage == "" || template.Spec.Artifact.Image != workerImage {
		return fmt.Errorf("AgentTemplate image does not match the installed runtime compatible worker image")
	}
	command := template.Spec.Entrypoint.Command
	if len(command) != 2 || command[0] != "/agenova-workerctl" || command[1] != "serve" {
		return fmt.Errorf("reference runtime requires a controlled-worker entrypoint")
	}
	if ceiling := template.Spec.CapabilityCeiling; ceiling == nil || ceiling.MaxTimeout == nil || time.Duration(*ceiling.MaxTimeout) > 5*time.Minute || time.Duration(*ceiling.MaxTimeout) <= 0 {
		return fmt.Errorf("reference runtime requires a positive execution timeout no greater than five minutes")
	}
	return nil
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
