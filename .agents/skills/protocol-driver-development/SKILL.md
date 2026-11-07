---
name: protocol-driver-development
description: Create new protocol drivers (communication drivers) for the JSON-SCADA platform. Covers the MongoDB-centric driver architecture, the protocolDriverInstances / protocolConnections / realtimeData / commandsQueue collections and all their parameters, client (data acquisition) vs server (data distribution) driver patterns, redundancy management, command handling, auto-tag creation, and system integration (AdminUI, process manager catalog, demo seeds, platform services). Includes code snippets for Node.js, C#/.NET, Go and Java. Use when the user asks to create, scaffold, port, or extend a protocol driver, connect a new protocol/fieldbus/API to JSON-SCADA (e.g. Modbus, DNP3, IEC 60870, IEC 61850, OPC, MQTT, BACnet, REST, or any custom protocol), or asks how drivers talk to the JSON-SCADA database.
---

# JSON-SCADA Protocol Driver Development

## Overview

A JSON-SCADA **protocol driver** is a standalone OS process that bridges the central
MongoDB database and external devices/systems. The platform is *unopinionated about
language*: drivers exist in Node.js, C#/.NET, Go and Java. All of them follow the
same contract:

1. Read the main config file `conf/json-scada.json` (MongoDB URI, database name, node name).
2. Load their instance config from the `protocolDriverInstances` collection and their
   connection configs from the `protocolConnections` collection.
3. Manage active/standby **redundancy** through keep-alive fields in their instance document.
4. **Acquire data** from the field and write it into the `sourceDataUpdate` sub-document of
   tags in the `realtimeData` collection (client drivers), and/or **distribute data** to
   remote masters from tags carrying matching `protocolDestinations` entries (server drivers).
5. **Execute commands** picked up from the `commandsQueue` collection (usually via MongoDB
   change streams) and write back delivery/ack status.

Drivers **never** compute the final tag `value`, alarms, or historical records — the
`cs_data_processor` module (src/cs_data_processor) watches `sourceDataUpdate` changes via
change stream and derives `value`, `valueString`, `timeTag`, quality, alarms, SOE and
historian records. A client driver's only write target for data is `sourceDataUpdate`
(plus optional `stats` on its connection document).

Authoritative references in this repo (read them when in doubt):

* `docs/DEVELOPER_GUIDE.md` — full protocol driver developer guide with extended snippets.
* `docs/schema.md` — complete MongoDB schema documentation (every field explained).
* `docs/JSON-SCADA_ARCHITECTURE.png` and `docs/JSON-SCADA_Connections.png` — architecture diagrams.
* `src/<driver>/README.md` — per-driver docs listing that driver's specific connection parameters.

## Driver Types

| Type | Direction | Tag linkage | Examples |
|---|---|---|---|
| **Client (master)** | Field device → JSON-SCADA (+ commands out) | `protocolSourceConnectionNumber`, `protocolSourceObjectAddress`, ... on the tag | IEC 104 client, OPC-UA client, DNP3 client, MQTT client |
| **Server (slave/outstation)** | JSON-SCADA → remote master (+ commands in) | entries in the tag's `protocolDestinations` array | IEC 104 server, OPC-UA server, DNP3 server (Dnp3Server), IEC 61850 server |

A client driver *consumes* commands from `commandsQueue` and sends them to devices.
A server driver *receives* commands from a remote master and *inserts* documents into
`commandsQueue` so that a client driver (or calculation) delivers them onward.

Server drivers select which tags to publish either by explicit `protocolDestinations`
entries or (for browse-style servers like OPC-UA/OPC-DA/IEC 61850 servers) by filtering on
tag `group1` names / `topics` listed in the connection document.

## Standard Process Contract

Command line (identical across all drivers):

```
driver-executable <InstanceNumber> <LogLevel> <PathToConfigFile>
# e.g.:  iec104client 1 1 ../conf/json-scada.json
# e.g.:  node src/n8n-client/index.js 1 1 c:/json-scada/conf/json-scada.json
```

* `InstanceNumber` (default 1) — must match `protocolDriverInstanceNumber` in `protocolDriverInstances`.
* `LogLevel` (0=min, 1=basic, 2=detailed, 3=debug) — command line overrides the instance doc.
* Config path (default `../conf/json-scada.json`). Also honor the `JS_CONFIG_FILE`
  environment variable when present (most drivers do).

