# DOX: src/cs_data_processor-go — Change Stream Data Processor (Go)

## Purpose

Go port of `src/cs_data_processor`. Processes MongoDB Change Stream events for
value conversion, alarm detection, SOE logging and historian feeding. Same
process name, arguments, environment prefix and outputs as the Node.js
version, tuned for lower change-stream processing latency.

## Ownership

- Shares the change stream data processing pipeline with `src/cs_data_processor`.
- Exactly one of the two may run per instance number — both register as
  `CS_DATA_PROCESSOR` in `processInstances`.

## Local Contracts

- **Language:** Go (no cgo), MongoDB driver `go.mongodb.org/mongo-driver/v2`
- **Main entry:** `main.go`, binary `cs_data_processor[.exe]`
- **Arguments:** `instance logLevel [configFile]` (same as Node.js)
- **Env prefix:** `JS_CSDATAPROC_` (see README for the full table)
- **Structure:**
  - `main.go` — connection lifecycle, change stream, resume tokens
  - `process.go` — conversion logic, channels, sharded worker pool
  - `writers.go` — batching writers (realtimeData, hist + SQL files, soeData)
  - `metrics.go`, `hrtime*.go` — latency instrumentation
  - `redundancy.go`, `maintenance.go`, `specialtags.go`
  - `commstatus.go` — communication status tags (`_System.Driver.*`, `_System.Conn.*`, `_System.Comm.*`); port of the same logic in `../cs_data_processor/commstatus.js` — change both together, contract in `docs/schema.md` (Communication status tags)
  - `rawdoc.go`, `jsutil.go` — raw BSON access and JavaScript-compatible rendering
  - `config.go` — config file and env vars
  - `tools/compare-latency.js` — compares two metrics snapshots

## Work Guidance

- **Parity first.** Any change to the conversion must keep the same observable
  output as `../cs_data_processor/cs_data_processor.js`. The JavaScript
  semantics the original relies on (loose `!=` against `undefined`,
  truthiness, `Number` formatting, `JSON.stringify` field order) are
  reproduced in `jsutil.go` and `rawdoc.go` — do not "clean them up".
- Changing the metric names, stages or JSON layout in `metrics.go` requires
  the same change in `../cs_data_processor/metrics.js`, otherwise the
  comparison tool breaks.
- Queues are channels; do not reintroduce polling loops. Writers batch on
  size **or** linger, whichever comes first.
- Per-point ordering is guaranteed by hashing `fullDocument._id` to a worker
  inbox. Any new dispatch must preserve that.
- Communication status tags: only instance 1 on the active node writes them,
  always through `sourceDataUpdate` (never `$set` of value fields), and
  identifies them by the `commStatus` field, never by tag name. Liveness uses
  the unchanged-reading count, not the local clock (the first reading alone
  uses a 30 s grace). Tag names, points or keys are a display contract:
  changing them breaks SVG bindings, so update `docs/schema.md` with them.
- Wall-clock stages (`sourceToRecv`, `endToEnd`) must stay on `time.Now()`;
  in-process stages must use `hrNow()`.

## Verification

- `go vet ./... && go test ./...` — unit tests cover the JavaScript-compatible
  rendering (values cross-checked against real Node.js output), the histogram,
  and the conversion for digital/analog/double-point/bitstring/beep/SOE/
  dead-band/standby/backfill cases.
- `gofmt -l .` must print nothing.
- Communication status tags end to end: a throwaway replica-set `mongod`
  seeded with two driver instances (one with an advancing keep-alive) and
  connections, run the binary with a temp config; check creation, rename on
  connection rename, removal on connection delete, `Alive` false ~25 s after
  the keep-alive stops, SOE entries for `Status`/`Alive`, and invalidation
  with `commStatusTags: false`. Parity: seed two databases identically, run
  one implementation against each, and compare the `commStatus` tag documents
  (raw BSON types, without time fields) and `soeData` — they must be equal.
- End-to-end parity: run both implementations against the same single-node
  replica set with a race-free load (one update round every few hundred ms,
  so `updateLookup` never races the processor), then diff the resulting
  `realtimeData` / `soeData` / `hist` documents and the generated
  `pg_hist_*.sql` / `pg_rtdata_*.sql` rows with timestamps normalised. They
  must be identical.
- Latency: `node tools/compare-latency.js --reset --wait 120 http://localhost:8081 http://localhost:8082`.
