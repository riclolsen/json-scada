# DOX: platform-linux — Generic Linux Build & Management Scripts

## Purpose

Cross-distribution Linux scripts for building, exporting, and managing JSON-SCADA processes. These scripts work across Ubuntu, RHEL, and other Linux distributions.

## Ownership

- platform-linux owns generic Linux management scripts
- Distribution-specific installers (platform-ubuntu-*, platform-rhel*) own their automation

## Local Contracts

- `build.sh` — build all Go/Node.js/.NET components for Linux; also runs `src/cim-ems/install.sh` (Python venv for the CIM-EMS driver, skipped on non-x86_64)
- `restart_processes.sh` — restart all JSON-SCADA processes
- `restart_protocols.sh` — restart protocol driver processes only
- `export_project.sh` — export project configuration
- `import_project.sh` — import project configuration
- `install-lib.sh` — POSIX sh helpers sourced by every distribution installer (`. ../platform-linux/install-lib.sh; js_begin`) so that re-running an installer over an existing installation is a clean upgrade. Upgrade = `git pull` + the same installer again:
  - `js_begin` detects an existing installation (`/var/lib/json-scada/installed-commit`, the `server_realtime_auth.ini` program, `bin/calculations` or MongoDB `WiredTiger` data), refuses a checkout that does not contain the installed commit unless confirmed, backs up `conf/` and the Supervisor program dir to `backup/upgrade_<date>_<time>/`, records running programs in `/var/lib/json-scada/running-programs`, stops all Supervisor programs and raises the MongoDB FCV (`mongo-fcv.js`) before packages are upgraded
  - `js_install_config SRC DEST` installs every system config file: on upgrades a locally changed DEST is kept and the new file goes to `DEST.jsnew` (return 2). "Unchanged" = checksum in `/var/lib/json-scada/config-manifest` or equal to any git version of SRC. `js_record` updates the manifest after a scripted edit of an installed file
  - `js_install_inis SRCDIR DESTDIR` installs Supervisor programs: a changed program file gets the new version with its `autostart` kept per section (warning if other lines were changed too); unchanged program files no longer shipped are removed
  - `js_pg_prepare MAJOR` sets `JS_PG_HAS_DATA` and `JS_PG_KEEP_MAJOR` (data in an older major cluster: do not initdb, configure or start MAJOR; the old cluster keeps serving). `js_pg_upgrade` runs `ALTER EXTENSION ... UPDATE` for timescaledb/timescaledb_toolkit
  - `js_mongo_has_data` (json_scada database exists) chooses between the demo seed and `js_mongo_upgrade` (FCV + `mongo_seed/b_create-db.js` via mongosh stdin)
  - `js_append_once` for lines added to system or user files; `js_finish` restarts the recorded programs on upgrades (`supervisorctl reload` first), stores the installed commit and prints the warnings (also in `log/upgrade.log`)
- `mongo-fcv.js` — raises the MongoDB feature compatibility version to the server version (standalone or single-member replica set only)

## Work Guidance

- Scripts should be POSIX-compatible (bash + standard tools)
- Assume `supervisorctl` for process management
- Detect distribution at runtime for package-appropriate behavior
- Installer rules for clean upgrades (all `platform-ubuntu-*`/`platform-rhel*` installers): copy system config files only with `js_install_config`/`js_install_inis`, never plain `cp`; database initialization and demo imports only in the "no data" branch; appends through `js_append_once`; services started with `systemctl restart` (package upgrades leave the old daemon running); `mkdir -p`; `gpg --dearmor --yes`; keep the user's files in `conf/` (copy templates only when missing)

## Verification

- Run `build.sh` on both Ubuntu and RHEL to verify cross-distro compatibility
- Test `restart_processes.sh` and `restart_protocols.sh` with a running deployment
- Installers: `dash -n` on each `json-scada-install*.sh` and on `install-lib.sh`; upgrade test = install the previous release, change configs and `autostart`, `git pull`, run the installer again, check `log/upgrade.log`, the `.jsnew` files and that data and running programs are back
