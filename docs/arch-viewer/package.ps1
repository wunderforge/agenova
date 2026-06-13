param(
  [string]$OutputDir,
  [switch]$Clean
)

$ErrorActionPreference = "Stop"

$ViewerRoot = $PSScriptRoot
$Root = Split-Path -Parent (Split-Path -Parent $ViewerRoot)
if ([string]::IsNullOrWhiteSpace($OutputDir)) {
  $OutputDir = Join-Path $Root "dist/arch-viewer-site"
}

$sourceStatus = Join-Path $Root "docs/status/implementation-status.js"

if (-not (Test-Path -LiteralPath $sourceStatus)) {
  throw "missing status file: $sourceStatus"
}

if ($Clean -and (Test-Path -LiteralPath $OutputDir)) {
  $resolvedOutput = Resolve-Path -LiteralPath $OutputDir
  $resolvedRoot = Resolve-Path -LiteralPath $Root
  if (-not $resolvedOutput.Path.StartsWith($resolvedRoot.Path, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "refusing to clean output outside repo: $($resolvedOutput.Path)"
  }
  Remove-Item -LiteralPath $resolvedOutput.Path -Recurse -Force
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $OutputDir "status") | Out-Null

Copy-Item -LiteralPath (Join-Path $ViewerRoot "app.js") -Destination (Join-Path $OutputDir "app.js") -Force
Copy-Item -LiteralPath (Join-Path $ViewerRoot "data.js") -Destination (Join-Path $OutputDir "data.js") -Force
Copy-Item -LiteralPath (Join-Path $ViewerRoot "README.md") -Destination (Join-Path $OutputDir "README.md") -Force
Copy-Item -LiteralPath $sourceStatus -Destination (Join-Path $OutputDir "status/implementation-status.js") -Force

$indexSource = Join-Path $ViewerRoot "index.html"
$indexTarget = Join-Path $OutputDir "index.html"
$index = Get-Content -LiteralPath $indexSource -Raw
$index = $index.Replace("../status/implementation-status.js", "status/implementation-status.js")
Set-Content -LiteralPath $indexTarget -Value $index -Encoding UTF8

Write-Host "Packaged arch viewer site:"
Write-Host "  $OutputDir"
Write-Host ""
Write-Host "Upload or publish the contents of this folder with any static hosting provider. Use index.html as the index document."
