// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/platform"
)

func TestReferenceTemplateCompatibilityBeforeRegistration(t *testing.T) {
	installed := &platform.ResolvedPlatform{Instances: []platform.ResolvedInstance{{Category: platform.CapabilityRuntime, Config: map[string]any{"compatible-worker-image": "trusted-worker:v1"}}}}
	template := &v0.AgentTemplate{Spec: v0.AgentTemplateSpec{Artifact: &v0.AgentTemplateArtifact{Image: "trusted-worker:v1"}, Entrypoint: &v0.AgentTemplateEntrypoint{Command: []string{"/agenova-workerctl", "serve"}}}}
	if err := validateReferenceTemplate(template, installed); err != nil {
		t.Fatal(err)
	}
	template.Spec.Artifact.Image = "unverified-worker:v1"
	if err := validateReferenceTemplate(template, installed); err == nil {
		t.Fatal("unverified worker image accepted")
	}
	template.Spec.Artifact.Image = "trusted-worker:v1"
	template.Spec.Entrypoint.Command = []string{"/bin/sh"}
	if err := validateReferenceTemplate(template, installed); err == nil {
		t.Fatal("incompatible worker entrypoint accepted")
	}
}
