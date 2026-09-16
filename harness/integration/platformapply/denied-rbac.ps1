# Reproduce a denied, zero-target-mutation Platform apply on the local reference kind cluster.
# Requires Go, kubectl and an already-ready deploy/reference/platform.kind.yaml target.
param([string]$Context = 'kind-agenova-k8s-lab')

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../../..')).Path
$namespace = 'agenova-system'
$role = 'agenova-platform-readonly-test'
$user = 'agenova-platform-readonly-test'
$realKubectl = (Get-Command kubectl -ErrorAction Stop).Source
$scratch = Join-Path $repo '.tmp/platform-denied-rbac'
$wrapper = Join-Path $scratch 'kubectl.exe'
$cli = Join-Path $scratch 'agenova.exe'
$probe = Join-Path $scratch 'platform.probe.yaml'
$oldPath = $env:PATH
$oldReal = $env:AGENOVA_TEST_REAL_KUBECTL
$oldUser = $env:AGENOVA_TEST_IMPERSONATE_USER
$createdRole = $false
$createdBinding = $false

Push-Location $repo
try {
    $existingRole = & $realKubectl --context $Context get clusterrole $role --ignore-not-found -o name
    if ($LASTEXITCODE -ne 0) { throw 'inspect test ClusterRole failed' }
    if ($existingRole) { throw "test ClusterRole $role already exists; refusing to replace it" }
    $existingBinding = & $realKubectl --context $Context get clusterrolebinding $role --ignore-not-found -o name
    if ($LASTEXITCODE -ne 0) { throw 'inspect test ClusterRoleBinding failed' }
    if ($existingBinding) { throw "test ClusterRoleBinding $role already exists; refusing to replace it" }

    New-Item -ItemType Directory -Path $scratch -Force | Out-Null
    go build -o $wrapper ./harness/integration/platformapply/impersonate-kubectl
    if ($LASTEXITCODE -ne 0) { throw 'build test kubectl wrapper failed' }
    go build -o $cli ./cmd/agenova
    if ($LASTEXITCODE -ne 0) { throw 'build Agenova CLI failed' }
    $source = [IO.File]::ReadAllText((Join-Path $repo 'deploy/reference/platform.kind.yaml'))
    $changed = $source.Replace('name: reference-kind', 'name: reference-kind-rbac-probe')
    if ($changed -eq $source) { throw 'reference Platform name was not found; cannot create changed-revision probe' }
    [IO.File]::WriteAllText($probe, $changed)

    & $realKubectl --context $Context create clusterrole $role --verb=get --resource=namespaces,configmaps,services,deployments.apps | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'create read-only test ClusterRole failed' }
    $createdRole = $true
    & $realKubectl --context $Context create clusterrolebinding $role --clusterrole=$role --user=$user | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'create read-only test ClusterRoleBinding failed' }
    $createdBinding = $true

    $names = @('configmap/agenova-platform', 'configmap/agenova-policy-reference-default-deny-v1', 'deployment/agenova-control-plane', 'service/agenova-control-plane')
    $before = @(& $realKubectl --context $Context --namespace $namespace get $names -o name)
    if ($LASTEXITCODE -ne 0) { throw 'read managed resources before probe failed' }
    $beforeRecord = & $realKubectl --context $Context --namespace $namespace get configmap agenova-platform -o json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0) { throw 'read Platform revision before probe failed' }
    $beforeRevision = $beforeRecord.metadata.annotations.'agenova.io/platform-revision'

    $env:AGENOVA_TEST_REAL_KUBECTL = $realKubectl
    $env:AGENOVA_TEST_IMPERSONATE_USER = $user
    $env:PATH = "$scratch$([IO.Path]::PathSeparator)$oldPath"
    & $cli platform apply -f $probe --state-dir (Join-Path $scratch 'state') --yes --json
    $applyExit = $LASTEXITCODE
    $env:PATH = $oldPath

    $after = @(& $realKubectl --context $Context --namespace $namespace get $names -o name)
    if ($LASTEXITCODE -ne 0) { throw 'read managed resources after probe failed' }
    $afterRecord = & $realKubectl --context $Context --namespace $namespace get configmap agenova-platform -o json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0) { throw 'read Platform revision after probe failed' }
    $afterRevision = $afterRecord.metadata.annotations.'agenova.io/platform-revision'
    Write-Output "apply_exit=$applyExit managed_before=$($before.Count) managed_after=$($after.Count)"
    Write-Output "revision_before=$beforeRevision revision_after=$afterRevision"
    if ($applyExit -eq 0 -or $before.Count -ne $after.Count -or $beforeRevision -ne $afterRevision) {
        throw 'denied apply was not safely rejected without target mutation'
    }
} finally {
    $env:PATH = $oldPath
    $env:AGENOVA_TEST_REAL_KUBECTL = $oldReal
    $env:AGENOVA_TEST_IMPERSONATE_USER = $oldUser
    if ($createdBinding) { & $realKubectl --context $Context delete clusterrolebinding $role --ignore-not-found | Out-Null }
    if ($createdRole) { & $realKubectl --context $Context delete clusterrole $role --ignore-not-found | Out-Null }
    Pop-Location
}
