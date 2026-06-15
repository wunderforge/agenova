param(
  [switch]$All,
  [switch]$Docs,
  [switch]$Unit,
  [switch]$Manifests,
  [switch]$Names,
  [switch]$Phase2Evidence,
  [switch]$Phase3Evidence,
  [string]$Scenario
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot

function Pass($Message) { Write-Host "[pass] $Message" }
function Fail($Message) { throw "[fail] $Message" }

function Test-RequiredDocs {
  $required = @(
    "AGENTS.md",
    "CLAUDE.md",
    "README.md",
    "docs/product/purpose.md",
    "docs/product/architecture-contract.md",
    "docs/product/agent-sandbox-pivot.md",
    "docs/product/roadmap.md",
    "docs/phases/phase-0-foundation-alpha/README.md",
    "docs/phases/phase-0-foundation-alpha/prd.md",
    "docs/phases/phase-0-foundation-alpha/spec.md",
    "docs/phases/phase-0-foundation-alpha/acceptance.md",
    "docs/phases/phase-0-foundation-alpha/progress.md",
    "docs/phases/phase-1-agent-sandbox-adapter-spike/README.md",
    "docs/phases/phase-1-agent-sandbox-adapter-spike/prd.md",
    "docs/phases/phase-1-agent-sandbox-adapter-spike/spec.md",
    "docs/phases/phase-1-agent-sandbox-adapter-spike/acceptance.md",
    "docs/phases/phase-1-agent-sandbox-adapter-spike/progress.md",
    "docs/phases/phase-1-agent-sandbox-adapter-spike/backend-capability-matrix.md",
    "docs/harness/gotchas.md",
    "docs/harness/playbooks.md",
    "docs/harness/learnings.md",
    "docs/harness/phase-delivery.md",
    "docs/harness/evidence-gates.md",
    "docs/harness/claude-worker-playbook.md",
    "tasks/task-template.md"
  )

  foreach ($path in $required) {
    $full = Join-Path $Root $path
    if (-not (Test-Path -LiteralPath $full)) { Fail "missing required doc: $path" }
  }

  Pass "required docs exist"
}

function Test-Phase1Spike {
  $matrix = Join-Path $Root "docs/phases/phase-1-agent-sandbox-adapter-spike/backend-capability-matrix.md"
  $raw = Get-Content -LiteralPath $matrix -Raw

  foreach ($status in @("Supported", "Needs verification", "Not supported", "Agenova-owned")) {
    if ($raw -notmatch [regex]::Escape($status)) { Fail "Phase 1 matrix missing status: $status" }
  }

  foreach ($required in @("Claim as one worker-run lease", "External credential isolation behind gateways", "Control Plane / Runtime Plane separation")) {
    if ($raw -notmatch [regex]::Escape($required)) { Fail "Phase 1 matrix missing capability: $required" }
  }

  $pivot = Get-Content -LiteralPath (Join-Path $Root "docs/product/agent-sandbox-pivot.md") -Raw
  if ($pivot -notmatch "Application-facing Agenova APIs must not depend on upstream Agent Sandbox CRD shape") {
    Fail "pivot doc must preserve adapter boundary"
  }
  if ($pivot -notmatch "alternative backend adapter") {
    Fail "pivot doc must name the alternative backend path"
  }

  $contract = Get-Content -LiteralPath (Join-Path $Root "docs/product/architecture-contract.md") -Raw
  if ($contract -notmatch "RuntimeBackend") {
    Fail "architecture contract must name the RuntimeBackend boundary"
  }
  if ($contract -notmatch "Application-facing Agenova APIs must not change when the selected backend changes") {
    Fail "architecture contract must preserve backend-neutral application APIs"
  }

  $spec = Get-Content -LiteralPath (Join-Path $Root "docs/phases/phase-1-agent-sandbox-adapter-spike/spec.md") -Raw
  if ($spec -notmatch "type RuntimeBackend interface") {
    Fail "Phase 1 spec must include the RuntimeBackend interface sketch"
  }
  if ($spec -notmatch "Contract tests must run against the in-memory reference backend") {
    Fail "Phase 1 spec must require backend-neutral contract tests"
  }
  if ($raw -notmatch "Required for any backend") {
    Fail "Phase 1 matrix missing Required for any backend column"
  }

  $scenarioDir = Join-Path $Root "harness/phase-1-agent-sandbox-adapter-spike/scenarios/smoke-backend-capability-matrix"
  foreach ($fileName in @("README.md", "given.yaml")) {
    $path = Join-Path $scenarioDir $fileName
    if (-not (Test-Path -LiteralPath $path)) { Fail "Phase 1 scenario missing ${fileName}" }
  }

  $scenarioRaw = Get-Content -LiteralPath (Join-Path $scenarioDir "given.yaml") -Raw
  if ($scenarioRaw -notmatch "BackendCapabilityMatrixCheck") { Fail "Phase 1 scenario should check the capability matrix" }

  $neutralityDir = Join-Path $Root "harness/phase-1-agent-sandbox-adapter-spike/scenarios/smoke-backend-neutrality"
  foreach ($fileName in @("README.md", "given.yaml")) {
    $path = Join-Path $neutralityDir $fileName
    if (-not (Test-Path -LiteralPath $path)) { Fail "Phase 1 backend-neutrality scenario missing ${fileName}" }
  }
  $neutralityRaw = Get-Content -LiteralPath (Join-Path $neutralityDir "given.yaml") -Raw
  if ($neutralityRaw -notmatch "BackendNeutralityCheck") { Fail "Phase 1 scenario should check backend neutrality" }
  if ($neutralityRaw -notmatch "application-facing-api-neutrality") { Fail "Phase 1 neutrality scenario must protect app-facing API neutrality" }

  Pass "Phase 1 adapter spike docs pass static checks"
}

function Test-Phase1ContractArtifacts {
  $backendFile = Join-Path $Root "internal/runtime/backend.go"
  if (-not (Test-Path -LiteralPath $backendFile)) { Fail "missing RuntimeBackend definition: internal/runtime/backend.go" }
  $backendRaw = Get-Content -LiteralPath $backendFile -Raw
  if ($backendRaw -notmatch "type RuntimeBackend interface") { Fail "internal/runtime/backend.go must define RuntimeBackend interface" }

  $contractFile = Join-Path $Root "internal/runtime/contracttest/run.go"
  if (-not (Test-Path -LiteralPath $contractFile)) { Fail "missing contract test suite: internal/runtime/contracttest/run.go" }
  $contractRaw = Get-Content -LiteralPath $contractFile -Raw
  if ($contractRaw -notmatch "func Run\(") { Fail "contracttest/run.go must export a Run function" }
  if ($contractRaw -notmatch "runtime\.RuntimeBackend") { Fail "contracttest/run.go must reference RuntimeBackend" }

  $docFile = Join-Path $Root "internal/operator/doc.go"
  if (-not (Test-Path -LiteralPath $docFile)) { Fail "missing operator doc.go with compile-time assertion" }
  $docRaw = Get-Content -LiteralPath $docFile -Raw
  if ($docRaw -notmatch "runtime\.RuntimeBackend") { Fail "internal/operator/doc.go must assert Runtime satisfies RuntimeBackend" }

  $testFile = Join-Path $Root "internal/operator/runtime_test.go"
  if (-not (Test-Path -LiteralPath $testFile)) { Fail "missing operator runtime_test.go" }
  $testRaw = Get-Content -LiteralPath $testFile -Raw
  if ($testRaw -notmatch "contracttest\.Run") { Fail "runtime_test.go must wire contracttest.Run against the in-memory backend" }

  $matrix = Join-Path $Root "docs/phases/phase-1-agent-sandbox-adapter-spike/backend-capability-matrix.md"
  $matrixRaw = Get-Content -LiteralPath $matrix -Raw
  if ($matrixRaw -notmatch "Backend-neutral contract tests") { Fail "backend capability matrix must include Backend-neutral contract tests row" }

  Pass "Phase 1 contract code artifacts present and correctly wired"
}

function Test-DeliveryHarness {
  $delivery = Get-Content -LiteralPath (Join-Path $Root "docs/harness/phase-delivery.md") -Raw
  foreach ($required in @("codex/phase3-delivery", "Phase 2: Deployable Runtime", "Phase 3: Governance Runtime", "Do not build a competing Kubernetes sandbox controller")) {
    if ($delivery -notmatch [regex]::Escape($required)) { Fail "delivery harness missing: $required" }
  }

  $gates = Get-Content -LiteralPath (Join-Path $Root "docs/harness/evidence-gates.md") -Raw
  foreach ($required in @("Evidence gates are mandatory", "Contract", "Deploy", "Governance", "docs/evidence/<phase>/<gate>/")) {
    if ($gates -notmatch [regex]::Escape($required)) { Fail "evidence gates missing: $required" }
  }

  $worker = Get-Content -LiteralPath (Join-Path $Root "docs/harness/claude-worker-playbook.md") -Raw
  foreach ($required in @("Worker Packet", "Preserve RuntimeBackend boundary", "Do not merge to main", "Evidence commands and results")) {
    if ($worker -notmatch [regex]::Escape($required)) { Fail "Claude worker playbook missing: $required" }
  }

  $evidenceScript = Join-Path $Root "scripts/evidence.ps1"
  if (-not (Test-Path -LiteralPath $evidenceScript)) { Fail "missing scripts/evidence.ps1" }

  Pass "Phase 1-3 delivery harness passes static checks"
}

function Test-GoPackages {
  if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "[warn] go not found; skipped go test ./..."
    return
  }

  Push-Location $Root
  try {
    if (-not $env:GOCACHE) { $env:GOCACHE = Join-Path $Root ".gocache" }
    go test ./...
    if ($LASTEXITCODE -ne 0) { Fail "go test ./... failed" }
    Pass "go test ./..."
  }
  finally { Pop-Location }
}

