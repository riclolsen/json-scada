<#
.SYNOPSIS
  In-place upgrade helper for the JSON-SCADA Windows installer (json-scada.nsi).

.DESCRIPTION
  The installer runs this script around the file copy when it finds an existing installation.

  -Phase PreInstall (before any file is replaced):
    * records which JSON_SCADA_* services are running, then stops all of them (applications,
      Grafana/Metabase, databases last) and ends leftover processes running from the install dir
    * backs up conf\, the platform-windows batch scripts, the WinSW service XML files and mongod.cfg to -BackupDir
    * keeps the current mongod.exe as mongod-prev.exe, so data with an old feature compatibility
      version can still be opened
    * when the PostgreSQL data major version differs from the bundled one, moves the current
      PostgreSQL runtime to postgresql-runtime-<data major> so the installer does not overwrite it

  -Phase PostInstall (after the new files are in place):
    * points JSON_SCADA_postgresql at the runtime that matches its data
    * converts the services still run by NSSM to WinSW (winsw-services.ps1 -Action Migrate, settings kept)
      and registers the services of platform-windows\service-templates that do not exist yet
      (winsw-services.ps1 -Action Install, existing services keep their settings); log.io file paths of
      the WinSW services are switched to their <logname>.out.log names
    * starts the databases, falls back to mongod-prev.exe when the new mongod cannot open the data,
      raises the MongoDB feature compatibility version (single member replica sets only),
      runs mongo_seed\b_create-db.js (new collections, indexes, validators) and
      ALTER EXTENSION timescaledb UPDATE
    * restarts the services that were running before the upgrade and stops the rest

  The log goes to <InstallDir>\log\upgrade.log.
  Exit codes: 0 = success, 1 = fatal (PreInstall: do not overwrite files), 2 = finished with warnings.
#>
param(
    [Parameter(Mandatory = $true)][ValidateSet('PreInstall', 'PostInstall')][string]$Phase,
    [Parameter(Mandatory = $true)][string]$InstallDir,
    [string]$BackupDir = '',
    [int]$BundledPgMajor = 0,
    [int]$InstallerPid = 0
)

