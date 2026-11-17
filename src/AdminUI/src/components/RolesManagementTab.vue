<template>
  <div class="roles-tab">
    <AdminSummary v-model="tileFilter" :tiles="summaryTiles" />

    <div class="admin-toolbar">
      <v-btn color="primary" variant="flat" prepend-icon="mdi-shield-plus-outline" @click="openAddRoleDialog">
        {{ $t('admin.rolesManagement.addRole') }}
      </v-btn>
      <v-text-field
        v-model="search"
        class="admin-search"
        density="compact"
        variant="outlined"
        hide-details
        clearable
        prepend-inner-icon="mdi-magnify"
        :label="$t('admin.rolesManagement.search')"
      ></v-text-field>
      <v-btn-toggle v-model="view" mandatory density="compact" variant="outlined" divided color="primary">
        <v-btn value="list" size="small" prepend-icon="mdi-format-list-bulleted">{{ $t('admin.rolesManagement.viewList') }}</v-btn>
        <v-btn value="matrix" size="small" prepend-icon="mdi-table-check">{{ $t('admin.rolesManagement.viewMatrix') }}</v-btn>
      </v-btn-toggle>
      <v-spacer></v-spacer>
      <span class="admin-count">{{ $t('admin.common.results', { n: filteredRoles.length }) }}</span>
      <v-btn icon variant="text" size="small" :title="$t('common.refresh')" :loading="loading" @click="refreshAll">
        <v-icon>mdi-refresh</v-icon>
      </v-btn>
      <v-btn icon variant="text" size="small" :title="$t('admin.common.exportCsv')" @click="exportCsv">
        <v-icon>mdi-file-delimited-outline</v-icon>
      </v-btn>
    </div>

    <!-- list -->
    <v-data-table
      v-if="view === 'list'"
      v-model:items-per-page="itemsPerPage"
      :headers="headers"
      :items="filteredRoles"
      :items-per-page-options="[10, 25, 50, 100, -1]"
      :items-per-page-text="$t('common.itemsPerPageText')"
      :sort-by="[{ key: 'name', order: 'asc' }]"
      :loading="loading"
      item-value="_id"
      hover
      density="compact"
      class="admin-table"
    >
      <template #[`item.name`]="{ item }">
        <div class="d-flex align-center ga-2">
          <v-icon size="18" :color="item.isAdmin ? 'primary' : undefined">
            {{ item.isAdmin ? 'mdi-shield-crown-outline' : 'mdi-shield-account-outline' }}
          </v-icon>
          <span class="font-weight-medium">{{ item.name }}</span>
        </div>
      </template>
      <template #[`item.rightsCount`]="{ item }">
        <div class="d-flex align-center ga-1 flex-wrap">
          <v-icon
            v-for="r in ROLE_RIGHTS"
            :key="r.key"
            size="16"
            :color="item[r.key] ? 'success' : undefined"
            :class="{ 'right-off': !item[r.key] }"
            :title="$t('admin.rolesManagement.' + r.label) + (item[r.key] ? ' ✓' : ' ✕')"
          >{{ r.icon }}</v-icon>
          <span class="text-body-small text-medium-emphasis ms-1">{{ item.rightsCount }}/{{ ROLE_RIGHTS.length }}</span>
        </div>
      </template>
      <template #[`item.group1Count`]="{ item }">
        <span class="text-body-small" :title="(item.group1List || []).join(', ')">{{ scopeShort(item.group1List) }}</span>
      </template>
      <template #[`item.commandCount`]="{ item }">
        <span class="text-body-small" :title="(item.group1CommandList || []).join(', ')">{{ scopeShort(item.group1CommandList) }}</span>
      </template>
      <template #[`item.displayCount`]="{ item }">
        <span class="text-body-small" :title="(item.displayList || []).join(', ')">{{ scopeShort(item.displayList) }}</span>
      </template>
      <template #[`item.memberCount`]="{ item }">
        <v-chip
          size="x-small"
          label
          variant="tonal"
          :color="item.memberCount ? 'primary' : undefined"
          :title="(membersOf[item.name] || []).join(', ')"
          @click="emit('show-users', item.name)"
        >
          <v-icon start size="14">mdi-account-multiple-outline</v-icon>{{ item.memberCount }}
        </v-chip>
      </template>
      <template #[`item.actions`]="{ item }">
        <div class="admin-row-actions">
          <v-btn icon variant="text" size="x-small" :title="$t('admin.rolesManagement.editRole')" @click="openEditRoleDialog(item)">
            <v-icon size="18">mdi-pencil-outline</v-icon>
          </v-btn>
          <v-btn icon variant="text" size="x-small" :title="$t('admin.common.clone')" @click="openCloneRoleDialog(item)">
            <v-icon size="18">mdi-content-duplicate</v-icon>
          </v-btn>
          <v-btn
            icon
            variant="text"
            size="x-small"
            color="error"
            :disabled="item.name === 'admin' || item.memberCount > 0"
            :title="deleteTitle(item)"
            @click="openDeleteRoleDialog(item)"
          >
            <v-icon size="18">mdi-delete-outline</v-icon>
          </v-btn>
        </div>
      </template>
    </v-data-table>

    <!-- rights matrix: a click toggles the right of that role -->
    <div v-else class="admin-table matrix-wrap">
      <table class="matrix">
        <thead>
          <tr>
            <th class="matrix-right">{{ $t('admin.rolesManagement.headers.rights') }}</th>
            <th v-for="r in filteredRoles" :key="r._id" class="matrix-role">
              <span :title="r.name">{{ r.name }}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="right in ROLE_RIGHTS" :key="right.key">
            <td class="matrix-right">
              <v-icon size="16" class="me-2">{{ right.icon }}</v-icon>
              <span :title="$t('admin.rolesManagement.' + right.label + 'Hint')">{{ $t('admin.rolesManagement.' + right.label) }}</span>
            </td>
            <td v-for="r in filteredRoles" :key="r._id" class="matrix-cell">
              <v-btn
                icon
                variant="text"
                size="small"
                :loading="togglingCell === r._id + right.key"
                :disabled="r.name === 'admin' && ['isAdmin', 'changePassword'].includes(right.key)"
                :title="r.name + ' · ' + $t('admin.rolesManagement.' + right.label)"
                @click="toggleRight(r, right.key)"
              >
                <v-icon :color="r[right.key] ? 'success' : undefined" :class="{ 'right-off': !r[right.key] }">
                  {{ r[right.key] ? 'mdi-check-circle' : 'mdi-circle-outline' }}
                </v-icon>
              </v-btn>
            </td>
          </tr>
          <tr class="matrix-scope">
            <td class="matrix-right">{{ $t('admin.rolesManagement.headers.group1') }}</td>
            <td v-for="r in filteredRoles" :key="r._id" class="matrix-cell text-body-small">{{ scopeShort(r.group1List) }}</td>
          </tr>
          <tr class="matrix-scope">
            <td class="matrix-right">{{ $t('admin.rolesManagement.headers.displays') }}</td>
            <td v-for="r in filteredRoles" :key="r._id" class="matrix-cell text-body-small">{{ scopeShort(r.displayList) }}</td>
          </tr>
          <tr class="matrix-scope">
            <td class="matrix-right">{{ $t('admin.rolesManagement.headers.session') }}</td>
            <td v-for="r in filteredRoles" :key="r._id" class="matrix-cell text-body-small">{{ r.maxSessionDays }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- add / edit role -->
    <v-dialog v-model="roleDialog" scrollable max-width="760px">
      <v-card>
        <v-card-title class="d-flex align-center ga-2">
          <v-icon>{{ isNewRole ? 'mdi-shield-plus-outline' : 'mdi-shield-edit-outline' }}</v-icon>
          {{ isNewRole ? $t('admin.rolesManagement.addRole') : $t('admin.rolesManagement.editRole') }}
        </v-card-title>
        <v-divider></v-divider>
        <v-card-text>
          <v-form ref="roleForm" @submit.prevent="saveRole">
            <div class="admin-dialog-section">{{ $t('admin.rolesManagement.sectionGeneral') }}</div>
            <div class="role-general">
              <v-text-field
                v-model="editedRole.name"
                :label="$t('admin.rolesManagement.roleName')"
                :rules="[rules.required]"
                :disabled="!isNewRole && originalName === 'admin'"
                prepend-inner-icon="mdi-shield-outline"
                variant="outlined"
                density="comfortable"
              ></v-text-field>
              <v-text-field
                v-model.number="editedRole.maxSessionDays"
                type="number"
                min="0"
                max="9999"
                :label="$t('admin.rolesManagement.maxSessionDays')"
                :hint="$t('admin.rolesManagement.maxSessionDaysHint')"
                persistent-hint
                prepend-inner-icon="mdi-timer-outline"
                variant="outlined"
                density="comfortable"
              ></v-text-field>
            </div>

            <div class="admin-dialog-section mt-4">{{ $t('admin.rolesManagement.sectionPermissions') }}</div>
            <div class="role-rights">
              <label
                v-for="r in ROLE_RIGHTS"
                :key="r.key"
                class="role-right"
                :class="{ 'role-right-on': editedRole[r.key], 'role-right-locked': rightLocked(r.key) }"
              >
                <v-checkbox-btn
                  v-model="editedRole[r.key]"
                  color="primary"
                  density="compact"
                  :disabled="rightLocked(r.key)"
                ></v-checkbox-btn>
                <v-icon size="18" class="mx-1">{{ r.icon }}</v-icon>
                <span>
                  <span class="d-block text-body-medium font-weight-medium">{{ $t('admin.rolesManagement.' + r.label) }}</span>
                  <span class="d-block text-body-small text-medium-emphasis">{{ $t('admin.rolesManagement.' + r.label + 'Hint') }}</span>
                </span>
              </label>
            </div>

            <div class="admin-dialog-section mt-4">{{ $t('admin.rolesManagement.sectionScope') }}</div>
            <v-combobox
              v-model="editedRole.group1List"
              :items="group1ListAll"
              :label="$t('admin.rolesManagement.canViewGroup1List')"
              :hint="$t('admin.rolesManagement.scopeEmptyHint')"
              persistent-hint
              prepend-inner-icon="mdi-eye-outline"
              variant="outlined"
              density="comfortable"
              chips
              closable-chips
              multiple
              class="mb-2"
            ></v-combobox>
            <v-combobox
              v-model="editedRole.group1CommandList"
              :items="['$$', ...group1ListAll]"
              :label="$t('admin.rolesManagement.canCommandGroup1List')"
              :hint="$t('admin.rolesManagement.scopeCommandHint')"
              persistent-hint
              prepend-inner-icon="mdi-gesture-tap-button"
              variant="outlined"
              density="comfortable"
              chips
              closable-chips
              multiple
              class="mb-2"
            ></v-combobox>
            <v-combobox
              v-model="editedRole.displayList"
              :items="displayListAll"
              :label="$t('admin.rolesManagement.canAccessDisplayList')"
              :hint="$t('admin.rolesManagement.scopeEmptyHint')"
              persistent-hint
              prepend-inner-icon="mdi-monitor-dashboard"
              variant="outlined"
              density="comfortable"
              chips
              closable-chips
              multiple
            ></v-combobox>
          </v-form>
          <v-alert v-if="dialogError" type="error" variant="tonal" density="compact" class="mt-4">{{ dialogError }}</v-alert>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="roleDialog = false">{{ $t('common.cancel') }}</v-btn>
          <v-btn color="primary" variant="flat" :loading="saving" @click="saveRole">{{ $t('common.save') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- delete role -->
    <v-dialog v-model="deleteRoleDialog" max-width="420px">
      <v-card>
        <v-card-title class="d-flex align-center ga-2">
          <v-icon color="error">mdi-shield-remove-outline</v-icon>
          {{ $t('admin.rolesManagement.deleteRole') }}
        </v-card-title>
        <v-card-text>
          {{ $t('admin.rolesManagement.deleteRoleConfirm') }}
          <div class="mt-2 font-weight-medium">{{ roleToDelete.name }}</div>
          <v-alert v-if="dialogError" type="error" variant="tonal" density="compact" class="mt-4">{{ dialogError }}</v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="deleteRoleDialog = false">{{ $t('common.cancel') }}</v-btn>
          <v-btn color="error" variant="flat" :loading="saving" @click="deleteRole(roleToDelete)">{{ $t('common.delete') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-snackbar v-model="snackbar.show" :color="snackbar.color" :timeout="3500" location="bottom right">
      {{ snackbar.text }}
    </v-snackbar>
  </div>
</template>

<script setup>
  import { ref, computed, watch, onMounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import AdminSummary from './AdminSummary.vue'
  import { postJson, toCsv, downloadText, fileStamp, loadPref, savePref } from '../lib/adminList'
  import { ROLE_RIGHTS } from '../lib/roleRights'

  const emit = defineEmits(['show-users'])
  const { t } = useI18n()

  const roles = ref([])
  const users = ref([])
  const loading = ref(false)
  const saving = ref(false)
  const search = ref('')
  const tileFilter = ref(null)
  const view = ref(loadPref('roles-view', 'list'))
  watch(view, (v) => savePref('roles-view', v))
  const itemsPerPage = ref(loadPref('roles-page-size', 25))
  watch(itemsPerPage, (n) => savePref('roles-page-size', n))
  const displayListAll = ref([])
  const group1ListAll = ref([])

  const headers = computed(() => [
    { title: t('admin.rolesManagement.headers.name'), key: 'name' },
    { title: t('admin.rolesManagement.headers.rights'), key: 'rightsCount' },
    { title: t('admin.rolesManagement.headers.group1'), key: 'group1Count', align: 'end' },
    { title: t('admin.rolesManagement.headers.commands'), key: 'commandCount', align: 'end' },
    { title: t('admin.rolesManagement.headers.displays'), key: 'displayCount', align: 'end' },
    { title: t('admin.rolesManagement.headers.session'), key: 'maxSessionDays', align: 'end' },
    { title: t('admin.rolesManagement.headers.users'), key: 'memberCount', align: 'end' },
    { title: '', key: 'actions', sortable: false, align: 'end', width: 120 },
  ])

  // users of each role (by name)
  const membersOf = computed(() => {
    const m = {}
    for (const u of users.value)
      for (const r of u.roles || []) {
        const name = r?.name
        if (name) (m[name] = m[name] || []).push(u.username)
      }
    return m
  })

  const rows = computed(() =>
    roles.value.map((r) => ({
      ...r,
      rightsCount: ROLE_RIGHTS.filter((x) => r[x.key]).length,
      group1Count: (r.group1List || []).length,
      commandCount: (r.group1CommandList || []).length,
      displayCount: (r.displayList || []).length,
      memberCount: (membersOf.value[r.name] || []).length,
    }))
  )
  const filteredRoles = computed(() => {
    const q = (search.value || '').trim().toLowerCase()
    return rows.value
      .filter(
        (r) =>
          (!q || r.name.toLowerCase().includes(q)) &&
          (tileFilter.value === null ||
            (tileFilter.value === 'admin' && r.isAdmin) ||
            (tileFilter.value === 'unused' && !r.memberCount))
      )
      .sort((a, b) => a.name.localeCompare(b.name))
  })

  const summaryTiles = computed(() => [
    { key: 'total', icon: 'mdi-shield-account-outline', color: 'primary', value: roles.value.length, label: t('admin.rolesManagement.summary.total') },
    { key: 'admin', filter: 'admin', icon: 'mdi-shield-crown-outline', color: 'primary', value: rows.value.filter((r) => r.isAdmin).length, label: t('admin.rolesManagement.summary.admin') },
    { key: 'unused', filter: 'unused', icon: 'mdi-shield-off-outline', color: 'warning', value: rows.value.filter((r) => !r.memberCount).length, label: t('admin.rolesManagement.summary.unused') },
  ])

  const scopeShort = (list) =>
    list && list.length
      ? list.length <= 2
        ? list.join(', ')
        : t('admin.rolesManagement.scopeCount', { n: list.length })
      : t('admin.rolesManagement.scopeAll')

  function deleteTitle(item) {
    if (item.name === 'admin') return t('admin.rolesManagement.protectedRole')
    if (item.memberCount > 0) return t('admin.rolesManagement.inUse')
    return t('admin.rolesManagement.deleteRole')
  }

  const snackbar = ref({ show: false, text: '', color: undefined })
  function notify(text, color) {
    snackbar.value = { show: true, text, color }
  }

  onMounted(async () => {
    await refreshAll()
    fetchDisplayList()
    fetchGroup1List()
  })

  async function refreshAll() {
    await Promise.all([fetchRoles(), fetchUsers()])
  }

  async function fetchRoles() {
    loading.value = true
    try {
      const json = await (await fetch('/Invoke/auth/listRoles')).json()
      if (json.error || !Array.isArray(json)) {
        notify(t('common.error') + ' ' + (json.error || ''), 'error')
        return
      }
      roles.value = json
    } catch (err) {
      notify(t('common.error') + ' ' + err.message, 'error')
    } finally {
      loading.value = false
    }
  }

  async function fetchUsers() {
    try {
      const json = await (await fetch('/Invoke/auth/listUsers')).json()
      if (Array.isArray(json)) users.value = json
    } catch (err) {
      console.warn(err)
    }
  }

  async function fetchDisplayList() {
    try {
      const json = await (await fetch('/Invoke/auth/listDisplays')).json()
      if (Array.isArray(json)) displayListAll.value = json
    } catch (err) {
      console.warn(err)
    }
  }

  async function fetchGroup1List() {
    try {
      const json = await (await fetch('/Invoke/auth/listGroup1')).json()
      if (Array.isArray(json)) group1ListAll.value = json.filter((g) => typeof g === 'string' && g !== '').sort()
    } catch (err) {
      console.warn(err)
    }
  }

  // ---- matrix: one right of one role ----
  const togglingCell = ref('')
  async function toggleRight(role, key) {
    togglingCell.value = role._id + key
    try {
      const json = await postJson('/Invoke/auth/updateRole', { _id: role._id, [key]: !role[key] })
      if (json.error) throw new Error(json.error)
      await fetchRoles()
    } catch (err) {
      notify(t('common.error') + ' ' + err.message, 'error')
    } finally {
      togglingCell.value = ''
    }
  }

  // ---- add / edit dialog ----
  const roleDialog = ref(false)
  const deleteRoleDialog = ref(false)
  const isNewRole = ref(false)
  const dialogError = ref('')
  const roleForm = ref(null)
  const originalName = ref('')
  const editedRole = ref({})
  const roleToDelete = ref({})
  const rules = {
    required: (v) => !!(v && String(v).trim()) || t('admin.common.required'),
  }
  // the admin role keeps admin rights
  const rightLocked = (key) =>
    !isNewRole.value && originalName.value === 'admin' && ['isAdmin', 'changePassword'].includes(key)

  const defaultRoleConfig = {
    name: '',
    group1List: [],
    group1CommandList: [],
    displayList: [],
    isAdmin: false,
    changePassword: true,
    sendCommands: true,
    enterAnnotations: true,
    enterNotes: true,
    enterManuals: true,
    enterLimits: true,
    substituteValues: true,
    ackEvents: true,
    ackAlarms: true,
    disableAlarms: true,
    cimEngineering: false,
    maxSessionDays: 3,
  }
  const roleFields = Object.keys(defaultRoleConfig)
  const copyOf = (r) => {
    const out = {}
    for (const k of roleFields) out[k] = Array.isArray(r[k]) ? [...r[k]] : r[k] ?? defaultRoleConfig[k]
    return out
  }

  function openDialog(role, isNew) {
    isNewRole.value = isNew
    dialogError.value = ''
    fetchDisplayList()
    fetchGroup1List()
    roleDialog.value = true
    editedRole.value = role
  }
  function openAddRoleDialog() {
    originalName.value = ''
    openDialog(copyOf(defaultRoleConfig), true)
  }
  function openEditRoleDialog(item) {
    originalName.value = item.name
    openDialog({ ...copyOf(item), _id: item._id }, false)
  }
  function openCloneRoleDialog(item) {
    originalName.value = ''
    openDialog({ ...copyOf(item), name: item.name + '_copy' }, true)
  }

  async function saveRole() {
    const { valid } = (await roleForm.value?.validate()) || { valid: true }
    if (!valid) return
    saving.value = true
    dialogError.value = ''
    const body = copyOf(editedRole.value)
    body.name = String(body.name).trim()
    body.maxSessionDays = Number(body.maxSessionDays) || 0
    try {
      const json = isNewRole.value
        ? await postJson('/Invoke/auth/createRole', body)
        : await postJson('/Invoke/auth/updateRole', { ...body, _id: editedRole.value._id })
      if (json.error) throw new Error(json.error)
      roleDialog.value = false
      notify(t('admin.common.saved'), 'success')
    } catch (err) {
      dialogError.value = err.message
    } finally {
      saving.value = false
      refreshAll()
    }
  }

  function openDeleteRoleDialog(role) {
    dialogError.value = ''
    roleToDelete.value = role
    deleteRoleDialog.value = true
  }

  async function deleteRole(role) {
    if (role.name === 'admin') return
    saving.value = true
    dialogError.value = ''
    try {
      const json = await postJson('/Invoke/auth/deleteRole', { name: role.name, _id: role._id })
      if (json.error) throw new Error(json.error)
      deleteRoleDialog.value = false
    } catch (err) {
      dialogError.value = err.message
    } finally {
      saving.value = false
      refreshAll()
    }
  }

  function exportCsv() {
    const cols = [
      { title: 'name', value: (r) => r.name },
      ...ROLE_RIGHTS.map((x) => ({ title: x.key, value: (r) => !!r[x.key] })),
      { title: 'group1List', value: (r) => (r.group1List || []).join(' ') },
      { title: 'group1CommandList', value: (r) => (r.group1CommandList || []).join(' ') },
      { title: 'displayList', value: (r) => (r.displayList || []).join(' ') },
      { title: 'maxSessionDays', value: (r) => r.maxSessionDays },
      { title: 'users', value: (r) => (membersOf.value[r.name] || []).join(' ') },
    ]
    downloadText('roles_' + fileStamp() + '.csv', toCsv(filteredRoles.value, cols))
  }

  defineExpose({ fetchRoles: refreshAll })
</script>

<style scoped>
  .right-off {
    opacity: 0.35;
  }
  .role-general {
    display: grid;
    grid-template-columns: 2fr 1fr;
    gap: 12px;
  }
  .role-rights {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 6px;
  }
  .role-right {
    display: flex;
    align-items: center;
    padding: 6px 8px;
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 8px;
    cursor: pointer;
  }
  .role-right-on {
    border-color: rgba(var(--v-theme-primary), 0.6);
    background: rgba(var(--v-theme-primary), 0.06);
  }
  .role-right-locked {
    cursor: default;
    opacity: 0.7;
  }
  .matrix-wrap {
    overflow: auto;
  }
  .matrix {
    border-collapse: collapse;
    width: 100%;
    font-size: 0.85rem;
  }
  .matrix th,
  .matrix td {
    border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    padding: 2px 8px;
  }
  .matrix thead th {
    font-weight: 600;
    height: 40px;
    position: sticky;
    top: 0;
    background: rgb(var(--v-theme-surface));
  }
  .matrix-right {
    text-align: start;
    white-space: nowrap;
    position: sticky;
    left: 0;
    background: rgb(var(--v-theme-surface));
  }
  .matrix-role {
    text-align: center;
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .matrix-cell {
    text-align: center;
  }
  .matrix tbody tr:hover td {
    background: rgba(var(--v-theme-on-surface), 0.04);
  }
  .matrix-scope td {
    opacity: 0.85;
  }
  @media (max-width: 600px) {
    .role-general {
      grid-template-columns: 1fr;
    }
  }
</style>