function Test-RuntimeBoundary {
  $patterns = @(
    "sigs.k8s.io/agent-sandbox",
    "github.com/kubernetes-sigs/agent-sandbox",
    "agentsandboxv1",
    "AgentSandboxClaim"
  )

  $sourceRoots = @("api", "cmd", "internal")
  $allowedAdapterPath = Join-Path $Root "internal/runtime/agentsandbox"
  $bad = @()
  foreach ($rootName in $sourceRoots) {
    $sourceRoot = Join-Path $Root $rootName
    if (-not (Test-Path -LiteralPath $sourceRoot)) { continue }
    $files = Get-ChildItem -LiteralPath $sourceRoot -Recurse -File -Filter *.go
    foreach ($file in $files) {
      if ($file.FullName.StartsWith($allowedAdapterPath, [System.StringComparison]::OrdinalIgnoreCase)) {
        continue
      }
      $raw = Get-Content -LiteralPath $file.FullName -Raw
      foreach ($pattern in $patterns) {
        if ($raw -match [regex]::Escape($pattern)) {
          $bad += "$($file.FullName): $pattern"
        }
      }
    }
  }

  if ($bad) {
    Fail "application/runtime source leaks upstream Agent Sandbox shape: $($bad -join '; ')"
  }

  Pass "runtime boundary has no known upstream Agent Sandbox type leaks"
}

