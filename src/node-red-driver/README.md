# Node-RED Integration Driver (NODE-RED)

This bidirectional driver bridges [Node-RED](https://nodered.org) and JSON-SCADA. It
hosts a **WebSocket JSON server** that Node-RED flows connect to (via the companion
[`node-red-contrib-jsonscada`](https://github.com/json-scada/node-red-contrib-jsonscada.git) palette, or plain
`websocket` nodes), enabling four data paths:

1. **Node-RED → JSON-SCADA (monitoring)** — flows inject values that land in
   `realtimeData` as regular protocol-sourced tags, with quality, source timestamps,
   auto-tag creation, alarms, historian and SOE — exactly like any native driver.
2. **JSON-SCADA → Node-RED (distribution)** — flows subscribe to live tag updates
   (filtered by tag or `group1`), receiving a snapshot on subscribe and spontaneous
   updates thereafter.
3. **Operator → Node-RED (control)** — commands issued in JSON-SCADA on tags a flow
   owns are delivered into the flow, which can actuate and report a result back.
4. **Node-RED → field device (control)** — flows issue commands on any commandable
   JSON-SCADA tag, routed through `commandsQueue` to the owning field driver.

No broker is required (unlike the MQTT-Sparkplug path) and the driver works with a
Node-RED that is local, remote, or containerized.

The wire protocol is specified in [PROTOCOL.md](./PROTOCOL.md).

## Build

    cd src/node-red-driver
    npm install
    npm run build      # tsc → dist/
    npm test           # unit tests (no MongoDB required)

Run: `node dist/main.js <instance> <logLevel> <configFile>` (all optional; env
fallbacks `JS_NODERED_INSTANCE`, `JS_NODERED_LOGLEVEL`, `JS_CONFIG_FILE`).

## Configure a driver instance

Create a document in `protocolDriverInstances` (or use the Admin UI):

    use json_scada_db_name
    db.protocolDriverInstances.insertOne({
        protocolDriver: "NODE-RED",
        protocolDriverInstanceNumber: 1,
        enabled: true,
        logLevel: 1,
        nodeNames: ["mainNode"],
        activeNodeName: "mainNode",
        activeNodeKeepAliveTimeTag: new Date(),
    });

- **protocolDriver** [String] - must be `"NODE-RED"`. **Mandatory.**
- **protocolDriverInstanceNumber** [Double] - 1..N, unique per driver. **Mandatory.**
- **enabled** [Boolean] - false disables the instance. **Mandatory.**
- **logLevel** [Double] - 0=min, 1=basic, 2=detailed, 3=debug. **Mandatory.**
- **nodeNames** [Array of Strings] - nodes allowed to run this instance (redundancy).
  **Mandatory.**
- **activeNodeName** / **activeNodeKeepAliveTimeTag** - maintained by the driver for
  redundancy. **Optional.**

Only the **active** node binds the WebSocket port and processes data; a standby stays
connected to MongoDB and takes over on keep-alive timeout.

## Configure the connection

Each instance has exactly one connection in `protocolConnections`:

    use json_scada_db_name
    db.protocolConnections.insertOne({
        protocolDriver: "NODE-RED",
        protocolDriverInstanceNumber: 1,
        protocolConnectionNumber: 9100,
        name: "NODERED1",
        description: "Node-RED integration link",
        enabled: true,
        commandsEnabled: true,
        autoCreateTags: true,
        ipAddressLocalBind: "0.0.0.0:51931",
        ipAddresses: ["127.0.0.1"],
        topics: [],
        password: "",
        endpointURLs: [],
    });

- **protocolConnectionNumber** [Double] - unique across all connections of all drivers;
  owns the tags injected by flows. **Mandatory.**
- **name** [String] - used for logging and command originator. **Mandatory.**
- **enabled** [Boolean] - false disables the connection. **Mandatory.**
- **commandsEnabled** [Boolean] - gates **both** command directions (paths 3 & 4).
- **autoCreateTags** [Boolean] - auto-create tags for addresses seen from flows.
- **ipAddressLocalBind** [String] - `address:port` for the WS server
  (default `0.0.0.0:51931`). Use `127.0.0.1:51931` for same-host Node-RED.
- **ipAddresses** [Array of Strings] - allow-list of client source IPs; `[]` = allow any.
- **topics** [Array of Strings] - `group1` filter for tags **published** to Node-RED;
  `[]` = all tags.
- **password** [String] - shared access token required in the `hello` message;
  `""` = open (lab use only).
- **endpointURLs** [Array] - reserved for a future client mode; must be `[]` in v1.
- TLS (optional): **localCertFilePath**, **privateKeyFilePath**, **rootCertFilePath**,
  **chainValidation**, **allowTLSv10..13**, **cipherList**. When cert+key are set the
  server serves `wss://`.

## Connection status

On the active node the driver writes `stats.isConnected` on its connection
every 5 s: true while at least one Node-RED client is authenticated. The data
processor shows it as `_System.Conn.<name>.LinkUp` (see
[docs/schema.md](../../docs/schema.md#communication-status-tags)).

## Tag addressing

Flows address points with a free-form string. The suggested convention
`group1~group2~name` (using the `~` separator) makes auto-created tags land with
`group1`/`group2`/`description` populated, so they appear organized in the viewers.
Auto-created tag keys use `_id = protocolConnectionNumber * 100000 + sequence`.

## Security notes

- Prefer binding `127.0.0.1` when Node-RED runs on the same host.
- Set a non-empty `password` (token) and restrict `ipAddresses` for any networked use.
- Enable TLS (`wss://`) for off-host links.
- `commandsEnabled: false` disables both command directions entirely.

## Relationship to other integration options

See [`docs/nodered-integration.md`](../../docs/nodered-integration.md) for a comparison
with the broker-based MQTT-Sparkplug path and the telegraf UDP listener, plus runtime
installation and reverse-proxy setup for Windows and Linux.
