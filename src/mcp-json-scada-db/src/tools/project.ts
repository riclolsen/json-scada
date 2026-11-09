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

// Project backup/restore through the AdminUI API (exportProject /
// importProject, which run the platform export_project / import_project
// scripts: configuration collections, displays and settings in one zip).

import fs from 'node:fs/promises'
import path from 'node:path'
import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import { AdminApiClient } from '../jsonscada/admin-api.js'
import { errorResult, jsonResult, textResult } from './util.js'

const ZIP_NAME = /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,250}\.zip$/

export function registerProjectTools(server: McpServer, api: AdminApiClient) {
  server.registerTool(
    'export_project',
    {
      title: 'Export Project',
      description:
        'Back up the JSON-SCADA project (configuration collections such as connections, tags, drivers, users ' +
        'and roles, plus displays and settings, as the AdminUI Project export does) to a zip file saved on the ' +
        'MCP server host in outDir. Do this before larger configuration changes.',
      inputSchema: z.object({
        fileName: z.string().regex(ZIP_NAME).optional().describe('Plain zip name (default project_<timestamp>.zip)'),
        outDir: z.string().min(1).describe('Folder on the MCP server host to save the zip into'),
      }),
      annotations: { readOnlyHint: false, destructiveHint: false },
    },
    async ({ fileName, outDir }) => {
      try {
        const name = fileName || `project_${new Date().toISOString().replace(/[-:]/g, '').replace(/\..*/, '')}.zip`
        const data = await api.postDownload('exportProject', { project: { fileName: name } })
        if (data.length < 4 || data.readUInt32LE(0) !== 0x04034b50)
          return textResult(`Export did not return a zip file (${data.length} bytes): ${data.toString('utf8', 0, 300)}`, true)
        await fs.mkdir(outDir, { recursive: true })
        const file = path.join(outDir, name)
        await fs.writeFile(file, data)
        return jsonResult({ saved: file, sizeKB: Math.round(data.length / 1024) })
      } catch (e) {
        return errorResult('Error exporting the project', e)
      }
    }
  )

  server.registerTool(
    'import_project',
    {
      title: 'Import Project',
      description:
        'Restore a project zip (from export_project or the AdminUI) on the JSON-SCADA server: the platform ' +
        'import script deletes realtimeData (tags), protocolConnections, protocolDriverInstances, ' +
        'processInstances, users and roles and loads them from the zip, and copies its displays into svg. This ' +
        'overwrites the current configuration for every user (the MCP admin account must exist in the ' +
        'imported users to keep using the admin tools): export a backup first. Needs confirm=true.',
      inputSchema: z.object({
        filePath: z.string().min(1).describe('Project zip on the MCP server host'),
        confirm: z.boolean().optional().default(false),
      }),
      annotations: { readOnlyHint: false, destructiveHint: true },
    },
    async ({ filePath, confirm }) => {
      try {
        const name = path.basename(filePath)
        if (!ZIP_NAME.test(name)) return textResult('The file name must be a plain .zip name (letters, digits, . _ -).', true)
        const data = await fs.readFile(filePath)
        if (data.length < 4 || data.readUInt32LE(0) !== 0x04034b50) return textResult('Not a zip file.', true)
        if (!confirm)
          return jsonResult({ imported: false, file: filePath, sizeKB: Math.round(data.length / 1024), note: 'Pass confirm=true to replace the current project configuration.' })
        const form = new FormData()
        form.append('projectFileName', name)
        form.append('projectFileData', new Blob([data], { type: 'application/zip' }), name)
        await api.postForm('importProject', form)
        return jsonResult({
          imported: true,
          file: filePath,
          note: 'Restart processes and drivers to load the imported configuration (restart_all_processes, restart_all_protocol_drivers).',
        })
      } catch (e) {
        return errorResult('Error importing the project', e)
      }
    }
  )
}
