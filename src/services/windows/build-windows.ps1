param(
    [string]$Architecture = "amd64",
    [switch]$NoCGO,
    [switch]$Help
)

if ($Help) {
    Write-Host "Usage: .\src\services\windows\build-windows.ps1 [-Architecture <arch>] [-NoCGO] [-Help]"
    Write-Host "Architectures: amd64, 386, arm64"
    Write-Host "Examples:"
    Write-Host "  .\build-windows.ps1                    # Build for amd64 with CGO"
    Write-Host "  .\build-windows.ps1 -Architecture 386 # Build for 32-bit Windows"
    Write-Host "  .\build-windows.ps1 -NoCGO            # Build without CGO"
    exit 0
}

Write-Host "Building B-UI for Windows ($Architecture)..." -ForegroundColor Green

$ServiceDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ServiceDir "..\..\..")
$FrontendDir = Join-Path $RepoRoot "src\frontend"
$FrontendDistDir = Join-Path $FrontendDir "dist"
$BackendWebDir = Join-Path $RepoRoot "src\backend\internal\infra\web\html"

# Check if Go is installed
try {
    $goVersion = go version 2>$null
    if ($LASTEXITCODE -ne 0) {
        throw "Go not found"
    }
    Write-Host "Go version: $goVersion" -ForegroundColor Green
} catch {
    Write-Host "Error: Go is not installed or not in PATH" -ForegroundColor Red
    Write-Host "Please install Go from https://golang.org/dl/" -ForegroundColor Yellow
    exit 1
}

# Check if Node.js is installed
try {
    $nodeVersion = node --version 2>$null
    if ($LASTEXITCODE -ne 0) {
        throw "Node.js not found"
    }
    Write-Host "Node.js version: $nodeVersion" -ForegroundColor Green
} catch {
    Write-Host "Error: Node.js is not installed or not in PATH" -ForegroundColor Red
    Write-Host "Please install Node.js from https://nodejs.org/" -ForegroundColor Yellow
    exit 1
}

Write-Host "Building frontend..." -ForegroundColor Yellow
Push-Location $FrontendDir

try {
    Write-Host "Installing dependencies..." -ForegroundColor Cyan
    npm ci
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to install frontend dependencies"
    }

    Write-Host "Building frontend..." -ForegroundColor Cyan
    npm run build:dist
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to build frontend"
    }

    if (Test-Path $BackendWebDir) {
        Remove-Item -Recurse -Force $BackendWebDir
    }
    New-Item -ItemType Directory -Force -Path $BackendWebDir | Out-Null
    Copy-Item (Join-Path $FrontendDistDir "*") $BackendWebDir -Recurse -Force
} catch {
    Write-Host "Error: $_" -ForegroundColor Red
    Pop-Location
    exit 1
}

Pop-Location

Write-Host "Building backend..." -ForegroundColor Yellow
Push-Location $RepoRoot

$env:GOOS = "windows"
$env:GOARCH = $Architecture

if ($NoCGO) {
    $env:CGO_ENABLED = "0"
    Write-Host "Building without CGO..." -ForegroundColor Yellow
} else {
    $env:CGO_ENABLED = "1"
    Write-Host "Building with CGO..." -ForegroundColor Yellow
}

$buildCmd = "go build -ldflags `"-w -s`" -tags `"with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_tailscale`" -o sui.exe ./src/backend/cmd/b-ui"

try {
    Invoke-Expression $buildCmd
    if ($LASTEXITCODE -ne 0) {
        if (!$NoCGO) {
            Write-Host "CGO build failed, trying without CGO..." -ForegroundColor Yellow
            $env:CGO_ENABLED = "0"
            Invoke-Expression $buildCmd
            if ($LASTEXITCODE -ne 0) {
                throw "Failed to build backend even without CGO"
            }
            Write-Host "Built without CGO (some features may be limited)" -ForegroundColor Yellow
        } else {
            throw "Failed to build backend"
        }
    } else {
        if ($env:CGO_ENABLED -eq "1") {
            Write-Host "Built successfully with CGO" -ForegroundColor Green
        } else {
            Write-Host "Built successfully without CGO" -ForegroundColor Green
        }
    }
} catch {
    Write-Host "Error: $_" -ForegroundColor Red
    exit 1
} finally {
    Pop-Location
}

Write-Host "Build completed successfully!" -ForegroundColor Green
Write-Host "Output: sui.exe" -ForegroundColor Green

# Show file info
if (Test-Path "sui.exe") {
    $fileInfo = Get-Item "sui.exe"
    Write-Host "File size: $([math]::Round($fileInfo.Length / 1MB, 2)) MB" -ForegroundColor Cyan
    Write-Host "Created: $($fileInfo.CreationTime)" -ForegroundColor Cyan
}
