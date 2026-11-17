# DOX: src/OPC-UA-Server — OPC UA Server Driver

## Purpose

Node.js OPC UA server that exposes JSON-SCADA real-time data as OPC UA variables. Allows external OPC UA clients (like SCADA systems, HMIs) to read/write JSON-SCADA tags.

## Ownership

- OPC-UA-Server owns the OPC UA server implementation

## Local Contracts

- **Language:** Node.js
- **Structure:**
  - `index.js` — main application logic
  - `historian.js` — OPC UA Historical Access: backend abstraction (MongoDB `hist` timeseries / PostgreSQL-TimescaleDB `hist` hypertable), `JsonScadaVariableHistorian` (IVariableHistorian, read-only — cs_data_processor writes history), `convertHistValue` (scalar value conversion shared with `index.js` `convertValueVariant`), `installTagHistory` (full + "lite" install)
  - `app-defs.js` — application definitions
  - `load-config.js` — configuration loader
  - `simple-logger.js` — logging utility
  - `test-history.js` — self-test (in-process server+client, mock backend, no DB): HistoryRead raw/reverse/cap/continuation, integer conversion, async write status
- **Config:** `../../conf/json-scada.json` (override `JS_CONFIG_FILE`); instance/log level via arguments or `JS_OPCUASERVER_*` environment variables; connection settings in `protocolConnections`
- **Security:** `useSecurity` removes the unsecured endpoint (only Sign/SignAndEncrypt); optional `users` list enables username/password login and turns anonymous access off unless `allowAnonymous` is true; `ipAddresses` is enforced at channel level (fail closed on unknown address)
- **Commands:** only `origin: command` variables are writable; the write is async and returns the real status (never a blind `Good`)
- **History:** opt-in (`historyEnabled` defaults to false); results cut off at `historyMaxReturnDataValues` return `GoodMoreData`; reverse reads query newest-first
- **Startup:** the server listens only after the address space is built; a start failure (e.g. port in use) drops the Mongo client so the main loop retries
- **Keep-alive and stats:** no redundancy arbitration, so a 5 s loop in `index.js` updates the instance `activeNodeKeepAliveTimeTag`/`activeNodeName`/`softwareVersion` and each connection's `stats.isConnected` (at least one client session) and `stats.sessions`; with the same instance on several nodes all write

## Work Guidance

- Creates an OPC UA server that exposes MongoDB real-time data as OPC UA variables
- External OPC UA clients connect and browse/subscribe to tags
- Supports read, write, and subscription (monitored items)
- Supports OPC UA Historical Access (HistoryRead: raw + aggregates) when `historyEnabled` is set on the connection; read-only, backed by the `hist` store (see `historian.js`)
- TLS/security configuration via OPC UA certificates

## Verification

- `npm install` — dependencies install cleanly (includes `pg` and `node-opcua-aggregates` for history)
- `npm test` / `node test-history.js` — self-test (no DB required); expect `ALL PASS`
- Test connection with an OPC UA client (e.g., UaExpert)
- Verify tag browsing and data subscription work; for history, use UaExpert's History Trend View against a historized tag
