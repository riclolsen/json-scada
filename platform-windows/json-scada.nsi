; json-scada.nsi
; {json:scada} installer script
; Copyright 2020-2024 - Ricardo L. Olsen

; NSIS (Nullsoft Scriptable Install System) - http://nsis.sourceforge.net/Main_Page

SetCompressor LZMA 

Unicode True
; RequestExecutionLevel user
RequestExecutionLevel admin

!include "TextFunc.nsh"
!include "WordFunc.nsh"
!include "x64.nsh"
!include "LogicLib.nsh"

;--------------------------------

!define VERSION "v.0.39"
!define VERSION_ "0.39.0.0"

Function .onInit
 System::Call 'keexrnel32::CreateMutexA(i 0, i 0, t "MutexJsonScadaInstall") i .r1 ?e'
 Pop $R0
 StrCmp $R0 0 +3
   MessageBox MB_OK|MB_ICONEXCLAMATION "Installer already executing!"
   Abort
FunctionEnd

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
VIAddVersionKey LegalCopyright "Copyright 2020-2024 Ricardo L. Olsen"
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
BGGradient 000000 000080 FFFFFF
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

; REJECT NON 64 BIT OS
${IfNot} ${RunningX64}

    Abort

${EndIf}

${DisableX64FSRedirection}

SetRegView 64

