# OPC-UA Server

This driver implements a server for the OPC-UA protocol (binary transport only, opc.tcp://hostname:port/resourcePath).

Implemented using the Node OPC-UA library.

    https://github.com/node-opcua/node-opcua

The driver can serve multiple connections to OPC-UA clients on multiple computers, if needed.

To configure the driver it is necessary to create one or more driver instances and one protocol connection per instance.

## Run

    cd src/OPC-UA-Server
    npm install
    node index.js [instance number] [log level] [config file]

The optional arguments default to instance `1`, log level `1` and
`../../conf/json-scada.json`; the equivalent environment variables are
`JS_OPCUASERVER_INSTANCE`, `JS_OPCUASERVER_LOGLEVEL` and `JS_CONFIG_FILE`.
Command-line arguments take precedence.

## Configure a driver instance

To create a new OPC-UA server instance, insert a new document in the _protocolDriverInstances_ collection using a command like this:

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "OPC-UA_SERVER",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: [],
        });

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "OPC-UA_SERVER". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
- _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
- _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Leave empty to allow any node to run this driver. **Mandatory parameter**.

Changes in the _protocolDriverInstances_ config requires that the driver instances processes be restarted to be effective.

## Configure client connections to OPC-UA servers

Each instance for this driver can have many client connection defined that must be described in the _protocolConnections_ collection.

This driver will make all points available to the clients, unless filtered. There is no need to configure tags for protocol destinations (_protocolDestinations_ property).

    use json_scada_db_name
    db.protocolConnections.insert({
        protocolDriver: "OPC-UA_SERVER",
        protocolDriverInstanceNumber: 1.0,
        protocolConnectionNumber: 81.0,
        name: "OPCUAServer",
        description: "OPC-UA Server",
        enabled: true,
        commandsEnabled: true,
        groupId: "UA/JsonScada",
        ipAddressLocalBind: "0.0.0.0:4840",
        ipAddresses: ["192.168.1.1"],
        topics: ["KAW2", "KOR1"],
        timeoutMs: 15000,
        useSecurity: false,
        localCertFilePath: "",
        privateKeyFilePath: "",
        historyEnabled: false,
        historian: "mongodb",
        stats: {}
    });

