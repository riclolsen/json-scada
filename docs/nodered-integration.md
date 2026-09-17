# Integrating Node-RED with JSON-SCADA

[Node-RED](https://nodered.org) is a flow-based programming tool with thousands of
community nodes for protocols, cloud services, databases and devices. This guide covers
the ways to connect it to JSON-SCADA, from the native driver (recommended) to zero-code
alternatives.

## Options at a glance

| Approach | Directions | Broker? | Best for |
|---|---|---|---|
| **NODE-RED driver** + `node-red-contrib-jsonscada` | both, push | no | first-class, low-latency, command-capable integration |
| MQTT-Sparkplug driver + MQTT broker + Node-RED MQTT nodes | both | yes | sites already running an MQTT/Sparkplug bus |
| telegraf-listener (UDP JSON) + Node-RED `udp out` | Node-RED → SCADA only | no | quick one-way metrics ingest |

The rest of this guide focuses on the native driver; the alternatives are at the end.

## 1. Native driver (recommended)

### Architecture

The **NODE-RED** driver (`src/node-red-driver`) hosts a WebSocket JSON server. Node-RED
flows connect to it — from the same host, another host, or a container — using the
`node-red-contrib-jsonscada` palette (or plain `websocket` nodes). Four data paths are
supported:

1. Node-RED → JSON-SCADA (monitoring): flows inject values as protocol-sourced tags.
2. JSON-SCADA → Node-RED (distribution): flows subscribe to live tag updates.
3. Operator → Node-RED (control): commands on flow-owned tags are delivered to flows.
4. Node-RED → field device (control): flows command any commandable JSON-SCADA tag.

The wire protocol is documented in
[`src/node-red-driver/PROTOCOL.md`](../src/node-red-driver/PROTOCOL.md).

### Step 1 — configure the driver

Create a `NODE-RED` instance and a connection (Admin UI → Protocols, or via mongosh):

```js
db.protocolDriverInstances.insertOne({
  protocolDriver: "NODE-RED", protocolDriverInstanceNumber: 1,
  enabled: true, logLevel: 1, nodeNames: ["mainNode"],
})
db.protocolConnections.insertOne({
  protocolDriver: "NODE-RED", protocolDriverInstanceNumber: 1,
  protocolConnectionNumber: 9100, name: "NODERED1", enabled: true,
  commandsEnabled: true, autoCreateTags: true,
  ipAddressLocalBind: "0.0.0.0:51931", ipAddresses: ["127.0.0.1"],
  topics: [], password: "",
})
```

See [`src/node-red-driver/README.md`](../src/node-red-driver/README.md) for every field.

### Step 2 — run the driver service

- **Windows**: the `JSON_SCADA_nodered_driver` service is created by
  `platform-windows/create_services.bat`. Start it from `start_protocols.bat`, the
  Services console, or the Admin UI process manager.
- **Linux**: the `nodered_driver` supervisor program is installed from
  `platform-*/nodered_driver.ini`. Enable it via `supervisorctl` or the Admin UI.
- Or manage the instance directly from the Admin UI (Protocol Driver Instances → start).

### Step 3 — install Node-RED and the palette

The driver works with any Node-RED. If your JSON-SCADA doesn't already have it, you can install a **local** runtime to be managed by the platform:

- **Windows**: run `platform-windows\install_nodered.bat` (installs `node-red` and
  `node-red-contrib-jsonscada` into `platform-windows\nodered-runtime`, copies
  `conf\node-red-settings.js`). Start the `JSON_SCADA_nodered_runtime` service.
- **Linux**: use the optional Node-RED block at the end of the platform install script,
  or:

  ```bash
  cd ~/json-scada && mkdir -p nodered-runtime
  npm install --prefix nodered-runtime node-red@4 node-red-contrib-jsonscada
  cp conf-templates/node-red-settings.js conf/node-red-settings.js
  mkdir -p conf/node-red
  ```

  Enable the `nodered_runtime` supervisor program.

For an **existing** Node-RED, just install the palette from Manage Palette → Install →
`node-red-contrib-jsonscada`.

### Step 4 — build a flow

1. Drag a **jsonscada tag out** node; create a server config pointing at the driver
   (`127.0.0.1:51931`, or the driver host); set an address like `PLANT1~L2~temp`; wire an
   inject/function producing a numeric payload. The tag auto-creates.
2. Drag a **jsonscada tag in** node; subscribe by tag or topic; wire a debug node to see
   updates flowing back.
3. For commands, use **jsonscada cmd out** (command a field tag) or **jsonscada tag out**
   with *Commandable* + **jsonscada cmd in** (receive operator commands).

Import ready-made flows from Node-RED → Import → Examples → node-red-contrib-jsonscada.

### Security checklist

- Bind the driver to `127.0.0.1` when Node-RED is on the same host.
- Set a non-empty connection `password` (token) and restrict `ipAddresses`.
- Use TLS (`wss://`) for off-host links (set the cert/key fields on the connection).
- Keep `commandsEnabled: false` unless flows must issue or receive commands.
- Enable Node-RED `adminAuth` before any network exposure of the editor.
- Keep the credential secret out of the repository default: the supervisor program
  starts Node-RED through `platform-*/start_nodered_runtime.sh`, which generates a
  random `JS_NODERED_CRED_SECRET` into `conf/nodered.secret` on first start (set the
  environment variable to override it). Changing it invalidates the credentials
  already stored in flows.

## 2. Zero-code alternative: MQTT-Sparkplug

If you already run an MQTT broker, use the existing `MQTT-SPARKPLUG-B` driver and
Node-RED's built-in MQTT nodes. Configure the driver to publish/subscribe on your broker;
in Node-RED use `mqtt in`/`mqtt out`. This needs a broker and loose payload conventions,
but requires no new components.

## 3. Zero-code alternative: telegraf-listener (one-way ingest)

For quick monitoring-only ingest, the `TELEGRAF-LISTENER` driver accepts UDP JSON. From
Node-RED, send a `udp out` node to the listener's port (default 51920) with a JSON
payload shaped like Telegraf's socket_writer output. No commands, no delivery guarantee,
but zero extra software. See [`src/telegraf-listener/README.md`](../src/telegraf-listener/README.md).
