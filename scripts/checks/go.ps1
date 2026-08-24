function Initialize-GoCache {
  if (-not $env:GOCACHE) {
    $cache = Join-Path $Root ".tmp/gocache"
    New-Item -ItemType Directory -Force -Path $cache | Out-Null
    $env:GOCACHE = $cache
  }
}

function Get-GoCheckFiles {
  param([switch]$ChangedOnly)

  if (-not $ChangedOnly) {
    return @(
      Get-ChildItem -LiteralPath $Root -Recurse -File -Filter *.go |
        Where-Object { $_.FullName -notmatch "[\\/](\.git|\.claude|\.tmp|\.worktrees)[\\/]" } |
        ForEach-Object { $_.FullName }
    )
  }

  Push-Location $Root
  try {
    $relativePaths = @(git diff --cached --name-only --diff-filter=ACMR -- "*.go")
    if ($LASTEXITCODE -ne 0) { Fail "could not list staged Go files" }
    return @(
      $relativePaths |
        Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
        ForEach-Object { Join-Path $Root $_ } |
        Where-Object { Test-Path -LiteralPath $_ }
    )
  }
  finally {
    Pop-Location
  }
}

function Test-Fast {
  param([switch]$ChangedOnly)

  if (-not (Get-Command git -ErrorAction SilentlyContinue)) { Fail "git executable not found" }
  if (-not (Get-Command gofmt -ErrorAction SilentlyContinue)) { Fail "gofmt executable not found" }

  Push-Location $Root
  try {
    if ($ChangedOnly) {
      git diff --cached --check
    }
    else {
      git diff --check
    }
    if ($LASTEXITCODE -ne 0) { Fail "whitespace or conflict-marker check failed" }
  }
  finally {
    Pop-Location
  }
  Pass "diff contains no whitespace errors or conflict markers"

  $goFiles = @(Get-GoCheckFiles -ChangedOnly:$ChangedOnly)
  if (-not $goFiles) {
    Pass "no Go files selected for the fast gate"
    return
  }

  $unformatted = gofmt -l $goFiles
  if ($LASTEXITCODE -ne 0) { Fail "gofmt check failed" }
  if ($unformatted) { Fail "gofmt required: $($unformatted -join ', ')" }
  Pass "selected Go files are formatted"

  $missingHeaders = @()
  foreach ($path in $goFiles) {
    $raw = Get-Content -LiteralPath $path -Raw
    if ($raw -notmatch "SPDX-License-Identifier: Apache-2\.0") {
      $missingHeaders += $path.Substring($Root.Length + 1)
    }
  }
  if ($missingHeaders) { Fail "selected Go files missing Apache-2.0 SPDX headers: $($missingHeaders -join ', ')" }
  Pass "selected Go files contain Apache-2.0 SPDX headers"
}

function Test-Go {
  param([switch]$Race)

  if (-not (Get-Command go -ErrorAction SilentlyContinue)) { Fail "go executable not found" }
  Initialize-GoCache

  Push-Location $Root
  try {
    $goFiles = @(Get-GoCheckFiles)
    if ($goFiles) {
      $unformatted = gofmt -l $goFiles
      if ($LASTEXITCODE -ne 0) { Fail "gofmt check failed" }
      if ($unformatted) { Fail "gofmt required: $($unformatted -join ', ')" }
    }
    Pass "current Go files are formatted"

    go mod tidy
    if ($LASTEXITCODE -ne 0) { Fail "go mod tidy failed" }
    git diff --exit-code -- go.mod go.sum
    if ($LASTEXITCODE -ne 0) { Fail "go.mod or go.sum needs 'go mod tidy'" }
    Pass "Go module metadata is tidy"

    go vet ./...
    if ($LASTEXITCODE -ne 0) { Fail "go vet ./... failed" }
    Pass "go vet ./..."

    go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { Fail "go test ./... failed" }
    Pass "go test ./..."

    if ($Race) {
      go test -race -count=1 ./...
      if ($LASTEXITCODE -ne 0) { Fail "go test -race ./... failed" }
      Pass "go test -race ./..."
    }

    go test -run '^$' -tags integration ./harness/integration/agentsandbox/
    if ($LASTEXITCODE -ne 0) { Fail "Agent Sandbox integration package does not compile" }
    Pass "Agent Sandbox integration package compiles"
  }
  finally {
    Pop-Location
  }
}
