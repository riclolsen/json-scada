# DOX: src/server_realtime_auth — Realtime Data Server

## Purpose

Core realtime data server for JSON-SCADA. Handles client connections, JWT authentication, real-time data streaming from MongoDB Change Streams, user session management, and action queue processing. The primary API gateway for the web UI.

## Ownership

- server_realtime_auth owns the realtime data serving infrastructure
- All web UI clients connect through this server

## Local Contracts

- **Language:** Node.js
- **Main entry:** `index.js`
- **Auth:** JWT (JSON Web Tokens) for client authentication
- **Structure:**
  - `index.js` — main server entry point
  - `graphql-server.js` — GraphQL API endpoint
  - `customJsonQueries.js` — custom query support
  - `userActionsQueue.js` — user action processing queue
  - `opc_codes.js` — OPC data type codes
  - `app-defs.js` — application definitions
  - `load-config.js` — configuration loader
  - `simple-logger.js` — logging utility
  - `app/` — Express.js application subdirectory:
    - `controllers/` — route handlers
    - `models/` — data models
    - `routes/` — Express route definitions
    - `middlewares/` — Express middleware (auth, etc.)
    - `config/` — submodule configuration
- **Config:** INI file via Supervisor or environment variables
- **CIM-EMS:** `app/controllers/cimEms.controller.js`, routes `/Invoke/auth/cimEms/*` (list models, bindings, runs, requests; `uploadModel`, `request`, `artifact`). All need only a signed-in user (`authJwt.isUser`, not admin); model, binding and artifact actions also need a role with `cimEngineering` (`authJwt.isCimEngineer` / `hasCimEngineering`). The server only stores uploads/binding sets in the `cimArtifacts` GridFS bucket and queues `cimStudyRequests`; the CIM-EMS driver executes them (`src/cim-ems/cim_ems/api.py`), so request shapes change together. Every request and download is logged as a `cimEms:<type>` user action. `/GetFile` refuses the `cimArtifacts` bucket
- **Realtime Read extension:** a `NodesToRead` entry with `_ValueJson: true` also gets the point's `valueJson` in `_Properties.valueJson` (`null` when absent), on both Value and Description reads; other entries never carry it. Used by the AdminUI Display Viewer for `!JSON` markup
- **Point property writes** (`/Invoke/` WriteRequest, `AttributeId` Description, `Value._Properties`): each field is changed only when present and the user's roles grant its right (`enterAnnotations`, `disableAlarms`, `enterLimits`, `enterNotes`, `substituteValues`); fields without the right are skipped silently. A `null` `loLimit`/`hiLimit` (an Infinity serialized as JSON) means no limit (stored as -Infinity/Infinity)
- **Communication status tags:** `createTag`/`updateTag`/`deleteTag` refuse tags carrying `commStatus` (or named `_System.Driver.`/`_System.Conn.`/`_System.Comm.`) — no create or delete, no change to `tag`, `type`, `origin`, `commStatus`, groups or descriptions; other fields stay editable. The generator is `cs_data_processor` (see `docs/schema.md`)
- **Configuration prune:** `auth/pruneConfiguration` (admin, `pruneConfiguration` user action) is a dry run unless `confirm: true`; it deletes every protocol connection (scheduling the affected driver restarts), every tag except reserved keys -1/-2, `origin: "system"`, `commStatus` and `_System.*` tags (or, with `tags: false`, only the tags of the pruned connections), all CIM-EMS collections and the `cimArtifacts` bucket, and the shipped example displays matched by `DEMO_DISPLAY_RE` in `auth.controller.js`; parts are skipped with `connections`/`tags`/`cimModels`/`displays: false`. Driver/process instances, users, roles and settings stay
- **API consumers:** the `/Invoke/auth/*` admin routes (sign-in cookie `x-access-token`, `{ error }` responses, driver instance/connection/tag CRUD, process management, system settings, restart scripts, users/roles, displays, project export/import, configuration prune, `cimEms/*` CIM-EMS routes) and the realtime `/Invoke/` WriteRequest (alarm/event acknowledgement, point properties) are also called by `src/mcp-json-scada-db` (`jsonscada/admin-api.ts` and the admin and operator tools); keep request/response shapes compatible or update the MCP tools with them

## Work Guidance

- WebSocket connections stream real-time data from MongoDB Change Streams
- JWT tokens are issued on login and validated on every WebSocket message
- User action queue processes write operations to MongoDB
- The GraphQL endpoint provides an alternative query interface
- Supports RBAC (role-based access control) per user/role configuration
- Session management tracks connected clients and their subscriptions

## Verification

- `npm install` — dependencies install cleanly
- `node index.js` — server starts and listens on configured port
- Test WebSocket connection from AdminUI or wscat
- Verify JWT login flow (admin/jsonscada default credentials)
- Verify Change Stream subscriptions deliver real-time updates
