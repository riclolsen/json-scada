# DOX: src — Source Code

## Purpose

All source code for the JSON-SCADA platform: protocol drivers, web UI, data processors, tools, and utilities. Each subdirectory is a self-contained component (Node.js, Go, .NET/C#, PHP, Python).

## Ownership

- **Root** owns the component catalog, language conventions, and build patterns
- Each child subdirectory owns its own implementation, dependencies, and AGENTS.md where one exists
- Submodules (git) are documented but their internals are owned upstream

## Local Contracts

- Node.js apps follow the pattern: `index.js` (main), `app-defs.js`, `load-config.js`, `simple-logger.js`, `redundancy.js`
- Node.js apps share `package.json` with `@json-scada/common` or similar local deps
- .NET Core projects follow the pattern: `Program.cs`, `TagsCreation.cs`, `MongoCommands.cs`, `MongoUpdate.cs`, `AsduReceiveHandler.cs`, `Redundancy.cs`
- Go projects follow: base package with `go.mod`/`go.sum`, single `main.go` or split by concern
- All components use `README.md` for documentation
- Node.js and scripted components default to `../../conf/json-scada.json` (override with `JS_CONFIG_FILE`); compiled drivers run from `bin/` and use `../conf/json-scada.json`
- Build artifacts go to `../bin/` or `./bin/`
- Prefer `str_replace` over `write_file` for targeted edits in existing files
- Load `svg-scada` skill when editing SVG SCADA files
- Use `gravity_index` before recommending/integrating third-party services
- Connection-oriented drivers publish `protocolConnections.stats.isConnected` (Boolean) and keep `activeNodeKeepAliveTimeTag` advancing, including drivers without redundancy; contract in `docs/DEVELOPER_GUIDE.md` (`protocolConnections`, Redundancy Management) and `docs/schema.md` (Communication status tags). Each driver's AGENTS.md states what "connected" means for it. Connection documents are created with `stats: null`, and MongoDB rejects a dotted `$set` of `stats.*` on a null element (write error 28), so a driver writing those paths must replace the null with an empty document and retry

## Work Guidance

- Protocol drivers: maintain zero-downtime reconnection, TLS support, config via MongoDB
- Node.js apps: use the `redundancy.js` module for HA, `simple-logger.js` for logging
- .NET Core apps: target .NET 8.0+, use `MongoDB.Driver` for DB access
- Go apps: target Go 1.26+, use `go.mongodb.org/mongo-driver` for DB access
- Web UI (AdminUI): Vue.js SPA in `src/AdminUI`, build with Vite
- New components should follow the established pattern of the closest sibling

## Verification

- Node.js: `npm install` and `npm test` (if tests exist) in each component
- .NET: `dotnet build` and `dotnet test` in project directories
- Go: `go build ./...` and `go test ./...` in component directories
- AdminUI: `npm run build` for production build check

## Child DOX Index

- [AdminUI](AdminUI/AGENTS.md) — Vue.js web admin interface (SPA)
- [iccp](iccp/AGENTS.md) — ICCP TASE2 client/server protocol drivers (Go)
- [mcp-json-scada-db](mcp-json-scada-db/AGENTS.md) — MCP server for AI access to data, commands, configuration, operation, health/log diagnosis, displays, calculated tags and CIM-EMS control (TypeScript)
- [dnp3-go](dnp3-go/AGENTS.md) — DNP3 client/server on go-dnp3 (Go, drop-in alternative)
- [modbus-go](modbus-go/AGENTS.md) — Modbus client/server on go-modbus (Go, drop-in replacement; the services run it)
- [mqtt-sparkplug](mqtt-sparkplug/AGENTS.md) — MQTT/Sparkplug-B pub/sub client (Node.js)
- [server_realtime_auth](server_realtime_auth/AGENTS.md) — Realtime WebSocket data server with JWT auth (Node.js)
- [calculations](calculations/AGENTS.md) — Compiled cyclic calculations engine (Go)
- [mongowr](mongowr/AGENTS.md) — MongoDB writer for one-way UDP replication (Node.js)
- [mongofw](mongofw/AGENTS.md) — MongoDB forwarder/firewall for one-way air-gap replication (Node.js)
- [cs_custom_processor](cs_custom_processor/AGENTS.md) — Customizable change stream data processor (TypeScript)
- [cs_data_processor](cs_data_processor/AGENTS.md) — Change stream data processor (Node.js)
- [OPC-UA-Client-Go](OPC-UA-Client-Go/AGENTS.md) — OPC UA client driver (Go, drop-in alternative, no .NET runtime)
- [OPC-UA-Server](OPC-UA-Server/AGENTS.md) — OPC UA server driver (Node.js)
- [OPC-DA-Client](OPC-DA-Client/AGENTS.md) — OPC DA client driver (.NET Core, Windows)
- [OPC-DA-Server](OPC-DA-Server/AGENTS.md) — OPC DA server plugin (.NET Framework, Windows)
- [iec61850/iec61850_client](iec61850/iec61850_client/AGENTS.md) — IEC 61850 MMS client driver (Go, drop-in alternative, no native library)
- [iec61850/iec61850_server](iec61850/iec61850_server/AGENTS.md) — IEC 61850 MMS server driver (Go, drop-in alternative, no native library)
- [telegraf-listener](telegraf-listener/AGENTS.md) — Telegraf UDP JSON listener (Node.js)
- [plc4j-client](plc4j-client/AGENTS.md) — Multi-protocol PLC client on Apache PLC4X/PLC4J: Modbus, S7, ADS, OPC UA, KNXnet/IP, EtherNet/IP (Java)
- [camera-onvif](camera-onvif/AGENTS.md) — ONVIF camera control and streaming (Node.js)
- [cim-ems](cim-ems/AGENTS.md) — CIM/CGMES EMS driver: model exchange, load flow, state estimation, contingency analysis (Python)

### Protocol Drivers (no AGENTS.md, use parent doc)

| Component | Language | Purpose |
|---|---|---|
| libiec61850 | C | libiec61850 C library (submodule) |
| libiec61850-1.6.0 | C | libiec61850 v1.6.0 (submodule) |

### Core Services (no AGENTS.md, use parent doc)

*(All core services now have their own AGENTS.md — see index above)*

### Tools & Utilities

| Component | Language | Purpose |
|---|---|---|
| [oshmi2json](oshmi2json/AGENTS.md) | Node.js | OSHMI point list to JSON-SCADA import converter |
| [config_server_for_excel](config_server_for_excel/AGENTS.md) | Node.js | HTTP server feeding the Excel configuration workbook |
| [oshmi_sync](oshmi_sync/AGENTS.md) | PHP | OSHMI settings sync |
| [inkscape-extension](inkscape-extension/AGENTS.md) | Python | Inkscape SVG SCADA editor extension |
| svg-display-editor | — | SVG display editor (docs) |
| svgedit | JS | Web-based SVG editor (submodule) |
| log-io | Node.js | Log streaming UI |
| [carbone-reports](carbone-reports/AGENTS.md) | Node.js | Carbone report generation example |
| [backup-mongo](backup-mongo/AGENTS.md) | Node.js | Configuration collection export to mongo shell scripts |
| [updateUser](updateUser/AGENTS.md) | Node.js | User creation and password reset utility |
| [alarm_beep](alarm_beep/AGENTS.md) | Node.js | Alarm audio notification |
| [demo_simul](demo_simul/AGENTS.md) | Node.js | Demo data simulator |
| [grafana_alert2event](grafana_alert2event/AGENTS.md) | Node.js | Grafana alert to SCADA event bridge |
| certificate-creator | Bash | TLS cert creation scripts |
| ClassicClientSolutions | C# .NET | OPC Classic client libraries (submodule) |
| ClassicServerSolutions | C# .NET | OPC Classic server libraries (submodule) |
| opcdaaehda-client-solution-net | C# .NET | OPC DA/AE/HDA client .NET Standard |

### Dependencies (no AGENTS.md, use parent doc)

| Component | Type | Purpose |
|---|---|---|
| custom-developments | Templates | AI-assisted custom app templates |
