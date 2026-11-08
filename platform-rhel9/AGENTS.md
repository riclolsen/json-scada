# DOX: platform-rhel9 — RHEL 9 / Rocky 9 / Alma 9 Deployment

## Purpose

Automated installation and configuration scripts for deploying JSON-SCADA on RHEL 9 and compatible derivatives (Rocky Linux 9, AlmaLinux 9). Supports both x86-64 and ARM64 architectures.

## Ownership

- platform-rhel9 owns the RHEL 9 installer and service configs
- Sister directory to platform-rhel10 — differs in package versions and compatibility

## Local Contracts

- **Installer:** `json-scada-install.sh` (x86-64), `json-scada-install-aarch64.sh` (ARM64)
- **Process manager:** Supervisor (`supervisord.conf`)
- **CIM-EMS driver:** `cim_ems.ini` and the Svedala demo field simulator `cim_ems_demo_field.ini` (both `autostart=false`); the x86-64 installer imports the Svedala demo seed (`demo_cim_*.json`) and adds `python3.12` (RHEL 9) or `python3` (RHEL 10) for the venv that `platform-linux/build.sh` creates. Not installed by the aarch64 installers
- One `.ini` file per service (same structure as Ubuntu platform)
- **No `telegraf.ini`** — RHEL uses separate `telegraf-input-*.conf` files
- **No `grafana_server.ini`, `json_scada_http_open.conf`** — fewer config variants than Ubuntu
- Nginx configs: `nginx.conf`, `json_scada_http.conf`, `json_scada_https.conf`, `fastcgi.conf`
- Database configs: `mongod.conf`, `postgresql.conf`, `pg_hba.conf`

## Work Guidance

- The installer handles: package installation (dnf/yum), MongoDB replica set, PostgreSQL, Node.js, Supervisor configs
- ARM64 variant (`json-scada-install-aarch64.sh`) may have different package sources
- SELinux context may need adjustments for Nginx and custom services
- Keep in sync with platform-rhel10 — differences are version-related

## Verification

- Run installer on fresh RHEL 9 / Rocky 9 / Alma 9 VM
- Validate all daemons via `supervisorctl status`
