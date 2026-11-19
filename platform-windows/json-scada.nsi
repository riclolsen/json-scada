; json-scada.nsi
; {json:scada} installer script
; Copyright 2020-2026 - Ricardo L. Olsen

; NSIS (Nullsoft Scriptable Install System) - http://nsis.sourceforge.net/Main_Page

SetCompress Auto
SetCompressor lzma ; do not use /SOLID, it fails to build!
SetCompressorDictSize 128
SetDatablockOptimize On

Unicode True
; RequestExecutionLevel user
RequestExecutionLevel admin

;!include "MUI2.nsh"
!include "TextFunc.nsh"
!include "WordFunc.nsh"
!include "x64.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"

;--------------------------------

!define VERSION "v.0.72"
!define VERSION_ "0.72.0.0"

; PostgreSQL major version bundled in postgresql-runtime (PGBIN_1), read from pg_ctl.exe at compile time.
; Upgrades over data of another major keep the old runtime (see upgrade.ps1).
!getdllversion "postgresql-runtime\bin\pg_ctl.exe" PGBIN_

; Upgrade state, set in .onInit
Var IS_UPGRADE   ; 1 = previous installation files, registry entry or databases found
Var HAS_DATA     ; 1 = MongoDB or PostgreSQL data found: never run initial_setup.bat
Var PREV_VERSION ; version recorded by the previous installer ("" for 0.71 and older)
Var BACKUP_DIR   ; per-upgrade backup of conf and scripts

Function .onInit
  System::Call 'kernel32::CreateMutex(p 0, i 0, t "MutexJsonScadaInstall") p .r1 ?e'
  Pop $0
  ${If} $0 = 183
    MessageBox MB_OK|MB_ICONEXCLAMATION "Installer already executing!" /SD IDOK
    Abort
  ${EndIf}

  ${IfNot} ${RunningX64}
    MessageBox MB_OK|MB_ICONSTOP "JSON-SCADA requires a 64-bit Windows." /SD IDOK
    Abort
  ${EndIf}

  SetRegView 64
  StrCpy $IS_UPGRADE 0
  StrCpy $HAS_DATA 0
  ReadRegStr $PREV_VERSION HKLM "Software\JSON_SCADA" "Version"
  ReadRegStr $0 HKLM "Software\JSON_SCADA" "Install_Dir"
  ${If} $0 != ""
  ${OrIf} ${FileExists} "$INSTDIR\platform-windows\*.*"
    StrCpy $IS_UPGRADE 1
  ${EndIf}
  ${If} ${FileExists} "$INSTDIR\platform-windows\postgresql-data\PG_VERSION"
  ${OrIf} ${FileExists} "$INSTDIR\platform-windows\mongodb-data\WiredTiger"
    StrCpy $IS_UPGRADE 1
    StrCpy $HAS_DATA 1
  ${EndIf}

  ${If} $IS_UPGRADE == 1
    ${If} $PREV_VERSION == ""
      StrCpy $1 "0.71 or older"
    ${Else}
      StrCpy $1 $PREV_VERSION
      ${VersionCompare} $PREV_VERSION ${VERSION_} $2
      ${If} $2 == 1
        MessageBox MB_YESNO|MB_ICONEXCLAMATION|MB_DEFBUTTON2 "A newer JSON-SCADA ($PREV_VERSION) is installed in $INSTDIR.$\r$\nDowngrades are not supported and may leave the databases unusable.$\r$\n$\r$\nContinue anyway?" /SD IDNO IDYES +2
        Abort
      ${EndIf}
    ${EndIf}
    MessageBox MB_OKCANCEL|MB_ICONINFORMATION "An existing JSON-SCADA installation (version $1) was found in $INSTDIR and will be upgraded to ${VERSION}.$\r$\n$\r$\nDatabases, configuration (conf folder), displays and service settings are kept. All JSON-SCADA services are stopped during the upgrade and the ones running now are restarted at the end. Configuration and scripts are backed up to $INSTDIR\backup first." /SD IDOK IDOK +2
    Abort
  ${EndIf}
FunctionEnd

;!macro CreateInternetShortcut FILEPATH URL
;WriteINIStr "${FILEPATH}" "InternetShortcut" "URL" "${URL}"
;!macroend
;--------------------------------

!ifdef HAVE_UPX
!packhdr tmp.dat "upx\upx -9 tmp.dat"
!endif

!ifdef NOCOMPRESS
SetCompress off
!endif

;--------------------------------

!define /date DATEBAR "%d/%m/%Y"
Name "JSON-SCADA"
Caption "{json:scada} Installer ${VERSION} ${DATEBAR}"
Icon "..\src\AdminUI\public\images\j-s-256.ico"

!define /date DATE "%d_%m_%Y"
OutFile "installer-release\json-scada_setup_${VERSION}.exe"

VIProductVersion ${VERSION_}
VIAddVersionKey ProductName "JSON SCADA"
VIAddVersionKey Comments "SCADA IIoT Software"
VIAddVersionKey CompanyName "Ricardo Olsen"
VIAddVersionKey LegalCopyright "Copyright 2020-Present Ricardo L. Olsen"
VIAddVersionKey FileDescription "JSON SCADA Installer"
VIAddVersionKey FileVersion ${VERSION}
VIAddVersionKey ProductVersion ${VERSION}
VIAddVersionKey InternalName "JSON SCADA Installer"
VIAddVersionKey LegalTrademarks "{json:scada}"
VIAddVersionKey OriginalFilename "json-scada_setup_${VERSION}.exe"

SetDateSave on
SetDatablockOptimize on
CRCCheck on
SilentInstall normal
;BGGradient 000000 000080 FFFFFF
BGGradient off
InstallColors FF8080 000030
XPStyle on

InstallDir "c:\json-scada"
InstallDirRegKey HKLM "Software\JSON_SCADA" "Install_Dir"

CheckBitmap "${NSISDIR}\Contrib\Graphics\Checks\classic-cross.bmp"

LicenseText "JSON-SCADA Release Notes"
LicenseData "release_notes.txt"

;--------------------------------

Page license
;Page components
;Page directory
Page instfiles

