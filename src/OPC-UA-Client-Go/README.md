# OPC-UA Client (Go)

A driver that connects JSON-SCADA to OPC UA servers. It reads data from the server into the
JSON-SCADA real-time database and carries operator commands back to the server.

It is a single executable with no runtime to install, built on the pure-Go
[gopcua](https://github.com/gopcua/opcua) library.

What it does:

- connects to one or more OPC UA servers, each with its own security settings and identity;
- **discovers the server's address space and creates the JSON-SCADA tags for you**, or updates tags
  you configured by hand;
- subscribes to the points, so values arrive when they change instead of being polled;
- executes **commands** (writes to variables, calls to methods) queued by JSON-SCADA operators;
- recovers from lost connections, fails over between redundant servers, and takes part in
  JSON-SCADA's active/standby node redundancy.

## Contents

- [How it works](#how-it-works)
- [Requirements](#requirements)
- [Installing](#installing)
- [Quick start](#quick-start)
- [Running the driver](#running-the-driver)
- [Configuration: the driver instance](#configuration-the-driver-instance)
- [Configuration: connections](#configuration-connections)
- [Security and certificates](#security-and-certificates)
- [Tags](#tags)
- [Commands](#commands)
- [Redundancy](#redundancy)
- [Reconnection and recovery](#reconnection-and-recovery)
- [Logging and troubleshooting](#logging-and-troubleshooting)
- [Limits and sizing](#limits-and-sizing)
- [Known limitations](#known-limitations)
- [Moving from the .NET driver](#moving-from-the-net-driver)
- [Building and testing](#building-and-testing)

## How it works

```
 OPC UA server ──subscription──▶  driver  ──sourceDataUpdate──▶  MongoDB  ──▶  JSON-SCADA
      ▲                             │         (realtimeData)       │
      └───────── Write / Call ──────┘◀── commandsQueue (change stream)
```

- The driver never computes a tag's final value, alarms or history. It only writes what the server
  reported into the tag's `sourceDataUpdate`; JSON-SCADA's data processor derives everything else.
- All configuration lives in MongoDB: a **driver instance** document says which node runs the
  driver, and one or more **connection** documents describe the servers.
- One process can hold many connections. Run several processes (distinct *instance numbers*) when
  you want to separate them.

## Requirements

- A JSON-SCADA installation: a MongoDB **replica set** (commands are picked up through a change
  stream, which MongoDB only offers on a replica set — the standard JSON-SCADA setup), and
  `conf/json-scada.json` pointing at it.
- Network access from the driver's machine to the OPC UA servers.
- For secure connections, a client certificate the server trusts (see
  [Security and certificates](#security-and-certificates)).

## Installing

The driver is built together with the rest of JSON-SCADA: the platform build scripts
(`platform-windows/build.bat`, `platform-linux/build.sh`, `platform-mac/build.sh`) put it in `bin/`
as `opcua-client` (`opcua-client.exe` on Windows).

To build only this driver, from a JSON-SCADA checkout (it needs the repository's `go-common`
directory next to it) with Go 1.26 or later:

```bash
cd src/OPC-UA-Client-Go
go build -ldflags="-s -w" -o ../../bin/opcua-client
```

### Running it as a service

| Platform | How |
|---|---|
| Linux (supervisor) | Enable `opcua_goclient.ini` from the platform folder (`platform-ubuntu-*`, `platform-rhel*`). It is installed with `autostart=false`. |
| Windows | `platform-windows/create_services.bat` installs the `JSON_SCADA_opcuagoclient` service with manual start. |
| AdminUI process manager | Put `processExecutableVariant: "opcuago"` in the driver instance document and the process manager starts this binary for the `OPC-UA` driver. |

> **Run only one OPC-UA client per instance number.** If the .NET driver (`OPC-UA-Client`) is also
> installed, enable one or the other for a given instance, never both: both would connect to the
> same servers and write the same tags.

## Quick start

Everything below is typed in `mongosh`, connected to the JSON-SCADA database.

**1. Describe the driver instance.** `nodeNames` must contain this machine's `nodeName` from
`conf/json-scada.json`.

```javascript
db.protocolDriverInstances.insertOne({
  protocolDriver: "OPC-UA",
  protocolDriverInstanceNumber: 1,
  enabled: true,
  nodeNames: ["mainNode"],
  activeNodeName: "mainNode",
  activeNodeKeepAliveTimeTag: new Date()
});
```

**2. Describe the connection.** The only fields you must change are `protocolConnectionNumber`
(unique across the whole system), `name` and `endpointURLs`.

```javascript
db.protocolConnections.insertOne({
  protocolDriver: "OPC-UA",
  protocolDriverInstanceNumber: 1,
  protocolConnectionNumber: 81,
  name: "PLC1",
  description: "Boiler PLC",
  enabled: true,
  commandsEnabled: true,
  endpointURLs: ["opc.tcp://myserver:4840"],
  autoCreateTags: true,
  autoCreateTagPublishingInterval: 5,
  autoCreateTagSamplingInterval: 5,
  autoCreateTagQueueSize: 5,
  useSecurity: false
});
```

**3. Start the driver** from the `bin` directory:

```
opcua-client 1 1
```

Within a minute the log shows `Session created successfully.`, then the discovery of the server's
address space, then `Running...`. The tags appear in `realtimeData` under group `PLC1`.

## Running the driver

```
opcua-client [instance] [logLevel] [configFile]
```

| Argument | Meaning | Default |
|---|---|---|
| `instance` | `protocolDriverInstanceNumber` of the instance to run | `1` |
| `logLevel` | `0` minimal, `1` basic, `2` detailed, `3` debug | `1` |
| `configFile` | path of the JSON-SCADA configuration file | see below |

The configuration file is the first of these that exists: the `configFile` argument, the file named
by the `JS_CONFIG_FILE` environment variable, `../conf/json-scada.json`, and
`c:/json-scada/conf/json-scada.json`.

Relative paths — the configuration file, certificates, `conf/opcua/` — are resolved from the
**working directory**, so run the driver from the `bin` directory (the service definitions already
do). Stop it with Ctrl-C or `SIGTERM`.

**Log level.** It is taken from the command line only. The `logLevel` field of the instance
document is not used by this driver.

**Startup errors.** The process exits with status −1 (255 on Linux) after logging one of:

| Message | Cause |
|---|---|
| `Missing config file ...` / `Missing ... in JSON config file` | `json-scada.json` not found, or `mongoConnectionString`, `mongoDatabaseName` or `nodeName` empty |
| `Error connecting to MongoDB ...` | MongoDB unreachable (20 s timeout) |
| `Driver instance [N] not found in configuration!` | no **enabled** instance document for this driver and number — a disabled instance reports this too |
| `Node 'X' not found in instances configuration!` | this machine's `nodeName` is not in the instance's `nodeNames` (an empty list allows any node) |
| `Missing remote endpoint URLs list!` | a connection has an empty `endpointURLs` |
| `No connections found!` | no enabled connection for this instance |

If several instance documents match, only the first is used.

## Configuration: the driver instance

Collection `protocolDriverInstances`.

| Field | Type | Meaning |
|---|---|---|
| `protocolDriver` | string | Must be `"OPC-UA"`. |
| `protocolDriverInstanceNumber` | number | The instance number; unique per driver. Several processes can run with different numbers. |
| `enabled` | boolean | `false` stops the instance from starting. |
| `nodeNames` | array of strings | Names of the nodes allowed to run this instance. Use two for [redundancy](#redundancy). Empty allows any node. |
| `activeNodeName` | string | The node currently active. Maintained by the drivers; set it to your node name when you first create the instance. |
| `activeNodeKeepAliveTimeTag` | date | Refreshed by the active node every 5 s. Maintained by the drivers. |
| `processExecutableVariant` | string | Read by the AdminUI process manager only: `"opcuago"` selects this binary. |

`logLevel` and `keepProtocolRunningWhileInactive` may be present in the document but this driver
does not read them. Changes to the instance document take effect after a restart.

## Configuration: connections

Collection `protocolConnections`, one document per server. Changes take effect after a restart.

### Identity and behaviour

| Field | Default | Meaning |
|---|---|---|
| `protocolDriver` | — | Must be `"OPC-UA"`. |
| `protocolDriverInstanceNumber` | — | The instance that runs this connection. |
| `protocolConnectionNumber` | — | Number of the connection, **unique across all drivers in the system**. Tags name it to say who may update them. It also fixes the range of `_id` values used for auto-created tags (see [Tags](#automatic-tag-creation)). |
| `name` | `"NO NAME"` | Short name. It prefixes every tag the driver creates and appears in every log line, so keep it short and stable. |
| `description` | — | Free text. |
| `enabled` | `true` | `false` skips the connection. |
| `commandsEnabled` | `true` | `false` refuses all commands for this connection, and also stops the driver from creating command tags and discovering methods. |

### Server and session

| Field | Default | Meaning |
|---|---|---|
| `endpointURLs` | — | **Required.** One or more `opc.tcp://host:port/path` URLs. With more than one, the driver moves to the next URL after each failed connection attempt, so a redundant server pair is covered. All URLs must belong to the same logical server: they share one tag set. |
| `timeoutMs` | `20000` | Time allowed for connecting, for endpoint discovery, and for each request to the server. |
| `configFileName` | `../conf/Opc.Ua.DefaultClient.Config.xml` | Optional XML file. Only `ApplicationName`, `ApplicationUri` and `ProductUri` are read from it; the rest of an OPC Foundation configuration file is ignored. If it is missing the defaults `JSON-SCADA OPC-UA Client` and `urn:localhost:OPCUA:JSON_SCADA_OPCUAClient` are used. |
| `hoursShift` | `0` | Hours added to the timestamps the server provides. Use it to correct a server that stamps local time instead of UTC. |

### Security and identity

| Field | Default | Meaning |
|---|---|---|
| `useSecurity` | `false` | `false` forces an unsecured connection and ignores `securityMode` and `securityPolicy`. |
| `securityMode` | `"None"` | `None`, `Sign` or `SignAndEncrypt`. |
| `securityPolicy` | `"None"` | `None`, `Basic128Rsa15`, `Basic256`, `Basic256Sha256`, `Aes128_Sha256_RsaOaep` or `Aes256_Sha256_RsaPss`. |
| `localCertFilePath` | — | The client's own certificate and private key (`.pfx`, `.p12`, or PEM). Empty means the driver generates one. |
| `passphrase` | — | Password of `localCertFilePath` and of `pfxFilePath`. |
| `username`, `password` | — | If `username` is set, the driver logs in with user name and password. |
| `pfxFilePath` | — | If `username` is empty and this is set, the driver logs in with this user certificate and its private key. |
| `autoAcceptUntrustedCertificates` | `true` | `true` accepts any server certificate without checking it; `false` checks it. See [Security and certificates](#security-and-certificates). |

The identity is chosen in this order: user name, then user certificate, then anonymous.

### Automatic tags

| Field | Default | Meaning |
|---|---|---|
| `autoCreateTags` | `true` | `true`: discover the server's address space, create the tags and subscribe to them. `false`: only the tags you configured are updated. |
| `autoCreateTagPublishingInterval` | `5` | Seconds between the server's reports for auto-created tags. |
| `autoCreateTagSamplingInterval` | `5` | Seconds between the server's samples of each point. `0` lets the server choose, usually its fastest rate. |
| `autoCreateTagQueueSize` | `5` | How many changes the server buffers per point between reports. |
| `topics` | `[]` | Restricts discovery to parts of the address space. See [Choosing what to discover](#choosing-what-to-discover). |

### Maintained by the driver

| Field | Meaning |
|---|---|
| `stats` | `{ nodeName, timeTag }`, refreshed every 5 s while this node is active. A heartbeat other tools can watch. |
| `giInterval` | Accepted, ignored. |

### Several connections

One instance can hold any number of connections; each is independent and has its own session,
subscriptions and security. Give each a different `protocolConnectionNumber` and `name`.

## Security and certificates

### Choosing the endpoint

The driver asks the server for its list of endpoints and picks one, in this order:

1. an endpoint whose policy **and** mode match `securityPolicy` and `securityMode`;
2. otherwise the first endpoint whose mode matches;
3. otherwise the first endpoint the server offers.

> **Steps 2 and 3 can connect with a different security setting than the one you configured.**
> The log says which endpoint was used (`Selected endpoint uses: Basic256Sha256`, and the line
> before it says how it was chosen). After setting up a secure connection, check that line.

If the server cannot be asked, the driver builds an endpoint from your settings and tries that.
The URL you configured is always the one connected to, whatever host name the server advertises.

### The client certificate

For a secured connection (`useSecurity: true` and a mode other than `None`) the driver needs an
application instance certificate with its private key:

- **`localCertFilePath` set** — that file is used. `.pfx` and `.p12` are read as PKCS#12; anything
  else as PEM, where the key may be in the same file or in a sibling file named `*_key.pem` for a
  certificate named `*_cert.pem`. Encrypted keys are decrypted with `passphrase`. Only RSA keys are
  supported. If the file cannot be read the process logs `FATAL: error in local certificate file!`
  and exits.
- **`localCertFilePath` empty** — a self-signed certificate is generated once in `conf/opcua/` as
  `js_opcua_client_go_cert.pem` and `js_opcua_client_go_key.pem` (valid for 10 years) and reused
  on every start.

`platform-windows/create_client_cert.ps1` creates a suitable pair if you prefer your own.

The certificate's URI becomes the session's `ApplicationUri`. A certificate without a URI in its
Subject Alternative Name makes the driver fall back to the `ApplicationUri` from
`configFileName`, and servers then reject the session with `BadCertificateUriInvalid`.

### Making the server trust the client

Servers refuse clients whose certificate they do not trust. This is configured **on the server**:
copy the client certificate (`js_opcua_client_go_cert.pem`, or your own) into its trust list or
approve it in its administration interface. Until you do, the connection fails with a
certificate-related status such as `BadSecurityChecksFailed` or `BadCertificateUntrusted`.

### Trusting the server

| `autoAcceptUntrustedCertificates` | Behaviour |
|---|---|
| `true` | The server's certificate is **not checked**. Convenient on a closed network; it means anyone who can impersonate the server's address can talk to the driver. |
| `false` | The server's certificate must chain to a system root or be one of the certificates you placed in `conf/opcua/trusted/` (`.pem`, `.crt` or `.der`). The chain and validity dates are checked; the host name is not. Applies to secured endpoints only. |

### User credentials

A user name and password are sent as the server's user-token policy requires. On an unsecured
(`None`) endpoint that may mean the password is sent unprotected, so use a secured connection for
real credentials.

## Tags

A *tag* is a document in `realtimeData`. The driver updates a tag when its
`protocolSourceConnectionNumber` is the connection's number, its `origin` is `supervised`, and its
`protocolSourceObjectAddress` is exactly the OPC UA node id the value came from.

### Automatic tag creation

With `autoCreateTags: true` the driver browses the server's `Objects` folder when it connects,
reads the attributes and current value of every node it finds, and creates a tag for each variable
that does not already have one.

**What is discovered.** Everything reachable from `Objects` through hierarchical references:
objects (folders), variables and methods. Properties of a variable are discovered as tags of their
own, but their children are not expanded. Types, views and anything outside `Objects` are not
browsed.

**What is skipped.** The server's own bookkeeping about its clients: `SessionsDiagnosticsSummary`
(with everything below it), `SessionDiagnosticsArray`, `SessionSecurityDiagnosticsArray` and
`SubscriptionDiagnosticsArray`. They describe sessions that come and go, so tags made from them are
dead by the next run, and on some servers they change on every request and would flood the driver.
Tags of these nodes left by an earlier run (or by the .NET driver) stay in the database but are not
monitored. `ServerDiagnosticsSummary` and the other `Server` nodes are discovered as usual.

**Names.** For a variable with browse path `/Objects/Boiler/Drum/Level` and display name `Level`
on connection `PLC1`:

| Tag field | Value |
|---|---|
| `tag` | `PLC1;ns=2;s=Boiler.Drum.Level` — the connection name and the node id |
| `ungroupedDescription` | `Level` — the node's display name |
| `group1` | `PLC1` |
| `group2` | `Boiler/Drum` — the path without `/Objects/` and without the node's own name |
| `description` | `PLC1~Boiler/Drum~Level` |
| `protocolSourceBrowsePath` | `Boiler/Drum` |
| `protocolSourceObjectAddress` | `ns=2;s=Boiler.Drum.Level` |
| `protocolSourceAccessLevel` | the server's user access level, as a number (`3` is read and write) |

A node directly under `Objects` has no folder of its own, so its `group2` and
`protocolSourceBrowsePath` are `/Objects`; this keeps such tags together.

**Tag type** follows the OPC UA type of the value:

| OPC UA type | Tag `type` | `valueAtSource` | `valueStringAtSource` |
|---|---|---|---|
| Boolean | `digital` | 0 or 1 | `True` / `False` |
| SByte, Byte, Int16, UInt16, Int32, UInt32, Int64, UInt64, Float, Double | `analog` | the number | the number as text |
| StatusCode | `analog` | the numeric code | the code name, e.g. `Good` |
| DateTime | `analog` | Unix time in milliseconds | ISO-8601, UTC |
| String, XmlElement, Guid | `string` | 0 | the text |
| ByteString | `string` | 0 | Base64 |
| LocalizedText | `string` | 0 | the text |
| QualifiedName | `string` | 0 | `name`, or `ns:name` when the namespace is not 0 |
| NodeId, ExpandedNodeId | `json` | 0 | the node id as text |
| ExtensionObject (a structure) | `json` | 0 | the structure as JSON |
| **Any array** | `json` | 0 | the array as JSON |

`protocolSourceASDU` holds the lower-case OPC UA type name: `double`, `boolean`, `datetime`,
`extensionobject`, and for arrays the name plus brackets, `double[]`. Every tag also receives the
raw value in `valueJsonAtSource` and `valueBsonAtSource`.

> 64-bit integers above 2^53 lose precision in `valueAtSource`, which is a floating-point number.
> `valueStringAtSource` keeps them exact.

**Subscription parameters** for the new tag are the connection's `autoCreateTag...` values.

**Keys.** Each connection owns the `_id` range from `protocolConnectionNumber × 1,000,000` up. A
new tag takes the next free key in its range. Keep a connection under 1,000,000 tags: the range is
not enforced, and going past it runs into the next connection's keys.

**Writable variables get a command twin.** When `commandsEnabled` is true and the variable's user
access level includes write, the driver also creates a *command tag* for it — see
[Commands](#commands). The two are linked through `commandOfSupervised` and `supervisedOfCommand`.

**Methods** that the server marks executable get a command tag too (and no data tag).

**When a node has no value yet** it is subscribed anyway, and its tag is created when the first
value arrives. A node whose value currently reads as an error still gets its tag, with bad quality.

**Re-discovery.** Discovery runs every time the connection is (re)established. Tags that already
exist are left exactly as they are, so it is safe to edit auto-created tags by hand (descriptions,
limits, alarm settings). Nodes the server adds later are tagged on the next reconnection or
restart. Tags are never deleted, even when the node disappears from the server.

#### Choosing what to discover

`topics` limits discovery. Each topic is matched against the **whole segments** of a node's browse
path, which you can read in the log at level 2 (`Path: /Objects/Boiler/Drum/Level`). A node is
kept when any topic matches.

| Topic | Keeps | Does not keep |
|---|---|---|
| `Boiler` | everything below `Boiler`, and a node named `Boiler` | |
| `Boiler/Drum` | everything below `Boiler/Drum` | `Boiler/Text` |
| `Objects/Boiler` | same as `Boiler` | |
| `Boil` | nothing — a partial name never matches | |
| `/Objects/Boiler` | nothing — **do not start a topic with a slash** | |

Folders are always walked; the topics decide which variables and methods become tags.

### Tags you configure yourself

Set `autoCreateTags: false` to update only your own tags, or leave it on to add your own tags to
the discovered ones. To make a tag read from a server, set these fields on an existing tag:

```javascript
db.realtimeData.updateOne({ tag: "Boiler.Temperature" }, { $set: {
  origin: "supervised",
  protocolSourceConnectionNumber: 81,
  protocolSourceCommonAddress: "",
  protocolSourceObjectAddress: "ns=2;s=Boiler.Temp",
  protocolSourcePublishingInterval: 5,
  protocolSourceSamplingInterval: 1,
  protocolSourceQueueSize: 10,
  kconv1: 1,
  kconv2: 0
}});
```

| Field | Meaning |
|---|---|
| `protocolSourceConnectionNumber` | The connection that updates the tag. Only one connection may. |
| `protocolSourceObjectAddress` | The OPC UA node id, such as `ns=2;s=Boiler.Temp`, `i=2258` or `ns=3;g=<guid>`. It is used exactly as written, so it must match what the server uses. Node ids contain namespace *indexes*, which can change if a server's namespace table changes. |
| `protocolSourcePublishingInterval` | Seconds between the server's reports. Tags that share a value share one subscription, so give the same value to every tag that should be reported together. |
| `protocolSourceSamplingInterval` | Seconds between the server's samples. `0` lets the server choose. |
| `protocolSourceQueueSize` | Changes the server buffers between reports. |
| `kconv1`, `kconv2` | Scaling (multiplier and offset) applied by JSON-SCADA's data processor, not by this driver. |
| `protocolSourceCommonAddress` | Not used; keep it empty. |
| `protocolSourceASDU` | Not used for reading — the type comes from the server. It matters for command tags. |

`protocolSourceDiscardOldest` is not read: when the server's queue fills, it always discards the
oldest change. Edits to tags are picked up when the driver restarts.

### What the driver writes

For every value received, the driver sets `sourceDataUpdate` on the tag:

| Field | Meaning |
|---|---|
| `valueAtSource` | the value as a number (see the table above) |
| `valueStringAtSource` | the value as text |
| `valueJsonAtSource`, `valueBsonAtSource` | the raw value as JSON text and as a BSON value |
| `asduAtSource` | the OPC UA type name, e.g. `double`, `string[]` |
| `invalidAtSource` | `true` unless the server's status for the value is *Good* (an *Uncertain* status counts as invalid) |
| `timeTagAtSource`, `timeTagAtSourceOk` | the server's source timestamp, shifted by `hoursShift`; `timeTagAtSourceOk` is `false` when the server gave none |
| `timeTag` | when the driver received the value |
| `causeOfTransmissionAtSource` | `"20"` for the value read when the connection starts, `"3"` for a value reported by the subscription |
| `notTopicalAtSource`, `overflowAtSource`, `blockedAtSource`, `substitutedAtSource` | always `false` |

The subscription reports a point when its value, its status **or its timestamp** changes. A server
that stamps every sample therefore produces an update for every sample even if the value is
constant.

A value whose update would take more than 3 MB once stored (large structures and arrays) is
discarded, with `NAME - Update too large for ADDRESS - N bytes (limit 3145728), discarded` in the
log at every level. The limit is well below MongoDB's 16 MB per document because the tag document
holds the value several times over (`sourceDataUpdate`, then `value`, `valueString` and `valueJson`
once processed); above it the write would fail with `BSONObjectTooLarge`. If the point has no tag
yet, the tag is still created, without the value.

## Commands

Commands travel the other way: an operator commands a tag in JSON-SCADA, which queues a document
in `commandsQueue`; the driver writes the value to the server and records the outcome in that same
document.

### Command tags

A command tag is a tag with `origin: "command"` that carries the connection number, the node id and
the OPC UA type to write. Auto-created command twins have this already; to make one by hand:

```javascript
db.realtimeData.updateOne({ tag: "Boiler.SetPoint.cmd" }, { $set: {
  origin: "command",
  protocolSourceConnectionNumber: 81,
  protocolSourceCommonAddress: "",
  protocolSourceObjectAddress: "ns=2;s=Boiler.SetPoint",
  protocolSourceASDU: "Double",
  kconv1: 1,
  kconv2: 0
}});
```

### Which types can be written

`protocolSourceASDU` selects the OPC UA type, matched without regard to case.

| `protocolSourceASDU` | Written as | Value taken from |
|---|---|---|
| `Boolean` | Boolean (true when `value` is not 0) | `value` |
| `SByte`, `Byte`, `Int16`, `UInt16`, `Int32` (or `Integer`), `UInt32`, `Int64`, `UInt64` | that integer type | `value` |
| `Float`, `Double` | that floating-point type | `value` |
| `DateTime` | DateTime | `value` as Unix milliseconds |
| `String`, `ByteString`, `LocalizedText`, `QualifiedName`, `NodeId`, `Guid`, `ExpandedNodeId`, `XmlElement` | a String | `valueString` |
| any of the above followed by `[]`, such as `Double[]` | an array | `valueString`, a JSON array such as `[1, 2.5, 3]` |
| `method` | a method call | `valueString`, optional JSON array of arguments |

Structures (`ExtensionObject`), `Variant`, `DataValue`, `NumericRange` and `DiagnosticInfo` cannot
be written. Array elements of type DateTime are ISO-8601 strings, unlike a single DateTime.

#### Numeric values are checked, never wrapped

A command whose value does not fit the target type is **refused** with `type conversion error`;
nothing is written to the server.

| Target | Rule |
|---|---|
| `SByte`, `Byte`, `Int16`, `UInt16`, `Int32`, `UInt32`, `Int64`, `UInt64` | `value` is rounded to the nearest whole number (a half goes to the even neighbour: 2.5 becomes 2, 3.5 becomes 4), and the result must be inside the type's range. `300` for a `Byte`, `-1` for a `UInt32` and `3e9` for an `Int32` are refused. `NaN` and the infinities are refused. |
| `DateTime` | The same rounding, and the result must be a date a .NET `DateTime` can hold (years 0001 to 9999). |
| `Float` | A finite value too large for a `Float` is refused. `NaN` and the infinities are passed on. |
| `Double` | Any `value`, `NaN` and the infinities included, is passed on. |
| Integer array elements | Each element must be written as a plain integer inside the type's range. `2.0`, `1e2` and `40000` for an `Int16` are refused, and nothing is rounded. 64-bit values are kept exact. |
| `Float` or `Double` array elements | Any number; one too large for the type is refused. |

### What happens to a command

1. The driver sees the new document (it only reacts on the active node).
2. It refuses the command, writing a `cancelReason` into the document, if:

   | `cancelReason` | Because |
   |---|---|
   | `expired` | the command is more than 10 seconds old |
   | `not connected` | the connection has no live session |
   | `commands disabled` | `commandsEnabled` is `false` for the connection |
   | `type conversion error` | the value does not fit the type (out of range, `NaN`, ...), a node id or array element cannot be converted, or the type cannot be written |
   | `unsupported command type` | `protocolSourceASDU` is not one of those above |
   | `empty array json error` | an array command with an empty `valueString` |
   | `array invalid json format error` | an array command whose `valueString` is not a JSON array (`null` and an object included) |

3. Otherwise it writes the value (only the value — no timestamps or status), or calls the method,
   and sets `delivered: true`, `ack`, `ackTimeTag` and `resultDescription` on the document. `ack`
   is `true` when the server accepted the command; `resultDescription` is the server's status
   (`Good`, or a name such as `BadTypeMismatch` or `BadUserAccessDenied`) or the error text.

Commands for a connection that belongs to another instance are ignored.

### Method calls

A command with `protocolSourceASDU: "method"` calls the OPC UA method at the node id. The driver
finds the object that owns the method itself. Input arguments come from `valueString` as a JSON
array — `[]` or empty for none. Elements are sent as Boolean, Int64 (whole numbers), Double or
String, so a method that expects another numeric type (Int32, Float, ...) may reject the call.
`resultDescription` is `OK`, or `OK:` followed by the method's output arguments.

## Redundancy

Run the same instance on two nodes by listing both in `nodeNames` and starting the driver on each.
The nodes arbitrate through the instance document:

- The node named in `activeNodeName` is **active**. It refreshes `activeNodeKeepAliveTimeTag`
  every 5 s.
- A standby node watches that time tag. If it fails to change for more than four *consecutive*
  checks — about 25 to 30 s — the standby makes itself active. Clocks are never compared, so the
  two machines need not agree about the time.
- An active node that finds another node named active steps down after a random pause of 1 to 5 s.
- If the instance document disappears, the node becomes inactive.

**The active flag controls commands only.** Both nodes connect to the servers, subscribe and write
to MongoDB all the time; only the active node executes commands. Keep that in mind: two redundant
nodes write the same data.

The active node's log line shows the health of acquisition:

```
Redundancy - This node is active. - Notification events: 183250 - Lost updates: 0 - Flood-dropped: 0
```

`Lost updates` counts values dropped because the 50,000-entry queue was full. `Flood-dropped`
counts values dropped because a single point reported more than 200 values in one second: that
cap keeps one misbehaving node from filling the queue and starving every other point. The first
time a point trips it the log says which one (`NAME - ns=2;s=X Y reports more than 200 values per
second; the excess is dropped`). Fix the node or move it out of the subscription.
## Reconnection and recovery

| Situation | What the driver does |
|---|---|
| The server is unreachable or refuses the session | Logs `FATAL: error creating session!` and tries again every 5 s, moving to the next `endpointURLs` entry each time. The wording is alarming but the condition is routine; the driver keeps trying. |
| An established connection drops | The library retries every 10 s, restoring the session and its subscriptions. If it has not recovered after 60 s, the driver tears the connection down and starts over, on the next URL. |
| A subscription cannot be created | Retried 3 times at 0.5 s intervals, then always logged as `... WILL NOT UPDATE`, naming how many points are affected. That group stays without data until the connection is rebuilt. |
| The server closes the connection during discovery | The driver does **not** publish a partial address space. It rebuilds the connection and discovers again, waiting 5 s, then 10 s, doubling up to 5 minutes between attempts. |
| The server drops the connection when asked for many values at once | Each failed discovery halves the number of values requested per read, from 500 down to 10, and tries again (`Reading fewer values at a time from now on: 250`). It can take several attempts. |
| MongoDB is unreachable | Each worker (data writer, command listener, redundancy) retries by itself. Received values keep queuing, up to 50,000; beyond that they are dropped and counted in `Lost updates`. |

## Logging and troubleshooting

Log lines go to standard output as `[timestamp] message`, with the timestamp in ISO 8601 with a
UTC offset. A service definition sends them to a log file.

### Following a connection

| Log line | Meaning |
|---|---|
| `NAME - Selected endpoint uses: Basic256Sha256` | The security policy actually in use. |
| `NAME - Using anonymous authentication.` / `... username/password ...` | The identity in use. |
| `NAME - Session created successfully.` | Connected. |
| `NAME - BrowseFullAddressSpace found N references on server in Xms.` | Discovery has browsed the namespace. |
| `NAME -  Autotag - Read 500 nodes at offset 0 from a total of 4316` | Discovery progress. |
| `NAME - N variables added to monitoring.` / `N Monitored items` | What is subscribed. The two should agree; if the second is lower, some items were rejected. |
| `NAME - N monitored items rejected: BadNodeIDUnknown - e.g. ns=2;s=A, ns=2;s=B and 1195 more` | The server refused those items, one line per status code with a few example addresses. Every address is listed at level 2. Usually tags of nodes that no longer exist. |
| `NAME - Skipped N per-session/per-subscription server diagnostics nodes.` | Discovery left out the server's session and subscription diagnostics (see [Automatic tag creation](#automatic-tag-creation)). |
| `NAME - Running...` | Acquiring. |
| `MongoDB - Bulk written N documents in X ms, updates per second: N` | Data reaching the database. |
| `NAME - Connection lost (...), reconnecting...` | The session was torn down and is being rebuilt. |

### Common problems

| Symptom | Likely cause and remedy |
|---|---|
| `error creating session! ... connection refused` or `no such host` | Wrong URL or port, a firewall, or the server is down. The driver keeps retrying. |
| `BadServerTooBusy` | The server is out of resources, often a limit on concurrent sessions. Close other clients; the driver waits and retries by itself. |
| `BadSecurityChecksFailed`, `BadCertificateUntrusted` | The server does not trust the client certificate. Add it to the server's trust list. |
| `BadCertificateUriInvalid` | The certificate has no application URI. Use the generated certificate or add a URI to yours. |
| `BadIdentityTokenRejected`, `BadUserAccessDenied` | Wrong user name, password or user certificate, or the server does not allow that kind of login on this endpoint. |
| The wrong security policy is in use | `securityPolicy` and `securityMode` matched no endpoint, so another was chosen. Compare them with the server's endpoints. |
| `Tag discovery incomplete: ... unexpected EOF` | The server closes the connection on large reads. The driver adapts by itself; see [Reconnection and recovery](#reconnection-and-recovery). |
| No tags appear | `topics` is too narrow (see the table above), the tags already exist, or the discovery log shows `0 variables added`. Run at level 2 to see every node. |
| Tags exist but never update | The tag's `protocolSourceConnectionNumber` or `protocolSourceObjectAddress` does not match, `origin` is not `supervised`, or the connection is not `Running`. The address is compared as written. |
| A command has a `cancelReason` | See the table under [Commands](#commands). |
| Nothing happens to a command at all | This node is not the active one (check `activeNodeName`), or MongoDB is not a replica set so the command listener cannot start. |
| Values arrive but with `invalidAtSource: true` | The server reports the value as Bad or Uncertain; the quality comes from the server. |

Raise the log level to `2` for per-node detail during discovery and to `3` to see each value
written. Higher levels cost performance on large systems.

## Limits and sizing

| Item | Value |
|---|---|
| Tags per connection | 1,000,000 (the key range of the connection) |
| Nodes read per request during discovery | 500, falling to 10 if the server needs it |
| Monitored items per `CreateMonitoredItems` request | 1,000 |
| Received values waiting to be written | 50,000; more are dropped and counted |
| Size of one value update | 3 MB; larger updates are logged and discarded |
| Database operations per bulk write | up to 6,000, at least every 750 ms while data is pending |
| Command age limit | 10 s |
| Time to connect, discover and answer a request | `timeoutMs` |

Writes to `realtimeData` are unacknowledged: they favour throughput, and an individual failed
update is not reported. Discovering a server of about 5,000 nodes took around a minute on the
public servers this driver was tried against.

## Known limitations

- **Method calls have not been run against a real server yet.** Resolving the owning object and
  building the call are tested; the call itself needs equipment that implements the method service.
  Method arguments are limited to Boolean, Int64, Double and String.
- Only the **Value** attribute is read and written. History, events and alarms, and PubSub are not
  implemented.
- A multi-dimensional array is reported and written as a flat array.
- No deadband filter: every change of value, status or timestamp is reported.
- Tags are never deleted, and tags and connections are read at startup: restart the driver after
  changing them.
- `giInterval` (periodic reads of points that are not subscribed) is accepted but does nothing.
- The server certificate's host name is not checked, and only RSA certificates are supported.
- A point reporting more than 200 values per second has the excess dropped; the newest value of
  such a burst can be lost.
- Redundant nodes both acquire and write (see [Redundancy](#redundancy)).

## Moving from the .NET driver

The documents are the same, so a connection configured for the .NET driver (`OPC-UA-Client`) works
unchanged and existing tags keep updating. To switch:

1. Stop and disable the .NET instance (`opcua_client` / `JSON_SCADA_opcuaclient`) and enable this
   one (`opcua_goclient` / `JSON_SCADA_opcuagoclient`, or `processExecutableVariant: "opcuago"`).
2. If the connection uses security, supply the certificate as a file (see
   [Security and certificates](#security-and-certificates)): this driver does not use the OPC
   Foundation certificate stores. The server must trust it.
3. Start it and compare the log with what you expect.

The visible differences are: certificates are plain files; numbers in `valueStringAtSource` always
use a decimal point; commands write only the value; discovery is more complete on large servers
(it follows continuation points and survives servers that drop the connection on large reads);
the JSON of structured values has a different shape; and a standby node needs consecutive missed
keep-alives before taking over.

## Building and testing

```bash
go vet ./...
go test ./...
go build -ldflags="-s -w" -o ../../bin/opcua-client
```

The tests need neither MongoDB nor a device: they start an in-process OPC UA server and run
discovery, tag creation, subscriptions and the command conversions against it. Contributors should
read [AGENTS.md](AGENTS.md) first.

## License

GNU General Public License, version 3 — see the JSON-SCADA distribution.
