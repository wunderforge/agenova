// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/policy"
)

func TestKubernetesStoreReferenceRegistrationAndActivation(t *testing.T) {
	objects := map[string]map[string]any{}
	mutations := 0
	allowMutation := true
	store := KubernetesStore{Context: "kind-test", Namespace: "agenova-system", Invoke: func(_ context.Context, input []byte, args ...string) ([]byte, error) {
		if len(args) < 5 || args[0] != "--context" || args[2] != "--namespace" || args[3] != "agenova-system" {
			t.Fatalf("unpinned target: %v", args)
		}
		command := args[4:]
		switch command[0] {
		case "auth":
			if len(command) != 4 || command[1] != "can-i" {
				t.Fatalf("unexpected ConfigMap authorization: %v", command)
			}
			if command[2] == "patch" && !strings.HasPrefix(command[3], "configmaps/") {
				t.Fatalf("patch authorization did not name its ConfigMap: %v", command)
			}
			if command[2] == "create" && command[3] != "configmaps" {
				t.Fatalf("create authorization must be namespace-wide: %v", command)
			}
			if !allowMutation {
				return []byte("no\n"), nil
			}
			return []byte("yes\n"), nil
		case "get":
			if command[2] == "-l" {
				items := make([]map[string]any, 0, len(objects))
				for _, object := range objects {
					items = append(items, object)
				}
				return json.Marshal(map[string]any{"items": items})
			}
			object, ok := objects[command[2]]
			if !ok {
				return nil, errMissingRecord
			}
			return json.Marshal(object)
		case "create":
			var object map[string]any
			if err := json.Unmarshal(input, &object); err != nil {
				t.Fatal(err)
			}
			metadata := object["metadata"].(map[string]any)
			name := metadata["name"].(string)
			if _, exists := objects[name]; exists {
				return nil, fmt.Errorf("already exists")
			}
			metadata["resourceVersion"] = "1"
			objects[name] = object
			mutations++
			return nil, nil
		case "patch":
			t.Fatal("identical activation patched the pointer")
		}
		return nil, fmt.Errorf("unexpected command: %v", command)
	}}
	service := Service{Store: store}
	policyYAML := []byte(`apiVersion: agenova.io/v1alpha1
kind: PolicyBundle
metadata: {name: reference-default-deny, version: "1"}
spec:
  rules:
    - {team: team-a, action: claim.create, project: payments, templateRef: engineer}
`)
	for i := 0; i < 2; i++ {
		result, err := service.ApplyPolicy(policyYAML)
		if err != nil || result.Changed != (i == 0) {
			t.Fatalf("apply %d = %#v, %v", i, result, err)
		}
	}
	if mutations != 2 {
		t.Fatalf("repeat changed %d objects, want immutable record and one active pointer", mutations)
	}
	active, err := store.ActivePolicy()
	if err != nil || !EqualJSON(active, policy.ReferenceBundle()) {
		t.Fatalf("active = %#v, %v", active, err)
	}
	conflict := policy.ReferenceBundle()
	conflict.Rules[0].Team = "team-b"
	if _, err := store.PutPolicy(conflict); !errors.Is(err, ErrConflict) {
		t.Fatalf("mutated seed accepted: %v", err)
	}
	if mutations != 2 {
		t.Fatal("conflicting reference policy mutated Kubernetes")
	}
}

func TestKubernetesPolicyApplyPreflightsActivationBeforeRecordCreation(t *testing.T) {
	for _, pointerExists := range []bool{false, true} {
		t.Run(fmt.Sprintf("pointer-exists=%t", pointerExists), func(t *testing.T) {
			mutations := 0
			store := KubernetesStore{Namespace: "agenova-system", Invoke: func(_ context.Context, _ []byte, args ...string) ([]byte, error) {
				command := args[2:]
				switch command[0] {
				case "get":
					if pointerExists {
						return []byte(`{"metadata":{"name":"agenova-active-policy","resourceVersion":"1","labels":{"app.kubernetes.io/managed-by":"agenova"}}}`), nil
					}
					return nil, errMissingRecord
				case "auth":
					if pointerExists && (len(command) != 4 || command[2] != "patch" || command[3] != "configmaps/agenova-active-policy") {
						t.Fatalf("expected named patch preflight: %v", command)
					}
					if !pointerExists && (len(command) != 4 || command[2] != "create" || command[3] != "configmaps") {
						t.Fatalf("expected create preflight: %v", command)
					}
					return []byte("no\n"), nil
				case "create", "patch":
					mutations++
				}
				return nil, nil
			}}
			_, err := (Service{Store: store}).ApplyPolicy([]byte(`apiVersion: agenova.io/v1alpha1
kind: PolicyBundle
metadata: {name: reference-default-deny, version: "1"}
spec:
  rules:
    - {team: team-a, action: claim.create, project: payments, templateRef: engineer}
`))
			if err == nil || mutations != 0 {
				t.Fatalf("unauthorized policy apply mutated registry: err=%v mutations=%d", err, mutations)
			}
		})
	}
}

