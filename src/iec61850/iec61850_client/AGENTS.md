# DOX: src/iec61850/iec61850_client — IEC 61850 MMS Client Driver (Go)

## Purpose

Pure-Go IEC 61850 MMS client driver for JSON-SCADA. Drop-in alternative to the C# driver in
`src/iec61850_client`: same `protocolDriver` name (`IEC61850`), same configuration documents, same
MongoDB semantics, no native library dependency.

## Ownership

- iec61850/iec61850_client owns the Go implementation of the IEC 61850 MMS client driver
- `src/iec61850_client` (C#) remains the reference implementation; behaviour is matched to it

## Local Contracts

- **Language:** Go 1.26, module `iec61850_client`, flat `package main`
- **Library:** `github.com/dscsystems/go-iec61850` v0.2.5 (pure Go, GPLv3) — **pin the version**,
  the API is pre-v1
- **Binary:** `iec61850-client(.exe)` — must differ from the C# `iec61850_client(.exe)` so both can
  live in `bin/`
- **Files** (one per C# file, to keep them diffable):
  - `main.go` — startup, instance and connection loading (← `Main.cs`)
  - `config.go` — documents, permissive BSON decoding, MongoDB connect (← `Common_srv_cli.cs`)
  - `connection.go` — per-IED state machine and polling (← `Process()` in `AsduReceiveHandler.cs`)
  - `discovery.go` — ACSI browse of devices, data sets and report control blocks, and the
    registration of browsed points when autoCreateTags is on
  - `reports.go` — report control block activation, RptID hygiene
  - `report_handler.go` — report reception and value extraction (← `reportHandler`)
  - `mmsconv.go` — MMS value conversions (← the `MMSGet*` helpers)
  - `mongo_update.go` — acquired-value queue and bulk writer (← `MongoUpdate.cs`)
  - `tags_creation.go` — automatic tag documents (← `TagsCreation.cs`)
  - `mongo_commands.go` — command change stream and dispatch (← `MongoCommands.cs`)
  - `redundancy.go` — active/standby arbitration (← `Redundancy.cs`)
  - `tlsconf.go` — TLS configuration
- **Build:** `go build -o ../../../bin/iec61850-client`
- **Config:** `conf/json-scada.json` plus MongoDB documents; CLI `<instance> <logLevel> <configFile>`

## Work Guidance

- The C# driver is the specification. Before changing behaviour, check what
  `src/iec61850_client` does; quirks are reproduced on purpose and are marked `parity:` in
  comments. Intentional differences are numbered D1..D11 and listed in `README.md` — add to that
  list rather than silently diverging.
- Only `sourceDataUpdate` is written for data; never tag `value`, alarms or history.
- `Iec61850Entry.AutoPublish` marks a point the driver discovered itself (browse or report); only those carry the self-publish flag, so a point configured in realtimeData never gets a second tag.
- Command tags are created by the MongoDB writer, not the value path: a control object carries no value, so `createCommandTags` inserts it and links it to its supervised twin (`supervisedOfCommand` / `commandOfSupervised`). It waits for the twin to exist, up to `commandLinkAttempts` writer cycles.
- All numbers written to MongoDB must be Go `float64` so they land as BSON doubles.
- Report callbacks run on the association's reader goroutine: never block them, only enqueue.
- Report entries are identified from the data set members, and reports are matched to their
  subscription by `RptID` — see the RptID handling in `reports.go` before touching that path.

## Verification

- `go test ./...` — conversions, tag documents, and a loopback run against an in-process IEC 61850
  server from `testdata/simpleIO_direct_control.cid` (no MongoDB or device needed)
- `go vet ./...`
- `go list -deps ./... | grep charm` must be empty — the library's TUI dependencies must not be
  linked in
- End-to-end: seed an instance and connection, run against a real IED or the C# driver's usual
  device, and diff the resulting `realtimeData` documents against the C# driver's
