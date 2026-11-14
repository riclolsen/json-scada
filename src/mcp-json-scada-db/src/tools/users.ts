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

// Users and roles (access rights) through the AdminUI API: the server hashes
// passwords, protects the admin user/role and records user actions. Password
// hashes are never returned.

import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import { AdminApiClient } from '../jsonscada/admin-api.js'
import { errorResult, jsonResult, textResult } from './util.js'

const RIGHTS = {
  isAdmin: 'full administration: AdminUI configuration and every /Invoke/auth route',
  changePassword: 'may change own password',
  sendCommands: 'may send commands (limited to group1CommandList when not empty)',
  enterAnnotations: 'may set/clear point annotations (tag-outs)',
  enterNotes: 'may edit point notes',
  enterManuals: 'may enter manual values',
  enterLimits: 'may change analog alarm limits',
  substituteValues: 'may substitute point values',
  ackEvents: 'may acknowledge/remove events',
  ackAlarms: 'may acknowledge alarms and silence the beep',
  disableAlarms: 'may inhibit point alarms',
  cimEngineering: 'may import/activate CIM models and edit measurement bindings (CIM-EMS)',
}

const roleRights = z.object({
  isAdmin: z.boolean().optional(),
  changePassword: z.boolean().optional(),
  sendCommands: z.boolean().optional(),
  enterAnnotations: z.boolean().optional(),
  enterNotes: z.boolean().optional(),
  enterManuals: z.boolean().optional(),
  enterLimits: z.boolean().optional(),
  substituteValues: z.boolean().optional(),
  ackEvents: z.boolean().optional(),
  ackAlarms: z.boolean().optional(),
  disableAlarms: z.boolean().optional(),
  cimEngineering: z.boolean().optional(),
  group1List: z.array(z.string()).optional().describe('Visible group1 (stations); empty = all'),
  group1CommandList: z.array(z.string()).optional().describe('group1 allowed for commands; empty = all'),
  displayList: z.array(z.string()).optional().describe('Allowed displays; empty = all'),
  maxSessionDays: z.number().min(0).optional(),
})

async function listRoles(api: AdminApiClient): Promise<any[]> {
  const r = await api.call('listRoles')
  return Array.isArray(r) ? r : []
}

async function listUsers(api: AdminApiClient): Promise<any[]> {
  const r = await api.call('listUsers')
  return Array.isArray(r) ? r : []
}

const userView = (u: any) => ({
  _id: u._id,
  username: u.username,
  email: u.email,
  roles: (u.roles || []).map((r: any) => r?.name ?? r),
  ...(u.isLDAPUser ? { isLDAPUser: true, ldapDN: u.ldapDN, lastLDAPSync: u.lastLDAPSync } : {}),
})

