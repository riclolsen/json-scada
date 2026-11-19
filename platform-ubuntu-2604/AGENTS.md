# DOX: platform-ubuntu-2604 — Ubuntu 26.04 Deployment

## Purpose

Automated installation and configuration scripts for deploying JSON-SCADA on Ubuntu 26.04 (both x86-64 and ARM64). Mirrors the platform-ubuntu-2404 layout with version-appropriate package references.

## Ownership

- platform-ubuntu-2604 owns the Ubuntu 26.04 installer and service configs
- Sister directory to platform-ubuntu-2404 — differences are limited to package versions

## Local Contracts

Identical structure to [platform-ubuntu-2404/AGENTS.md](../platform-ubuntu-2404/AGENTS.md):
- Installer: `json-scada-install.sh`
- **Upgrades:** re-running the installer over an existing installation upgrades it through `../platform-linux/install-lib.sh` (see [platform-linux/AGENTS.md](../platform-linux/AGENTS.md)): config files via `js_install_config`/`js_install_inis`, database init and demo import only without existing data, programs restored at the end
- Process manager: `supervisord.conf`
- One `.ini` file per service
- Nginx, MongoDB, PostgreSQL, Grafana, Telegraf, Metabase configs
- `inkscape-plus-sage.sh` for SVG/SAGE workflow

## Work Guidance

- Keep configs in sync with platform-ubuntu-2404 — the only differences are Ubuntu version-specific package names and paths
- When adding a new service, add its `.ini` and config files to both Ubuntu platform dirs

## Verification

- Run `json-scada-install.sh` in Ubuntu 26.04 VM/container to verify full deployment
- Validate all daemons start via `supervisorctl status`
