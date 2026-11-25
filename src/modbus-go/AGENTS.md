# DOX: src/modbus-go — Modbus Client and Server, SunSpec Client Drivers (Go)

## Purpose

Go Modbus master and slave drivers for JSON-SCADA, built on go-modbus (the fork
`github.com/riclolsen/go-modbus` of `github.com/otfabric/go-modbus`).
Drop-in replacements for the former Node.js drivers in `src/modbus` (sources removed in 0.69):
same `protocolDriver` names (`MODBUS`, `MODBUS_SERVER`), same configuration documents and
defaults, same CLI and environment variables (`JS_MODBUS_`, `JS_MODBUSSRV_`), same service names.
Also the SunSpec client (`SUNSPEC`, new), on go-sunspec (`github.com/riclolsen/go-sunspec`)
over the same go-modbus transports.

## Ownership

- modbus-go owns the Go implementation of both Modbus drivers
- The Node.js sources this implementation was matched against are gone; `README.md` keeps the
  numbered behavioural differences for reference

## Local Contracts

- **Language:** Go 1.26, module `modbus-go`, `cmd/` + `internal/` layout, listed in `src/go.work`
- **Library:** go-modbus v1.24.1 (MIT) from the fork: the module path stays
  `github.com/otfabric/go-modbus`, and `go.mod` has
  `replace github.com/otfabric/go-modbus => github.com/riclolsen/go-modbus v1.24.1` (bump both
  together). `github.com/otfabric/go-serial` v0.5.0 (the version go-modbus requires; `mbnet`
  uses it directly). Pin the versions; diff the API of what this module uses, and read the
  library's `RELEASE.md`, before bumping.
- **SunSpec library:** go-sunspec (MIT) from the fork, untagged: the module path stays
  `github.com/otfabric/go-sunspec`, and `go.mod` has
  `replace github.com/otfabric/go-sunspec => github.com/riclolsen/go-sunspec <pseudo-version>`.
  Its own go.mod requires go-modbus; this module's go-modbus `replace` decides the version.
- **Binaries:** `modbus-client(.exe)`, `modbus-server(.exe)` and `sunspec-client(.exe)` in `bin/`.
- **Service names do not change:** Supervisor programs `modbusclient` / `modbusserver`, Windows
  services `JSON_SCADA_modbusclient` / `JSON_SCADA_modbusserver`, process-manager keys
  `MODBUS` / `MODBUS_SERVER`. SunSpec: `sunspecclient`, `JSON_SCADA_sunspecclient`, `SUNSPEC`,
  environment prefix `JS_SUNSPEC_`.
- **Packages:**
  - `cmd/modbusclient`, `cmd/modbusserver`, `cmd/sunspecclient` — entry points; `Main` takes
    the full `os.Args`
  - `internal/mbcore` — codec, byte orders, addresses/ASDUs, CRC/RTU framing, request
    descriptions, JavaScript number semantics. No MongoDB or driver imports.
  - `internal/mbnet` — TLS configs, client loopback `Bridge`, server `Front` and RTU gateway,
    serial ports
  - `internal/drv` — CLI/config start-up, instance document, loggers, BSON/JS conversions
  - `internal/mbsec` — security event kinds and their classification, the mapping of go-modbus
    `secevent` events and the Modbus fields onto `secEvents` documents
  - `internal/clientapp` — `MODBUS` driver, and `Run` (start-up, instance, connections through a
    `Factory` of `ManagedConnection`s, redundancy, stats, bindings reload, the `jssecevents.Writer`)
    shared with `SUNSPEC`; `internal/serverapp` — `MODBUS_SERVER` driver
  - `internal/sunspecapp` — `SUNSPEC` driver: discovery per `unitIds`, point addresses
    (`<model>[#n]/[<group>[i]/...]<point>`), automatic tags from the discovered models,
    `ReadModels` polling, `WritePoint` commands, security events. Values (invalid ones too) are
    written as whole `sourceDataUpdate` documents (the data processors watch that field), in
    read order (`nextSeq` taken inside the gate); a binding's `last` moves only after the write.
    Commands recheck expiry, active node and session inside the gate.
  - `internal/loopback` — client ↔ server tests over every network transport
  - `src/go-common` via a `replace` directive — `jsconfig`, `jslog`, `jsmongo`,
    `jsredundancy`, `jsstats`, `jsrtdata`, `jscommands`, `jstls`, `jsmodel`, `jssecevents`
- **Build:** `build.bat`, or `go build -ldflags="-s -w" -o ../../bin/modbus-client ./cmd/modbusclient`
  (and `modbus-server` from `./cmd/modbusserver`)
