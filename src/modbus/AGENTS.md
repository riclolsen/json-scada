# DOX: src/modbus — Modbus Client & Server Drivers

## Purpose

Native TypeScript/Node.js Modbus drivers. Two independent processes from one package:
`MODBUS` (client/master — polls devices, updates `realtimeData`, dispatches
`commandsQueue`) and `MODBUS_SERVER` (server/slave — serves tags to external masters,
relays their writes as JSON-SCADA commands). Supports Modbus TCP, TCP/TLS, RTU serial,
and RTU-encapsulated-in-TCP/TLS, with arbitrary byte orders for multi-register values.

## Ownership

- This folder owns both driver implementations and the shared Modbus protocol core.
- The services (`modbusclient`, `modbusserver`) and the AdminUI process manager run the Go
  replacement in [src/modbus-go](../modbus-go/AGENTS.md). This package is kept as the behavioural
  reference; a behaviour change here must be mirrored there (or listed as a difference in its README).

## Local Contracts

- **Language:** TypeScript (ESM), compiled with `tsc` to `dist/`. Node >= 20.
- **Structure:**
  - `src/core/` — transport-agnostic protocol engine (crc16, pdu, framing-mbap,
    framing-rtu, datacodec [byte orders], address, client-stack, server-stack,
    transports). No MongoDB / JSON-SCADA imports here.
  - `src/common/` — ported JSON-SCADA scaffolding (simple-logger, load-config, redundancy).
  - `src/client/` — `MODBUS` driver (entry `dist/client/main.js`).
  - `src/server/` — `MODBUS_SERVER` driver (entry `dist/server/main.js`).
  - `test/unit/`, `test/loopback/` — `node:test` suites.
- **Config:** MongoDB `protocolConnections` + `realtimeData`; args `<instance> <logLevel>
  <configFile>`; env prefixes `JS_MODBUS_` (client) / `JS_MODBUSSRV_` (server).
- **Only native dependency:** `serialport`, imported lazily (serial modes only).

## Work Guidance

- Keep `src/core/` free of database and driver concerns so it stays unit-testable and
  reusable by both drivers.
- Byte order is expressed as a permutation of the value's bytes (`CDAB`, `GHEFCDAB`, …)
  or a named alias (`BE`/`LE`/`SW`/`SB`); the codec reorders raw wire bytes then decodes
  as big-endian. All new value types must round-trip under arbitrary permutations.
- Tag addresses: `<area>:<offset>[.bN]` with `co`/`di`/`hr`/`ir`, 0-based; unit id is the
  `protocolSource/DestinationCommonAddress`.
- The server relays writes using the command tag's `protocolSource*` routing (like the
  IEC 104 server), not its own connection.

## Verification

- `npm install && npm run build` — compiles cleanly.
- `npm test` — unit (crc/codec/pdu/address) + loopback (client↔server over TCP & RTU).
- Interop: `diagslave`/`modpoll` for TCP/RTU/RTU-over-TCP; `pymodbus==3.15.0` (pinned; re-run the recipes when bumping it) for TLS and byte-order cross-checks.
  See `test/e2e/README.md`.
