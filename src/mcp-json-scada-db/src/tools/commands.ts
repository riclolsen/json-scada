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

import { z } from 'zod'
import { Double } from 'mongodb'
import type { McpServer } from '@modelcontextprotocol/server'
import { ConnectionManager } from '../jsonscada/connection-manager.js'
import { Origin } from '../jsonscada/types.js'
import { errorResult, notConnectedResult, textResult } from './util.js'

export function registerCommandsTools(
  server: McpServer,
  mgr: ConnectionManager
) {
  server.registerTool(
    'send_command',
    {
      title: 'Send Command',
      annotations: { readOnlyHint: false, destructiveHint: true, idempotentHint: false, openWorldHint: true },
      description:
        'Send a command to a command point (origin=command). Queues the command for ' +
        'dispatch by the protocol driver and optionally waits for delivery confirmation.',
      inputSchema: z.object({
        tag: z.string().describe('The tag name of the command point'),
        value: z.number().describe('The numeric value for the command'),
        username: z.string().optional().default('mcp-agent'),
        waitSeconds: z
          .number()
          .int()
          .min(0)
          .max(10)
          .optional()
          .default(3)
          .describe(
            'Seconds to wait for the protocol driver to confirm delivery (0 = do not wait)'
          ),
      }),
    },
    async ({ tag, value, username, waitSeconds }: any) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()

      try {
        const point = await mgr
          .getRealtimeDataCollection()
          .findOne({ tag, origin: Origin.Command } as any)

        if (!point)
          return textResult(
            `Command point with tag '${tag}' not found or is not a command origin.`,
            true
          )

        if (point.commandBlocked === true)
          return textResult(
            `Command point '${tag}' is blocked by the operator (commandBlocked=true). Command not sent.`,
            true
          )

        // Numerical protocol addresses are stored as BSON Doubles (as the rest
        // of the system does); non-numerical addressing is kept as-is.
        let addressing: any
        if (
          (point.protocolSourceCommonAddress != '' &&
            isNaN(point.protocolSourceCommonAddress as any)) ||
          isNaN(point.protocolSourceObjectAddress as any) ||
          isNaN(point.protocolSourceASDU as any)
        ) {
          addressing = {
            protocolSourceCommonAddress: point.protocolSourceCommonAddress,
            protocolSourceObjectAddress: point.protocolSourceObjectAddress,
            protocolSourceASDU: point.protocolSourceASDU,
          }
        } else {
          addressing = {
            protocolSourceCommonAddress: new Double(
              (point.protocolSourceCommonAddress as number) || 0
            ),
            protocolSourceObjectAddress: new Double(
              (point.protocolSourceObjectAddress as number) || 0
            ),
            protocolSourceASDU: new Double(
              (point.protocolSourceASDU as number) || 0
            ),
          }
        }

        const command = {
          protocolSourceConnectionNumber: new Double(
            (point.protocolSourceConnectionNumber as number) || 0
          ),
          ...addressing,
          protocolSourceCommandDuration: new Double(
            (point.protocolSourceCommandDuration as number) || 0
          ),
          protocolSourceCommandUseSBO: point.protocolSourceCommandUseSBO || false,
          pointKey: new Double((point._id as number) || 0),
          tag: point.tag || '',
          timeTag: new Date(),
          value: new Double(value),
          valueString: value.toString(),
          originatorUserName: 'MCP:' + username,
          originatorIpAddress: '127.0.0.1',
          delivered: false,
        }

        const result = await mgr
          .getCommandsQueueCollection()
          .insertOne(command as any)

        // register the action in the user actions log (audit trail)
        await mgr.getUserActionsCollection().insertOne({
          username: 'MCP:' + username,
          pointKey: point._id,
          tag: point.tag,
          action: 'Command',
          properties: {
            value: new Double(value),
            valueString: value.toString(),
          },
          timeTag: new Date(),
        } as any)

        if (!waitSeconds)
          return textResult(
            `Command queued. ID: ${result.insertedId}. Delivery not awaited.`
          )

        // poll the queued command for delivery/ack by the protocol driver
        const deadline = Date.now() + waitSeconds * 1000
        while (Date.now() < deadline) {
          await new Promise((resolve) => setTimeout(resolve, 250))
          const cmd = await mgr
            .getCommandsQueueCollection()
            .findOne({ _id: result.insertedId } as any)
          if (cmd?.['delivered'] === true) {
            const ack =
              cmd['ack'] === undefined
                ? 'no acknowledge information'
                : cmd['ack']
                  ? 'acknowledged (positive confirmation)'
                  : 'NEGATIVE acknowledge'
            return textResult(
              `Command delivered to protocol driver. ID: ${result.insertedId}. Ack: ${ack}.`
            )
          }
        }

        return textResult(
          `Command queued (ID: ${result.insertedId}) but not confirmed as delivered ` +
            `within ${waitSeconds}s. The protocol driver may be offline or the connection disabled.`
        )
      } catch (e) {
        return errorResult('Error sending command', e)
      }
    }
  )
}