`conf/json-scada.json` relevant fields:

```json
{
  "nodeName": "mainNode",
  "mongoConnectionString": "mongodb://user:pwd@127.0.0.1:27017/json_scada?tls=false...",
  "mongoDatabaseName": "json_scada",
  "tlsCaPemFile": "", "tlsClientPemFile": "", "tlsClientPfxFile": "",
  "tlsClientKeyPassword": "", "tlsAllowInvalidHostnames": true,
  "tlsAllowChainErrors": true, "tlsInsecure": false
}
```

Support the Mongo TLS options if practical (see how `src/n8n-client/load-config.js` or
`src/lib60870.netcore` drivers apply them).

## MongoDB Collections and Parameters

### `protocolDriverInstances` (one doc per runnable driver process)

| Field | Type | Meaning |
|---|---|---|
| `protocolDriver` | String | Unique driver name, e.g. `"IEC60870-5-104"`, `"MQTT-SPARKPLUG-B"`, `"MY_PROTOCOL"` |
| `protocolDriverInstanceNumber` | Double | Instance id (1, 2, ...) allowing multiple processes of the same driver |
| `enabled` | Boolean | Instance may run; exit (or idle-retry) if false |
| `logLevel` | Double | 0–3 verbosity |
| `nodeNames` | [String] | Nodes allowed to run this instance (empty = any); used for redundancy |
| `activeNodeName` | String | Managed by the driver: node currently active |
| `activeNodeKeepAliveTimeTag` | Date | Managed by the driver: heartbeat of the active node |
| `keepProtocolRunningWhileInactive` | Boolean | Usually false; if true a standby keeps protocol links up |

### `protocolConnections` (one doc per link/endpoint; N per instance)

Common fields every driver must honor:

| Field | Type | Meaning |
|---|---|---|
| `protocolDriver` | String | Must match the instance's driver name |
| `protocolDriverInstanceNumber` | Double | Which instance owns this connection |
| `protocolConnectionNumber` | Double | **System-wide unique** number; tags reference it |
| `name` | String | Short unique connection name (used in `originator`, logs, stats) |
| `description` | String | Free text |
| `enabled` | Boolean | Connection active |
| `commandsEnabled` | Boolean | Process commands for this connection |
| `stats` | Object/null | Optional driver-maintained counters (writes allowed) |

Protocol-specific fields are added freely, but **reuse existing field names** where they fit
so the AdminUI forms work unchanged. Frequently reused names (see `docs/schema.md` and
existing drivers): `ipAddressLocalBind`, `ipAddresses` (array), `endpointURLs` (array),
`topics` (array), `username`, `password`, `passphrase`, `localCertFilePath`,
`peerCertFilePath`, `rootCertFilePath`, `chainValidation`, `allowTLSv10/11/12/13`,
`cipherList`, `giInterval` (general interrogation/integrity poll seconds), `pollingInterval`,
`timeoutMs`, `deadBand`, `hoursShift`, `maxQueueSize`, `options` (free-form string/JSON for
extra settings), `autoCreateTags`, `commandTimeout` — plus IEC-style link-layer fields
(`baudRate`, `parity`, `stopBits`, `handshake`, `localLinkAddress`, `remoteLinkAddress`, ...).

Example (the N8N driver connection from the demo seed):

```json
{
  "protocolDriver": "N8N", "protocolDriverInstanceNumber": 1,
  "protocolConnectionNumber": 9200, "name": "N8N1",
  "description": "n8n integration link", "enabled": false, "commandsEnabled": true,
  "autoCreateTags": true, "ipAddressLocalBind": "0.0.0.0:51930",
  "ipAddresses": [], "topics": [], "endpointURLs": ["http://n8n:5678/webhook/jsonscada-demo"],
  "username": "n8n", "password": "jsonscada", "passphrase": "",
  "giInterval": 0, "deadBand": 0, "maxQueueSize": 5000, "options": "", "stats": null
}
```

### `realtimeData` (the tags)

Tag fields a **client driver reads** to know what/how to acquire:

