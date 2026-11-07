/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

import { McpServer, createMcpHandler } from "@modelcontextprotocol/server";
import { serveStdio } from "@modelcontextprotocol/server/stdio";
import {
  toNodeHandler,
  hostHeaderValidation,
  originValidation,
  localhostHostValidation,
  localhostOriginValidation,
} from "@modelcontextprotocol/node";
import { createServer } from "node:http";
import { registerPointsTools } from "./tools/points.js";
import { registerCommandsTools } from "./tools/commands.js";
import { registerCollectionsTools } from "./tools/collections.js";
import { registerInspectTools } from "./tools/inspect.js";
import { registerHistoryTools } from "./tools/history.js";
import { registerStatusTools } from "./tools/status.js";
import { registerDriverDocTools, registerDriverTools } from "./tools/drivers.js";
import {
  registerConnectionReadTools,
  registerConnectionTools,
} from "./tools/connections.js";
import { registerTagTools } from "./tools/tags.js";
import { registerAuditTools, registerSystemTools } from "./tools/system.js";
import { AdminApiClient } from "./jsonscada/admin-api.js";
import { ConnectionManager } from "./jsonscada/connection-manager.js";
import { Log } from "./jsonscada/index.js";
import packageInfo from "../package.json" with { type: "json" };

const ENV_PREFIX = packageInfo.config.envPrefix || "JS_MCPJSDB_";

// Initialize Connection Manager (no redundancy control: active on every node)
const mgr = new ConnectionManager();

// Configuration and process management tools go through the AdminUI backend
// API; they are only offered when admin credentials are configured.
const adminApi = new AdminApiClient();

// Builds a fully configured MCP server instance. A factory is needed because
// the SDK serves each stdio connection / HTTP request from its own instance.
// The same factory serves both protocol eras (2026-07-28 and 2025-era clients).
function buildServer(): McpServer {
  const server = new McpServer({
    name: packageInfo.name || "mcp-json-scada-db",
    version: packageInfo.version || "0.0.0",
  });

  registerPointsTools(server, mgr);
  registerCommandsTools(server, mgr);
  registerCollectionsTools(server, mgr);
  registerInspectTools(server, mgr);
  registerHistoryTools(server, mgr);
  registerStatusTools(server, mgr);
  registerDriverDocTools(server);
  registerConnectionReadTools(server, mgr);
  registerAuditTools(server, mgr);

  if (adminApi.enabled) {
    registerDriverTools(server, adminApi);
    registerConnectionTools(server, adminApi);
    registerTagTools(server, adminApi);
    registerSystemTools(server, adminApi);
  }

  // Resource: Schema
  server.registerResource(
    "schema",
    "json-scada://schema",
    { description: "JSON-SCADA Database Schema Summary" },
    async (uri) => {
      return {
        contents: [
          {
            uri: uri.href,
            text: `JSON-SCADA Database Schema Summary:
- realtimeData: Contains the current state of all points (tags). Fields: _id (pointKey), tag, description, type, value, timeTag, unit, invalid, alarmed, group1/2/3, etc.
- hist: Historical data for points. Fields: timeTag, tag, value, timeTagAtSource, invalid.
- soeData: Sequence of Events. Fields: timeTag, timeTagAtSource, tag, description, eventText, ack.
- commandsQueue: Queue for commands to be dispatched. Fields: tag, value, timeTag, delivered, ack, originatorUserName.
- processInstances: Status and config of system processes.
- protocolDriverInstances: Status and config of protocol drivers.
- protocolConnections: Configuration of individual communication links.
- userActions: Audit log of actions performed by users.`,
          },
        ],
      };
    }
  );

  return server;
}

async function main() {
  Log.log(
    adminApi.enabled
      ? `Admin tools enabled (AdminUI API ${adminApi.baseUrl}, user '${adminApi.user}')`
      : `Admin tools disabled (set ${ENV_PREFIX}ADMIN_USERNAME and ${ENV_PREFIX}ADMIN_PASSWORD to enable)`
  );
  // Start MongoDB connection (runs a reconnect loop in the background)
  mgr
    .run(() => {
      Log.log("MCP Server connected to MongoDB");
    })
    .catch((error) => {
      console.error("Fatal error in MongoDB connection loop:", error);
      process.exit(1);
    });

  const transportType =
    process.env[ENV_PREFIX + "TRANSPORT"] ||
    (process.argv.includes("--http") ? "http" : "stdio");

  try {
    if (transportType === "http") {
      const bind =
        process.env[ENV_PREFIX + "IP_BIND"] ||
        process.argv.find((arg) => arg.startsWith("--bind="))?.split("=")[1] ||
        "127.0.0.1";
      const portArg = process.argv
        .find((arg) => arg.startsWith("--port="))
        ?.split("=")[1];
      const port = parseInt(portArg || process.env[ENV_PREFIX + "HTTP_PORT"] || "6001", 10);

      // Stateless per-request handler: serves 2026-07-28 clients natively and
      // 2025-era clients through the SDK's stateless legacy fallback.
      const mcpHandler = toNodeHandler(createMcpHandler(() => buildServer()), {
        onerror: (error) => console.error("HTTP request error:", error),
      });

      // DNS rebinding protection: Host/Origin must be in ALLOWED_HOSTS when it is set
      // (any bind address, e.g. 0.0.0.0 in a container published on the host loopback),
      // otherwise localhost when bound to loopback, otherwise no check
      const allowedHosts = (process.env[ENV_PREFIX + "ALLOWED_HOSTS"] || "")
        .split(",")
        .map((h) => h.trim())
        .filter((h) => h !== "");
      const loopback = ["127.0.0.1", "localhost", "::1"].includes(bind);
      const guarded = allowedHosts.length > 0 || loopback;
      const validateHost = allowedHosts.length
        ? hostHeaderValidation(allowedHosts)
        : localhostHostValidation();
      const validateOrigin = allowedHosts.length
        ? originValidation(allowedHosts)
        : localhostOriginValidation();

      const httpServer = createServer(async (req, res) => {
        if (guarded && (!validateHost(req, res) || !validateOrigin(req, res)))
          return;
        await mcpHandler(req, res);
      });

      httpServer.listen(port, bind, () => {
        Log.log(`MCP Server running on HTTP at http://${bind}:${port}`);
      });
    } else {
      serveStdio(() => buildServer(), {
        onerror: (error) => console.error("stdio error:", error),
      });
      Log.log("MCP Server running on stdio");
    }
  } catch (error) {
    console.error("Failed to start MCP server:", error);
    process.exit(1);
  }
}

async function shutdown(signal: string) {
  Log.log(`Received ${signal}, shutting down...`);
  try {
    if (mgr.client) await mgr.client.close();
  } catch {
    // ignore errors while closing
  }
  process.exit(0);
}

process.on("SIGINT", () => shutdown("SIGINT"));
process.on("SIGTERM", () => shutdown("SIGTERM"));

main().catch((error) => {
  console.error("Fatal error in main():", error);
  process.exit(1);
});
