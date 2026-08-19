param(
  [switch]$All,
  [switch]$Docs,
  [switch]$Unit,
  [switch]$Integration,
  [switch]$Race,
  [string]$KubeContext = "kind-agenova-k8s-lab",
  [string]$Namespace = "default"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot

function Pass($Message) { Write-Host "[pass] $Message" }
function Fail($Message) { throw "[fail] $Message" }

function Test-RequiredDocs {
  $required = @(
    "LICENSE",
    "NOTICE",
    "THIRD_PARTY_NOTICES.md",
    "README.md",
    "AGENTS.md",
    "CONTRIBUTING.md",
    ".github/workflows/pr.yml",
    "docs/project-design.md",
    "docs/project-status.md",
    "docs/product/prd.md",
    "docs/product/architecture-contract.md",
    "docs/backends/agent-sandbox.md",
    "docs/harness/quality-gates.md",
    "docs/harness/gotchas.md",
    "docs/harness/playbooks.md",
    "docs/harness/learnings.md",
    "harness/README.md",
    "tasks/task-template.md"
  )

  foreach ($path in $required) {
    if (-not (Test-Path -LiteralPath (Join-Path $Root $path))) {
      Fail "missing required file: $path"
    }
  }

  $retired = @(
    "docs/phases",
    "docs/human-design-decisions",
    "docs/harness/phase-delivery.md",
    "docs/harness/evidence-gates.md",
    "docs/harness/claude-worker-playbook.md",
    "harness/phase-0-foundation-alpha",
    "harness/phase-1-agent-sandbox-adapter-spike"
  )

  foreach ($path in $retired) {
    if (Test-Path -LiteralPath (Join-Path $Root $path)) {
      Fail "retired path still exists: $path"
    }
  }

  Pass "current documentation set exists and retired paths are absent"
}

function Test-OpenSourceMetadata {
  $license = Get-Content -LiteralPath (Join-Path $Root "LICENSE") -Raw
  $notice = Get-Content -LiteralPath (Join-Path $Root "NOTICE") -Raw
  $thirdParty = Get-Content -LiteralPath (Join-Path $Root "THIRD_PARTY_NOTICES.md") -Raw
  $goMod = Get-Content -LiteralPath (Join-Path $Root "go.mod") -Raw

  if ($license -notmatch "Apache License\s+Version 2\.0, January 2004") {
    Fail "LICENSE is not the canonical Apache License 2.0 text"
  }
  if ($notice -notmatch "Copyright 2026 Dapeng Zhang and Agenova contributors") {
    Fail "NOTICE is missing the Agenova copyright attribution"
  }
  foreach ($required in @("Kubernetes SIGs Agent Sandbox", "v0.4.6", "Apache-2.0")) {
    if ($thirdParty -notmatch [regex]::Escape($required)) {
      Fail "third-party notice missing: $required"
    }
  }
  if ($goMod -notmatch "(?m)^module github\.com/wunderforge/agenova$") {
    Fail "go.mod must use the public github.com/wunderforge/agenova module path"
  }

  $goFiles = Get-ChildItem -LiteralPath $Root -Recurse -File -Filter *.go |
    Where-Object { $_.FullName -notmatch "[\\/](\.git|\.claude|\.tmp)[\\/]" }
  $missingHeaders = @()
  $legacyImports = @()
  foreach ($file in $goFiles) {
    $raw = Get-Content -LiteralPath $file.FullName -Raw
    $relative = $file.FullName.Substring($Root.Length + 1)
    if ($raw -notmatch "SPDX-License-Identifier: Apache-2\.0") {
      $missingHeaders += $relative
    }
    if ($raw -match "github\.com/donozhang1992/agenova") {
      $legacyImports += $relative
    }
  }

  if ($missingHeaders) { Fail "Go files missing Apache-2.0 SPDX headers: $($missingHeaders -join ', ')" }
  if ($legacyImports) { Fail "legacy Go module imports remain: $($legacyImports -join ', ')" }
  Pass "open-source license, attribution, source headers, and module path are consistent"
}

function Test-ArchitectureText {
  $readme = Get-Content -LiteralPath (Join-Path $Root "README.md") -Raw
  $design = Get-Content -LiteralPath (Join-Path $Root "docs/project-design.md") -Raw
  $contract = Get-Content -LiteralPath (Join-Path $Root "docs/product/architecture-contract.md") -Raw
  $prd = Get-Content -LiteralPath (Join-Path $Root "docs/product/prd.md") -Raw
  $status = Get-Content -LiteralPath (Join-Path $Root "docs/project-status.md") -Raw

  foreach ($required in @("backend-neutral governance runtime", "claim-scoped governance contract")) {
    if (($readme + $design + $contract) -notmatch [regex]::Escape($required)) {
      Fail "core positioning missing: $required"
    }
  }

  foreach ($required in @("one agent worker run", "not one tool call", "RuntimeBackend", "External system credentials remain behind")) {
    if ($contract -notmatch [regex]::Escape($required)) {
      Fail "architecture contract missing: $required"
    }
  }

  foreach ($required in @("ClaimRequest", "requested access", "effective claim authority", "system-managed")) {
    foreach ($document in @(
      @{ Name = "project design"; Raw = $design },
      @{ Name = "PRD"; Raw = $prd },
      @{ Name = "architecture contract"; Raw = $contract }
    )) {
      if ($document.Raw -notmatch [regex]::Escape($required)) {
        Fail "$($document.Name) missing request-resolution contract: $required"
      }
    }
  }

  if ($design -notmatch [regex]::Escape("agenova run -f")) {
    Fail "project design must show the file-based ClaimRequest CLI"
  }

  if ($contract -notmatch [regex]::Escape("Task input does not grant resource access")) {
    Fail "architecture contract must keep task data separate from resource authority"
  }

  foreach ($path in @("docs/project-design.md", "docs/product/prd.md")) {
    $raw = Get-Content -LiteralPath (Join-Path $Root $path) -Raw
    if ($raw -match "agenova run\s+\S+\s+--(repo|tools|model)" -or $raw -match "--(repo|tools|model)\b") {
      Fail "flag-based authority shorthand is not the canonical design: $path"
    }
  }

  if ($design -notmatch "```mermaid") { Fail "project design must contain Mermaid diagrams" }
  if ($prd -notmatch "## Acceptance Scenario") { Fail "PRD must contain an acceptance scenario" }
  if ($status -notmatch "## Next Delivery Slice") { Fail "project status must state the next delivery slice" }

  foreach ($stale in @("preparing Phase 1", "Current Phase", "Phase 1-3 Delivery")) {
    foreach ($path in @("README.md", "AGENTS.md", "docs/project-design.md", "docs/project-status.md", "docs/product/prd.md")) {
      $raw = Get-Content -LiteralPath (Join-Path $Root $path) -Raw
      if ($raw -match [regex]::Escape($stale)) { Fail "stale phase wording in ${path}: $stale" }
    }
  }

  Pass "current product and architecture language is present"
}

function Test-MarkdownLinks {
  $files = Get-ChildItem -LiteralPath $Root -Recurse -File -Filter *.md |
    Where-Object {
      $_.FullName -notmatch "[\\/](\.git|\.claude|\.tmp)[\\/]" -and
      $_.FullName -notmatch "[\\/]docs[\\/]evidence[\\/].*[\\/]summary\.md$"
    }

  $broken = @()
  foreach ($file in $files) {
    $raw = Get-Content -LiteralPath $file.FullName -Raw
    $matches = [regex]::Matches($raw, "\[[^\]]+\]\(([^)]+)\)")
    foreach ($match in $matches) {
      $target = $match.Groups[1].Value.Trim().Trim('<', '>')
      if ($target -match "^(https?://|mailto:|#)") { continue }
      $pathPart = ($target -split "#", 2)[0]
      if ([string]::IsNullOrWhiteSpace($pathPart)) { continue }
      $resolved = Join-Path $file.DirectoryName $pathPart
      if (-not (Test-Path -LiteralPath $resolved)) {
        $broken += "$($file.FullName.Substring($Root.Length + 1)) -> $target"
      }
    }
  }

  if ($broken) { Fail "broken local Markdown links: $($broken -join '; ')" }
  Pass "local Markdown links resolve"
}

function Test-RuntimeBoundary {
  $patterns = @(
    "sigs.k8s.io/agent-sandbox",
    "github.com/kubernetes-sigs/agent-sandbox",
    "agentsandboxv1",
    "AgentSandboxClaim",
    "agents.x-k8s.io",
    "extensions.agents.x-k8s.io"
  )

  $allowedAdapterPath = Join-Path $Root "internal/runtime/agentsandbox"
  $bad = @()
  foreach ($rootName in @("api", "cmd", "internal")) {
    $sourceRoot = Join-Path $Root $rootName
    if (-not (Test-Path -LiteralPath $sourceRoot)) { continue }
    foreach ($file in Get-ChildItem -LiteralPath $sourceRoot -Recurse -File -Filter *.go) {
      if ($file.FullName.StartsWith($allowedAdapterPath, [System.StringComparison]::OrdinalIgnoreCase)) { continue }
      $raw = Get-Content -LiteralPath $file.FullName -Raw
      foreach ($pattern in $patterns) {
        if ($raw -match [regex]::Escape($pattern)) {
          $bad += "$($file.FullName.Substring($Root.Length + 1)): $pattern"
        }
      }
    }
  }

  if ($bad) { Fail "backend-specific shape leaked outside adapter: $($bad -join '; ')" }
  Pass "backend-specific Agent Sandbox shape stays inside its adapter"
}

function Initialize-GoCache {
  if (-not $env:GOCACHE) {
    $cache = Join-Path $Root ".tmp/gocache"
    New-Item -ItemType Directory -Force -Path $cache | Out-Null
    $env:GOCACHE = $cache
  }
}

function Test-Go {
  if (-not (Get-Command go -ErrorAction SilentlyContinue)) { Fail "go executable not found" }
  Initialize-GoCache

  Push-Location $Root
  try {
    $goFiles = Get-ChildItem -LiteralPath $Root -Recurse -File -Filter *.go |
      Where-Object { $_.FullName -notmatch "[\\/](\.git|\.claude|\.tmp)[\\/]" } |
      ForEach-Object { $_.FullName }
    if ($goFiles) {
      $unformatted = gofmt -l $goFiles
      if ($LASTEXITCODE -ne 0) { Fail "gofmt check failed" }
      if ($unformatted) { Fail "gofmt required: $($unformatted -join ', ')" }
    }
    Pass "current Go files are formatted"

    go mod tidy
    if ($LASTEXITCODE -ne 0) { Fail "go mod tidy failed" }
    git diff --exit-code -- go.mod go.sum
    if ($LASTEXITCODE -ne 0) { Fail "go.mod or go.sum needs 'go mod tidy'" }
    Pass "Go module metadata is tidy"

    go vet ./...
    if ($LASTEXITCODE -ne 0) { Fail "go vet ./... failed" }
    Pass "go vet ./..."

    go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { Fail "go test ./... failed" }
    Pass "go test ./..."

    if ($Race) {
      go test -race -count=1 ./...
      if ($LASTEXITCODE -ne 0) { Fail "go test -race ./... failed" }
      Pass "go test -race ./..."
    }

    go test -run '^$' -tags integration ./harness/integration/agentsandbox/
    if ($LASTEXITCODE -ne 0) { Fail "Agent Sandbox integration package does not compile" }
    Pass "Agent Sandbox integration package compiles"
  }
  finally {
    Pop-Location
  }
}

function Test-AgentSandboxIntegration {
  if (-not (Get-Command go -ErrorAction SilentlyContinue)) { Fail "go executable not found" }
  if (-not (Get-Command kubectl -ErrorAction SilentlyContinue)) { Fail "kubectl executable not found" }
  Initialize-GoCache

  Push-Location $Root
  try {
    go test -v -tags integration -timeout 5m ./harness/integration/agentsandbox/ -args -kube-context $KubeContext -namespace $Namespace
    if ($LASTEXITCODE -ne 0) { Fail "Agent Sandbox integration gate failed" }
    Pass "Agent Sandbox integration gate"
  }
  finally {
    Pop-Location
  }
}

if (-not ($All -or $Docs -or $Unit -or $Integration)) { $All = $true }

if ($All -or $Docs) {
  Test-RequiredDocs
  Test-OpenSourceMetadata
  Test-ArchitectureText
  Test-MarkdownLinks
  Test-RuntimeBoundary
}

if ($All -or $Unit) { Test-Go }
if ($Integration) { Test-AgentSandboxIntegration }
