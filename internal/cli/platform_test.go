// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/platformapply"
	"gopkg.in/yaml.v3"
)

type cliDeployment struct{ applied bool }

func (d *cliDeployment) Preflight(context.Context, platformapply.DeploymentRequest) error { return nil }

func (d *cliDeployment) Plan(_ context.Context, _ platformapply.DeploymentRequest) (string, []platformapply.Change, []platformapply.ComponentStatus, error) {
	if d.applied {
		return "test/target", nil, []platformapply.ComponentStatus{{Name: "control", Category: "deployment", State: "available"}}, nil
	}
	return "test/target", []platformapply.Change{{Component: "control", Action: "create", Detail: "test"}}, []platformapply.ComponentStatus{{Name: "control", Category: "deployment", State: "unavailable"}}, nil
}

func (d *cliDeployment) Apply(_ context.Context, _ platformapply.DeploymentRequest) ([]platformapply.ComponentStatus, error) {
	d.applied = true
	return []platformapply.ComponentStatus{{Name: "control", Category: "deployment", State: "available"}}, nil
}

func TestPlatformCLIValidatePlanConfirmApplyAndNoOp(t *testing.T) {
	path := writePlatformFile(t)
	deployment := &cliDeployment{}
	factory := cliPlatformFactory(t, deployment)

	stdout, stderr, code := runPlatformCLI([]string{"agenova", "platform", "validate", "-f", path, "--json"}, "", factory)
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"valid":true`) {
		t.Fatalf("validate = %d %q %q", code, stdout, stderr)
	}
	stdout, stderr, code = runPlatformCLI([]string{"agenova", "platform", "plan", "-f", path, "--json"}, "", factory)
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"target":"test/target"`) || !strings.Contains(stdout, `"action":"activate"`) {
		t.Fatalf("plan = %d %q %q", code, stdout, stderr)
	}
	_, stderr, code = runPlatformCLI([]string{"agenova", "platform", "apply", "-f", path}, "no\n", factory)
	if code != 1 || !strings.Contains(stderr, "apply cancelled") || deployment.applied {
		t.Fatalf("cancel = %d %q applied=%t", code, stderr, deployment.applied)
	}
	stdout, stderr, code = runPlatformCLI([]string{"agenova", "platform", "apply", "-f", path, "--yes", "--json"}, "", factory)
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"applied":true`) || !strings.Contains(stdout, `"ready":true`) {
		t.Fatalf("apply = %d %q %q", code, stdout, stderr)
	}
	stdout, stderr, code = runPlatformCLI([]string{"agenova", "platform", "apply", "-f", path, "--yes", "--json"}, "", factory)
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"applied":false`) || !strings.Contains(stdout, `"changes":[]`) {
		t.Fatalf("no-op apply = %d %q %q", code, stdout, stderr)
	}
}

func runPlatformCLI(args []string, input string, factory PlatformServiceFactory) (string, string, int) {
	var stdout, stderr bytes.Buffer
	code := MainWithServices(args, &stdout, &stderr, Services{NewPlatform: factory, Input: strings.NewReader(input)})
	return stdout.String(), stderr.String(), code
}

func cliPlatformFactory(t *testing.T, deployment *cliDeployment) PlatformServiceFactory {
	t.Helper()
	registry, err := adapterregistry.New(cliRegistration("example.com/deployment/test", platform.CapabilityDeployment, func() (any, error) { return deployment, nil }), cliRegistration("example.com/runtime/test", platform.CapabilityRuntime, func() (any, error) { return &struct{}{}, nil }), cliRegistration("example.com/model/test", platform.CapabilityModel, func() (any, error) { return &struct{}{}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := adapterregistry.NewLifecycle(registry, adapterregistry.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	service := platformapply.Service{Adapters: lifecycle}
	return func(string) (platformapply.Service, error) { return service, nil }
}

func cliRegistration(id string, capability platform.Capability, factory adapterregistry.Factory) adapterregistry.Registration {
	descriptor := platform.Descriptor{ID: id, Version: "1.0.0", Capabilities: []platform.Capability{capability}, CanonicalizeInstance: func(_ platform.Capability, config map[string]any) (map[string]any, error) { return config, nil }}
	manifest := adapterregistry.Manifest{ID: id, Version: "1.0.0", Protocol: adapterregistry.ProtocolVersion, Capabilities: descriptor.Capabilities}
	if capability != platform.CapabilityDeployment {
		descriptor.CanonicalizeProfile = func(_ platform.Capability, _, config map[string]any) (map[string]any, error) { return config, nil }
	}
	return adapterregistry.Registration{Manifest: manifest, Descriptor: descriptor, Factories: map[platform.Capability]adapterregistry.Factory{capability: factory}}
}

func writePlatformFile(t *testing.T) string {
	t.Helper()
	input := v1alpha1.Platform{APIVersion: v1alpha1.PlatformAPIVersion, Kind: v1alpha1.PlatformKind, Metadata: v1alpha1.ObjectMeta{Name: "reference"}, Spec: v1alpha1.PlatformSpec{
		Adapters:         []v1alpha1.PlatformAdapterRequirement{{Name: "deployment", ID: "example.com/deployment/test", Version: "1.0.0"}, {Name: "runtime", ID: "example.com/runtime/test", Version: "1.0.0"}, {Name: "model", ID: "example.com/model/test", Version: "1.0.0"}},
		Infrastructure:   v1alpha1.PlatformInfrastructure{Deployment: &v1alpha1.PlatformInstance{Name: "control", AdapterRef: "deployment"}, RuntimeBackends: []v1alpha1.PlatformInstance{{Name: "runtime", AdapterRef: "runtime"}}, RuntimeProfiles: []v1alpha1.PlatformProfile{{Name: "standard", BackendRef: "runtime"}}},
		Services:         v1alpha1.PlatformServices{ModelBackends: []v1alpha1.PlatformInstance{{Name: "model", AdapterRef: "model"}}, ModelProfiles: []v1alpha1.PlatformProfile{{Name: "coding", BackendRef: "model"}}},
		InitialPolicyRef: &v1alpha1.PlatformPolicyReference{ID: platformapply.ReferencePolicyID, Version: platformapply.ReferencePolicyVersion},
	}}
	data, err := yaml.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "platform.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
