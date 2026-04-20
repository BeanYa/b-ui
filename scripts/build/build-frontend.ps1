param(
    [Parameter(Mandatory = $true)]
    [string] $RepoRoot,

    [Parameter(Mandatory = $true)]
    [string] $FrontendDir,

    [Parameter(Mandatory = $true)]
    [string] $FrontendDistDir,

    [Parameter(Mandatory = $true)]
    [string] $BackendWebDir
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

Set-Location -LiteralPath $FrontendDir

if (Test-Path -LiteralPath 'node_modules') {
    Remove-Item -LiteralPath 'node_modules' -Recurse -Force -ErrorAction SilentlyContinue
}

npm ci --include=dev
if ($LASTEXITCODE -ne 0) {
    throw "npm ci failed with exit code $LASTEXITCODE"
}

npm run build:dist
if ($LASTEXITCODE -ne 0) {
    throw "npm run build:dist failed with exit code $LASTEXITCODE"
}

if (Test-Path -LiteralPath $BackendWebDir) {
    Remove-Item -LiteralPath $BackendWebDir -Recurse -Force
}

$null = New-Item -ItemType Directory -Path $BackendWebDir -Force
Copy-Item -Path (Join-Path $FrontendDistDir '*') -Destination $BackendWebDir -Recurse -Force
