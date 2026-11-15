<template>
  <div class="admin-root">
    <!-- one row: title, then the section tabs; a tab's tooltip describes its section -->
    <header class="admin-header">
      <div class="admin-title">
        <NavIcon name="admin" :size="20" class="admin-title-icon" />
        <h1 class="admin-h1 text-title-medium">{{ $t('admin.title') }}</h1>
      </div>
      <v-tabs v-model="activeTab" color="primary" density="compact" show-arrows class="admin-tabs">
        <v-tab
          v-for="s in sections"
          :key="s.key"
          :value="s.key"
          class="admin-tab"
          :title="$t('admin.sections.' + s.key)"
        >
          <NavIcon :name="s.icon" :size="18" class="me-2" />
          {{ $t(s.titleKey) }}
        </v-tab>
      </v-tabs>
    </header>
    <v-divider></v-divider>

    <div class="admin-body">
      <v-window v-model="activeTab" :touch="false">
        <v-window-item value="users">
          <user-management-tab ref="usersRef" :role-filter="usersRoleFilter" />
        </v-window-item>
        <v-window-item value="roles">
          <roles-management-tab ref="rolesRef" @show-users="showUsersOfRole" />
        </v-window-item>
        <v-window-item value="drivers">
          <protocol-driver-instances-tab
            ref="driversRef"
            :active="activeTab === 'drivers'"
            @show-connections="showConnectionsOfInstance"
          />
        </v-window-item>
        <v-window-item value="connections">
          <protocol-connections-tab
            ref="connectionsRef"
            :active="activeTab === 'connections'"
            :driver-filter="connectionsDriverFilter"
            @show-tags="showTagsOfConnection"
          />
        </v-window-item>
        <v-window-item value="tags">
          <tags-tab ref="tagsRef" :active="activeTab === 'tags'" :connection-filter="tagsConnectionFilter" />
        </v-window-item>
        <v-window-item value="userActions">
          <user-actions-tab ref="userActionsRef" :active="activeTab === 'userActions'" />
        </v-window-item>
        <v-window-item value="system">
          <system-settings-tab ref="systemRef" />
        </v-window-item>
      </v-window>
    </div>
  </div>
</template>

<script setup>
  import { ref, watch } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  import NavIcon from './NavIcon.vue'
  import UserManagementTab from './UserManagementTab.vue'
  import RolesManagementTab from './RolesManagementTab.vue'
  import ProtocolDriverInstancesTab from './ProtocolDriverInstancesTab.vue'
  import ProtocolConnectionsTab from './ProtocolConnectionsTab.vue'
  import TagsTab from './TagsTab.vue'
  import UserActionsTab from './UserActions.vue'
  import SystemSettingsTab from './SystemSettingsTab.vue'

  // Sections of the Admin page; the active one is kept in the URL (?tab=<key>)
  // so a section can be linked and survives a reload.
  const sections = [
    { key: 'users', icon: 'users', titleKey: 'admin.tabs.userManagement' },
    { key: 'roles', icon: 'roles', titleKey: 'admin.tabs.rolesManagement' },
    { key: 'drivers', icon: 'driverInstance', titleKey: 'admin.tabs.protocolDriverInstances' },
    { key: 'connections', icon: 'connection', titleKey: 'admin.tabs.protocolConnections' },
    { key: 'tags', icon: 'tag', titleKey: 'admin.tabs.tags' },
    { key: 'userActions', icon: 'history', titleKey: 'admin.tabs.userActions' },
    { key: 'system', icon: 'settings', titleKey: 'admin.tabs.systemSettings' },
  ]
  const route = useRoute()
  const router = useRouter()
  const tabOf = (q) => (sections.some((s) => s.key === q) ? q : 'users')
  const activeTab = ref(tabOf(route.query.tab))

  const usersRef = ref(null)
  const rolesRef = ref(null)
  const driversRef = ref(null)
  const connectionsRef = ref(null)
  const tagsRef = ref(null)
  const userActionsRef = ref(null)
  const systemRef = ref(null)
  // cross-section links (Roles → Users, Driver Instances → Connections,
  // Connections → Tags): a new object each time, so asking twice for the same
  // item still re-applies the filter
  const tagsConnectionFilter = ref(null)
  const connectionsDriverFilter = ref(null)
  const usersRoleFilter = ref(null)

  // a section loads its list when first shown (its own onMounted); later
  // visits refresh it
  const refresh = {
    users: () => usersRef.value?.refreshAll?.(),
    roles: () => rolesRef.value?.fetchRoles?.(),
    drivers: () => driversRef.value?.fetchProtocolDriverInstances?.(),
    connections: () => connectionsRef.value?.fetchProtocolConnections?.(),
    tags: () => tagsRef.value?.fetchTags?.(),
    userActions: () => userActionsRef.value?.fetchUserActions?.(),
    system: () => systemRef.value?.refresh?.(),
  }

  watch(activeTab, (tab) => {
    if (route.query.tab !== tab) router.replace({ query: { ...route.query, tab } })
    refresh[tab]?.()
  })
  watch(
    () => route.query.tab,
    (q) => (activeTab.value = tabOf(q))
  )

  function showUsersOfRole(name) {
    usersRoleFilter.value = { name }
    activeTab.value = 'users'
  }
  function showConnectionsOfInstance(f) {
    connectionsDriverFilter.value = { ...f }
    activeTab.value = 'connections'
  }
  function showTagsOfConnection(connectionNumber) {
    tagsConnectionFilter.value = { n: connectionNumber }
    activeTab.value = 'tags'
  }
