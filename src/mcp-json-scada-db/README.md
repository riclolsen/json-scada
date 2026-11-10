# {json:scada} MCP Database Server

This is a Model Context Protocol (MCP) server that provides access to a {json:scada} MongoDB database. It allows AI models to query real-time data, historical records, system logs, and send commands to control points within the SCADA system.

## Features

- **Point Discovery**: Search for points by tag, description, or group.
- **Data Access**: Retrieve current values and attributes for specific tags.
- **Database Exploration**: List collections, inspect schemas, and perform custom MongoDB queries.
- **Command Dispatch**: Send commands to control points (requires command origin).
- **System Inspection**: Monitor process instances and driver statuses, and read the audit log.
- **Health Diagnosis**: whole-system and per-connection checks (MongoDB, processes, drivers, point quality, command and event queues, process logs) with plain-language findings.
- **Logs**: list, tail, filter and search the process log files.
- **Displays**: read SCADA SVG displays with their bound points and live values; serve the svg-scada markup skill for authoring displays; with the admin tools, validate and save displays.
- **Operator Actions** (admin tools): acknowledge alarms and events, set annotations, alarm inhibition, limits, notes and substituted values.
- **Calculated Tags**: guide and formula catalog of the compiled calculations engine, design checks with a value preview, an audit of all calculated points; with the admin tools, checked creation and changes.
- **CIM-EMS network analysis**: decoded load flow, state estimation and contingency results with limit checks; with the admin tools, model import/activation, binding review and study runs.
- **Configuration & Process Management** (optional, see below): create/update/delete protocol driver instances, protocol connections and tags; start/stop/restart driver services; change process management settings; users and roles; project export/import.

## Redundancy

The MCP server has no redundancy control. It is always active, so it can run on every node of a redundant installation at the same time, each instance serving the clients that connect to it. It does not register in `processInstances`.

## Protocol Support

Built on the MCP TypeScript SDK v2 (`@modelcontextprotocol/server`). It implements MCP protocol revision **2026-07-28** and still serves older clients (2025-11-25 and earlier) on both transports:

- **stdio**: each connection uses the protocol era the client opens with.
- **HTTP**: stateless. Every request is handled on its own, with no `Mcp-Session-Id`. Older clients get stateless responses, and GET/DELETE session requests return `405`.
- DNS rebinding protection: HTTP requests whose `Host`/`Origin` hostname is not in `JS_MCPJSDB_ALLOWED_HOSTS` are rejected. When that variable is unset, requests must use a localhost name if the server is bound to a loopback address, and are not checked on any other bind address.

## Configuration

The server expects a MongoDB connection string. You can configure this via environment variables or a configuration file in the parent directory (standard {json:scada} behavior).

### Environment Variables

- `JS_MCPJSDB_TRANSPORT`: Set to `http` to run as an HTTP streamable server (defaults to `stdio`).
- `JS_MCPJSDB_HTTP_PORT`: HTTP port if running in HTTP mode (defaults to `6001`).
- `JS_MCPJSDB_IP_BIND`: Binding address if running in HTTP mode (defaults to `127.0.0.1`), use `0.0.0.0` to allow external connections from any host (use with caution, insecure).
- `JS_MCPJSDB_ALLOWED_HOSTS`: Comma-separated hostnames (no ports, IPv6 in brackets, e.g. `localhost,127.0.0.1,[::1]`) accepted in the HTTP `Host` and `Origin` headers, on any bind address. Set it when binding `0.0.0.0` behind a port published only on the loopback (as in a container). Unset by default.

