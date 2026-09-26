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
	if len(catalog.Adapters) != 4 || catalog.Adapters[0].ID != bundled.KubernetesDeploymentID || catalog.Adapters[1].ID != bundled.OpenAICompatibleModelID || catalog.Adapters[2].ID != bundled.AgentSandboxRuntimeID || catalog.Adapters[3].ID != bundled.MCPHTTPToolID {
		t.Fatalf("catalog = %#v", catalog.Adapters)
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
