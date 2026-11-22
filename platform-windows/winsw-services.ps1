<#
.SYNOPSIS
  Creates, migrates and removes the JSON-SCADA Windows services, which run under WinSW.

.DESCRIPTION
  Layout (<root> is the installation folder, normally C:\json-scada):
    <root>\platform-windows\WinSW.exe                    WinSW 2.x (https://github.com/winsw/winsw)
    <root>\platform-windows\service-templates\<svc>.xml  service definitions shipped by the installer;
                                                         {root} and {jwtsecret} are filled in at install
    <root>\platform-windows\services\<svc>.xml           live configuration of an installed service: edit it,
                                                         then restart the service (start mode, failure actions
                                                         and the id are applied only when the service is installed)
    <root>\platform-windows\services\<svc>.exe           hard link to (or copy of) WinSW.exe: the service binary
    <root>\log\<logname>.out.log, .err.log               stdout/stderr, rolled at 10 MB, one rolled file kept;
                                                         <svc>.wrapper.log holds the WinSW messages

  Actions:
    Install  registers the services of the template folder (or the -Name ones) that do not exist yet.
             Existing services keep their settings. A live XML left in services\ is reused, so local edits
             survive a remove/install cycle; -Force re-creates the services from their templates (keeping
             JS_JWT_SECRET). Also refreshes outdated wrapper binaries of stopped services.
    Migrate  converts the JSON_SCADA_* services run by NSSM to WinSW, keeping executable, arguments,
             working directory, environment, log file names and start mode; restarts the ones that ran.
    Remove   stops and deletes the JSON_SCADA_* services (or the -Name ones), whatever runs them, and their
             wrapper binary and live XML (logs are kept).
    Status   lists the JSON_SCADA_* services and what runs them.

  Exit codes: 0 = ok, 1 = at least one service failed.
#>
param(
    [ValidateSet('Install', 'Migrate', 'Remove', 'Status')][string]$Action = 'Install',
    [string[]]$Name = @(),
    [string]$InstallDir = '',
    [string]$TemplateDir = '',
    [switch]$Force
)

$ErrorActionPreference = 'Continue'
if (-not $InstallDir) { $InstallDir = Split-Path -Parent $PSScriptRoot }
$InstallDir = $InstallDir.TrimEnd('\')
$PW = Join-Path $InstallDir 'platform-windows'
$WinSW = Join-Path $PW 'WinSW.exe'
$ServicesDir = Join-Path $PW 'services'
$LogDir = Join-Path $InstallDir 'log'
if (-not $TemplateDir) { $TemplateDir = Join-Path $PW 'service-templates' }
$script:Failures = 0
$Utf8 = New-Object System.Text.UTF8Encoding($false)

function Write-Msg([string]$Msg) { Write-Host $Msg }
function Write-Fail([string]$Msg) { $script:Failures++; Write-Host "ERROR: $Msg" }

function ConvertTo-SvcName([string]$N) {
    if ($N -match '^JSON_SCADA_') { return $N }
    return "JSON_SCADA_$N"
}

function Get-SvcKey([string]$N) { "HKLM:\SYSTEM\CurrentControlSet\Services\$N" }

# Executable path of a service's ImagePath (quoted or not).
function Get-SvcExe([string]$N) {
    $img = (Get-ItemProperty (Get-SvcKey $N) -Name ImagePath -ErrorAction SilentlyContinue).ImagePath
    if (-not $img) { return $null }
    $img = $img.Trim()
    if ($img.StartsWith('"')) { return $img.Substring(1, $img.IndexOf('"', 1) - 1) }
    $i = $img.ToLower().IndexOf('.exe')
    if ($i -ge 0) { return $img.Substring(0, $i + 4) }
    return $img.Split(' ')[0]
}

function Get-SvcManager([string]$N) {
    $exe = Get-SvcExe $N
    if (-not $exe) { return 'none' }
    if ((Split-Path -Leaf $exe) -ieq 'nssm.exe') { return 'nssm' }
    if ($exe -ieq (Join-Path $ServicesDir "$N.exe")) { return 'winsw' }
    return 'other'
}

function Wait-SvcGone([string]$N, [int]$TimeoutSec = 30) {
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Service -Name $N -ErrorAction SilentlyContinue) -and (Get-Date) -lt $deadline) { Start-Sleep -Milliseconds 500 }
    return -not (Get-Service -Name $N -ErrorAction SilentlyContinue)
}

