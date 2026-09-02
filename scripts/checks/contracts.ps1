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

function Assert-NonBlankLabeledBullet {
  param(
    [Parameter(Mandatory=$true)][string]$Body,
    [Parameter(Mandatory=$true)][string]$Field,
    [Parameter(Mandatory=$true)][string]$Section
  )

  $escapedField = [regex]::Escape($Field)
  $match = [regex]::Match($Body, "(?im)^\s*-\s*${escapedField}:[ \t]*(?<value>[^\r\n]*)$")
  $value = if ($match.Success) { $match.Groups["value"].Value.Trim().Trim([char]96) } else { "" }
  if ([string]::IsNullOrWhiteSpace($value)) {
    throw "${Section} must contain a non-empty ${Field} value"
  }
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
    "Review context",
    "MVP-path outcome",
    "Changes",
    "Scope and deferrals",
    "Verification",
    "Backend neutrality",
    "Risks and blockers"
  )) {
    Assert-CompletedSection -Body $Body -Heading $heading | Out-Null
  }

  $reviewContext = Remove-MarkdownComments (Get-MarkdownSection -Body $Body -Heading "Review context")
  Assert-NonBlankLabeledBullet -Body $reviewContext -Field "Task packet" -Section "Review context"

  $scope = Remove-MarkdownComments (Get-MarkdownSection -Body $Body -Heading "Scope and deferrals")
  foreach ($field in @("Contract or boundary changed", "Deferred / non-goal")) {
    Assert-NonBlankLabeledBullet -Body $scope -Field $field -Section "Scope and deferrals"
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
    Assert-NonBlankLabeledBullet -Body $risks -Field $field -Section "Risks and blockers"
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

  $specDepth = [regex]::Match($raw, "(?ms)^\s*- type:\s*dropdown\s*$.*?^\s+id:\s*spec_depth\s*$.*?(?=^\s*- type:|\z)")
  if (-not $specDepth.Success) { throw "issue form is missing required field: spec_depth" }
  if ($specDepth.Value -notmatch "(?m)^\s+required:\s*true\s*$") {
    throw "issue form field is not required: spec_depth"
  }
  foreach ($option in @("Task packet only", "Task packet and feature spec", "Task packet, feature spec, and technical design")) {
    if ($specDepth.Value -notmatch [regex]::Escape($option)) {
      throw "issue form planning depth is missing: $option"
    }
  }
}

function Test-TaskPacketContract {
  $taskTemplatePath = Join-Path $Root "docs/harness/templates/task.md"
  $specTemplatePath = Join-Path $Root "docs/harness/templates/spec.md"
  $designTemplatePath = Join-Path $Root "docs/harness/templates/design.md"

  $taskTemplate = Get-Content -LiteralPath $taskTemplatePath -Raw
  foreach ($required in @(
    "- Mission:",
    "- Target:",
    "- User value:",
    "- PRD outcome:",
    "## Context to Read",
    "## Scope",
    "## Acceptance Criteria",
    "## Negative Case",
    "## Execution Todo",
    "## Quality Gates",
    "## Evidence Required",
    "## Constraints",
    "## Decisions and Blockers"
  )) {
    if ($taskTemplate -notmatch [regex]::Escape($required)) {
      throw "task packet template is missing: $required"
    }
  }
  foreach ($projectField in @("- Owner:", "- Reviewer:", "- Priority:", "- Sequence:", "- Status:")) {
    if ($taskTemplate -match [regex]::Escape($projectField)) {
      throw "task packet must not mirror GitHub Project state: $projectField"
    }
  }

  $specTemplate = Get-Content -LiteralPath $specTemplatePath -Raw
  foreach ($required in @("## Intent", "## Requirements", "## Negative Cases", "## Compatibility")) {
    if ($specTemplate -notmatch [regex]::Escape($required)) {
      throw "feature spec template is missing: $required"
    }
  }

  $designTemplate = Get-Content -LiteralPath $designTemplatePath -Raw
  foreach ($required in @("## Decision", "## Ownership and Contract Boundaries", "## Alternatives Considered", "## Verification Strategy")) {
    if ($designTemplate -notmatch [regex]::Escape($required)) {
      throw "technical design template is missing: $required"
    }
  }

  $tempParent = Join-Path $Root ".tmp"
  $tempRoot = Join-Path $tempParent "task-scaffold-contract"
  $resolvedParent = [System.IO.Path]::GetFullPath($tempParent).TrimEnd([System.IO.Path]::DirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
  $resolvedTemp = [System.IO.Path]::GetFullPath($tempRoot)
  if (-not $resolvedTemp.StartsWith($resolvedParent, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "unsafe task scaffold test path: $resolvedTemp"
  }

  if (Test-Path -LiteralPath $tempRoot) {
    Remove-Item -LiteralPath $tempRoot -Recurse -Force
  }

  try {
    & (Join-Path $Root "scripts/new-task.ps1") -Issue 9999 -Slug "contract-check" -Title "Contract Check" -WithSpec -WithDesign -OutputRoot $tempRoot
    $packetPath = Join-Path $tempRoot "9999-contract-check"
    foreach ($fileName in @("task.md", "spec.md", "design.md")) {
      $generatedPath = Join-Path $packetPath $fileName
      if (-not (Test-Path -LiteralPath $generatedPath)) {
        throw "task scaffold did not generate: $fileName"
      }
      $generated = Get-Content -LiteralPath $generatedPath -Raw
      if ($generated -match "\{\{(?:ISSUE|ISSUE_URL|TITLE)\}\}") {
        throw "task scaffold left an unresolved template token in: $fileName"
      }
    }

    $generatedTask = Get-Content -LiteralPath (Join-Path $packetPath "task.md") -Raw
    if ($generatedTask -notmatch [regex]::Escape("[#9999](https://github.com/wunderforge/agenova/issues/9999)")) {
      throw "task scaffold did not link the source Issue"
    }

    $overwriteRejected = $false
    try {
      & (Join-Path $Root "scripts/new-task.ps1") -Issue 9999 -Slug "contract-check" -Title "Contract Check" -OutputRoot $tempRoot
    }
    catch {
      $overwriteRejected = $true
    }
    if (-not $overwriteRejected) { throw "task scaffold overwrote an existing packet" }
  }
  finally {
    if (Test-Path -LiteralPath $tempRoot) {
      Remove-Item -LiteralPath $tempRoot -Recurse -Force
    }
  }
}

function Test-DeliveryContracts {
  Test-IssueFormContract
  Test-TaskPacketContract

  $headings = @(
    "Linked ticket",
    "Review context",
    "MVP-path outcome",
    "Changes",
    "Scope and deferrals",
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
    @{
      Name = "review-context task packet"
      From = '- Task packet: `work/0019-delivery-contract/task.md`'
      To = '- Task packet:'
    },
    @{
      Name = "scope deferred non-goal"
      From = '- Deferred / non-goal: Automatic-review provider configuration and merge protection settings.'
      To = '- Deferred / non-goal:'
    }
  )) {
    $rejected = $false
    try {
      Test-PullRequestContract -Body $valid.Replace($case.From, $case.To)
    }
    catch {
      $rejected = $true
    }
    if (-not $rejected) { throw "PR contract accepted a missing $($case.Name)" }
  }

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

  Pass "issue, task-packet, PR, and evidence delivery contracts are mechanically validated"
}
