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

function Test-CLICompositionBoundary {
  $cmdMain = Join-Path $Root "cmd/agenova/main.go"
  if (-not (Test-Path -LiteralPath $cmdMain)) {
    Fail "CLI composition root is missing: cmd/agenova/main.go"
  }

  $providerPatterns = @(
    "k8s.io/",
    "sigs.k8s.io/",
    "github.com/kubernetes-sigs/",
    "github.com/wunderforge/agenova/internal/runtime/agentsandbox"
  )

  $targets = @(
    @{ Rel = "cmd"; Extra = @() },
    @{ Rel = "internal/app"; Extra = @() },
    @{ Rel = "internal/cli"; Extra = @(
        "github.com/wunderforge/agenova/internal/operator",
        "github.com/wunderforge/agenova/internal/app"
      )
    }
  )

  $bad = @()
  foreach ($target in $targets) {
    $sourceRoot = Join-Path $Root $target.Rel
    if (-not (Test-Path -LiteralPath $sourceRoot)) {
      Fail "CLI composition path missing: $($target.Rel)"
    }
    $patterns = $providerPatterns + @($target.Extra)
    foreach ($file in Get-ChildItem -LiteralPath $sourceRoot -Recurse -File -Filter *.go) {
      if ($file.Name -like "*_test.go") { continue }
      $raw = Get-Content -LiteralPath $file.FullName -Raw
      if ([string]::IsNullOrWhiteSpace($raw)) { continue }
      foreach ($pattern in $patterns) {
        if ($raw -match [regex]::Escape($pattern)) {
          $bad += "$($file.FullName.Substring($Root.Length + 1)): $pattern"
        }
      }
    }
  }

  if ($bad) { Fail "CLI composition imported provider or command-layer backend types: $($bad -join '; ')" }
  Pass "CLI composition root stays backend-neutral"
}
