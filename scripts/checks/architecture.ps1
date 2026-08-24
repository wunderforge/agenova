function Test-RuntimeBoundary {
  $patterns = @(
    "sigs.k8s.io/agent-sandbox",
    "github.com/kubernetes-sigs/agent-sandbox",
    "agentsandboxv1",
    "AgentSandboxClaim",
    "agents.x-k8s.io",
    "extensions.agents.x-k8s.io"
  )

  $allowedAdapterPath = Join-Path $Root "internal/runtime/agentsandbox"
  $bad = @()
  foreach ($rootName in @("api", "cmd", "internal")) {
    $sourceRoot = Join-Path $Root $rootName
    if (-not (Test-Path -LiteralPath $sourceRoot)) { continue }
    foreach ($file in Get-ChildItem -LiteralPath $sourceRoot -Recurse -File -Filter *.go) {
      if ($file.FullName.StartsWith($allowedAdapterPath, [System.StringComparison]::OrdinalIgnoreCase)) { continue }
      $raw = Get-Content -LiteralPath $file.FullName -Raw
      foreach ($pattern in $patterns) {
        if ($raw -match [regex]::Escape($pattern)) {
          $bad += "$($file.FullName.Substring($Root.Length + 1)): $pattern"
        }
      }
    }
  }

  if ($bad) { Fail "backend-specific shape leaked outside adapter: $($bad -join '; ')" }
  Pass "backend-specific Agent Sandbox shape stays inside its adapter"
}