function Stop-JsService([string]$N, [int]$TimeoutSec = 60) {
    $s = Get-Service -Name $N -ErrorAction SilentlyContinue
    if (-not $s -or $s.Status -eq 'Stopped') { return }
    Write-Msg "Stopping $N"
    try { Stop-Service -Name $N -Force -NoWait -ErrorAction Stop } catch {}
    try { $s.WaitForStatus('Stopped', [TimeSpan]::FromSeconds($TimeoutSec)) } catch {}
    $s.Refresh()
    if ($s.Status -ne 'Stopped') {
        $svcPid = (Get-CimInstance Win32_Service -Filter "Name='$N'" -ErrorAction SilentlyContinue).ProcessId
        if ($svcPid) { Stop-Process -Id $svcPid -Force -ErrorAction SilentlyContinue; Start-Sleep -Seconds 1 }
    }
}

# Creates or refreshes services\<svc>.exe: a hard link to WinSW.exe (no extra disk space), else a copy.
function Sync-Wrapper([string]$N) {
    $dst = Join-Path $ServicesDir "$N.exe"
    if (Test-Path $dst) {
        $a = Get-Item $dst; $b = Get-Item $WinSW
        if ($a.Length -eq $b.Length -and $a.VersionInfo.FileVersion -eq $b.VersionInfo.FileVersion) { return $true }
        $s = Get-Service -Name $N -ErrorAction SilentlyContinue
        if ($s -and $s.Status -ne 'Stopped') { Write-Msg "  $N is running, its wrapper binary is refreshed on the next run of this script"; return $true }
        try { Remove-Item $dst -Force -ErrorAction Stop } catch { Write-Fail "cannot replace ${dst}: $($_.Exception.Message)"; return $false }
    }
    try { New-Item -ItemType HardLink -Path $dst -Target $WinSW -ErrorAction Stop | Out-Null }
    catch {
        try { Copy-Item -Path $WinSW -Destination $dst -Force -ErrorAction Stop }
        catch { Write-Fail "cannot create ${dst}: $($_.Exception.Message)"; return $false }
    }
    return $true
}

function New-Secret {
    $chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
    $bytes = New-Object byte[] 26
    [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    -join ($bytes | ForEach-Object { $chars[$_ % $chars.Length] })
}

function Get-XmlEnv([string]$XmlPath, [string]$Var) {
    if (-not (Test-Path $XmlPath)) { return $null }
    try {
        $x = [xml](Get-Content -Raw $XmlPath)
        $e = $x.service.env | Where-Object { $_.name -eq $Var } | Select-Object -First 1
        if ($e) { return $e.value }
    } catch {}
    return $null
}

function Expand-Template([string]$TplPath, [string]$Secret) {
    $t = [IO.File]::ReadAllText($TplPath)
    if (-not $Secret) { $Secret = New-Secret }
    return $t.Replace('{root}', $InstallDir).Replace('{jwtsecret}', $Secret)
}

function Esc([string]$S) { [Security.SecurityElement]::Escape($S) }

# Writes services\<svc>.xml, the wrapper, and registers the service with WinSW.
function Install-WinSWService([string]$N, [string]$Xml) {
    try { [xml]$Xml | Out-Null } catch { Write-Fail "${N}: invalid XML: $($_.Exception.Message)"; return $false }
    New-Item -ItemType Directory -Force -Path $ServicesDir | Out-Null
    New-Item -ItemType Directory -Force -Path $LogDir | Out-Null
    $xmlPath = Join-Path $ServicesDir "$N.xml"
    [IO.File]::WriteAllText($xmlPath, $Xml, $Utf8)
    if (-not (Sync-Wrapper $N)) { return $false }
    $exe = Join-Path $ServicesDir "$N.exe"
    $out = & $exe install 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0 -or -not (Get-Service -Name $N -ErrorAction SilentlyContinue)) {
        Write-Fail "${N}: WinSW install failed (exit $LASTEXITCODE): $($out.Trim())"
        return $false
    }
    Write-Msg "Installed $N ($xmlPath)"
    return $true
}

# Deletes a service whatever runs it; -Purge also deletes its wrapper binary and live XML.
function Remove-JsService([string]$N, [switch]$Purge) {
    if (Get-Service -Name $N -ErrorAction SilentlyContinue) {
        Stop-JsService $N
        $exe = Join-Path $ServicesDir "$N.exe"
        if ((Get-SvcManager $N) -eq 'winsw' -and (Test-Path $exe)) { & $exe uninstall 2>&1 | Out-Null }
        if (Get-Service -Name $N -ErrorAction SilentlyContinue) { & sc.exe delete $N | Out-Null }
        if (-not (Wait-SvcGone $N)) { Write-Fail "$N is marked for deletion but still exists (close services.msc and retry)"; return $false }
        Write-Msg "Removed $N"
    }
    if ($Purge) {
        foreach ($ext in 'exe', 'xml') { Remove-Item (Join-Path $ServicesDir "$N.$ext") -Force -ErrorAction SilentlyContinue }
    }
    return $true
}

