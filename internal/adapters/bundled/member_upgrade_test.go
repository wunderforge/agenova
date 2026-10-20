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

	"github.com/wunderforge/agenova/internal/platformapply"
)

func memberFixture(t *testing.T) (platformapply.DeploymentRequest, map[string]any) {
	t.Helper()
	r := deploymentRequest()
	object := map[string]any{}
	raw := `{"metadata":{"uid":"11111111-1111-1111-1111-111111111111","resourceVersion":"4","generation":2},"spec":{"replicas":1,"strategy":{"type":"Recreate"},"template":{"metadata":{"labels":{"app":"member"}},"spec":{"serviceAccountName":"existing","containers":[{"name":"proxy","image":"proxy:unchanged"},{"name":"control-plane","image":"old","env":[{"name":"AGENOVA_AUTH_MODE","value":"member"},{"name":"AGENOVA_MEMBER_CONFIG","value":"/etc/member/config.json"}],"volumeMounts":[{"name":"secrets","mountPath":"/run/secrets"}]}],"volumes":[{"name":"secrets","secret":{"secretName":"retained"}}]}}},"status":{"observedGeneration":2,"replicas":1,"updatedReplicas":1,"availableReplicas":1}}`
	if err := json.Unmarshal([]byte(raw), &object); err != nil {
		t.Fatal(err)
	}
	binding, _, _, err := memberTemplateBinding(object)
	if err != nil {
		t.Fatal(err)
	}
	r.Config["mode"] = "member-upgrade"
	r.Config["deployment-name"] = "member"
	r.Config["deployment-uid"] = "11111111-1111-1111-1111-111111111111"
	r.Config["pod-template-sha256"] = binding
	r.Config["control-plane-image"] = "registry.example/control@sha256:" + strings.Repeat("a", 64)
	return r, object
}

func TestMemberUpgradePatchesOnlyImageAndIsIdempotent(t *testing.T) {
	r, object := memberFixture(t)
	before, _, _, _ := memberTemplateBinding(object)
	patches := 0
	runner := &fakeKubectl{}
	runner.run = func(args []string) (commandResult, error) {
		if contains(args, "can-i") {
			return commandResult{stdout: "yes"}, nil
		}
		if contains(args, "patch") {
			patches++
			var ops []map[string]any
			if err := json.Unmarshal([]byte(args[len(args)-1]), &ops); err != nil {
				t.Fatal(err)
			}
			if len(ops) != 4 || ops[0]["op"] != "test" || ops[0]["path"] != "/metadata/uid" || ops[1]["path"] != "/metadata/resourceVersion" || ops[2]["value"] != "old" || ops[3]["path"] != "/spec/template/spec/containers/1/image" {
				t.Fatalf("unsafe patch: %#v", ops)
			}
			spec := object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
			spec["containers"].([]any)[1].(map[string]any)["image"] = ops[3]["value"]
		}
		if contains(args, "configmap") {
			raw, _ := json.Marshal(r.Platform)
			record, _ := json.Marshal(map[string]any{"data": map[string]any{"effective-platform.json": string(raw)}})
			return commandResult{stdout: string(record)}, nil
		}
		raw, _ := json.Marshal(object)
		return commandResult{stdout: string(raw)}, nil
	}
	k := newKubernetesDeployment(runner)
	_, changes, _, err := k.Plan(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	r.TargetChanges = changes
	status, attempted, err := k.Apply(context.Background(), r)
	if err != nil || !attempted || status[0].State != "available" {
		t.Fatalf("apply %v %v %v", status, attempted, err)
	}
	after, _, _, _ := memberTemplateBinding(object)
	if before != after {
		t.Fatal("changed non-image member configuration")
	}
	r.TargetChanges = nil
	_, attempted, err = k.Apply(context.Background(), r)
	if err != nil || attempted || patches != 1 {
		t.Fatalf("repeat: %v %v patches=%d", attempted, err, patches)
	}
}

func TestMemberUpgradeRejectsUnsafeTargetsBeforeMutation(t *testing.T) {
	for _, test := range []string{"uid", "auth", "sidecar", "strategy", "rbac", "runtime", "plan", "conflict"} {
		t.Run(test, func(t *testing.T) {
			r, object := memberFixture(t)
			if test == "uid" {
				r.Config["deployment-uid"] = "other"
			}
			template := object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
			if test == "auth" {
				template["containers"].([]any)[1].(map[string]any)["env"] = []any{}
			}
			if test == "sidecar" {
				template["containers"].([]any)[0].(map[string]any)["image"] = "changed"
			}
			if test == "strategy" {
				object["spec"].(map[string]any)["strategy"] = map[string]any{"type": "RollingUpdate"}
			}
			if test == "plan" {
				r.TargetChanges = []platformapply.Change{}
			}
			patches := 0
			runner := &fakeKubectl{run: func(args []string) (commandResult, error) {
				if contains(args, "can-i") {
					if test == "rbac" {
						return commandResult{stdout: "no"}, errors.New("denied")
					}
					return commandResult{stdout: "yes"}, nil
				}
				if contains(args, "patch") {
					patches++
					return commandResult{}, errors.New("conflict")
				}
				if contains(args, "configmap") {
					p := *r.Platform
					if test == "runtime" {
						p.PlatformName = "foreign"
					}
					raw, _ := json.Marshal(p)
					record, _ := json.Marshal(map[string]any{"data": map[string]any{"effective-platform.json": string(raw)}})
					return commandResult{stdout: string(record)}, nil
				}
				raw, _ := json.Marshal(object)
				return commandResult{stdout: string(raw)}, nil
			}}
			_, _, err := newKubernetesDeployment(runner).Apply(context.Background(), r)
			if err == nil || (test != "conflict" && patches != 0) || (test == "conflict" && patches != 1) {
				t.Fatalf("unsafe apply err=%v patches=%d", err, patches)
			}
		})
	}
}

func TestMemberUpgradeConfigBindings(t *testing.T) {
	r, _ := memberFixture(t)
	got, err := canonicalizeKubernetesDeployment("deployment", r.Config)
	if err != nil || !reflect.DeepEqual(got, r.Config) {
		t.Fatalf("canonical %v %v", got, err)
	}
	for _, key := range []string{"deployment-name", "deployment-uid", "pod-template-sha256", "control-plane-image"} {
		copy := map[string]any{}
		for k, v := range r.Config {
			copy[k] = v
		}
		delete(copy, key)
		if _, err = canonicalizeKubernetesDeployment("deployment", copy); err == nil {
			t.Fatalf("missing %s accepted", key)
		}
	}
}
