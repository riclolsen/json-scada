# DOX: src/mqtt-sparkplug — MQTT / Sparkplug B Client

## Purpose

Node.js MQTT client with Sparkplug B payload support for JSON-SCADA. Bridges MQTT topics and Sparkplug B device metrics to/from the MongoDB real-time database.

## Ownership

- mqtt-sparkplug owns the MQTT and Sparkplug B integration

## Local Contracts

- **Language:** Node.js
- **Main entry:** `index.js`
- **Structure:**
  - `index.js` — main application logic
  - `sparkplug-client.js` — Sparkplug B protocol adapter (MQTT sessions, birth/death, payload decoding)
  - `sparkplug-utils.js` — pure helpers (topic matching, command addresses, path matching, seq math); tested by `sparkplug-utils.test.js`
  - `cast.js` — data type casting utilities
  - `auto-tag.js` — automatic tag creation from MQTT topics
  - `device-simul.js` — Sparkplug B device simulator for testing (uses the `sparkplug-client.js` API)
  - `app-defs.js` — application definitions; `VERSION` matches `package.json` `version`
  - `load-config.js` — configuration loader
  - `simple-logger.js` — logging utility
  - `redundancy.js` — high-availability support
- **Config:** INI file via Supervisor or environment variables
- **Build:** `npm install` for dependencies
- **Dependency override:** `package.json` `overrides` forces `protobufjs` ^8 under `sparkplug-payload` (which pins a vulnerable ^6; it only uses the `protobufjs/minimal` runtime with pre-generated code). After changing either package, confirm encode/decode output is unchanged and `npm audit` is clean
- **Connection stats:** a 5 s loop in `index.js` (active node only) writes `stats.nodeName`, `stats.timeTag` and `stats.isConnected` = broker session connected, with dotted `$set` paths (`updateConnectionStats`: on error 28, a null `stats` is replaced with an empty document and the update retried)
- **Broker sessions:** one MQTT client per session, recreated by the main loop (mqtt.js auto-reconnect is off); 3 consecutive failed sessions rotate to the next `endpointURLs` entry. `bdSeq` increments per session and matches the NDEATH will (QoS 1, only when `groupId` and `edgeNodeId` are set) and the NBIRTH; the last used `bdSeq` is stored as `sparkplugBdSeq` in the `protocolDriverInstances` document, so it continues across restarts and failovers. A clean stop publishes the NDEATH (STATE sessions: OFFLINE) explicitly and ends the session after the ack or 3 s, never reconnecting
- **Listener errors:** the adapter's MQTT `message` handler catches every exception (including those thrown by application listeners), so one bad message never stalls the mqtt.js session; `index.js` also guards each message handler
- **TLS:** the `allowTLSv1x` range (default TLS 1.2–1.3) applies to every TLS connection, also an `mqtts://` URL without `useSecurity`; chain validation is on unless `chainValidation` is false
- **Log level:** argument or environment; a higher `logLevel` in the driver instance document raises it at run time
- **Host STATE:** with `scadaHostId`, two extra MQTT sessions publish retained STATE in Sparkplug 3.0 (`spBv1.0/STATE/<id>`, JSON) and 2.2 (`STATE/<id>`) formats, each with its own OFFLINE will, and republish ONLINE when they see themselves reported offline
- **Edge node publishing:** the DBIRTH lists `realtimeData` tags of other connections (never this connection's own); DDATA carries only aliases (`_id`) declared in the last DBIRTH, and a new tag triggers a full rebirth at most every `SECONDS_BETWEEN_REBIRTHS`. NBIRTH and DBIRTH go out back-to-back (`publishBirths`); a DBIRTH above `DBIRTH_SIZE_WARNING` bytes logs a warning. Changes seen while the first births are built are published right after them. A `Node Control/Rebirth` within `SECONDS_BETWEEN_REBIRTHS` of the last birth is answered by one deferred birth at the end of that time. Commands (NCMD/DCMD) sent as host carry no edge `seq`. `Node Control/Reboot` and `Next Server` are offered and honoured only with `commandsEnabled`
- **Receiving:** datatypes/names missing from DATA and CMD metrics are restored from the last BIRTH (adapter registry, one per MQTT session); each new session marks every other node unbirthed, so their first data requests a rebirth, and DATA metrics still untyped also request one; per edge node a `seq` gap requests a rebirth (throttled by `SECONDS_BETWEEN_NODE_REQUESTS`, data still processed); an NDEATH with a `bdSeq` different from the NBIRTH is ignored; node/device paths match exactly or by `path/` prefix, never by bare string prefix. `isHistorical`/`isNotForHistorical` are passed in `sourceDataUpdate` for the data processor
- **MongoDB writes:** queued values are written by one pass at a time as an ordered `bulkWrite` of `sourceDataUpdate`; every fire-and-forget promise has a `.catch`. A dropped MongoDB client is closed together with its change streams, and events from a replaced client are ignored
- **Routed commands (received DCMD):** refused when the metric has no value (null) or when `commandBlocked === true` on the command tag or its supervised tag
- **Plain topics:** a JSONPath that matches nothing in the payload leaves the tag unchanged. OPC UA PubSub (`ua-data`) Uncertain/Bad or malformed status codes mark values invalid; invalid timestamps fall back to the receive time
- **Auto-tag keys:** a duplicate `_id` on insert (tags created by the other redundant node) refreshes the key from the database and retries
- **Scripts (`topicsScripted`):** run in a Node.js `vm` context with no host objects, 1 s timeout per run; `shared.payload` is a UTF-8 string, `shared.payloadBytes` a Uint8Array; results are copied out as JSON. Not a security boundary: scripts are trusted admin configuration
- **Unexpected errors:** `uncaughtException`/`unhandledRejection` are logged with stack and the process keeps running (a crash loop could be triggered remotely by a retained message)

## Work Guidance

- MQTT 3.1.1 only
- Sparkplug B: birth/death certificates, metrics, data set, template, Bytes/File (base64 string tags); array datatypes are not decoded by `sparkplug-payload` and are discarded with a log line; `sparkplug-payload` 1.0.3 decodes DataSet columns only of UInt32, Int64, Float, Double, Boolean and String types, so the adapter rewrites other columns before decoding (Int8–UInt32 → Double, UInt64/DateTime → Int64, Text/UUID → String)
- TLS support for MQTT broker connections
- Auto-tag feature creates MongoDB tags from discovered MQTT topics
- Device simulator (`device-simul.js`) useful for testing without real devices
- Keep pure, testable logic in `sparkplug-utils.js` with tests in `sparkplug-utils.test.js`

## Verification

- `npm install` — dependencies install cleanly
- `npm test` — unit tests of `sparkplug-utils.js`
- `node --check` on each changed `.js` file
- End-to-end: run against a broker and a MongoDB replica set (change streams) with `node index.js 1 2 <config>`; run the device simulator and verify data appears in MongoDB
