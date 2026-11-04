# DOX framework

- DOX is highly performant AGENTS.md hierarchy installed here
- Agent must follow DOX instructions across any edits

## Core Contract

- AGENTS.md files are binding work contracts for their subtrees
- Work products, source materials, instructions, records, assets, and durable docs must stay understandable from the nearest applicable AGENTS.md plus every parent AGENTS.md above it

## Read Before Editing

1. Read the root AGENTS.md
2. Identify every file or folder you expect to touch
3. Walk from the repository root to each target path
4. Read every AGENTS.md found along each route
5. If a parent AGENTS.md lists a child AGENTS.md whose scope contains the path, read that child and continue from there
6. Use the nearest AGENTS.md as the local contract and parent docs for repo-wide rules
7. If docs conflict, the closer doc controls local work details, but no child doc may weaken DOX

Do not rely on memory. Re-read the applicable DOX chain in the current session before editing.

## Update After Editing

Every meaningful change requires a DOX pass before the task is done.

Update the closest owning AGENTS.md when a change affects:

- purpose, scope, ownership, or responsibilities
- durable structure, contracts, workflows, or operating rules
- required inputs, outputs, permissions, constraints, side effects, or artifacts
- user preferences about behavior, communication, process, organization, or quality
- AGENTS.md creation, deletion, move, rename, or index contents

Update parent docs when parent-level structure, ownership, workflow, or child index changes. Update child docs when parent changes alter local rules. Remove stale or contradictory text immediately. Small edits that do not change behavior or contracts may leave docs unchanged, but the DOX pass still must happen.

## Hierarchy

- Root AGENTS.md is the DOX rail: project-wide instructions, global preferences, durable workflow rules, and the top-level Child DOX Index
- Child AGENTS.md files own domain-specific instructions and their own Child DOX Index
- Each parent explains what its direct children cover and what stays owned by the parent
- The closer a doc is to the work, the more specific and practical it must be

## Child Doc Shape

- Create a child AGENTS.md when a folder becomes a durable boundary with its own purpose, rules, responsibilities, workflow, materials, or quality standards
- Work Guidance must reflect the current standards of the project or user instructions; if there are no specific standards or instructions yet, leave it empty
- Verification must reflect an existing check; if no verification framework exists yet, leave it empty and update it when one exists

Default section order:
- Purpose
- Ownership
- Local Contracts
- Work Guidance
- Verification
- Child DOX Index

## Style

- Keep docs concise, current, and operational
- Document stable contracts, not diary entries
- Put broad rules in parent docs and concrete details in child docs
- Prefer direct bullets with explicit names
- Do not duplicate rules across many files unless each scope needs a local version
- Delete stale notes instead of explaining history
- Trim obvious statements, repeated rules, misplaced detail, and warnings for risks that no longer exist

## Closeout

1. Re-check changed paths against the DOX chain
2. Update nearest owning docs and any affected parents or children
3. Refresh every affected Child DOX Index
4. Remove stale or contradictory text
5. Run existing verification when relevant
6. Report any docs intentionally left unchanged and why

## User Preferences

When the user requests a durable behavior change, record it here or in the relevant child AGENTS.md

- Markdown links (including Child DOX Index entries) may only point to files committed to the repository: not untracked, gitignored (`*_PLAN.md`, `/conf`, `/demo-docker/*`) or local-only material. Run `node docs/check-links.mjs` after editing Markdown or renaming files/headings.
- Literal JSON examples in docs, READMEs and skills (fenced `json` blocks, inline `{...}` snippets) must be strict JSON that parses as written: no trailing commas, comments or unquoted keys. Parse them after editing.

## Child DOX Index

### Source Code — [src/](src/AGENTS.md)

All protocol drivers, web UI, data processors, and tools. Contains its own child index covering 25+ components with individual AGENTS.md files:

