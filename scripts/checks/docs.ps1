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
