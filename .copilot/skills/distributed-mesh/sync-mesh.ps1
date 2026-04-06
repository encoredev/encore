# sync-mesh.ps1 — Materialize remote squad state locally
#
# Reads mesh.json, fetches remote squads into local directories.
# Run before agent reads. No daemon. No service. ~40 lines.
#
# Usage: .\sync-mesh.ps1 [path-to-mesh.json]
#        .\sync-mesh.ps1 -Init [path-to-mesh.json]
# Requires: git
param(
    [switch]$Init,
    [string]$MeshJson = "mesh.json"
)
$ErrorActionPreference = "Stop"

# Handle -Init mode
if ($Init) {
    if (-not (Test-Path $MeshJson)) {
        Write-Host "❌ $MeshJson not found"
        exit 1
    }
    
    Write-Host "🚀 Initializing mesh state repository..."
    $config = Get-Content $MeshJson -Raw | ConvertFrom-Json
    $squads = $config.squads.PSObject.Properties.Name
    
    # Create squad directories with placeholder SUMMARY.md
    foreach ($squad in $squads) {
        if (-not (Test-Path $squad)) {
            New-Item -ItemType Directory -Path $squad | Out-Null
            Write-Host "  ✓ Created $squad/"
        } else {
            Write-Host "  • $squad/ exists (skipped)"
        }
        
        $summaryPath = "$squad/SUMMARY.md"
        if (-not (Test-Path $summaryPath)) {
            "# $squad`n`n_No state published yet._" | Set-Content $summaryPath
            Write-Host "  ✓ Created $summaryPath"
        } else {
            Write-Host "  • $summaryPath exists (skipped)"
        }
    }
    
    # Generate root README.md
    if (-not (Test-Path "README.md")) {
        $readme = @"
# Squad Mesh State Repository

This repository tracks published state from participating squads.

## Participating Squads

"@
        foreach ($squad in $squads) {
            $zone = $config.squads.$squad.zone
            $readme += "- **$squad** (Zone: $zone)`n"
        }
        $readme += @"

Each squad directory contains a ``SUMMARY.md`` with their latest published state.
State is synchronized using ``sync-mesh.sh`` or ``sync-mesh.ps1``.
"@
        $readme | Set-Content "README.md"
        Write-Host "  ✓ Created README.md"
    } else {
        Write-Host "  • README.md exists (skipped)"
    }
    
    Write-Host ""
    Write-Host "✅ Mesh state repository initialized"
    exit 0
}

$config = Get-Content $MeshJson -Raw | ConvertFrom-Json

# Zone 2: Remote-trusted — git clone/pull
foreach ($entry in $config.squads.PSObject.Properties | Where-Object { $_.Value.zone -eq "remote-trusted" }) {
    $squad  = $entry.Name
    $source = $entry.Value.source
    $ref    = if ($entry.Value.ref) { $entry.Value.ref } else { "main" }
    $target = $entry.Value.sync_to

    if (Test-Path "$target/.git") {
        git -C $target pull --rebase --quiet 2>$null
        if ($LASTEXITCODE -ne 0) { Write-Host "⚠ ${squad}: pull failed (using stale)" }
    } else {
        New-Item -ItemType Directory -Force -Path (Split-Path $target -Parent) | Out-Null
        git clone --quiet --depth 1 --branch $ref $source $target 2>$null
        if ($LASTEXITCODE -ne 0) { Write-Host "⚠ ${squad}: clone failed (unavailable)" }
    }
}

# Zone 3: Remote-opaque — fetch published contracts
foreach ($entry in $config.squads.PSObject.Properties | Where-Object { $_.Value.zone -eq "remote-opaque" }) {
    $squad  = $entry.Name
    $source = $entry.Value.source
    $target = $entry.Value.sync_to
    $auth   = $entry.Value.auth

    New-Item -ItemType Directory -Force -Path $target | Out-Null
    $params = @{ Uri = $source; OutFile = "$target/SUMMARY.md"; UseBasicParsing = $true }
    if ($auth -eq "bearer") {
        $tokenVar = ($squad.ToUpper() -replace '-', '_') + "_TOKEN"
        $token = [Environment]::GetEnvironmentVariable($tokenVar)
        if ($token) { $params.Headers = @{ Authorization = "Bearer $token" } }
    }
    try { Invoke-WebRequest @params -ErrorAction Stop }
    catch { "# ${squad} — unavailable ($(Get-Date))" | Set-Content "$target/SUMMARY.md" }
}

Write-Host "✓ Mesh sync complete"