function Test-EvidenceGate($Phase, $Gate) {
  $gateDir = Join-Path $Root "docs/evidence/$Phase/$Gate"
  if (-not (Test-Path -LiteralPath $gateDir)) { Fail "missing evidence gate: $Phase/$Gate" }

  $summaries = Get-ChildItem -LiteralPath $gateDir -Recurse -File -Filter summary.md
  if ($summaries.Count -lt 1) { Fail "evidence gate has no summary.md: $Phase/$Gate" }

  $passed = $false
  foreach ($summary in $summaries) {
    $raw = Get-Content -LiteralPath $summary.FullName -Raw
    if ($raw -match "Result:\s*pass") { $passed = $true }
  }
  if (-not $passed) { Fail "evidence gate has no passing summary: $Phase/$Gate" }
}

function Test-Phase2Evidence {
  foreach ($gate in @("cluster-bootstrap", "upstream-agent-sandbox-install-or-blocker", "claim-lifecycle-e2e", "kubectl-runtime-state", "backend-neutral-api")) {
    Test-EvidenceGate "phase-2" $gate
  }

  Pass "Phase 2 deploy evidence gates pass"
}

function Test-Phase3Evidence {
  foreach ($gate in @("tool-gateway", "model-gateway", "authorization-negative", "claim-lineage", "facts-query", "multi-agent-reference", "multi-agent-kubernetes-or-blocker")) {
    Test-EvidenceGate "phase-3" $gate
  }

  Pass "Phase 3 governance evidence gates pass"
}