$ErrorActionPreference = 'Continue'
$InstallDir = $InstallDir.TrimEnd('\')
$PW = Join-Path $InstallDir 'platform-windows'
$LogDir = Join-Path $InstallDir 'log'
$LogFile = Join-Path $LogDir 'upgrade.log'
$StateFile = Join-Path $LogDir 'upgrade-state.json'
$script:Warnings = 0

$DbServices = @('JSON_SCADA_postgresql', 'JSON_SCADA_mongodb', 'JSON_SCADA_mongodb_arb')
$Tier2Services = @('JSON_SCADA_grafana', 'JSON_SCADA_metabase')

New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

function Write-Log([string]$Msg) {
    $line = '{0} [{1}] {2}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $Phase, $Msg
    Write-Host $line
    try { Add-Content -Path $LogFile -Value $line -Encoding UTF8 } catch {}
}

function Write-Warn([string]$Msg) {
    $script:Warnings++
    Write-Log "WARNING: $Msg"
}

function Get-JsService([string]$Name) {
    Get-Service -Name $Name -ErrorAction SilentlyContinue
}

function Get-ServiceStartMode([string]$Name) {
    $s = Get-CimInstance Win32_Service -Filter "Name='$Name'" -ErrorAction SilentlyContinue
    if ($s) { return $s.StartMode }
    return $null
}

$MongoXml = Join-Path $PW 'services\JSON_SCADA_mongodb.xml'

# mongod.exe run by JSON_SCADA_mongodb: <executable> of its WinSW XML, else the Application of a
# not yet migrated NSSM service
function Get-MongodServiceExe {
    if (Test-Path $MongoXml) {
        try { return ([xml](Get-Content -Raw $MongoXml)).service.executable } catch {}
    }
    return (Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Services\JSON_SCADA_mongodb\Parameters' -Name Application -ErrorAction SilentlyContinue).Application
}

# Points the WinSW XML of JSON_SCADA_mongodb at another mongod.exe (WinSW reads it at each service start).
function Set-MongodServiceExe([string]$Exe) {
    try {
        $x = [xml](Get-Content -Raw $MongoXml)
        if ($x.service.executable -eq $Exe) { return }
        $x.service.executable = $Exe
        $x.Save($MongoXml)
    } catch { Write-Warn "Could not set the mongod executable in ${MongoXml}: $($_.Exception.Message)" }
}

# Runs winsw-services.ps1 (service install/migration helper) and copies its output to the log.
function Invoke-WinSWServices([string[]]$Arguments) {
    $helper = Join-Path $PW 'winsw-services.ps1'
    if (-not (Test-Path $helper)) { Write-Warn "$helper not found, services not updated"; return }
    $out = & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $helper @Arguments -InstallDir $InstallDir 2>&1 | Out-String
    $rc = $LASTEXITCODE
    foreach ($l in ($out -split "`r?`n" | Where-Object { $_.Trim() })) { Write-Log "  $l" }
    if ($rc -ne 0) { Write-Warn "winsw-services.ps1 $($Arguments -join ' ') reported errors (see above)" }
}

function Stop-ServiceGroup([string[]]$Names, [int]$TimeoutSec) {
    $list = @($Names | ForEach-Object { Get-JsService $_ } | Where-Object { $_ -and $_.Status -ne 'Stopped' })
    if ($list.Count -eq 0) { return }
    foreach ($s in $list) {
        Write-Log "Stopping $($s.Name)"
        try { Stop-Service -Name $s.Name -Force -NoWait -ErrorAction Stop } catch { Write-Log "  $($_.Exception.Message)" }
    }
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    foreach ($s in $list) {
        $left = [int][Math]::Max(1, ($deadline - (Get-Date)).TotalSeconds)
        try { $s.WaitForStatus('Stopped', [TimeSpan]::FromSeconds($left)) } catch {}
        $s.Refresh()
        if ($s.Status -ne 'Stopped') {
            $svcPid = (Get-CimInstance Win32_Service -Filter "Name='$($s.Name)'" -ErrorAction SilentlyContinue).ProcessId
            if ($svcPid) {
                Write-Log "  $($s.Name) did not stop in time, ending process $svcPid"
                Stop-Process -Id $svcPid -Force -ErrorAction SilentlyContinue
            }
        }
    }
}

function Wait-TcpPort([string]$TargetHost, [int]$Port, [int]$TimeoutSec) {
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        $c = New-Object System.Net.Sockets.TcpClient
        try {
            $ar = $c.BeginConnect($TargetHost, $Port, $null, $null)
            if ($ar.AsyncWaitHandle.WaitOne(1000) -and $c.Connected) { return $true }
        } catch {} finally { $c.Close() }
        Start-Sleep -Seconds 2
    }
    return $false
}

function Get-MongoPort {
    $cfg = Join-Path $PW 'mongodb-conf\mongod.cfg'
    if (Test-Path $cfg) {
        foreach ($l in Get-Content $cfg) {
            if ($l -match '^\s*port\s*:\s*(\d+)') { return [int]$Matches[1] }
        }
    }
    return 27017
}

function Get-JsConfig {
    $f = Join-Path $InstallDir 'conf\json-scada.json'
    $cfg = [pscustomobject]@{
        Uri    = 'mongodb://127.0.0.1:27017/?replicaSet=rs1&directConnection=true'
        DbName = 'json_scada'
    }
    if (Test-Path $f) {
        try {
            $j = Get-Content $f -Raw | ConvertFrom-Json
            if ($j.mongoConnectionString) { $cfg.Uri = $j.mongoConnectionString }
            if ($j.mongoDatabaseName) { $cfg.DbName = $j.mongoDatabaseName }
        } catch { Write-Warn "Could not parse ${f}: $($_.Exception.Message)" }
    }
    return $cfg
}

# Runs JavaScript in mongosh fed through stdin (REPL mode: a failing statement does not stop the rest).
function Invoke-Mongosh([string]$Uri, [string]$Script) {
    $mongosh = Join-Path $PW 'mongodb-runtime\bin\mongosh.exe'
    if (-not (Test-Path $mongosh)) { return $null }
    $tmp = [IO.Path]::GetTempFileName()
    $outFile = "$tmp.out"
    $errFile = "$tmp.err"
    [IO.File]::WriteAllText($tmp, $Script, (New-Object System.Text.UTF8Encoding($false)))
    try {
        $p = Start-Process -FilePath $mongosh -ArgumentList @("`"$Uri`"", '--quiet', '--norc') -NoNewWindow -Wait -PassThru `
            -RedirectStandardInput $tmp -RedirectStandardOutput $outFile -RedirectStandardError $errFile
        return ((Get-Content $outFile -Raw -ErrorAction SilentlyContinue) + (Get-Content $errFile -Raw -ErrorAction SilentlyContinue))
    } finally { Remove-Item $tmp, $outFile, $errFile -Force -ErrorAction SilentlyContinue }
}

function Get-FileVersion([string]$Path) {
    if (-not (Test-Path $Path)) { return $null }
    $vi = (Get-Item $Path).VersionInfo
    return [pscustomobject]@{ Major = $vi.FileMajorPart; Minor = $vi.FileMinorPart; Text = "$($vi.FileMajorPart).$($vi.FileMinorPart)" }
}

function Get-PgDataMajor {
    $f = Join-Path $PW 'postgresql-data\PG_VERSION'
    if (-not (Test-Path $f)) { return 0 }
    $t = (Get-Content $f -TotalCount 1).Trim()
    if ($t -match '^(\d+)') { return [int]$Matches[1] }
    return 0
}

# ------------------------------------------------------------------------------------------------
function Invoke-PreInstall {
    Write-Log "===== Upgrade preparation of $InstallDir ====="

    # 1. Remember running services (merge with a state left by an interrupted upgrade)
    $running = @(Get-Service -Name 'JSON_SCADA_*' -ErrorAction SilentlyContinue |
        Where-Object { $_.Status -eq 'Running' } | ForEach-Object { $_.Name })
    if (Test-Path $StateFile) {
        try {
            $old = Get-Content $StateFile -Raw | ConvertFrom-Json
            $running = @($running + @($old.running) | Where-Object { $_ } | Sort-Object -Unique)
            Write-Log 'Merged the service state left by an interrupted upgrade'
        } catch {}
    }
    @{ running = $running; time = (Get-Date).ToString('o') } | ConvertTo-Json | Set-Content -Path $StateFile -Encoding UTF8
    Write-Log ("Running services: " + ($(if ($running.Count) { $running -join ', ' } else { '(none)' })))

    # 2. Stop services: applications and drivers, then Grafana/Metabase, databases last
    $all = @(Get-Service -Name 'JSON_SCADA_*' -ErrorAction SilentlyContinue | ForEach-Object { $_.Name })
    $apps = @($all | Where-Object { ($DbServices + $Tier2Services) -notcontains $_ })
    Stop-ServiceGroup $apps 60
    Stop-ServiceGroup $Tier2Services 60
    Stop-ServiceGroup $DbServices 180

    # 3. End leftover processes started from the install dir (tools, browser, unmanaged drivers)
    $root = $InstallDir + '\'
    $procs = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
            $_.ExecutablePath -and $_.ExecutablePath.StartsWith($root, [StringComparison]::OrdinalIgnoreCase) -and
            $_.ProcessId -ne $PID -and $_.ProcessId -ne $InstallerPid
        })
    foreach ($p in $procs) {
        Write-Log "Ending process $($p.ProcessId) $($p.ExecutablePath)"
        Stop-Process -Id $p.ProcessId -Force -ErrorAction SilentlyContinue
    }
    if ($procs.Count) { Start-Sleep -Seconds 3 }

    # 4. Back up configuration and scripts the installer overwrites
    if ($BackupDir) {
        try {
            New-Item -ItemType Directory -Force -Path $BackupDir -ErrorAction Stop | Out-Null
            $conf = Join-Path $InstallDir 'conf'
            if (Test-Path $conf) { Copy-Item -Path $conf -Destination (Join-Path $BackupDir 'conf') -Recurse -Force -ErrorAction Stop }
            $pwBak = Join-Path $BackupDir 'platform-windows'
            New-Item -ItemType Directory -Force -Path $pwBak | Out-Null
            Copy-Item -Path (Join-Path $PW '*.bat') -Destination $pwBak -Force -ErrorAction SilentlyContinue
            $svcDir = Join-Path $PW 'services'
            if (Test-Path $svcDir) {
                New-Item -ItemType Directory -Force -Path (Join-Path $pwBak 'services') | Out-Null
                Copy-Item -Path (Join-Path $svcDir '*.xml') -Destination (Join-Path $pwBak 'services') -Force -ErrorAction SilentlyContinue
            }
            $mcfg = Join-Path $PW 'mongodb-conf'
            if (Test-Path $mcfg) { Copy-Item -Path $mcfg -Destination (Join-Path $pwBak 'mongodb-conf') -Recurse -Force -ErrorAction Stop }
            $uiConf = Join-Path $InstallDir 'src\AdminUI\public\conf'
            if (Test-Path $uiConf) { Copy-Item -Path $uiConf -Destination (Join-Path $BackupDir 'AdminUI-public-conf') -Recurse -Force -ErrorAction Stop }
            Write-Log "Configuration backed up to $BackupDir"
        } catch {
            Write-Log "FATAL: backup to $BackupDir failed: $($_.Exception.Message)"
            return 1
        }
    }

    # 5. Keep the mongod binary that opens the current data
    $bin = Join-Path $PW 'mongodb-runtime\bin'
    $mongod = Join-Path $bin 'mongod.exe'
    $prev = Join-Path $bin 'mongod-prev.exe'
    $app = Get-MongodServiceExe
    if ($app -and $app -like '*\mongod-prev.exe') {
        Write-Log 'JSON_SCADA_mongodb still runs mongod-prev.exe (earlier upgrade not completed); keeping that binary'
    } elseif (Test-Path $mongod) {
        try {
            Copy-Item -Path $mongod -Destination $prev -Force -ErrorAction Stop
            Write-Log "Kept current mongod.exe ($((Get-FileVersion $mongod).Text)) as mongod-prev.exe"
        } catch {
            Write-Warn "Could not copy mongod.exe to mongod-prev.exe: $($_.Exception.Message)"
        }
    }

    # 6. PostgreSQL major version change: keep the binaries that match the data
    $dataMajor = Get-PgDataMajor
    if ($dataMajor -gt 0 -and $BundledPgMajor -gt 0 -and $dataMajor -ne $BundledPgMajor) {
        $keep = Join-Path $PW "postgresql-runtime-$dataMajor"
        $cur = Join-Path $PW 'postgresql-runtime'
        if (Test-Path (Join-Path $keep 'bin\pg_ctl.exe')) {
            Write-Log "PostgreSQL $dataMajor runtime already preserved in $keep"
        } else {
            $curVer = Get-FileVersion (Join-Path $cur 'bin\pg_ctl.exe')
            if ($curVer -and $curVer.Major -eq $dataMajor) {
                try {
                    Rename-Item -Path $cur -NewName "postgresql-runtime-$dataMajor" -ErrorAction Stop
                    Write-Log "PostgreSQL data is version $dataMajor, this release bundles $BundledPgMajor; moved the current runtime to $keep"
                } catch {
                    Write-Log "FATAL: could not move $cur to ${keep}: $($_.Exception.Message)"
                    return 1
                }
            } else {
                Write-Warn "PostgreSQL data is version $dataMajor but no PostgreSQL $dataMajor binaries were found"
            }
        }
    }

    Write-Log 'Upgrade preparation finished'
    return 0
}

