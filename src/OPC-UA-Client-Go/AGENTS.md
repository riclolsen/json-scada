# DOX: src/OPC-UA-Client-Go — OPC UA Client Driver (Go)

## Purpose

Pure-Go OPC UA client driver for JSON-SCADA. Drop-in alternative to the C# driver in
`src/OPC-UA-Client`: same `protocolDriver` name (`OPC-UA`), same configuration documents, same
MongoDB semantics, no .NET runtime dependency.

## Ownership

- OPC-UA-Client-Go owns the Go implementation of the OPC UA client driver
- `src/OPC-UA-Client` (C#) remains the reference implementation; behaviour is matched to it

## Local Contracts

- **Language:** Go 1.26, module `opcua-client`, flat `package main`
- **Library:** `github.com/gopcua/opcua` v0.9.1 (MIT, pure Go) — **pin the version**, the API is
  pre-v1. Plus `software.sslmate.com/src/go-pkcs12` for `.pfx` files.
- **Binary:** `opcua-client(.exe)` — must differ from the C# `OPC-UA-Client(.exe)` so both can
  live in `bin/`. Do not rename it to anything that collides case-insensitively on Windows.
- **Files** (one per C# file where possible, to keep them diffable):
  - `main.go` — startup, instance and connection loading, tag preload (← `Program.cs`)
  - `config.go` — documents, permissive BSON decoding, MongoDB connect (← `Common_srv_cli.cs`)
  - `log.go` — the `[timestamp] message` logger (← `Log()` in `Common_srv_cli.cs`)
  - `security.go` — certificates, PKCS#12/PEM loading, self-signed generation, the UA XML subset
  - `session.go` — endpoint discovery and selection, client options, connect/retry loop
    (← `ConsoleClient()`)
  - `browse.go` — address space walk (← `BrowseFullAddressSpaceAsync`)
  - `autotag.go` — the autoCreateTags read pass, browse-path splitting
  - `subscribe.go` — subscriptions and the notification pump (← `OnNotification`)
  - `uaconv.go` — value conversion (← `ConvertOpcValue`)
  - `mongo_update.go` — acquired-value queue and bulk writer (← `MongoUpdate.cs`)
  - `tags_creation.go` — automatic tag documents (← `TagsCreation.cs`)
  - `mongo_commands.go` — command change stream and dispatch (← `MongoCommands.cs`)
  - `redundancy.go` — active/standby arbitration (← `Redundancy.cs`)
- **Build:** `go build -ldflags="-s -w" -o ../../bin/opcua-client`
- **Config:** `conf/json-scada.json` plus MongoDB documents; CLI `<instance> <logLevel> <configFile>`

## Work Guidance

- The C# driver is the specification. Before changing behaviour, check what
  `src/OPC-UA-Client` does; quirks are reproduced on purpose and marked `parity:` in comments.
  Intentional differences are numbered D1..D15 and listed in `README.md` — add to that list
  rather than silently diverging.
- Only `sourceDataUpdate` is written for data; never tag `value`, alarms or history.
- All numbers written to MongoDB must be Go `float64` so they land as BSON doubles.
- A subscription notification carries only a `ClientHandle`, never the node id. The driver owns
  the handle → item map (`OPCUAConnection.handles`); for a preconfigured tag the item's `NodeID`
  must be the **verbatim** `protocolSourceObjectAddress` string, never a re-rendered `ua.NodeID`,
  or the update filter stops matching and the tag silently goes stale.
- `gopcua` option order matters: the `Auth*` options create the user identity token and
  `SecurityFromEndpoint` only fills in its `PolicyID`, so `SecurityFromEndpoint` must come last.
  Reversed, every connection silently authenticates anonymously.
- Never read connection state through `opcua.StateChangedCh`: the client sends to that channel
  synchronously and a slow reader stalls the client. `StateChangedFunc` may only log.
- `ua.StatusCode` has no `IsGood`; use `statusIsGood` (severity bits 30-31).
- The notification pump must never block — it only converts and enqueues.

## Verification

- `go test ./...` — conversions, tag documents, browse paths, command conversions, and loopback
  runs (browse, autotag, subscribe, write) against an in-process gopcua server. No MongoDB or
  device needed.
- `go vet ./...` and `gofmt -l .` must be clean.
- End-to-end: seed an instance and connection, run against a real server, and diff the resulting
  `realtimeData` documents against the C# driver's, allowing for the deviations in `README.md`.

## Known gaps in the test harness

The in-process gopcua server is not a full OPC UA server. It does **not**:

- link custom namespaces under the standard `ns=0` Objects folder, so a browse from
  `ObjectsFolder` cannot reach them — tests browse the namespace's own Objects node instead;
- implement the `Call` service (`MethodService.Call` returns `BadServiceUnsupported`), so the
  method command path can only be verified against real equipment;
- synthesize inverse hierarchical references — the test tree adds them explicitly;
- allocate unique subscription ids across sessions (`len(subs)+1`), which can make a second
  subscription fail with `BadSubscriptionIDInvalid` when reusing a long-lived server process.

None of these are driver defects; do not "fix" the driver to work around them.
