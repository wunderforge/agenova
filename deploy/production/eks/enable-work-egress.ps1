# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
# Operator-only migration for the existing shared demo, not a platform installer.
# Default is server dry-run. No resources, worker labels or RBAC are created.
param(
  [Parameter(Mandatory)][string]$ExpectedPolicyUID,
  [Parameter(Mandatory)][string]$ExpectedControlPlaneImage,
  [switch]$Execute
)
$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($env:KUBECONFIG)) { throw 'Set an explicit dedicated KUBECONFIG first; its contents are not read by this script' }
if ($ExpectedPolicyUID -notmatch '^[0-9a-f-]{36}$' -or $ExpectedControlPlaneImage -notmatch '^931228356546\.dkr\.ecr\.ap-southeast-2\.amazonaws\.com/agenova-demo/control-plane@sha256:[0-9a-f]{64}$') { throw 'Explicit reviewed policy UID and control-plane digest required' }
$taskContext = 'arn:aws:eks:ap-southeast-2:931228356546:cluster/agenova-demo'
function Read-TaskKube([string[]]$Arguments) {
  $taskRaw = & kubectl --context $taskContext --request-timeout=20s @Arguments
  if ($LASTEXITCODE -ne 0) { throw 'Bounded Kubernetes command failed' }
  return (($taskRaw -join "`n") | ConvertFrom-Json)
}
function Test-TaskEqual($Left, $Right) {
  return [System.Text.Json.Nodes.JsonNode]::DeepEquals(
    [System.Text.Json.Nodes.JsonNode]::Parse(($Left | ConvertTo-Json -Depth 30 -Compress)),
    [System.Text.Json.Nodes.JsonNode]::Parse(($Right | ConvertTo-Json -Depth 30 -Compress)))
}
$taskIdentity = aws sts get-caller-identity --query '{Account:Account}' --output json | ConvertFrom-Json
if ($LASTEXITCODE -ne 0 -or $taskIdentity.Account -ne '931228356546') { throw 'Wrong AWS account' }
$taskClaims = Read-TaskKube @('-n','agenova-system','get','sandboxclaims','-o','json')
if (@($taskClaims.items).Count -ne 0) { throw 'Drain existing allocations first' }
$taskCP = Read-TaskKube @('-n','agenova-system','get','deployment','agenova-member-control-plane','-o','json')
if ($taskCP.spec.replicas -ne 1 -or $taskCP.status.readyReplicas -ne 1 -or @($taskCP.spec.template.spec.containers | Where-Object {$_.name -eq 'control-plane' -and $_.image -eq $ExpectedControlPlaneImage}).Count -ne 1) { throw 'Unexpected control-plane image or readiness' }
$taskCNI = Read-TaskKube @('-n','kube-system','get','daemonset','aws-node','-o','json')
$taskCNIMain = @($taskCNI.spec.template.spec.containers | Where-Object name -eq 'aws-node')[0]
if (@($taskCNIMain.env | Where-Object {$_.name -eq 'NETWORK_POLICY_ENFORCING_MODE' -and $_.value -eq 'strict'}).Count -ne 1 -or $taskCNI.status.numberReady -ne $taskCNI.status.desiredNumberScheduled) { throw 'Strict ready CNI required' }
$taskSandboxSelector = '{"matchExpressions":[{"key":"agents.x-k8s.io/sandbox-name-hash","operator":"Exists"}]}' | ConvertFrom-Json
$taskTrustedSelector = '{"matchExpressions":[{"key":"agents.x-k8s.io/sandbox-name-hash","operator":"DoesNotExist"}]}' | ConvertFrom-Json
$taskPolicies = Read-TaskKube @('-n','agenova-system','get','networkpolicies','-o','json')
$taskBaseline = @($taskPolicies.items | Where-Object {$_.metadata.name -eq 'demo-sandbox-deny-network'})
if ($taskBaseline.Count -ne 1 -or -not (Test-TaskEqual $taskBaseline[0].spec.podSelector $taskSandboxSelector) -or @($taskBaseline[0].spec.egress | Where-Object {$null -ne $_}).Count -ne 0 -or @($taskBaseline[0].spec.ingress | Where-Object {$null -ne $_}).Count -ne 0 -or 'Egress' -notin $taskBaseline[0].spec.policyTypes -or 'Ingress' -notin $taskBaseline[0].spec.policyTypes) { throw 'Exact namespace baseline deny required' }
foreach ($taskPolicy in $taskPolicies.items) {
  if (@($taskPolicy.spec.egress | Where-Object {$null -ne $_}).Count -gt 0 -and -not (Test-TaskEqual $taskPolicy.spec.podSelector $taskTrustedSelector)) { throw 'Potential additive worker egress allow; review/drain first' }
}
$taskAllAdmin = Read-TaskKube @('get','clusternetworkpolicies','-o','json')
if (@($taskAllAdmin.items).Count -ne 1) { throw 'Additional cluster policy requires review' }
$taskPolicy = $taskAllAdmin.items[0]
if ($taskPolicy.metadata.name -ne 'agenova-demo-worker-egress-deny' -or $taskPolicy.metadata.uid -ne $ExpectedPolicyUID) { throw 'Unexpected cluster policy identity' }
$taskBefore = '{"tier":"Admin","priority":10,"subject":{"pods":{"namespaceSelector":{"matchLabels":{"kubernetes.io/metadata.name":"agenova-system"}},"podSelector":{"matchExpressions":[{"key":"agents.x-k8s.io/sandbox-name-hash","operator":"Exists"}]}}},"egress":[{"name":"no-direct-network-egress","action":"Deny","to":[{"networks":["0.0.0.0/0"]}]}]}' | ConvertFrom-Json
$taskDesired = ($taskBefore | ConvertTo-Json -Depth 25 | ConvertFrom-Json)
$taskDesired.subject.pods.podSelector.matchExpressions += [pscustomobject]@{key='agenova.io/network-incarnation';operator='DoesNotExist'}
if (Test-TaskEqual $taskPolicy.spec $taskDesired) { Write-Output 'Already configured; prerequisites verified. Packet acceptance still required.'; exit 0 }
if (-not (Test-TaskEqual $taskPolicy.spec $taskBefore)) { throw 'Policy drift; do not overwrite it' }
$taskPatch = @(
  @{op='test';path='/metadata/uid';value=$taskPolicy.metadata.uid},
  @{op='test';path='/metadata/resourceVersion';value=$taskPolicy.metadata.resourceVersion},
  @{op='test';path='/spec';value=$taskPolicy.spec},
  @{op='add';path='/spec/subject/pods/podSelector/matchExpressions/-';value=@{key='agenova.io/network-incarnation';operator='DoesNotExist'}}
) | ConvertTo-Json -Depth 25 -Compress
$taskPreview = Read-TaskKube @('patch','clusternetworkpolicy','agenova-demo-worker-egress-deny','--type=json','--patch',$taskPatch,'--dry-run=server','-o','json')
if (-not (Test-TaskEqual $taskPreview.spec $taskDesired)) { throw 'Unexpected server dry-run' }
if (-not $Execute) { Write-Output 'Server dry-run passed; no changes applied.'; exit 0 }
$taskClaimsAgain = Read-TaskKube @('-n','agenova-system','get','sandboxclaims','-o','json')
if (@($taskClaimsAgain.items).Count -ne 0) { throw 'New allocation appeared; abort' }
$taskChanged = Read-TaskKube @('patch','clusternetworkpolicy','agenova-demo-worker-egress-deny','--type=json','--patch',$taskPatch,'-o','json')
if ($taskChanged.metadata.uid -ne $ExpectedPolicyUID -or -not (Test-TaskEqual $taskChanged.spec $taskDesired)) { throw 'Unexpected mutation result; operator inspection required' }
Write-Output 'Existing policy selector updated. Zero resources/RBAC created. Packet acceptance still required.'
