# DOX: src/OPC-UA-Client-Go — OPC UA Client Driver (Go)

## Purpose

Pure-Go OPC UA client driver for JSON-SCADA. Drop-in alternative to the C# driver in
`src/OPC-UA-Client`: same `protocolDriver` name (`OPC-UA`), same configuration documents, same
MongoDB semantics, no .NET runtime dependency. The C# driver has been removed from the repository.

## Ownership

- OPC-UA-Client-Go owns the Go implementation of the OPC UA client driver
- Behaviour is matched to the former C# driver (`parity:` comments); no C# source remains to compare against
- `src/go-common` owns the shared plumbing (config, logging, MongoDB access, redundancy, tag and
  `sourceDataUpdate` defaults); this driver owns everything OPC UA specific
- `README.md` is the **user guide**, complete without any other document; `DEVIATIONS.md` is the
  developer reference for how this driver differs from the C# one and how that was verified

## Local Contracts

- **Language:** Go 1.26, module `opcua-client`, flat `package main`
- **Library:** `github.com/gopcua/opcua` v0.9.1 (MIT, pure Go), replaced in `go.mod` by the
  `github.com/riclolsen/opcua` fork at a pinned commit of its `main` (the gopcua module path is
  kept, so imports do not change): it adds the `secevent` package, `opcua.SecurityEventHandler`,
  `Client.RemoteMAC` and stricter secure channel checks. **Pin the commit**, the API is pre-v1;
  bump it with `go mod edit -replace github.com/gopcua/opcua=github.com/riclolsen/opcua@<commit>`
  and `go mod tidy`, and keep `LibraryVersion` (`config.go`) naming it. Plus `software.sslmate.com/src/go-pkcs12` for `.pfx` files, and the local
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
  - `browse.go` — address space walk (← `BrowseFullAddressSpaceAsync`); `refEntry.Parent` links a
    property to its variable
  - `autotag.go` — the autoCreateTags read pass, browse-path splitting, unit and range properties
    folded into the parent's tag (`metadataProps`, never tagged)
  - `subscribe.go` — subscriptions, the notification pump (← `OnNotification`), per-item flood cap
  - `diagnostics.go` — per-session/per-subscription server diagnostics, never tagged or monitored
  - `uaconv.go` — value conversion (← `ConvertOpcValue`)
  - `mongo_update.go` — acquired-value queue and bulk writer (← `MongoUpdate.cs`)
  - `tags_creation.go` — automatic tag documents (← `TagsCreation.cs`)
  - `mongo_commands.go` — command change stream and dispatch (← `MongoCommands.cs`)
  - `commandconv.go` — checked numeric conversion of command values (rounding, range, JSON arrays)
  - `redundancy.go` — active/standby arbitration (← `Redundancy.cs`), connection stats
  - `secevents.go` — security events (`secEvents`) and the server's identity
    (`stats.peerIdentity`): the library event handler, identification, command and node checks
  - `robustness_test.go` — session health, rediscovery, typed commands, writer drain
- **Tests:** `*_test.go`, plus `testdata/*.golden` — change-detectors for the auto-created tag
  document and the `sourceDataUpdate` write. Regenerate only deliberately:
  `go test -run TestGoldenTagDoc -update`, then review the diff.
- **Build:** `go build -ldflags="-s -w" -o ../../bin/opcua-client`
- **Config:** `conf/json-scada.json` plus MongoDB documents; CLI `<instance> <logLevel> <configFile>`
- **Platform integration:** `opcua_goclient.ini` supervisor units (`platform-rhel`,
  `platform-ubuntu`), the `JSON_SCADA_opcuagoclient` WinSW service (`platform-windows`), and the
  `opcuago` variant of the `OPC-UA` entry in
  `src/server_realtime_auth/app/services/process-manager/driver-catalog.js`
- **Connection stats:** `updateConnectionStats` (`redundancy.go`) writes `stats.isConnected` = session state `opcua.Connected` for every connection, and `stats.peerIdentity` when the connection has one; the C# parity heartbeat (`nodeName`, `timeTag`) is otherwise unchanged

## Work Guidance

- Behaviour of the former C# driver is the specification; its quirks are reproduced on purpose
  and marked `parity:` in comments. Intentional differences are numbered (D1 to D39) and listed in
  `DEVIATIONS.md`, each marked `deviation Dn` at its code site — add to that list
  rather than silently diverging.
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
  synchronously and a slow reader stalls the client. `StateChangedFunc` may only log and
  notify the `identifySignal`, which never blocks.
- `ua.StatusCode` has no `IsGood`; use `statusIsGood` (severity bits 30-31).
- The notification pump must never block — it only converts and enqueues.
- Everything a session starts (the pumps) runs on the session context created in `connectionLoop`
  and ends with it; never start per-session goroutines on the process context.
- Discovery that does not complete is never published (D18, D27): return an error and let
  `connectionLoop` rebuild. `setupSubscriptions` errors rebuild too, at most `maxSetupFailures` times.
- `watchClient` ends a session on state loss, a pump-reported subscription error, or silence
  (`sessionHealth`, D28); the heartbeat item uses the reserved client handle `heartbeatHandle`.
- **Security events** (`secevents.go`): the library handler (`opcua.SecurityEventHandler`) runs on
  the protocol goroutines, so it only filters, throttles and queues. Identification runs on the
  session context in `identifyLoop`, triggered by a non-blocking signal from `StateChangedFunc`
  on every `Connected` (the client reconnects by itself) and once after `connectOnce`. The MAC
  history is the driver's `connSec.changes`, seeded from `stats.peerIdentity`: the library's
  own lives in one `opcua.Client`, which `connectionLoop` rebuilds, so its `connection.mac_changed`
  is stored only when the driver's history agrees it is new. With `peerIdentification` off no
  identity is stored, events carry no peer IP (the secEvents writer would look up its MAC), and
  the handler goes on a `opcua.DefaultDialer()` with `Events` and no `MACs`, so connecting does
  no lookup; the fork's `uacp.Conn.Emit` still looks one up for a connection's first event, which
  only a library option could stop.
- Each `opcua.Client` has its own `identifySignal` (built in `connectOnce`, notified by its
  `StateChangedFunc`); `identifyLoop` publishes a result only while the signal's generation is
  the one it read under, so a superseded or cancelled read is discarded. History keys are per
  configured endpoint URL (`field|url`) and the MAC per IP (`mac|ip`): failover between
  `endpointURLs` must not look like a change or a restart.
- Command auditing never delays or outlives a command: the expiry is checked again right before
  `Write`/`Call`, method classification (`classifyMethod`, cached per connection) runs in a
  goroutine after the call, and `valueString` is never put in an event. A bad service result
  arrives as a `ua.StatusCode` error (`serviceStatus`); transport statuses are not refusals.
- Before `reloadConnectionTags`, wait for the writer with `waitWriterIdle` (D29); `main` waits for
  the connection loops on shutdown (D32).

## Verification

- `go test ./...` — conversions, tag documents (including the golden files), browse paths, topic
  matching, command conversions, discovery backoff, security events and server identification
  (`secevents_test.go`), and loopback runs (browse, autotag, unit and range in
  `engmeta_test.go`, subscribe, write, identify) against an in-process gopcua server. No MongoDB or
  device needed.
- `go vet ./...` and `gofmt -l .` must be clean.
- `go test -race` can report a race inside gopcua's in-process test server (`server/node.go`,
  reached from `setServerValue` in `loopback_test.go`) in the two loopback acquisition tests; it is
  in the harness, not the driver. Any race with a frame in a non-test file of this module is real.
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
