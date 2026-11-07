# n8n ↔ JSON-SCADA Integration Guide

This guide covers the bidirectional integration between JSON-SCADA and the
[n8n](https://n8n.io) workflow-automation platform, on both Linux and Windows.

## Components

| Component | Location | Role |
|-----------|----------|------|
| `N8N` protocol driver | [src/n8n-client](../src/n8n-client) | Pushes real-time notifications to n8n (webhooks) and runs an HTTP listener for inbound values/commands |
| `n8n-nodes-jsonscada` | [JSON-SCADA n8n Nodes](https://github.com/json-scada/n8n-nodes-jsonscada) | n8n community nodes: an action node (read/browse/command/ack/send-values) and a webhook trigger node |

## Architecture

```
  MongoDB (realtimeData / soeData)                         server_realtime_auth
        │  change streams                                    POST /Invoke (JWT, RBAC)
        ▼                                                          ▲
  ┌──────────────┐   (A) HTTP POST batches   ┌───────────────┐    │ (B2) read / command /
  │  N8N driver  │ ────────────────────────► │ n8n Webhook /  │    │      ack / browse
  │ (n8n-client) │                            │ "JSON-SCADA    │    │
  │              │ ◄──────────────────────── │  Trigger" node │    │
  │  listener    │   (B1) POST /n8n/updates   └───────────────┘    │
  │  :51930      │        values / autoCreate  "JSON-SCADA" action node
  └──────────────┘                                   (n8n-nodes-jsonscada)
```

- **(A) SCADA → n8n**: driver batches value changes / SOE events / integrity snapshots /
  heartbeats and POSTs them to the webhook URLs in the connection's `endpointURLs`.
- **(B1) n8n → SCADA (data)**: workflows POST values to the driver listener; tags are
  updated/auto-created under the N8N connection (like the telegraf-listener).
- **(B2) n8n → SCADA (supervision)**: the action node talks to `server_realtime_auth`
  `/Invoke` with a JWT — reads, browse, commands (RBAC-enforced, audited), alarm/event ack.

## 1. Configure the N8N connection (AdminUI)

1. AdminUI → **Protocol Driver Instances** → add an instance with driver `N8N`.
2. AdminUI → **Protocol Connections** → add a connection with driver `N8N`. Key fields:
   - **N8N Webhook URLs** (`endpointURLs`) — the Production Webhook URL(s) of your n8n
     *JSON-SCADA Trigger* (or generic Webhook) node.
   - **Outbound Bearer Token** (`passphrase`) — optional; if set, the driver adds
     `Authorization: Bearer <token>` to every webhook POST.
   - **Bind** (`ipAddressLocalBind`) — inbound listener, defaults to `0.0.0.0:51930`.
   - **Username / Password** — Basic-auth for inbound calls. **Leave username empty to
     refuse all inbound data/command calls.**
   - **Filters** (`topics`) — outbound filter grammar (see the
     [driver README](../src/n8n-client/README.md)); empty = all value changes, no SOE.
   - **Auto Create Tags**, **giInterval**, **deadBand**, **maxQueueSize**, **timeoutMs**,
     **options** (JSON) — see the driver README.

## 2. Run the driver

### Linux (supervisord)

The installer copies `n8nclient.ini` into the supervisor include dir. Enable it:

```bash
sudo supervisorctl reread
sudo supervisorctl update
sudo supervisorctl start n8nclient
```

Or manage it from the AdminUI process-management UI (driver `N8N`, service `n8nclient`).

### Windows (NSSM)

`create_services.bat` installs `JSON_SCADA_n8nclient` (demand-start). Start it:

```bat
net start JSON_SCADA_n8nclient
```

Or from the AdminUI process-management UI.

## 3. Install the n8n nodes

In n8n: **Settings → Community Nodes → Install** → `n8n-nodes-jsonscada`.

Air-gapped:

```bash
cd ~/.n8n/nodes                       # Windows: %USERPROFILE%\.n8n\nodes
npm install /path/to/json-scada/src/n8n-nodes-jsonscada
```

Create credentials in n8n:

- **JSON-SCADA API** — base URL of `server_realtime_auth`, plus a dedicated automation
  user. Create a JSON-SCADA role for it (e.g. `sendCommands`, `ackAlarms`, `ackEvents`,
  `group1List` scope) — least privilege.
- **JSON-SCADA N8N Listener** — the driver listener URL (`http://host:51930`) and the
  Basic-auth username/password from the connection (needed only for **Send Values**).

## 4. Wire the two directions

### SCADA → n8n

1. Add a **JSON-SCADA Trigger** node; copy its Production Webhook URL.
2. Paste it into the connection's **N8N Webhook URLs** in the AdminUI.
3. If you set an Outbound Bearer Token, enable **Require Bearer Token** on the node and
   paste the same value.
4. Activate the workflow. Value changes / events now start it.

### n8n → SCADA

- **Commands / ack / reads**: use the **JSON-SCADA** action node with the *JSON-SCADA API*
  credential. This is the recommended command path (RBAC + `userActions` audit).
- **Push values**: use the action node's **Send Values** operation with the *Listener*
  credential.

## 5. Optional: install n8n itself

n8n is **fair-code** licensed; JSON-SCADA does not bundle it. Install it on your machine:

- **Docker (any OS)**: the demo compose (`demo-docker/docker-compose.yaml`) includes an
  `n8n` service that pulls `docker.n8n.io/n8nio/n8n` at runtime.
- **npm (Linux)**: `sudo npm install -g n8n`; point it at the local PostgreSQL for
  persistence (`DB_TYPE=postgresdb`, `DB_POSTGRESDB_*`).
- **npm (Windows)**: `npm install n8n --prefix c:\json-scada\n8n`; run
  `node c:\json-scada\n8n\node_modules\n8n\bin\n8n` (optionally as an NSSM service).

Keep n8n's port (5678) firewalled; expose it via SSH tunnel or a reverse proxy.

### Open the n8n editor from the web UI

`server_realtime_auth` proxies the n8n editor on `/n8n`, and the AdminUI dashboard has an
**n8n** shortcut (admin users only) that shows it at `/#/n8n`. The route requires a
JSON-SCADA admin login, and n8n keeps its own login on top of it.

- Run n8n with `N8N_PATH=/n8n/` and `N8N_EDITOR_BASE_URL=http(s)://<server>/n8n/`. n8n still
  serves everything (pages, `/rest`, the `/rest/push` websocket, `/webhook`) from its root:
  `N8N_PATH` only sets the links in its pages, and the proxy strips `/n8n`. Webhook URLs
  therefore stay `http://<n8n host>:5678/webhook/<path>`.
- Point `server_realtime_auth` at n8n with `JS_N8N_SERVER` (default `http://127.0.0.1:5678/`).
- With `N8N_PATH` set, the editor no longer works when opened on port 5678 directly; use
  `/n8n/` through the web server.

## 6. Try it with the demo stack

```bash
cd demo-docker
docker compose up -d
```

Services `n8n` (port 5678, webhooks) and `n8n-client` start alongside the demo. Open the
n8n editor at http://127.0.0.1/n8n/ (or the dashboard **n8n** shortcut) and create the n8n
owner account on first use. Enable the demo
**N8N1** connection in the AdminUI (it ships disabled), point its `endpointURLs` at your
n8n webhook, and import an example workflow from
[examples](https://github.com/json-scada/n8n-nodes-jsonscada/tree/main/examples).

## No-code quick paths

- **n8n → SCADA now**: n8n HTTP Request node → `POST /Invoke/auth/signin` then `POST /Invoke`.
- **SCADA → n8n now**: MQTT-Sparkplug-B driver + n8n MQTT Trigger node.
- **Direct DB**: n8n MongoDB node against a read-only `realtimeData` user.

## Security checklist

- [ ] Dedicated least-privilege automation user/role for the action node.
- [ ] Inbound listener has a username/password (empty username = all inbound refused).
- [ ] HTTPS (or private network) between n8n and SCADA when they are on different hosts.
- [ ] Outbound Bearer Token set when the webhook is reachable by others.
- [ ] Direct driver commands stay disabled unless explicitly needed
      (`commandsEnabled` + `options.enableDirectCommands`).
- [ ] n8n port 5678 not exposed to untrusted networks.