# Builds a WinSW XML from the registry settings of an NSSM service.
function ConvertFrom-Nssm([string]$N) {
    $key = Get-SvcKey $N
    $p = Get-ItemProperty "$key\Parameters" -ErrorAction SilentlyContinue
    $svc = Get-ItemProperty $key -ErrorAction SilentlyContinue
    if (-not $p -or -not $p.Application) { return $null }
    $short = $N -replace '^JSON_SCADA_', ''
    $tpl = Join-Path $TemplateDir "$N.xml"
    $envs = @()
    if ($p.AppEnvironment) { $envs += @($p.AppEnvironment) }
    if ($p.AppEnvironmentExtra) { $envs += @($p.AppEnvironmentExtra) }
    $secret = ($envs | Where-Object { $_ -like 'JS_JWT_SECRET=*' } | Select-Object -First 1)
    # services still pointing at removed programs (e.g. the former sql\process_pg_*.bat loaders): use the template
    $app = [Environment]::ExpandEnvironmentVariables($p.Application)
    if ((Test-Path $tpl) -and (($app -match '\.(bat|cmd)$') -or -not (Test-Path $app))) {
        Write-Msg "  $N runs $app, which is obsolete or missing: recreated from its template"
        return Expand-Template $tpl $(if ($secret) { $secret.Substring(14) } else { $null })
    }
    $logFile = if ($p.AppStdout) { $p.AppStdout } elseif ($p.AppStderr) { $p.AppStderr } else { '' }
    $logName = if ($logFile) { [IO.Path]::GetFileNameWithoutExtension($logFile) } else { $short }
    $logPath = if ($logFile) { Split-Path -Parent $logFile } else { $LogDir }
    if (-not $logPath) { $logPath = $LogDir }
    $dir = if ($p.AppDirectory) { $p.AppDirectory } else { Split-Path -Parent $app }
    $delay = if ($p.AppRestartDelay) { [Math]::Max(1, [int]([int]$p.AppRestartDelay / 1000)) } else { 2 }
    $desc = if ($svc.Description) { $svc.Description } else { "JSON-SCADA $short" }
    $lines = @('<service>', "  <!-- converted from the NSSM service settings -->",
        "  <id>$(Esc $N)</id>", "  <name>$(Esc $(if ($svc.DisplayName) { $svc.DisplayName } else { $N }))</name>",
        "  <description>$(Esc $desc)</description>", "  <executable>$(Esc $p.Application)</executable>")
    if ($p.AppParameters) { $lines += "  <arguments>$(Esc $p.AppParameters)</arguments>" }
    $lines += "  <workingdirectory>$(Esc $dir)</workingdirectory>"
    foreach ($e in $envs) {
        $i = $e.IndexOf('=')
        if ($i -gt 0) { $lines += "  <env name=`"$(Esc $e.Substring(0, $i))`" value=`"$(Esc $e.Substring($i + 1))`"/>" }
    }
    $lines += $(if ($svc.Start -eq 2) { '  <startmode>Automatic</startmode>' } else { '  <startmode>Manual</startmode>' })
    if ($svc.Start -eq 2 -and $svc.DelayedAutostart -eq 1) { $lines += '  <delayedAutoStart>true</delayedAutoStart>' }
    $lines += @("  <logpath>$(Esc $logPath)</logpath>", "  <logname>$(Esc $logName)</logname>",
        '  <log mode="roll-by-size">', '    <sizeThreshold>10240</sizeThreshold>', '    <keepFiles>1</keepFiles>', '  </log>',
        "  <onfailure action=`"restart`" delay=`"$delay sec`"/>", '  <resetfailure>1 hour</resetfailure>',
        "  <stoptimeout>$(if ($N -like 'JSON_SCADA_mongodb*') { '120' } else { '15' }) sec</stoptimeout>",
        '  <stopparentprocessfirst>true</stopparentprocessfirst>', '</service>')
    return ($lines -join "`r`n") + "`r`n"
}

function Get-JsServiceNames {
    @(Get-Service -Name 'JSON_SCADA_*' -ErrorAction SilentlyContinue | ForEach-Object { $_.Name } | Sort-Object)
}

function Get-Targets {
    if ($Name.Count) { return @($Name | ForEach-Object { ConvertTo-SvcName $_ }) }
    return $null
}

# ------------------------------------------------------------------------------------------------
function Invoke-Install {
    if (-not (Test-Path $WinSW)) { Write-Fail "$WinSW not found"; return }
    $targets = Get-Targets
    if (-not $targets) { $targets = @(Get-ChildItem -Path $TemplateDir -Filter 'JSON_SCADA_*.xml' -File -ErrorAction SilentlyContinue | ForEach-Object { $_.BaseName }) }
    foreach ($n in $targets) {
        $tpl = Join-Path $TemplateDir "$n.xml"
        $live = Join-Path $ServicesDir "$n.xml"
        $exists = [bool](Get-Service -Name $n -ErrorAction SilentlyContinue)
        if ($exists -and -not $Force) {
            if ((Get-SvcManager $n) -eq 'winsw') { [void](Sync-Wrapper $n) }
            continue
        }
        if ($exists -or $Force) {
            if (-not (Test-Path $tpl)) { Write-Fail "${n}: no template $tpl to re-create it from"; continue }
            $secret = Get-XmlEnv $live 'JS_JWT_SECRET'
            if (-not $secret) {
                $p = (Get-ItemProperty "$(Get-SvcKey $n)\Parameters" -ErrorAction SilentlyContinue).AppEnvironmentExtra
                $s = @($p) | Where-Object { $_ -like 'JS_JWT_SECRET=*' } | Select-Object -First 1
                if ($s) { $secret = $s.Substring(14) }
            }
            $xml = Expand-Template $tpl $secret
            if (-not (Remove-JsService $n)) { continue }
        } elseif (Test-Path $live) {
            $xml = [IO.File]::ReadAllText($live)
        } elseif (Test-Path $tpl) {
            $xml = Expand-Template $tpl $null
        } else { Write-Fail "${n}: no template $tpl"; continue }
        [void](Install-WinSWService $n $xml)
    }
}

function Invoke-Migrate {
    if (-not (Test-Path $WinSW)) { Write-Fail "$WinSW not found"; return }
    $targets = Get-Targets
    if (-not $targets) { $targets = Get-JsServiceNames }
    foreach ($n in $targets) {
        if ((Get-SvcManager $n) -ne 'nssm') { continue }
        $xml = ConvertFrom-Nssm $n
        if (-not $xml) { Write-Fail "${n}: cannot read its NSSM settings, left as is"; continue }
        $start = (Get-ItemProperty (Get-SvcKey $n) -ErrorAction SilentlyContinue).Start
        $wasRunning = (Get-Service -Name $n).Status -eq 'Running'
        Write-Msg "Migrating $n from NSSM to WinSW"
        if (-not (Remove-JsService $n)) { continue }
        if (-not (Install-WinSWService $n $xml)) {
            Write-Fail "${n} was removed from NSSM but not installed with WinSW; fix $(Join-Path $ServicesDir "$n.xml") and run: `"$(Join-Path $ServicesDir "$n.exe")`" install"
            continue
        }
        if ($start -eq 4) { & sc.exe config $n start= disabled | Out-Null }
        if ($wasRunning) { try { Start-Service $n -ErrorAction Stop } catch { Write-Fail "${n} did not start: $($_.Exception.Message)" } }
    }
    # nssm.exe is no longer needed once no service uses it
    foreach ($f in @((Join-Path $PW 'nssm.exe'), (Join-Path $InstallDir 'bin\nssm.exe'))) {
        if (-not (Test-Path $f)) { continue }
        $users = @(Get-CimInstance Win32_Service -ErrorAction SilentlyContinue | Where-Object { $_.PathName -and $_.PathName.ToLower().Contains($f.ToLower()) })
        if ($users.Count -eq 0) { Remove-Item $f -Force -ErrorAction SilentlyContinue; Write-Msg "Deleted $f (no service uses it)" }
        else { Write-Msg "Kept $f, still used by: $(($users | ForEach-Object { $_.Name }) -join ', ')" }
    }
}

function Invoke-Remove {
    $targets = Get-Targets
    if (-not $targets) {
        $targets = @(Get-JsServiceNames) + @(Get-ChildItem -Path $ServicesDir -Filter 'JSON_SCADA_*.xml' -File -ErrorAction SilentlyContinue | ForEach-Object { $_.BaseName })
        $targets = @($targets | Sort-Object -Unique)
    }
    foreach ($n in $targets) { [void](Remove-JsService $n -Purge) }
}

function Invoke-Status {
    foreach ($n in Get-JsServiceNames) {
        $s = Get-CimInstance Win32_Service -Filter "Name='$n'" -ErrorAction SilentlyContinue
        '{0,-40} {1,-6} {2,-8} {3}' -f $n, (Get-SvcManager $n), $s.State, $s.StartMode
    }
}

switch ($Action) {
    'Install' { Invoke-Install }
    'Migrate' { Invoke-Migrate }
    'Remove' { Invoke-Remove }
    'Status' { Invoke-Status }
}
if ($script:Failures) { exit 1 }
exit 0
