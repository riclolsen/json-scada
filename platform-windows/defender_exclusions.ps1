# Adds Microsoft Defender real-time scanning exclusions for {json:scada} data folders.
# Reduces CPU load caused by scanning the SQL files constantly written/deleted by
# cs_data_processor and process_hist/process_rtdata, logs and database files.
#
# Run from an elevated PowerShell (Run as Administrator):
#   powershell -ExecutionPolicy Bypass -File C:\json-scada\platform-windows\defender_exclusions.ps1
# To undo:
#   powershell -ExecutionPolicy Bypass -File C:\json-scada\platform-windows\defender_exclusions.ps1 -Remove

param(
  [string]$JsonScadaRoot = 'C:\json-scada',
  [switch]$Remove
)

$principal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  Write-Error 'This script must be run as Administrator.'
  exit 1
}

$paths = @(
  (Join-Path $JsonScadaRoot 'sql'),
  (Join-Path $JsonScadaRoot 'log'),
  (Join-Path $JsonScadaRoot 'platform-windows\mongodb-data'),
  (Join-Path $JsonScadaRoot 'platform-windows\postgresql-data')
)

foreach ($p in $paths) {
  if (-not (Test-Path $p)) {
    Write-Warning "Folder not found, skipping: $p"
    continue
  }
  if ($Remove) {
    Remove-MpPreference -ExclusionPath $p
    Write-Host "Removed exclusion: $p"
  } else {
    Add-MpPreference -ExclusionPath $p
    Write-Host "Added exclusion:   $p"
  }
}

Write-Host ''
Write-Host 'Current Defender path exclusions:'
(Get-MpPreference).ExclusionPath | ForEach-Object { Write-Host "  $_" }
