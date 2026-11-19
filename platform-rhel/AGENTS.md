# DOX: platform-rhel — RHEL 9/10, Rocky, Alma Deployment

## Purpose

Automated installation and configuration scripts for deploying JSON-SCADA on RHEL 9 and RHEL 10 and compatible derivatives (Rocky Linux, AlmaLinux 9/10). One installer and one config set serve both versions on x86-64 and ARM64.

## Ownership

- platform-rhel owns the RHEL installer, the Inkscape+SAGE build script and the service configs for RHEL 9 and 10
- Former `platform-rhel9/` and `platform-rhel10/` were merged here; never split it again into per-version or per-architecture copies

## Local Contracts

- **Installer:** one `json-scada-install.sh`. `JS_EL` = `rpm -E %rhel` (9 or 10, anything else exits) and `JS_ARCH` (`amd64`/`arm64`, from `uname -m`) gate the differences:
  - RHEL 9: Python `python3.12` + `python3-tkinter`, Docker CE (x86-64), RPM Fusion ffmpeg, TimescaleDB Toolkit package and `timescaledb_toolkit` extension (extension on x86-64 only)
  - RHEL 10: Python `python3`, Podman only, no ffmpeg, no TimescaleDB Toolkit, no `dnf module disable` (RHEL 10 has no AppStream modules)
  - TimescaleDB: `timescaledb-2-postgresql-18` on RHEL 9 x86-64, `timescaledb_18` otherwise; ARM64 adds the pgdg `.rpmnew` fix and the TimescaleDB packagecloud repo and skips `timescaledb-tune`
  - CIM-EMS demo seed (`demo_cim_*.json`) on x86-64 only
- `inkscape-plus-sage.sh` branches on `JS_EL` for its build dependencies
- **Build prerequisites:** install development groups separately from Maven and the Java 21 JDK; if either installation fails, restore previously running programs on upgrades and stop. The build and PLC4J service use Java 21 explicitly
- **Upgrades:** re-running the installer over an existing installation upgrades it through `../platform-linux/install-lib.sh` (see [platform-linux/AGENTS.md](../platform-linux/AGENTS.md)): config files via `js_install_config`/`js_install_inis`, database init and demo import only without existing data, programs restored at the end. Installations made from `platform-rhel9/` or `platform-rhel10/` upgrade by running this installer; `js_unmodified` also checks the git history of those former paths
- **Process manager:** Supervisor (`supervisord.conf`); `.ini` commands that call scripts in this folder use `/home/jsonscada/json-scada/platform-rhel/`
- **CIM-EMS driver:** `cim_ems.ini` and the Svedala demo field simulator `cim_ems_demo_field.ini` (both `autostart=false`); the venv is created by `platform-linux/build.sh` (x86-64 only)
- One `.ini` file per service (same structure as Ubuntu platform)
- IEC 61850: only `iec61850client.ini`/`iec61850server.ini`, which run the Go drivers `bin/iec61850_client`/`bin/iec61850_server` built by `platform-linux/build.sh`; no separate Go-variant programs
- **No `telegraf.ini`** — RHEL uses separate `telegraf-input-*.conf` files
- **No `grafana_server.ini`, `json_scada_http_open.conf`** — fewer config variants than Ubuntu
- Nginx configs: `nginx.conf`, `json_scada_http.conf`, `json_scada_https.conf`, `fastcgi.conf`
- Database configs: `mongod.conf`, `postgresql.conf` (needs `glibc-langpack-en` for `en_US.UTF-8`), `pg_hba.conf`

## Work Guidance

- The installer handles: package installation (dnf), MongoDB replica set, PostgreSQL, Node.js, Supervisor configs
- Keep RHEL version and CPU differences inside `JS_EL`/`JS_ARCH` conditionals; prefer one shared command when a package works on both
- SELinux context may need adjustments for Nginx and custom services

## Verification

- `bash -n json-scada-install.sh inkscape-plus-sage.sh`
- Run the installer on fresh RHEL / Rocky / Alma 9 and 10 machines (a systemd-enabled `rockylinux/rockylinux:9`/`:10` container works for x86-64)
- Validate all daemons via `supervisorctl status` and the services `mongod`, `postgresql-18`, `nginx`, `grafana-server`
