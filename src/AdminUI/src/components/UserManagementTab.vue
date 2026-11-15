<template>
  <div class="users-tab">
    <AdminSummary v-model="tileFilter" :tiles="summaryTiles" />

    <div class="admin-toolbar">
      <v-btn color="primary" variant="flat" prepend-icon="mdi-account-plus-outline" @click="openAddUserDialog">
        {{ $t('admin.userManagement.addUser') }}
      </v-btn>
      <v-text-field
        v-model="search"
        class="admin-search"
        density="compact"
        variant="outlined"
        hide-details
        clearable
        prepend-inner-icon="mdi-magnify"
        :label="$t('admin.userManagement.search')"
      ></v-text-field>
      <v-select
        v-model="filterRoles"
        class="admin-search"
        :items="roles"
        item-title="name"
        item-value="name"
        :label="$t('admin.userManagement.filterRole')"
        multiple
        chips
        closable-chips
        clearable
        density="compact"
        variant="outlined"
        hide-details
      ></v-select>
      <v-spacer></v-spacer>
      <span class="admin-count">{{ $t('admin.common.results', { n: filteredUsers.length }) }}</span>
      <v-btn icon variant="text" size="small" :title="$t('common.refresh')" :loading="loading" @click="refreshAll">
        <v-icon>mdi-refresh</v-icon>
      </v-btn>
      <v-btn icon variant="text" size="small" :title="$t('admin.common.exportCsv')" @click="exportCsv">
        <v-icon>mdi-file-delimited-outline</v-icon>
      </v-btn>
    </div>

    <v-data-table
      v-model:expanded="expanded"
      v-model:items-per-page="itemsPerPage"
      :headers="headers"
      :items="filteredUsers"
      :items-per-page-options="[10, 25, 50, 100, -1]"
      :items-per-page-text="$t('common.itemsPerPageText')"
      :sort-by="[{ key: 'username', order: 'asc' }]"
      :loading="loading"
      item-value="_id"
      show-expand
      hover
      density="compact"
      class="admin-table"
    >
      <template #[`item.username`]="{ item }">
        <div class="d-flex align-center ga-3 py-1">
          <v-avatar size="30" :color="item.isAdminUser ? 'primary' : 'secondary'" variant="tonal">
            <span class="text-body-small font-weight-bold">{{ initials(item.username) }}</span>
          </v-avatar>
          <div>
            <div class="font-weight-medium">{{ item.username }}</div>
            <div v-if="item.isAdminUser" class="text-body-small text-primary">{{ $t('admin.rolesManagement.isAdmin') }}</div>
          </div>
        </div>
      </template>
      <template #[`item.email`]="{ item }">
        <a v-if="item.email" :href="'mailto:' + item.email" class="text-body-small user-mail">{{ item.email }}</a>
      </template>
      <template #[`item.rolesText`]="{ item }">
        <div class="d-flex flex-wrap ga-1">
          <v-chip
            v-for="r in item.roleNames"
            :key="r"
            size="x-small"
            label
            variant="tonal"
            :color="adminRoleNames.has(r) ? 'primary' : undefined"
          >{{ r }}</v-chip>
          <span v-if="!item.roleNames.length" class="text-body-small text-warning">{{ $t('admin.userManagement.noRoles') }}</span>
        </div>
      </template>
      <template #[`item.isLDAPUser`]="{ item }">
        <v-chip size="x-small" label variant="outlined" :color="item.isLDAPUser ? 'info' : undefined">
          {{ item.isLDAPUser ? 'LDAP' : $t('admin.userManagement.local') }}
        </v-chip>
        <div v-if="item.isLDAPUser && item.lastLDAPSync" class="text-body-small text-medium-emphasis">
          {{ new Date(item.lastLDAPSync).toLocaleString() }}
        </div>
      </template>
      <template #[`item.actions`]="{ item }">
        <div class="admin-row-actions">
          <v-btn icon variant="text" size="x-small" :title="$t('admin.userManagement.editUser')" @click="openEditUserDialog(item)">
            <v-icon size="18">mdi-pencil-outline</v-icon>
          </v-btn>
          <v-btn
            icon
            variant="text"
            size="x-small"
            color="error"
            :disabled="item.username === 'admin'"
            :title="item.username === 'admin' ? $t('admin.userManagement.protectedUser') : $t('admin.userManagement.deleteUser')"
            @click="openDeleteConfirmDialog(item)"
          >
            <v-icon size="18">mdi-delete-outline</v-icon>
          </v-btn>
        </div>
      </template>
      <template #expanded-row="{ columns, item }">
        <tr>
          <td :colspan="columns.length" class="admin-details">
            <div class="admin-details-grid">
              <div>
                <div class="text-label-medium text-uppercase">{{ $t('admin.userManagement.effectiveRights') }}</div>
                <div class="d-flex flex-wrap ga-1">
                  <v-chip
                    v-for="r in ROLE_RIGHTS"
                    :key="r.key"
                    size="small"
                    label
                    :variant="item.rights[r.key] ? 'tonal' : 'outlined'"
                    :color="item.rights[r.key] ? 'success' : undefined"
                    :class="{ 'right-off': !item.rights[r.key] }"
                    :prepend-icon="r.icon"
                  >{{ $t('admin.rolesManagement.' + r.label) }}</v-chip>
                </div>
              </div>
              <dl class="admin-kv">
                <dt>{{ $t('admin.rolesManagement.headers.group1') }}</dt>
                <dd>{{ scopeText(item.rights.group1List) }}</dd>
                <dt>{{ $t('admin.rolesManagement.headers.commands') }}</dt>
                <dd>{{ scopeText(item.rights.group1CommandList) }}</dd>
                <dt>{{ $t('admin.rolesManagement.headers.displays') }}</dt>
                <dd>{{ scopeText(item.rights.displayList) }}</dd>
                <dt>{{ $t('admin.rolesManagement.maxSessionDays') }}</dt>
                <dd>{{ item.rights.maxSessionDays }}</dd>
                <template v-if="item.isLDAPUser">
                  <dt>{{ $t('admin.userManagement.ldapDN') }}</dt>
                  <dd class="admin-mono">{{ item.ldapDN || '—' }}</dd>
                </template>
              </dl>
            </div>
          </td>
        </tr>
      </template>
    </v-data-table>

    <!-- add / edit user -->
    <v-dialog v-model="userDialog" scrollable max-width="560px">
      <v-card>
        <v-card-title class="d-flex align-center ga-2">
          <v-icon>{{ isNewUser ? 'mdi-account-plus-outline' : 'mdi-account-edit-outline' }}</v-icon>
          {{ isNewUser ? $t('admin.userManagement.addNewUser') : $t('admin.userManagement.editUser') }}
        </v-card-title>
        <v-divider></v-divider>
        <v-card-text>
          <v-form ref="userForm" @submit.prevent="saveUser">
            <div class="admin-dialog-section">{{ $t('admin.userManagement.sectionAccount') }}</div>
            <v-text-field
              v-model="editedUser.username"
              :label="$t('admin.userManagement.username')"
              :rules="[rules.required]"
              :disabled="!isNewUser && originalUsername === 'admin'"
              prepend-inner-icon="mdi-account-outline"
              variant="outlined"
              density="comfortable"
              autofocus
            ></v-text-field>
            <v-text-field
              v-model="editedUser.email"
              :label="$t('admin.userManagement.email')"
              :rules="[rules.email]"
              prepend-inner-icon="mdi-email-outline"
              variant="outlined"
              density="comfortable"
              type="email"
            ></v-text-field>
            <v-text-field
              v-if="!editedUser.isLDAPUser"
              v-model="editedUser.password"
              :label="$t('admin.userManagement.password')"
              :rules="[rules.password]"
              :hint="isNewUser ? $t('admin.userManagement.passwordMin', { n: MIN_PASSWORD_LENGTH }) : $t('admin.userManagement.passwordKeep')"
              persistent-hint
              prepend-inner-icon="mdi-lock-outline"
              :append-inner-icon="showPassword ? 'mdi-eye-off-outline' : 'mdi-eye-outline'"
              :type="showPassword ? 'text' : 'password'"
              autocomplete="new-password"
              variant="outlined"
              density="comfortable"
              class="mb-2"
              @click:append-inner="showPassword = !showPassword"
            ></v-text-field>

            <div class="admin-dialog-section mt-4">{{ $t('admin.userManagement.roles') }}</div>
            <v-autocomplete
              v-model="editedUser.roleNames"
              :items="roles"
              item-title="name"
              item-value="name"
              :label="$t('admin.userManagement.roles')"
              :hint="$t('admin.userManagement.rolesHint')"
              persistent-hint
              prepend-inner-icon="mdi-shield-account-outline"
              variant="outlined"
              density="comfortable"
              chips
              closable-chips
              multiple
            ></v-autocomplete>

            <template v-if="editedUser.username !== 'admin'">
              <v-switch
                v-model="editedUser.isLDAPUser"
                color="primary"
                inset
                hide-details
                class="mt-2"
                :label="$t('admin.userManagement.ldapUser')"
              ></v-switch>
              <dl v-if="editedUser.isLDAPUser && !isNewUser" class="admin-kv mt-1">
                <dt>{{ $t('admin.userManagement.ldapDN') }}</dt>
                <dd class="admin-mono">{{ editedUser.ldapDN || '—' }}</dd>
                <dt>{{ $t('admin.userManagement.lastLDAPSync') }}</dt>
                <dd>{{ editedUser.lastLDAPSync ? new Date(editedUser.lastLDAPSync).toLocaleString() : '—' }}</dd>
              </dl>
            </template>
          </v-form>
          <v-alert v-if="dialogError" type="error" variant="tonal" density="compact" class="mt-4">{{ dialogError }}</v-alert>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="userDialog = false">{{ $t('common.cancel') }}</v-btn>
          <v-btn color="primary" variant="flat" :loading="saving" @click="saveUser">{{ $t('common.save') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- delete user -->
    <v-dialog v-model="deleteConfirmDialog" max-width="420px">
      <v-card>
        <v-card-title class="d-flex align-center ga-2">
          <v-icon color="error">mdi-account-remove-outline</v-icon>
          {{ $t('admin.userManagement.deleteUser') }}
        </v-card-title>
        <v-card-text>
          {{ $t('admin.userManagement.deleteConfirmMessage') }}
          <div class="mt-2 font-weight-medium">{{ userToDelete.username }}</div>
          <v-alert v-if="dialogError" type="error" variant="tonal" density="compact" class="mt-4">{{ dialogError }}</v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="deleteConfirmDialog = false">{{ $t('common.cancel') }}</v-btn>
          <v-btn color="error" variant="flat" :loading="saving" @click="deleteUser(userToDelete)">{{ $t('common.delete') }}</v-btn>
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
  import { ROLE_RIGHTS, MIN_PASSWORD_LENGTH, effectiveRights, initials } from '../lib/roleRights'

  const props = defineProps({
    // { name } from the Roles section: list the users of that role
    roleFilter: { type: Object, default: null },
  })
  const { t } = useI18n()

  const users = ref([])
  const roles = ref([])
  const loading = ref(false)
  const saving = ref(false)
  const expanded = ref([])
  const search = ref('')
  const filterRoles = ref([])
  const tileFilter = ref(null)
  watch(
    () => props.roleFilter,
    (f) => {
      if (!f) return
      filterRoles.value = [f.name]
      tileFilter.value = null
      search.value = ''
    },
    { immediate: true }
  )
  const itemsPerPage = ref(loadPref('users-page-size', 25))
  watch(itemsPerPage, (n) => savePref('users-page-size', n))

  const headers = computed(() => [
    { title: t('admin.userManagement.headers.username'), key: 'username' },
    { title: t('admin.userManagement.headers.email'), key: 'email' },
    { title: t('admin.userManagement.headers.roles'), key: 'rolesText' },
    { title: t('admin.userManagement.authentication'), key: 'isLDAPUser', width: 170 },
    { title: '', key: 'actions', sortable: false, align: 'end', width: 90 },
    { title: '', key: 'data-table-expand', width: 40 },
  ])

  const adminRoleNames = computed(() => new Set(roles.value.filter((r) => r.isAdmin).map((r) => r.name)))

  const filteredUsers = computed(() => {
    const q = (search.value || '').trim().toLowerCase()
    return users.value.filter(
      (u) =>
        (!q || u.username.toLowerCase().includes(q) || String(u.email || '').toLowerCase().includes(q)) &&
        (!filterRoles.value.length || u.roleNames.some((r) => filterRoles.value.includes(r))) &&
        (tileFilter.value === null ||
          (tileFilter.value === 'admin' && u.isAdminUser) ||
          (tileFilter.value === 'ldap' && u.isLDAPUser) ||
          (tileFilter.value === 'noRoles' && !u.roleNames.length))
    )
  })

  const summaryTiles = computed(() => [
    { key: 'total', icon: 'mdi-account-multiple-outline', color: 'primary', value: users.value.length, label: t('admin.userManagement.summary.total') },
    { key: 'admin', filter: 'admin', icon: 'mdi-shield-crown-outline', color: 'primary', value: users.value.filter((u) => u.isAdminUser).length, label: t('admin.userManagement.summary.admins') },
    { key: 'ldap', filter: 'ldap', icon: 'mdi-server-network', color: 'info', value: users.value.filter((u) => u.isLDAPUser).length, label: t('admin.userManagement.summary.ldap') },
    { key: 'noRoles', filter: 'noRoles', icon: 'mdi-account-alert-outline', color: 'warning', value: users.value.filter((u) => !u.roleNames.length).length, label: t('admin.userManagement.summary.noRoles') },
  ])

  const scopeText = (list) => (list && list.length ? list.join(', ') : t('admin.rolesManagement.scopeAll'))

  const snackbar = ref({ show: false, text: '', color: undefined })
  function notify(text, color) {
    snackbar.value = { show: true, text, color }
  }

  onMounted(refreshAll)

  async function refreshAll() {
    await fetchRoles()
    await fetchUsers()
  }

  async function fetchUsers() {
    loading.value = true
    try {
      const json = await (await fetch('/Invoke/auth/listUsers')).json()
      if (json.error || !Array.isArray(json)) {
        notify(t('common.error') + ' ' + (json.error || ''), 'error')
        return
      }
      users.value = json.map((u) => {
        const userRoles = (u.roles || []).filter((r) => r && typeof r === 'object')
        const rights = effectiveRights(userRoles)
        return {
          ...u,
          roleNames: userRoles.map((r) => r.name),
          rolesText: userRoles.map((r) => r.name).join(', '),
          rights,
          isAdminUser: rights.isAdmin,
        }
      })
    } catch (err) {
      notify(t('common.error') + ' ' + err.message, 'error')
    } finally {
      loading.value = false
    }
  }

  async function fetchRoles() {
    try {
      const json = await (await fetch('/Invoke/auth/listRoles')).json()
      if (Array.isArray(json)) roles.value = json
    } catch (err) {
      console.warn(err)
    }
  }

  // ---- add / edit dialog ----
  const userDialog = ref(false)
  const deleteConfirmDialog = ref(false)
  const isNewUser = ref(false)
  const showPassword = ref(false)
  const dialogError = ref('')
  const userForm = ref(null)
  const originalUsername = ref('')
  const originalRoles = ref([])
  const editedUser = ref({})
  const userToDelete = ref({})

  const rules = {
    required: (v) => !!(v && String(v).trim()) || t('admin.common.required'),
    email: (v) => !v || /^[^\s@]+@[^\s@]+$/.test(v) || t('admin.userManagement.invalidEmail'),
    password: (v) =>
      (!isNewUser.value && !v) ||
      (typeof v === 'string' && v.length >= MIN_PASSWORD_LENGTH) ||
      t('admin.userManagement.passwordMin', { n: MIN_PASSWORD_LENGTH }),
  }

  async function openAddUserDialog() {
    await fetchRoles()
    isNewUser.value = true
    dialogError.value = ''
    showPassword.value = false
    originalUsername.value = ''
    originalRoles.value = []
    editedUser.value = { username: '', email: '', password: '', isLDAPUser: false, roleNames: [] }
    userDialog.value = true
  }

  async function openEditUserDialog(item) {
    await fetchRoles()
    isNewUser.value = false
    dialogError.value = ''
    showPassword.value = false
    originalUsername.value = item.username
    originalRoles.value = [...item.roleNames]
    editedUser.value = {
      _id: item._id,
      username: item.username,
      email: item.email || '',
      password: '',
      isLDAPUser: !!item.isLDAPUser,
      ldapDN: item.ldapDN,
      lastLDAPSync: item.lastLDAPSync,
      roleNames: [...item.roleNames],
    }
    userDialog.value = true
  }

  async function saveUser() {
    const { valid } = (await userForm.value?.validate()) || { valid: true }
    if (!valid) return
    const u = editedUser.value
    // LDAP users authenticate on the directory: no local password
    const password = u.isLDAPUser ? '' : u.password
    saving.value = true
    dialogError.value = ''
    try {
      if (isNewUser.value) {
        const roleIds = roles.value.filter((r) => u.roleNames.includes(r.name)).map((r) => r._id)
        const body = { username: u.username.trim(), email: u.email, isLDAPUser: u.isLDAPUser, roles: roleIds }
        if (password) body.password = password
        const json = await postJson('/Invoke/auth/createUser', body)
        if (json.error) throw new Error(json.error)
      } else {
        const body = { _id: u._id, username: u.username.trim(), email: u.email, isLDAPUser: u.isLDAPUser }
        if (password) body.password = password
        const json = await postJson('/Invoke/auth/updateUser', body)
        if (json.error) throw new Error(json.error)
        // roles change one at a time, by name (the user may have been renamed)
        const added = u.roleNames.filter((r) => !originalRoles.value.includes(r))
        const removed = originalRoles.value.filter((r) => !u.roleNames.includes(r))
        for (const role of added) {
          const r = await postJson('/Invoke/auth/userAddRole', { username: body.username, role })
          if (r.error) throw new Error(r.error)
        }
        for (const role of removed) {
          const r = await postJson('/Invoke/auth/userRemoveRole', { username: body.username, role })
          if (r.error) throw new Error(r.error)
        }
      }
      userDialog.value = false
      notify(t('admin.common.saved'), 'success')
    } catch (err) {
      dialogError.value = err.message
    } finally {
      saving.value = false
      fetchUsers()
    }
  }

  function openDeleteConfirmDialog(user) {
    dialogError.value = ''
    userToDelete.value = user
    deleteConfirmDialog.value = true
  }

  async function deleteUser(user) {
    if (user.username === 'admin') return
    saving.value = true
    dialogError.value = ''
    try {
      const json = await postJson('/Invoke/auth/deleteUser', { username: user.username, _id: user._id })
      if (json.error) throw new Error(json.error)
      deleteConfirmDialog.value = false
    } catch (err) {
      dialogError.value = err.message
    } finally {
      saving.value = false
      fetchUsers()
    }
  }

  function exportCsv() {
    const cols = [
      { title: 'username', value: (u) => u.username },
      { title: 'email', value: (u) => u.email },
      { title: 'roles', value: (u) => u.roleNames.join(' ') },
      { title: 'isLDAPUser', value: (u) => !!u.isLDAPUser },
      { title: 'isAdmin', value: (u) => u.isAdminUser },
      { title: 'lastLDAPSync', value: (u) => u.lastLDAPSync || '' },
    ]
    downloadText('users_' + fileStamp() + '.csv', toCsv(filteredUsers.value, cols))
  }

  defineExpose({ fetchUsers, fetchRoles, refreshAll })
</script>

<style scoped>
  .user-mail {
    color: inherit;
    opacity: 0.85;
  }
  .right-off {
    opacity: 0.45;
  }
</style>
