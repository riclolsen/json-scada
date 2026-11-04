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

import path from 'node:path'
import { fileURLToPath } from 'node:url'

// Protocol driver names accepted by the system (mirrors the AdminUI
// ProtocolDriverInstancesTab list), with the README (relative to the src/
// folder of the installation) that documents the driver's connection settings.
export const PROTOCOL_DRIVERS: Record<string, { doc?: string; role: string }> =
  {
    'IEC60870-5-104': { doc: 'iec60870-5/cmd/iec104client/README.md', role: 'client' },
    'IEC60870-5-104_SERVER': { doc: 'iec60870-5/cmd/iec104server/README.md', role: 'server' },
    'IEC60870-5-101': { doc: 'iec60870-5/cmd/iec101client/README.md', role: 'client' },
    'IEC60870-5-101_SERVER': { doc: 'iec60870-5/cmd/iec101server/README.md', role: 'server' },
    'IEC60870-5-103': { doc: 'iec60870-5/cmd/iec103client/README.md', role: 'client' },
    IEC61850: { doc: 'iec61850/iec61850_client/README.md', role: 'client' },
    IEC61850_SERVER: { doc: 'iec61850/iec61850_server/README.md', role: 'server' },
    DNP3: { doc: 'dnp3-go/README.md', role: 'client' },
    DNP3_SERVER: { doc: 'dnp3-go/README.md', role: 'server' },
    'MQTT-SPARKPLUG-B': { doc: 'mqtt-sparkplug/README.md', role: 'client/server' },
    MODBUS: { doc: 'modbus-go/cmd/modbusclient/README.md', role: 'client' },
    MODBUS_SERVER: { doc: 'modbus-go/cmd/modbusserver/README.md', role: 'server' },
    'OPC-UA': { doc: 'OPC-UA-Client-Go/README.md', role: 'client' },
    'OPC-UA_SERVER': { doc: 'OPC-UA-Server/README.md', role: 'server' },
    'OPC-DA': { doc: 'OPC-DA-Client/README.md', role: 'client' },
    'OPC-DA_SERVER': { doc: 'OPC-DA-Server/README.md', role: 'server' },
    PLCTAG: { role: 'client' },
    PLC4X: { doc: 'plc4j-client/README.md', role: 'client' },
    'TELEGRAF-LISTENER': { doc: 'telegraf-listener/README.md', role: 'server' },
    'NODE-RED': { doc: 'node-red-driver/README.md', role: 'server' },
    N8N: { doc: 'n8n-client/README.md', role: 'client/server' },
    ICCP: { doc: 'iccp/iccp-client/README.md', role: 'client' },
    ICCP_SERVER: { doc: 'iccp/iccp-server/README.md', role: 'server' },
    I104M: { role: 'client' },
    PI_DATA_ARCHIVE_INJECTOR: { role: 'server' },
    PI_DATA_ARCHIVE_CLIENT: { role: 'client' },
    ONVIF: { doc: 'camera-onvif/README.md', role: 'client' },
  }

export const PROTOCOL_DRIVER_NAMES = Object.keys(PROTOCOL_DRIVERS) as [
  string,
  ...string[],
]

// The src/ folder of the installation (this package lives in src/mcp-json-scada-db,
// compiled code in its dist/ folder). Override with JS_MCPJSDB_SRC_DIR.
export function srcDir(): string {
  return (
    process.env['JS_MCPJSDB_SRC_DIR'] ||
    path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')
  )
}
