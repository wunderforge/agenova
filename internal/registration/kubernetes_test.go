// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/wunderforge/agenova/internal/policy"
)

func TestKubernetesStoreReferenceRegistrationAndActivation(t *testing.T) {
	objects := map[string]map[string]any{}
	mutations := 0
	store := KubernetesStore{Context: "kind-test", Namespace: "agenova-system", Invoke: func(_ context.Context, input []byte, args ...string) ([]byte, error) {
		if len(args) < 5 || args[0] != "--context" || args[2] != "--namespace" || args[3] != "agenova-system" {
			t.Fatalf("unpinned target: %v", args)
		}
		command := args[4:]
		switch command[0] {
		case "get":
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

func TestKubernetesStoreRejectsUnmanagedActivePointer(t *testing.T) {
	store := KubernetesStore{Namespace: "agenova-system", Invoke: func(_ context.Context, _ []byte, args ...string) ([]byte, error) {
		if len(args) > 4 && args[4] == "agenova-active-policy" {
			return []byte(`{"metadata":{"resourceVersion":"1"},"data":{}}`), nil
		}
		return []byte(`{"metadata":{"labels":{"app.kubernetes.io/managed-by":"agenova"}},"data":{"policy.json":"{}"}}`), nil
	}}
	if err := store.ActivatePolicy(PolicyReference{ID: "any", Version: "1"}); err == nil {
		t.Fatal("unmanaged pointer was overwritten")
	}
}