; Closes all processes
  nsExec::Exec 'net stop JSON_SCADA_grafana'
  nsExec::Exec 'net stop JSON_SCADA_metabase'
  nsExec::Exec 'net stop JSON_SCADA_mongodb'
  nsExec::Exec 'net stop JSON_SCADA_calculations'
  nsExec::Exec 'net stop JSON_SCADA_cs_data_processor'
  nsExec::Exec 'net stop JSON_SCADA_cs_custom_processor'
  nsExec::Exec 'net stop JSON_SCADA_server_realtime_auth'
  nsExec::Exec 'net stop JSON_SCADA_config_server_excel'
  nsExec::Exec 'net stop JSON_SCADA_demo_simul'
  nsExec::Exec 'net stop JSON_SCADA_mongofw'
  nsExec::Exec 'net stop JSON_SCADA_mongowr'
  nsExec::Exec 'net stop JSON_SCADA_alarm_beeep'
  nsExec::Exec 'net stop JSON_SCADA_process_rtdata'
  nsExec::Exec 'net stop JSON_SCADA_process_hist'
  nsExec::Exec 'net stop JSON_SCADA_iec101client'
  nsExec::Exec 'net stop JSON_SCADA_iec101server'
  nsExec::Exec 'net stop JSON_SCADA_iec104client'
  nsExec::Exec 'net stop JSON_SCADA_iec104server'
  nsExec::Exec 'net stop JSON_SCADA_iccpclient'
  nsExec::Exec 'net stop JSON_SCADA_iccpserver'
  nsExec::Exec 'net stop JSON_SCADA_iec61850client'
  nsExec::Exec 'net stop JSON_SCADA_plctags'
  nsExec::Exec 'net stop JSON_SCADA_dnp3client' 
  nsExec::Exec 'net stop JSON_SCADA_opcuaclient' 
  nsExec::Exec 'net stop JSON_SCADA_opcdaclient' 
  nsExec::Exec 'net stop JSON_SCADA_opcuaserver' 
  nsExec::Exec 'net stop JSON_SCADA_mqttsparkplugclient'  
  nsExec::Exec 'net stop JSON_SCADA_plc4xclient'  
  nsExec::Exec 'net stop JSON_SCADA_telegraf_runtime'
  nsExec::Exec 'net stop JSON_SCADA_telegraf_listener'
  nsExec::Exec 'net stop JSON_SCADA_nginx'
  nsExec::Exec 'net stop JSON_SCADA_php'
  nsExec::Exec 'net stop JSON_SCADA_log_io_file'
  nsExec::Exec 'net stop JSON_SCADA_log_io_server'
  nsExec::Exec 'c:\json-scada\platform-windows\stop_services.bat'
  nsExec::Exec '"c:\json-scada\platform-windows\postgresql-runtime\bin\pg_ctl" stop -D c:\json-scada\platform-windows\postgresql-runtime'
  nsExec::Exec 'c:\json-scada\platform-windows\stop_services.bat'
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
    
  # PROTOCOL://IP:PORT  
  StrCpy $HTTPSRV   "http://127.0.0.1" 
  StrCpy $NAVWINCMD "platform-windows\browser-runtime\chrome.exe"
  StrCpy $NAVDATDIR "--user-data-dir=$INSTDIR\platform-windows\browser-data"
  StrCpy $NAVPREOPT ""
  StrCpy $NAVPOSOPT "--disable-popup-blocking --no-proxy-server --bwsi"
  StrCpy $NAVINDEX  "/"
  StrCpy $NAVVISABO "/about.html"
  StrCpy $NAVVISEVE "/events.html"
  StrCpy $NAVVISHEV "/events.html?MODO=4"
  StrCpy $NAVVISTAB "/tabular.html"
  StrCpy $NAVVISANO "/tabular.html?SELMODULO=ALARMS_VIEWER"
  StrCpy $NAVVISTEL "/display.html"
  StrCpy $NAVGRAFAN "/grafana"
  StrCpy $NAVMETABA "/metabase"

  ; write reg info
  WriteRegStr HKLM SOFTWARE\JSON_SCADA "Install_Dir" "$INSTDIR"

  ; write uninstall strings
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\JSON_SCADA" "DisplayName" "JSON SCADA (remove only)"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\JSON_SCADA" "UninstallString" '"$INSTDIR\bt-uninst.exe"'

  ; erases all of the old chromium
  RMDir /r "$INSTDIR\browser-runtime" 
  RMDir /r "$INSTDIR\browser-data" 

  ; erases all of the old Inkscape but the share dir
  ; New files will replace old.
  RMDir /r "$INSTDIR\platform-windows\inkscape-runtime\doc" 
  RMDir /r "$INSTDIR\platform-windows\inkscape-runtime\etc" 
  RMDir /r "$INSTDIR\platform-windows\inkscape-runtime\lib"  
  Delete "$INSTDIR\platform-windows\inkscape-runtime\*.*"  
  
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
  CreateDirectory "$INSTDIR\platform-windows\browser-runtime"
  CreateDirectory "$INSTDIR\platform-windows\browser-data"
  CreateDirectory "$INSTDIR\platform-windows\mongodb-compass-runtime"
  CreateDirectory "$INSTDIR\platform-windows\mongodb-data"
  CreateDirectory "$INSTDIR\platform-windows\mongodb-conf"
  CreateDirectory "$INSTDIR\platform-windows\mongodb-runtime"
  CreateDirectory "$INSTDIR\platform-windows\nodejs-runtime"
  CreateDirectory "$INSTDIR\platform-windows\inkscape-runtime"
  CreateDirectory "$INSTDIR\platform-windows\postgresql-data"
  CreateDirectory "$INSTDIR\platform-windows\postgresql-runtime"
  CreateDirectory "$INSTDIR\platform-windows\nginx_php-runtime"
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

  SetOutPath $INSTDIR\bin
  File /a /r "..\bin\*.*"
  File /a "..\platform-windows\nssm.exe"

  ; preserve previous start_services.bat
  Rename $INSTDIR\platform-windows\start_services.bat $INSTDIR\platform-windows\start_services.bat.bak

  SetOutPath $INSTDIR\platform-windows
  File /a "..\platform-windows\*.bat"
  File /a "..\platform-windows\*.ps1"
  File /a "..\platform-windows\nssm.exe"
  File /a "..\platform-windows\sounder.exe"
  File /a "..\platform-windows\vc_redist.x64.exe"
  File /a "..\platform-windows\dotnet-runtime-8.0.10-win-x64.exe"
  File /a "..\platform-windows\OPC Core Components Redistributable (x64) 3.00.108.msi"
  ;File /a "..\platform-windows\gbda_aut.dll"
  ;File /a "..\platform-windows\gbhda_aw.dll"
  ;ExecWait `regsvr32 gbda_aut.dll`
  ;ExecWait `regsvr32 gbhda_aw.dll`

  ; Visual C redist: needed for timescaledb
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
  ExecWait '"$INSTDIR\platform-windows\dotnet-runtime-8.0.10-win-x64.exe" /install /passive /quiet'
  Sleep 1000
  ExecWait 'msiexec /i "$INSTDIR\platform-windows\OPC Core Components Redistributable (x64) 3.00.108.msi" /qn'
  Sleep 1000
  
  SetOutPath $INSTDIR\platform-windows\nodejs-runtime
  File /a /r "..\platform-windows\nodejs-runtime\*.*"

  SetOutPath $INSTDIR\platform-windows\telegraf-runtime
  File /a "..\platform-windows\telegraf-runtime\telegraf.exe"

  SetOutPath $INSTDIR\docs
  File /a /r "..\docs\*.*"
  
  SetOutPath $INSTDIR\platform-windows\jdk-runtime
  File /a /r "..\platform-windows\jdk-runtime\*.*"

  SetOutPath $INSTDIR\platform-windows\metabase-runtime
  File /a "..\platform-windows\metabase-runtime\metabase.jar"

  SetOutPath $INSTDIR\platform-windows\grafana-runtime
  File /a /r "..\platform-windows\grafana-runtime\*.*"

  SetOutPath $INSTDIR\mongo_seed_demo
  File /a /r "..\demo-docker\mongo_seed\files\*.*"

  SetOutPath $INSTDIR\mongo_seed_demo
  File /a /r "..\mongo_seed\a_rs-init.js"

  SetOutPath $INSTDIR\mongo_seed
  File /a /r "..\mongo_seed\*.*"

  SetOutPath $INSTDIR\platform-windows\mongodb-compass-runtime
  File /a /r "..\platform-windows\mongodb-compass-runtime\*.*"

  SetOutPath $INSTDIR\platform-windows\mongodb-runtime
  File /a /r "..\platform-windows\mongodb-runtime\*.*"

  SetOutPath $INSTDIR\platform-windows\mongodb-conf
  File /a /r "..\platform-windows\mongodb-conf\*.*"

  SetOutPath $INSTDIR\platform-windows\postgresql-runtime
  File /a /r "..\platform-windows\postgresql-runtime\*.*"

  SetOutPath $INSTDIR\sql
  File /a "..\sql\*.bat"
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

  SetOutPath $INSTDIR\src\certificate-creator
  File /a "..\src\certificate-creator\client.conf"
  File /a "..\src\certificate-creator\server.conf"
  File /a "..\src\certificate-creator\create_certs.sh"

  SetOutPath $INSTDIR\svg
  File /a    "..\conf-templates\*.svg"
  File /a    "..\conf-templates\screen_list.js"
  SetOutPath $INSTDIR\src\AdminUI
  File /a    "..\src\AdminUI\*.*"
  SetOutPath $INSTDIR\src\AdminUI\src
  File /a /r "..\src\AdminUI\src\*.*"
  SetOutPath $INSTDIR\src\AdminUI\dist
  File /a /r   "..\src\AdminUI\dist\*.*"
  File /a    ".\release_notes.txt"
  
  #SetOutPath $INSTDIR\src\htdocs
  #File /a "..\src\htdocs\*.*"
  #File /a ".\release_notes.txt"
  #SetOutPath $INSTDIR\src\htdocs\sounds
  #File /a "..\src\htdocs\sounds\critical.wav"
  #File /a "..\src\htdocs\sounds\noncritical.wav"
  #SetOutPath $INSTDIR\src\htdocs\scripts
  #File /a /r "..\src\htdocs\scripts\*.*"
  #SetOutPath $INSTDIR\src\htdocs\sage-cepel-displays
  #File /a "..\src\htdocs\sage-cepel-displays\README.md"
  #SetOutPath $INSTDIR\src\htdocs\images
  #File /a /r "..\src\htdocs\images\*.*"
  #SetOutPath $INSTDIR\src\htdocs\charts
  #File /a /r "..\src\htdocs\charts\*.*"
  #SetOutPath $INSTDIR\src\htdocs\lib
  #File /a /r "..\src\htdocs\lib\*.*"
  #SetOutPath $INSTDIR\src\htdocs\i18n
  #File /a /r "..\src\htdocs\i18n\*.*"
  #SetOutPath $INSTDIR\src\htdocs-admin
  #File /a /r "..\src\htdocs-admin\*.*"
  #SetOutPath $INSTDIR\src\htdocs-login
  #File /a /r "..\src\htdocs-login\*.*"

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
  File /a "..\src\cs_custom_processor\README.md"
  File /a "..\src\cs_custom_processor\*.js"
  File /a "..\src\cs_custom_processor\package.json"
  File /a "..\src\cs_custom_processor\package-lock.json"
  SetOutPath $INSTDIR\src\cs_custom_processor\node_modules
  File /a /r "..\src\cs_custom_processor\node_modules\*.*"

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

  SetOutPath $INSTDIR\src\mqtt-sparkplug
  File /a "..\src\mqtt-sparkplug\*.js"
  File /a "..\src\mqtt-sparkplug\*.json"
  File /a "..\src\mqtt-sparkplug\*.md"
  SetOutPath $INSTDIR\src\mqtt-sparkplug\node_modules
  File /a /r "..\src\mqtt-sparkplug\node_modules\*.*"

  SetOutPath $INSTDIR\src\OPC-UA-Server
  File /a "..\src\OPC-UA-Server\*.js"
  File /a "..\src\OPC-UA-Server\*.json"
  File /a "..\src\OPC-UA-Server\*.md"
  SetOutPath $INSTDIR\src\OPC-UA-Server\node_modules
  File /a /r "..\src\OPC-UA-Server\node_modules\*.*"

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

  SetOutPath $INSTDIR\platform-windows\browser-runtime
  File /a /r "..\platform-windows\browser-runtime\*.*"

  SetOutPath $INSTDIR\platform-windows\browser-data
  File /a /r "..\platform-windows\browser-data\*.*"

  ; Inkscape custom built
  SetOutPath $INSTDIR\platform-windows\inkscape-runtime
  File /a /r "..\platform-windows\inkscape-runtime\*.*"

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
  File /a /r "..\svg\*.*"

  SetOutPath $INSTDIR\src\cs_custom_processor
  File /a "..\src\cs_custom_processor\customized_module.js"

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

