Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackageScriptPath = Join-Path $ScriptDir 'package-windows.ps1'
$content = Get-Content -LiteralPath $PackageScriptPath -Raw
$WorkflowRoot = Join-Path (Split-Path -Parent $ScriptDir) '..\.github\workflows'
$WindowsWorkflowPath = Join-Path $WorkflowRoot 'windows.yml'
$ReleaseWorkflowPath = Join-Path $WorkflowRoot 'release.yml'
$WindowsServiceXmlPath = Join-Path (Split-Path -Parent $ScriptDir) '..\src\services\windows\b-ui-windows.xml'
$WindowsControlBatPath = Join-Path (Split-Path -Parent $ScriptDir) '..\src\services\windows\b-ui-windows.bat'
$WindowsInstallBatPath = Join-Path (Split-Path -Parent $ScriptDir) '..\src\services\windows\install-windows.bat'
$WindowsUninstallBatPath = Join-Path (Split-Path -Parent $ScriptDir) '..\src\services\windows\uninstall-windows.bat'
$WindowsBuildScriptPath = Join-Path (Split-Path -Parent $ScriptDir) '..\src\services\windows\build-windows.ps1'
$windowsWorkflow = Get-Content -LiteralPath $WindowsWorkflowPath -Raw
$releaseWorkflow = Get-Content -LiteralPath $ReleaseWorkflowPath -Raw
$windowsServiceXml = Get-Content -LiteralPath $WindowsServiceXmlPath -Raw
$windowsControlBat = Get-Content -LiteralPath $WindowsControlBatPath -Raw
$windowsInstallBat = Get-Content -LiteralPath $WindowsInstallBatPath -Raw
$windowsUninstallBat = Get-Content -LiteralPath $WindowsUninstallBatPath -Raw
$windowsBuildScript = Get-Content -LiteralPath $WindowsBuildScriptPath -Raw

if ($content -match 'src\\services\\windows\\build-windows\.ps1') {
    throw 'package-windows.ps1 still invokes the legacy src/services/windows/build-windows.ps1 script'
}

if ($content -notmatch 'build-frontend\.ps1') {
    throw 'package-windows.ps1 should invoke the centralized frontend build script'
}

if ($content -notmatch 'build-backend\.ps1') {
    throw 'package-windows.ps1 should invoke the centralized backend build script'
}

if ($content -match 'build\\out\\sui\.exe') {
    throw 'package-windows.ps1 still packages the legacy sui.exe backend output'
}

if ($content -notmatch 'build\\out\\b-ui\.exe') {
    throw 'package-windows.ps1 should package build\\out\\b-ui.exe'
}

if ($content -notmatch 'b-ui-windows-\$\{?Architecture\}?\.zip') {
    throw 'package-windows.ps1 should emit b-ui-windows-<arch>.zip'
}

if ($content -notmatch 'build\\out\\b-ui\.exe') {
    throw 'package-windows.ps1 should package build\out\b-ui.exe'
}

if ($content -match 'build\\out\\sui\.exe') {
    throw 'package-windows.ps1 should not package build\out\sui.exe'
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

if ($windowsWorkflow -match '-o\s+sui\.exe' -or $releaseWorkflow -match '-o\s+(?:build/out/)?sui(?:\s|$)') {
    throw 'release workflows still build legacy sui executables'
}

if ($windowsWorkflow -notmatch '-o\s+b-ui\.exe' -or $releaseWorkflow -notmatch '-o\s+(?:build/out/)?b-ui(?:\s|$)') {
    throw 'release workflows should build b-ui-named executables'
}

if ($windowsWorkflow -notmatch 'github\.com/BeanYa/b-ui/src/backend/internal/domain/config\.buildVersion') {
    throw 'windows workflow should stamp the new buildVersion linker path'
}

if ($releaseWorkflow -notmatch 'github\.com/BeanYa/b-ui/src/backend/internal/domain/config\.buildVersion') {
    throw 'release workflow should stamp the new buildVersion linker path'
}

if ($windowsWorkflow -match '\bsui\.exe\b' -or $releaseWorkflow -match '\bsui\b') {
    throw 'release workflows should not emit legacy sui binaries'
}

if ($windowsWorkflow -notmatch '\bb-ui\.exe\b' -or $releaseWorkflow -notmatch '\bb-ui\b') {
    throw 'release workflows should emit b-ui binaries'
}

if ($windowsServiceXml -match 'sui\.exe' -or $windowsControlBat -match 'sui\.exe' -or $windowsInstallBat -match 'sui\.exe' -or $windowsUninstallBat -match 'sui\.exe' -or $windowsBuildScript -match 'sui\.exe') {
    throw 'windows packaging scripts still reference the legacy sui.exe runtime'
}

if ($windowsServiceXml -notmatch 'b-ui\.exe' -or $windowsBuildScript -notmatch 'b-ui\.exe') {
    throw 'windows packaging scripts should reference b-ui.exe'
}

Write-Host 'package-windows script wiring ok'