</script>

<style scoped>
  .admin-root {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
  }
  .admin-header {
    position: sticky;
    top: 0;
    z-index: 5;
    display: flex;
    align-items: center;
    gap: 16px;
    background: rgb(var(--v-theme-surface));
    padding: 0 12px 0 20px;
  }
  .admin-title {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 0 0 auto;
    white-space: nowrap;
  }
  .admin-h1 {
    margin: 0;
    line-height: 1;
  }
  .admin-title-icon {
    color: rgb(var(--v-theme-primary));
  }
  .admin-tabs {
    flex: 1 1 auto;
    min-width: 0;
  }
  .admin-tab {
    text-transform: none;
    letter-spacing: normal;
  }
  .admin-body {
    flex: 1 1 auto;
    padding: 16px 20px 24px;
  }
  @media (max-width: 600px) {
    .admin-header {
      padding: 0 4px 0 12px;
      gap: 8px;
    }
    .admin-h1 {
      display: none;
    }
    .admin-body {
      padding: 12px 8px 16px;
    }
  }
</style>

<style>
  /* shared by the Admin section lists (TagsTab, ProtocolConnectionsTab, UserActions) */
  .admin-toolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-bottom: 12px;
  }
  .admin-toolbar .admin-search {
    flex: 1 1 200px;
    max-width: 320px;
    min-width: 160px;
  }
  .admin-filters {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
    gap: 10px;
    padding: 12px;
    margin-bottom: 12px;
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 8px;
    background: rgba(var(--v-theme-on-surface), 0.02);
  }
  .admin-table {
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 8px;
    overflow: hidden;
  }
  .admin-table th {
    font-weight: 600 !important;
    white-space: nowrap;
  }
  .admin-count {
    font-size: 0.8rem;
    opacity: 0.7;
    white-space: nowrap;
    margin-inline: 4px;
  }
  .admin-mono {
    font-family: ui-monospace, 'Cascadia Mono', Consolas, 'DejaVu Sans Mono', monospace;
    font-size: 0.8rem;
  }
  .admin-row-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 2px;
    white-space: nowrap;
  }
  .admin-clickable {
    cursor: pointer;
  }
  .admin-clickable:hover {
    text-decoration: underline dotted;
  }
  .admin-summary {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 10px;
    margin-bottom: 14px;
  }
  .admin-tile {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 8px;
    background: rgba(var(--v-theme-on-surface), 0.02);
    user-select: none;
  }
  .admin-tile[role='button'] {
    cursor: pointer;
  }
  .admin-tile[role='button']:hover {
    background: rgba(var(--v-theme-on-surface), 0.06);
  }
  .admin-tile-active {
    border-color: rgb(var(--v-theme-primary));
    background: rgba(var(--v-theme-primary), 0.08);
  }
  .admin-tile-value {
    font-size: 1.25rem;
    font-weight: 600;
    line-height: 1.2;
  }
  .admin-tile-label {
    font-size: 0.75rem;
    opacity: 0.7;
  }
  .admin-details {
    background: rgba(var(--v-theme-on-surface), 0.03);
    padding: 12px 16px !important;
  }
  .admin-details-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: 16px;
    align-items: start;
  }
  .admin-dialog-section {
    font-size: 0.75rem;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    opacity: 0.7;
    margin: 8px 0 10px;
  }
  .admin-kv {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 2px 16px;
    font-size: 0.8rem;
  }
  .admin-kv dt {
    opacity: 0.7;
  }
  .admin-kv dd {
    margin: 0;
    overflow-wrap: anywhere;
  }
</style>
