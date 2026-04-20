param(
    [Parameter(Mandatory = $true)]
    [string] $RepoRoot,

    [Parameter(Mandatory = $true)]
    [string] $OutputPath,

    [string] $BuildTags = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if (-not $BuildTags) {
    if ($env:BUILD_TAGS) {
        $BuildTags = $env:BUILD_TAGS
    } elseif ($env:GOOS -eq 'windows') {
        $BuildTags = 'with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_purego,badlinkname,tfogo_checklinkname0,with_tailscale'
    } else {
        $BuildTags = 'with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_musl,badlinkname,tfogo_checklinkname0,with_tailscale'
    }
}

$outputDir = Split-Path -Path $OutputPath -Parent
if ($outputDir) {
    $null = New-Item -ItemType Directory -Path $outputDir -Force
}

Set-Location -LiteralPath $RepoRoot
go build -ldflags '-w -s -checklinkname=0' -tags $BuildTags -o $OutputPath ./src/backend/cmd/b-ui
