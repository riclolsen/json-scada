# DOX: src/iec61850/iec61850_client — IEC 61850 MMS Client Driver (Go)

## Purpose

Pure-Go IEC 61850 MMS client driver for JSON-SCADA. Replaces the C# driver that lived in
`src/iec61850_client`: same `protocolDriver` name (`IEC61850`), same configuration documents, same
MongoDB semantics, no native library dependency.

## Ownership

- iec61850/iec61850_client owns the Go implementation of the IEC 61850 MMS client driver
- The C# driver remains the behavioural reference. It was removed from the tree in commit
  `8a3f80de`; read it with `git show 8a3f80de^:src/iec61850_client/<file>`

## Local Contracts

- **Language:** Go 1.26, module `iec61850_client`, flat `package main`
- **Library:** `github.com/dscsystems/go-iec61850` v0.6.5 (pure Go, GPLv3) — **pin the version**,
  the API is pre-v1
- **Binary:** `iec61850_client(.exe)` in `bin/`, as `platform-windows/build.bat` and
  `platform-linux/build.sh` build it
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
- **Build:** `go build -ldflags="-s -w" -o ../../../bin/iec61850_client`
- **Config:** `conf/json-scada.json` plus MongoDB documents; CLI `<instance> <logLevel> <configFile>`

## Work Guidance

- The C# driver is the specification. Before changing behaviour, check what it does (see
  Ownership); quirks are reproduced on purpose and are marked `parity:` in
  comments. Intentional differences are explained in a comment where the code diverges (older ones
  are tagged `deviation Dn`) — never diverge silently.
- Only `sourceDataUpdate` is written for data; never tag `value`, alarms or history.
- Automatically created tags (`tags_creation.go`): `tag` is `<connection>;<object reference>[<FC>]`
  with no driver prefix; `group1` the connection name, `group2` the logical device, `group3` the
  logical node (`splitRef`); `description` is `group1~group2~group3~ungroupedDescription`, starting
  with `group1` because viewers strip it. Documented in `README.md` (autoCreateTags).
- A point's tag is found by object reference + FC (`pointKey`, kept in `InsertedTags`), never by tag
  name, so a renamed scheme never duplicates the tags already in `realtimeData`.
- `Iec61850Entry.AutoPublish` marks a point the driver discovered itself (browse or report); only those carry the self-publish flag, so a point configured in realtimeData never gets a second tag.
- Command tags are created by the MongoDB writer, not the value path: a control object carries no value, so `createCommandTags` inserts it and links it to its supervised twin (`supervisedOfCommand` / `commandOfSupervised`). It waits for the twin to exist, up to `commandLinkAttempts` writer cycles.
- All numbers written to MongoDB must be Go `float64` so they land as BSON doubles.
- Report callbacks run on the association's reader goroutine: never block them, only enqueue, and
  never let them panic — a panic there takes the whole driver down.
- Connection fields come from a document decoded into `bson.M`, where sub-documents arrive as
  `bson.D`: read them through the `jsmongo` helpers, and test new fields through a real BSON round
  trip (`config_test.go`), not a hand-built `bson.M`.
- Report entries are identified from the data set members, and reports are matched to their
  subscription by `RptID` — see the RptID handling in `reports.go` before touching that path.
- `TrgOps` must include GI: the driver requests a GI right after enabling a block, and a
  conformant server ignores GI when its trigger is off.
- `configureRCB` leaves `BufTm` at 0, so the IED keeps its own buffer time: the library writes
  back any non-zero value `GetRCB` read, and an IED with fixed report settings may refuse it.
- A buffered block resumes after the last EntryID the driver saw (`resyncEntryID`). With none
  saved, no EntryID is written — never all zeros, which some IEDs refuse. An EntryID the IED
  refuses (`entryIDRefused`, e.g. after an IED restart) is forgotten and the block is enabled
  without resync; retrying the same ID would never succeed.
- An MMS write answers per item: check the item results of `MMS().Write`, not only the call error
  (`disableRCB`). A block another client holds refuses even `RptEna=false`.
- Stop the driver gracefully (SIGINT/SIGTERM, Ctrl+C/Ctrl+Break): it disables its report blocks on
  the way out. A forced kill leaves BRCBs enabled, and some IEDs keep them owned by the dead
  association, refusing every other client, until the IED restarts.

## Verification

- `go test ./...` — conversions, tag documents, connection documents decoded from BSON, report
  block recovery (stale EntryID, block held by another client), and a loopback run against an
  in-process IEC 61850 server from `testdata/simpleIO_direct_control.cid` (no MongoDB or device
  needed)
- `go test -race ./...`
- `go vet ./...`
- `go list -deps ./... | grep charm` must be empty — the library's TUI dependencies must not be
  linked in
- End-to-end: seed an instance and connection, run against a real IED, and stop the driver
  gracefully (see Work Guidance) so it leaves no report block enabled on the device
