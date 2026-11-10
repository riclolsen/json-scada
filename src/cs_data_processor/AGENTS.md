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
  - `commstatus.js` — communication status tags (`_System.Driver/Conn/Comm.*`); port of `../cs_data_processor-go/commstatus.go`, keep tag names, keys, fields, BSON number types (`Double`) and timing identical; contract in `docs/schema.md` (Communication status tags)
  - `commstatus.test.js` — `node:test` suite mirroring `commstatus_test.go`
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
- `npm test` — communication status tag logic (expected values shared with the Go tests)
- Verify Change Stream processing with active protocol drivers producing data
- Check alarm detection with known test data
- Latency: start with `JS_CSDATAPROC_METRICS_PORT` set and read
  `http://localhost:<port>/metrics/text`, or compare against the Go port with
  `node ../cs_data_processor-go/tools/compare-latency.js`
