param(
    [ValidateScript({ -not [string]::IsNullOrWhiteSpace($_) })]
    [string[]]$Patterns
)

if (-not $PSBoundParameters.ContainsKey('Patterns')) {
    $installRoot = Split-Path -Parent $PSScriptRoot
    $Patterns = @(
        Join-Path $installRoot 'sql\'
        Join-Path $installRoot 'platform-windows\'
        Join-Path $installRoot 'bin\'
    )
}

try {
    $processes = Get-CimInstance -ClassName Win32_Process -ErrorAction Stop
} catch {
    Write-Warning "Failed to query running processes: $_"
    exit 2
}

# The helper and its callers can contain the install path in their own command lines.
$processesById = @{}
foreach ($process in $processes) {
    $processesById[[int]$process.ProcessId] = $process
}
$callerIds = @{}
$callerId = $PID
while ($callerId -gt 0 -and -not $callerIds.ContainsKey($callerId)) {
    $callerIds[$callerId] = $true
    $callerId = [int]$processesById[$callerId].ParentProcessId
}

$exitCode = 0
foreach ($process in $processes) {
    if (-not $process.CommandLine -or $callerIds.ContainsKey([int]$process.ProcessId)) {
        continue
    }

    foreach ($pattern in $Patterns) {
        if ($process.CommandLine.IndexOf($pattern, [StringComparison]::OrdinalIgnoreCase) -lt 0) {
            continue
        }

        try {
            Write-Host "Terminating process $($process.Name) (PID $($process.ProcessId))"
            $result = Invoke-CimMethod -InputObject $process -MethodName Terminate -ErrorAction Stop
            if ($result.ReturnValue -ne 0) {
                Write-Warning "Terminate returned $($result.ReturnValue) for PID $($process.ProcessId)"
                $exitCode = 2
            }
        } catch {
            Write-Warning "Failed to terminate PID $($process.ProcessId): $_"
            $exitCode = 2
        }
        break
    }
}

exit $exitCode
