function Test-Frontend {
  if (-not (Get-Command node -ErrorAction SilentlyContinue)) { Fail "Node.js 24 is required for frontend checks" }
  if (-not (Get-Command npm -ErrorAction SilentlyContinue)) { Fail "npm is required for frontend checks" }
  if (-not (Test-Path -LiteralPath (Join-Path $Root "ui/node_modules"))) {
    Fail "install frontend dependencies with 'npm --prefix ui ci' and Chromium with 'npm --prefix ui run browsers:install'"
  }
  Initialize-GoCache
  Push-Location $Root
  try {
    npm --prefix ui run check
    if ($LASTEXITCODE -ne 0) { Fail "frontend contract, type, component, build or browser smoke check failed" }
    Pass "frontend contracts, types, components, production build and browser smoke"
  }
  finally { Pop-Location }
}
