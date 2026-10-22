# DOX: src/dnp3-go — DNP3 Client and Server Drivers (Go)

## Purpose

Pure-Go DNP3 master and outstation drivers for JSON-SCADA. Drop-in alternatives to the C++
drivers in `src/dnp3`: same `protocolDriver` names (`DNP3`, `DNP3_SERVER`), same configuration
documents, same MongoDB semantics, no opendnp3, mongo-cxx-driver, OpenSSL, vcpkg or CMake.

## Ownership

- dnp3-go owns the Go implementation of both DNP3 drivers
- `src/dnp3` (C++) remains the reference implementation; behaviour is matched to it

## Local Contracts

- **Language:** Go 1.26, module `dnp3-go`, `cmd/` + `internal/` layout as in `src/iec60870-5`
- **Library:** `github.com/dscsystems/go-dnp3` v0.8.1 (GPLv3+, pure Go) — **pin the version**,
  the API is pre-1.0 and the SKILL.md in that repo says so explicitly. JSON-SCADA is GPL-3.0, so
  the copyleft is not a problem; note it rather than re-litigating it.
  - Used beyond the basics: device attributes (group 0) on the server, `multidrop.Bus` everywhere.
    Not used: file transfer (group 70) and `multidrop.Registry` — `dnp3util.BuildGroups` owns bus
    construction. `multidrop.DefaultQueue` is still 16, so `StationQueue` is still needed.
  - v0.5.0 widened `dnp3.Indexed.Index` (and `master.Update.Index`) to `uint32`. It also
    reports each point in its own static variation on a class 0 read, freezes and reassigns
    class only for the points a request names, clears the event overflow indication once the
    master confirms, and sets OVER_RANGE on values an integer variation cannot hold.
  - v0.5.2 renumbered the derived device attributes to IEEE 1815-2012 set 0 (counts at
    221/224/229/233/236/239, fragment sizes 240/241; earlier releases used 208–228), fixed the
    display names and `dnp3-explorer` labels to match, and answers the g0v255 list of attributes
    (`dnp3.AttrAttributeList`, `Attribute.List()`, `objects.ListAttribute`).
  - v0.5.3 fixed `dnp3.ControlClose`/`ControlTrip`, which every earlier release had transposed
    (close is trip/close code 1 = 0x40, trip is 2 = 0x80). **Builds on earlier releases sent trip
    for close** on durations 11/13/21/23 and read a standard close as 0. It also clears the request
    error IIN bits (NO_FUNC_CODE_SUPPORT, OBJECT_UNKNOWN, PARAMETER_ERROR, ALREADY_EXECUTING) once
    a response has reported them, and widened `decoder.Value.Index` to `uint32`.
  - v0.5.4/v0.5.5 fixed the master's command encoding, which wrote every index with a one-octet
    prefix: **builds on earlier releases operated point index mod 256** for any command above
    255 (300 operated 44). They keep periodic tasks across the startup sequence each connection
    runs (see the class scan rule below), stop an unsolicited response from deleting the events
    that did not fit in it, apply queued updates before answering a request, and read clock writes
    with an index prefix correctly. Additive API: command events (groups 13/43,
    `PointConfig.CommandEventClass`, default none), `Session.SetIndication`, master
    `FreezeCounters`/`FreezeAtTime`/`SyncTimeRecorded`, and `dnp3.ErrRejected` (returned only
    by those new calls).
  - v0.6.0 is additive and opt-in: DNP3 secure authentication (SAv5), writable device
    attributes, datasets, virtual terminal, frozen analogs, time-and-interval, self-address
    discovery, file authentication. None is used here. `master.Update` gained `FrozenAnalog`; a
    handler without `FrozenAnalogHandler` still receives frozen analogs through `HandleAnalog`.
    Behaviour changes that do apply: UDP sends each application fragment as one datagram (a
    loss drops the whole fragment), event reads are answered by event group, and pattern control
    (g12v2/v3) is refused rather than operated as a CROB. Nothing in this module changed.
  - v0.7.0/v0.8.0 changed what an outstation does with requests, and **two of those changes
    needed code here**: a control for an index the database has no output status for is refused
    before the handler (`OutputPoints`, see Work Guidance), and `Database.Configure` returns false
    for an event variation it cannot encode. Also: a variation without flags is sent with flags for
    a point that is not in normal condition; a read past the last point earns OBJECT_UNKNOWN or
    PARAMETER_ERROR beside the points that exist; `Session.Update` applies the change itself when no
    connection is being served; a select that expired answers TIMEOUT rather than NO_SELECT; passive
    channels retry a failed accept instead of ending the session. Additive and unused:
    `ClockConfig` (re-asserting NEED_TIME), `MaxControlsPerRequest`, `RestartSupport`,
    `DisableAttributes`, `EventBuffer.Discard`. v0.8.1 fixes events a master had just confirmed
    being reported again when a request arrived in the same read as the confirm. The server still accepts restart requests, as
    `NopApplication` does; the C++ server refuses them.
  - Stats counters available if the stats document ever wants them:
    `outstation.Stats.RepeatedRequests`/`IncompleteRequests`, `master.Stats.FragmentsDiscarded`.
  - Before the next bump, diff the API of the packages this module imports rather than trusting
    the version number: pre-1.0 minors may break.
