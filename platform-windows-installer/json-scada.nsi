; json-scada.nsi
; {json:scada} installer script
; Copyright 2020 - Ricardo L. Olsen

; NSIS (Nullsoft Scriptable Install System) - http://nsis.sourceforge.net/Main_Page

Unicode True
RequestExecutionLevel user

!include "TextFunc.nsh"
!include "WordFunc.nsh"

;--------------------------------

!define VERSION "v.0.1"
!define VERSION_ "0.1.0.0"

Function .onInit
 System::Call 'keexrnel32::CreateMutexA(i 0, i 0, t "MutexOshmiInstall") i .r1 ?e'
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
Icon "..\src\htdocs\images\j-s-256.ico"

!define /date DATE "%d_%m_%Y"
OutFile "json-scada_setup_${VERSION}.exe"

VIProductVersion ${VERSION_}
VIAddVersionKey ProductName "JSON SCADA"
VIAddVersionKey Comments "SCADA IoT Software"
VIAddVersionKey CompanyName "Ricardo Olsen"
VIAddVersionKey LegalCopyright "Copyright 2020 Ricardo L. Olsen"
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
BGGradient 000000 800000 FFFFFF
InstallColors FF8080 000030
XPStyle on

InstallDir "c:\json-scada"
InstallDirRegKey HKLM "Software\JSON_SCADA" "Install_Dir"

CheckBitmap "${NSISDIR}\Contrib\Graphics\Checks\classic-cross.bmp"

LicenseText "JSON-SCADA Release Notes"
LicenseData "release_notes.txt"

; Must be admin
RequestExecutionLevel admin

;--------------------------------

Page license
;Page components
;Page directory
Page instfiles

UninstPage uninstConfirm
UninstPage instfiles

; LoadLanguageFile "${NSISDIR}\Contrib\Language files\PortugueseBR.nlf"

;--------------------------------

AutoCloseWindow false
ShowInstDetails show

;--------------------------------

Section "" ; empty string makes it hidden, so would starting with -

