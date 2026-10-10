// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/adapters/bundled"
	"github.com/wunderforge/agenova/internal/platform"
)

func TestAdapterCommandsStableJSONLifecycle(t *testing.T) {
	factory := adapterTestFactory(t)

	stdout, stderr, code := runAdapterCLI([]string{"agenova", "adapters", "catalog", "--json"}, factory)
	if code != 0 || stderr != "" {
		t.Fatalf("catalog = %d, %q, %q", code, stdout, stderr)
	}
	var catalog struct {
		Adapters []adapterregistry.Manifest `json:"adapters"`
	}
	if err := json.Unmarshal([]byte(stdout), &catalog); err != nil {
		t.Fatal(err)
	}
	wantIDs := []string{bundled.KubernetesSecretCredentialID, bundled.KubernetesDeploymentID, bundled.OpenAICompatibleModelID, bundled.AgentSandboxRuntimeID}
	if len(catalog.Adapters) != len(wantIDs) {
		t.Fatalf("catalog = %#v", catalog.Adapters)
	}
	for index, want := range wantIDs {
		if catalog.Adapters[index].ID != want {
			t.Fatalf("catalog[%d] = %q, want %q", index, catalog.Adapters[index].ID, want)
		}
	}

	reference := bundled.AgentSandboxRuntimeID + "@" + bundled.ReferenceVersion
	stdout, stderr, code = runAdapterCLI([]string{"agenova", "adapters", "install", reference, "--json"}, factory)
	wantFirst := `{"adapter":{"id":"agenova.io/runtime/agent-sandbox","version":"0.1.0","protocol":"agenova.adapter/v1alpha1","capabilities":["runtime"]},"changed":true}` + "\n"
	if code != 0 || stderr != "" || stdout != wantFirst {
		t.Fatalf("first install = %d, %q, %q", code, stdout, stderr)
	}
	stdout, stderr, code = runAdapterCLI([]string{"agenova", "adapters", "install", reference, "--json"}, factory)
	if code != 0 || stderr != "" || stdout != strings.Replace(wantFirst, `"changed":true`, `"changed":false`, 1) {
		t.Fatalf("idempotent install = %d, %q, %q", code, stdout, stderr)
	}
	stdout, stderr, code = runAdapterCLI([]string{"agenova", "adapters", "list", "--json"}, factory)
	wantList := `{"apiVersion":"agenova.io/v1alpha1","kind":"AdapterLock","adapters":[{"id":"agenova.io/runtime/agent-sandbox","version":"0.1.0","protocol":"agenova.adapter/v1alpha1","capabilities":["runtime"]}]}` + "\n"
	if code != 0 || stderr != "" || stdout != wantList {
		t.Fatalf("list = %d, %q, %q", code, stdout, stderr)
	}
	stdout, stderr, code = runAdapterCLI([]string{"agenova", "adapters", "inspect", reference, "--json"}, factory)
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"installed":true`) || !strings.Contains(stdout, `"profileSchema"`) {
		t.Fatalf("inspect = %d, %q, %q", code, stdout, stderr)
	}
}

func TestAdapterInitYAMLIsReviewablePlatformFragment(t *testing.T) {
	factory := adapterTestFactory(t)
	reference := bundled.AgentSandboxRuntimeID + "@" + bundled.ReferenceVersion
	stdout, stderr, code := runAdapterCLI([]string{"agenova", "adapters", "init", reference, "--name", "primary-runtime"}, factory)
	want := `spec:
    adapters:
        - name: primary-runtime
          id: agenova.io/runtime/agent-sandbox
          version: 0.1.0
    infrastructure:
        runtimeBackends:
            - name: primary-runtime
              adapterRef: primary-runtime
              config:
                compatible-worker-image: agenova-testworker:kind
                connection:
                    mode: in-cluster
                    namespace: agenova-system
        runtimeProfiles:
            - name: primary-runtime-profile
              backendRef: primary-runtime
              config:
                isolation: dedicated
`
	if code != 0 || stderr != "" || stdout != want {
		t.Fatalf("init = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if strings.Contains(stdout, "credential") || strings.Contains(stdout, "authority") {
		t.Fatalf("init output implied forbidden state: %s", stdout)
	}
}

func TestCredentialAdapterCLIActivationAndInitContainMetadataOnly(t *testing.T) {
	bundledRegistry, err := bundled.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	var manifest adapterregistry.Manifest
	for _, candidate := range bundledRegistry.Catalog() {
		if candidate.ID == bundled.KubernetesSecretCredentialID {
			manifest = candidate
		}
	}
	descriptor, ok := bundledRegistry.Lookup(manifest.ID, manifest.Version)
	if !ok {
		t.Fatal("credential descriptor is unavailable")
	}
	constructCalls := 0
	registry, err := adapterregistry.New(adapterregistry.Registration{
		Manifest: manifest, Descriptor: descriptor,
		Factories: map[platform.Capability]adapterregistry.Factory{
			platform.CapabilityCredential: func() (any, error) { constructCalls++; return &struct{}{}, nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	store := adapterregistry.NewMemoryStore()
	factory := AdapterLifecycleFactory(func(string) (*adapterregistry.Lifecycle, error) {
		return adapterregistry.NewLifecycle(registry, store)
	})
	reference := bundled.KubernetesSecretCredentialID + "@" + bundled.ReferenceVersion
	wantInstall := `{"adapter":{"id":"agenova.io/credential/kubernetes-secret","version":"0.1.0","protocol":"agenova.adapter/v1alpha1","capabilities":["credential"]},"changed":true}` + "\n"
	for index := 0; index < 2; index++ {
		stdout, stderr, code := runAdapterCLI([]string{"agenova", "adapters", "install", reference, "--json"}, factory)
		want := wantInstall
		if index > 0 {
			want = strings.Replace(want, `"changed":true`, `"changed":false`, 1)
		}
		if code != 0 || stderr != "" || stdout != want {
			t.Fatalf("credential activation = %d, %q, %q", code, stdout, stderr)
		}
	}
	stdout, stderr, code := runAdapterCLI([]string{"agenova", "adapters", "inspect", reference, "--json"}, factory)
	var inspected adapterregistry.InspectResult
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &inspected) != nil || !inspected.Installed || len(inspected.Manifest.InstanceSchema.Fields) != 1 || inspected.Manifest.InstanceSchema.Fields[0].Path != "namespace" || len(inspected.Manifest.ProfileSchema.Fields) != 0 {
		t.Fatalf("credential inspection = %d, %q, %q", code, stdout, stderr)
	}
	stdout, stderr, code = runAdapterCLI([]string{"agenova", "adapters", "init", reference, "--name", "host-secrets", "--json"}, factory)
	var fragment adapterregistry.PlatformFragment
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &fragment) != nil || fragment.Spec.Services == nil || len(fragment.Spec.Services.CredentialResolvers) != 1 || fragment.Spec.Infrastructure != nil {
		t.Fatalf("credential init = %d, %q, %q", code, stdout, stderr)
	}
	resolver := fragment.Spec.Services.CredentialResolvers[0]
	if resolver.Name != "host-secrets" || resolver.AdapterRef != "host-secrets" || resolver.CredentialRef != nil || len(resolver.Config) != 1 || resolver.Config["namespace"] != "agenova-system" || len(fragment.Spec.Services.ModelBackends) != 0 || len(fragment.Spec.Services.ModelProfiles) != 0 {
		t.Fatal("credential init invented a selected credential or consumer")
	}
	// Activation and initialization produce catalog metadata and never construct
	// a live Getter or resolve Secret material. A reference is selected later in
	// the operator's typed model-backend field.
	stdout, stderr, code = runAdapterCLI([]string{"agenova", "adapters", "list", "--json"}, factory)
	wantList := `{"apiVersion":"agenova.io/v1alpha1","kind":"AdapterLock","adapters":[{"id":"agenova.io/credential/kubernetes-secret","version":"0.1.0","protocol":"agenova.adapter/v1alpha1","capabilities":["credential"]}]}` + "\n"
	if code != 0 || stderr != "" || stdout != wantList {
		t.Fatalf("credential list = %d, %q, %q", code, stdout, stderr)
	}
	if constructCalls != 0 {
		t.Fatal("metadata command constructed a credential backend")
	}
	// Positive control connects the counter to the same active registration.
	lifecycle, err := factory("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lifecycle.Construct(manifest.ID, manifest.Version, platform.CapabilityCredential); err != nil || constructCalls != 1 {
		t.Fatal("credential construction spy is disconnected")
	}
}

func TestAdapterCommandNegativeCases(t *testing.T) {
	factory := adapterTestFactory(t)
	tests := []struct {
		args []string
		code int
		want string
	}{
		{[]string{"agenova", "adapters"}, ExitUsage, "Usage:"},
		{[]string{"agenova", "adapters", "unknown"}, ExitUsage, `unknown adapters command "unknown"`},
		{[]string{"agenova", "adapters", "inspect", "not-qualified"}, ExitUsage, "qualified adapter ID"},
		{[]string{"agenova", "adapters", "install"}, ExitUsage, "install requires"},
		{[]string{"agenova", "adapters", "inspect", bundled.AgentSandboxRuntimeID, "--name", "wrong"}, ExitUsage, "--name is only valid"},
		{[]string{"agenova", "run", "--state-dir", "somewhere"}, ExitUsage, "run requires -f"},
	}
	for _, test := range tests {
		stdout, stderr, code := runAdapterCLI(test.args, factory)
		if code != test.code || !strings.Contains(stdout+stderr, test.want) {
			t.Fatalf("%v = %d, %q, %q; want %d and %q", test.args, code, stdout, stderr, test.code, test.want)
		}
	}
}

func adapterTestFactory(t *testing.T) AdapterLifecycleFactory {
	t.Helper()
	registry, err := bundled.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	store := adapterregistry.NewMemoryStore()
	return func(string) (*adapterregistry.Lifecycle, error) {
		return adapterregistry.NewLifecycle(registry, store)
	}
}

func runAdapterCLI(args []string, factory AdapterLifecycleFactory) (string, string, int) {
	var stdout, stderr bytes.Buffer
	code := MainWithServices(args, &stdout, &stderr, Services{NewAdapters: factory})
	return stdout.String(), stderr.String(), code
}
