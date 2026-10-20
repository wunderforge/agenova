// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package bundled

import (
	"encoding/json"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/platformapply"
	"gopkg.in/yaml.v3"
	"os"
	"testing"
)

func TestObservabilityReferenceComposition(t *testing.T) {
	data, err := os.ReadFile("../../../deploy/reference/platform.observability.kind.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, validation := v0.ParsePlatformYAML(data)
	if validation != nil {
		t.Fatal(validation)
	}
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	resolved, lock, failure := platform.Resolve(doc, registry)
	if failure != nil {
		t.Fatal(failure)
	}
	request := platformapply.DeploymentRequest{Platform: resolved, Lock: lock, Config: resolved.Instances[0].Config}
	// Locate by capability instead of relying on sort order.
	for _, instance := range resolved.Instances {
		if instance.Category == platform.CapabilityDeployment {
			request.Config = instance.Config
		}
	}
	if err := validateStackComposition(request); err != nil {
		t.Fatal(err)
	}
	steps, err := referenceSteps(request, "test-observation", true)
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].object["kind"] != "Namespace" {
		t.Fatal("namespace must precede resources")
	}
	if output := os.Getenv("AGENOVA_OBSERVABILITY_RENDER"); output != "" {
		objects := []map[string]any{}
		for _, step := range stackSteps(request, "agenova-observability-180") {
			objects = append(objects, step.object)
		}
		b, _ := json.Marshal(objects)
		if err := os.WriteFile(output, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	deployments := 0
	pvc := 0
	for _, step := range steps {
		data, err := yaml.Marshal(step.object)
		if err != nil || len(data) == 0 {
			t.Fatal("unrenderable object")
		}
		if stackKind(step) == "deployment" {
			deployments++
		}
		if stackKind(step) == "persistentvolumeclaim" {
			pvc++
		}
		normalizedBytes, _ := json.Marshal(step.object)
		var actual map[string]any
		json.Unmarshal(normalizedBytes, &actual)
		if step.category == "observability" && stackKind(step) != "deployment" && !stackMatches(actual, step, resolved.Revision) {
			t.Fatalf("non-idempotent %s", step.name)
		}
	}
	if deployments != 6 || pvc != 5 {
		t.Fatalf("components missing: deployments=%d pvc=%d", deployments, pvc)
	}
	// A reference without explicit telemetry must not provision a stack.
	request.Config = map[string]any{"context": "kind-test", "namespace": "test"}
	if len(stackSteps(request, "test")) != 0 {
		t.Fatal("implicit stack")
	}
}
