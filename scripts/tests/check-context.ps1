$ErrorActionPreference = "Stop"
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("agenova-check-context-" + [guid]::NewGuid())
$pwsh = (Get-Process -Id $PID).Path

try {
  $scriptRoot = Join-Path $tempRoot "scripts"
  $checksRoot = Join-Path $scriptRoot "checks"
  $testsRoot = Join-Path $scriptRoot "tests"
  New-Item -ItemType Directory -Path $checksRoot, $testsRoot -Force | Out-Null
  $entrypoint = Join-Path $scriptRoot "check.ps1"
  Copy-Item -LiteralPath (Join-Path $PSScriptRoot "../check.ps1") -Destination $entrypoint
  $callLog = Join-Path $scriptRoot "calls.txt"

  # Run the real entry point in a child process. Replace only its check modules
  # so even a regressed guard cannot run Go, kubectl, or touch a real cluster.
  foreach ($module in @("repository", "docs", "architecture", "contracts", "go", "frontend", "backend")) {
    Set-Content -LiteralPath (Join-Path $checksRoot "$module.ps1") -Value ""
  }
  $baselineCalls = @(
    "Test-RequiredDocs", "Test-OpenSourceMetadata", "Test-ArchitectureText",
    "Test-MarkdownLinks", "Test-RuntimeBoundary", "Test-CLICompositionBoundary",
    "Test-DeliveryContracts", "Test-CheckContext", "Test-Go", "Test-Frontend"
  )
  $stubs = '$CallLog = Join-Path (Split-Path -Parent $PSScriptRoot) "calls.txt"' + "`n"
  foreach ($name in $baselineCalls) {
    $stubs += @'
function __NAME__ { Add-Content -LiteralPath $CallLog -Value '__NAME__' }

'@.Replace("__NAME__", $name)
  }
  $stubs += @'
function Test-AgentSandboxIntegration {
  param([string]$KubeContext, [string]$Namespace)
  Add-Content -LiteralPath $CallLog -Value "backend|$KubeContext|$Namespace"
}
'@
  Set-Content -LiteralPath (Join-Path $checksRoot "backend.ps1") -Value $stubs
  Set-Content -LiteralPath (Join-Path $testsRoot "check-context.ps1") -Value "Test-CheckContext"

  $cases = @()
  foreach ($mode in @(
    @{ Name = "Integration"; Arguments = @("-Integration") },
    @{ Name = "Backend"; Arguments = @("-Profile", "Backend") }
  )) {
    foreach ($context in @(
      @{ Name = "omitted"; Arguments = @() },
      @{ Name = "empty"; Arguments = @("-KubeContext", "") },
      @{ Name = "whitespace"; Arguments = @("-KubeContext", " `t ") }
    )) {
      $cases += @{
        Name = "$($mode.Name)/$($context.Name)"
        Arguments = $mode.Arguments + $context.Arguments
        Reject = $true
        Calls = @()
      }
    }
    $cases += @{
      Name = "$($mode.Name)/explicit"
      Arguments = $mode.Arguments + @("-KubeContext", "review-context", "-Namespace", "review-namespace")
      Reject = $false
      Calls = @(if ($mode.Name -eq "Backend") { $baselineCalls }) + @("backend|review-context|review-namespace")
    }
  }
  $cases += @{ Name = "All/no-context"; Arguments = @("-All"); Reject = $false; Calls = $baselineCalls }

  foreach ($case in $cases) {
    Remove-Item -LiteralPath $callLog -ErrorAction SilentlyContinue
    $arguments = $case.Arguments
    $output = & $pwsh -NoLogo -NoProfile -File $entrypoint @arguments 2>&1 | Out-String
    $exitCode = $LASTEXITCODE
    $calls = @(if (Test-Path -LiteralPath $callLog) { Get-Content -LiteralPath $callLog })
    if ($case.Reject) {
      if ($exitCode -eq 0 -or $output -notlike "*integration requires explicit -KubeContext*") {
        throw "$($case.Name): expected explicit-context rejection; exit=$exitCode; output=$output"
      }
    } elseif ($exitCode -ne 0) {
      throw "$($case.Name): expected success; exit=$exitCode; output=$output"
    }
    if (($calls -join "`n") -cne ($case.Calls -join "`n")) {
      throw "$($case.Name): unexpected check calls: $($calls -join ', ')"
    }
    Write-Host "[case] $($case.Name): exit=$exitCode; calls=$($calls.Count)"
  }
  Write-Host "[pass] integration entry points require explicit context before checks and preserve explicit arguments"
} finally {
  Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}