UninstPage uninstConfirm
UninstPage instfiles

;--------------------------------

AutoCloseWindow false
ShowInstDetails show

;--------------------------------

Section "" ; empty string makes it hidden, so would starting with -

${DisableX64FSRedirection}

SetRegView 64

; Upgrade preparation (upgrade.ps1 -Phase PreInstall): stop every JSON_SCADA_* service (databases last)
; and leftover processes from $INSTDIR, back up conf and scripts, keep database binaries the existing
; data still needs. Runs from $PLUGINSDIR because older installations do not have the script.
  InitPluginsDir
  File "/oname=$PLUGINSDIR\upgrade.ps1" "upgrade.ps1"
  ${If} $IS_UPGRADE == 1
    ${GetTime} "" "L" $0 $1 $2 $3 $4 $5 $6
    StrCpy $BACKUP_DIR "$INSTDIR\backup\upgrade_$2$1$0_$4$5$6"
    System::Call 'kernel32::GetCurrentProcessId() i .r7'
    DetailPrint "Preparing the upgrade: stopping JSON-SCADA services, backing up configuration..."
    nsExec::ExecToLog 'powershell -NoProfile -ExecutionPolicy Bypass -File "$PLUGINSDIR\upgrade.ps1" -Phase PreInstall -InstallDir "$INSTDIR" -BackupDir "$BACKUP_DIR" -BundledPgMajor ${PGBIN_1} -InstallerPid $7'
    Pop $0
    ${If} $0 != 0
      MessageBox MB_YESNO|MB_ICONSTOP "The upgrade preparation failed (result: $0, see $INSTDIR\log\upgrade.log).$\r$\nContinuing may overwrite files that are still needed.$\r$\n$\r$\nAbort the installation (nothing was replaced yet)?" /SD IDYES IDNO +2
      Abort
    ${EndIf}
  ${EndIf}
  SetOverwrite on

  var /GLOBAL NAVWINCMD
  var /GLOBAL NAVDATDIR
  var /GLOBAL NAVPREOPT
  var /GLOBAL NAVPOSOPT
  var /GLOBAL NAVINDEX
  var /GLOBAL NAVVISABO
  var /GLOBAL NAVVISEVE
  var /GLOBAL NAVVISHEV
  var /GLOBAL NAVVISTAB
  var /GLOBAL NAVVISANO
  var /GLOBAL NAVVISTEL
  var /GLOBAL NAVGRAFAN
  var /GLOBAL NAVMETABA
  var /GLOBAL HTTPSRV
  var /GLOBAL HTTPSSRV
    
  # PROTOCOL://IP:PORT  
  StrCpy $HTTPSRV   "http://127.0.0.1" 
  StrCpy $HTTPSSRV  "https://127.0.0.1" 
  StrCpy $NAVWINCMD "platform-windows\browser-runtime\chrome.exe"
  StrCpy $NAVDATDIR "--user-data-dir=$INSTDIR\platform-windows\browser-data"
  StrCpy $NAVPREOPT ""
  StrCpy $NAVPOSOPT "--disable-popup-blocking --no-proxy-server --bwsi"
  StrCpy $NAVINDEX  "/"
  StrCpy $NAVVISABO "/#/about"
  StrCpy $NAVVISEVE "/#/events-viewer"
  StrCpy $NAVVISHEV "/#/events-viewer?modo=4"
  StrCpy $NAVVISTAB "/#/tabular-viewer"
  StrCpy $NAVVISANO "/#/alarms-viewer"
  StrCpy $NAVVISTEL "/#/display-viewer"
  StrCpy $NAVGRAFAN "/grafana"
  StrCpy $NAVMETABA "/metabase"

  ; write reg info
  WriteRegStr HKLM SOFTWARE\JSON_SCADA "Install_Dir" "$INSTDIR"

  ; write uninstall strings
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\JSON_SCADA" "DisplayName" "JSON SCADA (remove only)"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\JSON_SCADA" "UninstallString" '"$INSTDIR\bt-uninst.exe"'
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\JSON_SCADA" "Publisher" "Ricardo Olsen"
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\JSON_SCADA" "NoModify" 1
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\JSON_SCADA" "NoRepair" 1

  ; erases all of the old chromium
  ;RMDir /r "$INSTDIR\browser-runtime" 
  ;RMDir /r "$INSTDIR\browser-data" 

  ; erases all of the old Inkscape but the share dir
  ; New files will replace old.
  RMDir /r "$INSTDIR\platform-windows\inkscape-runtime\doc" 
  RMDir /r "$INSTDIR\platform-windows\inkscape-runtime\etc" 
  RMDir /r "$INSTDIR\platform-windows\inkscape-runtime\lib"  
  Delete "$INSTDIR\platform-windows\inkscape-runtime\*.*"  
  
  CreateDirectory "$INSTDIR\.agents\skills"
  CreateDirectory "$INSTDIR\bin"
  CreateDirectory "$INSTDIR\conf"
  CreateDirectory "$INSTDIR\docs"
  CreateDirectory "$INSTDIR\log"
  CreateDirectory "$INSTDIR\mongo_seed"
  CreateDirectory "$INSTDIR\sql"
  CreateDirectory "$INSTDIR\src"
