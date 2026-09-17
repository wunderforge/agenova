// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"strings"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/platform"
)

func TestReferenceTemplateCompatibilityBeforeRegistration(t *testing.T) {
	installed := &platform.ResolvedPlatform{Instances: []platform.ResolvedInstance{{Category: platform.CapabilityRuntime, Config: map[string]any{"compatible-worker-image": "trusted-worker:v1"}}}}
	allowed := v0.Duration(5 * time.Minute)
	template := &v0.AgentTemplate{Metadata: v0.ObjectMeta{Name: "engineer"}, Spec: v0.AgentTemplateSpec{Artifact: &v0.AgentTemplateArtifact{Image: "trusted-worker:v1"}, Entrypoint: &v0.AgentTemplateEntrypoint{Command: []string{"/agenova-workerctl", "serve"}}, CapabilityCeiling: &v0.AgentTemplateCapabilityCeiling{MaxTimeout: &allowed}}}
	if err := validateReferenceTemplate(template, installed); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{strings.Repeat("a", 59), "Invalid-Name", "bad.name"} {
		template.Metadata.Name = name
		if err := validateReferenceTemplate(template, installed); err == nil {
			t.Fatalf("unusable template name %q accepted", name)
		}
	}
	template.Metadata.Name = strings.Repeat("a", 58)
	if err := validateReferenceTemplate(template, installed); err != nil {
		t.Fatalf("maximum supported template name rejected: %v", err)
	}
	template.Metadata.Name = "engineer"
	template.Spec.Artifact.Image = "unverified-worker:v1"
	if err := validateReferenceTemplate(template, installed); err == nil {
		t.Fatal("unverified worker image accepted")
	}
	template.Spec.Artifact.Image = "trusted-worker:v1"
	template.Spec.Entrypoint.Command = []string{"/bin/sh"}
	if err := validateReferenceTemplate(template, installed); err == nil {
		t.Fatal("incompatible worker entrypoint accepted")
	}
	template.Spec.Entrypoint.Command = []string{"/agenova-workerctl", "serve"}
	tooLong := v0.Duration(6 * time.Minute)
	template.Spec.CapabilityCeiling.MaxTimeout = &tooLong
	if err := validateReferenceTemplate(template, installed); err == nil {
		t.Fatal("timeout above the controlled worker execution limit accepted")
	}
	template.Spec.CapabilityCeiling.MaxTimeout = &allowed
	if err := validateReferenceTemplate(template, installed); err != nil {
		t.Fatalf("five-minute timeout rejected: %v", err)
	}
	template.Spec.CapabilityCeiling.MaxTimeout = nil
	if err := validateReferenceTemplate(template, installed); err == nil {
		t.Fatal("unbounded worker execution timeout accepted")
	}
}
