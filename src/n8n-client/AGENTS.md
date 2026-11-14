# DOX: src/n8n-client — N8N Integration Driver

## Purpose

Node.js protocol driver (`N8N`) that integrates JSON-SCADA with the n8n workflow-automation
platform in both directions:
- Outbound: MongoDB change streams (`realtimeData` value changes, `soeData` events) →
  batched HTTP POSTs to n8n Webhook URLs.
- Inbound: HTTP listener where n8n pushes values (auto-created supervised tags) and,
  optionally, commands.

## Ownership

- n8n-client owns the `N8N` protocol driver and its inbound HTTP listener (default port 51930).
- The companion operator/API path (commands, ack, history) is owned by
  `server_realtime_auth` `/Invoke` and the npm package `n8n-nodes-jsonscada`
  (`src/n8n-nodes-jsonscada`), not this driver.

## Local Contracts

- **Language:** Node.js (plain JS, modular; modeled on `mqtt-sparkplug` / `telegraf-listener`).
- **Main entry:** `index.js`. Modules: `app-defs`, `simple-logger`, `load-config`,
  `redundancy`, `filters`, `webhook-push`, `tags-creation`, `http-listener`.
- **CLI:** `node index.js [instance] [logLevel] [configFile]`; env prefix `JS_N8N_`.
- **Config:** one `protocolConnections` doc with `protocolDriver: "N8N"` per instance.
  Reuses existing schema fields — **no protocolConnection schema change**. See README for
  the field→meaning map and the `topics` filter grammar / `options` JSON.
- **Redundancy:** standard `protocolDriverInstances` keep-alive election; only the active
  node streams/pushes/accepts writes.
- **Key allocation:** auto-created tags use `connectionNumber * 100000 + seq`.
- **Connection stats:** `stats.isConnected` = the last webhook POST to n8n succeeded (`WebhookPusher.stats.lastOk`); before any POST, the inbound listener is up

## Work Guidance

- Outbound uses the same change-stream pipeline convention as `mqtt-sparkplug`
  (forwards *processed* value changes: updates where `sourceDataUpdate` is NOT in
  `updatedFields`, plus `replace`).
- Direct inbound commands are double opt-in: `commandsEnabled` AND
  `options.enableDirectCommands`; they insert into `commandsQueue` and write `userActions`.
- No external HTTP client dependency — outbound uses Node `http`/`https`; do not add axios.
- Keep payloads under n8n's default 16 MB body cap (batchMaxSize default 50).

## Verification

- `npm install` — dependencies install cleanly (express, mongodb, queue-fifo).
- `npm test` — runs `test/self-test.js` (no MongoDB): filter grammar, tag inference/creation,
  live outbound webhook delivery, inbound listener auth/auto-create/command-gating/health.
- Full E2E: demo stack + stub or real n8n; see `docs/n8n-integration.md`.