; Desktop shortcuts
  Delete "$DESKTOP\JSON-SCADA\*.*"
  CreateDirectory "$DESKTOP\JSON-SCADA"

; App shortcuts 
  CreateShortCut "$DESKTOP\JSON-SCADA\_Start_Services.lnk"               "$INSTDIR\platform-windows\start_services.bat"  
  CreateShortCut "$DESKTOP\JSON-SCADA\_Stop_Services.lnk"                "$INSTDIR\platform-windows\stop_services.bat"  
  CreateShortCut "$DESKTOP\JSON-SCADA\Windows Services.lnk"              "services.msc"  
  CreateShortCut "$DESKTOP\JSON-SCADA\_JSON SCADA WEB.lnk"               "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVINDEX $NAVPOSOPT" "$INSTDIR\src\htdocs\images\j-s-256.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Chromium Browser.lnk"              "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT $NAVPOSOPT"
  CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Display.lnk"              "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISTEL $NAVPOSOPT" "$INSTDIR\src\htdocs\images\tela.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Events.lnk"               "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISEVE $NAVPOSOPT" "$INSTDIR\src\htdocs\images\chrono.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Historical.lnk"           "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISHEV $NAVPOSOPT" "$INSTDIR\src\htdocs\images\calendar.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Tabular.lnk"              "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISTAB $NAVPOSOPT" "$INSTDIR\src\htdocs\images\tabular.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Alarms.lnk"               "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISANO $NAVPOSOPT" "$INSTDIR\src\htdocs\images\firstaid.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Grafana.lnk"              "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVGRAFAN $NAVPOSOPT" "$INSTDIR\src\htdocs\images\grafana.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Viewer - Metabase.lnk"             "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVMETABA $NAVPOSOPT" "$INSTDIR\src\htdocs\images\metabase.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Excel Config Spreadsheet.lnk"      "$INSTDIR\conf\json-scada-config.xlsm"
  CreateShortCut "$DESKTOP\JSON-SCADA\Compass (Mongodb GUI Client).lnk"  "$INSTDIR\platform-windows\mongodb-compass-runtime\MongoDBCompass.exe"
  CreateShortCut "$DESKTOP\JSON-SCADA\Inkscape SAGE (SVG Editor).lnk"    "$INSTDIR\platform-windows\inkscape-runtime\bin\inkscape.exe"
  CreateShortCut "$DESKTOP\JSON-SCADA\Uninstall.lnk"                     "$INSTDIR\bt-uninst.exe"


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

