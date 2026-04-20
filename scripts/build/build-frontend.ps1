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
npm ci
npm run build:dist

if (Test-Path -LiteralPath $BackendWebDir) {
    Remove-Item -LiteralPath $BackendWebDir -Recurse -Force
}

$null = New-Item -ItemType Directory -Path $BackendWebDir -Force
Copy-Item -Path (Join-Path $FrontendDistDir '*') -Destination $BackendWebDir -Recurse -Force