| Field | Type | Meaning |
|---|---|---|
| `protocolSourceConnectionNumber` | Double | Links the tag to one of the driver's connections |
| `protocolSourceObjectAddress` | String/Double | Protocol point address (NodeId, IOA, register, topic, ...) |
| `protocolSourceCommonAddress` | String/Double | Device/sector/group address (e.g. IEC CA, DNP3 group) |
| `protocolSourceASDU` | String/Double | Protocol type id (e.g. `"M_ME_NC_1"`, `13`, `"FLOAT32"`) |
| `protocolSourceCommandUseSBO` | Boolean | Command requires Select-Before-Operate |
| `protocolSourceCommandDuration` | Double | Command qualifier (pulse duration etc.) |
| `kconv1` / `kconv2` | Double | Scaling: `value = valueAtSource * kconv1 + kconv2` (kconv1=-1 inverts digitals) — applied by cs_data_processor, *not* by the driver |
| `interrogationOaDbId` / protocol-specific extras | — | Only if your protocol needs them |
| `type` | String | JSON-SCADA type: `"digital"`, `"analog"`, `"string"`, `"json"` |
| `origin` | String | `"supervised"` (data point), `"command"` (control point) |

A **client driver writes** new values *only* as a `$set` of the `sourceDataUpdate` object:

```json
"sourceDataUpdate": {
  "valueAtSource": 123.45,
  "valueStringAtSource": "123.45",
  "valueJsonAtSource": "{...}",
  "asduAtSource": "M_ME_NC_1",
  "causeOfTransmissionAtSource": "3",
  "timeTagAtSource": {"$date": "..."},      // device timestamp (or null)
  "timeTagAtSourceOk": true,                 // device timestamp trustworthy
  "timeTag": {"$date": "..."},              // local time of processing
  "invalidAtSource": false,
  "notTopicalAtSource": false,
  "substitutedAtSource": false,
  "blockedAtSource": false,
  "overflowAtSource": false,
  "transientAtSource": false,
  "carryAtSource": false,
  "originator": "MY_PROTOCOL|9999|CONN_NAME"
}
```

Filter by `{protocolSourceConnectionNumber, protocolSourceObjectAddress}` (add
`protocolSourceCommonAddress` when addresses repeat across groups). Never upsert from the
update path; use **bulk writes** for bursts (all high-volume drivers batch updates on a
queue flushed every few hundred ms).

Tag fields a **server driver reads** — each tag holds a `protocolDestinations` **array**
(one entry per server connection publishing that tag):

```json
"protocolDestinations": [{
  "protocolDestinationConnectionNumber": 1001,
  "protocolDestinationCommonAddress": 1,
  "protocolDestinationObjectAddress": 3285,
  "protocolDestinationASDU": 13,
  "protocolDestinationCommandDuration": 0,
  "protocolDestinationCommandUseSBO": false,
  "protocolDestinationKConv1": 1, "protocolDestinationKConv2": 0,
  "protocolDestinationGroup": 0, "protocolDestinationHoursShift": 0
}]
```

Server drivers watch `realtimeData` change streams (on `sourceDataUpdate`/value changes)
and forward updates whose `protocolDestinations` match their connection numbers, applying
`KConv1/KConv2` scaling and `HoursShift` themselves (see
`src/lib60870.netcore/iec104server/TagsDestinations.cs`).

### `commandsQueue`

Inserted by the UI/server or by server drivers; consumed by client drivers (change stream
on inserts; ignore docs older than ~10s at startup to avoid replaying stale commands).

| Field | Who writes | Meaning |
|---|---|---|
| `protocolSourceConnectionNumber` | inserter | Target connection (client driver filters on it) |
| `protocolSourceCommonAddress`, `protocolSourceObjectAddress`, `protocolSourceASDU`, `protocolSourceCommandDuration`, `protocolSourceCommandUseSBO` | inserter | Copied from the command tag |
| `pointKey`, `tag` | inserter | `_id` and tag name of the command tag |
| `value`, `valueString` | inserter | Command value |
| `timeTag`, `originatorUserName`, `originatorIpAddress` | inserter | Audit info |
| `delivered` (Boolean), `deliveredTimeTag` | **driver** | Command dispatched to the device |
| `ack` (Boolean), `ackTimeTag` | **driver** | Positive (`true`) / negative (`false`) confirmation |
| `cancelReason`, `resultDescription` | **driver** | Diagnostics |

