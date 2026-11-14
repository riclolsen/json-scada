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

// The _System.* tags maintained by the data processor (cs_data_processor,
// Node.js and Go): the special status tags (keys -1, -2) and the
// communication status tags (commstatus.js / commstatus.go). Keep the names,
// points and rules in step with those files and docs/schema.md
// ("Communication status tags").

export const SYSTEM_PREFIX = '_System.'
export const COMM_PREFIXES = ['_System.Driver.', '_System.Conn.', '_System.Comm.']
export const DATA_PROCESSOR = 'CS_DATA_PROCESSOR'
export const ALARM_BEEP_KEY = -1
export const DIGITAL_UPDATES_KEY = -2
// generated tags take keys below this one
export const COMM_KEY_CEILING = -1000
// the generator polls every 5 s; a tag not rewritten for this long while its
// source exists means the generator is not running
export const COMM_STALE_MS = 60000

export const DRIVER_POINTS = ['Alive', 'Enabled', 'ActiveNode', 'Version', 'KeepAliveAge']
export const CONN_POINTS = [
  'Status',
  'Enabled',
  'LinkUp',
  'StatsAge',
  'ActiveNode',
  'TagCount',
  'InvalidTags',
  'Health',
]
export const SUMMARY_POINTS = ['DriversDown', 'ConnectionsDown', 'ConnectionsTotal']

// fields the generator owns; the admin API refuses to change them
// (server_realtime_auth commStatusOwnedFields)
export const COMM_OWNED_FIELDS = [
  'tag',
  'type',
  'origin',
  'commStatus',
  'group1',
  'group2',
  'group3',
  'description',
  'ungroupedDescription',
]

// commstatus.js sanitize: characters outside A-Z a-z 0-9 _ - become _
export function sanitizeName(s: unknown): string {
  const r = String(s ?? '')
    .trim()
    .replace(/[^A-Za-z0-9_-]/g, '_')
  return r === '' ? '_' : r
}

export const driverTagBase = (driver: string, instance: number | string) =>
  `_System.Driver.${sanitizeName(driver)}.${sanitizeName(String(instance))}.`
export const connTagBase = (name: string) => `_System.Conn.${sanitizeName(name)}.`

export const isSystemTagName = (name: unknown) =>
  typeof name === 'string' && name.trim().startsWith(SYSTEM_PREFIX)
export const isCommStatusTagName = (name: unknown) =>
  typeof name === 'string' && COMM_PREFIXES.some((p) => name.trim().startsWith(p))

export const RESERVED_NAME_MESSAGE =
  'Tag names beginning with "_System." are reserved for tags the data processor creates and maintains ' +
  '(see get_system_tags_guide). Choose another name.'

