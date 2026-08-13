param(
  [Parameter(Mandatory=$true)][string]$Ticket,
  [Parameter(Mandatory=$true)][string]$Gate,
  [Parameter(Mandatory=$true)][string]$Command
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$safeTicket = $Ticket -replace "[^A-Za-z0-9._-]", "-"
$safeGate = $Gate -replace "[^A-Za-z0-9._-]", "-"
$outDir = Join-Path $Root "docs/evidence/$safeTicket/$safeGate"
$summary = Join-Path $outDir "summary.md"
$output = Join-Path $outDir "output.txt"

New-Item -ItemType Directory -Force -Path $outDir | Out-Null

Push-Location $Root
try {
  if (-not $env:GOCACHE) {
    $env:GOCACHE = Join-Path $Root ".tmp/gocache"
    New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
  }

  $branch = (git -c safe.directory=$Root rev-parse --abbrev-ref HEAD 2>$null)
  $commit = (git -c safe.directory=$Root rev-parse HEAD 2>$null)

  @(
    "# Evidence Summary",
    "",
    "- Ticket: $Ticket",
    "- Gate: $Gate",
    "- Date: $(Get-Date -Format o)",
    "- Branch: $branch",
    "- Commit: $commit",
    "- Command: ``$Command``",
    "- Result: pending",
    "",
    "Raw output: ``output.txt``"
  ) | Set-Content -LiteralPath $summary -Encoding UTF8

  $wrapped = "`$ErrorActionPreference = 'Continue'; $Command; if (`$null -ne `$LASTEXITCODE) { exit `$LASTEXITCODE }"
  powershell -NoProfile -ExecutionPolicy Bypass -Command $wrapped *>&1 |
    Tee-Object -FilePath $output

  $result = if ($LASTEXITCODE -eq 0) { "pass" } else { "fail" }
  $raw = Get-Content -LiteralPath $summary -Raw
  ($raw -replace "Result: pending", "Result: $result") | Set-Content -LiteralPath $summary -Encoding UTF8

  if ($LASTEXITCODE -ne 0) { throw "evidence command failed: $Command" }
  Write-Host "[pass] evidence captured at $outDir"
}
finally {
  Pop-Location
}