- **Binaries:** `dnp3-client(.exe)` and `dnp3-server(.exe)`. They must **not** collide
  case-insensitively with `Dnp3ClientCpp.exe` / `Dnp3Server.exe`, so both implementations can
  live in `bin/` during migration. Do not rename them to `dnp3client` / `dnp3server`.
- **Packages:**
  - `cmd/dnp3client`, `cmd/dnp3server` — entry points, banner, CLI, config file
  - `src/go-common` (via a `replace` directive) — configuration (`jsconfig`), the levelled logger
    (`jslog`), BSON accessors and client factory (`jsmongo`), redundancy (`jsredundancy`) and
    the other shared JSON-SCADA helpers
  - `internal/dnp3util` — group tables, quality mapping, CROB codes, endpoints, the
    `multidrop.Bus` grouping layer, the counting and link-state channel wrappers
  - `internal/clientapp` — the master driver (← `Dnp3ClientCpp`)
  - `internal/serverapp` — the outstation driver (← `Dnp3Server`)
- **Build:** `build.bat`, or `go build -ldflags="-s -w" -o ../../bin/dnp3-client ./cmd/dnp3client`
- **Config:** `conf/json-scada.json` plus MongoDB documents; CLI `<instance> <logLevel> <configFile>`
- **Documentation:**
  - `cmd/dnp3client/README.md` and `cmd/dnp3server/README.md` are the **standalone operator
    references** of the two drivers: configuration, tag setup, behaviour, statistics,
    troubleshooting. They must stand on their own — never link to or defer to the C++ drivers'
    documentation in `src/dnp3`.
  - `README.md` (this directory) keeps what spans both: build, deviations from the C++ drivers
    (D-numbers), reproduced quirks (Q-numbers), engineering notes and testing. Describe a driver's
    behaviour in that driver's README and not here.
  - Every configuration key the code reads must be documented in the matching driver README;
    `TestReadmeDocumentsEveryConfigKey` (in both `internal/clientapp` and `internal/serverapp`)
    fails otherwise. A changed default, statistic, command result or log line is a README change
    in the same commit.

## Work Guidance

- The C++ drivers are the specification. Before changing behaviour, check what `src/dnp3` does.
  Quirks are reproduced on purpose and marked `parity:` in comments; intentional differences are
  numbered D1..D27 in `README.md` — add to that list rather than diverging silently.
- Only `sourceDataUpdate` is written for client data; never the tag `value`, alarms or history.
- All numbers written to MongoDB must be Go `float64` so they land as BSON doubles. The `stats`
  counters are the exception: they are `int64`, as in the C++ drivers.
- **Do not set a client-level `Timeout` on the MongoDB client.** It applies to every operation
  including a change stream's `Next`, so the watchers get torn down and rebuilt on each expiry.
  Every call that needs a bound passes its own context deadline.
- **Periodic class scans are registered once**, when the session first connects. From go-dnp3
  v0.5.4 the startup sequence each connection runs keeps periodic tasks, so registering again on
  every reconnection multiplies the polls. `TestClassScansSurviveReconnection` cuts the link
  three times and checks the poll rate is unchanged; it only catches this when the outage lasts
  long enough for `Connected()` to drop, so keep its outage.