; If database data empty (no previous installation), then do initial setup for MongoDB and PostgreSQL
IfFileExists "$INSTDIR\postgresql-data\base" pgDatabaseExists 0

  ExpandEnvStrings $0 %COMSPEC%
  ExecWait '"$0" /C "$INSTDIR\platform-windows\initial_setup.bat"'

pgDatabaseExists:

  MessageBox MB_OK "Installation finished! To run the system: Open the JSON-SCADA desktop folder, execute the '_Start_Services' shortcut and then open '_JSON SCADA WEB'."
  
SectionEnd

; Uninstaller

UninstallText "JSON SCADA Uninstall. All files will be removed from $INSTDIR !"
UninstallIcon "${NSISDIR}\Contrib\Graphics\Icons\nsis1-uninstall.ico"

Section "Uninstall"
  
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

  ExecWait `"${SC}" stop "JSON_SCADA_shell_api"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_shell_api"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_php"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_php"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_nginx"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_nginx"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_i104m"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_i104m"`
  ClearErrors

  ExecWait `"${SC}" stop "JSON_SCADA_plctags"`
  Sleep 50
  ExecWait `"${SC}" delete "JSON_SCADA_plctags"`
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

  ExecWait '"$0" /C "$INSTDIR\platform-windows\mongodb-stop.bat"'
  ExecWait '"$0" /C "$INSTDIR\platform-windows\postgresql-stop.bat"'
  ExecWait '"$0" /C "$INSTDIR\platform-windows\stop_services.bat"'
  Sleep 5000
  ExecWait `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\sql\\%'" CALL TERMINATE`
  Sleep 1000
  ExecWait `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\jdk-runtime\\bin\\%'" CALL TERMINATE`
  ExecWait `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\grafana-runtime\\bin\\%'" CALL TERMINATE`
  ExecWait `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\nginx_php-runtime\\php\\%'" CALL TERMINATE`
  ExecWait `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\nginx_php-runtime\\%'" CALL TERMINATE`
  ExecWait `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\nodejs-runtime\\%'" CALL TERMINATE`
  ExecWait `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\browser-runtime\\%'" CALL TERMINATE`
  ExecWait `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\mongodb-runtime\\%'" CALL TERMINATE`
  ExecWait `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\inkscape-runtime\\%'" CALL TERMINATE`
  ExecWait `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\platform-windows\\%'" CALL TERMINATE`
  ExecWait `wmic PROCESS WHERE "COMMANDLINE LIKE '%..\\platform-windows\\%'" CALL TERMINATE`
  ExecWait `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\bin\\%'" CALL TERMINATE`
  ExecWait `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\sql\\%'" CALL TERMINATE`
  Sleep 5000
  ExecWait '"$0" /C "$INSTDIR\platform-windows\remove_services.bat"'
  Sleep 5000
  
  ; ExecWait `regsvr32 gbda_aut.dll -u`
  ; ExecWait `regsvr32 gbhda_aw.dll -u`

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
                                                                                                                                            

