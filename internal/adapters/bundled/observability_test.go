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

func TestGrafanaGoogleSessionProxyRequiresPrivateIngress(t *testing.T) {
	input := map[string]any{"storageClass": "gp2", "grafanaAdminSecretRef": "grafana-admin", "grafanaAuthProxy": map[string]any{"publicOrigin": "https://app.example", "namespace": "observability", "app": "agenova-grafana-proxy"}}
	config, err := canonicalizeStack(input)
	if err != nil {
		t.Fatal(err)
	}
	steps := stackSteps(platformapply.DeploymentRequest{Config: map[string]any{"observability": config}, Platform: &platform.ResolvedPlatform{Revision: "test"}}, "observability")
	if stackKind(steps[0]) != "networkpolicy" {
		t.Fatal("private ingress must precede auth-proxy workload")
	}
	found := false
	for _, step := range steps {
		if step.name != "agenova-grafana" {
			continue
		}
		raw, _ := json.Marshal(step.object)
		var obj map[string]any
		json.Unmarshal(raw, &obj)
		selector := steps[0].object["spec"].(map[string]any)["podSelector"].(map[string]any)["matchLabels"].(map[string]any)
		labels := obj["spec"].(map[string]any)["template"].(map[string]any)["metadata"].(map[string]any)["labels"].(map[string]any)
		for key, value := range selector {
			if labels[key] != value {
				t.Fatal("private policy does not select the Grafana workload")
			}
		}
		container := obj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)
		env := map[string]any{}
		for _, v := range container["env"].([]any) {
			e := v.(map[string]any)
			env[e["name"].(string)] = e["value"]
		}
		if env["GF_AUTH_PROXY_ENABLED"] != "true" || env["GF_AUTH_BASIC_ENABLED"] != "false" || env["GF_AUTH_PROXY_ENABLE_LOGIN_TOKEN"] != "false" || env["GF_SERVER_ROOT_URL"] != "https://app.example/grafana/" {
			t.Fatal("auth proxy permits bypass or wrong URL")
		}
		found = true
	}
	if !found {
		t.Fatal("Grafana missing")
	}
	input["grafanaAuthProxy"].(map[string]any)["publicOrigin"] = "http://insecure.example"
	if _, err := canonicalizeStack(input); err == nil {
		t.Fatal("non-HTTPS public entry accepted")
	}
}