export function registerUserTools(server: McpServer, api: AdminApiClient) {
  server.registerTool(
    'list_users_and_roles',
    {
      title: 'List Users and Roles',
      description:
        'List the JSON-SCADA users (username, email, role names; password hashes are never returned) and the ' +
        'roles with their rights and group/display filters, plus a glossary of the rights. A user has the union ' +
        'of its roles\' rights.',
      inputSchema: z.object({}),
      annotations: { readOnlyHint: true },
    },
    async () => {
      try {
        const [users, roles] = await Promise.all([listUsers(api), listRoles(api)])
        return jsonResult({
          users: users.map(userView),
          roles: roles.map(({ __v, ...r }: any) => ({ ...r, users: users.filter((u) => (u.roles || []).some((x: any) => String(x?._id ?? x) === String(r._id))).map((u) => u.username) })),
          rights: RIGHTS,
        })
      } catch (e) {
        return errorResult('Error listing users and roles', e)
      }
    }
  )

  server.registerTool(
    'save_user',
    {
      title: 'Create or Update User',
      description:
        'Create a user or update an existing one (matched by username): email, password (set/reset; hashed by ' +
        'the server, never echoed back) and roles (role names; when given, the user ends with exactly these ' +
        'roles). The admin user always keeps the admin role.',
      inputSchema: z.object({
        username: z.string().min(1).max(100),
        email: z.string().max(200).optional(),
        password: z.string().min(1).max(200).optional(),
        roles: z.array(z.string()).optional(),
      }),
      annotations: { readOnlyHint: false, destructiveHint: false },
    },
    async ({ username, email, password, roles }) => {
      try {
        const [users, allRoles] = await Promise.all([listUsers(api), listRoles(api)])
        if (roles) {
          const unknown = roles.filter((n) => !allRoles.some((r) => r.name === n))
          if (unknown.length) return textResult(`Unknown roles: ${unknown.join(', ')}. Existing: ${allRoles.map((r) => r.name).join(', ')}`, true)
          if (username === 'admin' && !roles.includes('admin')) return textResult('The admin user must keep the admin role.', true)
        }
        let user = users.find((u) => u.username === username)
        const created = !user
        if (!user) {
          if (!password) return textResult('A new user needs a password.', true)
          await api.call('createUser', {
            username,
            email: email ?? '',
            password,
            roles: (roles || []).map((n) => allRoles.find((r) => r.name === n)._id),
          })
        } else {
          const upd: Record<string, unknown> = { _id: user._id, username }
          if (email !== undefined) upd['email'] = email
          if (password !== undefined) upd['password'] = password
          if (Object.keys(upd).length > 2) await api.call('updateUser', upd)
          if (roles) {
            const current = (user.roles || []).map((r: any) => r?.name ?? String(r))
            for (const n of roles.filter((n) => !current.includes(n))) await api.call('userAddRole', { username, role: n })
            for (const n of current.filter((n: string) => !roles.includes(n))) await api.call('userRemoveRole', { username, role: n })
          }
        }
        user = (await listUsers(api)).find((u) => u.username === username)
        return jsonResult({ created, ...(password ? { passwordSet: true } : {}), user: user ? userView(user) : null })
      } catch (e) {
        return errorResult(`Error saving user ${username}`, e)
      }
    }
  )

  server.registerTool(
    'delete_user',
    {
      title: 'Delete User',
      description: 'Delete a user by username. The admin user cannot be deleted.',
      inputSchema: z.object({ username: z.string().min(1) }),
      annotations: { readOnlyHint: false, destructiveHint: true },
    },
    async ({ username }) => {
      try {
        if (username === 'admin') return textResult('The admin user cannot be deleted.', true)
        const user = (await listUsers(api)).find((u) => u.username === username)
        if (!user) return textResult(`User '${username}' not found.`, true)
        await api.call('deleteUser', { _id: user._id, username })
        return jsonResult({ deleted: username })
      } catch (e) {
        return errorResult(`Error deleting user ${username}`, e)
      }
    }
  )

  server.registerTool(
    'save_role',
    {
      title: 'Create or Update Role',
      description:
        'Create a role or update an existing one (matched by name) with access rights (' +
        Object.keys(RIGHTS).join(', ') +
        '), group1List / group1CommandList / displayList filters and maxSessionDays. Rights not given keep ' +
        'their value (false on a new role). Changes apply to the users\' next sign-in.',
      inputSchema: z.object({ name: z.string().min(1).max(100), rights: roleRights }),
      annotations: { readOnlyHint: false, destructiveHint: false },
    },
    async ({ name, rights }) => {
      try {
        const role = (await listRoles(api)).find((r) => r.name === name)
        if (role) {
          if (name === 'admin' && rights.isAdmin === false) return textResult('The admin role must keep isAdmin.', true)
          await api.call('updateRole', { _id: role._id, name, ...rights })
        } else await api.call('createRole', { name, ...rights })
        const saved = (await listRoles(api)).find((r) => r.name === name)
        return jsonResult({ created: !role, role: saved })
      } catch (e) {
        return errorResult(`Error saving role ${name}`, e)
      }
    }
  )

  server.registerTool(
    'delete_role',
    {
      title: 'Delete Role',
      description: 'Delete a role by name. A role still given to a user cannot be deleted (remove it from the users first).',
      inputSchema: z.object({ name: z.string().min(1) }),
      annotations: { readOnlyHint: false, destructiveHint: true },
    },
    async ({ name }) => {
      try {
        if (name === 'admin') return textResult('The admin role cannot be deleted.', true)
        const role = (await listRoles(api)).find((r) => r.name === name)
        if (!role) return textResult(`Role '${name}' not found.`, true)
        await api.call('deleteRole', { _id: role._id })
        return jsonResult({ deleted: name })
      } catch (e) {
        return errorResult(`Error deleting role ${name}`, e)
      }
    }
  )
}
