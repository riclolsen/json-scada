# DOX: src/iccp — ICCP TASE2 Protocol Drivers

## Purpose

ICCP (IEC 60870-6 / TASE.2) client and server protocol drivers for JSON-SCADA, written in Go. Provides interoperability with utility control center systems using the ICCP/TASE.2 standard.

## Ownership

- iccp owns the ICCP client and server implementations
- The tase2 subdirectory contains the TASE2 library dependency
- Interop boundaries: MongoDB change streams (in), real-time data events (out)

## Local Contracts

- **Language:** Go (1.26+, per `go.mod`)
- **Dependencies:** `go.mod` / `go.sum`
- **Build:** `GOWORK=off go build` in component directories
- Sub-projects:
  - `iccp-client/` — ICCP TASE.2 client driver
  - `iccp-server/` — ICCP TASE.2 server driver
  - `tase2/` — TASE.2 communication library (closed source, 100% native Go)
- Configuration via the central `json-scada.json` or environment variables
- Uses MongoDB Go driver for database access
- `iccp-client` and `iccp-server` are intentionally commented out of `src/go.work`; build and test them standalone with `GOWORK=off`
- Each module resolves the library through `replace github.com/riclolsen/tase2 => ../tase2`, so `tase2/` must be checked out at a compatible revision (currently `v0.5.0`+, `origin/main`)
- **Connection stats:** `stats.isConnected` — client: written true on association and every stats tick, false on connect failure and after disconnect (`updateConnectionStats`); server: at least one association open (`statsDocument`)
- **Startup:** each driver starts its goroutines, listeners and associations once, after the first successful MongoDB connection; a failed ping afterwards is only logged (the MongoDB driver reconnects, each loop retries its own operations). Never re-run startup on a lost ping: it duplicates writers, watchers and associations
- **Redundancy:** `isActive`/`keepRunningWhileInactive` are `atomic.Bool`; `protocolRunning()` = active or `keepProtocolRunningWhileInactive`. Client: associations only while `protocolRunning()`, data written (`connRuntime.publish`) and commands sent only while active. Server: associations refused (closed at accept) and aborted while not `protocolRunning()`, same rule for commands

## Work Guidance

- ICCP uses the TASE.2 MMS protocol — be familiar with IEC 60870-6 standards
- The TASE2 library is closed source; treat as an opaque dependency
- Client side: initiate connections to remote ICCP servers
- Server side: accept connections from remote ICCP clients
- Both sides: support TCP/TLS transport
- Follow Go idioms: `error` returns, `context.Context`, idiomatic naming
- Use the exported `tase2` quality constants (`QualityValid`/`QualityHeld`/`QualitySuspect`/`QualityInvalid`, `SourceTelemetered`/`SourceCalculated`/`SourceEntered`/`SourceEstimated`) instead of literal strings; decoded values only ever use the canonical names, while legacy spellings such as `questionable`, `substituted` and `process` are accepted on encode only and will silently fail to match on decode
- Time stamps: the server publishes indication points as IEC 60870-6-802 Ed.2 `*QTimeTagExtended` types (`TimeStampExtended` = GMT seconds + milliseconds, UTC) stamped from `timeTagAtSource`; the client reads every time stamp through `DecodedPoint.Time()` plus `hoursShift`. Never use the deprecated `TimeTagNow`/`TimeTagFrom` (legacy milliseconds since local midnight) — `TimeStampFrom` is the second-resolution form
- Listening: tase2 binds passive endpoints to loopback unless `Endpoint.SetListenAddress` says otherwise; the server must always set it from `ipAddressLocalBind` via `parseLocalBind` (empty host = all interfaces `0.0.0.0`, port default 102) before `Listen`
- Logging: each driver's `logger.go` routes all library output (`iso`/`mms`/`tase2`) into `LogMsg` levels via `configureTASE2Logging` (call at startup) and `tase2LogHandler(conn.Name)` (set on every endpoint before `Listen`/`Connect`); library Error/Info/Debug map to driver levels 0/2/3; the driver level is read from library goroutines, so access it only through `currentLogLevel()`/`setLogLevel()` (atomic), never a plain variable
- Server statistics and events live in `iccp-server/stats.go`: `serveClientCallbacks` is the only `ServeClients` wiring (used by `main` and the loopback test); per-association `Server.Stats()` are summed per connection, ended associations folded in by the post-disconnect hook, and published to `protocolConnections.stats` every 10 s; the server event observer records peer identity and logs events
- Server data model: `buildModel` is the only model builder (used by `main` and the tests). Domains are `group1` sanitized like point names (`domainNameOf`); compare topics to domains only through `topicAllowsDomain`. Name collisions keep the first tag by name. Command tags are control points only when some commands-enabled connection covers the domain. Change-stream values go to `DataModel.UpdateOnlineValue` (model stays current with no client; every attached server is notified), never to per-server updates
- Server commands: operate, write-data and select handlers are always registered and gated by `commandTarget` (commands enabled, node running, topic, live `commandBlocked`); an unregistered handler would make the library acknowledge and drop the command
- Client values: ICCP states map to digital 0/1 (Off/On; in-between and invalid flagged invalid). Tags are invalidated by re-sending their last raw value flagged invalid (`connRuntime.invalidate`, seeded from `sourceDataUpdate`), never by writing a value of 0. DSTS coverage is per point (`covered` "Domain/Item" keys); the poll reads every mapped point not covered

## Verification

- `GOWORK=off go build ./...` in `iccp-client/` and `iccp-server/`
- `GOWORK=off go vet ./...` and `GOWORK=off go test ./...` in each module (both have unit plus in-process loopback tests; no MongoDB required)
- Run the same checks after bumping `tase2/`: the library compiles against unchanged driver code even when quality or data-model semantics shift, so the tests are the real gate