; Closes all OSHMI processes
  nsExec::Exec 'net stop JSON_SCADA_mongodb'
  nsExec::Exec 'net stop JSON_SCADA_calculations'
  nsExec::Exec 'net stop JSON_SCADA_cs_data_processor'
  nsExec::Exec 'net stop JSON_SCADA_server_realtime'
  nsExec::Exec 'net stop JSON_SCADA_process_rtdata'
  nsExec::Exec 'net stop JSON_SCADA_process_hist'
  nsExec::Exec 'net stop OSHMI_iec104'
  nsExec::Exec 'net stop OSHMI_iec104'
  nsExec::Exec 'c:\json-scada\platform-windows\stop_services.bat'
  nsExec::Exec '..\postgresql-runtime\bin\pg_ctl stop -D ..\postgresql-data'
  ; nsExec::Exec 'taskkill /F /IM mon_proc.exe'
  ; nsExec::Exec `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\oshmi\\bin\\%'" CALL TERMINATE`
  SetOverwrite on

  var /GLOBAL NAVWINCMD
  var /GLOBAL NAVDATDIR
  var /GLOBAL NAVPREOPT
  var /GLOBAL NAVPOSOPT
  var /GLOBAL NAVVISABO
  var /GLOBAL NAVVISEVE
  var /GLOBAL NAVVISHEV
  var /GLOBAL NAVVISTAB
  var /GLOBAL NAVVISANO
  var /GLOBAL NAVVISTEL
  var /GLOBAL NAVVISTRE
  var /GLOBAL NAVVISOVW
  var /GLOBAL NAVVISDOC
  var /GLOBAL NAVVISLOG
  var /GLOBAL NAVGRAFAN
  var /GLOBAL HTTPSRV
    
  StrCpy $HTTPSRV   "http://127.0.0.1:8080"
 #StrCpy $HTTPSRV   "https://127.0.0.1"
  StrCpy $NAVWINCMD "browser-runtime\chrome.exe"
  StrCpy $NAVDATDIR "--user-data-dir=$INSTDIR\browser-data"
  StrCpy $NAVPREOPT "--process-per-site --no-sandbox"
  StrCpy $NAVPOSOPT "--disable-popup-blocking --no-proxy-server --bwsi --disable-extensions --disable-sync --no-first-run"
  StrCpy $NAVVISABO "/htdocs/about.html"
  StrCpy $NAVVISEVE "/htdocs/events.html"
  StrCpy $NAVVISHEV "/htdocs/events.html?MODO=4"
  StrCpy $NAVVISTAB "/htdocs/tabular.html"
  StrCpy $NAVVISANO "/htdocs/tabular.html?SELMODULO=TODOS_ANORMAIS"
  StrCpy $NAVVISTEL "/htdocs/display.html"
  StrCpy $NAVVISTRE "/htdocs/trend.html"
  StrCpy $NAVVISOVW "/htdocs/overview.html"
  StrCpy $NAVVISDOC "/htdocs/listdocs.php"
  StrCpy $NAVVISLOG "/htdocs/listlogs.php"
  StrCpy $NAVGRAFAN "/grafana"

  ; write reg info
  WriteRegStr HKLM SOFTWARE\OSHMI "Install_Dir" "$INSTDIR"

  ; write uninstall strings
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\JSON_SCADA" "DisplayName" "JSON SCADA (remove only)"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\JSON_SCADA" "UninstallString" '"$INSTDIR\bt-uninst.exe"'

  ; erases all of the old chromium
  RMDir /r "$INSTDIR\browser-runtime" 
  RMDir /r "$INSTDIR\browser-data" 

  ; erases all of the old Inkscape but the share dir
  ; New files will replace old.
  RMDir /r "$INSTDIR\inkscape-runtime\doc" 
  RMDir /r "$INSTDIR\inkscape-runtime\etc" 
  RMDir /r "$INSTDIR\inkscape-runtime\lib"  
  Delete "$INSTDIR\inkscape-runtime\*.*"  
  
  CreateDirectory "$INSTDIR\bin"
  CreateDirectory "$INSTDIR\browser-runtime"
  CreateDirectory "$INSTDIR\browser-data"
  CreateDirectory "$INSTDIR\conf"
  CreateDirectory "$INSTDIR\docs"
  CreateDirectory "$INSTDIR\grafana-dashboards"
  CreateDirectory "$INSTDIR\grafana-runtime"
  CreateDirectory "$INSTDIR\inkscape-runtime"
  CreateDirectory "$INSTDIR\log"
  CreateDirectory "$INSTDIR\mongo_seed"
  CreateDirectory "$INSTDIR\mongodb-data"
  CreateDirectory "$INSTDIR\mongodb-runtime"
  CreateDirectory "$INSTDIR\nodejs-runtime"
  CreateDirectory "$INSTDIR\platform-windows"
  CreateDirectory "$INSTDIR\postgresql-data"
  CreateDirectory "$INSTDIR\postgresql-runtime"
  CreateDirectory "$INSTDIR\sql"
  CreateDirectory "$INSTDIR\src"
  CreateDirectory "$INSTDIR\nginx_php-runtime"
