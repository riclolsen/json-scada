# go-common — shared support library for the {json:scada} Go drivers

Module: `github.com/riclolsen/json-scada/src/go-common`

Each Go driver used to carry its own copy of the `{json:scada}` plumbing: reading
`conf/json-scada.json`, the levelled logger, the MongoDB connection and its permissive BSON
accessors, the driver-instance document, the redundancy loop, the statistics writer. That is what
lives here now.

Plan and rationale: [`docs/GO_DRIVERS_UNIFICATION_PLAN.md`](../../docs/GO_DRIVERS_UNIFICATION_PLAN.md).

## The contract

**This library must not change any driver's observable behaviour.** Same MongoDB collections, fields
and document shapes; same CLI contract; same log line format; same failover semantics. Every driver
remains a drop-in replacement for the C#/C++ original it was ported from.

When two drivers disagreed about something, the disagreement is preserved as an explicit,
documented option — it is never flattened to whichever behaviour was more convenient. Three places
where that matters:

- **Statistics.** The error wording, log level and update timeout differ per driver, so
  `jsstats.Writer` takes them as fields rather than assuming.
- **`sourceDataUpdate` and tag documents.** The protocol-specific fields differ; see the notes below.

Two things were deliberately unified rather than preserved, by decision of the maintainer:

- **Log timestamps.** Five layouts collapsed to one, `jslog.TimeFormat`.
- **Redundancy takeover.** Two algorithms collapsed to one, the unchanged-count method.

Both are described below.

## Packages

| Package | What it owns |
|---|---|
| `jsconfig` | `conf/json-scada.json`, CLI args (`<instance> <logLevel> <configFile>`), path resolution |
| `jslog` | Levelled logger, buffered stdout writer, `slog` bridge for protocol libraries |
| `jsmongo` | Connect/ping, collection names, permissive BSON accessors |
| `jsmodel` | `protocolDriverInstances` document, node-allowed check |
| `jstls` | Certificate/version/cipher plumbing for IEC 62351-3 and equivalents |
| `jsstats` | The `protocolConnections` statistics update shape and stamping |
| `jsrtdata` | The acquired-value queue and the `sourceDataUpdate` document |
| `jscommands` | The `commandsQueue` pipeline, field extraction, expiry and ack/cancel writeback |
| `jstags` | The auto-created tag defaults and the `_id` partition per connection |
| `jsredundancy` | Active-node arbitration: the unchanged-count takeover rule |

Planned, not yet implemented: `jsdriver`.

## Log timestamps

One layout, `jslog.TimeFormat` — C#'s "o" round-trip form, seven fractional digits always present,
with the local UTC offset:

```
[2026-03-04T05:06:07.0800000-03:00] message
```

The OPC-UA and both IEC 61850 drivers already emitted this. **dnp3-go** moved from three fractional
digits and **iec60870-5** from `RFC3339Nano`, which stripped trailing zeros so its field width varied
line to line. Anything that parsed those two drivers' logs by column position needs updating;
anything parsing the bracketed timestamp as RFC 3339 does not. `cs_data_processor-go` and the `iccp`
pair still have their own loggers and are unaffected until Phase 5.

Drivers must not call `jslog.SetTimeFormat` — it exists for tests.

### What deliberately stayed in the drivers

The drain loops, the change-stream loops and the protocol-specific halves of every document. They are
tuned and shaped differently on purpose — see §5.13 of the plan. Two consequences worth knowing:

- `jsrtdata.SourceDataUpdate` has twelve typed fields and an `Extra` map. The four drivers disagree
  about which other fields exist (`carryAtSource`, `transientAtSource`, `valueBsonAtSource`,
  `valueJsonAtSource`, `originator`), so a union struct would have made each driver start writing
  fields it never wrote. Put driver-specific fields in `Extra`.
- `jstags.BaseDoc()` holds only the 38 fields that were identical everywhere. `invalid`,
  `invalidDetectTimeout` and `protocolDestinations` are excluded because the drivers genuinely
  differ; set them yourself.

### Golden change-detectors

