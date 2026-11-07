# DOX: src/mcp-json-scada-db — MCP Server

## Purpose

Model Context Protocol (MCP) server that gives AI assistants (Claude, Cline, Cursor, Copilot, etc.) access to a JSON-SCADA system: realtime data, history, events, schema and commands, plus optional configuration and process management (driver instances, protocol connections, tags, driver services) through the AdminUI API.

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
- All environment variables use the `JS_MCPJSDB_` prefix (`package.json` `config.envPrefix`); never read generic names like `PORT`/`BIND`
- Tool `inputSchema` must be a `z.object({...})` (zod >= 4.2), not a raw shape
- **No redundancy control:** the server is always active and may run on every node at once; it does not register in `processInstances` or take part in active/standby election. Do not add redundancy logic
- **Two data paths:**
  - Read tools query MongoDB directly through `ConnectionManager` and are always registered
  - Configuration/process tools (`tools/drivers.ts`, `connections.ts`, `tags.ts`, `system.ts`) call the AdminUI API (`server_realtime_auth` `/Invoke/auth/*`) through `jsonscada/admin-api.ts`, signing in as `JS_MCPJSDB_ADMIN_USERNAME`/`_PASSWORD` (JWT from the `x-access-token` cookie, re-sign-in once on rejection). They are registered only when both are set
- Admin tools never write MongoDB config collections directly; validation, protocol defaults, restart scheduling, NSSM/supervisord control and `userActions` auditing stay owned by `server_realtime_auth`
- Updates read the current document, merge only the given fields and send the full document back (the API's update handlers expect it). Never send back driver-maintained runtime fields (`activeNodeName`, `activeNodeKeepAliveTimeTag`, `softwareVersion`, `stats`) or tag live-value fields
- Create flows (create empty, then update) delete the placeholder if the update fails
- CIM-EMS models, bindings, runs, study requests and the `cimArtifacts` GridFS bucket are confidential: tools that take a collection name refuse them (`isConfidentialCollection` in `tools/util.ts`); CIM-EMS is exposed only through its driver status and result points
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
- Protocol smoke test: connect with a v1 SDK client (legacy) and a v2 `@modelcontextprotocol/client` pinned to `2026-07-28` over stdio and over `--http`; `listTools`, `callTool` and `readResource` must succeed (without MongoDB, tools return "Database not connected.")
