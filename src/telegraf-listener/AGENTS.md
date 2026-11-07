# DOX: src/telegraf-listener — Telegraf Listener

## Purpose

Node.js UDP listener that receives JSON messages from a Telegraf `socket_writer` output and writes them to MongoDB. Acts as the bridge between Telegraf data collection and the JSON-SCADA real-time database.

## Ownership

- telegraf-listener owns the Telegraf data ingestion endpoint

## Local Contracts

- **Language:** Node.js
- **Main entry:** `index.js` — simple single-file application
- **Config:** connection and instance in MongoDB; `conf/json-scada.json` for the database connection
- **Arguments/env:** instance, log level and config file arguments; `JS_TELEGRAF_LISTENER_*` environment variables

## Work Guidance

- Listens for UDP datagrams from Telegraf output plugins (`socket_writer` with `data_format = "json"`)
- Parses incoming Telegraf metrics and maps them to JSON-SCADA real-time data format
- Supports multiple Telegraf input sources (MQTT, Modbus, SNMP, OPC UA, etc.)
- The correlating Telegraf configs are in platform config dirs (e.g., `telegraf-input-*.conf`, `telegraf-output-json-scada.conf`)

## Verification

- `npm install` — dependencies install cleanly
- Send test Telegraf-formatted datagrams via UDP and verify they appear in MongoDB
