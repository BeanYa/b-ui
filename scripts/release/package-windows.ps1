param(
    [string]$Architecture = "amd64"
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir "..\..")
$BuildScriptDir = Join-Path $RepoRoot "scripts\build"
$OutputDir = Join-Path $RepoRoot "dist\release\windows"
$ServiceDir = Join-Path $RepoRoot "src\services\windows"
$BackendOutputPath = Join-Path $RepoRoot "build\out\sui.exe"

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

Push-Location $RepoRoot
try {
    $env:GOOS = 'windows'
    $env:GOARCH = $Architecture
    Remove-Item Env:GOARM -ErrorAction SilentlyContinue

    & pwsh -File (Join-Path $BuildScriptDir "build-frontend.ps1") `
        -RepoRoot $RepoRoot `
        -FrontendDir (Join-Path $RepoRoot "src\frontend") `
        -FrontendDistDir (Join-Path $RepoRoot "src\frontend\dist") `
        -BackendWebDir (Join-Path $RepoRoot "src\backend\internal\infra\web\html")

    & pwsh -File (Join-Path $BuildScriptDir "build-backend.ps1") `
        -RepoRoot $RepoRoot `
        -OutputPath $BackendOutputPath

    Copy-Item $BackendOutputPath $OutputDir -Force
    Copy-Item (Join-Path $ServiceDir "b-ui-windows.xml") $OutputDir -Force
    Copy-Item (Join-Path $ServiceDir "b-ui-windows.bat") $OutputDir -Force
    Copy-Item (Join-Path $ServiceDir "install-windows.bat") $OutputDir -Force
    Copy-Item (Join-Path $ServiceDir "uninstall-windows.bat") $OutputDir -Force
} finally {
    Pop-Location
}

Write-Host "Created Windows package in $OutputDir"
