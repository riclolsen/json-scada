# Adds Microsoft Defender exclusions for {json:scada}, to reduce the CPU used by
# Defender (MsMpEng.exe) on a busy SCADA server.
#
#  - Folder exclusions: SQL files constantly written/deleted by cs_data_processor and
#    process_hist/process_rtdata, logs and database files.
#  - Process exclusions (full paths): JSON-SCADA runtimes and drivers, plus the
#    Windows tools started in a loop by sql\process_pg_hist.bat and process_pg_rtdata.bat
#    (forfiles.exe, PING.EXE). Files opened by excluded processes are not scanned.
#  - cmd.exe is NOT excluded unless -IncludeCmd is given: excluding it lowers protection
#    for every batch script on the machine, not just JSON-SCADA ones.
#
# Run from an elevated PowerShell (Run as Administrator):
#   powershell -ExecutionPolicy Bypass -File C:\json-scada\platform-windows\defender_exclusions.ps1
# Options: -IncludeCmd  -JsonScadaRoot <path>
# To undo everything this script adds:
#   powershell -ExecutionPolicy Bypass -File C:\json-scada\platform-windows\defender_exclusions.ps1 -Remove -IncludeCmd

param(
  [string]$JsonScadaRoot = 'C:\json-scada',
  [switch]$IncludeCmd,
  [switch]$Remove
)

$principal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  Write-Error 'This script must be run as Administrator.'
  exit 1
}

$pw = Join-Path $JsonScadaRoot 'platform-windows'

$paths = @(
  (Join-Path $JsonScadaRoot 'sql'),
  (Join-Path $JsonScadaRoot 'log'),
  (Join-Path $pw 'mongodb-data'),
  (Join-Path $pw 'postgresql-data')
)

$processes = @(
  (Join-Path $pw 'nodejs-runtime\node.exe'),
  (Join-Path $pw 'mongodb-runtime\bin\mongod.exe'),
  (Join-Path $pw 'postgresql-runtime\bin\postgres.exe'),
  (Join-Path $pw 'postgresql-runtime\bin\psql.exe'),
  (Join-Path $env:SystemRoot 'System32\forfiles.exe'),
  (Join-Path $env:SystemRoot 'System32\PING.EXE')
)
# protocol drivers and other JSON-SCADA executables
$binDir = Join-Path $JsonScadaRoot 'bin'
if (Test-Path $binDir) {
  $processes += Get-ChildItem $binDir -Filter *.exe | Select-Object -ExpandProperty FullName
}
if ($IncludeCmd) {
  Write-Warning 'cmd.exe exclusion requested: files opened by ANY batch script will not be scanned.'
  $processes += (Join-Path $env:SystemRoot 'System32\cmd.exe')
}

function Set-Exclusion($kind, $items) {
  foreach ($p in $items) {
    if (-not $Remove -and -not (Test-Path $p)) {
      Write-Warning "Not found, skipping: $p"
      continue
    }
    if ($kind -eq 'Path') {
      if ($Remove) { Remove-MpPreference -ExclusionPath $p } else { Add-MpPreference -ExclusionPath $p }
    } else {
      if ($Remove) { Remove-MpPreference -ExclusionProcess $p } else { Add-MpPreference -ExclusionProcess $p }
    }
    Write-Host ("{0} {1} exclusion: {2}" -f $(if ($Remove) { 'Removed' } else { 'Added  ' }), $kind, $p)
  }
}

Set-Exclusion 'Path' $paths
Set-Exclusion 'Process' $processes

$pref = Get-MpPreference
Write-Host ''
Write-Host 'Current Defender path exclusions:'
$pref.ExclusionPath | ForEach-Object { Write-Host "  $_" }
Write-Host 'Current Defender process exclusions:'
$pref.ExclusionProcess | ForEach-Object { Write-Host "  $_" }
