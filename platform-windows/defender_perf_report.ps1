# Records Microsoft Defender real-time scanning activity and writes a report of
# what it spends time on (top files, processes, extensions, paths, scans).
# Use it to find what to exclude when MsMpEng.exe uses a lot of CPU.
#
# Run from an elevated PowerShell (Run as Administrator):
#   powershell -ExecutionPolicy Bypass -File C:\json-scada\platform-windows\defender_perf_report.ps1
# Optional: -Seconds 120 -Top 25 -OutDir C:\json-scada\log

param(
  [int]$Seconds = 120,
  [int]$Top = 25,
  [string]$OutDir = 'C:\json-scada\log'
)

$principal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  Write-Error 'This script must be run as Administrator.'
  exit 1
}
if (-not (Get-Command New-MpPerformanceRecording -ErrorAction SilentlyContinue)) {
  Write-Error 'New-MpPerformanceRecording not available (needs Defender platform 4.18.2108 or later).'
  exit 1
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$stamp = Get-Date -Format 'yyyyMMdd_HHmmss'
$etl = Join-Path $OutDir "defender_perf_$stamp.etl"
$report = Join-Path $OutDir "defender_perf_report_$stamp.txt"

Write-Host "Recording Defender activity for $Seconds seconds..."
New-MpPerformanceRecording -RecordTo $etl -Seconds $Seconds

Write-Host 'Building report...'
$fmt = @{ AutoSize = $true }
& {
  "Microsoft Defender performance report - $(Get-Date -Format 's') - $Seconds s recording"
  "Recording: $etl"
  ''
  "=== Top $Top processes (time spent scanning on their behalf) ==="
  Get-MpPerformanceReport -Path $etl -TopProcesses $Top | Format-Table @fmt | Out-String -Width 400
  "=== Top $Top files ==="
  Get-MpPerformanceReport -Path $etl -TopFiles $Top | Format-Table @fmt | Out-String -Width 400
  "=== Top $Top paths ==="
  Get-MpPerformanceReport -Path $etl -TopPaths $Top -TopPathsDepth 3 | Format-Table @fmt | Out-String -Width 400
  "=== Top $Top extensions ==="
  Get-MpPerformanceReport -Path $etl -TopExtensions $Top | Format-Table @fmt | Out-String -Width 400
  "=== Top $Top individual scans ==="
  Get-MpPerformanceReport -Path $etl -TopScans $Top | Format-Table @fmt | Out-String -Width 400
  '=== Current exclusions ==='
  $p = Get-MpPreference
  'Paths:      ' + ($p.ExclusionPath -join '; ')
  'Processes:  ' + ($p.ExclusionProcess -join '; ')
  'Extensions: ' + ($p.ExclusionExtension -join '; ')
} | Out-File -FilePath $report -Encoding utf8 -Width 400

Write-Host "Report saved to: $report"
