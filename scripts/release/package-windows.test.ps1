Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackageScriptPath = Join-Path $ScriptDir 'package-windows.ps1'
$content = Get-Content -LiteralPath $PackageScriptPath -Raw
$WorkflowRoot = Join-Path (Split-Path -Parent $ScriptDir) '..\.github\workflows'
$WindowsWorkflowPath = Join-Path $WorkflowRoot 'windows.yml'
$ReleaseWorkflowPath = Join-Path $WorkflowRoot 'release.yml'
$windowsWorkflow = Get-Content -LiteralPath $WindowsWorkflowPath -Raw
$releaseWorkflow = Get-Content -LiteralPath $ReleaseWorkflowPath -Raw

if ($content -match 'src\\services\\windows\\build-windows\.ps1') {
    throw 'package-windows.ps1 still invokes the legacy src/services/windows/build-windows.ps1 script'
}

if ($content -notmatch 'build-frontend\.ps1') {
    throw 'package-windows.ps1 should invoke the centralized frontend build script'
}

if ($content -notmatch 'build-backend\.ps1') {
    throw 'package-windows.ps1 should invoke the centralized backend build script'
}

if ($content -notmatch 'b-ui-windows-\$\{?Architecture\}?\.zip') {
    throw 'package-windows.ps1 should emit b-ui-windows-<arch>.zip'
}

if ($content -notmatch 'libcronet\.dll') {
    throw 'package-windows.ps1 should place libcronet.dll into the Windows package'
}

if ($content -notmatch 'libcronet-windows-\$\{?Architecture\}?\.dll') {
    throw 'package-windows.ps1 should download the architecture-specific libcronet artifact'
}

if ($content -notmatch 'Compress-Archive') {
    throw 'package-windows.ps1 should create a zip archive for release packaging'
}

if ($windowsWorkflow -match 'github\.com/alireza0/s-ui/config\.buildVersion' -or $releaseWorkflow -match 'github\.com/alireza0/s-ui/config\.buildVersion') {
    throw 'release workflows still stamp the old buildVersion linker path'
}

if ($windowsWorkflow -notmatch 'github\.com/BeanYa/b-ui/src/backend/internal/domain/config\.buildVersion') {
    throw 'windows workflow should stamp the new buildVersion linker path'
}

if ($releaseWorkflow -notmatch 'github\.com/BeanYa/b-ui/src/backend/internal/domain/config\.buildVersion') {
    throw 'release workflow should stamp the new buildVersion linker path'
}

Write-Host 'package-windows script wiring ok'
