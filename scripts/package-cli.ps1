param([Parameter(Mandatory=$true)][string]$Version, [string]$OutputDirectory = '.tmp/cli-release')
$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^0\.1\.0-preview\.[0-9.]+$') { throw 'Expected a preview version' }
$platform = if ($IsWindows) { 'windows' } elseif ($IsMacOS) { 'darwin' } else { 'linux' }
$arch = (& go env GOARCH).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Go unavailable' }
$name = "agenova_${Version}_${platform}_${arch}"
$stage = Join-Path $OutputDirectory $name
New-Item -ItemType Directory -Force $stage | Out-Null
$binary = if ($IsWindows) { 'agenova.exe' } else { 'agenova' }
& go build -trimpath "-ldflags=-X github.com/wunderforge/agenova/internal/cli.Version=$Version" -o (Join-Path $stage $binary) ./cmd/agenova
if ($LASTEXITCODE -ne 0) { throw 'CLI build failed' }
& (Join-Path $stage $binary) --version
if ($LASTEXITCODE -ne 0) { throw 'CLI smoke failed' }
if ($IsWindows) {
 $archive = Join-Path $OutputDirectory "$name.zip"
 Compress-Archive -Path (Join-Path $stage $binary) -DestinationPath $archive -Force
} else {
 $archive = Join-Path $OutputDirectory "$name.tar.gz"
 & tar -czf $archive -C $stage $binary
 if ($LASTEXITCODE -ne 0) { throw 'Archive failed' }
}
$hash = (Get-FileHash $archive -Algorithm SHA256).Hash.ToLowerInvariant()
"$hash  $([IO.Path]::GetFileName($archive))" | Set-Content -Encoding ascii "$archive.sha256"