Server drivers inserting commands should fill the `protocolSource*` fields from the target
tag's configuration and set `originatorUserName` to something like
`"Protocol connection: CONN_NAME"`.

Feedback to the remote master (command termination on server drivers) is achieved by
watching the command tag's value/ack updates or the `commandsQueue` ack fields.

## Core Logic Blueprint

Every driver implements this loop structure (details and full snippets in
`docs/DEVELOPER_GUIDE.md`):

1. **Startup** — parse args, load `json-scada.json`, connect MongoDB, load instance +
   enabled connections; exit if instance missing/disabled.
2. **Redundancy task** (every ~5–10 s): if `activeNodeName == nodeName` → refresh
   `activeNodeKeepAliveTimeTag`; else if empty or stale (> ~30–60 s) → take over by setting
   both fields; else stay inactive (close protocol links unless
   `keepProtocolRunningWhileInactive`).
3. **Acquisition** (active only) — connect per protocol, poll/subscribe, enqueue
   `sourceDataUpdate` bulk writes.
4. **Commands** (active only, `commandsEnabled` connections) — change stream on
   `commandsQueue` inserts, filter by connection number, dispatch, update
   `delivered`/`ack`.
5. **Shutdown** — handle SIGINT/SIGTERM, close protocol and Mongo connections.
6. **Resilience** — auto-reconnect Mongo and protocol links forever; a driver must survive
   database restarts (re-open change streams) and device outages.

### Auto-tag creation (optional, recommended for browsable protocols)

When the connection has `autoCreateTags: true`, discover points and insert missing tags
into `realtimeData`: derive a unique `tag` name (convention: prefix with connection name,
e.g. `CONN_NAME.discovered_path`), set `protocolSource*` fields, `type` inferred from the
protocol data type, `origin: "supervised"` (or create command twins with
`origin: "command"`), `kconv1: 1, kconv2: 0`, `invalid: true` initially, and allocate
`_id`/`pointKey` from a driver-specific numeric range to avoid collisions. Respect
`topics`/browse-path filters from the connection doc, and `NoRemoteCommands`-style guards
where applicable. Complete implementations to copy:
`src/OPC-UA-Client/` (`autoCreateTags*` parameters), `src/lib60870.netcore/*/TagsCreation.cs`,
`src/dnp3-go/internal/clientapp/`, `src/n8n-client/tags-creation.js`.

## Language Snippets

Pick the language that best fits the protocol library available. Below are the essential
idioms; copy fuller versions from the referenced drivers.

### Node.js (see src/n8n-client — small, modern, complete)

```javascript
const { MongoClient, Double } = require('mongodb')

// ---- update a tag (client driver data path) ----
async function queueUpdate(rtCollection, connNumber, objAddr, value, quality) {
  await rtCollection.updateOne(
    { protocolSourceConnectionNumber: connNumber, protocolSourceObjectAddress: objAddr },
    { $set: { sourceDataUpdate: {
        valueAtSource: new Double(value),
        valueStringAtSource: String(value),
        invalidAtSource: !quality.ok, notTopicalAtSource: false,
        substitutedAtSource: false, blockedAtSource: false,
        overflowAtSource: false, transientAtSource: false, carryAtSource: false,
        timeTagAtSource: quality.srcTime || null,
        timeTagAtSourceOk: !!quality.srcTime,
        timeTag: new Date(), asduAtSource: 'FLOAT32',
        causeOfTransmissionAtSource: '3',
        originator: `MY_PROTOCOL|${connNumber}`,
    }}})
}

// ---- consume commands (change stream, resumable) ----
async function watchCommands(cmdCollection, myConnections, sendToDevice) {
  const cs = cmdCollection.watch(
    [{ $match: { operationType: 'insert' } }],
    { fullDocument: 'updateLookup' })
  cs.on('change', async (change) => {
    const cmd = change.fullDocument
    const conn = myConnections[cmd?.protocolSourceConnectionNumber]
    if (!conn || !conn.commandsEnabled) return
    if (new Date() - cmd.timeTag > 10000) return // stale command
    const ok = await sendToDevice(conn, cmd)
    await cmdCollection.updateOne({ _id: cmd._id },
      { $set: { delivered: true, deliveredTimeTag: new Date(), ack: ok, ackTimeTag: new Date() } })
  })
  cs.on('error', () => setTimeout(() => watchCommands(cmdCollection, myConnections, sendToDevice), 5000))
}
```

