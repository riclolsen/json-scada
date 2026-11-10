# {json:scada} cs_data_processor.js

This process is responsible for watching updates on the "sourceDataUpdate" field object from documents on the "realtimeData" collection using a change stream. All data sources like client protocol drivers and the calculation engine are expected to update data to the "sourceDataUpdate" field. This schema keeps protocol drivers as simple as possible and standardize data processing across all sources in just one place.

Upcoming data triggers

- Calculation of linear adjustment factors (a.x+b) or digital inversion.
- Update of the main document "value", "valueString", "timeTag\*", quality flags.
- Verification of digital (transition) and analog (limits violation) alarms, signalling on "alarmed" field.
- Update of the "soeData" collection when there is a source time tag for digital points.
- Data forward to PostgreSQL database historian.

This single process handles all upcoming realtime data and can be a bottleneck for the system. It is important to observe the latency and throughput on systems with high volume of upcoming data. Large delays on change stream processing may require a faster server or sharding.

The "timeTag" and "sourceDataUpdate.timeTag" timestamps can be subtracted to find the change stream latency.

Example pipeline to calculate latency (in ms)

    [{
        '$match': {
        'group1': 'a group1 name'
        }
     }, {
        '$set': {
        'diff': {
            '$subtract': [
            '$timeTag', '$sourceDataUpdate.timeTag'
            ]
        }
        }
     }, {
        '$sort': {
        'diff': -1
        }
     }]

Requires Node.js.

## Run

    cd src/cs_data_processor
    npm install
    node cs_data_processor.js [instance number] [log level] [config file]

The optional arguments default to instance `1`, log level `1` and
`../../conf/json-scada.json`; the equivalent environment variables are
`JS_CSDATAPROC_INSTANCE`, `JS_CSDATAPROC_LOGLEVEL` and `JS_CONFIG_FILE`.
Command-line arguments take precedence.

## Process Command Line Arguments And Environment Variables

This driver has the following command line arguments and equivalent environment variables.

- _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**. Env. variable: **JS_CSDATAPROC_INSTANCE**.
- _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**. Env. variable: **JS_CSDATAPROC_LOGLEVEL**.
- _**3rd arg. - Config File Path/Name**_ [String] - Path/name of the JSON-SCADA config file. **Optional argument, default="../../conf/json-scada.json"**. Env. variable: **JS_CONFIG_FILE**.

Env. variable: **JS_CSDATAPROC_DIVIDE_EXP** [String] - A JSON expression to divide change stream processing, will be part of the pipeline for the change stream. E.g: JS_CSDATAPROC_DIVIDE_EXP={ "fullDocument.\_id": { "$mod": [ 2, 1 ] } } or JS_CSDATAPROC_DIVIDE_EXP={"fullDocument.type":"analog"}. Use this to break change stream processing in multiple processes to reduce peak latency. This method can be paired with sharding for best results.
Env. variable: **JS_CSDATAPROC_READ_FROM_SECONDARY** [String] - Use "TRUE" to change the preferred read to a secondary MongoDB server. By default all read operations are directed to the primary server.

Command line args take precedence over environment variables.

## Latency Instrumentation

The process measures the latency of every stage of the pipeline and reports it
as counters and percentiles.

Env. variable: **JS_CSDATAPROC_METRICS_PORT** [Integer] - HTTP port exposing the
metrics. **Default=0 (disabled)**. Endpoints: `/metrics` (JSON),
`/metrics/text` (human readable), `/metrics/prom` (Prometheus),
`/metrics/reset` (clears the counters and restarts the measurement window),
`/health`.
Env. variable: **JS_CSDATAPROC_METRICS_LOG_INTERVAL** [Integer] - Seconds
between metric reports written to the log. **Default=60**, 0 disables.
Env. variable: **JS_CSDATAPROC_METRICS_FILE** [String] - Path of a file to dump
the metrics snapshot as JSON on every report.

A condensed snapshot is also published in the `stats` field of the
`processInstances` document on every redundancy keep-alive.

Stages measured:

| Stage | What it measures |
| --- | --- |
| `sourceToRecv` | Protocol driver's `sourceDataUpdate.timeTag` -> change event delivered to this process |
| `queueWait` | Delivered -> picked up for processing (always 0 here: the change stream callback runs straight on the event loop) |
| `processing` | Conversion and the decision of what to write |
| `writeLinger` | Queued -> included in a flushed batch (bounded by the 150 ms drain cycle) |
| `bulkWrite` | Duration of the `realtimeData` bulk write |
| `endToEnd` | `sourceDataUpdate.timeTag` -> `realtimeData` write completed |
| `soeWrite` | Duration of the `soeData` insert |
| `histWrite` | Duration of the historian insert plus the SQL file write |

### Comparing with the Go implementation

[`src/cs_data_processor-go`](../cs_data_processor-go) is a drop-in replacement
written in Go that replaces the FIFO queues and fixed drain cycles with
channels, goroutines and adaptive batching. It emits **the same** metrics
document, so the two can be compared directly:

    node ../cs_data_processor-go/tools/compare-latency.js --reset --wait 120 http://localhost:8081 http://localhost:8082

Only one of the two implementations may run for a given instance number: they
would both process the same change stream and share the same
`processInstances` entry.

## Communication Status Tags

Instance 1, while active, maintains `_System.Driver.*`, `_System.Conn.*` and
`_System.Comm.*` tags with the health of every protocol driver instance and
connection, for SVG displays and viewers (`commstatus.js`, same logic and tag
documents as `../cs_data_processor-go/commstatus.go`). Turn it off with
`commStatusTags: false` in the _processInstances_ entry. Tag names and points:
[Schema Documentation](../../docs/schema.md#communication-status-tags).
Unit tests: `npm test`.

## Process Instance Collection

A _processInstance_ entry will be created with defaults if one is not found. It can be used to configure some parameters and limit nodes allowed to run instances.

See also

- [Schema Documentation](../../docs/schema.md)