- **Configure the analog variations explicitly.** go-dnp3 defaults analogs to the 32-bit integer
  variations, so an unconfigured point truncates 123.5 to 123. The server applies g30v5/g32v7 to
  every analog and g40v3/g42v7 to every analog output status, as the C++ server does.
- **Every event variation carries an absolute time** (D27). A change of any digital or analog
  point is reported as a timed event; `variationsFor` must never select an untimed one (g32v1/2/5/6,
  g22v1/2, g42v1/2 and the like) — `TestEveryEventCarriesTime` enforces it. The event time comes
  from `timestampFor`: `timeTagAtSource`, then `timeTag`, then now.
- **Point indexes arrive as `uint32` but requests carry `uint16`.** Keep received indexes wide
  (`push` takes `uint32`); an object address sent in a command or range scan must pass
  `validPointIndex` (0..65535) and is refused otherwise, never narrowed — narrowing wraps onto a
  different point (65541 → 5).
- **The server accepts every clock write** (`application.SupportsWriteTime` is always true, README
  Q6), as the C++ server does. Do not make it follow `timeSyncMode`: the outstation asserts
  NEED_TIME until its clock is set, so a refusal is repeated on every connection.
  `TestClockWriteAccepted` pins it.
- **CROB tests pin raw octets, never library constants.** `TestCROBWireCodes` (client) and
  `TestCROBValueWireCodes` (server) compare against the bytes opendnp3 sends; a test written
  against `dnp3.ControlClose`/`ControlTrip` agreed with the transposed constants of go-dnp3
  before v0.5.3 and could not catch a trip sent for a close. Likewise `TestCommandsReachTheirIndex`
  operates points above 255, which no test did while go-dnp3 wrapped them onto 0–255.
- **Only `origin: supervised` tags are data**, at startup (`tagsFor`) and for changes
  (`Engine.distribute`). A command tag's destination is in group 12 or 41, which `familyOf` sends to
  the analog family, so applying one overwrites the supervised analog at that index.
  `TestCommandTagChangeDoesNotTouchSuppliedPoints` pins it.
- `outstation.Session.Update` queues the change for the session loop **while a connection is
  being served**; it is not applied when the call returns. A test that changes a value over a live
  session and then scans must poll until the change is reported. With no connection (before `Run`,
  between reconnections) it applies the change itself before returning.
- **The command handler implements `outstation.OutputPoints`** and says every output exists
  (`commandHandler.HasOutput`). Without it go-dnp3 v0.8.0 refuses a control whose index is past the
  database's output status count — NOT_SUPPORTED before the handler looks for the command tag — and
  controls and output statuses are separate address spaces here.
  `TestCommandsReachTheHandlerWhateverTheirIndex` pins it.
- `Database.Configure` returns false for an event variation the library cannot encode;
  `configure` logs it, and `TestEveryVariationIsAccepted` checks the whole variation table.
- The time quality a master sees on an event is the **outstation's** synchronisation state (set by a
  master's clock write, which the driver always accepts), not the tag's `timeTagAtSourceOk`.
- `Database.Configure` replaces the whole `PointConfig`, so every call is a read-modify-write: a
  zero `Class` is `ClassNone` and silently kills the point's events.
- An outstation has no `Connected()`. Read the link state from the channel the session runs on
  (`dnp3util.WrapLinkState`), never from traffic: the default integrity interval is 300 seconds,
  so an idle-looking station is normal.
- Multi-drop is `multidrop.Bus`, not hand-written framing. Every connection goes on a bus,
  single-station ones included, because the frame and CRC statistics come from it.
- Auto-created command destinations carry an output status companion (`statusGroup` /
  `statusASDU` on `autoCreatePass`), placed on the command's `supervisedOfCommand` twin at **the
  command's own object address**. That address is fixed by the protocol — a CROB at index N
  operates binary output N — so it cannot be reassigned to dodge a clash; report the clash and
  skip instead.
- **`ipAddresses` on an active connection is a list of ways to reach one device**, not a list of
  devices. `ChannelSpec.BuildChannel` makes one inner channel per address and `WrapCountingAll`
  rotates through them: next address on a failure with no delay, backoff only once the whole list
  has been tried, and stay put on success. The channel-sharing `GroupKey` already keys on the
  whole list, so two connections share a channel only when their address lists match.
