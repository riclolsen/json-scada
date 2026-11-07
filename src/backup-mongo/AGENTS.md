# DOX: src/backup-mongo — MongoDB Configuration Export Utility

## Purpose

Node.js command-line utility that exports the JSON-SCADA configuration collections to mongo shell scripts (`users`, `realtimeData`, `protocolConnections`, plus an update-only script for operator-editable point fields). It is not a full backup tool.

## Ownership

- backup-mongo owns the configuration export scripts

## Local Contracts

- **Language:** Node.js
- **Main entry:** `backup-mongo.js`
- **Pattern:** standard Node.js app (`app-defs.js`, `load-config.js`, `simple-logger.js`)
- **Config:** JSON-SCADA config file (`../../conf/json-scada.json` or `JS_CONFIG_FILE`)
- **Output:** `output/` in the current working directory

## Work Guidance

- Generates `*_insert.js` scripts with `insertOne` per document and `realtimeData_update.js` with the operator-editable fields only
- Uses the MongoDB Node.js driver directly (no `mongodump`)
- Reads whole collections into memory; run during low activity and protect the output (it contains password hashes)
- Full database backups are out of scope; use `mongodump`/`mongorestore` for those

## Verification

- Run against a test database and restore the generated scripts with `mongosh`