; CreateDirectory "$INSTDIR\PowerBI"
; CreateDirectory "$INSTDIR\Opc.Ua.CertificateGenerator"

  SetOutPath $INSTDIR

  File /a "..\platform-windows-installer\release_notes.txt"
  File /a "..\LICENSE"

  SetOutPath $INSTDIR\bin
  File /a /r "..\bin\*.*"
  File /a /r "..\platform-windows\nssm.exe"

  SetOutPath $INSTDIR\platform-windows
  File /a /r "..\platform-windows\*.*"

  SetOutPath $INSTDIR\nodejs-runtime
  File /a /r "..\nodejs-runtime\*.*"

  SetOutPath $INSTDIR\docs
  File /a /r "..\docs\*.*"

  SetOutPath $INSTDIR\grafana-dashboards
  File /a /r "..\grafana-dashboards\*.*"

  SetOutPath $INSTDIR\grafana-runtime
  File /a /r "..\grafana-runtime\*.*"

  SetOutPath $INSTDIR\mongo_seed_demo
  File /a /r "..\demo-docker\mongo_seed\*.*"

  SetOutPath $INSTDIR\mongo_seed
  File /a /r "..\mongo_seed\*.*"

  SetOutPath $INSTDIR\mongodb-runtime
  File /a /r "..\mongodb-runtime\*.*"

  SetOutPath $INSTDIR\postgresql-runtime
  File /a /r "..\postgresql-runtime\*.*"

  SetOutPath $INSTDIR\sql
  File /a "..\sql\*.bat"
  File /a "..\sql\*.md"

  SetOutPath $INSTDIR\nginx_php-runtime
  File /r /x *.log "..\nginx_php-runtime\*.*" 

;  SetOutPath $INSTDIR\nginx_php-runtime\php
;  File /a "..\conf-templates-runtime\php.ini"
;  SetOutPath $INSTDIR\nginx_php-runtime\conf
;  File /a "..\conf-templates\nginx.conf"

;  SetOutPath $INSTDIR\conf-templates
;  File /a "..\conf-templates\*.*"

  SetOutPath $INSTDIR\src\htdocs
  File /a "..\src\htdocs\*.*"
  SetOutPath $INSTDIR\src\htdocs-admin
  File /a "..\src\htdocs-admin\*.*"
  SetOutPath $INSTDIR\src\htdocs-login
  File /a "..\src\htdocs-login\*.*"

  SetOutPath $INSTDIR\src\alarm_beep
  File /a "..\src\alarm_beep\*.*"

  SetOutPath $INSTDIR\src\oshmi2json
  File /a "..\src\oshmi2json\*.*"

  SetOutPath $INSTDIR\src\cs_data_processor
  File /a "..\src\cs_data_processor\*.*"

  SetOutPath $INSTDIR\src\server_realtime
  File /a "..\src\server_realtime\*.*"
    
  ;SetOutPath $INSTDIR\extprogs
  ;File /a "..\extprogs\vcredist_x86.exe"
  ;File /a "..\extprogs\vcredist_x86-2012.exe"
  ;File /a "..\extprogs\vcredist_x86-2013.exe"
  ;File /a "..\extprogs\vcredist_x86-2015.exe"
  ;File /a "..\extprogs\vcredist_x86-2017.exe"
  ;File /a "..\extprogs\vcredist_x86-15-17-19.exe"

  SetOutPath $INSTDIR\browser-runtime
  File /a /r "..\browser-runtime\*.*"

  SetOutPath $INSTDIR\browser-data
  File /a /r "..\browser-data\*.*"

  ; Inkscape custom built
  SetOutPath $INSTDIR\inkscape-runtime
  File /a /r "..\inkscape-runtime\*.*"

  ; Inkscape additional symbols
  ; SetOutPath $INSTDIR\inkscape-runtime\share\symbols
  ; File /a /r "..\inkscape-symbols\*.*"
  
  ;SetOutPath $INSTDIR\Opc.Ua.CertificateGenerator
  ;File /a /r "..\Opc.Ua.CertificateGenerator\*.*"  

  SetOutPath $INSTDIR\conf-templates
  File /a "..\conf-templates\*.*"  

  SetOverwrite off

  ;SetOutPath $INSTDIR\mongodb-data
  ;File /a /r "..\mongodb-data\*.*"

  SetOutPath $INSTDIR\conf
  File /a "..\conf-templates\nginx_http.conf"  
  File /a "..\conf-templates\nginx_https.conf"  
  File /a "..\conf-templates\json-scada.json"

  SetOutPath "$INSTDIR\svg"
  File /a "..\src\htdocs\svg\kaw2.svg"
  File /a "..\src\htdocs\svg\kik3.svg"
  File /a "..\src\htdocs\svg\knh2.svg"
  File /a "..\src\htdocs\svg\kor1.svg"
  File /a "..\conf-templates\screen_list.js"

