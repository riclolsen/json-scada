# DOX: platform-ubuntu — Ubuntu 24.04 / 26.04 Deployment

## Purpose

Automated installation and configuration scripts for deploying JSON-SCADA on Ubuntu 24.04 and 26.04 (both x86-64 and ARM64). One installer and one config set serve both versions. Serves as the reference Linux platform for the project; the root `Dockerfile` copies its configs.

## Ownership

- platform-ubuntu owns the Ubuntu installer and service configs for 24.04 and 26.04
- Former `platform-ubuntu-2404/` and `platform-ubuntu-2604/` were merged here; never split it again into per-version copies

## Local Contracts

- **Installer:** `json-scada-install.sh` — full automated installation script; it runs under `sh` (dash), so keep it POSIX. `JS_UBUNTU` (`VERSION_ID` from `/etc/os-release`, 24.04 or 26.04, anything else exits) selects the version differences:
  - 24.04: `dotnet-sdk-8.0`, `php8.3-fpm`, the MongoDB kernel lock below
  - 26.04: `dotnet-sdk-10.0`, `php8.5-fpm`
  - Both use the `noble` MongoDB and TimescaleDB apt repos (no newer repo exists for 26.04)
- **Package prompts:** package upgrades and the Telegraf install use noninteractive debconf and keep existing ucf-managed configuration; the Compass install uses `apt -y`
- **Upgrades:** re-running the installer over an existing installation upgrades it through `../platform-linux/install-lib.sh` (see [platform-linux/AGENTS.md](../platform-linux/AGENTS.md)): config files via `js_install_config`/`js_install_inis`, database init and demo import only without existing data, programs restored at the end. Installations made from `platform-ubuntu-2404/` or `platform-ubuntu-2604/` upgrade by running this installer; `js_unmodified` also checks the git history of those former paths
- **Kernel lock (MongoDB, 24.04 only):** MongoDB does not start on kernel 6.19+. The installer first installs the GA kernel (`linux-generic`), purges the HWE metapackages, `apt-mark hold`s the GA metapackages and pins `linux-*-hwe-24.04`/`linux-hwe-*` to -1 in `/etc/apt/preferences.d/json-scada-kernel`. If the running kernel is 6.19+, it purges all installed kernel packages of 6.19+ (only when an older `linux-image` is present), runs `update-grub`, asks for a reboot and exits; the user re-runs the installer after the reboot
- **Process manager:** Supervisor (`supervisord.conf`) — manages all JSON-SCADA processes; `.ini` commands that call scripts in this folder use `/home/jsonscada/json-scada/platform-ubuntu/`
- **INI files:** One `.ini` file per service (e.g., `iec104server.ini`, `mqtt-sparkplug.ini`, `calculations.ini`, etc.)
- **Nginx configs:** `nginx.conf`, `json_scada_http.conf`, `json_scada_https.conf`, `json_scada_http_open.conf`, `fastcgi.conf`
- **Database configs:** `mongod.conf`, `postgresql.conf`, `pg_hba.conf`
- **Grafana config:** `grafana.ini`, `grafana_server.ini`
- **Metabase config:** `metabase.ini`
- **MCP server config:** `mcp_server.ini`
- **CIM-EMS driver:** `cim_ems.ini` (`autostart=false`) runs `src/cim-ems/.venv/bin/python -m cim_ems`; the venv comes from `platform-linux/build.sh` and needs `python3-venv`. x86_64 only (no pypowsybl ARM64 wheel). `cim_ems_demo_field.ini` (`autostart=false`) runs the Svedala demo field simulator `-m cim_ems.demo_field`; the installer imports the Svedala demo seed (`demo_cim_*.json`). The root `Dockerfile` copies both programs
- **SVG editor tool:** `inkscape-plus-sage.sh`
- **Telegraf config:** `telegraf.ini`, `telegraf-input-*.conf`, `telegraf-output-json-scada.conf`

## Work Guidance

- The installer script handles: package installation, MongoDB replica set init, PostgreSQL setup, Node.js/npm, service config generation, TLS certs
- All `.ini` files are read by Supervisor to define process behavior
- `.conf` files reference `conf-templates/` structure but with platform-appropriate paths
- Keep Ubuntu version differences inside `JS_UBUNTU` conditionals or the version `case`; prefer one shared command when a package works on both
- `json_scada_http_open.conf` provides an open (no auth) HTTP variant for development
- ARM64 support is built in — no separate config needed, except that the CIM-EMS driver is not installed on ARM64

## Verification

- `sh -n json-scada-install.sh` (dash)
- Run `json-scada-install.sh` in Ubuntu 24.04 and 26.04 VMs/containers to verify full deployment
- Validate all daemons start via `supervisorctl status`
- Test HTTP/HTTPS access via Nginx configs
