# ICCP/TASE.2 Server Driver for JSON-SCADA

This driver implements an ICCP/TASE.2 (IEC 60870-6-503) server for JSON-SCADA.

It exposes real-time SCADA data from MongoDB to ICCP/TASE.2 clients.

The driver is implemented in **Golang**.

The ICCP/TASE2 library is not publicly available, but the driver is open source.

## Architecture

The ICCP server:

- **Reads** tag data from the `realtimeData` MongoDB collection and builds a
  TASE.2 data model with domains (mapped from `group1` property of tags),
  indication points (analog and digital tags, with quality and timestamp),
  and control points (command tags).
- **Serves** multiple concurrent ICCP client connections, each via its own
  TASE.2 `Server` instance sharing the same `DataModel`.
- **Pushes** live updates to connected clients when MongoDB change stream events
  are detected for the watched tags.
- **Accepts** writes and device-control operates from ICCP clients and forwards
  them as commands to the `commandsQueue` MongoDB collection.
- **Supports** bilateral table access control (with per-connection topic
  isolation), ACSE password authentication, and TLS (secure ICCP,
  IEC 62351-3) with optional mutual certificate authentication.
- **Supports** Data Set Transfer Set (DSTS) reporting for periodic, integrity,
  and change-based data updates.
- **Supports** redundancy/high-availability via the standard JSON-SCADA
  protocol driver instance mechanism: only the active node serves
  associations (others are closed as soon as accepted, and live ones are
  aborted when the node becomes inactive), unless
  `keepProtocolRunningWhileInactive` is `true`.
- **Keeps** the data model current from the `realtimeData` change stream
  whether or not a client is connected.

## Configuration

### Driver Instance

Create a document in the `protocolDriverInstances` collection:

```javascript
db.protocolDriverInstances.insert({
    protocolDriver: "ICCP_SERVER",
    protocolDriverInstanceNumber: 1,
    enabled: true,
    logLevel: 1,
    nodeNames: [],
});
```

- **protocolDriver** [String] - Must be "ICCP_SERVER". **Mandatory**.
- **protocolDriverInstanceNumber** [Double] - Instance number (1..N). **Mandatory**.
- **enabled** [Boolean] - Enable/disable the instance. **Mandatory**.
- **logLevel** [Double] - Log level (0=min, 1=basic, 2=detailed, 3=debug). **Mandatory**.
- **nodeNames** [Array of Strings] - Node names allowed to run this instance. **Mandatory**.
- **keepProtocolRunningWhileInactive** [Boolean] - Serve associations while this node is not the active one. Default: false. **Optional**.

### Protocol Connection

Create a document in the `protocolConnections` collection:

```javascript
db.protocolConnections.insert({
    protocolDriver: "ICCP_SERVER",
    protocolDriverInstanceNumber: 1,
    protocolConnectionNumber: 32,
    name: "ICCP_Server_1",
    description: "ICCP/TASE.2 Server",
    enabled: true,
    commandsEnabled: true,
    ipAddressLocalBind: "0.0.0.0:102",
    ipAddresses: ["192.168.1.10"],
    topics: ["KAW2", "KOR1"],
    timeoutMs: 15000,
    localApTitle: "1.1.999.1",
    localAeQualifier: 12,
    remoteApTitle: "1.1.999.2",
    remoteAeQualifier: 12,
    useSecurity: false,
    password: "",
    localCertFilePath: "",
    privateKeyFilePath: "",
    rootCertFilePath: "",
    chainValidation: false,
    peerIdentification: true,
    stats: {}
});
```

