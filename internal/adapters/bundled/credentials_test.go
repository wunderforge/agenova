// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/credentials"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/platformapply"
)

func credentialRequest() platformapply.DeploymentRequest {
	request := deploymentRequest()
	request.Platform.Adapters = []platform.ResolvedAdapter{{Name: "model-adapter", ID: OpenAICompatibleModelID, Version: ReferenceVersion}, {Name: "secret-adapter", ID: KubernetesSecretCredentialID, Version: ReferenceVersion}}
	request.Platform.Instances[1].AdapterRef = "model-adapter"
	request.Platform.Instances[1].CredentialRef = &v0.PlatformCredentialReference{ResolverRef: "host-secrets", Name: "model-token", Key: "token"}
	request.Platform.Instances = append(request.Platform.Instances, platform.ResolvedInstance{Category: platform.CapabilityCredential, Name: "host-secrets", AdapterRef: "secret-adapter", Config: map[string]any{"namespace": "agenova-system"}})
	return request
}

func syntheticSecret(namespace, name string) []byte {
	data, _ := json.Marshal(map[string]any{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]any{"namespace": namespace, "name": name}, "type": "Opaque", "data": map[string][]byte{"token": []byte("synthetic-test-token")}})
	return data
}

func TestReferenceCredentialCompositionFailsBeforeIO(t *testing.T) {
	cases := map[string]func(*platform.ResolvedPlatform){
		"unknown resolver":     func(p *platform.ResolvedPlatform) { p.Instances[1].CredentialRef.ResolverRef = "other" },
		"missing resolver":     func(p *platform.ResolvedPlatform) { p.Instances = p.Instances[:2] },
		"wrong namespace":      func(p *platform.ResolvedPlatform) { p.Instances[2].Config["namespace"] = "other" },
		"extra config":         func(p *platform.ResolvedPlatform) { p.Instances[2].Config["path"] = "/tmp/token" },
		"unsupported resolver": func(p *platform.ResolvedPlatform) { p.Adapters[1].ID = "example.io/credential/custom" },
		"unsupported version":  func(p *platform.ResolvedPlatform) { p.Adapters[1].Version = "2.0.0" },
		"unsupported model":    func(p *platform.ResolvedPlatform) { p.Adapters[0].ID = "example.io/model/custom" },
		"bad name":             func(p *platform.ResolvedPlatform) { p.Instances[1].CredentialRef.Name = "../other" },
		"bad key":              func(p *platform.ResolvedPlatform) { p.Instances[1].CredentialRef.Key = "token/path" },
		"runtime ref":          func(p *platform.ResolvedPlatform) { p.Instances[0].CredentialRef = p.Instances[1].CredentialRef },
		"unused resolver":      func(p *platform.ResolvedPlatform) { p.Instances[1].CredentialRef = nil },
		"mixed selections": func(p *platform.ResolvedPlatform) {
			p.Instances = append(p.Instances, platform.ResolvedInstance{Category: platform.CapabilityModel, Name: "other-model", Config: map[string]any{"endpoint": "http://host.docker.internal:11434/v1"}})
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			request := credentialRequest()
			mutate(request.Platform)
			gets := 0
			get := func(context.Context, string, string) ([]byte, error) {
				gets++
				return nil, errors.New("private backend text")
			}
			binding, err := ReferenceModelCredential(request.Platform, "agenova-system", get)
			if !errors.Is(err, credentials.ErrConfiguration) || binding != nil || gets != 0 {
				t.Fatalf("invalid composition did not fail closed: binding=%v err=%v gets=%d", binding, err, gets)
			}
			runner := &fakeKubectl{}
			_, _, _, err = newKubernetesDeployment(runner).Plan(context.Background(), request)
			if err == nil || len(runner.calls) != 0 {
				t.Fatalf("invalid composition reached target: err=%v calls=%d", err, len(runner.calls))
			}
		})
	}
}

func TestReferenceCredentialPlanIsSecretFreeAndBindingIsCaptured(t *testing.T) {
	request := credentialRequest()
	gets := 0
	get := func(_ context.Context, namespace, name string) ([]byte, error) {
		gets++
		if namespace != "agenova-system" || name != "model-token" {
			t.Fatal("retargeted captured reference")
		}
		return syntheticSecret(namespace, name), nil
	}
	binding, err := ReferenceModelCredential(request.Platform, "agenova-system", get)
	if err != nil || gets != 0 {
		t.Fatalf("construction performed IO: %v %d", err, gets)
	}
	runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
		if contains(args, "version") {
			return commandResult{stdout: "ok"}, nil
		}
		return commandResult{stderr: "NotFound"}, errors.New("not found")
	}}
	adapter := newKubernetesDeployment(runner)
	adapter.credentialGetter = get
	if _, _, _, err := adapter.Plan(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if gets != 0 {
		t.Fatal("Plan fetched Secret")
	}
	for _, call := range runner.calls {
		if contains(call, "secret") || contains(call, "secrets") {
			t.Fatal("Plan discovered Secret")
		}
	}
	request.Platform.Instances[1].CredentialRef.Name = "other"
	if err := binding.Use(context.Background(), func(_ context.Context, value []byte) error {
		if string(value) != "synthetic-test-token" {
			t.Fatal("wrong material")
		}
		return nil
	}); err != nil || gets != 1 {
		t.Fatalf("captured binding use: %v %d", err, gets)
	}
}

