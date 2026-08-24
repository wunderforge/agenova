function Get-MarkdownSection {
  param(
    [Parameter(Mandatory=$true)][string]$Body,
    [Parameter(Mandatory=$true)][string]$Heading
  )

  $escapedHeading = [regex]::Escape($Heading)
  $match = [regex]::Match($Body, "(?ms)^##\s+$escapedHeading\s*\r?\n(.*?)(?=^##\s+|\z)")
  if (-not $match.Success) { throw "PR body is missing required section: $Heading" }
  return $match.Groups[1].Value.Trim()
}

function Remove-MarkdownComments {
  param([Parameter(Mandatory=$true)][string]$Value)
  return ([regex]::Replace($Value, "(?s)<!--.*?-->", "")).Trim()
}

function Assert-CompletedSection {
  param(
    [Parameter(Mandatory=$true)][string]$Body,
    [Parameter(Mandatory=$true)][string]$Heading
  )

  $content = Remove-MarkdownComments (Get-MarkdownSection -Body $Body -Heading $Heading)
  if ([string]::IsNullOrWhiteSpace($content)) { throw "PR section is empty: $Heading" }
  return $content
}

function Assert-NonBlankEvidenceInput {
  param(
    [Parameter(Mandatory=$true)][string]$Name,
    [AllowEmptyString()][string]$Value
  )

  if ([string]::IsNullOrWhiteSpace($Value)) { throw "evidence $Name must not be empty" }
}

function Assert-NonBlankEvidenceOutput {
  param([AllowEmptyString()][string]$Value)
  if ([string]::IsNullOrWhiteSpace($Value)) { throw "evidence command produced no output" }
}

function Test-PullRequestContract {
  param([AllowEmptyString()][string]$Body)

  if ([string]::IsNullOrWhiteSpace($Body)) { throw "PR body must not be empty" }
  if ($Body -notmatch "(?im)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+#\d+\b") {
    throw "PR body must close a linked ticket, for example: Closes #19"
  }

  foreach ($heading in @(
    "Linked ticket",
    "MVP-path outcome",
    "Changes",
    "Verification",
    "Backend neutrality",
    "Risks and blockers"
  )) {
    Assert-CompletedSection -Body $Body -Heading $heading | Out-Null
  }

  $verification = Get-MarkdownSection -Body $Body -Heading "Verification"
  if ($verification -notmatch '(?im)^\|[^\r\n]+\|\s*`[^`]+`\s*\|\s*(pass|fail|blocked)\s*\|') {
    throw "Verification must include at least one exact command or artifact and a pass, fail, or blocked result"
  }

  $neutrality = Get-MarkdownSection -Body $Body -Heading "Backend neutrality"
  if ($neutrality -notmatch "(?im)^\s*-\s*\[[xX]\]\s+") {
    throw "Backend neutrality must have one checked confirmation"
  }

  $risks = Remove-MarkdownComments (Get-MarkdownSection -Body $Body -Heading "Risks and blockers")
  foreach ($field in @("Risks", "Blockers")) {
    if ($risks -notmatch "(?im)^\s*-\s*${field}:\s*\S.+$") {
      throw "Risks and blockers must contain a non-empty ${field} value"
    }
  }
}

function Test-IssueFormContract {
  $path = Join-Path $Root ".github/ISSUE_TEMPLATE/ticket.yml"
  if (-not (Test-Path -LiteralPath $path)) { throw "missing issue form: .github/ISSUE_TEMPLATE/ticket.yml" }
  $raw = Get-Content -LiteralPath $path -Raw

  $configPath = Join-Path $Root ".github/ISSUE_TEMPLATE/config.yml"
  $config = Get-Content -LiteralPath $configPath -Raw
  if ($config -notmatch "(?m)^blank_issues_enabled:\s*false\s*$") {
    throw "blank GitHub issues must be disabled"
  }

  foreach ($id in @(
    "parent_epic",
    "mvp_path_outcome",
    "in_scope",
    "out_of_scope",
    "acceptance",
    "negative_case",
    "quality_gates",
    "evidence"
  )) {
    $escapedId = [regex]::Escape($id)
    $field = [regex]::Match($raw, "(?ms)^\s*- type:\s*(?:input|textarea)\s*$.*?^\s+id:\s*$escapedId\s*$.*?(?=^\s*- type:|\z)")
    if (-not $field.Success) { throw "issue form is missing required field: $id" }
    if ($field.Value -notmatch "(?m)^\s+required:\s*true\s*$") {
      throw "issue form field is not required: $id"
    }
  }
}

function Test-DeliveryContracts {
  Test-IssueFormContract

  $headings = @(
    "Linked ticket",
    "MVP-path outcome",
    "Changes",
    "Verification",
    "Backend neutrality",
    "Risks and blockers"
  )
  $template = Get-Content -LiteralPath (Join-Path $Root ".github/pull_request_template.md") -Raw
  foreach ($heading in $headings) {
    Get-MarkdownSection -Body $template -Heading $heading | Out-Null
  }

  $templateRejected = $false
  try {
    Test-PullRequestContract -Body $template
  }
  catch {
    $templateRejected = $true
  }
  if (-not $templateRejected) { throw "unfilled PR template was accepted" }

  $taskTemplate = Get-Content -LiteralPath (Join-Path $Root "tasks/task-template.md") -Raw
  foreach ($required in @(
    "Parent Epic:",
    "MVP-path outcome:",
    "## Scope",
    "## Acceptance Criteria",
    "## Negative Case",
    "## Quality Gates",
    "## Evidence Required"
  )) {
    if ($taskTemplate -notmatch [regex]::Escape($required)) {
      throw "local task template is missing: $required"
    }
  }

  $valid = Get-Content -LiteralPath (Join-Path $Root "harness/fixtures/contracts/pr-valid.md") -Raw
  Test-PullRequestContract -Body $valid

  $invalid = Get-Content -LiteralPath (Join-Path $Root "harness/fixtures/contracts/pr-invalid.md") -Raw
  $rejected = $false
  try {
    Test-PullRequestContract -Body $invalid
  }
  catch {
    $rejected = $true
  }
  if (-not $rejected) { throw "invalid PR contract fixture was accepted" }

  foreach ($case in @(
    @{ Name = "Ticket"; Value = " " },
    @{ Name = "Gate"; Value = "" },
    @{ Name = "Command"; Value = "`t" }
  )) {
    $rejected = $false
    try {
      Assert-NonBlankEvidenceInput -Name $case.Name -Value $case.Value
    }
    catch {
      $rejected = $true
    }
    if (-not $rejected) { throw "empty evidence $($case.Name) was accepted" }
  }

  $rejected = $false
  try {
    Assert-NonBlankEvidenceOutput -Value " "
  }
  catch {
    $rejected = $true
  }
  if (-not $rejected) { throw "empty evidence output was accepted" }

  Pass "issue, PR, and evidence delivery contracts are mechanically validated"
}
