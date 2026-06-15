param(
  [Parameter(Mandatory=$true)][string]$Phase,
  [Parameter(Mandatory=$true)][string]$Gate,
  [Parameter(Mandatory=$true)][string]$Command
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$safePhase = $Phase -replace "[^A-Za-z0-9._-]", "-"
$safeGate = $Gate -replace "[^A-Za-z0-9._-]", "-"
$outDir = Join-Path $Root "docs/evidence/$safePhase/$safeGate/$stamp"
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

$summary = Join-Path $outDir "summary.md"
$output = Join-Path $outDir "output.txt"

Push-Location $Root
try {
  if (-not $env:GOCACHE) { $env:GOCACHE = Join-Path $Root ".gocache" }

  $branch = (git -c safe.directory=$Root rev-parse --abbrev-ref HEAD 2>$null)
  $commit = (git -c safe.directory=$Root rev-parse HEAD 2>$null)

  @(
    "# Evidence Summary",
    "",
    "- Phase: $Phase",
    "- Gate: $Gate",
    "- Date: $(Get-Date -Format o)",
    "- Branch: $branch",
    "- Commit: $commit",
    "- Command: ``$Command``",
    "",
    "Raw output: ``output.txt``",
    "",
    "Result: pending"
  ) | Set-Content -LiteralPath $summary -Encoding UTF8

  $wrappedCommand = "`$ErrorActionPreference = 'Continue'; $Command; if (`$null -ne `$LASTEXITCODE) { exit `$LASTEXITCODE }"
  powershell -NoProfile -ExecutionPolicy Bypass -Command $wrappedCommand *>&1 |
    Tee-Object -FilePath $output

  if ($LASTEXITCODE -ne 0) {
    $rawSummary = Get-Content -LiteralPath $summary -Raw
    ($rawSummary -replace "Result: pending", "Result: fail") | Set-Content -LiteralPath $summary -Encoding UTF8
    throw "evidence command failed: $Command"
  }

  $rawSummary = Get-Content -LiteralPath $summary -Raw
  ($rawSummary -replace "Result: pending", "Result: pass") | Set-Content -LiteralPath $summary -Encoding UTF8
  Write-Host "[pass] evidence captured at $outDir"
}
finally {
  Pop-Location
}