; CreateDirectory "$INSTDIR\Opc.Ua.CertificateGenerator"
  CreateDirectory "$INSTDIR\platform-windows"
  CreateDirectory "$INSTDIR\platform-windows\jdk-runtime"
  CreateDirectory "$INSTDIR\platform-windows\metabase-runtime"
  CreateDirectory "$INSTDIR\platform-windows\grafana-runtime"
  ;CreateDirectory "$INSTDIR\platform-windows\browser-runtime"
  ;CreateDirectory "$INSTDIR\platform-windows\browser-data"
  CreateDirectory "$INSTDIR\platform-windows\mongodb-compass-runtime"
  CreateDirectory "$INSTDIR\platform-windows\mongodb-data"
  CreateDirectory "$INSTDIR\platform-windows\mongodb-conf"
  CreateDirectory "$INSTDIR\platform-windows\mongodb-runtime"
  CreateDirectory "$INSTDIR\platform-windows\nodejs-runtime"
  CreateDirectory "$INSTDIR\platform-windows\python-runtime"
  CreateDirectory "$INSTDIR\platform-windows\inkscape-runtime"
  CreateDirectory "$INSTDIR\platform-windows\postgresql-data"
  CreateDirectory "$INSTDIR\platform-windows\postgresql-runtime"
  CreateDirectory "$INSTDIR\platform-windows\nginx_php-runtime"
  CreateDirectory "$INSTDIR\platform-windows\nginx_php-runtime\temp"
  CreateDirectory "$INSTDIR\platform-windows\nginx_php-runtime\logs"
  CreateDirectory "$INSTDIR\platform-windows\telegraf-runtime"
  
  ; This is to try to avoid this Postgresql error:
  ; https://edwin.baculsoft.com/2014/05/fixing-postgresql-error-initdb-could-not-change-permissions-of-directory-permission-denied/
  ; (S-1-5-32-545)=Users (S-1-1-0)=Everyone
  ; https://docs.microsoft.com/pt-br/troubleshoot/windows-server/identity/security-identifiers-in-windows
  AccessControl::GrantOnFile "$INSTDIR\platform-windows\postgresql-data" "(S-1-5-32-545)" "FullAccess"
  AccessControl::GrantOnFile "$INSTDIR\platform-windows\postgresql-data" "(S-1-1-0)" "FullAccess"

  SetOutPath $INSTDIR

  File /a ".\release_notes.txt"
  File /a "..\LICENSE"

  SetOutPath $INSTDIR\.agents\skills
  File /a /r "..\.agents\skills\*.*"
  
  SetOutPath $INSTDIR\bin
  File /a /r "..\bin\*.*"
  File /a "..\platform-windows\nssm.exe"

  ; preserve previous start_services.bat (all scripts are also in $BACKUP_DIR on upgrades)
  ${If} ${FileExists} "$INSTDIR\platform-windows\start_services.bat"
    Delete "$INSTDIR\platform-windows\start_services.bat.bak"
    Rename "$INSTDIR\platform-windows\start_services.bat" "$INSTDIR\platform-windows\start_services.bat.bak"
  ${EndIf}

  SetOutPath $INSTDIR\platform-windows
  File /a "..\platform-windows\*.bat"
  File /a "..\platform-windows\*.ps1"
  File /a "..\platform-windows\nssm.exe"
  File /a "..\platform-windows\ffmpeg.exe"
  File /a "..\platform-windows\sounder.exe"
  File /a "..\platform-windows\OpcWatch.exe"
  File /a "..\platform-windows\vc_redist.x64.exe"
  ;File /a "..\platform-windows\aspnetcore-runtime-8.0.23-win-x64.exe"
  File /a "..\platform-windows\dotnet-runtime-8.0.31-win-x64.exe"
  ;File /a "..\platform-windows\dotnet-runtime-10.0.12-win-x64.exe"
  File /a "..\platform-windows\OPC Core Components Redistributable (x64) 3.00.108.msi"
  ;File /a "..\platform-windows\gbda_aut.dll"
  ;File /a "..\platform-windows\gbhda_aw.dll"
  ;ExecWait `regsvr32 gbda_aut.dll`
  ;ExecWait `regsvr32 gbhda_aw.dll`

  ; Visual C redist: needed for timescaledb and PHP
  ;ReadRegStr $0 HKLM "SOFTWARE\Microsoft\VisualStudio\14.0\VC\Runtimes\x86" "Major"
  ;; Check for version 14
  ;${If} $0 = "14"
  ;  DetailPrint "The installed VC redist version is usable"
  ;${Else}
  ;  DetailPrint "Must install VC 14 redist"
  ;  ExecWait '"$INSTDIR\platform-windows\vc_redist.x64.exe" /q'
  ;${EndIf}
  Sleep 1000
  ExecWait '"$INSTDIR\platform-windows\vc_redist.x64.exe" /install /passive /quiet'
  Sleep 1000
  ;ExecWait '"$INSTDIR\platform-windows\aspnetcore-runtime-8.0.23-win-x64.exe" /install /passive /quiet'
  ExecWait '"$INSTDIR\platform-windows\dotnet-runtime-8.0.31-win-x64.exe" /install /passive /quiet'
  ;ExecWait '"$INSTDIR\platform-windows\dotnet-runtime-10.0.12-win-x64.exe" /install /passive /quiet'
  Sleep 1000
  ExecWait 'msiexec /i "$INSTDIR\platform-windows\OPC Core Components Redistributable (x64) 3.00.108.msi" /qn'
  Sleep 1000
  ; WMIC was previously installed for postgresql injection batch files but WMIC is deprecated.
  ; We use PowerShell helpers (shipped with the installer) instead of WMIC. No DISM install attempted.
  Sleep 1000
  
  ; npm and corepack are replaced as a whole: stale files from an older npm break it.
  ; Other global packages in nodejs-runtime\node_modules are kept.
  RMDir /r "$INSTDIR\platform-windows\nodejs-runtime\node_modules\npm"
  RMDir /r "$INSTDIR\platform-windows\nodejs-runtime\node_modules\corepack"
  SetOutPath $INSTDIR\platform-windows\nodejs-runtime
  File /a /r "..\platform-windows\nodejs-runtime\*.*"

  ; Python with the CIM-EMS package and its pinned dependencies installed by
  ; build.bat (src\cim-ems\install.bat). Replaced as a whole so removed packages do not linger.
  RMDir /r "$INSTDIR\platform-windows\python-runtime"
  SetOutPath $INSTDIR\platform-windows\python-runtime
  File /a /r /x tests /x __pycache__ "..\platform-windows\python-runtime\*.*"

  SetOutPath $INSTDIR\platform-windows\telegraf-runtime
  File /a "..\platform-windows\telegraf-runtime\telegraf.exe"
  File /a "..\platform-windows\telegraf-runtime\*.conf"

  SetOutPath $INSTDIR\docs
  File /a /r "..\docs\*.*"
  
  SetOutPath $INSTDIR\platform-windows\jdk-runtime
  File /a /r /x jmods "..\platform-windows\jdk-runtime\*.*"

  SetOutPath $INSTDIR\platform-windows\metabase-runtime
  File /a "..\platform-windows\metabase-runtime\metabase.jar"

  SetOutPath $INSTDIR\platform-windows\grafana-runtime
  File /a /r /x *.map /x "docs" "..\platform-windows\grafana-runtime\*.*"

  SetOutPath $INSTDIR\mongo_seed_demo
  File /a /r "..\demo-docker\mongo_seed\files\*.*"

  SetOutPath $INSTDIR\mongo_seed_demo
  File /a /r "..\mongo_seed\a_rs-init.js"

  SetOutPath $INSTDIR\mongo_seed
  File /a /r "..\mongo_seed\*.*"

  SetOutPath $INSTDIR\platform-windows\mongodb-compass-runtime
  File /a /r "..\platform-windows\mongodb-compass-runtime\*.*"

  SetOutPath $INSTDIR\platform-windows\mongodb-runtime
  File /a /r /x *.pdb "..\platform-windows\mongodb-runtime\*.*"

  ; mongod.cfg is the live configuration: never overwrite it (the samples are refreshed)
  SetOutPath $INSTDIR\platform-windows\mongodb-conf
  File /a /r /x mongod.cfg "..\platform-windows\mongodb-conf\*.*"
  SetOverwrite off
  File /a "..\platform-windows\mongodb-conf\mongod.cfg"
  SetOverwrite on

  SetOutPath $INSTDIR\platform-windows\postgresql-runtime
  File /a /r /x doc /x include /x StackBuilder /x *.pdb /x "pgAdmin 4" "..\platform-windows\postgresql-runtime\*.*"

  SetOutPath $INSTDIR\platform-windows\dbx-runtime
  File /a /r "..\platform-windows\dbx-runtime\*.*"

  SetOutPath $INSTDIR\sql
  File /a "..\sql\create_tables.sql"
  File /a "..\sql\metabaseappdb.sql"
  File /a "..\sql\grafanaappdb.sql"
  File /a "..\sql\*.md"

  SetOutPath $INSTDIR\platform-windows\nginx_php-runtime
  File /r /x *.log "..\platform-windows\nginx_php-runtime\*.*" 

  SetOutPath $INSTDIR\conf-templates
  File /a "..\conf-templates\*.*"

  ; preserve previous screen_list
  ; Rename $INSTDIR\src\htdocs\svg\screen_list.js $INSTDIR\src\htdocs\svg\screen_list.js.bak

  ; Shipped node_modules are replaced as a whole: packages left over from the previous version can
  ; shadow the new dependency tree. node_modules the installer does not ship (user installs) are kept.
  !macro CleanNodeModules SUBDIR
    RMDir /r "$INSTDIR\src\${SUBDIR}\node_modules"
  !macroend
  !insertmacro CleanNodeModules "camera-onvif"
  !insertmacro CleanNodeModules "demo_simul"
  !insertmacro CleanNodeModules "mongofw"
  !insertmacro CleanNodeModules "mongowr"
  !insertmacro CleanNodeModules "grafana_alert2event"
  !insertmacro CleanNodeModules "alarm_beep"
  !insertmacro CleanNodeModules "backup-mongo"
  !insertmacro CleanNodeModules "oshmi2json"
  !insertmacro CleanNodeModules "oshmi_sync"
  !insertmacro CleanNodeModules "cs_data_processor"
  !insertmacro CleanNodeModules "mcp-json-scada-db"
  !insertmacro CleanNodeModules "config_server_for_excel"
  !insertmacro CleanNodeModules "server_realtime_auth"
  !insertmacro CleanNodeModules "updateUser"
  !insertmacro CleanNodeModules "telegraf-listener"
  !insertmacro CleanNodeModules "process_pg_sql"
  !insertmacro CleanNodeModules "mqtt-sparkplug"
  !insertmacro CleanNodeModules "OPC-UA-Server"
  !insertmacro CleanNodeModules "n8n-client"
  !insertmacro CleanNodeModules "node-red-driver"
  !insertmacro CleanNodeModules "carbone-reports"
  !insertmacro CleanNodeModules "log-io\inputs\file"
  !insertmacro CleanNodeModules "log-io\server"

  SetOutPath $INSTDIR\src\certificate-creator
  File /a "..\src\certificate-creator\client.conf"
  File /a "..\src\certificate-creator\server.conf"
  File /a "..\src\certificate-creator\create_certs.sh"

  SetOutPath $INSTDIR\svg\.agents\skills\svg-scada
  File /a "..\svg\.agents\skills\svg-scada\SKILL.md"
  SetOutPath $INSTDIR\src\AdminUI
  File /a /x *_plan.md  "..\src\AdminUI\*.*"
  SetOutPath $INSTDIR\src\AdminUI\src
  File /a /r "..\src\AdminUI\src\*.*"
  SetOutPath $INSTDIR\src\AdminUI\dist
  File /a /r   "..\src\AdminUI\dist\*.*"
  File /a    ".\release_notes.txt"

  SetOutPath $INSTDIR\src\svgedit\dist\editor
  File /a /r "..\src\svgedit\dist\editor\*.*"
  
  SetOutPath $INSTDIR\src\camera-onvif
  File /a /r /x ffmpeg.exe /x *.bak /x snapshot.jpg	 "..\src\camera-onvif\*.*"
  File /a "..\platform-windows\ffmpeg.exe"

  SetOutPath $INSTDIR\src\demo_simul
  File /a /r "..\src\demo_simul\*.*"

  SetOutPath $INSTDIR\src\mongofw
  File /a /r "..\src\mongofw\*.*"

  SetOutPath $INSTDIR\src\mongowr
  File /a /r "..\src\mongowr\*.*"

  SetOutPath $INSTDIR\src\grafana_alert2event
  File /a /r "..\src\grafana_alert2event\*.*"

  SetOutPath $INSTDIR\src\alarm_beep
  File /a /r "..\src\alarm_beep\*.*"

  SetOutPath $INSTDIR\src\backup-mongo
  File /a /r "..\src\backup-mongo\*.*"

  SetOutPath $INSTDIR\src\oshmi2json
  File /a /r "..\src\oshmi2json\*.*"

  SetOutPath $INSTDIR\src\oshmi_sync
  File /a /r "..\src\oshmi_sync\*.*"

  SetOutPath $INSTDIR\src\cs_data_processor
  File /a /r "..\src\cs_data_processor\*.*"

  SetOutPath $INSTDIR\src\cs_custom_processor
  File /a "..\src\cs_custom_processor\*.json"
  File /a "..\src\cs_custom_processor\README.md"
  SetOutPath $INSTDIR\src\cs_custom_processor\src
  File /a /r "..\src\cs_custom_processor\src\*.*"
  SetOutPath $INSTDIR\src\cs_custom_processor\dist
  File /a /r "..\src\cs_custom_processor\dist\*.*"
  ;SetOutPath $INSTDIR\src\cs_custom_processor\node_modules
  ;File /a /r "..\src\cs_custom_processor\node_modules\*.*"

  SetOutPath $INSTDIR\src\mcp-json-scada-db
  File /a /r "..\src\mcp-json-scada-db\*.json"
  File /a /r "..\src\mcp-json-scada-db\*.md"
  SetOutPath $INSTDIR\src\mcp-json-scada-db\src
  File /a /r "..\src\mcp-json-scada-db\src\*.*"
  SetOutPath $INSTDIR\src\mcp-json-scada-db\dist
  File /a /r "..\src\mcp-json-scada-db\dist\*.*"
  SetOutPath $INSTDIR\src\mcp-json-scada-db\node_modules
  File /a /r "..\src\mcp-json-scada-db\node_modules\*.*"
  SetOutPath $INSTDIR\src\mcp-json-scada-db\examples
  File /a /r "..\src\mcp-json-scada-db\examples\*.*"

  SetOutPath $INSTDIR\src\config_server_for_excel
  File /a "..\src\config_server_for_excel\README.md"
  File /a "..\src\config_server_for_excel\*.js"
  File /a "..\src\config_server_for_excel\*.json"
  SetOutPath $INSTDIR\src\config_server_for_excel\node_modules
  File /a /r "..\src\config_Server_for_excel\node_modules\*.*"

  SetOutPath $INSTDIR\src\server_realtime_auth
  File /a /r "..\src\server_realtime_auth\*.*"

  SetOutPath $INSTDIR\src\updateUser
  File /a /r "..\src\updateUser\*.*"

  SetOutPath $INSTDIR\src\telegraf-listener
  File /a /r "..\src\telegraf-listener\*.*"

  SetOutPath $INSTDIR\src\process_pg_sql
  File /a /r "..\src\process_pg_sql\*.*"

  SetOutPath $INSTDIR\src\cim-ems
  File /a /r /x .venv /x __pycache__ /x spike /x data /x build /x *.egg-info /x *_plan.md "..\src\cim-ems\*.*"

  SetOutPath $INSTDIR\src\mqtt-sparkplug
  File /a "..\src\mqtt-sparkplug\*.js"
  File /a "..\src\mqtt-sparkplug\*.json"
  File /a "..\src\mqtt-sparkplug\*.md"
  SetOutPath $INSTDIR\src\mqtt-sparkplug\node_modules
  File /a /r "..\src\mqtt-sparkplug\node_modules\*.*"

  SetOutPath $INSTDIR\src\OPC-UA-Server
  File /a "..\src\OPC-UA-Server\*.js"
  File /a "..\src\OPC-UA-Server\*.json"
  File /a /x *_plan.md "..\src\OPC-UA-Server\*.md"
  SetOutPath $INSTDIR\src\OPC-UA-Server\node_modules
  File /a /r "..\src\OPC-UA-Server\node_modules\*.*"

  SetOutPath $INSTDIR\src\n8n-client
  File /a "..\src\n8n-client\*.js"
  File /a "..\src\n8n-client\*.json"
  File /a "..\src\n8n-client\*.md"
  SetOutPath $INSTDIR\src\n8n-client\node_modules
  File /a /r "..\src\n8n-client\node_modules\*.*"

  SetOutPath $INSTDIR\src\node-red-driver
  File /a "..\src\node-red-driver\*.json"
  File /a "..\src\node-red-driver\*.md"
  SetOutPath $INSTDIR\src\node-red-driver\node_modules
  File /a /r "..\src\node-red-driver\node_modules\*.*"
  SetOutPath $INSTDIR\src\node-red-driver\src
  File /a /r "..\src\node-red-driver\src\*.*"
  SetOutPath $INSTDIR\src\node-red-driver\dist
  File /a /r "..\src\node-red-driver\dist\*.*"
  
  SetOutPath $INSTDIR\src\carbone-reports
  File /a /r "..\src\carbone-reports\*.*"

  SetOutPath $INSTDIR\src\log-io
  File /a "..\src\log-io\*.*"
  SetOutPath $INSTDIR\src\log-io\inputs\file
  File /a /r "..\src\log-io\inputs\file\*.*"
  SetOutPath $INSTDIR\src\log-io\server
  File /a /r "..\src\log-io\server\*.*"
  SetOutPath $INSTDIR\src\log-io\ui
  File /a "..\src\log-io\ui\*.*"
  SetOutPath $INSTDIR\src\log-io\ui\src
  File /a /r "..\src\log-io\ui\src\*.*"
  SetOutPath $INSTDIR\src\log-io\ui\public
  File /a /r "..\src\log-io\ui\public\*.*"
  SetOutPath $INSTDIR\src\log-io\ui\build
  File /a /r "..\src\log-io\ui\build\*.*"

  SetOutPath $INSTDIR\src\custom-developments
  File /a    "..\src\custom-developments\README.md"
  SetOutPath $INSTDIR\src\custom-developments\basic_bargraph
  File /a /r /x node_modules "..\src\custom-developments\basic_bargraph\*.*"
  SetOutPath $INSTDIR\src\custom-developments\advanced_dashboard
  File /a /r /x node_modules "..\src\custom-developments\advanced_dashboard\*.*"
  SetOutPath $INSTDIR\src\custom-developments\transformer_with_command
  File /a /r /x node_modules "..\src\custom-developments\transformer_with_command\*.*"

  ;SetOutPath $INSTDIR\platform-windows\browser-runtime
  ;File /a /r "..\platform-windows\browser-runtime\*.*"

  ;SetOutPath $INSTDIR\platform-windows\browser-data
  ;File /a /r "..\platform-windows\browser-data\*.*"

  ; nodered
  SetOutPath $INSTDIR\platform-windows\nodered-runtime
  File /a /r "..\platform-windows\nodered-runtime\*.*"

  ; ua-edge-translator
  ;SetOutPath $INSTDIR\platform-windows\ua-edge-translator-runtime
  ;File /a /r "..\platform-windows\ua-edge-translator-runtime\*.*"

  ; Inkscape + SCADA extension
  SetOutPath $INSTDIR\platform-windows\inkscape-runtime
  File /a /r /x "*.com" "..\platform-windows\inkscape-runtime\*.*"
  ;SetOutPath $INSTDIR\platform-windows\inkscape-runtime\share\inkscape\extensions
  ;File /a /r "..\src\inkscape-extension\scada.inx"
  ;File /a /r "..\src\inkscape-extension\scada.py"
  SetOutPath $AppData\inkscape\extensions
  File /a /r "..\src\inkscape-extension\scada.inx"
  File /a /r "..\src\inkscape-extension\scada.py"  

  ; Inkscape additional symbols
  ; SetOutPath $INSTDIR\platform-windows\inkscape-runtime\share\symbols
  ; File /a /r "..\platform-windows\inkscape-symbols\*.*"
  
  ;SetOutPath $INSTDIR\Opc.Ua.CertificateGenerator
  ;File /a /r "..\Opc.Ua.CertificateGenerator\*.*"  

  SetOutPath $INSTDIR\conf-templates
  File /a "..\conf-templates\*.*"  

  SetOverwrite off

  SetOutPath $INSTDIR\src\AdminUI\public\conf
  File /a /r "..\src\AdminUI\public\conf\*.*"

  SetOutPath $INSTDIR\svg
  File /a "..\conf-templates\*.svg"
  ; Svedala CIM-EMS demo displays (ReliCapGrid model, CC BY-SA 4.0)
  File /a "..\svg\svedala*.svg"

  SetOutPath $INSTDIR\conf
  File /a "..\conf-templates\php.ini"
  SetOutPath $INSTDIR\platform-windows\nginx_php-runtime\php
  File /a "..\conf-templates\php.ini"
  SetOutPath $INSTDIR\conf
  File /a "..\conf-templates\nginx.conf"
  File /a "..\conf-templates\nginx_access_control.conf"
  File /a "..\conf-templates\nginx_http.conf"  
  File /a "..\conf-templates\nginx_https.conf"  
  File /a "..\conf-templates\json-scada.json"
  File /a "..\conf-templates\Opc.Ua.DefaultClient.Config.xml"
  File /a "..\conf-templates\telegraf.conf"
  File /a "..\conf-templates\json-scada-config.xlsm"
  File /a "..\conf-templates\log.io-file.json"
  File /a "..\conf-templates\nginx.crt"
  File /a "..\conf-templates\nginx.key"
  File /a "..\conf-templates\node-red-settings.js"

