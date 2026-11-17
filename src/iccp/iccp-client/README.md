# ICCP/TASE.2 Client Driver for JSON-SCADA

This driver implements an ICCP/TASE.2 (IEC 60870-6-503) client for JSON-SCADA.

It connects to remote ICCP/TASE.2 servers to read real-time data and
write it to the MongoDB `realtimeData` collection using the standard
JSON-SCADA `sourceDataUpdate` mechanism. Commands from the `commandsQueue`
collection are forwarded to the remote ICCP server.

The driver is implemented in **Golang**.

The ICCP/TASE2 library is not publicly available, but the driver code is open source.

## Architecture

The ICCP client:

- **Connects** to one or more remote ICCP/TASE.2 servers (one per
  protocol connection), optionally over TLS (secure ICCP, IEC 62351-3).
- **Reads** tag configurations from the `realtimeData` MongoDB collection
  (tags assigned to this connection via `protocolSourceConnectionNumber`).
- **Subscribes** to live data updates via ICCP Data Set Transfer Sets
  (DSTS), one per domain with mapped tags: the driver obtains a transfer set
  through the domain's `Next_DSTransfer_Set` (or the first one listed, for
  servers without it), recreates a dataset (`JSSCADA_DS`) on the server
  containing exactly the mapped points of the domain and activates
  change-based (RBE) reporting plus periodic integrity snapshots. When the
  dataset cannot be created it falls back to the existing dataset holding
  the most mapped points.
- **Polls** every mapped point not reported by a DSTS subscription
  periodically via chunked ICCP `ReadMultiple` calls (integrity polling at
  configurable interval).
- **Decodes** ICCP typed points (RealQ, StateQTimeTag, ...Extended,
  ...QTimeTagExtended, etc.): value, quality (invalid / held-suspect /
  non-telemetered source flags) and the source timestamp (GMT seconds, with
  milliseconds for the 802 Ed.2 QTimeTagExtended types) are mapped into the
  `sourceDataUpdate` document. Points without a time stamp store a null
  `timeTagAtSource`.
- **Maps** ICCP states to digital values: On (closed) is 1, Off (open) 0;
  the in-between state is 0 flagged invalid and transient, the invalid
  state 0 flagged invalid.
- **Invalidates** the tags of a connection (keeping their last value) when
  the association fails or drops, and a single point when the server does
  not return it.
- **Writes** acquired values to MongoDB using bulk writes with the
  `sourceDataUpdate` sub-document pattern.
- **Watches** the `commandsQueue` MongoDB collection via change streams
  and forwards commands to the remote ICCP server as `WriteDataValue`
  calls (values are coerced to the type the server declares for the
  control point).
- **Supports** redundancy/high-availability via the standard JSON-SCADA
  protocol driver instance mechanism: only the active node keeps the ICCP
  associations up and writes data, unless `keepProtocolRunningWhileInactive`
  is `true` (associations stay up on the inactive node, which then neither
  writes data nor sends commands).

## Configuration

### Driver Instance

Create a document in the `protocolDriverInstances` collection:

```javascript
db.protocolDriverInstances.insert({
    protocolDriver: "ICCP",
    protocolDriverInstanceNumber: 1,
    enabled: true,
    logLevel: 1,
    nodeNames: [],
});
```

- **protocolDriver** [String] - Must be "ICCP". **Mandatory**.
- **protocolDriverInstanceNumber** [Double] - Instance number (1..N). **Mandatory**.
- **enabled** [Boolean] - Enable/disable the instance. **Mandatory**.
- **logLevel** [Double] - Log level (0=min, 1=basic, 2=detailed, 3=debug). **Mandatory**.
- **nodeNames** [Array of Strings] - Node names allowed to run this instance. **Mandatory**.
- **keepProtocolRunningWhileInactive** [Boolean] - Keep the ICCP associations up while this node is not the active one. Default: false. **Optional**.

### Protocol Connection

Create a document in the `protocolConnections` collection:

```javascript
db.protocolConnections.insert({
    protocolDriver: "ICCP",
    protocolDriverInstanceNumber: 1,
    protocolConnectionNumber: 201,
    name: "ICCP_Link_1",
    description: "ICCP link to control center",
    enabled: true,
    commandsEnabled: true,
    endpointURLs: ["192.168.1.60:102"],
    giInterval: 30.0,
    timeoutMs: 15000,
    localApTitle: "1.1.999.2",
    localAeQualifier: 12,
    remoteApTitle: "1.1.999.1",
    remoteAeQualifier: 12,
    password: "",
    hoursShift: 0.0,
    useSecurity: false,
    localCertFilePath: "",
    privateKeyFilePath: "",
    rootCertFilePath: "",
    chainValidation: false,
    autoAcceptUntrustedCertificates: true,
    stats: {}
});
```

