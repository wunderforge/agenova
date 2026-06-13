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
    "docs/product/roadmap.md",
    "docs/phases/phase-0-foundation-alpha/README.md",
    "docs/phases/phase-0-foundation-alpha/prd.md",
    "docs/phases/phase-0-foundation-alpha/spec.md",
    "docs/phases/phase-0-foundation-alpha/acceptance.md",
    "docs/phases/phase-0-foundation-alpha/progress.md",
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
if ($All -or $Unit) { Test-GoPackages }
if ($All -or $Manifests) { Test-Manifests }
if ($All -or $Names) { Test-Names }
if ($Scenario) { Test-Scenario $Scenario }

if ($All) {
  Test-Scenario "smoke-warmpool-claim"
  Test-Scenario "smoke-tool-gateway-secret-boundary"
}

