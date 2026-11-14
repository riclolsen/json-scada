# Change Stream Data Processor (Go)

Go implementation of [`src/cs_data_processor`](../cs_data_processor) (Node.js).

This process watches the MongoDB change stream on `realtimeData` for raw
protocol updates (`sourceDataUpdate`), converts them into analog/status
values, and writes the results to realtime data, the SOE log and the
historians (MongoDB `hist` plus SQL files for PostgreSQL).

It is a **drop-in replacement**: same process name (`CS_DATA_PROCESSOR`), same
command line (`instance loglevel [configFile]`), same environment variable
prefix (`JS_CSDATAPROC_`), same collections, same document shapes and the same
generated SQL files. Only one of the two implementations may run for a given
instance number at a time — they would both process the same change stream and
fight over the same `processInstances` entry.

## Why a second implementation

The Node.js version drains its FIFO queues on fixed `setTimeout` cycles
(150 ms for `realtimeData`, 333 ms for the historian), so every update waits
on average half a cycle before it is even sent to MongoDB. That fixed cost
dominates the end-to-end latency of the pipeline.

Here the queues are replaced by channels and goroutines:

```
   change stream cursor          (1 goroutine: read, copy, hand off)
            |
            v
      changeCh (buffered)
            |
      dispatcher               (hashes the point _id)
       /    |    \
    worker worker worker ...   (parallel conversion, per-point order kept)
       \    |    /
   rtCh   histCh   sqlRtCh   soeCh
      |      |        |        |
   rt writer |   hist + SQL writer   soe writer
             (batch on size OR linger, whichever comes first)
```

What that buys, measured (see *Measuring and comparing latency* below):

- **Adaptive batching instead of a fixed poll.** A writer blocks on its
  channel and only starts a linger timer when the first item of a batch
  arrives. A lone update leaves after the linger (20 ms by default, tunable
  down to 0); a burst is flushed as soon as it fills the batch. The Node.js
  `writeLinger` p50 of 76 ms becomes 20 ms, or 1.9 ms with linger 0.
- **Lazy BSON decoding.** Change events are never unmarshalled into a full
  object graph; only the fields the conversion needs are looked up in the raw
  BSON (`rawdoc.go`). Node.js builds a complete JS object for every event,
  including the whole document fetched by `updateLookup`.
- **Parallel conversion with ordering preserved.** Events are sharded across
  workers by a hash of the point `_id`, so two updates of the same point are
  never reordered while different points proceed in parallel.
- **The cursor is never blocked.** The read loop only copies the event and
  hands it off, so a slow database or a slow batch cannot stall the change
  stream. Under sustained overload the oldest queued event is dropped and
  counted in `droppedOnBackpressure` rather than letting the backlog grow
  without bound.
- **Batched SOE inserts.** Node.js issues one `insertOne` per SOE entry; here
  they are grouped into one `insertMany`, which is why the Go `soeWrite`
  count is the number of batches rather than the number of entries.

## Behaviour parity

Everything the Node.js version does is reproduced: the change stream pipeline
and its `DIVIDE_EXP` sharding, redundancy arbitration, the frozen-analog and
invalid-point timers, the stopped-driver detection that invalidates whole
connections, the two reserved system points, the beep and digital-update
counters, alarm limits with hysteresis, SOE generation, historian dead band,
and both PostgreSQL SQL file formats.

The JavaScript semantics the original depends on are reproduced explicitly in
`jsutil.go`: `Number::toString`, `parseFloat(v.toFixed(n))`,
`Number.toString(2)` for bitstrings, `isNaN(string)` coercion, and a
`JSON.stringify` over BSON that preserves the document's field order (the
`realtime_data.json_data` blob must match byte for byte).

Verified against the Node.js implementation on a single-node replica set: with
a race-free load, the resulting `realtimeData`, `soeData` and `hist`
collections are identical, and the generated `pg_hist_*.sql` /
`pg_rtdata_*.sql` rows are identical after normalising timestamps.

Two deliberate differences:

- SQL files are written to a temporary name and renamed into place, so
  `sql/process_pg_*` can never pick up a half-written file.
- A `sourceDataUpdate` without a `timeTag` is processed instead of throwing
  and being dropped (the Node.js version raises a `TypeError` on
  `timeTag.getTime()` and swallows the whole event).