func TestKubernetesStoreRejectsIdenticalReapplyWithoutMutationAuthority(t *testing.T) {
	objects := map[string]map[string]any{}
	allowMutation := true
	store := KubernetesStore{Namespace: "agenova-system", Invoke: func(_ context.Context, input []byte, args ...string) ([]byte, error) {
		command := args[2:]
		switch command[0] {
		case "auth":
			if !allowMutation {
				return []byte("no\n"), nil
			}
			return []byte("yes\n"), nil
		case "get":
			if command[2] == "-l" {
				items := make([]map[string]any, 0, len(objects))
				for _, object := range objects {
					items = append(items, object)
				}
				return json.Marshal(map[string]any{"items": items})
			}
			object, ok := objects[command[2]]
			if !ok {
				return nil, errMissingRecord
			}
			return json.Marshal(object)
		case "create":
			var object map[string]any
			if err := json.Unmarshal(input, &object); err != nil {
				t.Fatal(err)
			}
			metadata := object["metadata"].(map[string]any)
			metadata["resourceVersion"] = "1"
			objects[metadata["name"].(string)] = object
			return nil, nil
		default:
			t.Fatalf("unexpected command: %v", command)
			return nil, nil
		}
	}}
	template := &v0.AgentTemplate{APIVersion: "agenova.io/v1alpha1", Kind: v0.AgentTemplateKind, Metadata: v0.ObjectMeta{Name: "engineer"}, Spec: v0.AgentTemplateSpec{Artifact: &v0.AgentTemplateArtifact{Image: "example/agent:test"}, Entrypoint: &v0.AgentTemplateEntrypoint{Command: []string{"/agent"}}, CapabilityCeiling: &v0.AgentTemplateCapabilityCeiling{}}}
	if _, err := store.PutTemplate(template); err != nil {
		t.Fatalf("initial template: %v", err)
	}
	allowMutation = false
	if _, err := store.PutTemplate(template); err == nil {
		t.Fatal("identical template reapply succeeded without patch authority")
	}
	seed := policy.ReferenceBundle()
	if _, err := store.PutPolicy(seed); err != nil {
		t.Fatalf("initial policy: %v", err)
	}
	ref := PolicyReference{ID: seed.ID, Version: seed.Version}
	allowMutation = true
	if changed, err := store.ActivatePolicy(ref); err != nil || !changed {
		t.Fatalf("initial activation: changed=%t err=%v", changed, err)
	}
	allowMutation = false
	if _, err := store.ActivatePolicy(ref); err == nil {
		t.Fatal("identical policy activation succeeded without patch authority")
	}
}

func TestKubernetesReferenceRegistryRejectsSecondTemplateBeforeWrite(t *testing.T) {
	existing := &v0.AgentTemplate{APIVersion: "agenova.io/v1alpha1", Kind: v0.AgentTemplateKind, Metadata: v0.ObjectMeta{Name: "engineer"}, Spec: v0.AgentTemplateSpec{Artifact: &v0.AgentTemplateArtifact{Image: "agenova-testworker:kind"}, Entrypoint: &v0.AgentTemplateEntrypoint{Command: []string{"/agenova-workerctl", "serve"}}, CapabilityCeiling: &v0.AgentTemplateCapabilityCeiling{}}}
	encoded, err := json.Marshal(existing)
	if err != nil {
		t.Fatal(err)
	}
	mutations := 0
	store := KubernetesStore{Namespace: "agenova-system", Invoke: func(_ context.Context, _ []byte, args ...string) ([]byte, error) {
		command := args[2:]
		if command[0] == "get" && command[2] == "-l" {
			return json.Marshal(map[string]any{"items": []any{map[string]any{"metadata": map[string]string{"name": recordName("template", "engineer")}, "data": map[string]string{"template.json": string(encoded)}}}})
		}
		if command[0] == "create" || command[0] == "patch" {
			mutations++
		}
		return nil, fmt.Errorf("unexpected registry command: %v", command)
	}}
	other := *existing
	other.Metadata.Name = "reviewer"
	if _, err := store.PutTemplate(&other); err == nil || mutations != 0 {
		t.Fatalf("second template changed registry: err=%v mutations=%d", err, mutations)
	}
}

