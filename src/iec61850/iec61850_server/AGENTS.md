# DOX: src/iec61850/iec61850_server — IEC 61850 MMS Server Driver (Go)

## Purpose

Pure-Go IEC 61850 MMS server driver (IEC 61850-90-2 gateway) for JSON-SCADA. Drop-in alternative to
the C# driver in `src/iec61850_server`: same `protocolDriver` name (`IEC61850_SERVER`), same
connection document, same model layout and object references, no native library dependency.

## Ownership

- iec61850/iec61850_server owns the Go implementation of the IEC 61850 MMS server driver
- `src/iec61850_server` (C#) remains the reference implementation; behaviour is matched to it
- `../iec61850_client` is the Go client driver and the natural loopback partner

## Local Contracts

- **Language:** Go 1.26, module `iec61850_server`, flat `package main`
- **Library:** `github.com/dscsystems/go-iec61850` v0.2.5 (pure Go, GPLv3) — **pin the version**,
  the API is pre-v1, and keep it in step with the client driver
- **Binary:** `iec61850-server(.exe)` — must differ from the C# `iec61850_server(.exe)` so both can
  live in `bin/`
- **Files** (one per C# file, to keep them diffable):
  - `main.go` — startup, instance/connection loading, the 1 Hz lifecycle loop (← `Main.cs`)
  - `config.go` — documents, permissive BSON decoding, MongoDB connect (← `Common_srv_cli.cs`)
  - `points.go` — the realtimeData selection query
  - `cdc.go` — common data class wrappers over `model.NewDataObject`, plus `LastApplError`
  - `model_builder.go` — LD/LN/GGIO layout, sizing bounds, data sets, report controls (← `ModelBuilder.cs`)
  - `manifest.go` — the tag → object reference manifest
  - `server.go` — server creation, connection events, allow-list, start/stop
  - `change_stream.go` — realtimeData watch (← `MongoChangeStream.cs`)
  - `update_loop.go` — batched model updates and quality mapping
  - `controls.go` — control handler and the commandsQueue inserter (← `ControlHandlers.cs`)
  - `stats.go` — instance keep-alive and connection statistics (no active/standby arbitration)
  - `tlsconf.go` — server-side TLS
  - `selftest.go` — synthetic model, no MongoDB (← `SelfTest.cs`)
- **Build:** `go build -o ../../../bin/iec61850-server`
- **Config:** `conf/json-scada.json` plus MongoDB documents; CLI `<instance> <logLevel> <configFile>`

## Work Guidance

- The C# driver is the specification. Object references, data set composition and report control
  block names must stay identical, so a client cannot tell the two apart. Intentional differences
  are numbered D2..D9 in `README.md` — add to that list rather than silently diverging.
- Data sets use **DO-level FCDAs** (`GGIO1$ST$Ind1`), so a report entry carries value, quality and
  timestamp as one structure. This needs library ≥ v0.2.3, where `reporting.go:memberChanged`
  matches a member against changes below it as well as above.
- The model is built once and `server.New` is called once: it materialises report control blocks
  into the model, so a second call would duplicate them. Start/stop cycles reuse the same server.
- The node is always active: an MMS server is a passive listener, so there is no active/standby
  arbitration. The 1 Hz loop in `main.go` starts the server and retries the bind if it fails.
- The control handler runs on the connection's goroutine: it must not block, so commands are queued
  and inserted by a separate goroutine.
- Values are only ever written through `Update`, which is what drives reporting.

## Verification

- `go test ./...` — model builder, update path, command documents, and a loopback run against the
  library's own client (no MongoDB or device needed)
- `go vet ./...` and `go test -race ./...`
- `go list -deps ./... | grep charm` must be empty — the library's TUI dependencies must not be linked
- Independent check: `./iec61850-server selftest 10102` then, from the library module,
  `go run ./cmd/ied-client -addr 127.0.0.1:10102 test` — every applicable check must pass
- End-to-end: seed a connection, run against MongoDB, and browse with the Go client driver
