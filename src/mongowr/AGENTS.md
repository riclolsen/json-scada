# DOX: src/mongowr — MongoDB Writer

## Purpose

One-way data replication receiver for air-gapped environments. Receives compressed UDP packets from `mongofw` across a network boundary (e.g., via data diode or tap device) and writes them to the local MongoDB database.

## Ownership

- mongowr owns the MongoDB write path from received data packets.

## Local Contracts

- **Language:** Node.js
- **Main entry:** `index.js`
- **Structure:**
  - `index.js` — main application logic
  - `app-defs.js` — network and queue parameters (`IP_BIND`, `UDP_PORT`, `MAX_QUEUE`, `MAX_MSG_SEQ`)
  - `load-config.js` — configuration loader
  - `simple-logger.js` — logging utility
  - `redundancy.js` — high-availability support
  - `customized_module.js` — user-customizable processing module
- **Config:** JSON-SCADA config file (`JS_CONFIG_FILE`); instance/log level arguments or `JS_MONGOWR_*`

## Work Guidance

- Listens on UDP (default `0.0.0.0:12345`), inflates each packet and applies the changes to `realtimeData`
- Applies integrity (point-list) packets with upserts; real-time changes are bulk-written
- Tracks message sequence numbers and logs losses; clears the queue when `MAX_QUEUE` is exceeded
- There is no reverse path: commands are never sent back to the source
- The `customized_module.js` allows users to inject custom processing logic
- Counterpart of the sender: `src/mongofw`

## Verification

- `npm install` — dependencies install cleanly
- Run `mongofw` against a test database and verify points and values appear on the receiver