# ------------------------------------------------------------------------------------------------
# NSSM services of older releases become WinSW services (same settings), then the services of
# platform-windows\service-templates that do not exist yet are registered (existing ones keep their settings).
function Update-Services {
    Invoke-WinSWServices @('-Action', 'Migrate')
    Invoke-WinSWServices @('-Action', 'Install')
    Update-LogViewerConfig
}

# NSSM wrote <name>.log, WinSW writes <logname>.out.log and .err.log: repoint the log.io inputs of the
# WinSW services (the installer never overwrites conf\, so the old paths would stay).
function Update-LogViewerConfig {
    $f = Join-Path $InstallDir 'conf\log.io-file.json'
    $svcDir = Join-Path $PW 'services'
    if (-not (Test-Path $f) -or -not (Test-Path $svcDir)) { return }
    $names = @(Get-ChildItem -Path $svcDir -Filter '*.xml' -File | ForEach-Object {
            try { ([xml](Get-Content -Raw $_.FullName)).service.logname } catch {}
        } | Where-Object { $_ })
    $text = [IO.File]::ReadAllText($f)
    $new = $text
    foreach ($n in $names) {
        $new = [regex]::Replace($new, '(/log/)' + [regex]::Escape($n) + '\.log"', ('$1' + $n + '.out.log"'), 'IgnoreCase')
    }
    if ($new -ne $text) {
        [IO.File]::WriteAllText($f, $new, (New-Object System.Text.UTF8Encoding($false)))
        Write-Log "log.io inputs in $f now read the WinSW <logname>.out.log files"
    }
}