Parameters for communication with OPC-UA servers.

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "OPC-UA_SERVER". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
- _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
- _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
- _**commandsEnabled**_ [Boolean] - Allows to disable/enable commands (messages in control direction) for a connection. Use false to disable all commands. If true the driver will create writable command tags for the enabled topics (_group1_ list). **Mandatory parameter**.
- _**groupId**_ [String] - OPC-UA resource path. This path will be added to the endpoint resource name. Default value is "UA/JsonScada". **Optional parameter**.
- _**ipAddresses**_ [Array of Strings] - List of client's IP addresses allowed. Leave empty to allow any IP address to connect to the server. **Optional parameter**.
- _**ipAddressLocalBind**_ [String] - Interface bind IP address and port. Currently supports only IP "0.0.0.0". Default "0.0.0.0:4840". **Optional parameter**.
- _**topics**_ [Array of Strings] - List of _group1_ filter for the available tags on the OPC-UA server. Leave empty to include all tags. **Optional parameter**.
- _**timeoutMs**_ [Double] - Timeout. The HEL/ACK transaction timeout in ms. Use a large value (i.e. 15000 ms) for slow connections or embedded devices. **Mandatory parameter**.
- _**useSecurity**_ [Boolean] - Use (true) or not (false) secure encrypted connection. **Mandatory parameter**.
- _**localCertFilePath**_ [String] - File that contains the certificate (\*.PEM) that will be presented to the remote side of the connection (equiv. to NodeJS TLS option 'cert'). **Optional parameter**.
- _**privateKeyFilePath**_ [String] - File (\*.PEM) that contains the private key corresponding to the local certificate (equiv. to NodeJS TLS option 'key'). **Optional parameter**.
- _**historyEnabled**_ [Boolean] - Master switch for OPC UA Historical Access (HistoryRead). Default `false` (behavior identical to when this feature did not exist). See [Historical Data Access](#historical-data-access-opc-ua-ha). **Optional parameter**.
- _**historian**_ [String] - History backend when `historyEnabled` is true: `"mongodb"` (default) reads the MongoDB `hist` timeseries collection; `"postgresql"` reads the PostgreSQL/TimescaleDB `hist` hypertable. **Optional parameter**.
- _**postgresConnectionString**_ [String] - Optional `postgres://user:pass@host:port/db` for the `postgresql` historian. Empty ⇒ use `PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE` env vars if set, else the local defaults `127.0.0.1 / json_scada / json_scada : 5432`. Ignored for the mongodb backend. **Optional parameter**.
- _**historyMaxReturnDataValues**_ [Number] - Hard cap on values returned per tag per HistoryRead extraction (also published as `HistoryServerCapabilities.MaxReturnDataValues`). Default `20000`. **Optional parameter**.
- _**historyQueryTimeoutMs**_ [Number] - Backend query timeout in ms. Default `10000`. **Optional parameter**.
- _**historyTimestampField**_ [String] - Which timestamp the history range filters/orders on: `"timeTag"` (default, server receive time) or `"timeTagAtSource"` (source time; for MongoDB this mode benefits from an extra index `db.hist.createIndex({ tag: 1, timeTagAtSource: 1 })`). **Optional parameter**.
- _**historyFullHAConfiguration**_ [Boolean] - When `true` (default) each historized variable gets a full per-node `HA Configuration` object (spec-compliant). Set `false` for a "lite" install (no `HA Configuration` object) to keep the address space small on systems with tens of thousands of tags. **Optional parameter**.
- _**stats**_ [Object] - Protocol statistics updated by the driver. **Mandatory parameter**.

## Commands Routing

Commands received on this driver (OPC UA tag writes) can be routed to protocol clients. When commands are enabled for the connection, command tags will be automatically made available on the OPC-UA server. When a write is performed on an OPC object (associated with a command tag) on the OPC-UA server, the command is routed to the protocol source of the command tag.

## Historical Data Access (OPC UA HA)

When `historyEnabled: true`, the driver exposes JSON-SCADA history through the standard OPC UA Historical Access service (`HistoryRead`), so clients such as UaExpert (History Trend View), Ignition, Kepware and PI connectors can retrieve past values.

- **Raw history** (`ReadRawModifiedDetails`) — the full history of a tag over a time range, with quality (Good/Bad from the JSON-SCADA `invalid` flag) and both source and server timestamps.
- **Aggregates** (`ReadProcessedDetails`) — Interpolative, Average, Minimum, Maximum and Count, computed on the fly over the raw data. Only **one** aggregate function per request (a node-opcua limitation).
- Historical metadata is published: the `Historizing` attribute, the `AccessLevel.HistoryRead` bit, a per-node `HA Configuration` object (unless the "lite" install is chosen), and `Server/ServerCapabilities/HistoryServerCapabilities`.

The driver **reads** history; it never writes it. Historization is performed by `cs_data_processor`, which stores every eligible update into the `hist` store (MongoDB timeseries collection and/or the PostgreSQL/TimescaleDB hypertable). Choose the backend with the `historian` property.

A variable is exposed with history when all of the following hold: history is enabled for the connection; the tag is not a command (`origin != "command"`); the tag is historized (`historianPeriod` is not negative); and its type is `analog`, `digital` or `string` (`json`/array tags are not historized in this phase).

Retention is governed by the store, not the driver: the MongoDB `hist` timeseries collection has a TTL (about two months by default) and the TimescaleDB `hist` hypertable has its own retention policy (45 days by default). Requests for ranges older than the retained data simply return empty results.

### Limitations

- No `HistoryUpdate` (insert/replace/delete), no event history (`ReadEventDetails`), no `ReadAtTimeDetails`, no ModifiedValues (`isReadModified=true`) — these return `Bad` / unsupported.
- Bounding values (`returnBounds=true`) are not synthesized.
- Extractions are capped at `historyMaxReturnDataValues` per tag; clients should page with `numValuesPerNode` (continuation points are supported).
- As with live variables, tags are enumerated only at startup — tags created later appear after a driver restart.

## Command Line Arguments

This driver has the following command line arguments.

- _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
- _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
- _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../../conf/json-scada.json"**.

## Example of JSON-SCADA Protocol Driver Instances and Connections Numbering

![Driver instances and connections](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_Connections.png 'Driver Instances and Connections Numbering')
