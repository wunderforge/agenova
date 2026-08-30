function Test-RequiredDocs {
  $required = @(
    "LICENSE",
    "NOTICE",
    "THIRD_PARTY_NOTICES.md",
    "README.md",
    "AGENTS.md",
    "CONTRIBUTING.md",
    ".github/workflows/ci.yml",
    ".githooks/pre-commit",
    ".github/ISSUE_TEMPLATE/ticket.yml",
    ".github/ISSUE_TEMPLATE/config.yml",
    ".github/pull_request_template.md",
    "docs/project-design.md",
    "docs/project-status.md",
    "docs/development/AIDLC.md",
    "docs/product/prd.md",
    "docs/product/architecture-contract.md",
    "docs/backends/agent-sandbox.md",
    "docs/harness/quality-gates.md",
    "docs/harness/gotchas.md",
    "docs/harness/playbooks.md",
    "docs/harness/learnings.md",
    "docs/harness/templates/task.md",
    "docs/harness/templates/spec.md",
    "docs/harness/templates/design.md",
    "harness/README.md",
    "work/README.md",
    "scripts/install-hooks.ps1",
    "scripts/checks/repository.ps1",
    "scripts/checks/docs.ps1",
    "scripts/checks/architecture.ps1",
    "scripts/checks/contracts.ps1",
    "scripts/checks/go.ps1",
    "scripts/checks/backend.ps1",
    "scripts/new-task.ps1",
    "scripts/check-pr-body.ps1"
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
    "docs/development/workflow.md",
    "specs/README.md",
    "tasks/task-template.md",
    "harness/phase-0-foundation-alpha",
    "harness/phase-1-agent-sandbox-adapter-spike"
  )

  foreach ($path in $retired) {
    if (Test-Path -LiteralPath (Join-Path $Root $path)) {
      Fail "retired path still exists: $path"
    }
  }

  Pass "current documentation and harness files exist; retired paths are absent"
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
  if ($goMod -notmatch "(?m)^module github\.com/wunderforge/agenova\r?$") {
    Fail "go.mod must use the public github.com/wunderforge/agenova module path"
  }

  $goFiles = Get-ChildItem -LiteralPath $Root -Recurse -File -Filter *.go |
    Where-Object { $_.FullName -notmatch "[\\/](\.git|\.claude|\.tmp|\.worktrees)[\\/]" }
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
