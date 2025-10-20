param(
    [Parameter(Mandatory=$true)]
    [string[]]$Patterns
)

# For each pattern, find processes where CommandLine contains the pattern, then terminate them.
# Uses CIM (Get-CimInstance) instead of WMIC. Works on PowerShell 5.1 and later.

$errors = @()
foreach ($p in $Patterns) {
    try {
        # Escape single quotes in pattern for WQL-like contains by using simple substring match in PowerShell
        $pat = $p
        $matches = Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -and ($_.CommandLine -like "*$pat*") }
        foreach ($proc in $matches) {
            try {
                Write-Host "Terminating pid=$($proc.ProcessId) cmdline='$($proc.CommandLine)'"
                $rc = $proc | Invoke-CimMethod -MethodName Terminate
                if ($rc.ReturnValue -ne 0) { Write-Warning "Terminate returned $($rc.ReturnValue) for pid $($proc.ProcessId)" }
            } catch {
                Write-Warning "Failed to terminate pid $($proc.ProcessId): $_"
                $errors += $_
            }
        }
    } catch {
        Write-Warning "Error querying processes for pattern '$p': $_"
        $errors += $_
    }
}

if ($errors.Count -gt 0) { exit 2 } else { exit 0 }
