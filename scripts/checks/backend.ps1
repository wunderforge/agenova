function Test-AgentSandboxIntegration {
  param(
    [string]$KubeContext,
    [string]$Namespace
  )

  if (-not (Get-Command go -ErrorAction SilentlyContinue)) { Fail "go executable not found" }
  if (-not (Get-Command kubectl -ErrorAction SilentlyContinue)) { Fail "kubectl executable not found" }
  Initialize-GoCache

  Push-Location $Root
  try {
    go test -v -tags integration -timeout 5m ./harness/integration/agentsandbox/ -args -kube-context $KubeContext -namespace $Namespace
    if ($LASTEXITCODE -ne 0) { Fail "Agent Sandbox integration gate failed" }
    Pass "Agent Sandbox integration gate"
  }
  finally {
    Pop-Location
  }
}