Communication status tags are implemented in both (`commstatus.go` here,
`commstatus.js` in the Node.js version) with identical tag documents:

- **Communication status tags** (`commstatus.go`). Instance 1, while active,
  keeps `_System.Driver.*`, `_System.Conn.*` and `_System.Comm.*` tags with
  the health of every protocol driver instance and connection (driver alive
  by an unchanged-keep-alive count, link up, tag health, active node), so SVG
  displays and viewers can monitor communications. It polls
  `protocolDriverInstances` / `protocolConnections` every 5 s, counts invalid
  tags per connection every 15 s, and writes through `sourceDataUpdate` so
  the normal conversion adds alarms, SOE and history. Turned off with
  `commStatusTags: false` in the `processInstances` entry. Tag names and
  points are listed in
  [docs/schema.md](../../docs/schema.md#communication-status-tags).

## Building

```bash
go build -ldflags="-s -w" -o cs_data_processor .
```

`build.bat` (Windows) and `build.sh` (Linux/macOS) do the same and place the
binary in the folder. There are no cgo dependencies, so cross-compiling works:

```bash
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o cs_data_processor-linux-amd64 .
```

## Running

```bash
./cs_data_processor <instance> <logLevel> [configFile]
```

Same arguments as the Node.js version. The config file defaults to
`$JS_CONFIG_FILE`, then `../../conf/json-scada.json`, `../conf/json-scada.json`,
`conf/json-scada.json`, `c:\json-scada\conf\json-scada.json` and
`/json-scada/conf/json-scada.json`, whichever exists first.

### Environment variables

| Variable | Default | Meaning |
| --- | --- | --- |
| `JS_CSDATAPROC_INSTANCE` | `1` | Instance number (overridden by argv[1]) |
| `JS_CSDATAPROC_LOGLEVEL` | `1` | 0 min, 1 normal, 2 detailed, 3 debug (overridden by argv[2]) |
| `JS_CSDATAPROC_DIVIDE_EXP` | – | Extra `$match` stage to shard the change stream between instances (JSON) |
| `JS_CSDATAPROC_READ_FROM_SECONDARY` | `false` | Read preference secondaryPreferred |
| `JS_CSDATAPROC_SQL_FILES_PATH` | probed | Folder where `pg_hist_*.sql` / `pg_rtdata_*.sql` are written |
| **latency tuning** | | |
| `JS_CSDATAPROC_RT_WRITE_LINGER_MS` | `20` | Max time an update waits for batch mates; `0` writes immediately |
| `JS_CSDATAPROC_RT_WRITE_MAX_BATCH` | `2000` | Flush when the batch reaches this size |
| `JS_CSDATAPROC_SOE_WRITE_LINGER_MS` | `20` | Same, for `soeData` |
| `JS_CSDATAPROC_SOE_WRITE_MAX_BATCH` | `500` | Same, for `soeData` |
| `JS_CSDATAPROC_HIST_WRITE_LINGER_MS` | `250` | Historian / SQL file flush period |
| `JS_CSDATAPROC_HIST_WRITE_MAX_BATCH` | `5000` | Flush the historian early at this size |
| `JS_CSDATAPROC_WORKERS` | `4` | Conversion goroutines |
| `JS_CSDATAPROC_CHANGE_QUEUE_SIZE` | `65536` | Change event channel capacity |
| `JS_CSDATAPROC_WRITE_QUEUE_SIZE` | `65536` | Writer channel capacity |
| `JS_CSDATAPROC_CS_BATCH_SIZE` | `1000` | Change stream cursor batch size |
| `JS_CSDATAPROC_CS_MAX_AWAIT_MS` | `200` | Change stream `maxAwaitTimeMS` |
| **instrumentation** | | |
| `JS_CSDATAPROC_METRICS_PORT` | `0` (off) | HTTP port for `/metrics` |
| `JS_CSDATAPROC_METRICS_LOG_INTERVAL` | `60` | Seconds between metric log reports, `0` disables |
| `JS_CSDATAPROC_METRICS_FILE` | – | Path to dump the metrics snapshot as JSON on every report |

`RT_WRITE_LINGER_MS=0` gives the lowest possible latency at the cost of one
bulk write per update; the default of 20 ms keeps most of the benefit with a
fraction of the round trips.

## Measuring and comparing latency

Both implementations expose the **same** metrics document, so they can be
compared field by field. Enable the endpoint on each:

```bash
JS_CSDATAPROC_METRICS_PORT=8082 ./cs_data_processor 1 1        # Go
JS_CSDATAPROC_METRICS_PORT=8081 node cs_data_processor.js 1 1  # Node.js
```

Endpoints: `/metrics` (JSON), `/metrics/text` (human readable),
`/metrics/prom` (Prometheus), `/metrics/reset`, `/health`.

The same summary is written to the log every `METRICS_LOG_INTERVAL` seconds,
and a condensed version is published in the `stats` field of the
`processInstances` document on every redundancy keep-alive, so it is visible
without opening a port.

### Stages

| Stage | What it measures |
| --- | --- |
| `sourceToRecv` | Protocol driver's `sourceDataUpdate.timeTag` → event delivered to this process |
| `queueWait` | Delivered → picked up by a worker (always 0 in Node.js, which has no dispatch queue) |
| `processing` | Conversion and the decision of what to write |
| `writeLinger` | Handed to the writer → included in a flushed batch |
| `bulkWrite` | Duration of the `realtimeData` bulk write |
| `endToEnd` | `sourceDataUpdate.timeTag` → `realtimeData` write completed |
| `soeWrite` | Duration of the `soeData` insert |
| `histWrite` | Duration of the historian insert plus the SQL file write |

`endToEnd` is the number to compare: it is measured the same way in both
implementations and covers the whole pipeline. Percentiles come from an
HDR-style logarithmic histogram of microseconds (64 sub-buckets per octave,
worst-case bucket error ~1.6%).

On Windows the monotonic clock behind `time.Now()` ticks at ~0.5 ms, coarser
than several of these stages, so the short in-process stages read the
performance counter directly (`hrtime_windows.go`). Node.js already has
`process.hrtime.bigint()`.

### The comparison tool

```bash
node tools/compare-latency.js http://localhost:8081 http://localhost:8082
node tools/compare-latency.js --reset --wait 120 http://localhost:8081 http://localhost:8082
node tools/compare-latency.js nodejs-snapshot.json go-snapshot.json
```

`--reset` clears both processes' counters, waits, then reports — the usual way
to measure a defined window under an identical load.

### Reference measurement

Windows 11, single-node replica set on the same host, 40 points, 4000 source
updates at ~800 changes/s, both implementations processing the identical load:

| stage (p50 / p99, ms) | Node.js | Go, linger 20 ms | Go, linger 0 |
| --- | --- | --- | --- |
| `sourceToRecv` | 19.1 / 44.8 | 14.5 / 19.8 | 14.9 / 23.7 |
| `processing` | 0.096 / 0.251 | 0.088 / 0.221 | 0.093 / 0.266 |
| `writeLinger` | 76.3 / 152.6 | 19.8 / 20.6 | 1.9 / 4.4 |
| `bulkWrite` | 2.70 / 4.51 | 0.38 / 0.64 | 0.056 / 0.151 |
| **`endToEnd`** | **96.8 / 224.3** | **35.1 / 42.2** | **17.3 / 26.2** |

End to end: **2.8x lower median and 5.3x lower p99** at the default settings,
**5.6x lower median** with the linger disabled. Both implementations kept up
with the offered rate; the win is latency, not throughput.

## Files

| File | Contents |
| --- | --- |
| `main.go` | Startup, change stream connection lifecycle, resume tokens |
| `process.go` | The conversion, port of the change handler; channels and worker pool |
| `writers.go` | Batching writers for realtimeData, historian/SQL files and SOE |
| `metrics.go` | Histogram, counters, HTTP endpoint, reporting |
| `hrtime*.go` | High resolution monotonic clock |
| `redundancy.go` | Active/standby arbitration, publishes stats |
| `maintenance.go` | Frozen analogs, invalid points, stopped driver detection |
| `specialtags.go` | The two reserved system points |
| `commstatus.go` | Communication status tags for driver instances and connections |
| `rawdoc.go` | Typed accessors over raw BSON with JavaScript semantics |
| `jsutil.go` | JavaScript compatible number, string and JSON rendering |
| `config.go` | Config file and environment variables |
| `tools/compare-latency.js` | Side by side report of two metrics snapshots |
