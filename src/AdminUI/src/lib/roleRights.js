// Boolean rights of a role (src/server_realtime_auth/app/models/role.model.js),
// with their label/hint keys under admin.rolesManagement and an icon.
// A user with several roles has the OR of them (docs/schema.md, RBAC roles).
export const ROLE_RIGHTS = [
  { key: 'isAdmin', label: 'isAdmin', icon: 'mdi-shield-crown-outline' },
  { key: 'changePassword', label: 'canChangePassword', icon: 'mdi-key-outline' },
  { key: 'sendCommands', label: 'sendCommands', icon: 'mdi-gesture-tap-button' },
  { key: 'enterAnnotations', label: 'enterAnnotations', icon: 'mdi-note-alert-outline' },
  { key: 'enterNotes', label: 'enterNotes', icon: 'mdi-note-text-outline' },
  { key: 'enterManuals', label: 'enterManuals', icon: 'mdi-hand-back-right-outline' },
  { key: 'enterLimits', label: 'enterLimits', icon: 'mdi-arrow-collapse-vertical' },
  { key: 'substituteValues', label: 'substituteValues', icon: 'mdi-swap-horizontal-bold' },
  { key: 'ackEvents', label: 'ackEvents', icon: 'mdi-check-all' },
  { key: 'ackAlarms', label: 'ackAlarms', icon: 'mdi-bell-check-outline' },
  { key: 'disableAlarms', label: 'disableAlarms', icon: 'mdi-bell-off-outline' },
  { key: 'cimEngineering', label: 'cimEngineering', icon: 'mdi-transmission-tower' },
  { key: 'viewSecEvents', label: 'viewSecEvents', icon: 'mdi-shield-alert-outline' },
  { key: 'editDisplays', label: 'editDisplays', icon: 'mdi-vector-square-edit' },
]

// minimum password length of server_realtime_auth (MIN_PASSWORD_LENGTH)
export const MIN_PASSWORD_LENGTH = 8

// rights of a user from its (populated) roles, combined as the server does
// (combineRights in src/server_realtime_auth/app/services/user-rights.js): flags
// OR-ed, lists concatenated, an empty group1List in any role unrestricted
export function effectiveRights(roles) {
  const list = (roles || []).filter((r) => r && typeof r === 'object')
  const rights = {}
  for (const r of ROLE_RIGHTS) rights[r.key] = list.some((x) => !!x[r.key])
  const concat = (k) => [...new Set(list.flatMap((x) => x[k] || []))]
  rights.group1List = list.some((x) => !(x.group1List || []).length) ? [] : concat('group1List')
  rights.group1CommandList = concat('group1CommandList')
  rights.displayList = concat('displayList')
  rights.maxSessionDays = list.reduce((m, x) => Math.max(m, Number(x.maxSessionDays) || 0), 0)
  return rights
}

export function initials(name) {
  const parts = String(name || '?')
    .split(/[\s._-]+/)
    .filter(Boolean)
  return ((parts[0]?.[0] || '?') + (parts[1]?.[0] || '')).toUpperCase()
}
