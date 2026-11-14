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
  - `sparkplug-client.js` — Sparkplug B protocol handling
  - `cast.js` — data type casting utilities
  - `auto-tag.js` — automatic tag creation from MQTT topics
  - `device-simul.js` — Sparkplug B device simulator for testing
  - `app-defs.js` — application definitions
  - `load-config.js` — configuration loader
  - `simple-logger.js` — logging utility
  - `redundancy.js` — high-availability support
- **Config:** INI file via Supervisor or environment variables
- **Build:** `npm install` for dependencies
- **Connection stats:** a 5 s loop in `index.js` (active node only) writes `stats.nodeName`, `stats.timeTag` and `stats.isConnected` = broker session connected, with dotted `$set` paths (`updateConnectionStats`: on error 28, a null `stats` is replaced with an empty document and the update retried)

## Work Guidance

- Supports MQTT 3.1.1 and 5.0
- Sparkplug B: birth/death certificates, metrics, data set, template, array/parameter support
- TLS support for MQTT broker connections
- Auto-tag feature creates MongoDB tags from discovered MQTT topics
- Device simulator (`device-simul.js`) useful for testing without real devices

## Verification

- `npm install` — dependencies install cleanly
- `node index.js --help` — CLI arguments work
- Test with a public MQTT broker (e.g., test.mosquitto.org)
- Run device simulator and verify data appears in MongoDB
