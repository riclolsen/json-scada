# DOX: src/mcp-json-scada-db — MCP Server

## Purpose

Model Context Protocol (MCP) server that gives AI assistants (Claude, Cline, Cursor, Copilot, etc.) access to a JSON-SCADA system: realtime data, history, events, schema and commands, health diagnosis, process logs and SCADA displays (with the svg-scada markup skill for authoring them), plus optional configuration and operation through the AdminUI and realtime APIs (driver instances, protocol connections, tags, driver services, displays, users and roles, project export/import, alarm/event acknowledgement and point properties) and CIM-EMS network analysis control (models, bindings, study runs and results).

## Ownership

- mcp-json-scada-db owns the MCP server implementation and its AI-assistant skill docs and seed data (`.skills/`)
- `.skills/` holds reference material for AI tools (schema, `SKILL-mcp.md`, demo seed JSON); the server does not load it at runtime

## Local Contracts

- **Language:** TypeScript (Node.js)
- **Package manager:** npm
- **Build:** `npm run build` (TypeScript -> JS in `dist/`)
- **SDK:** MCP TypeScript SDK v2 (`@modelcontextprotocol/server` + `@modelcontextprotocol/node`), protocol revision 2026-07-28 with legacy (2025-era) fallback
- **Transports:** stdio via `serveStdio(factory)`; HTTP (`--http` / `JS_MCPJSDB_TRANSPORT=http`, `JS_MCPJSDB_IP_BIND`, `JS_MCPJSDB_HTTP_PORT`) via stateless `createMcpHandler(factory)` + `toNodeHandler` on plain `node:http`, with Host/Origin guards (`JS_MCPJSDB_ALLOWED_HOSTS` list on any bind address, else localhost when bound to loopback)
- `buildServer()` in `src/mcp-server.ts` is the single factory for both transports and both protocol eras; register tools/resources there
- Logs and diagnostics go to stderr (`jsonscada/logger.ts` uses `console.error`); stdout carries the stdio MCP stream, so nothing else may write to it
- All environment variables use the `JS_MCPJSDB_` prefix (`package.json` `config.envPrefix`); never read generic names like `PORT`/`BIND`
- Tool `inputSchema` must be a `z.object({...})` (zod >= 4.2), not a raw shape
- **No redundancy control:** the server is always active and may run on every node at once; it does not register in `processInstances` or take part in active/standby election. Do not add redundancy logic
- **Two data paths:**
  - Read tools query MongoDB directly through `ConnectionManager` and are always registered
  - Configuration/process tools (`tools/drivers.ts`, `connections.ts`, `tags.ts`, `system.ts`, `users.ts`, `project.ts`, `save_display` in `displays.ts`, `cim-ems-admin.ts`) call the AdminUI API (`server_realtime_auth` `/Invoke/auth/*`) through `jsonscada/admin-api.ts`, signing in as `JS_MCPJSDB_ADMIN_USERNAME`/`_PASSWORD` (JWT from the `x-access-token` cookie, re-sign-in once on rejection). They are registered only when both are set
  - Operator tools (`tools/operator.ts`) use the realtime Web API (`/Invoke/`, `AdminApiClient.invoke`: WriteRequest with the `ExtendedAlarmEventsAck` or `Description` attribute, as the HMI) with the same sign-in, so role rights (union over the roles), `group1List` (acknowledgements) and user-action auditing apply. OPC status severity decides failure (Good/Uncertain codes such as `GoodNoData` are success). The server silently skips property changes the role does not allow: send only the requested `_Properties` fields and re-read the point to report what was applied
