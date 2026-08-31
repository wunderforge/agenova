param(
  [ValidateSet("Fast", "PR", "Main", "Backend")]
  [string]$Profile,
  [switch]$ChangedOnly,
  [switch]$All,
  [switch]$Docs,
  [switch]$Unit,
  [switch]$Integration,
  [switch]$Race,
  [string]$KubeContext = "kind-agenova-k8s-lab",
  [string]$Namespace = "default"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot

function Pass($Message) { Write-Host "[pass] $Message" }
function Fail($Message) { throw "[fail] $Message" }

foreach ($module in @(
  "repository.ps1",
  "docs.ps1",
  "architecture.ps1",
  "contracts.ps1",
  "go.ps1",
  "backend.ps1"
)) {
  . (Join-Path $PSScriptRoot "checks/$module")
}

$legacyModeSelected = $All -or $Docs -or $Unit -or $Integration -or $Race
if ($Profile -and $legacyModeSelected) {
  Fail "choose either -Profile or the legacy -All/-Docs/-Unit/-Integration/-Race switches"
}
if ($ChangedOnly -and $Profile -ne "Fast") {
  Fail "-ChangedOnly is supported only with -Profile Fast"
}

$Fast = $false
switch ($Profile) {
  "Fast" {
    $Fast = $true
  }
  "PR" {
    $All = $true
    $Race = $true
  }
  "Main" {
    $All = $true
    $Race = $true
  }
  "Backend" {
    $All = $true
    $Integration = $true
  }
}

if (-not ($Fast -or $All -or $Docs -or $Unit -or $Integration)) {
  $All = $true
}

if ($Fast) {
  Test-Fast -ChangedOnly:$ChangedOnly
  exit 0
}

if ($All -or $Docs) {
  Test-RequiredDocs
  Test-OpenSourceMetadata
  Test-ArchitectureText
  Test-MarkdownLinks
  Test-RuntimeBoundary
  Test-CLICompositionBoundary
  Test-DeliveryContracts
}

if ($All -or $Unit) {
  Test-Go -Race:$Race
}

if ($Integration) {
  Test-AgentSandboxIntegration -KubeContext $KubeContext -Namespace $Namespace
}
