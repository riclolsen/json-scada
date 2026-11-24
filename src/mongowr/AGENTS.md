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
  - `udp-auth.js` — verifies the UDP envelope and packet freshness
- **Config:** JSON-SCADA config file (`JS_CONFIG_FILE`); instance/log level arguments or `JS_MONGOWR_*`
- **UDP key:** raw 32-byte shared file set by `udpAuthKeyFile` or `JS_MONGOWR_UDP_KEY_FILE`

## Work Guidance

- Listens on UDP (default `0.0.0.0:12345`) and inflates packets containing `realtimeData` changes and `secEvents` documents
- Accepts only HMAC-verified packets less than 60 seconds old and up to 5 seconds ahead. Journals the nonce and compressed payload together with majority write concern before decompressing
- Recovers pending `replicationUdpReceipts` before accepting more work and expires completed receipts via TTL. The writer account needs read/write and index creation access for the collection
- Persists source revisions and deletions in `replicationPointRevisions`. Holds a live update that arrives before its initial integrity record and applies it when the point becomes available
- Claims or renews ownership with a majority-acknowledged conditional update to the indexed `processInstances` document
- Inserts `secEvents` documents with their original BSON types and ObjectIds. Upserts by `_id` set fields only on insert, so repeated packets and replay preserve the stored event
- Integrity packets map the source point ID to a destination document in the indexed `replicatedSourceId` field. An existing tag can adopt the mapping while retaining its local `_id`
- Applies explicit deletions only to mapped points. The writer needs permission to create the sparse unique index on `replicatedSourceId`
- Applies MongoDB writes in arrival order, retains the failed tail for retry, and logs permanent per-operation failures before continuing past them
- Tracks sequence gaps and clears queued UDP packets when `MAX_QUEUE` is exceeded. A batch awaiting a MongoDB result is held separately
- There is no reverse path: commands are never sent back to the source
- The `customized_module.js` allows users to inject custom processing logic
- Counterpart of the sender: `src/mongofw`

## Verification

- `npm install` — dependencies install cleanly
- `npm test` — check sender/receiver envelope compatibility, signature validation and freshness
- Run `mongofw` against a test database and verify points, values and events appear on the receiver. Check BSON types and confirm that replay leaves the existing event intact
