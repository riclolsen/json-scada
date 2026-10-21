# DOX: platform-windows — Windows Deployment

## Purpose

Windows platform packaging, installation, and runtime management for JSON-SCADA. Includes the NSIS-based installer, service management scripts, runtime dependencies (MongoDB, PostgreSQL, Node.js, Grafana, etc.), and platform-specific protocol drivers.

## Ownership

- platform-windows owns the Windows distribution, installer, and runtime setup
- Platform-specific protocol drivers (DNP3, OPC-DA) depend on this environment

## Local Contracts

- **Installer:** NSIS script (`json-scada.nsi`) — builds the Windows installer package; runs `defender_exclusions.ps1 -JsonScadaRoot $INSTDIR` at install and the same with `-Remove` at uninstall (before `platform-windows` is deleted)
- **Service management:** `nssm.exe` — Non-Sucking Service Manager for Windows services
- **Runtime dirs:** `*-runtime/` directories contain pre-downloaded binaries for MongoDB, PostgreSQL, Node.js, Grafana, Metabase, Inkscape, Nginx/PHP, Telegraf, JDK, Ruby, browser, etc.
- **Service scripts:** `create_services.bat`, `remove_services.bat`, `start_services.bat`, `stop_services.bat`, `restart_services.bat`
- **Protocol scripts:** `start_protocols.bat`, `stop_protocols.bat`, `restart_protocols.bat`
- **Build:** `build.bat`, `buildupd.bat`, `build_msys2.sh`
- **Config:** `conf/` subdirectory mirrors `conf-templates/` for deployed configs
- **Credentials:** PSCP (`pscp.exe`) for secure file transfer
- **Tools:** `wget.exe`, `tar.exe`, `nssm.exe`, `ffmpeg.exe`, `sounder.exe`, `json-extractor.bat`
- **Release notes:** `release_notes.txt` — version history
- **Optimization:** `windows-optimizations.md` — Windows tuning guide

## Work Guidance

- NSIS installer must handle: service creation, runtime detection, MongoDB replica set init, PostgreSQL init, config deployment
- Runtime dependencies are bundled as pre-downloaded archives in `*-runtime/` dirs
- Keep service scripts idempotent (no double-registration)
- Protocol batch scripts should respect existing service state
- OPC-DA and DNP3 are Windows-only — their registration/removal is critical
- Test installer on clean Windows 10/11 and Server 2019/2022 VMs
- PowerShell scripts (`.ps1`) complement batch files for advanced operations

## Verification

- Run `initial_setup.bat` on a clean Windows VM to verify fresh installation
- Run `start_services.bat` then `stop_services.bat` — all services start/stop cleanly
- NSIS installer: compile with `makensis json-scada.nsi`, test install/uninstall cycle
- Verify all runtime binaries are present in `*-runtime/` directories