; Visual C redist: necessario para executar o PHP
  ;nsExec::Exec '"$INSTDIR\extprogs\vcredist_x86.exe" /q'
  ;nsExec::Exec '"$INSTDIR\extprogs\vcredist_x86-2012.exe" /q'
  ;nsExec::Exec '"$INSTDIR\extprogs\vcredist_x86-2013.exe" /q'
  ;nsExec::Exec '"$INSTDIR\extprogs\vcredist_x86-2015.exe" /q'
  ;nsExec::Exec '"$INSTDIR\extprogs\vcredist_x86-2017.exe" /q'
  ;nsExec::Exec '"$INSTDIR\extprogs\vcredist_x86-15-17-19.exe" /q'

;  MessageBox MB_YESNO "Wish to substitute Windows Shell by the HMIShell? \nWARNING: ANSWERING YES WILL BLOCK THE MACHINE FOR THE OPERATOR" IDNO InstFim 
; LabelShell:
; registry key to change Windows shell
;  WriteRegStr HKCU "Software\Microsoft\Windows NT\CurrentVersion\Winlogon" "Shell" "c:\\oshmi\\bin\\hmishell.exe"
; registry key to disable task manager
;  WriteRegDword HKCU "Software\Microsoft\Windows\CurrentVersion\Policies\System" "DisableTaskMgr" 0x01
; InstFim:

; chaves para o windows   
;  WriteINIStr "$INSTDIR\conf\hmi.ini"  "RUN" "EVENTS_VIEWER"     '"$INSTDIR\$NAVWINCMD $NAVDATDIR --bopt --app=$HTTPSRV$NAVVISEVE"'
;  WriteINIStr "$INSTDIR\conf\hmi.ini"  "RUN" "TABULAR_VIEWER"    '"$INSTDIR\$NAVWINCMD $NAVDATDIR --bopt --app=$HTTPSRV$NAVVISTAB"'
;  WriteINIStr "$INSTDIR\conf\hmi.ini"  "RUN" "SCREEN_VIEWER"     '"$INSTDIR\$NAVWINCMD $NAVDATDIR --bopt --app=$HTTPSRV$NAVVISTEL"'
;  WriteINIStr "$INSTDIR\conf\hmi.ini"  "RUN" "TREND_VIEWER"      '"$INSTDIR\$NAVWINCMD $NAVDATDIR --bopt --app=$HTTPSRV$NAVVISTRE"'
;  WriteINIStr "$INSTDIR\conf\hmi.ini"  "RUN" "DOCS_VIEWER"       '"$INSTDIR\$NAVWINCMD $NAVDATDIR --bopt --app=$HTTPSRV$NAVVISDOC"'
;  WriteINIStr "$INSTDIR\conf\hmi.ini"  "RUN" "LOGS_VIEWER"       '"$INSTDIR\$NAVWINCMD $NAVDATDIR --bopt --app=$HTTPSRV$NAVVISLOG"'
 
; Aqui ficam todos os atalhos no Desktop, apagando os antigos
  Delete "$DESKTOP\JSON-SCADA\*.*"
  CreateDirectory "$DESKTOP\JSON-SCADA"

