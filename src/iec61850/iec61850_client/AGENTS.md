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
- **Library:** `github.com/dscsystems/go-iec61850` v0.8.0 (pure Go, GPLv3) — **pin the version**,
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
  - `secevents.go` — security events and IED identification (driver-side detections, `stats.peerIdentity`)
  - `secevents_map.go` — go-iec61850 `secevent.Event` → `secEvents` document; identical in `../iec61850_server`, change both together
- **Build:** `go build -ldflags="-s -w" -o ../../../bin/iec61850_client`
- **Config:** `conf/json-scada.json` plus MongoDB documents; CLI `<instance> <logLevel> <configFile>`
- **Connection stats:** `updateConnectionStats` (`redundancy.go`) writes `stats.isConnected` = association `mms.StateConnected`; a connection without a client gets `isConnected: false` (and the last `peerIdentity`) only, leaving the stored `lastReportIds` untouched. Every update replaces the whole `stats` field, so `stats.peerIdentity` is written in both cases
- **Security events:** written to `secEvents` through `go-common/jssecevents` (`secWriter`, started in `main`, drained on shutdown); kinds and fields in `README.md` (Security events) and `docs/schema.md`. go-iec61850 has no client-side event channel yet, so every client event is detected in `secevents.go`. `peerIdentification` (default true) runs `identifyPeer` after Identify on every association: nameplates `LPHD1.PhyNam` / `LLN0.NamPlt` (FC DC, attribute names from `GetVariableAccessAttributes`), MAC from the OS neighbour table, IED certificate captured in `tls.Config.VerifyConnection`; change detection starts from the stored `stats.peerIdentity.keys`

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
- Command tags are created by the MongoDB writer, not the value path: a control object carries no value, so `createCommandTags` inserts it at once and links it to its supervised twin (`supervisedOfCommand` / `commandOfSupervised`). A twin that does not exist yet is linked later by `linkCommands`, for as long as the driver runs; `preloadEntries` queues the driver's own unlinked command tags at startup.
- A failed bulk write keeps its values (`Queue.Requeue`) and clears the `InsertedTags` marks of its inserts; the unique `tag` index turns a repeated insert into a harmless duplicate error.
- Value, quality and timestamp of a structured value come from the object itself (deviations D13/D14): the named value attribute (`valueAttributeNames`, `cVal` included) or the first one by type, `q` for quality (validity other than good is invalid), and the first `UtcTime` (`t`). Never let a later member override them, as the C# driver did.
- A composite data object (WYE/DEL/SEQ, `isComposite`) is one entry with `Parts`, one per member; `emitValues` splits its value. Never register the members as entries of their own as well, or they are read twice.
- All numbers written to MongoDB must be Go `float64` so they land as BSON doubles.
- Report callbacks run on the association's reader goroutine: never block them, only enqueue, and
  never let them panic — a panic there takes the whole driver down. The security checks there
  (`secReportSequence`, `secUnmatchedReport`) only queue an event.
- One-off security events (read refusals, unmatched reports) go through `secOnce`, keyed per
  association; never emit one per poll sweep or per report.
- Connection fields come from a document decoded into `bson.M`, where sub-documents arrive as
  `bson.D`: read them through the `jsmongo` helpers, and test new fields through a real BSON round
  trip (`config_test.go`), not a hand-built `bson.M`.
- A data object's attribute names (`Childs`) must be in the order its value carries them, since
  the value is read by position: take them from a type description (`GetVariableAccessAttributes`,
  per logical node in `learnBrowsedChilds`), never from an MMS name list, which is alphabetical
  (`q`, `stVal`, `t` would make `stVal` the quality). The library's own server lists names in
  declaration order, so loopback tests cannot catch a regression here: `discovery_test.go` pins it.
  `registerBrowsedPoints` reads one type description per logical node and `registerBrowsedEntry`
  applies it.
- Report entries are identified from the data set members, and reports are matched to their
  subscription by `RptID` — see the RptID handling in `reports.go` before touching that path.
  The data set is read from the domain its own reference names, which may differ from the block's.
- Report coverage is per association: `runConnection` clears it (`ResetEntryReports`) and
  `enableRCB` marks the members of each block it enables, so a point whose block is lost is polled.
- Every segment of a buffered report carries the same EntryID: the repeat check compares
  `SubSeqNum` within the report being received (`rcbState.segEntryID`/`segLast`).
- `TrgOps` must include GI: the driver requests a GI right after enabling a block, and a
  conformant server ignores GI when its trigger is off. It also includes quality change (D15).
- `configureRCB` leaves `BufTm` at 0, so the IED keeps its own buffer time: the library writes
  back any non-zero value `GetRCB` read, and an IED with fixed report settings may refuse it.
- A buffered block resumes after the last EntryID the driver saw (`resyncEntryID`). With none
  saved, no EntryID is written — never all zeros, which some IEDs refuse. An EntryID the IED
  refuses (`entryIDRefused`, e.g. after an IED restart) is forgotten and the block is enabled
  without resync; retrying the same ID would never succeed.
- An MMS write answers per item: check the item results of `MMS().Write`, not only the call error
  (`disableRCB`). A block another client holds refuses even `RptEna=false`.
- Stop the driver gracefully (SIGINT/SIGTERM, Ctrl+C/Ctrl+Break): it disables its report blocks on
  the way out. Each `connectionLoop` closes its own association (one disable budget per block) and
  `main` waits for them, up to `shutdownTimeout`; never close an association from `main` while its
  loop runs.
- A request that times out while the driver is not stopping means the IED stopped answering
  (`isAssociationDown`); a sweep with nothing to poll probes with `Identify`.
- Commands run on one worker per connection (`startCommandWorkers`), so a slow control never
  delays the commands of other connections. Entries are keyed by `entryKey(address, FC)`
  everywhere, an empty common address being ST. A forced kill leaves BRCBs enabled, and some IEDs keep them owned by the dead
  association, refusing every other client, until the IED restarts.

## Verification

- `go test ./...` — security events and identification against the loopback IED (`secevents_test.go`), conversions, value/quality extraction and composite objects
  (`review_fixes_test.go`), tag documents, connection documents decoded from BSON, report block
  recovery (stale EntryID, block held by another client), and a loopback run against an
  in-process IEC 61850 server from `testdata/simpleIO_direct_control.cid` (no MongoDB or device
  needed)
- `go test -race ./...`
- `go vet ./...`
- End-to-end: seed an instance and connection, run against a real IED, and stop the driver
  gracefully (see Work Guidance) so it leaves no report block enabled on the device
