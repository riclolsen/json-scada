# {json:scada} MCP Database Server

This is a Model Context Protocol (MCP) server that provides access to a {json:scada} MongoDB database. It allows AI models to query real-time data, historical records, system logs, and send commands to control points within the SCADA system.

## Features

- **Point Discovery**: Search for points by tag, description, or group.
- **Data Access**: Retrieve current values and attributes for specific tags.
- **Database Exploration**: List collections, inspect schemas, and perform custom MongoDB queries.
- **Command Dispatch**: Send commands to control points (requires command origin).
- **System Inspection**: Monitor process instances and driver statuses, and read the audit log.
- **Configuration & Process Management** (optional, see below): create/update/delete protocol driver instances, protocol connections and tags; start/stop/restart driver services; change process management settings.

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
- `JS_MCPJSDB_SRC_DIR`: Installation `src` folder used to read driver documentation (defaults to the parent folder of this package).

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
- `query_collection`: Run a custom read-only query (MongoDB Extended JSON filter, projection and sort) on any collection. Credential fields are redacted.
- `describe_collection`: Get estimated document counts and sample documents.
- `list_database_info`: High-level summary of the database.
- `get_collection_fields`: Inspect unique fields in a collection.
- `get_database_schema`: View the known {json:scada} schema.

- `list_protocol_drivers`: Supported protocol driver names (values for `protocolDriver`) and their client/server role.
- `get_protocol_driver_documentation`: The README of a driver, documenting its connection settings and tag addressing.
- `get_protocol_connection`: Full configuration of one protocol connection (secrets redacted).
- `get_user_actions`: Audit log (`userActions`) filtered by user, action, tag and time.

### Configuration and Process Management Tools

These tools are registered only when `JS_MCPJSDB_ADMIN_USERNAME` and `JS_MCPJSDB_ADMIN_PASSWORD` are set. They call the AdminUI backend API (`server_realtime_auth`, `/Invoke/auth/*`) as that user instead of writing MongoDB directly, so they get the same validation, protocol defaults, automatic driver restarts, OS service handling (NSSM on Windows, supervisord on Linux) and audit trail as the AdminUI. `server_realtime_auth` must be running and reachable at `JS_MCPJSDB_ADMIN_URL`.

Use a dedicated admin account, since these tools can stop data acquisition and change the system configuration. Tools that delete, stop or restart are marked `destructiveHint` so MCP clients can ask for confirmation. Every change is recorded in `userActions` under that account.

- Driver instances and processes: `list_driver_processes`, `create_driver_instance`, `update_driver_instance`, `delete_driver_instance`, `control_driver_process` (start/stop/restart), `sync_driver_services`.
- Protocol connections: `get_protocol_connection_template`, `create_protocol_connection`, `update_protocol_connection`, `delete_protocol_connection` (optionally deleting its tags).
- Tags: `create_tag`, `update_tag` (including rename), `delete_tag`. Live value fields are never written; use `send_command` to act on the field.
- System: `get_system_settings`, `update_system_settings`, `restart_all_protocol_drivers`, `restart_all_processes`.

Updates change only the given fields. Fields maintained by running drivers (redundancy keep-alive, `stats`) are never sent back, so an update cannot trigger a redundancy failover.

Example (Windows service):

```
nssm set JSON_SCADA_mcp_server AppEnvironmentExtra JS_MCPJSDB_TRANSPORT=http JS_MCPJSDB_IP_BIND=127.0.0.1 JS_MCPJSDB_HTTP_PORT=6001 JS_MCPJSDB_ADMIN_USERNAME=mcp-admin JS_MCPJSDB_ADMIN_PASSWORD=<password>
```

### Registered Resources

- `json-scada://schema`: Provides a detailed text summary of the database structure and key collections.
