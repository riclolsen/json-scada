# DOX: src/mongofw — MongoDB Firewall / Forwarder

## Purpose

One-way data replication firewall/forwarder for air-gapped environments. Provides secure real-time data replication across network boundaries (e.g., via data diode or tap device).

## Ownership

- mongofw owns the one-way replication infrastructure

## Local Contracts

- **Language:** Node.js
- **Main entry:** `index.js`
- **Structure:**
  - `index.js` — main application logic
  - `app-defs.js` — network, packet and backfill parameters (`IP_DESTINATION`, `UDP_PORT`, `BACKFILL_*`)
  - `load-config.js` — configuration loader
  - `simple-logger.js` — logging utility
  - `redundancy.js` — high-availability support
  - `customized_module.js` — user-customizable processing module
- **Diagrams:** `jsonscada-onewayrepl.drawio`, `jsonscada-onewayrepl.svg`
- **Config:** JSON-SCADA config file (`JS_CONFIG_FILE`); instance/log level arguments or `JS_MONGOFW_*`

## Work Guidance

- Designed for unidirectional data flow (source → destination) over UDP
- Watches `realtimeData` for `sourceDataUpdate` changes (change streams require a replica set), compresses and broadcasts them
- Sends a full point-list integrity sync every `INTERVAL_INTEGRITY` seconds
- Stores forwarded changes in the `backfillData` time series collection for `BACKFILL_EXPIRATION` days and replays them so an offline receiver catches up
- Configures the UDP destination and tuning constants in `app-defs.js`; the receiver is `src/mongowr`
- Suitable for an Ethernet data diode or tap device for physical isolation

## Verification

- `npm install` — dependencies install cleanly
- Test with source and destination MongoDB instances
- Verify no data flows in reverse direction