Reusable helper modules exist in most Node drivers: `load-config.js` (args + json-scada.json +
Mongo TLS options), `simple-logger.js`, `redundancy.js`, `app-defs.js` (driver name/version
constants). Copy them from `src/n8n-client/` or `src/telegraf-listener/`.

### C#/.NET 8 (see src/lib60870.netcore/iec104client — canonical C# client)

```csharp
// Bulk data path with a concurrent queue (pattern used by all C# drivers)
public static ConcurrentQueue<BsonDocument> UpdateQueue = new();

// producer: protocol callback
UpdateQueue.Enqueue(new BsonDocument {
  { "protocolSourceConnectionNumber", conNumber },
  { "protocolSourceObjectAddress", objectAddress },
  { "sourceDataUpdate", new BsonDocument {
      { "valueAtSource", BsonDouble.Create(value) },
      { "valueStringAtSource", value.ToString("G") },
      { "invalidAtSource", BsonBoolean.Create(invalid) },
      { "timeTagAtSource", srcTimestamp is null ? BsonNull.Value : BsonValue.Create(srcTimestamp) },
      { "timeTagAtSourceOk", BsonBoolean.Create(srcTimestamp != null) },
      { "timeTag", BsonValue.Create(DateTime.Now) },
      { "asduAtSource", typeName },
      { "causeOfTransmissionAtSource", cot.ToString() },
      { "originator", ProtocolDriverName + "|" + conNumber },
  }}});

// consumer task: flush every ~250ms
var bulkOps = new List<WriteModel<rtData>>();
while (UpdateQueue.TryDequeue(out var upd)) {
  var filter = Builders<rtData>.Filter.And(
    Builders<rtData>.Filter.Eq("protocolSourceConnectionNumber", upd["protocolSourceConnectionNumber"]),
    Builders<rtData>.Filter.Eq("protocolSourceObjectAddress", upd["protocolSourceObjectAddress"]));
  bulkOps.Add(new UpdateOneModel<rtData>(filter,
    Builders<rtData>.Update.Set("sourceDataUpdate", upd["sourceDataUpdate"])));
}
if (bulkOps.Count > 0)
  collectionRtData.BulkWrite(bulkOps, new BulkWriteOptions { IsOrdered = false });
```

Structure: `Program.cs` (main + redundancy), `ConfigInfo`/`*_connection` classes with the
Mongo document mapping (`[BsonDefaultValue]` attributes give parameter defaults),
`AsduReceiveHandler.cs`-style protocol callbacks, `MongoTaskWriteRtData.cs` (bulk flush),
`CommandsQueueHandler.cs`-style change stream watcher (`collection.Watch()` +
`ChangeStreamOperationType.Insert`). Server-side C# example: `src/lib60870.netcore/iec104server/`
(watches `realtimeData`, applies `protocolDestinations`). C# gateway with a native library
via P/Invoke: `src/iec61850_server/`.

### Go (see src/iccp/iccp-client, src/plc4x-client, src/calculations)

```go
type sourceDataUpdate struct {
    ValueAtSource       float64   `bson:"valueAtSource"`
    ValueStringAtSource string    `bson:"valueStringAtSource"`
    InvalidAtSource     bool      `bson:"invalidAtSource"`
    NotTopicalAtSource  bool      `bson:"notTopicalAtSource"`
    TimeTagAtSource     time.Time `bson:"timeTagAtSource"`
    TimeTagAtSourceOk   bool      `bson:"timeTagAtSourceOk"`
    TimeTag             time.Time `bson:"timeTag"`
    AsduAtSource        string    `bson:"asduAtSource"`
    CauseOfTransmissionAtSource string `bson:"causeOfTransmissionAtSource"`
    Originator          string    `bson:"originator"`
}

// bulk write pattern
models := []mongo.WriteModel{}
for _, upd := range pending {
    models = append(models, mongo.NewUpdateOneModel().
        SetFilter(bson.M{
            "protocolSourceConnectionNumber": upd.connNumber,
            "protocolSourceObjectAddress":    upd.objAddr,
        }).
        SetUpdate(bson.M{"$set": bson.M{"sourceDataUpdate": upd.sdu}}))
}
if len(models) > 0 {
    _, err := rtCol.BulkWrite(context.TODO(), models,
        options.BulkWrite().SetOrdered(false))
    _ = err // log it
}

// commands change stream
cs, _ := cmdCol.Watch(context.TODO(),
    mongo.Pipeline{bson.D{{Key: "$match", Value: bson.D{{Key: "operationType", Value: "insert"}}}}})
for cs.Next(context.TODO()) {
    var ev struct{ FullDocument bson.M `bson:"fullDocument"` }
    cs.Decode(&ev)
    // filter by protocolSourceConnectionNumber, dispatch, then:
    cmdCol.UpdateOne(context.TODO(), bson.M{"_id": ev.FullDocument["_id"]},
        bson.M{"$set": bson.M{"delivered": true, "ack": ok, "ackTimeTag": time.Now()}})
}
```

