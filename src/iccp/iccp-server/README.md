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
    authenticationPassword: "",
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
- **ipAddresses** [Array of Strings] - Allowed client IP addresses. Empty = allow all. **Optional**.
- **topics** [Array of Strings] - `group1` filter for exposed tags. Empty = all tags. **Optional**.
- **timeoutMs** [Double] - Connection timeout in ms. **Optional**.
- **localApTitle** [String] - Local AP Title (e.g. "1.1.999.1"). Gets a default if empty. **Optional**.
- **localAeQualifier** [Integer] - Local AE Qualifier. Default: 12. **Optional**.
- **remoteApTitle** [String] - Authorized remote AP Title for bilateral table. Empty = open mode. **Optional**.
- **remoteAeQualifier** [Integer] - Remote AE Qualifier. **Optional**.
- **useSecurity** [Boolean] - Reserved for future TLS support. **Optional**.
- **authenticationPassword** [String] - ACSE authentication password. Empty = no auth. **Optional**.
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
