$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot

Push-Location $Root
try {
  git config core.hooksPath .githooks
  if ($LASTEXITCODE -ne 0) { throw "failed to configure core.hooksPath" }
  Write-Host "[pass] Agenova Git hooks enabled from .githooks"
}
finally {
  Pop-Location
}
