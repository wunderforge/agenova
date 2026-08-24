function Test-ArchitectureText {
  $readme = Get-Content -LiteralPath (Join-Path $Root "README.md") -Raw
  $design = Get-Content -LiteralPath (Join-Path $Root "docs/project-design.md") -Raw
  $contract = Get-Content -LiteralPath (Join-Path $Root "docs/product/architecture-contract.md") -Raw
  $prd = Get-Content -LiteralPath (Join-Path $Root "docs/product/prd.md") -Raw
  $status = Get-Content -LiteralPath (Join-Path $Root "docs/project-status.md") -Raw
  $workflow = Get-Content -LiteralPath (Join-Path $Root "docs/development/workflow.md") -Raw
  $specs = Get-Content -LiteralPath (Join-Path $Root "specs/README.md") -Raw

  foreach ($required in @("backend-neutral governance runtime", "ClaimRequest", "RuntimeBackend")) {
    if ($readme -notmatch [regex]::Escape($required)) {
      Fail "README product entry is missing: $required"
    }
  }

  foreach ($required in @(
    "claim-scoped governance contract",
    "one agent worker run",
    "not one tool call",
    "RuntimeBackend",
    "Requested access is intent",
    "External system credentials remain behind"
  )) {
    if ($contract -notmatch [regex]::Escape($required)) {
      Fail "architecture contract missing: $required"
    }
  }

  foreach ($required in @("ClaimRequest", "trusted principal", "effective claim authority", "system-managed", "## Acceptance Scenario")) {
    if ($prd -notmatch [regex]::Escape($required)) {
      Fail "PRD MVP requirement is missing: $required"
    }
  }

  if ($design -notmatch [regex]::Escape("agenova run -f")) {
    Fail "project design must show the file-based ClaimRequest CLI"
  }
  if ($design -notmatch "```mermaid") { Fail "project design must contain Mermaid diagrams" }
  if ($status -notmatch "## Next Delivery Slice") { Fail "implementation snapshot must state the next delivery slice" }
  if ($status -notmatch [regex]::Escape("does not track ticket owners")) {
    Fail "implementation snapshot must defer mutable delivery state to GitHub"
  }

  foreach ($path in @("docs/project-design.md", "docs/product/prd.md")) {
    $raw = Get-Content -LiteralPath (Join-Path $Root $path) -Raw
    if ($raw -match "agenova run\s+\S+\s+--(repo|tools|model)" -or $raw -match "--(repo|tools|model)\b") {
      Fail "flag-based authority shorthand is not the canonical design: $path"
    }
  }

  foreach ($required in @(
    "## Source of Truth Map",
    "## Adaptive Planning Depth",
    "## Delivery Loop",
    "## Ten-Person Collaboration",
    "## Rules, Skills, and Code Style",
    "## Human Decision Gates",
    "docs/product/prd.md",
    "docs/product/architecture-contract.md",
    "GitHub Issue and Delivery Project"
  )) {
    if ($workflow -notmatch [regex]::Escape($required)) {
      Fail "development workflow is missing: $required"
    }
  }

  foreach ($required in @(
    "## Choose the Smallest Useful Level",
    "### Issue only",
    "### Feature spec",
    "### Feature spec plus technical design",
    'Do not create `tasks.md`',
    'Nested `AGENTS.md`'
  )) {
    if ($specs -notmatch [regex]::Escape($required)) {
      Fail "feature-spec convention is missing: $required"
    }
  }

  $claudePath = Join-Path $Root "CLAUDE.md"
  if (Test-Path -LiteralPath $claudePath) {
    $claude = Get-Content -LiteralPath $claudePath -Raw
    if ($claude -notmatch [regex]::Escape("AGENTS.md")) {
      Fail "CLAUDE.md must route to the canonical AGENTS.md"
    }
    if ($claude.Length -gt 1200) {
      Fail "CLAUDE.md must remain a thin tool adapter instead of duplicating repository rules"
    }
  }

  foreach ($stale in @("preparing Phase 1", "Current Phase", "Phase 1-3 Delivery")) {
    foreach ($path in @("README.md", "AGENTS.md", "docs/project-design.md", "docs/project-status.md", "docs/product/prd.md")) {
      $raw = Get-Content -LiteralPath (Join-Path $Root $path) -Raw
      if ($raw -match [regex]::Escape($stale)) { Fail "stale phase wording in ${path}: $stale" }
    }
  }

  Pass "product authorities, delivery routing, and adaptive spec structure are present"
}

function Test-MarkdownLinks {
  $files = Get-ChildItem -LiteralPath $Root -Recurse -File -Filter *.md |
    Where-Object {
      $_.FullName -notmatch "[\\/](\.git|\.claude|\.tmp|\.worktrees)[\\/]" -and
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