func TestCredentialApplyChecksEvenEmptyPlanBeforeMutation(t *testing.T) {
	for _, available := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "ready"}[available], func(t *testing.T) {
			request := credentialRequest()
			request.TargetChanges = nil
			runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
				args = append([]string(nil), args...)
				singular := map[string]string{"namespaces": "namespace", "roles.rbac.authorization.k8s.io": "role", "rolebindings.rbac.authorization.k8s.io": "rolebinding", "deployments.apps": "deployment", "services": "service", "serviceaccounts": "serviceaccount"}
				for i, arg := range args {
					if replacement := singular[arg]; replacement != "" {
						args[i] = replacement
					}
				}
				if contains(args, "can-i") {
					return commandResult{stdout: "yes"}, nil
				}
				if contains(args, "get") && contains(args, "name") {
					return commandResult{stdout: "exists"}, nil
				}
				if contains(args, "version") {
					return commandResult{stdout: "ok"}, nil
				}
				if contains(args, "role") {
					data, _ := json.Marshal(roleObject("agenova-system", "model-token"))
					return commandResult{stdout: string(data)}, nil
				}
				if result, ok := readyResourceResult(args, request); ok {
					return result, nil
				}
				return commandResult{}, errors.New("unexpected target operation")
			}}
			gets := 0
			adapter := newKubernetesDeployment(runner)
			adapter.credentialGetter = func(_ context.Context, namespace, name string) ([]byte, error) {
				gets++
				if !available {
					return nil, errors.New("private Secret detail")
				}
				return syntheticSecret(namespace, name), nil
			}
			_, mutated, err := adapter.Apply(context.Background(), request)
			if gets != 1 || mutated || (!available && !errors.Is(err, credentials.ErrUnavailable)) || (available && err != nil) {
				t.Fatalf("empty apply readiness: gets=%d mutated=%v err=%v", gets, mutated, err)
			}
			for _, call := range runner.calls {
				if !contains(call, "can-i") && (contains(call, "apply") || contains(call, "patch") || contains(call, "create")) {
					t.Fatal("readiness wrote target")
				}
			}
		})
	}
}

func TestCredentialRoleAndAuthorityAreExactNamedGet(t *testing.T) {
	request := credentialRequest()
	steps, err := referenceSteps(request, "agenova-system", false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, step := range steps {
		if step.name == controlPlaneRole {
			for _, entry := range step.object["rules"].([]any) {
				rule := entry.(map[string]any)
				if reflect.DeepEqual(rule["resources"], []any{"secrets"}) {
					found = true
					if !reflect.DeepEqual(rule["verbs"], []any{"get"}) || !reflect.DeepEqual(rule["resourceNames"], []any{"model-token"}) {
						t.Fatal("Secret permission exceeds exact named get")
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("selected Secret permission missing")
	}
	for _, permit := range []bool{false, true} {
		named := 0
		runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
			if contains(args, "escalate") || contains(args, "bind") {
				return commandResult{stdout: "no"}, errors.New("denied")
			}
			if contains(args, "secrets") {
				t.Fatal("preflight requested broad Secret get")
			}
			if contains(args, "secrets/model-token") {
				named++
				if !permit {
					return commandResult{stdout: "no"}, errors.New("denied")
				}
			}
			return commandResult{stdout: "yes"}, nil
		}}
		err := newKubernetesDeployment(runner).preflightRoleAuthority(context.Background(), "kind-agenova", "agenova-system", []platformapply.Change{{Component: controlPlaneRole}}, "model-token")
		if named != 1 || ((err == nil) != permit) {
			t.Fatalf("named grant preflight: calls=%d err=%v", named, err)
		}
	}
	objects := make([]map[string]any, len(steps))
	for i, step := range steps {
		objects[i] = step.object
	}
	encoded, _ := json.Marshal(objects)
	if !strings.Contains(string(encoded), "model-token") {
		t.Fatal("scan did not include credential reference metadata")
	}
	if strings.Contains(string(encoded), "synthetic-test-token") {
		t.Fatal("material entered manifests")
	}
}