- [src/AdminUI/](src/AdminUI/AGENTS.md) — Vue.js web admin interface SPA
- [src/mcp-json-scada-db/](src/mcp-json-scada-db/AGENTS.md) — MCP server for AI tooling (data, commands, configuration, driver processes)
- [src/dnp3-go/](src/dnp3-go/AGENTS.md) — DNP3 client/server (Go, drop-in alternative)
- [src/modbus-go/](src/modbus-go/AGENTS.md) — Modbus client/server (Go, drop-in replacement; the services run it)
- [src/modbus/](src/modbus/AGENTS.md) — Modbus client/server (Node.js, behavioural reference)
- [src/mqtt-sparkplug/](src/mqtt-sparkplug/AGENTS.md) — MQTT/Sparkplug B (Node.js)
- [src/server_realtime_auth/](src/server_realtime_auth/AGENTS.md) — Realtime WebSocket server (Node.js)
- [src/calculations/](src/calculations/AGENTS.md) — Calculations engine (Go)
- [src/mongowr/](src/mongowr/AGENTS.md) — 1-way replication: MongoDB writer (Node.js)
- [src/mongofw/](src/mongofw/AGENTS.md) — 1-way replication: MongoDB forwarder (Node.js)
- [src/cs_custom_processor/](src/cs_custom_processor/AGENTS.md) — Custom processor (TypeScript)
- [src/cs_data_processor/](src/cs_data_processor/AGENTS.md) — Data processor (Node.js)
- [src/cs_data_processor-go/](src/cs_data_processor-go/AGENTS.md) — Data processor (Go, drop-in alternative)
- [src/OPC-UA-Client-Go/](src/OPC-UA-Client-Go/AGENTS.md) — OPC UA client (Go)
- [src/OPC-UA-Server/](src/OPC-UA-Server/AGENTS.md) — OPC UA server (Node.js)
- [src/OPC-DA-Client/](src/OPC-DA-Client/AGENTS.md) — OPC DA client (.NET Core)
- [src/OPC-DA-Server/](src/OPC-DA-Server/AGENTS.md) — OPC DA server (.NET Framework)
- [src/iec60870-5/cmd/iec101client] — IEC 101 client (Go)
- [src/iec60870-5/cmd/iec101server] — IEC 101 server (Go)
- [src/iec60870-5/cmd/iec103client] — IEC 103 client (Go)
- [src/iec60870-5/cmd/iec104client] — IEC 104 client (Go)
- [src/iec60870-5/cmd/iec104server] — IEC 104 server (Go)
- [src/iec61850/iec61850_client/](src/iec61850/iec61850_client/AGENTS.md) — IEC 61850 MMS client (Go)
- [src/iec61850/iec61850_server/](src/iec61850/iec61850_server/AGENTS.md) — IEC 61850 MMS server (Go)
- [src/iccp/](src/iccp/AGENTS.md) — ICCP TASE.2 client/server drivers (Go)
- [src/telegraf-listener/](src/telegraf-listener/AGENTS.md) — Telegraf listener (Node.js)
- [src/plc4j-client/](src/plc4j-client/AGENTS.md) — Various PLC protocols client via PLC4X (Java)
- [src/camera-onvif/](src/camera-onvif/AGENTS.md) — ONVIF camera (Node.js)
- Plus tools & utilities detailed in [src/AGENTS.md](src/AGENTS.md) (Node.js, Go, C#, C, Python, PHP)


### Platform Configurations

- [platform-windows/](platform-windows/AGENTS.md) — Windows installer, services, batch scripts, runtimes
- [platform-ubuntu-2404/](platform-ubuntu-2404/AGENTS.md) — Ubuntu 24.04 automated installer & config
- [platform-ubuntu-2604/](platform-ubuntu-2604/AGENTS.md) — Ubuntu 26.04 automated installer & config
- [platform-rhel9/](platform-rhel9/AGENTS.md) — RHEL 9 / Rocky 9 / Alma 9 installer & config
- [platform-rhel10/](platform-rhel10/AGENTS.md) — RHEL 10 / Rocky 10 / Alma 10 installer & config
- [platform-linux/](platform-linux/AGENTS.md) — Generic Linux build/export/restart scripts

### Configuration

- [conf-templates/](conf-templates/AGENTS.md) — template config files for all services: Nginx, MongoDB, PostgreSQL, Node.js apps, protocol drivers, Supervisor, TLS certs, Grafana dashboards
- `conf/` — central application configuration: `json-scada.json`, OPC UA TLS certs in `conf/opcua/` (see [conf/README.md](conf/README.md); no committed AGENTS.md, root rules apply)

### Documentation — [docs/](docs/AGENTS.md)

Installation guides, architecture docs, schemas, developer guides.

### Database Scripts

- [sql/](sql/AGENTS.md) — PostgreSQL/TimescaleDB DDL and maintenance scripts
- [mongo_seed/](mongo_seed/AGENTS.md) — MongoDB initialization and seed data

### SCADA Displays — [svg/](svg/AGENTS.md)

SVG synoptic display files with embedded SCADA JSON markup for real-time visualization.

### Docker Demo — `demo-docker/` (see [demo-docker/README.md](demo-docker/README.md); no committed AGENTS.md, root rules apply)

Docker Compose setup for full JSON-SCADA demo stack.

### Others

- `compile-docker/` — Docker Compose build of the demo binaries (Go drivers, PLC4J) and Node.js modules into `demo-docker/bin` and `src/`; keep its service set in step with the root `Dockerfile` build stages and `demo-docker/docker-compose.yaml` (see [compile-docker/README.md](compile-docker/README.md))
- `README.md` — authoritative project front page; `index.md` is generated from it by `node docs/sync-index.mjs` (never edit `index.md` by hand). After changing either file or any heading they link to, run `node docs/sync-index.mjs --check` (fails on a stale `index.md` or a broken link/anchor)
- `Dockerfile` — Main Docker image build
- `supervisord.conf` — Process manager config
- `.idx/` — IDX development environment config
- `.agents/skills/` — agent skills; `svg-scada` has an identical copy under `svg/.agents/skills/` (see [svg/AGENTS.md](svg/AGENTS.md))
- `.github/` — GitHub CI templates