`newRealtimeDoc` and `updateModel` write the documents the rest of JSON-SCADA reads, and had no tests
at all before this work. OPC-UA-Client-Go, the IEC 61850 client and dnp3-go now pin every field of
every shape in `testdata/*.golden`. They were generated **before** the migration and pass unchanged
after it, which is what makes "byte-identical" a claim rather than a hope.

Regenerate deliberately, and read the diff:

```bash
go test -run TestGolden -update
```

## Redundancy

### The takeover rule

**One algorithm: unchanged-count.** A standby reads the active node's `activeNodeKeepAliveTimeTag`
every 5 s and counts how many times it comes back *unchanged*; after more than 4 unchanged readings
it takes over. Nothing is compared against the local clock, so arbitration is immune to clock skew
between the two nodes.

`dnp3-go` previously used the C++ wall-clock test (take over when the keep-alive is older than 15 s
by the local clock) and now uses this one. What changed for it:

| | before | after |
|---|---|---|
| Takeover latency | ~15 s | ~25–30 s |
| Clock skew between nodes | sensitive | immune |
| On yielding the active role | no pause | random 1–5 s, so two nodes do not flip in lockstep |
| Node not in `nodeNames` | tolerated | fatal, as in every other driver |
| Missing instance document | stayed active | stayed active — kept via `MissingInstanceActive` |

`MissingInstanceActive` is the one thing not unified. The C++ lineage defaults to *active* when there
is no instance document at all, so a single-node system with an incomplete configuration still runs;
the C# lineage stays inactive. That is a bootstrap policy, not the takeover rule, and flipping it
would silently stop a misconfigured single-node deployment from acquiring. Only `dnp3-go` sets it.

### Fixed: stalls must be consecutive

`Redundancy.cs` only ever incremented the counter. It was cleared when a node activated or
deactivated, but never when the active node's keep-alive started advancing again, so separate short
stalls accumulated and a live-but-flaky active node was eventually displaced even though it had
never stopped. Every Go driver inherited that.

The counter is now reset whenever the keep-alive advances, so a takeover needs
`KeepAliveCountLimit+1` **consecutive** unchanged readings. This is strictly more conservative — it
makes spurious failovers less likely, never more — and a genuinely dead node is replaced just as
fast, because a dead node's keep-alive never advances.

The rule lives in one place, `stallCounter.observe`, which the tests drive directly rather than
reimplementing, so they cannot pass against a copy that has drifted.

**This is a deliberate divergence from the C# drivers.** In a mixed pair running one of each, the Go
node is the one less willing to take over.

### What the active flag gates, per driver

Two independent axes. Every client gates command execution; most also stop their protocol sessions
on standby. A driver that stops sessions supplies `OnActivate`/`OnDeactivate`, or polls `Active()`
from its own connection loop; a commands-only driver supplies neither.

| Driver | Gates commands | Gates sessions | Source |
|---|---|---|---|
| `dnp3-go` client | yes | yes (callbacks) | `clientapp/commands.go:103`, `clientapp/engine.go:123` |
| `dnp3-go` server | — | — | no arbitration |
| `iec60870-5` clients | yes | yes (polled in `supervise`) | `cliapp/engine.go:386`, `cmd/iec104client/main.go:226` |
| `iec60870-5` servers | — | — | no arbitration |
| `OPC-UA-Client-Go` | yes | **no** (deviation D9) | `mongo_commands.go:97`, `redundancy.go` |
| `iec61850_client` | yes | yes (polled in `connectionLoop`) | `mongo_commands.go:96`, `connection.go:51`, `connection.go:179` |

Only OPC-UA keeps acquiring and writing to MongoDB on the standby node. That is deliberate and
matches its C# original — deviation D9.

An earlier draft of this table claimed `iec60870-5` was commands-only. It is not: `supervise()`
calls `stopClient()` whenever the node is inactive. The error came from reading only the engine and
missing the per-connection loop in each `cmd/*/main.go`.

## Checking your work

```bash
cd src && ./go-check.sh
```

`src/` is not a module, so `go test ./...` from the workspace root does not work; the script loops
over the modules in `go.work`. It builds with `-o /dev/null` on purpose — a plain `go build ./...` in
a `package main` module would overwrite the prebuilt `iccp-*` binaries that `build.sh` ships.