; Cria atalhos para os aplicativos
  CreateShortCut "$DESKTOP\JSON-SCADA\_Start_OSHMI.lnk"                "$INSTDIR\bin\start_hmi.bat"  
  CreateShortCut "$DESKTOP\JSON-SCADA\Clean Browser Cache.lnk"         "$INSTDIR\bin\cache_clean.bat"  

  CreateShortCut "$DESKTOP\JSON-SCADA\Chromium Browser.lnk"            "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT $NAVPOSOPT"
  CreateShortCut "$DESKTOP\JSON-SCADA\About.lnk"                       "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISABO $NAVPOSOPT" "$INSTDIR\htdocs\images\oshmi.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Display Viewer.lnk"               "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISTEL $NAVPOSOPT" "$INSTDIR\htdocs\images\tela.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Events Viewer.lnk"               "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISEVE $NAVPOSOPT" "$INSTDIR\htdocs\images\chrono.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Historical Events.lnk"           "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISHEV $NAVPOSOPT" "$INSTDIR\htdocs\images\calendar.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Tabular Viewer.lnk"              "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISTAB $NAVPOSOPT" "$INSTDIR\htdocs\images\tabular.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Alarms Viewer.lnk"               "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVVISANO $NAVPOSOPT" "$INSTDIR\htdocs\images\firstaid.ico" 
  CreateShortCut "$DESKTOP\JSON-SCADA\Grafana.lnk"                     "$INSTDIR\$NAVWINCMD" " $NAVDATDIR $NAVPREOPT --app=$HTTPSRV$NAVGRAFAN $NAVPOSOPT" "$INSTDIR\htdocs\images\grafana.ico" 

;  CreateShortCut "$DESKTOP\JSON-SCADA\Operation Manual.lnk"            "$INSTDIR\bin\operation_manual.bat"
;  CreateShortCut "$DESKTOP\JSON-SCADA\Configuration Manual.lnk"        "$INSTDIR\bin\configuration_manual.bat"

  CreateShortCut "$DESKTOP\JSON-SCADA\Inkscape SAGE.lnk"               "$INSTDIR\inkscape-runtime\inkscape.exe"
  CreateShortCut "$DESKTOP\JSON-SCADA\Nginx and PHP Start.lnk"         "$INSTDIR\nginx_php-runtime\start_nginx_php.bat"

; apaga o cache do chrome
  Delete "$INSTDIR\browser-data\Default\Cache\*.*"
  RMDir /r "$INSTDIR\browser-data\Default\Web Aplications"

; cria regras de firewall

; Add an application to the firewall exception list - All Networks - All IP Version - Enabled
;  SimpleFC::AddApplication "OSHMI Webserver" "$INSTDIR\bin\webserver.exe" 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;
;  SimpleFC::AddApplication "OSHMI Shell" "$INSTDIR\bin\hmishell.exe" 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;
;  SimpleFC::AddApplication "OSHMI Mon_Proc" "$INSTDIR\bin\mon_proc.exe" 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;
;  SimpleFC::AddApplication "OSHMI QTester104" "$INSTDIR\bin\QTester104.exe" 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;
;  SimpleFC::AddApplication "OSHMI DNP3" "$INSTDIR\bin\dnp3.exe" 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;
;  SimpleFC::AddApplication "OSHMI MODBUS" "$INSTDIR\bin\modbus.exe" 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;
;  SimpleFC::AddApplication "OSHMI ICCP" "$INSTDIR\bin\iccp_client.exe" 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;
;  SimpleFC::AddApplication "OSHMI NGINX" "$INSTDIR\nginx_php\nginx.exe" 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;
;  SimpleFC::AddApplication "OSHMI PHP-CGI" "$INSTDIR\nginx_php\php\php-cgi.exe" 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)

;  SimpleFC::AddPort 65280 "OSHMI Webserver" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 65281 "OSHMI Webserver" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 8082 "OSHMI Webserver" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 8099 "OSHMI Webserver" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 51909 "OSHMI Shell" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 8081 "OSHMI Mon_Proc" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 51908 "OSHMI Webserver" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 2404 "OSHMI QTester104" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 65280 "OSHMI QTester104" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 65281 "OSHMI QTester104" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 51909 "OSHMI NGINX" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 80 "OSHMI NGINX" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 443 "OSHMI NGINX" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 9000 "OSHMI PHP-CGI" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 8098 "OSHMI ICCP" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 8097 "OSHMI MODBUS" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 8098 "OSHMI MODBUS" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 8098 "OSHMI DNP3" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
;  SimpleFC::AddPort 8096 "OSHMI DNP3" 256 0 2 "" 1
;  Pop $0 ; return error(1)/success(0)
  
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