Use goroutines per connection, `context.Context` for cancellation, and `bson` struct tags
for configs. Full redundancy loop examples: `src/calculations/main.go`, `src/mongofw/`.

### Java (see src/plc4j-client — Maven + Apache PLC4X)

Uses `org.mongodb:mongodb-driver-sync`; same document shapes. Note the Maven shade plugin
must keep `ServicesResourceTransformer` so PLC4X driver discovery works in the fat jar.

## Complete Existing Drivers (study these)

| Driver | Path | Language | Highlights |
|---|---|---|---|
| IEC 60870-5-104 client/server | `src/lib60870.netcore/iec104client`, `iec104server` | C# | Canonical client & server patterns, TagsCreation/TagsDestinations, SBO |
| IEC 60870-5-101/103 (Go) | `src/iec60870-5/` | Go | Go rewrite of IEC drivers |
| DNP3 client/server | `src/dnp3-go/cmd/dnp3client`, `dnp3server` | Go | autoCreateTags, TLS |
| OPC-UA client | `src/OPC-UA-Client` | C# | Subscriptions, browsing/auto-tag, certs |
| OPC-UA server | `src/OPC-UA-Server` | Node.js | node-opcua, group1/topics filtering |
| OPC-DA client/server | `src/OPC-DA-Client`, `src/OPC-DA-Server` | C# | Legacy COM interop |
| IEC 61850 client | `src/iec61850_client` | C# | MMS reads/reports via libiec61850 |
| IEC 61850 client (Go) | `src/iec61850/iec61850_client` | Go | Same driver name, pure-Go stack, drop-in |
| IEC 61850 server (Go) | `src/iec61850/iec61850_server` | Go | Dynamic model from realtimeData, DO-level data sets, controls |
| IEC 61850 server | `src/iec61850_server` | C# + native lib | Dynamic model, RCBs, controls → commandsQueue |
| MQTT / Sparkplug-B | `src/mqtt-sparkplug` | Node.js | Pub/sub both directions, TLS, JSON payloads |
| ICCP/TASE.2 client/server | `src/iccp/iccp-client`, `iccp-server` | Go | Datasets, TLS, BLT |
| PLC4X client | `src/plc4x-client` | Go | Multi-protocol (Modbus/S7/AB...) |
| PLC4J client | `src/plc4j-client` | Java | Same catalog name "PLC4X", drop-in |
| libplctag client | `src/libplctag/PLCTagsClient` | C# | Allen-Bradley/Modbus tags |
| Telegraf listener | `src/telegraf-listener` | Node.js | UDP ingest, auto-tag, simplest Node client |
| N8N driver | `src/n8n-client` | Node.js | Webhooks out / HTTP listener in — cleanest small modern Node.js driver |
| I104M, MongoFW/MongoWR | `src/i104m`, `src/mongofw`, `src/mongowr` | Go | UDP legacy protocol; database replication drivers |
| ONVIF camera | `src/camera-onvif` | Node.js | Media/event protocol example |

Also `src/demo_simul` (Node.js) simulates field data by writing `sourceDataUpdate` — useful
to understand the minimal data path, and for testing without real devices.

## System Integration Checklist (registering a new driver)

