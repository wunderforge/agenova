# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '../checks/go.ps1')
$script:seenFiles = @()
$script:failFormatter = $false
function gofmt {
  param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Arguments)
  if ($Arguments[0] -ne '-l' -or $Arguments.Count -gt 33) { throw 'unbounded formatter invocation' }
  $script:seenFiles += $Arguments[1..($Arguments.Count - 1)]
  $global:LASTEXITCODE = 0
  if ('file-64.go' -in $Arguments) { 'file-64.go' }
  if ($script:failFormatter) { $global:LASTEXITCODE = 1 }
}
$files = @(0..95 | ForEach-Object { "file-$_.go" })
$bad = @(Get-GoFormatFailures -Files $files)
if (($script:seenFiles -join ',') -ne ($files -join ',')) { throw 'formatter omitted or reordered files' }
if ($bad.Count -ne 1 -or $bad[0] -ne 'file-64.go') { throw 'formatter dropped a later-batch finding' }
$script:failFormatter = $true
$rejected = $false
try { Get-GoFormatFailures -Files $files | Out-Null } catch { $rejected = $true }
if (-not $rejected) { throw 'formatter error did not fail closed' }
$global:LASTEXITCODE = 0 # The simulated failure above was expected and asserted.
Write-Host '[pass] formatter batches every file and preserves findings/errors'