; Desktop shortcuts
  Delete "$DESKTOP\JSON-SCADA\*.*"
  CreateDirectory "$DESKTOP\JSON-SCADA"

; App shortcuts 
  SetOutPath $DESKTOP\JSON-SCADA
  File /a "..\platform-windows\_JSON SCADA WEB.url"
  File /a "..\platform-windows\_JSON SCADA WEB HTTPS.url"
  CreateShortCut "$DESKTOP\JSON-SCADA\_Start_Services.lnk"                "$INSTDIR\platform-windows\start_services.bat"  
  CreateShortCut "$DESKTOP\JSON-SCADA\_Stop_Services.lnk"                 "$INSTDIR\platform-windows\stop_services.bat"  
  CreateShortCut "$DESKTOP\JSON-SCADA\Windows Services.lnk"               "services.msc"  
  ;CreateShortCut "$DESKTOP\JSON-SCADA\_JSON SCADA WEB.lnk"                "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVINDEX $NAVPOSOPT" "$INSTDIR\src\htdocs\images\j-s-256.ico" 
  ;CreateShortCut "$DESKTOP\JSON-SCADA\_JSON SCADA WEB HTTPS.lnk"          "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSSRV$NAVINDEX $NAVPOSOPT" "$INSTDIR\src\htdocs\images\j-s-256.ico" 
  ;!insertmacro CreateInternetShortcut "$Desktop\_JSON SCADA WEB.URL" "http://127.0.0.1"
  ;!insertmacro CreateInternetShortcut "$Desktop\_JSON SCADA WEB HTTPS.URL" "https://127.0.0.1"
  
  ;CreateShortCut "$DESKTOP\JSON-SCADA\Chromium Browser.lnk"               "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT $NAVPOSOPT"
  ;CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Display.lnk"               "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISTEL $NAVPOSOPT" "$INSTDIR\src\htdocs\images\tela.ico" 
  ;CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Events.lnk"                "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISEVE $NAVPOSOPT" "$INSTDIR\src\htdocs\images\chrono.ico" 
  ;CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Historical.lnk"            "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISHEV $NAVPOSOPT" "$INSTDIR\src\htdocs\images\calendar.ico" 
  ;CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Tabular.lnk"               "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISTAB $NAVPOSOPT" "$INSTDIR\src\htdocs\images\tabular.ico" 
  ;CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Alarms.lnk"                "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISANO $NAVPOSOPT" "$INSTDIR\src\htdocs\images\firstaid.ico" 
  ;CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Grafana.lnk"               "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVGRAFAN $NAVPOSOPT" "$INSTDIR\src\htdocs\images\grafana.ico" 
  ;CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Metabase.lnk"              "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVMETABA $NAVPOSOPT" "$INSTDIR\src\htdocs\images\metabase.ico" 
  
  CreateShortCut "$DESKTOP\JSON-SCADA\Excel Config Spreadsheet.lnk"       "$INSTDIR\conf\json-scada-config.xlsm"
  CreateShortCut "$DESKTOP\JSON-SCADA\Compass (Mongodb GUI Client).lnk"   "$INSTDIR\platform-windows\mongodb-compass-runtime\MongoDBCompass.exe"
  ;CreateShortCut "$DESKTOP\JSON-SCADA\pgAdmin4 (Postgres GUI Client).lnk" "$INSTDIR\platform-windows\postgresql-runtime\pgAdmin 4\runtime\pgAdmin4.exe"
  CreateShortCut "$DESKTOP\JSON-SCADA\DBX (DB GUI Client).lnk"      "$INSTDIR\platform-windows\dbx-runtime\dbX.exe"
  CreateShortCut "$DESKTOP\JSON-SCADA\Inkscape SAGE (SVG Editor).lnk"     "$INSTDIR\platform-windows\inkscape-runtime\bin\inkscape.exe"
  CreateShortCut "$DESKTOP\JSON-SCADA\Uninstall.lnk"                      "$INSTDIR\bt-uninst.exe"

  ; clear chromium cache
  Delete "$INSTDIR\platform-windows\browser-data\Default\Cache\*.*"
  RMDir /r "$INSTDIR\platform-windows\browser-data\Default\Web Aplications"
  
  ; Verify system locale to set HMI language
  !define LOCALE_ILANGUAGE '0x1' ;System Language Resource ID     
  !define LOCALE_SLANGUAGE '0x2' ;System Language & Country [Cool]
  !define LOCALE_SABBREVLANGNAME '0x3' ;System abbreviated language
  !define LOCALE_SNATIVELANGNAME '0x4' ;System native language name [Cool]
  !define LOCALE_ICOUNTRY '0x5' ;System country code     
  !define LOCALE_SCOUNTRY '0x6' ;System Country
  !define LOCALE_SABBREVCTRYNAME '0x7' ;System abbreviated country name
  !define LOCALE_SNATIVECTRYNAME '0x8' ;System native country name [Cool]
  !define LOCALE_IDEFAULTLANGUAGE '0x9' ;System default language ID
  !define LOCALE_IDEFAULTCOUNTRY  '0xA' ;System default country code
  !define LOCALE_IDEFAULTCODEPAGE '0xB' ;System default oem code page

  WriteUninstaller "bt-uninst.exe"

  ; Windows Defender exclusions for write intensive data files and runtime processes
  ExecWait 'powershell -ExecutionPolicy Bypass -File "$INSTDIR\platform-windows\defender_exclusions.ps1" -JsonScadaRoot "$INSTDIR"'

  ; Initial setup (databases, demo seed, services) only when there is no database data at all.
  ; Never on upgrades: initial_setup.bat overwrites the PostgreSQL configuration, re-imports the demo
  ; users and points and rewrites every service.
  ${If} $HAS_DATA == 0
    Delete "$INSTDIR\log\upgrade-state.json"
    ExpandEnvStrings $0 %COMSPEC%
    ; am_admin: the installer is already elevated, run it synchronously (no self-elevation)
    ExecWait '"$0" /C ""$INSTDIR\platform-windows\initial_setup.bat" am_admin"'
  ${Else}
    Call UpgradeServicesAndData
  ${EndIf}

  ; record the installed version (read by the next installer for upgrade/downgrade detection)
  WriteRegStr HKLM "Software\JSON_SCADA" "Version" "${VERSION_}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\JSON_SCADA" "DisplayVersion" "${VERSION_}"

  ${If} $HAS_DATA == 0
    MessageBox MB_OK "Installation finished! To run the system: Open the JSON-SCADA desktop folder, execute the '_Start_Services' shortcut and then open '_JSON SCADA WEB'." /SD IDOK
  ${ElseIf} $R9 == 0
    MessageBox MB_OK|MB_ICONINFORMATION "Upgrade to ${VERSION} finished.$\r$\n$\r$\nServices that were running before the upgrade were restarted. The previous configuration and scripts were backed up to $BACKUP_DIR (log: $INSTDIR\log\upgrade.log)." /SD IDOK
  ${Else}
    MessageBox MB_OK|MB_ICONEXCLAMATION "Upgrade to ${VERSION} finished with warnings (result: $R9).$\r$\n$\r$\nReview $INSTDIR\log\upgrade.log before using the system. The previous configuration and scripts were backed up to $BACKUP_DIR." /SD IDOK
  ${EndIf}