- **protocolDriver** [String] - Must be "ICCP". **Mandatory**.
- **protocolDriverInstanceNumber** [Double] - Instance number. **Mandatory**.
- **protocolConnectionNumber** [Double] - Unique connection number. **Mandatory**.
- **name** [String] - Connection name for logging. **Mandatory**.
- **description** [String] - Description. **Optional**.
- **enabled** [Boolean] - Enable/disable connection. **Mandatory**.
- **commandsEnabled** [Boolean] - Enable command forwarding. **Mandatory**.
- **endpointURLs** [Array of Strings] - Server endpoints as `host`, `host:port` or `[ipv6]:port` (port default 102; a scheme prefix such as `iccp://` is ignored). After a failed connect the driver tries the next endpoint in the list. **Mandatory**.
- **giInterval** [Double] - Integrity poll interval in seconds. Default: 30. **Optional**.
- **timeoutMs** [Double] - Timeout in ms for establishing the association and for each request. Default: 30000. **Optional**.
- **localApTitle** [String] - Local AP Title (e.g. "1.1.999.2"). Gets a default if empty. **Optional**.
- **localAeQualifier** [Integer] - Local AE Qualifier. Default: 12. **Optional**.
- **remoteApTitle** [String] - Remote AP Title (server's identity). **Mandatory**.
- **remoteAeQualifier** [Integer] - Remote AE Qualifier. Default: 12. **Optional**.
- **password** [String] - ACSE authentication password. **Optional**.
- **hoursShift** [Double] - Hours added to source time tags (to correct a peer sending local time instead of UTC). Default: 0. **Optional**.
- **useSecurity** [Boolean] - Enable TLS (secure ICCP). Default: false. **Optional**.
- **localCertFilePath** [String] - Client certificate (PEM) for mutual TLS. **Optional**.
- **privateKeyFilePath** [String] - Client private key (PEM) for mutual TLS. **Optional**.
- **rootCertFilePath** [String] - CA certificate (PEM) to verify the server. **Optional**.
- **chainValidation** [Boolean] - Verify the server certificate chain. **Optional**.
- **autoAcceptUntrustedCertificates** [Boolean] - Accept untrusted server certificates (overrides chain validation). **Optional**.
- **stats** [Object] - Protocol statistics (updated by driver). **Mandatory**.

## Tag Configuration

Each tag to be updated by this connection must have its
`protocolSourceConnectionNumber` set to the connection number and
`protocolSourceObjectAddress` set to the ICCP `Domain/Item` reference.

### Supervised tags (data read from ICCP server)

```javascript
db.realtimeData.updateOne({"tag": "Transformer1.Voltage"}, {
    $set: {
        protocolSourceConnectionNumber: 201.0,
        protocolSourceObjectAddress: "SUB_A/Real_01",
        protocolSourceASDU: "float32",
        origin: "supervised"
    }
});
```

- **protocolSourceConnectionNumber** [Double] - Connection number. **Mandatory**.
- **protocolSourceObjectAddress** [String] - ICCP address as `Domain/Item`. **Mandatory**.
- **protocolSourceASDU** [String] - Data type hint (float32, int32, boolean, etc.). **Mandatory**.
- **origin** [String] - Must be "supervised". **Mandatory**.

### Command tags (data written to ICCP server)

```javascript
db.realtimeData.updateOne({"tag": "Breaker1.Command"}, {
    $set: {
        protocolSourceConnectionNumber: 201.0,
        protocolSourceObjectAddress: "SUB_A/Command_01",
        protocolSourceASDU: "boolean",
        origin: "command"
    }
});
```

- **protocolSourceObjectAddress** [String] - ICCP address as `Domain/Item`. **Mandatory**.
- **protocolSourceASDU** [String] - Data type (boolean, float, double, int32, string, etc.). **Mandatory**.
- **origin** [String] - Must be "command". **Mandatory**.

## Logging

Library messages (the `iso`, `mms` and `tase2` layers) go through the driver
log, filtered by the same `logLevel`: library errors at 0, informational
messages at 2 (detailed) and protocol debug at 3. Messages about one
association are prefixed with the connection name and the transport
context, e.g.

```
TASE2 ICCP_CLI1 [conn 3 10.0.0.9:40001 1.1.999.2/12] mms - ...
```

## Command Line Arguments

- **1st arg - Instance Number** [Integer] - Instance number. Default: 1.
- **2nd arg - Log Level** [Integer] - Log level (0-3). Default: 1.
- **3rd arg - Config File** [String] - Path to json-scada.json. Default: `../../conf/json-scada.json`.

## Building and Running

```bash
cd iccp-client
go build -o iccp-client .
./iccp-client [instance] [logLevel] [configFile]
```

## Data Flow

```
ICCP Server ──DSTS Report───> [ICCP Client Driver] ──sourceDataUpdate──> MongoDB realtimeData
ICCP Server ──ReadMultiple──> [ICCP Client Driver] ──sourceDataUpdate──> MongoDB realtimeData
MongoDB commandsQueue ──CS──> [ICCP Client Driver] ──WriteDataValue───> ICCP Server
```

DSTS reporting is preferred; `ReadMultiple` polling covers domains where no
transfer set could be activated. Select-before-operate (SBO) commands are not
supported by the underlying library; commands are sent as direct operates.

## License

GNU General Public License v3.0
