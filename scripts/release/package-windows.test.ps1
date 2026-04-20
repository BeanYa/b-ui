Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackageScriptPath = Join-Path $ScriptDir 'package-windows.ps1'
$content = Get-Content -LiteralPath $PackageScriptPath -Raw

if ($content -match 'src\\services\\windows\\build-windows\.ps1') {
    throw 'package-windows.ps1 still invokes the legacy src/services/windows/build-windows.ps1 script'
}

if ($content -notmatch 'build-frontend\.ps1') {
    throw 'package-windows.ps1 should invoke the centralized frontend build script'
}

if ($content -notmatch 'build-backend\.ps1') {
    throw 'package-windows.ps1 should invoke the centralized backend build script'
}

Write-Host 'package-windows script wiring ok'
