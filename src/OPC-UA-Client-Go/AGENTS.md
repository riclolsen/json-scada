# DOX: src/OPC-UA-Client-Go — OPC UA Client Driver (Go)

## Purpose

Pure-Go OPC UA client driver for JSON-SCADA. Drop-in alternative to the C# driver in
`src/OPC-UA-Client`: same `protocolDriver` name (`OPC-UA`), same configuration documents, same
MongoDB semantics, no .NET runtime dependency.

## Ownership

- OPC-UA-Client-Go owns the Go implementation of the OPC UA client driver
- `src/OPC-UA-Client` (C#) remains the reference implementation; behaviour is matched to it
- `src/go-common` owns the shared plumbing (config, logging, MongoDB access, redundancy, tag and
  `sourceDataUpdate` defaults); this driver owns everything OPC UA specific
- `README.md` is the **user guide**, complete without any other document; `DEVIATIONS.md` is the
  developer reference for how this driver differs from the C# one and how that was verified

## Local Contracts

- **Language:** Go 1.26, module `opcua-client`, flat `package main`
- **Library:** `github.com/gopcua/opcua` v0.9.1 (MIT, pure Go) — **pin the version**, the API is
  pre-v1. Plus `software.sslmate.com/src/go-pkcs12` for `.pfx` files, and the local
  `src/go-common` module (a `replace` in `go.mod`, so the driver builds only inside the repository).
- **Binary:** `opcua-client(.exe)` — must differ from the C# `OPC-UA-Client(.exe)` so both can
  live in `bin/`. Do not rename it to anything that collides case-insensitively on Windows.
- **Files** (one per C# file where possible, to keep them diffable):
  - `main.go` — startup, instance and connection loading, tag preload (← `Program.cs`)
  - `startup.go` — banner, command line and configuration-file policy
  - `config.go` — the connection document and its runtime state (← `Common_srv_cli.cs`)
  - `security.go` — certificates, PKCS#12/PEM loading, self-signed generation, the UA XML subset
  - `session.go` — endpoint discovery and selection, client options, connect/retry loop
    (← `ConsoleClient()`)
  - `browse.go` — address space walk (← `BrowseFullAddressSpaceAsync`)
  - `autotag.go` — the autoCreateTags read pass, browse-path splitting
  - `subscribe.go` — subscriptions, the notification pump (← `OnNotification`), per-item flood cap
  - `diagnostics.go` — per-session/per-subscription server diagnostics, never tagged or monitored
  - `uaconv.go` — value conversion (← `ConvertOpcValue`)
  - `mongo_update.go` — acquired-value queue and bulk writer (← `MongoUpdate.cs`)
  - `tags_creation.go` — automatic tag documents (← `TagsCreation.cs`)
  - `mongo_commands.go` — command change stream and dispatch (← `MongoCommands.cs`)
  - `commandconv.go` — checked numeric conversion of command values (rounding, range, JSON arrays)
  - `redundancy.go` — active/standby arbitration (← `Redundancy.cs`)
- **Tests:** `*_test.go`, plus `testdata/*.golden` — change-detectors for the auto-created tag
  document and the `sourceDataUpdate` write. Regenerate only deliberately:
  `go test -run TestGoldenTagDoc -update`, then review the diff.
- **Build:** `go build -ldflags="-s -w" -o ../../bin/opcua-client`
- **Config:** `conf/json-scada.json` plus MongoDB documents; CLI `<instance> <logLevel> <configFile>`
- **Platform integration:** `opcua_goclient.ini` supervisor units (`platform-rhel*`,
  `platform-ubuntu-*`), the `JSON_SCADA_opcuagoclient` NSSM service (`platform-windows`), and the
  `opcuago` variant of the `OPC-UA` entry in
  `src/server_realtime_auth/app/services/process-manager/driver-catalog.js`

## Work Guidance

- The C# driver is the specification. Before changing behaviour, check what
  `src/OPC-UA-Client` does; quirks are reproduced on purpose and marked `parity:` in comments.
  Intentional differences are numbered (D1 to D26) and listed in `DEVIATIONS.md`, each marked
  `deviation Dn` at its code site — add to that list rather than silently diverging.
- Keep `README.md` true. It states behaviour to users (defaults, `cancelReason` values, the topic
  rules, the type table, the log lines). A change to any of those updates it in the same change.
- Command numeric conversion lives in `commandconv.go` and is pinned to the .NET driver by
  `testdata/csharp_convert.golden`, measured with `testdata/csharp_convert_probe.cs.txt` on .NET 8.
  Never go back to plain Go casts for a command value: they wrap silently. If the .NET behaviour
  needs re-checking, regenerate the golden with the probe rather than editing it.
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

- `go test ./...` — conversions, tag documents (including the golden files), browse paths, topic
  matching, command conversions, discovery backoff, and loopback runs (browse, autotag, subscribe,
  write) against an in-process gopcua server. No MongoDB or device needed.
- `go vet ./...` and `gofmt -l .` must be clean.
- End-to-end: seed an instance and connection, run against a real server, and diff the resulting
  `realtimeData` documents against the C# driver's, allowing for the deviations in `DEVIATIONS.md`.
  The runs already done, with servers and numbers, are recorded there.

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