export const SYSTEM_TAGS_GUIDE = `# _System.* tags (maintained by the data processor)

The data processor (\`cs_data_processor\`, Node.js or the Go drop-in \`cs_data_processor-go\`; process name
\`${DATA_PROCESSOR}\`) creates and maintains tags whose names begin with \`_System.\`. They live in \`realtimeData\`
with \`origin: "system"\` and \`group1: "_System"\`, and work like any other point: displays, viewers, alarms, events,
history, Grafana and SQL read them by tag name. The \`_System.\` name space is reserved: never create user tags in it.

## Status tags (fixed keys)

Created by the database seed, and by the processor at start when missing.

| Tag | Key | Meaning |
|---|---|---|
| \`_System.Status.AlarmBeep\` | ${ALARM_BEEP_KEY} | Alarm horn. Value 1 ("Beep Active") when a new alarm of priority 0 or 1 appears; priority 0 also sets \`beepType: 2\` (important beep). \`beepGroup1List\` holds the \`group1\` (stations) with unsilenced alarms. Silencing in the HMI (or \`acknowledge_alarms_events\` with the silence option) removes the user's stations from the list, and the value drops to 0 when the list is empty. The \`alarm_beep\` process and the HMI viewers play the sound from it. |
| \`_System.Status.DigitalUpdatesCnt\` | ${DIGITAL_UPDATES_KEY} | Count of digital updates of alarm-enabled points since the processor started (restarts from 0). Viewers use it as a change indicator. |

Do not edit or delete them. A stuck beep is silenced, not deleted.

## Communication status tags

Maintained by processor **instance 1, on its active node**, every 5 s, from \`protocolDriverInstances\` and
\`protocolConnections\`. Keys are allocated below ${COMM_KEY_CEILING}; each tag carries
\`commStatus: { kind: "driver" | "conn" | "summary", ref: <source _id hex>, point }\`. Characters outside
\`A-Z a-z 0-9 _ -\` in a driver or connection name become \`_\` (connection \`KAW 2\` -> \`_System.Conn.KAW_2.Status\`).

Per driver instance, \`_System.Driver.<protocolDriver>.<instanceNumber>.<Point>\` (group2 "Drivers"):

| Point | Type | Meaning |
|---|---|---|
| \`Alive\` | digital Running/Stopped, alarmed, priority 2 | Stopped after more than 4 consecutive 5 s readings of an unchanged \`activeNodeKeepAliveTimeTag\` (about 25 s); invalid when the instance never wrote a keep-alive |
| \`Enabled\` | digital | Instance enabled |
| \`ActiveNode\`, \`Version\` | string | Node running the instance, driver software version |
| \`KeepAliveAge\` | analog, s, not historized | Seconds since the keep-alive last advanced |

Per connection, \`_System.Conn.<connectionName>.<Point>\` (group2 "Connections", group3 the connection name):

| Point | Type | Meaning |
|---|---|---|
| \`Status\` | digital Online/Offline, alarmed, priority 1 | Online when the connection and its driver instance are enabled, the instance is alive, \`stats.isConnected\` is not false, and not every supervised tag of the connection is invalid |
| \`Enabled\` | digital | Connection enabled |
| \`LinkUp\` | digital | \`stats.isConnected\` as reported by the driver; invalid when the driver does not report it |
| \`StatsAge\` | analog, s, not historized | Seconds since the driver last updated \`stats\` |
| \`ActiveNode\` | string | Node of the driver serving the connection |
| \`TagCount\`, \`InvalidTags\` | analog | Supervised tags of the connection, and how many are invalid |
| \`Health\` | analog, %, deadband 1 | Share of valid supervised tags |
| \`Stats.<field>\` | analog | Only with \`commStatusMirrorStats: true\`: each numeric field of the connection's \`stats\` |

Summaries (group2 "Comm"): \`_System.Comm.DriversDown\` (enabled instances not alive), \`_System.Comm.ConnectionsDown\`
(enabled connections not Online), \`_System.Comm.ConnectionsTotal\`.

### Lifecycle

- Created when a driver instance or connection appears, renamed when a connection is renamed (history is stored by
  tag name, so it continues under the new name), removed when the instance or connection is removed (history kept).
- \`prune_configuration\` and project export/import keep them; the generator recreates whatever is missing.
- A tag deleted or changed by hand in MongoDB is recreated or overwritten. A user tag that takes one of the generated
  names blocks the generator for that point: rename or delete the user tag.

### What can be changed

The admin API (AdminUI, \`create_tag\` / \`update_tag\` / \`delete_tag\`) refuses to create tags in these name spaces, to
delete them, or to change ${COMM_OWNED_FIELDS.map((f) => '`' + f + '`').join(', ')}.
Everything else is editable with \`update_tag\` or \`set_point_properties\`: \`priority\`, \`alarmDisabled\` (for example
alarm when \`_System.Conn.X.Enabled\` goes off), limits and hysteresis (for example an alarm when \`Health\` drops below
80 %), \`isEvent\`, \`historianDeadBand\`/\`historianPeriod\`, annotations and notes.

### Switches

On the \`processInstances\` document of \`${DATA_PROCESSOR}\` instance 1 (no API or MCP tool writes it; use mongosh):

- \`commStatusTags\` (default true): \`false\` stops maintaining the tags and marks them invalid.
- \`commStatusMirrorStats\` (default false): \`true\` adds the \`Stats.<field>\` tags.

\`\`\`
db.processInstances.updateOne({ processName: "${DATA_PROCESSOR}", processInstanceNumber: 1 },
  { $set: { commStatusTags: true, commStatusMirrorStats: false } })
\`\`\`

## Using them

- **Displays:** bind like any tag; ready-made markup (connection LED, driver running, link state, health bar,
  overview badge, one clone faceplate per connection) in the svg-scada skill, section "Communication Status Monitoring"
  (\`get_svg_scada_skill\` with that section).
- **Alarms:** \`Alive\` and \`Status\` alarm out of the box; enable others with \`set_point_properties\` / \`update_tag\`.
- **Overview for an operator or an assistant:** \`check_system_tags\` reads them with the configuration and explains
  every connection that is Offline.
- **History and trends:** all are historized except the \`*Age\` points.

## Troubleshooting

| Symptom | Meaning / action |
|---|---|
| Every communication status tag frozen or missing | Processor instance 1 is not running, not active, or \`commStatusTags\` is false. Check \`get_system_status\` and the switch |
| \`Alive\` = Stopped | The driver process stopped or hangs (keep-alive no longer advances): \`diagnose_connection\`, the driver log, \`control_driver_process\` |
| \`Alive\` invalid | The instance never wrote a keep-alive (driver never started on any node) |
| \`Status\` = Offline with \`Alive\` = Running | Connection or instance disabled, \`LinkUp\` = false (no session/peer), or every tag of the connection invalid |
| \`LinkUp\` invalid | The driver does not report \`stats.isConnected\` (older driver build) |
| \`Health\` low, \`Status\` Online | Part of the tags invalid: addressing errors or a remote device down; \`diagnose_connection\` lists them |
| A generated tag missing for one connection | A user tag holds its name, or two connections have names that become equal after replacing characters |
| AlarmBeep keeps sounding | Unsilenced alarms in some station: silence with \`acknowledge_alarms_events\` (silence option) or from the HMI |
`