;  System::Call 'kernel32::GetSystemDefaultLangID() i .r0'
;  System::Call 'kernel32::GetLocaleInfoA(i 1024, i ${LOCALE_SNATIVELANGNAME}, t .r1, i ${NSIS_MAX_STRLEN}) i r0'
;  System::Call 'kernel32::GetLocaleInfoA(i 1024, i ${LOCALE_SNATIVECTRYNAME}, t .r2, i ${NSIS_MAX_STRLEN}) i r0'
;  System::Call 'kernel32::GetLocaleInfoA(i 1024, i ${LOCALE_SLANGUAGE}, t .r3, i ${NSIS_MAX_STRLEN}) i r0'
;  ;MessageBox MB_OK|MB_ICONINFORMATION "Your System LANG Code is: $0. $\r$\nYour system language is: $1. $\r$\nYour system language is: $2. $\r$\nSystem Locale INFO: $3."
;  IntOp $R0 $0 & 0xFFFF
;  ;MessageBox MB_OK|MB_ICONINFORMATION "$R0"
;  IntCmp $R0 1046 lang_portuguese
;  IntCmp $R0 1033 lang_english
;
;  ; default is english - us
;  Goto lang_english
;  
;lang_portuguese:
;;  MessageBox MB_OK|MB_ICONINFORMATION "Portuguese"
;  nsExec::Exec '$INSTDIR\i18n\go-pt_br.bat'
;  Goto lang_end
;  
;lang_english:
;;  MessageBox MB_OK|MB_ICONINFORMATION "English"
;  nsExec::Exec '$INSTDIR\i18n\go-en_us.bat'
;  Goto lang_end
;
;  lang_end:    

  WriteUninstaller "bt-uninst.exe"


  IfFileExists "$INSTDIR\postgresql-data\base" pgDatabaseExists 0

  nsExec::Exec '$INSTDIR\platform-windows\postgrsql-initdb.bat'
  nsExec::Exec 'copy $INSTDIR\conf-templates\pg_hba.conf $INSTDIR\postgresql-data'
  nsExec::Exec 'copy $INSTDIR\conf-templates\postgresql.conf $INSTDIR\postgresql-data'
  nsExec::Exec '$INSTDIR\platform-windows\postgrsql-create_service.bat'
  nsExec::Exec '$INSTDIR\platform-windows\postgrsql-start.bat'
  nsExec::Exec '$INSTDIR\postgrsql-runtime\bin\psql -U postgres -h localhost -f c:\json-scada\sq\create-tables.sql'

  pgDatabaseExists:

  MessageBox MB_OK "JSON-SCADA Installed! To quickly run the system after installed: Open the JSON-SCADA desktop folder and execute the '_Start JSON-SCADA' shortcut."
  
SectionEnd

; Uninstaller

UninstallText "OSHMI Uninstall. All files will be removed from $INSTDIR !"
UninstallIcon "${NSISDIR}\Contrib\Graphics\Icons\nsis1-uninstall.ico"

Section "Uninstall"

