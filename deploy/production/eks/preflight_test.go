// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package eks

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// All cloud commands are shadowed by in-process PowerShell functions. No real
// AWS/kubectl invocation, credential access, cluster or resource is involved.
func TestOperatorMigrationPreflight(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("PowerShell host unavailable; CI's PowerShell-enabled runner exercises this gate")
	}
	path, err := filepath.Abs("enable-work-egress.ps1")
	if err != nil {
		t.Fatal(err)
	}
	image := "931228356546.dkr.ecr.ap-southeast-2.amazonaws.com/agenova-demo/control-plane@sha256:" + strings.Repeat("a", 64)
	uid := "ec1a6e6a-bde5-468f-9e94-dba91134db6f"
	for _, tc := range []struct {
		name, mutate, want string
		execute, fail      bool
	}{
		{"dry-run", "", "Server dry-run passed", false, false},
		{"apply-once", "", "Existing policy selector updated", true, false},
		{"already-configured", "$fixture.policy.spec=$fixture.desired", "Already configured", true, false},
		{"legacy-additive-allow", "$fixture.policies.items += [pscustomobject]@{metadata=@{name='legacy'};spec=@{podSelector=@{};egress=@(@{})}}", "Potential additive worker egress allow", true, true},
		{"active-allocation", "$fixture.claims.items=@(@{metadata=@{name='active'}})", "Drain existing allocations", true, true},
		{"missing-baseline", "$fixture.policies.items=@()", "Exact namespace baseline deny", true, true},
		{"non-strict-CNI", "$fixture.cni.spec.template.spec.containers[0].env[0].value='standard'", "Strict ready CNI", true, true},
		{"wrong-image", "$fixture.cp.spec.template.spec.containers[0].image='wrong'", "Unexpected control-plane image", true, true},
		{"wrong-policy-UID", "$fixture.policy.metadata.uid='changed'", "Unexpected cluster policy identity", true, true},
		{"policy-drift", "$fixture.policy.spec.priority=11", "Policy drift", true, true},
		{"server-mismatch", "$fixture.desired.priority=11", "Unexpected server dry-run", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile("worker-egress-deny.yaml")
			if err != nil {
				t.Fatal(err)
			}
			var desired map[string]any
			if err := yaml.Unmarshal(data, &desired); err != nil {
				t.Fatal(err)
			}
			clone := func(v any) map[string]any {
				b, _ := json.Marshal(v)
				var out map[string]any
				if err := json.Unmarshal(b, &out); err != nil {
					t.Fatal(err)
				}
				return out
			}
			before := clone(desired["spec"])
			pods := before["subject"].(map[string]any)["pods"].(map[string]any)
			pods["podSelector"].(map[string]any)["matchExpressions"] = []any{map[string]any{"key": "agents.x-k8s.io/sandbox-name-hash", "operator": "Exists"}}
			fixture := map[string]any{
				"desired": desired["spec"],
				"policy":  map[string]any{"metadata": map[string]any{"name": "agenova-demo-worker-egress-deny", "uid": uid, "resourceVersion": "1"}, "spec": before},
				"claims":  map[string]any{"items": []any{}},
				"cp":      map[string]any{"spec": map[string]any{"replicas": 1, "template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"name": "control-plane", "image": image}}}}}, "status": map[string]any{"readyReplicas": 1}},
				"cni":     map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"name": "aws-node", "env": []any{map[string]any{"name": "NETWORK_POLICY_ENFORCING_MODE", "value": "strict"}}}}}}}, "status": map[string]any{"numberReady": 1, "desiredNumberScheduled": 1}},
				"policies": map[string]any{"items": []any{map[string]any{"metadata": map[string]any{"name": "demo-sandbox-deny-network"}, "spec": map[string]any{
					"podSelector": map[string]any{"matchExpressions": []any{map[string]any{"key": "agents.x-k8s.io/sandbox-name-hash", "operator": "Exists"}}}, "policyTypes": []string{"Ingress", "Egress"},
				}}}},
			}
			encoded, _ := json.Marshal(fixture)
			command := "$ErrorActionPreference='Stop'; $env:KUBECONFIG='synthetic-not-read'; $fixture='" + strings.ReplaceAll(string(encoded), "'", "''") + "' | ConvertFrom-Json; " + tc.mutate + "; " + `
function aws { $global:LASTEXITCODE=0; return '{"Account":"931228356546"}' }
function kubectl {
 $global:LASTEXITCODE=0
 if($args -contains 'patch') {
  $patch=$args[([array]::IndexOf($args,'--patch')+1)] | ConvertFrom-Json
  if($patch.Count -ne 4 -or $patch[0].op -ne 'test' -or $patch[1].path -ne '/metadata/resourceVersion' -or $patch[2].path -ne '/spec' -or $patch[3].path -ne '/spec/subject/pods/podSelector/matchExpressions/-'){throw 'Missing mutation guards'}
  if($args -notcontains '--dry-run=server'){Write-Host 'TASK_APPLIED_ONCE'}
  return (@{metadata=$fixture.policy.metadata;spec=$fixture.desired}|ConvertTo-Json -Depth 30)
 }
 if($args -contains 'sandboxclaims'){return ($fixture.claims|ConvertTo-Json -Depth 30)}
 if($args -contains 'deployment'){return ($fixture.cp|ConvertTo-Json -Depth 30)}
 if($args -contains 'daemonset'){return ($fixture.cni|ConvertTo-Json -Depth 30)}
 if($args -contains 'networkpolicies'){return ($fixture.policies|ConvertTo-Json -Depth 30)}
 if($args -contains 'clusternetworkpolicies'){return (@{items=@($fixture.policy)}|ConvertTo-Json -Depth 30)}
 throw 'Unexpected command; no native command fallback'
}
` + "& '" + strings.ReplaceAll(path, "'", "''") + "' -ExpectedPolicyUID '" + uid + "' -ExpectedControlPlaneImage '" + image + "'"
			if tc.execute {
				command += " -Execute"
			}
			out, err := exec.Command(pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", command).CombinedOutput()
			if (err != nil) != tc.fail || !strings.Contains(string(out), tc.want) {
				t.Fatalf("result=%v, wantFailure=%v, expected %q; output=%s", err, tc.fail, tc.want, out)
			}
			wantWrites := 0
			if tc.name == "apply-once" {
				wantWrites = 1
			}
			if strings.Count(string(out), "TASK_APPLIED_ONCE") != wantWrites {
				t.Fatalf("unexpected cloud-mutation intent: %s", out)
			}
		})
	}
}
