param(
  [switch]$All,
  [switch]$Docs,
  [switch]$Unit,
  [switch]$Manifests,
  [switch]$Names,
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

if (-not ($All -or $Docs -or $Unit -or $Manifests -or $Names -or $Scenario)) { $All = $true }

if ($All -or $Docs) { Test-RequiredDocs }
if ($All -or $Docs) { Test-Phase1Spike }
if ($All -or $Unit) { Test-GoPackages }
if ($All -or $Manifests) { Test-Manifests }
if ($All -or $Names) { Test-Names }
if ($Scenario) { Test-Scenario $Scenario }

if ($All) {
  Test-Scenario "smoke-warmpool-claim"
  Test-Scenario "smoke-tool-gateway-secret-boundary"
}