- **The multi-drop bus needs a large station queue on a socket.** Its default is 16 frames, which
  a large integrity response overruns instantly; a dropped link frame loses the whole application
  fragment and every tag it would have created. `dnp3util.StationQueue` is 4096. Do not lower it.
- **The client value queue coalesces, it does not drop.** Statics for one point collapse to the
  newest; event sequences are kept whole; past the threshold events collapse too and the writer
  reports it. Never restore the C++ drop-the-oldest behaviour: on a large device it discards the
  same points on every poll, so their tags are never created.
- The `sourceDataUpdate` filter relies on the standard realtimeData index on
  `(protocolSourceConnectionNumber, protocolSourceCommonAddress, protocolSourceObjectAddress)`.
  Without it a large batch times out and updates stall while creation continues.
- Device attributes (group 0) live in `serverapp/attributes.go`, numbered per IEEE 1815-2012 set 0
  (the table Wireshark and stepfunc's dnp3 crate use), declared there rather than taken from
  `dnp3.AttributeName`. The driver reports the full capacity block (events supported, highest
  index, count per point type, empty types included) on top of what go-dnp3 derives; configured
  attributes replace derived ones key by key, and `TestAttributesCoverDerived` checks that every
  key the library derives is answered here — keep it passing on every library bump, because a gap
  mixes the library's values into the response. Do not add 249 (conformance) or 248 (serial
  number); see `README.md`. The g0v255 list is answered by the library from the same set.
- **The active channels are built with `channel.NoRetry` and the driver retries in `countchan`.**
  Do not put the retry back into the library channel: it loops inside `Connect` and returns only
  a context error at the end, so `numOpenFail` never increments and the reason the port would not
  open is lost. In that retry loop, `channel.ErrClosed` and a cancelled context must propagate
  rather than be retried: both are an orderly shutdown, a closed channel can never produce a
  connection, and retrying hides the shutdown signal from the session.

## Verification

- `go test ./...` — conversion tables, variation tables, tag documents, grouping and conflict
  detection, plus loopback runs of a real master against a real outstation over `channel.Pipe`
  and over a real mutually authenticated TLS socket. No MongoDB and no hardware needed; the TLS
  tests generate their own certificates into `t.TempDir()`.
- `go vet ./...` and `gofmt -l .` must be clean; `go test -race ./...` for the concurrency.
- `TestReadmeDocumentsEveryConfigKey` keeps the driver READMEs complete: it reads the key names out
  of `config.go` (and `model.go` for the server) and fails on any that the README does not name.
- End to end: a temporary `mongod` as a single-node replica set (change streams need one), seeded
  with an instance, connections and tags, then both drivers against each other on port 20000.
  Check `sourceDataUpdate`, `stats.isConnected`, the auto-created tags, and that a document
  inserted into `commandsQueue` comes back with `delivered`/`ack`/`resultDescription`.
- Serial: `TestSerialPortPair` carries a real session over two real ports and skips unless
  `-serial.a` and `-serial.b` name a pair, so the gap closes the moment one exists —
  `socat -d -d pty,raw,echo=0 pty,raw,echo=0` on Linux, com0com on Windows (an elevated install).
  Everything else about serial operation is covered without hardware: the configuration mapping,
  the open-failure path, and the link-layer confirmation and arbitration that serial mode turns
  on. A virtual pair could not be created on the development machine used here — WSL2 needs
  virtualization that is not enabled, and com0com needs elevation and, on current Windows, test
  signing, which is a security control not worth disabling for a test.

## Known gaps

- Octet strings (groups 110/111) are decoded and discarded, as in the C++ drivers.
- Device attributes are served but not writable, and the client driver does not read them; there
  is nowhere in the JSON-SCADA schema to put what it would learn.
- Group 50 (time and interval) destinations are unsupported: `outstation.DatabaseConfig` has no
  such family.
- A passive TCP/TLS connection serves one master connection at a time; the bus multiplexes
  outstations, not masters.
- CROB durations 10, 12, 20 and 22 are documented in the driver README but were never implemented
  by the C++ switch; they behave as NUL. Reproduced deliberately.
