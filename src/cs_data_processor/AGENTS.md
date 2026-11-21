# DOX: src/cs_data_processor — Change Stream Data Processor

## Purpose

Node.js service that processes MongoDB Change Stream events for alarm detection, event logging, and data transformation. The core event processing pipeline for SCADA data.

## Ownership

- cs_data_processor owns the change stream data processing pipeline
- `src/cs_data_processor-go` is a drop-in Go port of this service; only one of
  the two may run per instance number. Keep them behaviourally identical.

## Local Contracts

- **Language:** Node.js
- **Main entry:** `cs_data_processor.js`
- **Structure:**
  - `cs_data_processor.js` — main processing logic
  - `app-defs.js` — application definitions
  - `load-config.js` — configuration loader
  - `simple-logger.js` — logging utility
  - `redundancy.js` — high-availability support
  - `metrics.js` — latency instrumentation (histograms, counters, HTTP endpoint)
  - `commstatus.js` — communication status tags (`_System.Driver/Conn/Comm.*`); port of `../cs_data_processor-go/commstatus.go`, keep tag names, keys, fields, BSON number types (`Double`) and timing identical; the connection name comes from `protocolConnections.name`; contract in `docs/schema.md` (Communication status tags); the MCP server (`src/mcp-json-scada-db/src/jsonscada/system-tags.ts`) mirrors the names, points, keys and owned fields to explain and check these tags, so change it together
  - `commstatus.test.js` — `node:test` suite mirroring `commstatus_test.go`
  - `limits.js` — out-of-range alarm levels of analog tags (`alarmRange` -3..3 from `lololoLimit`…`hihihiLimit`, hysteresis per level, escalation as a new alarm, level label in the SOE text); port of `../cs_data_processor-go/limits.go`, change both together; contract in `docs/schema.md` (`hysteresis`, `alarmRange`)
  - `limits.test.js` — `node:test` suite mirroring `limits_test.go` (same cases and expected values)
  - `alarmlist.js` — alarm list (`alarmsData`, one document per alarm occurrence): a new condition of a point (analog `alarmRange` change, digital state change or alarm) first ends the point's earlier active alarms (`active: false`, `timeTagNormal`), then inserts the new alarm, through one ordered writer (`AlarmWriter`, ordered `bulkWrite`); `realtimeData.alarmed` stays the summary flag (true while the point has unacknowledged alarms); port of `../cs_data_processor-go/alarmlist.go`, change both together, same field order; contract in `docs/schema.md` (alarmsData); acknowledgements belong to `server_realtime_auth`
  - `retention.js` — daily data retention (instance 1, active node): deletes `soeData` and acknowledged inactive `alarmsData` older than `soeRetentionDays` / `alarmsRetentionDays`, sets the `hist` `expireAfterSeconds` to `histRetentionDays` (systemSettings `{ "key": "global" }`, default 60 days, limits equal to `server_realtime_auth` `services/system-settings.js`), creates the `alarmsData` indexes (also in `mongo_seed/b_create-db.js`) and converts a capped `soeData` once (rename, copy back, drop: change streams on `soeData` must reopen); port of `../cs_data_processor-go/retention.go`
  - `alarmlist.test.js`, `retention.test.js` — `node:test` suites mirroring `alarmlist_test.go` and `retention_test.go`
  - SQL files for PostgreSQL (`../../sql/pg_hist_*.sql`, `pg_rtdata_*.sql`) are written as `*.tmp` and renamed by `writeSqlFile()`; `src/process_pg_sql` loads them, so keep the name pattern in step with it and `../cs_data_processor-go/writers.go`
- **Config:** INI file via Supervisor or environment variables

## Work Guidance

- Subscribes to MongoDB Change Streams on protocol data collections
- Detects alarm conditions based on configurable thresholds
- Logs events to the SOE (Sequence of Events) collection
- Can transform data format between protocol and storage schemas
- Designed for low-latency event processing
- Latency is measured per stage by `metrics.js`; the stage names, counter names
  and JSON layout must stay identical to `../cs_data_processor-go/metrics.go`,
  otherwise `../cs_data_processor-go/tools/compare-latency.js` breaks

## Verification

- `npm install` — dependencies install cleanly
- `node --check cs_data_processor.js && node --check metrics.js`
- `npm test` — communication status tag logic, alarm limit levels, alarm list documents and retention settings (expected values shared with the Go tests)
- Verify Change Stream processing with active protocol drivers producing data
- Check alarm detection with known test data
- Latency: start with `JS_CSDATAPROC_METRICS_PORT` set and read
  `http://localhost:<port>/metrics/text`, or compare against the Go port with
  `node ../cs_data_processor-go/tools/compare-latency.js`
