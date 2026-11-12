# Pan-VlessBank release installer for Windows PowerShell 5.1+.
param(
  [string]$InstallDir = $(Join-Path $HOME 'pan-vlessbank'),
  [string]$Port,
  [string]$ProxyAuth,
  [string]$Release = 'latest',
  [string]$Repository = 'jhopan/Pan-VlessBank',
  [switch]$NoStart
)

$ErrorActionPreference = 'Stop'
if (-not [Environment]::Is64BitOperatingSystem) { throw 'Windows 32-bit tidak didukung.' }
$asset = 'pan-vlessbank_windows_amd64.exe'
if ([string]::IsNullOrWhiteSpace($Port)) { $Port = Read-Host 'Port panel [9000]'; if ([string]::IsNullOrWhiteSpace($Port)) { $Port = '9000' } }
if ($Port -notmatch '^\d+$' -or [int]$Port -lt 1 -or [int]$Port -gt 65535) { throw 'Port harus angka 1 sampai 65535.' }
if ([string]::IsNullOrWhiteSpace($ProxyAuth)) { $ProxyAuth = Read-Host 'Auth proxy user:pass (wajib untuk bind LAN)' }
if ([string]::IsNullOrWhiteSpace($ProxyAuth)) { throw 'Bind LAN 0.0.0.0 perlu auth proxy.' }

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$base = "https://github.com/$Repository/releases/download/$Release"
$binary = Join-Path $InstallDir 'pan-vlessbank.exe'
$checksums = Join-Path $InstallDir 'checksums.txt'
Invoke-WebRequest -Uri "$base/$asset" -OutFile $binary
Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile $checksums
$expected = ((Get-Content $checksums) | Where-Object { $_ -match "\s$([regex]::Escape($asset))$" } | Select-Object -First 1).Split()[0]
$actual = (Get-FileHash -Path $binary -Algorithm SHA256).Hash.ToLowerInvariant()
if ([string]::IsNullOrWhiteSpace($expected) -or $actual -ne $expected.ToLowerInvariant()) { Remove-Item $binary -Force -ErrorAction SilentlyContinue; throw 'Checksum binary tidak cocok.' }

@{
  xray = ''
  base = 1081
  base_http = 0
  listen = '0.0.0.0'
  auth = $ProxyAuth
  tunnel = ''
  tunnel_id = ''
  xray_path = 'xray.exe'
  insecure = $true
} | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $InstallDir 'webui.settings.json')

Write-Host "Binary verified: $binary"
Write-Host "Panel LAN: http://0.0.0.0:$Port/"
Write-Host 'Install xray.exe and cloudflared.exe in this folder or add both to PATH.'
if (-not $NoStart) {
  Start-Process -FilePath $binary -ArgumentList "--port $Port --listen 0.0.0.0 --dir `"$InstallDir`"" -WorkingDirectory $InstallDir -RedirectStandardOutput (Join-Path $InstallDir 'webui.log')
  Write-Host 'Panel started.'
}