Creating `src/<my-driver>/` with working code is only half the job. To make the driver a
first-class citizen, wire it into (grep for an existing name like `"N8N"` or
`"TELEGRAF-LISTENER"` to find every spot):

1. **Driver name** — pick a unique `protocolDriver` string (UPPERCASE, e.g. `MY_PROTOCOL`,
   suffix `_SERVER` for server drivers).
2. **AdminUI** — add the name to the driver lists in
   `src/AdminUI/src/components/ProtocolDriverInstancesTab.vue` and
   `src/AdminUI/src/components/ProtocolConnectionsTab.vue` (also gate which parameter
   fields show for it), plus i18n strings in `src/AdminUI/src/locales/en.json` / `pt.json`
   if new labels are needed. Rebuild AdminUI.
3. **Process manager catalog** — add an entry in
   `src/server_realtime_auth/app/services/process-manager/driver-catalog.js` so the
   AdminUI can create/start/stop the service:
   ```js
   MY_PROTOCOL: {
     key: 'myprotocol',            // service base name: JSON_SCADA_myprotocol
     type: 'node',                 // or 'exe' with exe: '{bin}/mydriver'
     script: '{src}/my-driver/index.js',
     passConfPath: true,
     defaultStartMode: 'manual',
   },
   ```
4. **Demo/seed configs** — add disabled example instance + connection docs to
   `demo-docker/mongo_seed/files/demo_instances.json`, `demo_connections.json` and
   `demo_connections_linux.json` (unique `$oid`s and `protocolConnectionNumber`).
5. **Windows services** — `platform-windows/create_services.bat` (nssm install block),
   `remove_services.bat`, `start_protocols.bat` / `stop_protocols.bat` /
   `restart_protocols.bat`, and build steps in `platform-windows/build.bat` / `buildupd.bat`.
6. **Linux** — build steps in `platform-linux/build.sh` and `[program:...]` sections in
   `platform-rhel9/supervisord.conf`, `platform-rhel10/supervisord.conf`,
   `platform-ubuntu-2404/supervisord.conf`, `platform-ubuntu-2604/supervisord.conf`
   (create with `autostart=false` unless it should run by default).
7. **Docker demo** — `demo-docker/docker-compose.yaml` if the driver should appear in the demo.
8. **Log viewer** — add the driver's log file to `conf-templates/log.io-file.json`.
9. **Docs** — write `src/<my-driver>/README.md` documenting every connection parameter
   (copy the structure of an existing driver README), link it from the main `README.md`
   protocol list, then regenerate `index.md` with `node docs/sync-index.mjs` (never edit
   `index.md` by hand).
10. **Release notes** — mention the driver in `platform-windows/release_notes.txt`.

## Conventions

* GPL-3.0 header comment (`{json:scada} - Copyright (c) 2020-20xx - Ricardo L. Olsen`) at
  the top of source files.
* Driver announces itself on stdout at startup: name, version, copyright.
* Version constants live in the source (`AppDefs`/`app-defs.js`/`const` block); bump on change.
* Logging to stdout only — process managers redirect to `log/<driver>.log`.
* Numbers stored in Mongo config are BSON Doubles — read them tolerantly (accept
  Int32/Int64/Double; Node: `?.valueOf()`, Go: switch on type, C#: `ToDouble()`).
* Do not create Mongo indexes or alter other collections from a driver.
* Never write tag `value`/alarms directly — only `sourceDataUpdate` (cs_data_processor does
  the rest).

## Testing

* **Unit/self-test without MongoDB**: several drivers include a loopback self-test mode
  (e.g. `src/n8n-client/test`, ICCP client/server loopback pattern) — protocol client and
  server instances of the driver talk to each other in-process and assert round-trips.
* **End-to-end**: run MongoDB (demo-docker or local), seed instance + connection docs,
  run the driver with `<instance> <loglevel> <conf>`, watch tag updates:
  ```js
  db.realtimeData.find({ protocolSourceConnectionNumber: 9999 },
                       { tag: 1, sourceDataUpdate: 1 })
  ```
  and insert a test command into `commandsQueue` to validate the command path, checking
  `delivered`/`ack` afterwards.
* **Redundancy**: start two instances with different `nodeName`s and kill the active one;
  the standby must take over within the keep-alive timeout.
* Simulate data with `src/demo_simul` when no field device is available.