- Filesystem reads are confined to one folder each, by base name: displays in `JS_MCPJSDB_SVG_DIR` (default `<src>/../svg`), logs in `JS_MCPJSDB_LOG_DIR` (default `<src>/../log`). Writes never touch the filesystem directly (`save_display`, `import_project` go through the API); `export_project` / `import_project` / `cim_ems_import_model` read or write only the host path the caller gives
- svg-scada skill (`tools/skills.ts`, always registered): `get_svg_scada_skill` tool, `json-scada://skills/svg-scada` resource and `svg-scada-display` prompt serve the committed skill read at request time from `JS_MCPJSDB_SVG_SKILL`, else `<svg dir>/.agents/skills/svg-scada/SKILL.md`, else `<install>/.agents/skills/svg-scada/SKILL.md`; never embed a copy of the skill in the server
- Display parsing (`parseDisplay` in `tools/displays.ts`) follows the svg-scada skill markup: tags from `tag`, `list[].tag`, clone `map`, `open` with `istag: 1`, popup `src` (except `block`, `notrace`, `preview:`), `set` sources of the chart macros (`#arc`, `#vega*`, ...) and `$V('TAG')` in scripts; `%n`/`@1` placeholders, `#` macros and `!` special tags are not database tags
- Health (`tools/health.ts`): liveness reuses `aliveInfo`/`isFresh` from `tools/status.ts`; driver log names mirror `driver-catalog.js` service names (`DRIVER_SERVICE`, `_<n>` suffix for instance > 1); keep both in step
- Admin tools never write MongoDB config collections directly; validation, protocol defaults, restart scheduling, NSSM/supervisord control and `userActions` auditing stay owned by `server_realtime_auth`
- Updates read the current document, merge only the given fields and send the full document back (the API's update handlers expect it). Never send back driver-maintained runtime fields (`activeNodeName`, `activeNodeKeepAliveTimeTag`, `softwareVersion`, `stats`) or tag live-value fields
- Create flows (create empty, then update) delete the placeholder if the update fails
- CIM-EMS models, bindings, runs, study requests and the `cimArtifacts` GridFS bucket are confidential: tools that take a collection name refuse them (`isConfidentialCollection` in `tools/util.ts`). Only the dedicated `cim_ems_*` tools serve CIM-EMS data:
  - `tools/cim-ems.ts` (always registered) reads the driver instances, CIM-EMS connections and result points (`realtimeData`) from MongoDB and decodes/limit-checks them
  - `tools/cim-ems-admin.ts` (admin API enabled and `JS_MCPJSDB_CIM_EMS_TOOLS` not `false`) uses the `cimEms/*` routes only: the API enforces isAdmin / cimEngineering and audits; changes are `cimStudyRequests` executed by the active driver node, so tools wait for the request (`waitSeconds`) and report `pending` when no driver picks it up
  - Model packages are never downloaded; artifacts are fetched only as JSON (binding proposals, element catalogs, contingency results, the `se-residuals` report)
  - Interpretation: `CIM_EMS_GUIDE` in `jsonscada/cim-ems.ts` is served by `cim_ems_get_guide` and the `json-scada://cim-ems/guide` resource; keep it consistent with the driver's conventions and status codes. Tools return plain-language `findings` next to the data; `residualHints` flags typical binding errors (sign, scale, open element) on suspect measurements
  - AI-assisted binding: `cim_ems_list_model_elements` (driver `listElements` catalog joined with the current binding revision and an optional proposal), `cim_ems_candidate_tags` (supervised analog/digital points outside CIM-EMS connections, quantity inferred as in `proposal.py` by `quantityOfTag`), `cim_ems_save_bindings` validation (tag exists, point type, unit vs quantity, no CIM-EMS result point, one model value per tag, slot exists in a given catalog; errors on approved entries block the save unless `force`; `point_key` refreshed from the point)
  - `jsonscada/cim-ems.ts` mirrors the driver's status codes and point addressing (`<mRID>|<side>|<quantity>`, kind in `protocolSourceCommonAddress`); keep it in step with `src/cim-ems`
- Every tool has a human-readable `title` and `annotations`: read tools carry `readOnlyHint: true`; delete/stop/restart tools carry `destructiveHint: true`; `send_command` (acts on field equipment) carries `destructiveHint: true` and `openWorldHint: true`
- `jsonscada/drivers.ts` holds the protocol driver name list (mirrors the AdminUI `ProtocolDriverInstancesTab` list) and each driver's README path under `src/`; keep both in sync when drivers are added

## Work Guidance

- Maintain MCP protocol compatibility for both stdio and HTTP transports and both protocol eras
- Tools must not rely on sessions, server-initiated requests, or unsolicited notifications; HTTP is stateless
- Tools must be self-documenting with clear descriptions and parameter schemas
- New skill docs go in `.skills/` with descriptive filenames

## Verification

- `npm install` — install dependencies
- `npm run build` — TypeScript compilation succeeds
- `npm test` (if tests exist)
- Admin tools e2e: seeded scratch `mongod` (replica set) + `server_realtime_auth` started with `JS_PROCESS_MGMT_DISABLE=true` and `JS_INSTALL_DIR` pointing at a scratch folder (so no OS services or maintenance scripts run). Drive every admin tool through an MCP client and verify the stored documents in MongoDB, including that keep-alive/`stats` fields survive updates
- CIM-EMS e2e: the scratch setup above seeded with the Svedala demo (`demo-docker/mongo_seed/files/init.sh` order) plus `cs_data_processor`, `python -m cim_ems` and `python -m cim_ems.demo_field`; drive every `cim_ems_*` tool, with and without `cimEngineering` on the role, and with the driver stopped (requests stay `pending`, status reports the dead instance). Interpretation: plant a sign error (`sign: -1` on one P binding), check that `cim_ems_get_estimation_report` lists it as bad data with the sign hint and `cim_ems_run_statistics` as chronic bad data. Binding assistance: unbind one substation, rebind it using only `cim_ems_list_model_elements` / `cim_ems_candidate_tags` / `cim_ems_save_bindings` (dry run clean with `elementsArtifactId`), and check that wrong type, wrong unit, missing tag, unknown slot and CIM-EMS result tags are reported
- Operation e2e (same scratch setup; `JS_MCPJSDB_SVG_DIR` / `JS_MCPJSDB_LOG_DIR` pointed at scratch folders): `get_display` on a demo display reports no false missing tags; `save_display` refuses invalid label JSON and an existing file without `overwrite`; `set_point_properties` changes only the given fields (a point with infinite limits keeps them after an annotation change) and reports `notApplied` for a role without the right; `acknowledge_alarms_events` counts drop; users/roles round trip (create, role change, delete; admin protected); `export_project` / `import_project` with stub platform scripts in `JS_INSTALL_DIR/platform-linux`; `diagnose_system` / `diagnose_connection` findings match the planted state; `read_log` refuses names outside the log folder
- Protocol smoke test: connect with a v1 SDK client (legacy) and a v2 `@modelcontextprotocol/client` pinned to `2026-07-28` over stdio and over `--http`; `listTools`, `callTool` and `readResource` must succeed (without MongoDB, tools return "Database not connected."); `get_svg_scada_skill` (full, `outline`, `section`, unknown section), the `json-scada://skills/svg-scada` resource and the `svg-scada-display` prompt work without MongoDB, also with `JS_MCPJSDB_SVG_DIR` pointed at a folder without the skill (falls back to the installation copy)
