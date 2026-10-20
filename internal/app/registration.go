// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// Registration validates structure and runtime bounds, not executable compatibility.
// A template owns its image. Successful controlled-v1 execution is verified at run time.
func validateReferenceTemplate(template *v0.AgentTemplate, resolved *platform.ResolvedPlatform) error {
	if resolved == nil {
		return fmt.Errorf("installed Platform is required")
	}
	if err := ValidateControlledTemplate(template); err != nil {
		return err
	}
	// Preserve the reference operator's existing registration bound. Member
	// registration previously allowed broader ceilings; do not break those
	// immutable records merely to decouple their images.
	if ceiling := template.Spec.CapabilityCeiling; ceiling.MaxTimeout == nil || time.Duration(*ceiling.MaxTimeout) > 5*time.Minute || time.Duration(*ceiling.MaxTimeout) <= 0 {
		return fmt.Errorf("reference runtime requires a positive execution timeout no greater than five minutes")
	}
	return nil
}

// ValidateControlledTemplate is shared by installed CLI/member registration and
// launch. It intentionally accepts independently published images and commands.
// Registration is not an attestation that an image implements controlled-v1.
func ValidateControlledTemplate(template *v0.AgentTemplate) error {
	if err := v0.ValidateAgentTemplate(template); err != nil {
		return err
	}
	// The name is also used as an Agent Sandbox template and in a "pool-"
	// resource name. Reject unusable names before create-only registration
	// reserves its immutable identity.
	name := template.Metadata.Name
	if len(name) == 0 || len(name) > 58 || !referenceTemplateName.MatchString(name) {
		return fmt.Errorf("reference runtime requires an AgentTemplate name of at most 58 lowercase DNS-label characters")
	}
	return nil
}

var referenceTemplateName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

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