SectionEnd

; Upgrade completion, existing databases: service path migrations, then upgrade.ps1 -Phase PostInstall
; (new services, database runtime/schema/FCV/TimescaleDB updates, restart of the services that were
; running). Result in $R9: 0 = ok, 2 = warnings, other = failure.
Function UpgradeServicesAndData

  ; Point the PostgreSQL loader services to src\process_pg_sql (upgrades from the former
  ; sql\process_pg_*.bat loaders)
  nsExec::Exec '"$INSTDIR\platform-windows\nssm.exe" set JSON_SCADA_process_hist Application "$INSTDIR\platform-windows\nodejs-runtime\node.exe"'
  nsExec::Exec '"$INSTDIR\platform-windows\nssm.exe" set JSON_SCADA_process_hist AppParameters "$INSTDIR\src\process_pg_sql\index.js" hist 1'
  nsExec::Exec '"$INSTDIR\platform-windows\nssm.exe" set JSON_SCADA_process_hist AppDirectory "$INSTDIR\src\process_pg_sql"'
  nsExec::Exec '"$INSTDIR\platform-windows\nssm.exe" set JSON_SCADA_process_hist AppStdout "$INSTDIR\log\process_pg_hist.log"'
  nsExec::Exec '"$INSTDIR\platform-windows\nssm.exe" set JSON_SCADA_process_rtdata Application "$INSTDIR\platform-windows\nodejs-runtime\node.exe"'
  nsExec::Exec '"$INSTDIR\platform-windows\nssm.exe" set JSON_SCADA_process_rtdata AppParameters "$INSTDIR\src\process_pg_sql\index.js" rtdata 1'
  nsExec::Exec '"$INSTDIR\platform-windows\nssm.exe" set JSON_SCADA_process_rtdata AppDirectory "$INSTDIR\src\process_pg_sql"'
  nsExec::Exec '"$INSTDIR\platform-windows\nssm.exe" set JSON_SCADA_process_rtdata AppStdout "$INSTDIR\log\process_pg_rtdata.log"'
  Delete "$INSTDIR\sql\process_pg_hist.bat"
  Delete "$INSTDIR\sql\process_pg_rtdata.bat"
  Delete "$INSTDIR\sql\terminate_pg_hist.bat"
  Delete "$INSTDIR\sql\terminate_pg_rtdata.bat"

  DetailPrint "Completing the upgrade: services, databases..."
  nsExec::ExecToLog 'powershell -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\platform-windows\upgrade.ps1" -Phase PostInstall -InstallDir "$INSTDIR" -BundledPgMajor ${PGBIN_1}'
  Pop $R9