func TestKubernetesReferenceTemplateSlotIsAtomicAcrossNames(t *testing.T) {
	var mu sync.Mutex
	var stored map[string]any
	listed := 0
	listBarrier := make(chan struct{})
	store := KubernetesStore{Namespace: "agenova-system", Invoke: func(_ context.Context, input []byte, args ...string) ([]byte, error) {
		command := args[2:]
		switch command[0] {
		case "get":
			if command[2] == "-l" {
				mu.Lock()
				listed++
				if listed == 2 {
					close(listBarrier)
				}
				mu.Unlock()
				<-listBarrier
				return []byte(`{"items":[]}`), nil
			}
			mu.Lock()
			defer mu.Unlock()
			if stored == nil {
				return nil, errors.New("NotFound")
			}
			return json.Marshal(stored)
		case "create":
			var candidate map[string]any
			if err := json.Unmarshal(input, &candidate); err != nil {
				t.Fatal(err)
			}
			if candidate["metadata"].(map[string]any)["name"] != templateSlot {
				t.Fatalf("template was not written to atomic slot: %v", candidate)
			}
			mu.Lock()
			defer mu.Unlock()
			if stored != nil {
				return nil, errors.New("AlreadyExists")
			}
			stored = candidate
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected command: %v", command)
	}}
	result := make(chan error, 2)
	for _, name := range []string{"engineer", "reviewer"} {
		go func(name string) {
			template := &v0.AgentTemplate{APIVersion: "agenova.io/v1alpha1", Kind: v0.AgentTemplateKind, Metadata: v0.ObjectMeta{Name: name}, Spec: v0.AgentTemplateSpec{Artifact: &v0.AgentTemplateArtifact{Image: "example/agent:test"}, Entrypoint: &v0.AgentTemplateEntrypoint{Command: []string{"/agent"}}, CapabilityCeiling: &v0.AgentTemplateCapabilityCeiling{}}}
			_, err := store.PutTemplate(template)
			result <- err
		}(name)
	}
	first, second := <-result, <-result
	if !((first == nil && errors.Is(second, ErrConflict)) || (second == nil && errors.Is(first, ErrConflict))) {
		t.Fatalf("concurrent slot results = %v, %v", first, second)
	}
}

func TestKubernetesActivePolicyRejectsRecordWithWrongIdentity(t *testing.T) {
	ref := PolicyReference{ID: "expected", Version: "1"}
	pointer, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := json.Marshal(policy.ReferenceBundle())
	if err != nil {
		t.Fatal(err)
	}
	store := KubernetesStore{Namespace: "agenova-system", Invoke: func(_ context.Context, _ []byte, args ...string) ([]byte, error) {
		command := args[2:]
		if len(command) < 3 || command[0] != "get" {
			t.Fatalf("unexpected registry command: %v", command)
		}
		key := "policy.json"
		data := bundle
		if command[2] == "agenova-active-policy" {
			key, data = "reference.json", pointer
		} else if command[2] != recordName("policy", "expected@1") {
			t.Fatalf("unexpected policy record: %v", command)
		}
		return json.Marshal(map[string]any{"metadata": map[string]any{"labels": map[string]string{"app.kubernetes.io/managed-by": "agenova"}}, "data": map[string]string{key: string(data)}})
	}}
	if _, err := store.ActivePolicy(); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("mismatched policy record accepted: %v", err)
	}
}

func TestKubernetesTemplateDiscoveryUsesManagedRecordsOnly(t *testing.T) {
	template, parseErr := v0.ParseAgentTemplateYAML([]byte(`apiVersion: agenova.io/v1alpha1
kind: AgentTemplate
metadata: {name: engineer}
spec:
  artifact: {image: agenova-testworker:kind}
  entrypoint: {command: [/agenova-workerctl, serve]}
  capabilityCeiling:
    modelProfiles: [coding-standard]
    runtimeProfiles: [standard-isolated]
`))
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	encoded, err := json.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}
	listing, err := json.Marshal(map[string]any{"items": []any{
		map[string]any{"metadata": map[string]any{"name": recordName("template", "engineer")}, "data": map[string]string{"template.json": string(encoded)}},
		map[string]any{"metadata": map[string]any{"name": "unmanaged-unrelated"}, "data": map[string]string{"template.json": string(encoded)}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	store := KubernetesStore{Namespace: "agenova-system", Invoke: func(_ context.Context, _ []byte, args ...string) ([]byte, error) {
		if len(args) < 5 || args[1] != "agenova-system" || args[2] != "get" {
			t.Fatalf("unexpected query: %v", args)
		}
		return listing, nil
	}}
	templates, err := store.Templates()
	if err != nil || len(templates) != 1 || templates[0].Metadata.Name != "engineer" {
		t.Fatalf("templates=%#v, %v", templates, err)
	}
}

func TestKubernetesStoreRejectsUnmanagedActivePointer(t *testing.T) {
	store := KubernetesStore{Namespace: "agenova-system", Invoke: func(_ context.Context, _ []byte, args ...string) ([]byte, error) {
		if len(args) > 4 && args[4] == "agenova-active-policy" {
			return []byte(`{"metadata":{"resourceVersion":"1"},"data":{}}`), nil
		}
		return []byte(`{"metadata":{"labels":{"app.kubernetes.io/managed-by":"agenova"}},"data":{"policy.json":"{}"}}`), nil
	}}
	if _, err := store.ActivatePolicy(PolicyReference{ID: "any", Version: "1"}); err == nil {
		t.Fatal("unmanaged pointer was overwritten")
	}
}
