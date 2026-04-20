param(
    [string]$Architecture = "amd64"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir "..\..")
$BuildScriptDir = Join-Path $RepoRoot "scripts\build"
$StageDir = Join-Path $RepoRoot "build\tmp\package-windows"
$PackageRoot = Join-Path $StageDir "b-ui-windows"
$OutputDir = Join-Path $RepoRoot "dist\release"
$OutputArchive = Join-Path $OutputDir "b-ui-windows-$Architecture.zip"
$ServiceDir = Join-Path $RepoRoot "src\services\windows"
$BackendOutputPath = Join-Path $RepoRoot "build\out\sui.exe"
$LibcronetBaseUrl = if ($env:LIBCRONET_BASE_URL) { $env:LIBCRONET_BASE_URL } else { 'https://github.com/SagerNet/cronet-go/releases/latest/download' }
$LibcronetUrl = "$LibcronetBaseUrl/libcronet-windows-$Architecture.dll"

Remove-Item -LiteralPath $StageDir -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $OutputArchive -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $PackageRoot | Out-Null
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

    Copy-Item $BackendOutputPath $PackageRoot -Force
    Copy-Item (Join-Path $ServiceDir "b-ui-windows.xml") $PackageRoot -Force
    Copy-Item (Join-Path $ServiceDir "b-ui-windows.bat") $PackageRoot -Force
    Copy-Item (Join-Path $ServiceDir "install-windows.bat") $PackageRoot -Force
    Copy-Item (Join-Path $ServiceDir "uninstall-windows.bat") $PackageRoot -Force
    Invoke-WebRequest -Uri $LibcronetUrl -OutFile (Join-Path $PackageRoot 'libcronet.dll')

    Compress-Archive -Path $PackageRoot -DestinationPath $OutputArchive -Force
} finally {
    Pop-Location
}

Write-Host "Created Windows package at $OutputArchive"
