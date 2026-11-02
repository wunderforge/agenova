// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package eks

import (
	"os"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAdminDenyDelegatesOnlyStampedSandboxPods(t *testing.T) {
	data, err := os.ReadFile("worker-egress-deny.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Spec       struct {
			Tier     string `yaml:"tier"`
			Priority int    `yaml:"priority"`
			Subject  struct {
				Pods struct {
					NamespaceSelector struct {
						MatchLabels map[string]string `yaml:"matchLabels"`
					} `yaml:"namespaceSelector"`
					PodSelector struct {
						MatchExpressions []struct {
							Key      string `yaml:"key"`
							Operator string `yaml:"operator"`
						} `yaml:"matchExpressions"`
					} `yaml:"podSelector"`
				} `yaml:"pods"`
			} `yaml:"subject"`
			Egress []struct {
				Name   string `yaml:"name"`
				Action string `yaml:"action"`
				To     []struct {
					Networks []string `yaml:"networks"`
				} `yaml:"to"`
			} `yaml:"egress"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &policy); err != nil {
		t.Fatal(err)
	}
	if policy.APIVersion != "networking.k8s.aws/v1alpha1" || policy.Kind != "ClusterNetworkPolicy" || policy.Spec.Tier != "Admin" || policy.Spec.Priority != 10 {
		t.Fatal("operator policy kind/tier/priority changed")
	}
	if !reflect.DeepEqual(policy.Spec.Subject.Pods.NamespaceSelector.MatchLabels, map[string]string{"kubernetes.io/metadata.name": "agenova-system"}) {
		t.Fatal("operator policy escaped its namespace")
	}
	selectors := policy.Spec.Subject.Pods.PodSelector.MatchExpressions
	if len(selectors) != 2 || selectors[0].Key != "agents.x-k8s.io/sandbox-name-hash" || selectors[0].Operator != "Exists" || selectors[1].Key != "agenova.io/network-incarnation" || selectors[1].Operator != "DoesNotExist" {
		t.Fatal("default deny must exclude only trusted stamped sandbox Pods")
	}
	rules := policy.Spec.Egress
	if len(rules) != 1 || rules[0].Name != "no-direct-network-egress" || rules[0].Action != "Deny" || len(rules[0].To) != 1 || !reflect.DeepEqual(rules[0].To[0].Networks, []string{"0.0.0.0/0"}) {
		t.Fatal("blanket denial changed or a static allow was introduced")
	}
	for _, tc := range []struct {
		name   string
		labels map[string]string
		denied bool
	}{
		{"startup", map[string]string{"agents.x-k8s.io/sandbox-name-hash": "new"}, true},
		{"offline", map[string]string{"agents.x-k8s.io/sandbox-name-hash": "offline", "agents.x-k8s.io/claim-uid": "claim"}, true},
		{"replacement-same-claim", map[string]string{"agents.x-k8s.io/sandbox-name-hash": "replacement", "agents.x-k8s.io/claim-uid": "claim"}, true},
		{"trusted-original-delegated-not-granted", map[string]string{"agents.x-k8s.io/sandbox-name-hash": "original", "agenova.io/network-incarnation": "trusted"}, false},
		{"trusted-service-unaffected", map[string]string{"app": "agenova-member-control-plane"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected := true
			for _, expr := range selectors {
				_, present := tc.labels[expr.Key]
				selected = selected && ((expr.Operator == "Exists" && present) || (expr.Operator == "DoesNotExist" && !present))
			}
			if selected != tc.denied {
				t.Fatalf("Admin deny selected=%v, want %v", selected, tc.denied)
			}
		})
	}
}
