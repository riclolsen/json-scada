# DOX: src/config_server_for_excel — Excel Configuration Server

## Purpose

HTTP server that feeds the `json-scada-config.xlsm` workbook with JSON-SCADA configuration collections (driver instances, connections, points) and applies edited points back to MongoDB.

## Ownership

- config_server_for_excel owns the Excel configuration bridge

## Local Contracts

- **Language:** Node.js
- **Main entry:** `index.js`
- **Pattern:** standard Node.js app (`app-defs.js`, `load-config.js`, `simple-logger.js`)
- **Config:** JSON-SCADA config file (`JS_CONFIG_FILE`)
- **Bind:** `JS_CSEXCEL_IP_BIND` / `JS_CSEXCEL_HTTP_PORT` (default `127.0.0.1:10001`)
- **Workbook:** `json-scada-config.xlsm` (a copy also lives in `conf-templates/`)

## Work Guidance

- Read endpoints export CSV/JSON for `protocolDriverInstances`, `protocolConnections` and `realtimeData`
- `POST /excel/realtimeDataUpdate` receives a JSON array as text; empty `tag` deletes, otherwise upsert by `_id`
- Numeric inputs are converted to BSON `Double`; JSON-encoded fields accept text
- No authentication: keep the loopback bind or protect the port with a reverse proxy
- `index.js` contains the endpoint list and field projections; `app-defs.js` holds port/bind defaults

## Verification

- `npm install` — dependencies install cleanly
- Start the server and request `/excel/realtimeData.json`; post a small edit array and confirm the point changes
