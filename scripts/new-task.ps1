param(
  [Parameter(Mandatory=$true)]
  [ValidateRange(1, 999999)]
  [int]$Issue,

  [Parameter(Mandatory=$true)]
  [ValidatePattern("^[a-z0-9]+(?:-[a-z0-9]+)*$")]
  [string]$Slug,

  [Parameter(Mandatory=$true)]
  [ValidateNotNullOrEmpty()]
  [string]$Title,

  [switch]$WithSpec,
  [switch]$WithDesign,

  [string]$Repository = "wunderforge/agenova",
  [string]$OutputRoot
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
if (-not $OutputRoot) { $OutputRoot = Join-Path $Root "work" }
if ($WithDesign -and -not $WithSpec) {
  throw "-WithDesign requires -WithSpec"
}

$packetName = "{0:D4}-{1}" -f $Issue, $Slug
$packetPath = Join-Path $OutputRoot $packetName
if (Test-Path -LiteralPath $packetPath) {
  throw "task packet already exists: $packetPath"
}

$templateRoot = Join-Path $Root "docs/harness/templates"
$issueUrl = "https://github.com/$Repository/issues/$Issue"

function New-PacketFile {
  param(
    [Parameter(Mandatory=$true)][string]$Template,
    [Parameter(Mandatory=$true)][string]$Destination
  )

  $templatePath = Join-Path $templateRoot $Template
  if (-not (Test-Path -LiteralPath $templatePath)) {
    throw "missing task template: $templatePath"
  }

  $content = Get-Content -LiteralPath $templatePath -Raw
  $content = $content.Replace("{{ISSUE}}", $Issue.ToString())
  $content = $content.Replace("{{ISSUE_URL}}", $issueUrl)
  $content = $content.Replace("{{TITLE}}", $Title)
  Set-Content -LiteralPath $Destination -Value $content -Encoding utf8NoBOM
}

New-Item -ItemType Directory -Force -Path $OutputRoot | Out-Null
New-Item -ItemType Directory -Path $packetPath | Out-Null
New-PacketFile -Template "task.md" -Destination (Join-Path $packetPath "task.md")
if ($WithSpec) {
  New-PacketFile -Template "spec.md" -Destination (Join-Path $packetPath "spec.md")
}
if ($WithDesign) {
  New-PacketFile -Template "design.md" -Destination (Join-Path $packetPath "design.md")
}

Write-Host "Created task packet: $packetPath"
Write-Host "Read the linked Issue, replace every TODO marker, and stop for Owner/Reviewer approval before implementation."
