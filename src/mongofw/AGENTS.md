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
  - `udp-auth.js` — versioned UDP envelope with timestamp, random nonce and HMAC-SHA-256
- **Diagrams:** `jsonscada-onewayrepl.drawio`, `jsonscada-onewayrepl.svg`
- **Config:** JSON-SCADA config file (`JS_CONFIG_FILE`); instance/log level arguments or `JS_MONGOFW_*`
- **UDP key:** raw 32-byte shared file set by `udpAuthKeyFile` or `JS_MONGOFW_UDP_KEY_FILE`

## Work Guidance

- Designed for unidirectional data flow (source → destination) over UDP
- Signs every UDP packet, including backfill packets, at send time
- Reads the initial point list at a MongoDB snapshot timestamp and starts the point change stream at that position. Point snapshots and live changes carry ordered source revisions
- Sends a point-list integrity sync every `INTERVAL_INTEGRITY` seconds and watches inserts, replacements, tag changes, deletions and `sourceDataUpdate` changes
- Anchors the `secEvents` insert stream before starting its replay. Sends the original BSON and replays retained events by `timeTag` within `BACKFILL_REPLAY_INTERVAL` days at activation and periodically
- Stores live values in `backfillData` for historical replay. Records published point IDs in `replicationSourcePoints` before sending them and reconciles missing IDs against the same database snapshot as the point list
- Bounds the live, background and archival queues. The sender gives live data priority while reserving capacity for integrity and replay
- Claims or renews ownership with a majority-acknowledged conditional update to the indexed `processInstances` document
- Runs one replay per collection at a time within each active connection. Losing ownership stops the streams and replays and discards queued UDP work from that active session
- Configures the UDP destination and tuning constants in `app-defs.js`; the receiver is `src/mongowr`
- Suitable for an Ethernet data diode or tap device for physical isolation

## Verification

- `npm install` — dependencies install cleanly
- Test live point and event forwarding plus recent event replay with source and destination MongoDB instances
- Verify no data flows in reverse direction
