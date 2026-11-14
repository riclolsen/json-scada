# DOX: platform-ubuntu-2404 — Ubuntu 24.04 Deployment

## Purpose

Automated installation and configuration scripts for deploying JSON-SCADA on Ubuntu 24.04 (both x86-64 and ARM64). Serves as the reference Linux platform for the project.

## Ownership

- platform-ubuntu-2404 owns the Ubuntu 24.04 installer and service configs
- Sister directories (platform-ubuntu-2604) track the same layout for newer Ubuntu releases

## Local Contracts

- **Installer:** `json-scada-install.sh` — full automated installation script; it runs under `sh` (dash), so keep it POSIX
- **Kernel lock (MongoDB):** MongoDB does not start on kernel 6.19+. The installer first installs the GA kernel (`linux-generic`), purges the HWE metapackages, `apt-mark hold`s the GA metapackages and pins `linux-*-hwe-24.04`/`linux-hwe-*` to -1 in `/etc/apt/preferences.d/json-scada-kernel`. If the running kernel is 6.19+, it purges all installed kernel packages of 6.19+ (only when an older `linux-image` is present), runs `update-grub`, asks for a reboot and exits; the user re-runs the installer after the reboot
- **Process manager:** Supervisor (`supervisord.conf`) — manages all JSON-SCADA processes
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
- Keep Ubuntu 24.04 and 26.04 configs in sync — differences are minimal (package versions)
- `json_scada_http_open.conf` provides an open (no auth) HTTP variant for development
- ARM64 support is built in — no separate config needed, except that the CIM-EMS driver is not installed on ARM64

## Verification

- Run `json-scada-install.sh` in Ubuntu 24.04 VM/container to verify full deployment
- Validate all daemons start via `supervisorctl status`
- Test HTTP/HTTPS access via Nginx configs
