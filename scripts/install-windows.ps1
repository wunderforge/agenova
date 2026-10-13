# Per-user installation. No administrator rights or execution-policy changes.
param([string]$Version = '0.1.0-preview.20261013.1', [string]$InstallDirectory = "$env:LOCALAPPDATA\Agenova\bin", [string]$ArchivePath)
$ErrorActionPreference = 'Stop'
if ([Environment]::OSVersion.Platform -ne 'Win32NT') { throw 'Use this installer in Windows PowerShell, not Linux PowerShell.' }
if ($Version -notmatch '^0\.1\.0-preview\.[0-9.]+$') { throw 'Invalid preview version' }
# x64 package also runs under Windows ARM64 emulation; native ARM64 not yet verified.
$name = "agenova_${Version}_windows_amd64.zip"
$base = "https://github.com/Leo1piece/homebrew-agenova/releases/download/v$Version"
$temp = Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
New-Item -ItemType Directory $temp | Out-Null
try {
 $zip = Join-Path $temp $name
 if ($ArchivePath) {
  Copy-Item $ArchivePath $zip
  Copy-Item "$ArchivePath.sha256" "$zip.sha256"
 } else {
  Invoke-WebRequest "$base/$name" -OutFile $zip
  Invoke-WebRequest "$base/$name.sha256" -OutFile "$zip.sha256"
 }
 $expected = ((Get-Content "$zip.sha256" -Raw).Trim() -split '\s+')[0]
 if ($expected -notmatch '^[a-fA-F0-9]{64}$' -or (Get-FileHash $zip -Algorithm SHA256).Hash -ne $expected) { throw 'Checksum verification failed' }
 Expand-Archive $zip -DestinationPath (Join-Path $temp 'unpacked')
 $exe = Join-Path $temp 'unpacked\agenova.exe'
 if (-not (Test-Path $exe)) { throw 'Package missing agenova.exe' }
 New-Item -ItemType Directory -Force $InstallDirectory | Out-Null
 Copy-Item $exe (Join-Path $InstallDirectory 'agenova.exe') -Force
 $userPath = [string][Environment]::GetEnvironmentVariable('Path','User')
 if (($userPath -split ';') -notcontains $InstallDirectory) {
  [Environment]::SetEnvironmentVariable('Path', (($userPath.TrimEnd(';') + ';' + $InstallDirectory).TrimStart(';')), 'User')
 }
 $env:Path = "$InstallDirectory;$env:Path"
 & (Join-Path $InstallDirectory 'agenova.exe') --version
 if ($LASTEXITCODE -ne 0) { throw 'Installed CLI failed to start' }
 Write-Host 'Installed. Open a new terminal, then run: agenova login --server https://app.agenova.app'
} finally { Remove-Item $temp -Recurse -Force }