- **protocolDriver** [String] - Must be "ICCP_SERVER". **Mandatory**.
- **protocolDriverInstanceNumber** [Double] - Instance number. **Mandatory**.
- **protocolConnectionNumber** [Double] - Unique connection number. **Mandatory**.
- **name** [String] - Connection name for logging. **Mandatory**.
- **description** [String] - Description. **Optional**.
- **enabled** [Boolean] - Enable/disable connection. **Mandatory**.
- **commandsEnabled** [Boolean] - Enable command forwarding. **Mandatory**.
- **ipAddressLocalBind** [String] - Listen address and port: `"host:port"`, `":port"`, `"host"` or `"[ipv6]:port"` (e.g. "0.0.0.0:102"). The host must be an IP address or `localhost`; an empty host listens on all interfaces (0.0.0.0), and the port defaults to 102. An invalid value is logged and that connection is not started. **Mandatory**.
- **ipAddresses** [Array of Strings] - Expected client IP addresses or CIDR prefixes. A connection from any other address is reported as a `conn.unexpected` security event; it is not refused. Empty = any address is expected. **Optional**.
- **topics** [Array of Strings] - `group1` filter for exposed tags. Empty = all tags. Topics always limit the connection's predefined datasets and the control points it may operate. When `remoteApTitle` is set, they are also enforced on discovery/reads/writes via the bilateral table (only the listed domains are granted); without a `remoteApTitle` (open mode) discovery and reads are not restricted. **Optional**.
- **timeoutMs** [Double] - Not used by the server. **Optional**.
- **localApTitle** [String] - Local AP Title (e.g. "1.1.999.1"). Gets a default if empty. **Optional**.
- **localAeQualifier** [Integer] - Local AE Qualifier. Default: 12. **Optional**.
- **remoteApTitle** [String] - Authorized remote AP Title for bilateral table. Empty = open mode. **Optional**.
- **remoteAeQualifier** [Integer] - Remote AE Qualifier. **Optional**.
- **useSecurity** [Boolean] - Enable TLS (secure ICCP, IEC 62351-3). Default: false. **Optional**.
- **localCertFilePath** [String] - Server certificate (PEM), required for TLS. **Optional**.
- **privateKeyFilePath** [String] - Server private key (PEM), required for TLS. **Optional**.
- **rootCertFilePath** [String] - CA certificate (PEM) to verify client certificates. **Optional**.
- **chainValidation** [Boolean] - Require and verify client certificates (mutual TLS) against the CA. Default: false. **Optional**.
- **password** [String] - ACSE authentication password. Empty = no auth. **Optional**.
- **peerIdentification** [Boolean] - Keep the identity of the connected clients in `stats.peerIdentity` and report a change of it for the same address (see [Security Events](#security-events)). Default: true. **Optional**.
- **stats** [Object] - Protocol statistics, written by the driver every 10 s (see [Statistics](#statistics)). **Mandatory**.

## Logging

Library messages (the `iso`, `mms` and `tase2` layers) go through the driver
log, filtered by the same `logLevel`: library errors at 0, informational
messages at 2 (detailed) and protocol debug at 3. Messages about one
association are prefixed with the connection name and the transport
context, e.g.

```
TASE2 ICCP_SRV1 [conn 3 10.0.0.9:40001 1.1.999.2/12] mms - ...
```

## Statistics

Every 10 seconds the driver writes each listening connection's `stats`.
Counters are cumulative since the driver started and include associations
that have already ended:

| Field | Meaning |
|---|---|
| `nodeName`, `timeTag` | writing node and time of the update |
| `clientConnections` | live associations |
| `clients` | one entry per live association: `remoteAddress`, `apTitle`, `aeQualifier`, `connectedAt` |
| `associations` | associations accepted |
| `requests` | confirmed MMS requests answered, per service (`read`, `write`, `getNameList`, ...) |
| `errors`, `rejects` | requests answered with an error PDU or a reject |
| `reportsSent`, `reportsDropped` | DSTS reports |
| `informationMessagesSent` | Information Messages |
| `bytesRx`, `bytesTx` | MMS PDU octets |
| `eventsDropped` | server events lost because the observer fell behind |
| `peerIdentity` | with `peerIdentification`: `clients` (open associations: `address`, `ip`, `mac`, `macSource`, `acseIdentity`, `authenticated`, `blt`, `tls`, `connectedAt`) and `known` (the last identity of up to 64 client IPs: `ip`, `mac`, `macSource`, `acseIdentity`, `tlsSha256`, `lastSeen`) |

Server events are logged too: association accepted, released, aborted or
lost, and dropped reports at `logLevel >= 1`; rejected requests and failed
controls at 2; everything else at 3.

## Security Events

The driver writes cyber-security events to the `secEvents` collection (see
[docs/schema.md](../../../docs/schema.md#secevents-collection); the Admin UI
Security Events viewer lists them). Each listening connection has a
report-only tase2 security policy: nothing is refused because of it.

Detected by the tase2 library (`source: "library"`), among others:

- connections from addresses outside `ipAddresses` (`conn.unexpected`),
  connection rates, handshake timeouts;
- TLS: rejected or missing client certificates, plaintext on the TLS port,
  weak parameters, a listener without TLS or without authentication;
- ACSE: missing, unknown or duplicated AP titles (`identity.*`), failed
  authentication and brute force (`auth.*`);
- malformed TPKT, COTP, session, presentation and MMS PDUs (`proto.*`);
- reads and writes of unknown domains or objects, writes to read-only
  objects, scans and enumeration (`access.*`);
- every select and operate (`cmd.select`, `cmd.operate`), operate without
  select, duplicate (replayed) operates, operates on tagged devices;
- transfer sets disabled or changed, datasets deleted, indication writes,
  conclude and abort (`ctrl.*`);
- duplicate invoke IDs and stale acknowledgements (`seq.*`), floods of
  requests, reads, controls or transfer set writes (`flood.*`, `dos.*`);
- MAC address changes for the same IP and certificate changes for the same
  AP title (`peer.mac_changed`, `tls.cert_changed`).

Routine informational events (connection accepted or closed, association
accepted) are not stored.

Detected by the driver (`source: "driver"`):

- `peer.identified`: the first association from an address;
  `peer.identity_changed`: an address states another AP title/AE qualifier
  than the last time (compared with `stats.peerIdentity.known`, so across
  restarts too; the MAC and certificate history of the library is seeded
  from it);
- `cmd.rejected`, `cmd.object_unknown`: a control refused by the driver,
  with the reason (commands disabled, node inactive, domain outside the
  `topics`, no or not a command tag, `commandBlocked`);
- `cmd.wrong_type`: a value that is not a number for an analog or digital
  command tag (refused), or a digital command other than 0/1 (forwarded as
  1, reported);
- `flood.controls`: more than 10 controls from one client IP within 10 s;
- `cmd.unnecessary`: a digital command that would not change the state of
  the tag's `supervisedOfCommand` point (opening an open breaker).

Forwarded commands carry the client IP in `commandsQueue.originatorIpAddress`.

## Command Line Arguments

- **1st arg - Instance Number** [Integer] - Instance number. Default: 1.
- **2nd arg - Log Level** [Integer] - Log level (0-3). Default: 1.
- **3rd arg - Config File** [String] - Path to json-scada.json. Default: `../../conf/json-scada.json`.

## Building and Running

```bash
cd iccp-server
go build -o iccp-server .
./iccp-server [instance] [logLevel] [configFile]
```

## Data Set Transfer Set (DSTS) Reporting

The ICCP server supports Data Set Transfer Set (DSTS) reporting per IEC 60870‑6‑503
§C.6.6. Each domain-scope DSTransferSet object can be configured by a client with
the following parameters:

| Field | Type | Description |
|---|---|---|
| `DataSetName` | Structure | Dataset reference: `{Scope, DomainName, Name}` |
| `StartTime` | Integer32 | Time to begin condition monitoring (0 = immediately) |
| `Interval` | Integer16 | Periodic report interval in seconds |
| `TLE` | Integer16 | Transfer set life expectancy (logical only) |
| `BufferTime` | Integer16 | Buffering window for ObjectChange in seconds |
| `IntegrityCheck` | Integer16 | Integrity (full-snapshot) interval in seconds |
| `DSConditionsRequested` | BitString(5) | Bitfield of trigger conditions: Interval (bit 0), Integrity (bit 1), Change (bit 2), Operator (bit 3), External (bit 4) |
| `BlockData` | Boolean | Block transfer encoding (not supported) |
| `Critical` | Boolean | Report requires acknowledgement (logical only) |
| `RBE` | Boolean | Report by Exception: only emit changed objects |
| `AllChangesReported` | Boolean | Emit every intermediate change vs. latest state only |
| `Status` | Boolean | Set to `true` to activate the transfer set |
| `EventCodeRequested` | Integer16 | Event code filter |

### Reporting Triggers

Three basic trigger mechanisms can be combined:

| Condition | Parameter | Behaviour |
|---|---|---|
| **IntervalTimeout** | `Interval` + `DSConditionsRequested.Interval` bit | Periodic reports at the configured `Interval` |
| **IntegrityTimeout** | `IntegrityCheck` + `DSConditionsRequested.Integrity` bit | Periodic full-snapshot reports at the configured `IntegrityCheck` interval. 0 = same as `Interval` |
| **ObjectChange** | `BufferTime` + `DSConditionsRequested.Change` bit | Reports triggered by value changes, optionally buffered by `BufferTime` |

When the Integrity bit is set in `DSConditionsRequested` (`IntegrityCheck > 0`), the Integrity
ticker sends the **entire dataset snapshot** on every tick regardless of RBE — this
gives the client a periodic full picture even when RBE suppresses unchanged values.

### IntervalTimeout Behaviour

Controls how reports are generated when the Interval timer fires:

| RBE | AllChangesReported | Behaviour |
|---|---|---|
| `false` | *ignored* | Full dataset snapshot — current state of **all** objects is sent at each interval tick |
| `true` | `false` | Only the **latest state** of objects that changed during the interval window is sent |
| `true` | `true` | **All intermediate state changes** that occurred during the interval window are sent (as individual reports), **plus** the latest state of changed objects |

### ObjectChange (BufferTime) Behaviour

Controls how reports are generated when `BufferTime > 0` and a value changes:

| BufferTime | RBE | AllChangesReported | Behaviour |
|---|---|---|---|
| `0` | *any* | *any* | Immediate report for each change — no buffering |
| `> 0` | `false` | *ignored* | Full dataset snapshot when the BufferTime window expires (currently sends only changed items — full-snapshot implementation pending) |
| `> 0` | `true` | `false` | Only the **latest state** of changed objects is sent when BufferTime expires |
| `> 0` | `true` | `true` | **All intermediate changes** during the BufferTime window are sent as individual reports |

### IntegrityTimeout Behaviour

When `IntegrityCheck > 0` and `DSConditionRequested.Integrity` is set, the server
sends a **full dataset snapshot** at the Integrity interval, bypassing RBE entirely.
This is typically combined with RBE-based change reporting so the client receives
both immediate change notifications and periodic full refreshes.

### Activation Flow

A client activates reporting on a DSTransferSet by:

1. Locating the DSTransferSet object via `Next_DSTransfer_Set` in the domain.
2. Writing configuration fields to the DSTransferSet object (e.g.
   `<domain>/DSTrans.Interval`, `<domain>/DSTrans.RBE`, etc.) or writing
the entire structure.
3. Writing `Status = true` on the DSTransferSet object.

Upon activation (`Status` transitions to `true`), the server:

1. Resolves the dataset referenced by `DataSetName`.
2. Snapshots the dataset member list (so changes to the dataset after activation
   do not affect active reporting).
3. Pre-resolves every member to an `IndicationPoint` pointer for zero-allocation
   reads at report time.
4. Sends an initial full-snapshot report immediately.
5. Starts the configured tickers (Interval, Integrity, BufferTime).

### Report Delivery

Reports are delivered as MMS **InformationReport** messages. Each report contains
the ObjectRef (domain + item) and the current `DataValue` for each included
dataset member. Reporting start and stop per transfer set are logged at
`logLevel >= 2`; each sent report is logged at `logLevel 3` as a
`ReportSent` server event, and counted in `stats.reportsSent` (see
[Statistics](#statistics)). Dropped reports are logged at `logLevel >= 1`.

On transport failure (e.g. client disconnect), the server stops the affected DSTS
goroutines automatically.

## Commands Routing

When a TASE.2 client writes to a control point, the driver:
1. Refuses it unless the connection has `commandsEnabled`, the node runs the
   protocol, the domain is in the connection's `topics` and the tag's
   `commandBlocked` (followed live from the change stream) is not set.
2. Looks up the corresponding realtimeData tag (`origin: "command"`).
3. Refuses a value that is not a finite number for an analog or digital tag
   (a numeric string may have surrounding spaces; NaN and infinities are
   refused), reporting `cmd.wrong_type`.
4. Creates a command document in the `commandsQueue` collection; when the
   insert fails the command is refused and `cmd.rejected` is reported.
5. The command is then picked up by the appropriate protocol client driver
   (the one matching `protocolSourceConnectionNumber`).

## Data Model Mapping

| realtimeData | TASE.2 |
|---|---|
| `group1` | Domain name (sanitized like point names: non-alphanumerics become `_`, a leading non-letter gets an `X` prefix, at most 32 characters) |
| `tag` | Point name (sanitized). Tags are processed in name order; a tag whose sanitized name collides with an earlier one in the same domain is logged and not exposed |
| `type: "digital"` | StateQTimeTagExtended Indication Point (state + quality + ms time stamp) |
| `type: "analog"` | RealQTimeTagExtended Indication Point (float32 + quality + ms time stamp) |
| `type: "analog"` with integer ASDU (int16/32/64, uint16/32/64) | DiscreteQTimeTagExtended Indication Point (int + quality + ms time stamp) |
| `type: "string"`, `"json"`, others | Not exposed via ICCP |
| `origin: "command"` | Control Point (SBO when `protocolSourceCommandUseSBO`, else direct operate), when some connection with `commandsEnabled` covers its domain; otherwise not exposed. A digital tag is a Control_Command (Integer16 0/1); an analog tag is a setpoint typed by its ASDU: Float32 (real setpoint, the default) or an integer (discrete setpoint, integer ASDUs) |
| `invalid`, `transient`, `frozen` | Quality validity: invalid, else suspect (transient), else held (frozen), else valid |
| `substituted`, `origin` | Quality source: entered (substituted or `origin: "manual"`), calculated (`origin: "calculated"`), else telemetered |
| `timeTagAtSource` (when `timeTagAtSourceOk`, else current time) | Point TimeStamp: IEC 60870-6-802 Ed.2 TimeStampExtended {GMTBasedS, Milliseconds}, UTC, millisecond resolution |

## License

GNU General Public License v3.0
