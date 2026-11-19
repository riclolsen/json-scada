# DOX: platform-rhel9 — RHEL 9 / Rocky 9 / Alma 9 Deployment

## Purpose

Automated installation and configuration scripts for deploying JSON-SCADA on RHEL 9 and compatible derivatives (Rocky Linux 9, AlmaLinux 9). Supports both x86-64 and ARM64 architectures.

## Ownership

- platform-rhel9 owns the RHEL 9 installer and service configs
- Sister directory to platform-rhel10 — differs in package versions and compatibility

## Local Contracts

- **Installer:** one `json-scada-install.sh` for x86-64 and ARM64; `JS_ARCH` (`amd64`/`arm64`) is detected from `uname -m` and gates the architecture-specific steps (Docker CE, TimescaleDB packages and `timescaledb-tune`, `timescaledb_toolkit` extension, CIM-EMS demo seed). Never add per-architecture installer copies
- **Upgrades:** re-running the installer over an existing installation upgrades it through `../platform-linux/install-lib.sh` (see [platform-linux/AGENTS.md](../platform-linux/AGENTS.md)): config files via `js_install_config`/`js_install_inis`, database init and demo import only without existing data, programs restored at the end
- **Process manager:** Supervisor (`supervisord.conf`)
- **CIM-EMS driver:** `cim_ems.ini` and the Svedala demo field simulator `cim_ems_demo_field.ini` (both `autostart=false`); the installer imports the Svedala demo seed (`demo_cim_*.json`) on x86-64 only and adds `python3.12` (RHEL 9) or `python3` (RHEL 10) for the venv that `platform-linux/build.sh` creates (x86-64 only)
- One `.ini` file per service (same structure as Ubuntu platform)
- **No `telegraf.ini`** — RHEL uses separate `telegraf-input-*.conf` files
- **No `grafana_server.ini`, `json_scada_http_open.conf`** — fewer config variants than Ubuntu
- Nginx configs: `nginx.conf`, `json_scada_http.conf`, `json_scada_https.conf`, `fastcgi.conf`
- Database configs: `mongod.conf`, `postgresql.conf`, `pg_hba.conf`

## Work Guidance

- The installer handles: package installation (dnf/yum), MongoDB replica set, PostgreSQL, Node.js, Supervisor configs
- ARM64 uses different package sources (pgdg repo `.rpmnew` fix, TimescaleDB packagecloud repo and `timescaledb_18`); keep such differences inside `JS_ARCH` conditionals
- SELinux context may need adjustments for Nginx and custom services
- Keep in sync with platform-rhel10 — differences are version-related

## Verification

- Run installer on fresh RHEL 9 / Rocky 9 / Alma 9 VM
- Validate all daemons via `supervisorctl status`
