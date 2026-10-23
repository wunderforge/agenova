# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
# Disposable loopback acceptance only. Never calls AWS or reads real credentials.
param([switch]$PrepareOnly, [switch]$SkipBuild)
$ErrorActionPreference = 'Stop'
$previewRoot = Split-Path -Parent $PSScriptRoot
$previewContainer = 'agenova-final-local-pg-20261023'
$previewOwner = 'final-local-20261023'
$previewImage = 'postgres:17-bookworm'
$previewGo = if (Get-Command go -ErrorAction SilentlyContinue) { 'go' } else { 'D:\Softwares\Go\bin\go.exe' }
$previewNode = if (Get-Command node -ErrorAction SilentlyContinue) { Split-Path (Get-Command node).Source } else { 'D:\Softwares\node' }
$previousEnvironment = @{}
foreach ($name in @('PATH','GOTMPDIR','GOCACHE','TEMP','TMP')) { $previousEnvironment[$name] = [Environment]::GetEnvironmentVariable($name,'Process') }
Push-Location $previewRoot
try {
 $env:PATH = $previewNode + ';' + $env:PATH
 $env:GOTMPDIR = Join-Path $previewRoot '.tmp/go-build'
 $env:GOCACHE = Join-Path $previewRoot '.tmp/local-go-cache'
 $env:TEMP = $env:GOTMPDIR
 $env:TMP = $env:TEMP
 New-Item -ItemType Directory -Force -Path $env:GOTMPDIR,$env:GOCACHE | Out-Null
 & docker info --format '{{.ServerVersion}}' | Out-Null
 if ($LASTEXITCODE -ne 0) { throw 'Start Docker Desktop first. This script does not configure Docker or cloud services.' }
 & docker image inspect $previewImage --format '{{.Id}}' | Out-Null
 if ($LASTEXITCODE -ne 0) { throw "Required cached image missing: $previewImage. No image was downloaded." }
 $existing = & docker ps -a --filter "name=^/$previewContainer`$" --format '{{.Names}}'
 if ($existing -eq $previewContainer) {
  $labels = & docker inspect $previewContainer --format '{{json .Config.Labels}}' | ConvertFrom-Json
  $ports = & docker inspect $previewContainer --format '{{json .HostConfig.PortBindings}}' | ConvertFrom-Json
  $binding = $ports.'5432/tcp'
  if ($labels.'agenova.owner' -ne $previewOwner -or $binding.Count -ne 1 -or $binding[0].HostIp -ne '127.0.0.1' -or $binding[0].HostPort -ne '55439') { throw 'Container ownership or loopback binding differs; refusing to reuse or modify it.' }
  & docker start $previewContainer | Out-Null
 } else {
  & docker run -d --name $previewContainer --label "agenova.owner=$previewOwner" -p 127.0.0.1:55439:5432 -e POSTGRES_HOST_AUTH_METHOD=trust -e POSTGRES_DB=agenova_local $previewImage | Out-Null
 }
 if ($LASTEXITCODE -ne 0) { throw 'Local PostgreSQL could not start. No cloud action was attempted.' }
 $ready = $false
 for ($attempt=0; $attempt -lt 30; $attempt++) {
  & docker exec $previewContainer pg_isready -U postgres -d agenova_local | Out-Null
  if ($LASTEXITCODE -eq 0) { $ready=$true; break }
  Start-Sleep -Seconds 1
 }
 if (-not $ready) { throw 'Local PostgreSQL not ready; preserve the container for diagnosis.' }
 if (-not $SkipBuild) {
  Push-Location 'harness/integration/mcpfixture'
  try { & $previewGo build -o (Join-Path $previewRoot '.tmp/local-mcp.exe') .; if ($LASTEXITCODE -ne 0) { throw 'Local MCP build failed.' } } finally { Pop-Location }
  & npm --prefix ui run build
  if ($LASTEXITCODE -ne 0) { throw 'UI build failed; no service was started.' }
 }
 if (-not (Test-Path -LiteralPath 'ui/dist/index.html')) { throw 'Build the UI first.' }
 if (-not (Test-Path -LiteralPath '.tmp/local-mcp.exe')) { throw 'Build the local MCP first (run without -SkipBuild).' }
 Write-Host 'Local-only database: 127.0.0.1:55439. Synthetic data; no real credentials.'
 if ($PrepareOnly) { Write-Host 'Prepared. Run again without -PrepareOnly to start the preview.'; return }
 Write-Host 'Open http://127.0.0.1:5210/login . Press Ctrl+C to stop the preview; local data stays in its labelled test container.'
 & $previewGo run ./tools/local-product-preview
 if ($LASTEXITCODE -ne 0) { throw 'Preview stopped or could not acquire its exclusive writer. Do not start a second copy.' }
} finally {
 Pop-Location
 foreach ($entry in $previousEnvironment.GetEnumerator()) { [Environment]::SetEnvironmentVariable($entry.Key,$entry.Value,'Process') }
}
