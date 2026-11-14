# {json:scada} Config Server For Excel

HTTP server that feeds an Excel workbook with the JSON-SCADA configuration
collections and applies point edits made in the workbook back to MongoDB.

The workbook is `json-scada-config.xlsm` (shipped in this directory, and also in
`conf-templates/`). Enable macros in Excel and start this server before opening
it.

Requires Node.js.

## Run

    cd src/config_server_for_excel
    npm install
    node index.js

The server reads `../../conf/json-scada.json`; set `JS_CONFIG_FILE` to use
another file. It binds to `127.0.0.1:10001` by default.

| Environment variable | Default | Description |
|---|---|---|
| `JS_CSEXCEL_IP_BIND` | `127.0.0.1` | Interface to bind |
| `JS_CSEXCEL_HTTP_PORT` | `10001` | TCP port to listen on |
| `JS_CONFIG_FILE` | `../../conf/json-scada.json` | JSON-SCADA config file |

## Endpoints

All endpoints return the fields the workbook needs, sorted by `_id`.

| Method and path | Description |
|---|---|
| `GET /excel/protocolDriverInstances.csv` | Driver instances as CSV |
| `GET /excel/protocolDriverInstances.json` | Driver instances as JSON |
| `GET /excel/connections.csv` | Protocol connections as CSV |
| `GET /excel/connections.json` | Protocol connections as JSON |
| `GET /excel/realtimeData.csv` | Points (excluding `_id <= 0`) as CSV |
| `GET /excel/realtimeData.json` | Points (excluding `_id <= 0`) as JSON |
| `POST /excel/realtimeDataUpdate` | Apply a JSON array of edited points |

The POST body is a JSON array serialized as text. For each element:

- The document is matched by `_id`.
- An empty `tag` deletes the point.
- Otherwise the point is updated, or inserted (`upsert`) when `_id` does not
  exist. New points receive the default runtime fields (`value`, `invalid`,
  timestamps, counters, and so on).

Values are converted to the BSON types expected by `realtimeData`: numeric
fields become `Double` (including `Double` limits where appropriate), booleans
accept `"true"`/`"false"`, and `location`, `parcels` and `protocolDestinations`
accept JSON text.

## Security

The server has no authentication and is meant to be reachable only from the
machine running Excel. Keep the default loopback bind; if the workbook runs on
another host, expose the port through a protected reverse proxy or VPN instead
of binding it to `0.0.0.0`.
