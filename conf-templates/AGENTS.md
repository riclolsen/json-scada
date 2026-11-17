# DOX: conf-templates — Configuration Templates

## Purpose

Template configuration files for all JSON-SCADA services: Nginx web server, MongoDB, PostgreSQL, Node.js apps, protocol drivers, Grafana, Metabase, Supervisor, and TLS certificates. These are the reference/default configs used during installation.

## Ownership

- conf-templates owns all configuration file templates
- Platform-specific installers reference these templates during setup

## Local Contracts

- All files here are templates — actual deployed configs may vary per platform
- Naming convention matches the service/component they configure
- `nginx_http.conf` / `nginx_https.conf` — web server config (choose one)
- Proxy locations of server_realtime_auth (`/`, `/nodered/`, `/log-io/`) forward `Host $http_host`, with the port: its websocket upgrades compare the browser `Origin` to it
- HTTPS proxy locations send `X-Forwarded-Proto $scheme` next to `X-Real-IP` (server_realtime_auth marks its session cookies `Secure` from it); keep the platform `json_scada_https.conf` copies in step
- `json-scada.json` — central application config
- `supervisord.conf` — process manager config
- `screen_list.js` — SVG screen definitions
- `config_viewers.js` — viewer configuration
- `customized_module.js` — custom module configuration
- `iccp_config.txt` — ICCP driver configuration
- Grafana dashboards in `grafana-dashboards/` subdirectory
- `json-scada-config.xlsm` / `.ods` — Excel/LibreOffice config workbooks
- TLS/cert files: `nginx.crt`, `nginx.key`, `openssl.cnf`

## Work Guidance

- When modifying templates, ensure backwards compatibility with existing installs
- Document new configuration parameters with comments
- Keep Excel/ODS config workbooks in sync with actual config file structure

## Verification

- Validate JSON/XML/INI syntax after changes
- Compare against corresponding service documentation