- `JS_MCPJSDB_ADMIN_USERNAME` / `JS_MCPJSDB_ADMIN_PASSWORD`: Credentials of an AdminUI user with an `isAdmin` role. When both are set, the configuration and process management tools are registered (see below). Unset by default.
- `JS_MCPJSDB_ADMIN_URL`: Base URL of the AdminUI server (`server_realtime_auth`) used by those tools (defaults to `http://127.0.0.1:8080`).
- `JS_MCPJSDB_CIM_EMS_TOOLS`: Set to `false` to leave out the CIM-EMS model, binding, run and request tools (for example when network model details must not reach the AI provider). The CIM-EMS status and result tools stay available.
- `JS_MCPJSDB_SRC_DIR`: Installation `src` folder used to read driver documentation (defaults to the parent folder of this package).
- `JS_MCPJSDB_SVG_DIR`: Folder of the SCADA displays read by `list_displays` / `get_display` (defaults to the `svg` folder next to `src`). `save_display` writes through the AdminUI API, which always uses the installation `svg` folder.
- `JS_MCPJSDB_SVG_SKILL`: Path of the svg-scada skill served by `get_svg_scada_skill` (defaults to `<svg dir>/.agents/skills/svg-scada/SKILL.md`, then `.agents/skills/svg-scada/SKILL.md` in the installation folder).
- `JS_MCPJSDB_LOG_DIR`: Folder of the process log files (defaults to the `log` folder next to `src`, where supervisor and the Windows services write `<process>.log`).

### Command Line Arguments