function Test-Manifests {
  $scenarioRoot = Join-Path $Root "harness/phase-0-foundation-alpha/scenarios"
  $files = Get-ChildItem -Path $scenarioRoot -Recurse -Filter given.yaml
  if ($files.Count -lt 2) { Fail "expected at least two smoke scenario manifests" }

  foreach ($file in $files) {
    $raw = Get-Content -LiteralPath $file.FullName -Raw
    if ($raw -notmatch "apiVersion:") { Fail "manifest missing apiVersion: $($file.FullName)" }
    if ($raw -notmatch "kind:") { Fail "manifest missing kind: $($file.FullName)" }
  }

  $claimScenario = Join-Path $scenarioRoot "smoke-warmpool-claim/given.yaml"
  $claimRaw = Get-Content -LiteralPath $claimScenario -Raw
  if ($claimRaw -match "(?m)^\s*tool:\s*") { Fail "claim fixture should not model a SandboxClaim as one tool call" }
  if ($claimRaw -notmatch "SandboxClaim") { Fail "warm-pool scenario should include a SandboxClaim" }

  $secretScenario = Join-Path $scenarioRoot "smoke-tool-gateway-secret-boundary/given.yaml"
  $secretRaw = Get-Content -LiteralPath $secretScenario -Raw
  if ($secretRaw -match "AGENOVA_UPSTREAM_TOKEN") { Fail "sandbox manifest appears to expose upstream token env var" }

  Pass "smoke manifests pass static checks"
}

function Test-Names {
  Push-Location $Root
  try {
    $tracked = git -c safe.directory=$Root ls-files
    if ($LASTEXITCODE -ne 0) { Fail "git ls-files failed" }

    $oldPathPatterns = @(("zh" + "-CN"), ("agent" + "os"), ("agent" + "-os"))
    $oldPathRegex = ($oldPathPatterns | ForEach-Object { [regex]::Escape($_) }) -join "|"
    $badTracked = $tracked | Where-Object { $_ -match $oldPathRegex }
    if ($badTracked) { Fail "tracked file uses old naming: $($badTracked -join ', ')" }

    $oldContentPatterns = @(("zh" + "-CN"), ("Agent" + " OS"), ("agent" + "os"), ("AGENT" + "OS"), ("agent" + "-os"))
    $oldContentRegex = ($oldContentPatterns | ForEach-Object { [regex]::Escape($_) }) -join "|"
    $badText = @()
    foreach ($path in $tracked) {
      if ($path -eq "scripts/check.ps1") { continue }
      $full = Join-Path $Root $path
      if (-not (Test-Path -LiteralPath $full)) { continue }
      $raw = Get-Content -LiteralPath $full -Raw -ErrorAction SilentlyContinue
      if ($null -ne $raw -and $raw -match $oldContentRegex) { $badText += $path }
    }
    if ($badText) { Fail "tracked content uses old naming: $($badText -join ', ')" }
  }
  finally { Pop-Location }

  Pass "tracked names use Agenova conventions"
}

function Test-Scenario($Name) {
  if ([string]::IsNullOrWhiteSpace($Name)) { Fail "Scenario name is required" }

  $scenarioDir = Join-Path $Root "harness/phase-0-foundation-alpha/scenarios/$Name"
  if (-not (Test-Path -LiteralPath $scenarioDir)) { Fail "unknown scenario: $Name" }

  foreach ($fileName in @("README.md", "given.yaml")) {
    $path = Join-Path $scenarioDir $fileName
    if (-not (Test-Path -LiteralPath $path)) { Fail "scenario missing ${fileName}: $Name" }
  }

  Pass "scenario scaffold exists: $Name"
}

if (-not ($All -or $Docs -or $Unit -or $Manifests -or $Names -or $Phase2Evidence -or $Phase3Evidence -or $Scenario)) { $All = $true }

if ($All -or $Docs) { Test-RequiredDocs }
if ($All -or $Docs) { Test-Phase1Spike }
if ($All -or $Docs) { Test-Phase1ContractArtifacts }
if ($All -or $Docs) { Test-DeliveryHarness }
if ($All -or $Docs) { Test-RuntimeBoundary }
if ($All -or $Unit) { Test-GoPackages }
if ($All -or $Manifests) { Test-Manifests }
if ($All -or $Names) { Test-Names }
if ($Scenario) { Test-Scenario $Scenario }
if ($Phase2Evidence) { Test-Phase2Evidence }
if ($Phase3Evidence) { Test-Phase3Evidence }

if ($All) {
  Test-Scenario "smoke-warmpool-claim"
  Test-Scenario "smoke-tool-gateway-secret-boundary"
}