- **Documentation:**
  - `cmd/modbusclient/README.md`, `cmd/modbusserver/README.md` and `cmd/sunspecclient/README.md`
    are the standalone operator references; the Node.js sources and their docs are no longer in the repository.
  - `README.md` keeps what both share (transports, types, byte orders, addressing, logging),
    build, architecture, the numbered differences from the former Node.js drivers, migration
    and testing.
  - Every key `config.go` reads must be in the driver README (`TestReadmeDocumentsEveryConfigKey`).
    A changed default, statistic, command result or log line is a README change in the same commit.
- **Security events and peer identification:** each driver process runs one
  `jssecevents.Writer`; each connection gets an `mbsec.Reporter` (`SetSecurity`, with the
  `realtimeData` collection for the unnecessary-command check). `peerIdentification` (default
  true) publishes `stats.peerIdentity` and seeds the change detection from the stored one.
  Client: `clientapp/secevents.go` (FC43/14 or FC17 per polled unit when the device first
  answers, MAC, certificate; read, response, command, flood, stale and unnecessary checks).
  Server: `serverapp/secevents.go` (front-end hooks `OnReject`/`OnHandshake`/`OnOpen`/`OnClose`/
  `OnBadFrame`, the handler's refusals, the backend's `ServerConfig.Security` sink and `PDUTap`).
  Server sessions are registered in `opened` at once with a token; `identify` completes one only
  while the same token is open. The unnecessary-command check runs in the background against a
  supervised value older than the command (`SupervisedState` `before`); the FC17 identity key
  masks the run indicator (`serverIDKey`).
  SunSpec client: `sunspecapp/secevents.go`, protocol `sunspec` (go-sunspec's `secevent` reports
  only on servers): identity from the Common model (1) and the model chain per unit after each
  discovery, unknown units, invalid chains, refused reads, configured points the device does not
  expose, and the Modbus client's response and command checks.
  Kinds, sources and the stats shape are documented in the driver READMEs and `docs/schema.md`
  (secEvents, Peer identification); a new kind goes into the `mbsec` catalogue and both.
- **Connection stats:** `stats.isConnected` (via `jsstats.Entry.IsConnected`) — SunSpec client:
  at least one unit discovered; Modbus client:
  `Stats().Status == "connected"`, set once the device answers (a silent peer is a lost link);
  server: at least one client attached, or on a serial port a request within `clientIdleTimeoutMs`

## Work Guidance

- The former Node.js drivers are the behavioural specification. Intentional differences are
  numbered in `README.md` ("Differences from the Node.js drivers"); add to that list rather
  than diverging silently.
- Read configuration with the `drv` helpers (`Num`, `Str`, `Bool`, `StrArr`, `ToNum`, `JSNumber`,
  `AnyString`) so missing, `null`, `""` and wrong-typed values behave as in JavaScript.
- Client data goes only to `sourceDataUpdate`; numbers written to MongoDB are `float64`, except
  the `stats` counters (`int64`).
- go-modbus gaps are bridged, not patched: client TLS and local bind go through `mbnet.Bridge`;
  the server's go-modbus instance listens on a private `127.0.0.1` port behind `mbnet.Front`,
  which owns the allow-list, client limit, idle timeout, TLS and RTU framing. Handlers must
  resolve `ClientAddr` through `AddrMap` to name the real master.
- Client commands go through one queue per connection and run one at a time, in arrival order.
- The server backend answers only connections registered in `AddrMap` (front-end and RTU
  gateway); register a new path's upstream connection there before it sends a request.
- Server write routing goes through `PointMap.LookupWrite` slots (word point, bit points): check
  every target of a request before relaying any.
- Server handler exceptions are wrapped in `handlerError` (logged and counted once); the
  `metrics` hook logs only exceptions the library sends itself. Keep that split.
- A write refused for a reason of its own returns a `refusal` (it unwraps to the exception
  answered) so it is reported as `cmd.rejected`; a bare `ErrIllegalDataAddress` is
  `cmd.object_unknown`.
- The backend's library events name the loopback connection: resolve them through `AddrMap`
  (it remembers closed connections for 30 s) and keep dropping the library's connection events;
  the front-end reports the masters' own.
- A condition that can repeat (a peer, unit, address, certificate error, noisy line) is
  reported through a `jssecevents.Throttle` (once per 10 minutes, with `count`), never once per
  request.
- SunSpec: a discovered model whose header changed (`ErrModelChanged`) is discovered again, never
  decoded against the old chain; scale factor and pad points and the model header get no tag;
  commands write only top-level points through `WritePoint`, without retries.
- Do not set a client-level timeout on the MongoDB client; give each call its own deadline
  (change streams would be torn down otherwise).

## Verification

- `go vet ./...` and `gofmt -l .` clean; `go test ./...` and `go test -race ./...` pass.
  No MongoDB or hardware needed; the TLS loopback tests generate certificates in `t.TempDir()`.
- `internal/sunspecapp` tests run the SunSpec client against an in-process go-sunspec device.
- Interop by hand: `diagslave` / `modpoll` (see `README.md`).

## Child DOX Index

None.
