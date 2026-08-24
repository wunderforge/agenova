param(
  [AllowEmptyString()][string]$Body,
  [string]$BodyPath
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot "checks/contracts.ps1")

if ($BodyPath) {
  if (-not (Test-Path -LiteralPath $BodyPath)) { throw "PR body file not found: $BodyPath" }
  $Body = Get-Content -LiteralPath $BodyPath -Raw
}

Test-PullRequestContract -Body $Body
Write-Host "[pass] PR delivery contract"
