# ICCP/TASE.2 Server Driver for JSON-SCADA

This driver implements an ICCP/TASE.2 (IEC 60870-6-503) server for JSON-SCADA.

It exposes real-time SCADA data from MongoDB to ICCP/TASE.2 clients using the
`github.com/riclolsen/tase2` Go library.

The driver is implemented in **Golang**.

## Architecture

The ICCP server:

- **Reads** tag data from the `realtimeData` MongoDB collection and builds a
  TASE.2 data model with domains (mapped from `group1` property of tags),
  indication points (analog, digital, string tags), and control points (command
  tags).
- **Serves** multiple concurrent ICCP client connections, each via its own
  TASE.2 `Server` instance sharing the same `DataModel`.
- **Pushes** live updates to connected clients when MongoDB change stream events
  are detected for the watched tags.
- **Accepts** writes from ICCP clients and forwards them as commands to the
  `commandsQueue` MongoDB collection.
- **Supports** bilateral table access control and ACSE password authentication.
- **Supports** Data Set Transfer Set (DSTS) reporting for periodic, integrity,
  and change-based data updates.
- **Supports** redundancy/high-availability via the standard JSON-SCADA
  protocol driver instance mechanism.

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
- **ipAddressLocalBind** [String] - Listen address and port (e.g. "0.0.0.0:102"). Default port is 102. **Mandatory**.
- **ipAddresses** [Array of Strings] - Allowed client IP addresses (not yet enforced!). Empty = allow all. **Optional**.
- **topics** [Array of Strings] - `group1` filter for exposed tags. Empty = all tags. **Optional**.
- **timeoutMs** [Double] - Connection timeout in ms. **Optional**.
- **localApTitle** [String] - Local AP Title (e.g. "1.1.999.1"). Gets a default if empty. **Optional**.
- **localAeQualifier** [Integer] - Local AE Qualifier. Default: 12. **Optional**.
- **remoteApTitle** [String] - Authorized remote AP Title for bilateral table. Empty = open mode. **Optional**.
- **remoteAeQualifier** [Integer] - Remote AE Qualifier. **Optional**.
- **useSecurity** [Boolean] - Reserved for future TLS support. **Optional**.
- **password** [String] - ACSE authentication password. Empty = no auth. **Optional**.
- **stats** [Object] - Protocol statistics (updated by driver). **Mandatory**.

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
dataset member. The server logs every report at `logLevel >= 1`:

```
ICCP Server: DSTS report '<dataset>' kind=<kind> values=<n> (first=<domain>/<item>)
```

On transport failure (e.g. client disconnect), the server stops the affected DSTS
goroutines automatically.

## Commands Routing

When a TASE.2 client writes to a control point, the driver:
1. Looks up the corresponding realtimeData tag (`origin: "command"`).
2. Creates a command document in the `commandsQueue` collection.
3. The command is then picked up by the appropriate protocol client driver
   (the one matching `protocolSourceConnectionNumber`).

## Data Model Mapping

| realtimeData | TASE.2 |
|---|---|
| `group1` | Domain name |
| `ungroupedDescription` or `tag` | Point name (sanitized) |
| `type: "digital"` | State Indication Point (INTEGER32) |
| `type: "analog"` | Real Indication Point (FLOAT32) |
| `type: "string"` | Complex Indication Point |
| `origin: "command"` | Command Control Point (INTEGER32, SBO) |
| `invalid` | Quality (good/invalid) |

## License

GNU General Public License v3.0