- **_1st arg. - Instance Number_** \[Integer] - Instance number to be executed. **Optional argument, default=1**. Env. variable: **JS_MCPJSDB_INSTANCE**.
- **_2nd arg. - Log. Level_** \[Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**. Env. variable: **JS_MCPJSDB_LOGLEVEL**.
- **_3rd arg. - Config File Path/Name_** \[String] - Path/name of the JSON-SCADA config file. **Optional argument, default="../../conf/json-scada.json"**. Env. variable: **JS_CONFIG_FILE**.
- **_4th arg. - --http_** \[String] - MCP transport (http or stdio). **Optional argument, default=stdio**. Env. variable: **JS_MCPJSDB_TRANSPORT**.
- **_5th arg. - --bind=ADDRESS_** \[String] - MCP bind address. **Optional argument, default=127.0.0.1**. Env. variable: **JS_MCPJSDB_IP_BIND**.
- **_6th arg. - --port=PORT_** \[Integer] - MCP port. **Optional argument, default=6001**. Env. variable: **JS_MCPJSDB_HTTP_PORT**.

## Usage

### Connect to the server

Via stdio, command:
c:\json-scada\platform-windows\nodejs-runtime\node.exe c:\json-scada\src\mcp-json-scada-db\dist\mcp-server.js 1 1 c:\json-scada\conf\json-scada.json

### Registered Tools

The server exposes the following tools to the MCP client:

- `search_points`: Search for points by tag/description regex, group, type, origin, alarmed and invalid filters. Returns compact summaries plus total match count.
- `get_point`: Get the full document of a specific point by tag or numeric point key.
- `send_command`: Write a value to a command point. Respects `commandBlocked`, records the action in `userActions` and can wait for delivery confirmation from the protocol driver.
- `get_history`: Get historical values for a point in a time range (from the `hist` collection).
- `get_soe_events`: Get Sequence of Events records with regex/group/time filters.
- `get_system_status`: Summary of process instances, protocol driver instances and protocol connections. Liveness comes from the instance keep-alive (under 30 s old); a driver instance with a stale keep-alive still counts as alive when one of its connections has fresh `stats` (server drivers without redundancy may not write a keep-alive). `aliveSource` (`keepAlive`, `connectionStats` or `null`) says which signal decided, and each connection carries `statsFresh`.
- `list_collections`: View all available database collections.
- `query_collection`: Run a custom read-only query (MongoDB Extended JSON filter, projection and sort) on any collection. Credential fields are redacted; CIM-EMS model, binding, run and artifact collections are refused.
- `describe_collection`: Get estimated document counts and sample documents.
- `list_database_info`: High-level summary of the database.
- `get_collection_fields`: Inspect unique fields in a collection.
- `get_database_schema`: View the known {json:scada} schema.

- `list_protocol_drivers`: Supported protocol driver names (values for `protocolDriver`) and their client/server role.
- `get_protocol_driver_documentation`: The README of a driver, documenting its connection settings and tag addressing.
- `get_protocol_connection`: Full configuration of one protocol connection (secrets redacted).
- `get_user_actions`: Audit log (`userActions`) filtered by user, action, tag and time.

Health and logs:

- `diagnose_system`: Overall health with `findings`: MongoDB (version, uptime, replica set members and lag, oplog window, database and disk usage), processes and driver instances alive, per-connection point quality (invalid, alarmed, alarm-disabled, substituted, frozen, last update), undelivered or rejected commands, unacknowledged events and recent error lines per log file.
- `diagnose_connection`: One connection: configuration (secrets redacted), driver instance liveness, statistics age, point quality with samples of invalid and least recently updated points, the last commands with delivery/ack results, warnings/errors of the driver log (`<service>.log`, e.g. `iec104client.log`, `iec104client_2.log` for instance 2) and other logs mentioning the connection name.
- `list_logs`: Log files with size and age; `scanErrors` counts recent error lines.
- `read_log`: Tail of one log, filtered by regex, level (`error`, `warn`) and time.
- `search_logs`: Regex/level search across all current logs.

Logs are read only from `JS_MCPJSDB_LOG_DIR` (file names cannot leave it).

Displays:

- `list_displays`: SVG displays with size, modification time and number of bound elements and points.
- `get_display`: One display described for interpretation: static texts with positions, every dynamic element with its animations (`get`, `color`, `bar`, `popup`, `clone`, `set` macros, ...) and bound tags, the bound points with live value, unit, quality, alarm, substitution and annotation, and broken bindings (missing tags, labels with invalid JSON).
- `get_svg_scada_skill`: The svg-scada skill, the markup rules for SCADA displays (HTML-escaped JSON in `inkscape:label`, animation types, clone faceplates, `set` macros, scripts, examples), so clients without repository skills can author displays. `outline: true` lists the headings; `section` returns one heading and its subsections (e.g. `"Color"`, `"Script"`). Read at request time, so skill edits apply without a rebuild.

### Configuration and Process Management Tools

These tools are registered only when `JS_MCPJSDB_ADMIN_USERNAME` and `JS_MCPJSDB_ADMIN_PASSWORD` are set. They call the AdminUI backend API (`server_realtime_auth`, `/Invoke/auth/*`) as that user instead of writing MongoDB directly, so they get the same validation, protocol defaults, automatic driver restarts, OS service handling (NSSM on Windows, supervisord on Linux) and audit trail as the AdminUI. `server_realtime_auth` must be running and reachable at `JS_MCPJSDB_ADMIN_URL`.

Use a dedicated admin account, since these tools can stop data acquisition and change the system configuration. Tools that delete, stop or restart are marked `destructiveHint` so MCP clients can ask for confirmation. Every change is recorded in `userActions` under that account.

- Driver instances and processes: `list_driver_processes`, `create_driver_instance`, `update_driver_instance`, `delete_driver_instance`, `control_driver_process` (start/stop/restart), `sync_driver_services`.
- Protocol connections: `get_protocol_connection_template`, `create_protocol_connection`, `update_protocol_connection`, `delete_protocol_connection` (optionally deleting its tags).
- Tags: `create_tag`, `update_tag` (including rename), `delete_tag`. Live value fields are never written; use `send_command` to act on the field. Calculated tags: `save_calculated_tag` (see Calculated Tags Tools).
- System: `get_system_settings`, `update_system_settings`, `restart_all_protocol_drivers`, `restart_all_processes`.
- Displays: `save_display` creates or overwrites (`overwrite: true`) an SVG display after checking that it is a complete SVG, that every SCADA label is valid JSON (refused otherwise) and which bound tags do not exist (warnings); `dryRun` runs the checks only.
- Operator actions, through the realtime Web API (`/Invoke/`, as the HMI): `acknowledge_alarms_events` (one tag or all: acknowledge alarms, acknowledge or remove events, silence the beep; returns counts before and after) and `set_point_properties` (annotation, alarm inhibition, analog limits and hysteresis, notes, substituted value; only the given fields change). The account's role rights (`ackAlarms`, `ackEvents`, `enterAnnotations`, `disableAlarms`, `enterLimits`, `enterNotes`, `substituteValues`, the union over its roles) apply, and `group1List` limits acknowledgements; the server ignores a change the role does not allow, so the tool re-reads the point and reports `applied` and `notApplied`.
- Users and roles: `list_users_and_roles` (password hashes never returned), `save_user` (create/update, password reset, exact role list), `delete_user`, `save_role` (rights, group/display filters, session length), `delete_role`. The admin user and role are protected.
- Project: `export_project` saves the AdminUI project zip to a folder on the MCP host; `import_project` (`confirm: true`) uploads one and runs the platform import script, which replaces tags, connections, driver and process instances, users and roles and copies the displays.
- Demo cleanup: `prune_configuration` removes the demo/example configuration so a system starts clean: every protocol connection (affected driver instances are scheduled for restart), every non-system tag (reserved keys, `origin: "system"`, communication status and `_System.*` tags stay), all CIM-EMS models, binding sets, runs, contingency lists, study requests and artifacts, and the example displays shipped in `svg` (`kaw2`, `kik3`, `knh2`, `kor1*`, `svedala*`; user displays stay). Driver and process instances, users, roles, settings, history and SOE records are kept. Each part can be skipped (`connections`, `tags`, `cimModels`, `displays`: `false`); without `confirm: true` it only reports what would be removed. Export the project first.

Non-driver processes (`cs_data_processor`, `calculations`, the realtime server, historians) can only be restarted all together (`restart_all_processes`): the AdminUI process manager starts, stops and configures protocol driver services only.

Updates change only the given fields. Fields maintained by running drivers (redundancy keep-alive, `stats`) are never sent back, so an update cannot trigger a redundancy failover.

Example (Windows service):

```
nssm set JSON_SCADA_mcp_server AppEnvironmentExtra JS_MCPJSDB_TRANSPORT=http JS_MCPJSDB_IP_BIND=127.0.0.1 JS_MCPJSDB_HTTP_PORT=6001 JS_MCPJSDB_ADMIN_USERNAME=mcp-admin JS_MCPJSDB_ADMIN_PASSWORD=<password>
```

### Calculated Tags Tools

Calculated points of the compiled [calculations engine](../calculations/README.md) (v0.2.0 or later) (`origin: "calculated"`, a `formula` code and its `parcels`, the input point keys in formula order). The engine has no expression language: each formula code is a compiled function with a fixed parcel order, so these tools carry the formula catalog and check designs against it.

Always registered:

- `get_calculations_guide`: How to create calculated tags (also the resource `json-scada://calculations/guide`): how the engine works (cycle, only `origin: "calculated"` points, reloaded every 30 s so no restart is needed; results go through the data processor, which applies `kconv1`/`kconv2`), the point fields, pitfalls, and every formula code with its expression, parcel count and order, result type and quality rule. With `formula`, one entry.
- `check_calculated_tag`: Checks a design (`formula` + `parcels` as tag names or point keys) or an existing tag: formula implemented, parcel count (a count that does not fit means the point is never computed), parcels exist (a missing parcel makes the result invalid), origin `calculated` (other origins are ignored), self references and loops through chains of calculated points, result type against the point type, and the calculations process status. Previews the engine output and the stored value from the current parcel values; for an existing tag, compares it with the stored value.
- `audit_calculated_tags`: Every calculated point (formula > 0 or origin `calculated`) with problems: unknown codes, wrong parcel counts, missing parcels, loops, formulas on non-calculated points (ignored by the engine), points never updated; plus formula usage, chain depth and the process status.

With the admin tools:

- `save_calculated_tag`: Creates a calculated tag, or changes the formula, parcels and other fields of an existing one, after the same checks; a design the engine cannot compute is refused. Sets `origin: "calculated"`, stores the parcels as point keys and picks the point type from the formula when none is given. `dryRun` only checks. The engine picks up the change within 30 s; nothing is restarted.

### CIM-EMS Tools

Control and interpretation of the [CIM-EMS driver](../cim-ems/README.md) (load flow, state estimation and contingency analysis on a CGMES model).

Always registered (read the driver status and the result points it publishes in `realtimeData`):

- `cim_ems_status`: Start here. Driver instances (alive), CIM-EMS connections and their study settings, last load flow / estimation / contingency status decoded from the `SYS` points, a limit check of the load flow results (branch loading above `loadingLimitPct`, bus voltages outside `vpuMin`..`vpuMax`) and plain-language findings.
- `cim_ems_get_results`: Results of one connection grouped per element: buses (V, VPU, ANG), branches (LOAD, P/Q/I per end and LOSS = P1 + P2) for `LF` or `SE`, one row per contingency for `CA`; filters by substation, voltage level and element, `violationsOnly` sorted by severity, `compareWith` (SE against LF or the reverse, sorted by the largest difference), total branch losses and a glossary of the quantities.
- `cim_ems_get_guide`: How to judge the results (also the resource `json-scada://cim-ems/guide`): what each study computes, status meanings, sign and unit conventions, voltage and loading judgement, normalized residuals and the chi-square test, typical causes of bad data, contingency outcomes.

With the admin tools (through the AdminUI `cimEms/*` routes, so with the same rights and audit trail as the Admin UI **CIM-EMS** tab: an Admin role for all, **CIM-EMS Engineering** for changes and result files):

- Inspection: `cim_ems_list_models` (revisions, validation report), `cim_ems_list_bindings` (revisions, coverage, filtered entries), `cim_ems_list_runs` (load flow with system totals, estimation chi-square, bad data and largest residuals with tags, insecure contingencies per run), `cim_ems_list_requests`, `cim_ems_get_request`.
- Interpretation:
  - `cim_ems_get_estimation_report`: every measurement of the latest estimate with tag, element name, measured, estimated, sigma and normalized residual; hints for typical binding errors (sign, kW/MW scale, phase/line voltage, element open in model or field); bias/RMS per quantity, the substations where residuals concentrate, the chi-square test and the measurements dropped before the estimate, with plain-language findings.
  - `cim_ems_get_contingency_result`: every contingency with its violations (element names, loading %) and `mostViolatedElements`, the elements overloaded by the most outages.
  - `cim_ems_run_statistics`: behaviour over the latest runs (up to 1000): convergence rate, durations, topology changes, frequent snapshot problems, generation/load/losses range, estimator chi-square pass rate and chronic bad data, contingencies most often insecure.
- Binding assistance (matching field tags to CIM objects): `cim_ems_list_model_elements` lists every model value a point can feed (switches, tap changers, P/Q per line or transformer end, V per busbar) with names, substation, voltage level, nominal kV, far end, the automatic matcher's label and its current/proposed binding, filtered by status (`unbound`, `unapproved`, `bound`); `cim_ems_candidate_tags` lists the supervised field points with unit, groups, live value, inferred quantity and current bindings. An assistant matches them substation by substation and saves with `cim_ems_save_bindings` (`upsert` with `approved: true`, `dryRun` first, `elementsArtifactId` to check the model slots).
- Control: `cim_ems_run_now` (immediate study, returns the new run), `cim_ems_import_model` (CGMES zip from a path on the MCP server host), `cim_ems_activate_model`, `cim_ems_rollback_model`, `cim_ems_propose_bindings`, `cim_ems_get_binding_proposal`, `cim_ems_save_bindings` (from the current revision, a proposal or empty; approve rules, removals, upserts; checks every entry against its point: tag exists, analog/digital type, unit against the quantity, not a CIM-EMS result point, one model value per tag; errors on approved entries block the save unless `force`; `dryRun` shows the checks and differences first).

Changes are study requests executed by the active CIM-EMS driver node: the tools wait up to `waitSeconds` and return the request outcome, or `pending` when the driver is not running (start it with `control_driver_process`, driver `CIM-EMS`). Connection settings (cycle, estimation, contingency list) are changed with `update_protocol_connection`. Model packages are never returned, and the generic collection tools still refuse the CIM-EMS collections.

### Registered Resources

- `json-scada://schema`: Provides a detailed text summary of the database structure and key collections.
- `json-scada://calculations/guide`: How to create calculated tags, with the formula catalog (same text as `get_calculations_guide`).
- `json-scada://cim-ems/guide`: How to interpret CIM-EMS results (same text as `cim_ems_get_guide`).
- `json-scada://skills/svg-scada`: The svg-scada skill (same text as `get_svg_scada_skill`).

### Registered Prompts

- `svg-scada-display` (`task`, optional `file`): starts a display creation/editing task with the svg-scada skill loaded and the steps to bind only existing tags and check with `save_display` `dryRun`.