FunctionEnd

; Uninstaller

UninstallText "JSON SCADA Uninstall. All files, including the databases and backups, will be removed from $INSTDIR ! To upgrade, run the new installer instead (data is kept)."
UninstallIcon "${NSISDIR}\Contrib\Graphics\Icons\nsis1-uninstall.ico"

Section "Uninstall"

  ${DisableX64FSRedirection}
  SetRegView 64
  ExpandEnvStrings $0 %COMSPEC%

  ; Close processes

  !define SC  `$SYSDIR\sc.exe`

  ExecWait `"${SC}" stop "JSON_SCADA_server_realtime_auth"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_server_realtime_auth"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_config_server_excel"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_config_server_excel"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_demo_simul"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_demo_simul"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_mongofw"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_mongofw"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_mongowr"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_mongowr"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_alarm_beep"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_alarm_beep"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_php"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_php"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_nginx"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_nginx"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_iec101client"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_iec101client"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_iec104client"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_iec104client"`
  ClearErrors
  
  ExecWait `"${SC}" stop "JSON_SCADA_iccpclient"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_iccpclient"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_iec61850client"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_iec61850client"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_dnp3client"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_dnp3client"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_opcuaclient"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_opcuaclient"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_opcdaclient"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_opcdaclient"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_telegraf_runtime"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_telegraf_runtime"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_mqttsparkplugclient"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_mqttsparkplugclient"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_plc4xclient"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_plc4xclient"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_modbusclient"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_modbusclient"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_modbusserver"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_modbusserver"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_iccpclient"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_iccpclient"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_iccpserver"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_iccpserver"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_telegraf_listener"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_telegraf_listener"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_iec101server"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_iec101server"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_iec104server"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_iec104server"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_dnp3server"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_dnp3server"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_iccpserver"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_iccpserver"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_opcuaserver"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_opcuaserver"`
  ClearErrors
  
  ExecWait `"${SC}" stop "JSON_SCADA_opcdaserver"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_opcdaserver"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_cs_data_processor"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_cs_data_processor"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_cs_custom_processor"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_cs_custom_processor"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_calculations"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_calculations"`
  ClearErrors
  
  ExecWait `"${SC}" stop "JSON_SCADA_grafana"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_grafana"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_n8nclient"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_n8nclient"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_nodered_driver"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_nodered_driver"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_nodered_runtime"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_nodered_runtime"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_metabase"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_metabase"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_mongodb"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_mongodb"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_postgresql"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_postgresql"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_postgresql"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_postgresql"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_process_hist"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_process_hist"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_process_rtdata"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_process_rtdata"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_postgresql"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_postgresql"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_log_io_file"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_log_io_file"`
  ClearErrors
  
  ExecWait `"${SC}" stop "JSON_SCADA_log_io_server"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_log_io_server"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_mcp_server"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_mcp_server"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_onvif"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_onvif"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_cim_ems"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_cim_ems"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_cim_ems_demo_field"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_cim_ems_demo_field"`
  ClearErrors

  ; the batch files resolve their helpers relative to the working directory ($OUTDIR)
  SetOutPath "$INSTDIR\platform-windows"
  ExecWait '"$0" /C ""$INSTDIR\platform-windows\mongodb-stop.bat" am_admin"'
  ExecWait '"$0" /C ""$INSTDIR\platform-windows\postgresql-stop.bat" am_admin"'
  ExecWait '"$0" /C ""$INSTDIR\platform-windows\stop_services.bat" am_admin"'
  Sleep 5000
  ; Replaced WMIC calls with a PowerShell helper script that uses Get-CimInstance/Invoke-CimMethod
  ; The installer will call remove_services.bat which in turn invokes
  ; kill_processes_by_pattern.ps1 to find and terminate processes by commandline pattern.
  ; This avoids using the deprecated WMIC tool.
  Sleep 5000
  ExecWait '"$0" /C ""$INSTDIR\platform-windows\remove_services.bat" am_admin"'
  Sleep 5000

  ; Remove Windows Defender exclusions (before platform-windows is deleted)
  ExecWait 'powershell -ExecutionPolicy Bypass -File "$INSTDIR\platform-windows\defender_exclusions.ps1" -JsonScadaRoot "$INSTDIR" -Remove'

  ; ExecWait `regsvr32 gbda_aut.dll -u`
  ; ExecWait `regsvr32 gbhda_aw.dll -u`

  SetOutPath $TEMP ; leave $INSTDIR so that it can be removed
  RMDir /r "$INSTDIR\bin"
  RMDir /r "$INSTDIR\platform-windows" 
  RMDir /r "$INSTDIR\conf" 
  RMDir /r "$INSTDIR\docs" 
  RMDir /r "$INSTDIR\src" 
  RMDir /r "$INSTDIR"
  RMDir /r "$DESKTOP\JSON-SCADA"

  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\JSON_SCADA"
  DeleteRegKey HKLM "SOFTWARE\JSON_SCADA"

SectionEnd
                                                                                                                                            

