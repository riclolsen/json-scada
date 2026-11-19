# DOX: platform-windows — Windows Deployment

## Purpose

Windows platform packaging, installation, and runtime management for JSON-SCADA. Includes the NSIS-based installer, service management scripts, runtime dependencies (MongoDB, PostgreSQL, Node.js, Grafana, etc.), and platform-specific protocol drivers.

## Ownership

- platform-windows owns the Windows distribution, installer, and runtime setup
- Platform-specific protocol drivers (DNP3, OPC-DA) depend on this environment

## Local Contracts

- **Installer:** NSIS script (`json-scada.nsi`) — builds the Windows installer package; runs `defender_exclusions.ps1 -JsonScadaRoot $INSTDIR` at install and the same with `-Remove` at uninstall (before `platform-windows` is deleted)
- **Upgrades (in-place install over an older version):** `.onInit` flags an upgrade when the `HKLM\Software\JSON_SCADA` key, `platform-windows\` files or database data exist, warns on downgrade (registry `Version` vs `VERSION_`) and asks for confirmation. `initial_setup.bat` runs only when there is no data (`postgresql-data\PG_VERSION` and `mongodb-data\WiredTiger` both absent); it must never run over existing data. `upgrade.ps1` does the rest:
  - `-Phase PreInstall` (run from `$PLUGINSDIR`, before any file is replaced): records running `JSON_SCADA_*` services in `log\upgrade-state.json`, stops them all (databases last) and ends processes running from `$INSTDIR`, backs up `conf\`, `platform-windows\*.bat`, `mongodb-conf\` and `src\AdminUI\public\conf` to `backup\upgrade_<date>_<time>\`, copies `mongod.exe` to `mongod-prev.exe`, and moves `postgresql-runtime` to `postgresql-runtime-<data major>` when the data major differs from the bundled one (`PGBIN_1`, read from `pg_ctl.exe` at compile time)
  - `-Phase PostInstall` (after the copy, only when data exists): points `JSON_SCADA_postgresql` at the runtime matching its data, registers the `nssm install` blocks of `create_services.bat` whose service does not exist yet (existing services keep their settings), starts the databases, falls back to `mongod-prev.exe` when the new `mongod` cannot open the data, raises the MongoDB FCV (single-member replica sets only), runs `mongo_seed\b_create-db.js` through `mongosh` stdin and `ALTER EXTENSION timescaledb UPDATE`, then restores the recorded service state. Exit code 0 ok, 1 fatal, 2 warnings; log in `log\upgrade.log`
  - Installer-owned trees are replaced as a whole so stale files do not linger: `python-runtime`, `nodejs-runtime\node_modules\npm` and `corepack`, and the `node_modules` of every shipped `src\` app (`CleanNodeModules` list; keep it in step with the `File` sections). User files are never overwritten: `conf\`, `svg\`, `src\AdminUI\public\conf` and `mongodb-conf\mongod.cfg` use `SetOverwrite off`
  - Upgrade migrations that rename or repoint existing services go in `UpgradeServicesAndData` (NSIS) or `upgrade.ps1`, never in `initial_setup.bat`
- **Service management:** `nssm.exe` — Non-Sucking Service Manager for Windows services
- **Runtime dirs:** `*-runtime/` directories contain pre-downloaded binaries for MongoDB, PostgreSQL, Node.js, Grafana, Metabase, Inkscape, Nginx/PHP, Telegraf, JDK, Ruby, browser, etc.
- **Python runtime:** `python-runtime/` is the `tools` folder of the NuGet `python` 3.14.x package (relocatable, with pip; Python 3.15 is blocked by the pypowsybl wheels). `src/cim-ems/install.bat` (called by `build.bat`/`buildupd.bat`) installs the CIM-EMS package into it; the installer replaces it as a whole. Service `JSON_SCADA_cim_ems` (`python.exe -m cim_ems 1 1 <conf>`, demand start) in `create_services.bat` and the protocol/remove scripts; same for the Svedala demo field simulator service `JSON_SCADA_cim_ems_demo_field` (`-m cim_ems.demo_field <conf>`, demand start). `initial_setup.bat` imports the Svedala demo seed (`demo_cim_*.json`), the installer ships `svg/svedala*.svg` (no overwrite)
- **Service scripts:** `create_services.bat`, `remove_services.bat`, `start_services.bat`, `stop_services.bat`, `restart_services.bat`
- **Protocol scripts:** `start_protocols.bat`, `stop_protocols.bat`, `restart_protocols.bat`
- **Build:** `build.bat`, `buildupd.bat`
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
- Upgrade: install the previous release, add data and change `conf\`, start some services, run the new installer; check `log\upgrade.log`, that data and config are intact and the same services run again
- Quick script check without packaging: compile a copy of `json-scada.nsi` with the `File` payload lines removed (keep the `$PLUGINSDIR\upgrade.ps1` one) using `makensis -NOCD` from `platform-windows`
- Verify all runtime binaries are present in `*-runtime/` directories