; Fecha processos

  ; SetOutPath $INSTDIR\bin
  nsExec::Exec 'net stop OSHMI_rtwebsrv'
  nsExec::Exec 'net stop OSHMI_iec104'
  nsExec::Exec 'net stop OSHMI_iccp'
  nsExec::Exec 'net stop OSHMI_dnp3'
  nsExec::Exec 'net stop OSHMI_modbus'
  nsExec::Exec 'net stop OSHMI_opc'
  nsExec::Exec 'net stop OSHMI_s7'
  nsExec::Exec 'c:\oshmi\bin\stop_all.bat'
  nsExec::Exec 'c:\oshmi\nginx_php\stop_nginx_php.bat'
  nsExec::Exec `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\bin\\%'" CALL TERMINATE`
  nsExec::Exec `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\browser-runtime\\%'" CALL TERMINATE`
  nsExec::Exec `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\nginx_php\\%'" CALL TERMINATE`
  nsExec::Exec `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\inkscape-runtime\\%'" CALL TERMINATE`
  nsExec::Exec `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\postgresql-runtime\\%'" CALL TERMINATE`
  nsExec::Exec `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\grafana-runtime\\%'" CALL TERMINATE`
  nsExec::Exec `wmic PROCESS WHERE "COMMANDLINE LIKE '%c:\\json-scada\\sql\\%'" CALL TERMINATE`

; Remove an application from the firewall exception list
; SimpleFC::RemoveApplication "$INSTDIR\webserver.exe"
; Pop $0 ; return error(1)/success(0)
; SimpleFC::RemoveApplication "$INSTDIR\hmishell.exe"
; Pop $0 ; return error(1)/success(0)
; SimpleFC::RemoveApplication "$INSTDIR\mon_proc.exe"
; Pop $0 ; return error(1)/success(0)
; SimpleFC::RemoveApplication "$INSTDIR\QTester104.exe"
; Pop $0 ; return error(1)/success(0)
; SimpleFC::RemoveApplication "$INSTDIR\dnp3.exe"
; Pop $0 ; return error(1)/success(0)
; SimpleFC::RemoveApplication "$INSTDIR\modbus.exe"
; Pop $0 ; return error(1)/success(0)
; SimpleFC::RemoveApplication "$INSTDIR\iccp_client.exe"
; Pop $0 ; return error(1)/success(0)
; SimpleFC::RemoveApplication  "$INSTDIR\nginx_php\nginx.exe"
; Pop $0 ; return error(1)/success(0)
; SimpleFC::RemoveApplication "$INSTDIR\nginx_php\php\php-cgi.exe"
; Pop $0 ; return error(1)/success(0)
; SimpleFC::RemoveApplication "$INSTDIR\opc_client.exe"
; Pop $0 ; return error(1)/success(0)
; SimpleFC::RemoveApplication "$INSTDIR\s7client.exe"
; Pop $0 ; return error(1)/success(0)

  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\JSON_SCADA"
  DeleteRegKey HKLM "SOFTWARE\JSON_SCADA"
  ;WriteRegStr  HKCU "Software\Microsoft\Windows NT\CurrentVersion\Winlogon" "Shell" "explorer.exe"
  ;WriteRegDword HKCU "Software\Microsoft\Windows\CurrentVersion\Policies\System" "DisableTaskMgr" 0x00
  
  nsExec::Exec '$INSTDIR\etc\remove_services.bat'
  
  Delete "$INSTDIR\*.*"
  Delete "$INSTDIR\bin\*.*"
  Delete "$INSTDIR\browser-data\*.*"
  Delete "$INSTDIR\browser-runtime\*.*"
  Delete "$INSTDIR\conf\*.*"
  Delete "$INSTDIR\docs\*.*"
  RMDir /r "$INSTDIR\bin" 
  RMDir /r "$INSTDIR\browser-runtime" 
  RMDir /r "$INSTDIR\browser-data" 
  RMDir /r "$INSTDIR\inkscape-runtime" 
  RMDir /r "$INSTDIR\mongodb-runtime" 
  RMDir /r "$INSTDIR\mongodb-data" 
  RMDir /r "$INSTDIR\postgresql-runtime" 
  RMDir /r "$INSTDIR\postgresql-data" 
  RMDir /r "$INSTDIR\conf" 
  RMDir /r "$INSTDIR\docs" 
  RMDir /r "$INSTDIR"
  RMDir /r "$DESKTOP\JSON-SCADA"

SectionEnd
                                                                                                                                            

