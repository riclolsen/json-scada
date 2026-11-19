# DOX: platform-rhel10 — RHEL 10 / Rocky 10 / Alma 10 Deployment

## Purpose

Automated installation and configuration scripts for deploying JSON-SCADA on RHEL 10 and compatible derivatives (Rocky Linux 10, AlmaLinux 10). Supports both x86-64 and ARM64 architectures.

## Ownership

- platform-rhel10 owns the RHEL 10 installer and service configs
- Sister directory to platform-rhel9 — differs in package versions and compatibility

## Local Contracts

Identical structure to [platform-rhel9/AGENTS.md](../platform-rhel9/AGENTS.md):
- Installer: one `json-scada-install.sh` for x86-64 and ARM64 (`JS_ARCH` detected from `uname -m`; ARM64 adds the pgdg/TimescaleDB packagecloud repo steps and skips Podman, `timescaledb-tune`, the `timescaledb_toolkit` extension and the CIM-EMS demo seed)
- **Upgrades:** re-running the installer over an existing installation upgrades it through `../platform-linux/install-lib.sh` (see [platform-linux/AGENTS.md](../platform-linux/AGENTS.md)): config files via `js_install_config`/`js_install_inis`, database init and demo import only without existing data, programs restored at the end
- Supervisor config: `supervisord.conf`
- One `.ini` file per service
- Nginx, MongoDB, PostgreSQL configs

## Work Guidance

- Keep configs in sync with platform-rhel9 — differences are RHEL 10 package versions and compatibility adjustments
- When adding a new service, add configs to both RHEL platform dirs

## Verification

- Run installer on fresh RHEL 10 / Rocky 10 / Alma 10 VM
- Validate all daemons via `supervisorctl status`