function Set-PostgresRuntime {
    $svc = Get-JsService 'JSON_SCADA_postgresql'
    $dataMajor = Get-PgDataMajor
    if (-not $svc -or $dataMajor -eq 0) { return $true }
    $key = 'HKLM:\SYSTEM\CurrentControlSet\Services\JSON_SCADA_postgresql'
    $img = (Get-ItemProperty $key -Name ImagePath -ErrorAction SilentlyContinue).ImagePath
    if (-not $img) { return $true }

    $folder = 'postgresql-runtime'
    if ($BundledPgMajor -gt 0 -and $dataMajor -ne $BundledPgMajor) {
        if (Test-Path (Join-Path $PW "postgresql-runtime-$dataMajor\bin\pg_ctl.exe")) {
            $folder = "postgresql-runtime-$dataMajor"
        } else {
            Write-Warn "PostgreSQL data is version $dataMajor; PostgreSQL $BundledPgMajor cannot open it and no PostgreSQL $dataMajor runtime is available"
            return $false
        }
    }
    $new = [regex]::Replace($img, '\\postgresql-runtime(-\d+)?\\bin\\', "\$folder\bin\", 'IgnoreCase')
    if ($new -ne $img) {
        Set-ItemProperty -Path $key -Name ImagePath -Value $new
        Write-Log "JSON_SCADA_postgresql now runs from $folder"
    }
    if ($folder -ne 'postgresql-runtime') {
        Write-Warn ("PostgreSQL data is version $dataMajor, kept running with the preserved PostgreSQL $dataMajor runtime " +
            "($PW\$folder). Migrate it to PostgreSQL $BundledPgMajor with pg_upgrade or pg_dumpall/psql when convenient.")
    }
    return ($folder -eq 'postgresql-runtime')
}

function Update-Postgres([bool]$SameMajor) {
    $svc = Get-JsService 'JSON_SCADA_postgresql'
    if (-not $svc -or (Get-ServiceStartMode 'JSON_SCADA_postgresql') -eq 'Disabled') { Write-Log 'JSON_SCADA_postgresql not in use, skipping'; return }
    try { Start-Service 'JSON_SCADA_postgresql' -ErrorAction Stop } catch { Write-Warn "JSON_SCADA_postgresql did not start: $($_.Exception.Message)"; return }
    if (-not (Wait-TcpPort '127.0.0.1' 5432 90)) { Write-Warn 'PostgreSQL is not accepting connections on 127.0.0.1:5432'; return }
    if (-not $SameMajor) { return }
    $psql = Join-Path $PW 'postgresql-runtime\bin\psql.exe'
    if (-not (Test-Path $psql)) { return }
    # must be the first command of the session (-X: no psqlrc); -w: never prompt for a password
    $out = & $psql -X -w -h 127.0.0.1 -U postgres -d json_scada -c 'ALTER EXTENSION timescaledb UPDATE;' 2>&1 | Out-String
    if ($LASTEXITCODE -eq 0) { Write-Log "TimescaleDB extension: $($out.Trim())" }
    else { Write-Warn "ALTER EXTENSION timescaledb UPDATE failed (run it manually as postgres in database json_scada): $($out.Trim())" }
}

function Get-MongoInfo([string]$Uri) {
    $js = @'
var r = { members: null, version: null, fcv: null };
try { r.members = rs.conf().members.length; } catch (e) {
  try { if (!db.adminCommand({ hello: 1 }).setName) r.members = 1; } catch (e2) {}
}
var v = db.version().split("."); r.version = v[0] + "." + v[1];
try { r.fcv = db.adminCommand({ getParameter: 1, featureCompatibilityVersion: 1 }).featureCompatibilityVersion.version; } catch (e) {}
print("JSINFO" + JSON.stringify(r));
'@
    $out = Invoke-Mongosh $Uri $js
    if ($out -and $out -match 'JSINFO(\{.*\})') { return ($Matches[1] | ConvertFrom-Json) }
    return $null
}

function Set-MongoFcv([string]$Uri) {
    $info = Get-MongoInfo $Uri
    if (-not $info) { Write-Warn 'Could not query MongoDB (mongosh with conf\json-scada.json connection string); feature compatibility version not checked'; return $false }
    if ($info.fcv -eq $info.version) { Write-Log "MongoDB $($info.version), feature compatibility version is current"; return $true }
    if ($info.members -ne 1) {
        Write-Warn ("MongoDB replica set has $(if ($info.members) { $info.members } else { 'an unknown number of' }) members, feature compatibility version left at $($info.fcv). " +
            "After all members run MongoDB $($info.version), run: db.adminCommand({setFeatureCompatibilityVersion: '$($info.version)', confirm: true})")
        return $false
    }
    $js = "var r = db.adminCommand({ setFeatureCompatibilityVersion: '$($info.version)', confirm: true }); print('JSFCV' + r.ok);"
    $out = Invoke-Mongosh $Uri $js
    if ($out -match 'JSFCV1') { Write-Log "MongoDB feature compatibility version raised from $($info.fcv) to $($info.version)"; return $true }
    Write-Warn "Setting the MongoDB feature compatibility version to $($info.version) failed: $out"
    return $false
}

function Start-Mongo([string]$Exe, [bool]$SetExe, [int]$Port) {
    if ($SetExe) { Set-MongodServiceExe $Exe }
    try { Start-Service 'JSON_SCADA_mongodb' -ErrorAction Stop } catch {}
    return (Wait-TcpPort '127.0.0.1' $Port 180)
}

function Update-Mongo {
    $svc = Get-JsService 'JSON_SCADA_mongodb'
    if (-not $svc -or (Get-ServiceStartMode 'JSON_SCADA_mongodb') -eq 'Disabled') { Write-Log 'JSON_SCADA_mongodb not in use, skipping'; return }
    $cfg = Get-JsConfig
    $port = Get-MongoPort
    $bin = Join-Path $PW 'mongodb-runtime\bin'
    $mongod = Join-Path $bin 'mongod.exe'
    $prev = Join-Path $bin 'mongod-prev.exe'
    $app = Get-MongodServiceExe
    # only switch binaries for a WinSW service running the bundled mongod (custom binaries stay untouched)
    $ours = [bool]($app -and (Test-Path $MongoXml) -and ($app -like '*\mongodb-runtime\bin\mongod.exe' -or $app -like '*\mongodb-runtime\bin\mongod-prev.exe'))

    $newVer = Get-FileVersion $mongod
    $prevVer = Get-FileVersion $prev
    if (-not $newVer) { Write-Warn "$mongod not found"; return }
    Write-Log ("MongoDB binaries: new {0}, previous {1}" -f $newVer.Text, $(if ($prevVer) { $prevVer.Text } else { '-' }))

    $ok = Start-Mongo $mongod $ours $port
    if (-not $ok -and $ours -and $prevVer -and $prevVer.Text -ne $newVer.Text) {
        Write-Log "MongoDB $($newVer.Text) did not open the data, trying the previous binary $($prevVer.Text)"
        Stop-ServiceGroup @('JSON_SCADA_mongodb') 60
        if (Start-Mongo $prev $ours $port) {
            if (Set-MongoFcv $cfg.Uri) {
                Stop-ServiceGroup @('JSON_SCADA_mongodb') 120
                $ok = Start-Mongo $mongod $ours $port
                if (-not $ok) {
                    Stop-ServiceGroup @('JSON_SCADA_mongodb') 60
                    [void](Start-Mongo $prev $ours $port)
                }
            }
            if (-not $ok) {
                Write-Warn ("MongoDB still runs the previous binary (mongod-prev.exe, $($prevVer.Text)). Its data needs an intermediate " +
                    "MongoDB version before $($newVer.Text); see https://www.mongodb.com/docs/manual/release-notes/")
                return
            }
        } else {
            Write-Warn "MongoDB did not start with either binary; see the mongod log configured in $PW\mongodb-conf\mongod.cfg"
            return
        }
    } elseif (-not $ok) {
        Write-Warn "MongoDB is not accepting connections on 127.0.0.1:$port; see the mongod log configured in $PW\mongodb-conf\mongod.cfg"
        return
    }

    Write-Log "MongoDB $($newVer.Text) is up"
    $fcvOk = Set-MongoFcv $cfg.Uri

    # new collections and indexes, refreshed validators (b_create-db.js is idempotent in stdin mode)
    $seed = Join-Path $InstallDir 'mongo_seed\b_create-db.js'
    if (Test-Path $seed) {
        $js = "db = db.getSiblingDB('$($cfg.DbName)');`n" + [IO.File]::ReadAllText($seed)
        $out = Invoke-Mongosh $cfg.Uri $js
        if ($null -eq $out) { Write-Warn 'mongosh.exe not found, database schema not updated' }
        else {
            $errs = @($out -split "`r?`n" | Where-Object { $_ -match 'Error' -and $_ -notmatch 'NamespaceExists|already exists' })
            if ($errs.Count) { Write-Warn ("Schema update reported: " + ($errs -join ' | ')) }
            else { Write-Log "Database schema updated ($($cfg.DbName))" }
        }
    }

    if ($fcvOk -and (Test-Path $prev)) {
        Remove-Item $prev -Force -ErrorAction SilentlyContinue
        Write-Log 'Removed mongod-prev.exe (no longer needed)'
    }
}

function Restore-Services {
    $running = @()
    if (Test-Path $StateFile) {
        try { $running = @((Get-Content $StateFile -Raw | ConvertFrom-Json).running | Where-Object { $_ }) } catch {}
    }
    $order = @($DbServices + $Tier2Services)
    $start = @($order | Where-Object { $running -contains $_ }) + @($running | Where-Object { $order -notcontains $_ })
    foreach ($name in $start) {
        $s = Get-JsService $name
        if (-not $s) { Write-Warn "Service $name was running before the upgrade but no longer exists"; continue }
        if ($s.Status -eq 'Running') { continue }
        Write-Log "Starting $name"
        try { Start-Service $name -ErrorAction Stop } catch { Write-Warn "$name did not start: $($_.Exception.Message)" }
    }
    # databases started only for the upgrade go back to stopped
    foreach ($name in $DbServices) {
        if ($running -notcontains $name) {
            $s = Get-JsService $name
            if ($s -and $s.Status -ne 'Stopped') { Stop-ServiceGroup @($name) 180 }
        }
    }
    Remove-Item $StateFile -Force -ErrorAction SilentlyContinue
}

function Invoke-PostInstall {
    Write-Log "===== Upgrade completion of $InstallDir ====="
    $pgSame = Set-PostgresRuntime
    Update-Services
    Update-Postgres $pgSame
    Update-Mongo
    Restore-Services
    if ($script:Warnings) { Write-Log "Upgrade finished with $($script:Warnings) warning(s)"; return 2 }
    Write-Log 'Upgrade finished'
    return 0
}

if ($Phase -eq 'PreInstall') { $rc = Invoke-PreInstall } else { $rc = Invoke-PostInstall }
exit $rc
