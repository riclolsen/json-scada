<template>
  <div class="user-actions-tab">
    <div class="admin-toolbar">
      <v-btn-toggle v-model="period" mandatory density="compact" variant="outlined" divided color="primary" class="ua-periods">
        <v-btn v-for="p in periods" :key="p.value" :value="p.value" size="small">{{ p.title }}</v-btn>
      </v-btn-toggle>
      <v-menu v-if="period === 'custom'" v-model="menu" :close-on-content-click="false" min-width="auto">
        <template #activator="{ props }">
          <v-text-field
            v-model="dateRangeText"
            class="admin-search"
            :label="t('admin.userActions.searchDateRange')"
            prepend-inner-icon="mdi-calendar-range"
            density="compact"
            variant="outlined"
            hide-details
            readonly
            v-bind="props"
          ></v-text-field>
        </template>
        <v-date-picker
          v-model="dates"
          :locale="locale"
          hide-title
          show-adjacent-months
          multiple="range"
          @update:model-value="applyFilters"
        ></v-date-picker>
      </v-menu>
      <v-spacer></v-spacer>
      <span class="admin-count">{{ $t('admin.common.results', { n: totalUserActions.toLocaleString() }) }}</span>
      <v-btn icon variant="text" size="small" :title="$t('common.refresh')" @click="fetchUserActions">
        <v-icon>mdi-refresh</v-icon>
      </v-btn>
      <v-btn
        icon
        variant="text"
        size="small"
        :color="autoRefresh ? 'primary' : undefined"
        :title="$t('admin.common.autoRefresh')"
        @click="autoRefresh = !autoRefresh"
      >
        <v-icon>{{ autoRefresh ? 'mdi-timer-sync-outline' : 'mdi-timer-off-outline' }}</v-icon>
      </v-btn>
      <v-btn icon variant="text" size="small" :loading="exporting" :title="$t('admin.common.exportCsv')" @click="exportCsv">
        <v-icon>mdi-file-delimited-outline</v-icon>
      </v-btn>
    </div>

    <div class="admin-filters">
      <v-text-field
        v-model="searchUsername"
        :label="t('admin.userActions.searchUsername')"
        prepend-inner-icon="mdi-account-search-outline"
        clearable
        density="compact"
        variant="outlined"
        hide-details
        @keyup.enter="applyFilters"
        @click:clear="clearAndApply('searchUsername')"
      ></v-text-field>
      <v-combobox
        v-model="searchAction"
        :items="knownActions"
        :label="t('admin.userActions.searchAction')"
        prepend-inner-icon="mdi-gesture-tap"
        clearable
        density="compact"
        variant="outlined"
        hide-details
        @update:model-value="applyFilters"
      ></v-combobox>
      <v-text-field
        v-model="searchTag"
        :label="t('admin.userActions.searchTag')"
        prepend-inner-icon="mdi-tag-search-outline"
        clearable
        density="compact"
        variant="outlined"
        hide-details
        @keyup.enter="applyFilters"
        @click:clear="clearAndApply('searchTag')"
      ></v-text-field>
      <div class="d-flex align-center">
        <v-btn variant="text" prepend-icon="mdi-filter-remove-outline" :disabled="!hasFilters" @click="clearFilters">
          {{ $t('admin.common.clearFilters') }}
        </v-btn>
      </div>
    </div>

    <v-data-table-server
      v-model:page="page"
      v-model:items-per-page="itemsPerPage"
      v-model:sort-by="sortBy"
      v-model:expanded="expanded"
      show-expand
      :headers="headers"
      :items="userActions"
      :items-length="totalUserActions"
      :loading="loading"
      :items-per-page-options="[10, 25, 50, 100, 250, 500, 1000]"
      :items-per-page-text="$t('common.itemsPerPageText')"
      multi-sort
      hover
      class="admin-table"
      density="compact"
      item-value="_id"
    >
      <template #[`item.timeTag`]="{ item }">
        <span class="text-body-small ua-time" :title="item.isoTime">{{ item.localTime }}</span>
      </template>
      <template #[`item.username`]="{ item }">
        <span class="admin-clickable" :title="$t('admin.userActions.filterBy')" @click="filterBy('username', item.username)">
          <v-icon size="16" class="me-1">mdi-account-outline</v-icon>{{ item.username }}
        </span>
      </template>
      <template #[`item.action`]="{ item }">
        <v-chip
          size="small"
          label
          variant="tonal"
          :color="actionColor(item.action)"
          class="admin-clickable"
          :title="$t('admin.userActions.filterBy')"
          @click="filterBy('action', item.action)"
        >
          {{ item.action }}
        </v-chip>
      </template>
      <template #[`item.tag`]="{ item }">
        <span v-if="item.tag" class="admin-mono admin-clickable" :title="$t('admin.userActions.filterBy')"
          @click="filterBy('tag', item.tag)">{{ item.tag }}</span>
      </template>
      <template #[`item.pointKey`]="{ item }">
        <span class="admin-mono">{{ item.pointKey ?? '' }}</span>
      </template>
      <template #[`item.summary`]="{ item }">
        <span class="text-body-small text-medium-emphasis ua-summary">{{ summaryOf(item) }}</span>
      </template>
      <template #expanded-row="{ columns, item }">
        <tr>
          <td :colspan="columns.length" class="ua-details">
            <div class="ua-details-grid">
              <dl class="admin-kv">
                <dt>{{ $t('admin.userActions.headers.time') }}</dt>
                <dd>{{ item.localTime }} <span class="admin-mono text-medium-emphasis">({{ item.isoTime }})</span></dd>
                <template v-for="[k, v] in propertyEntries(item)" :key="k">
                  <dt>{{ k }}</dt>
                  <dd class="admin-mono">{{ v }}</dd>
                </template>
              </dl>
              <div class="ua-json">
                <v-btn size="x-small" variant="text" prepend-icon="mdi-content-copy" class="ua-copy"
                  @click="copyJson(item)">{{ $t('admin.common.copy') }}</v-btn>
                <pre class="admin-mono">{{ JSON.stringify(item.properties ?? {}, null, 2) }}</pre>
              </div>
            </div>
          </td>
        </tr>
      </template>
    </v-data-table-server>

    <v-snackbar v-model="snackbar.show" :color="snackbar.color" :timeout="3500" location="bottom right">
      {{ snackbar.text }}
    </v-snackbar>
  </div>
</template>

<script setup>
  import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { postJson, fetchAllPages, toCsv, downloadText, fileStamp, loadPref, savePref, copyText } from '../lib/adminList'

  const props = defineProps({
    // the section is shown (auto refresh runs only then)
    active: { type: Boolean, default: false },
  })
  const { t, locale } = useI18n()

  const menu = ref(false)
  const userActions = ref([])
  const searchUsername = ref('')
  const searchAction = ref('')
  const searchTag = ref('')
  const totalUserActions = ref(0)
  const itemsPerPage = ref(loadPref('user-actions-page-size', 25))
  const dates = ref([])
  const period = ref('all')
  const loading = ref(true)
  const page = ref(1)
  const expanded = ref([])
  // most recent first
  const sortBy = ref([{ key: 'timeTag', order: 'desc' }])

  // actions recorded by server_realtime_auth (operator actions in index.js,
  // admin routes in auth.controller.js); others can be typed in
  const knownActions = [
    'Command',
    'Update Properties',
    'Ack All Alarms',
    'Ack Point Alarm',
    'Silence Beep',
    'Ack All Events',
    'Ack Point Events',
    'Ack One Event',
    'Remove All Events',
    'Remove Point Events',
    'Remove One Event',
    'signin',
    'signout',
    'changePassword',
    'createTag',
    'updateTag',
    'deleteTag',
    'createProtocolConnection',
    'updateProtocolConnection',
    'deleteProtocolConnection',
    'createProtocolDriverInstance',
    'updateProtocolDriverInstance',
    'deleteProtocolDriverInstance',
    'createUser',
    'updateUser',
    'deleteUser',
    'userAddRole',
    'userRemoveRole',
    'createRole',
    'updateRole',
    'deleteRole',
    'saveDisplay',
    'updateSystemSettings',
    'exportProject',
    'importProject',
    'restartProtocols',
    'restartProcesses',
    'syncDriverServices',
    'pruneConfiguration',
  ]

  const periods = computed(() => [
    { value: 'all', title: t('admin.userActions.periods.all') },
    { value: '1h', title: t('admin.userActions.periods.lastHour') },
    { value: '24h', title: t('admin.userActions.periods.last24h') },
    { value: '7d', title: t('admin.userActions.periods.last7d') },
    { value: '30d', title: t('admin.userActions.periods.last30d') },
    { value: 'custom', title: t('admin.userActions.periods.custom') },
  ])
  const periodMs = { '1h': 3600e3, '24h': 86400e3, '7d': 7 * 86400e3, '30d': 30 * 86400e3 }

  const headers = computed(() => [
    { title: t('admin.userActions.headers.time'), key: 'timeTag', width: 190 },
    { title: t('admin.userActions.headers.username'), key: 'username' },
    { title: t('admin.userActions.headers.action'), key: 'action' },
    { title: t('admin.userActions.headers.tag'), key: 'tag' },
    { title: t('admin.userActions.headers.pointKey'), key: 'pointKey', align: 'end' },
    { title: t('admin.userActions.headers.details'), key: 'summary', sortable: false },
    { title: '', key: 'data-table-expand', width: 40 },
  ])

  const dateRangeText = computed(() => dates.value.map((d) => new Date(d).toLocaleDateString()).join(' – '))
  const hasFilters = computed(
    () => !!(searchUsername.value?.trim() || searchAction.value?.trim?.() || searchTag.value?.trim()) || period.value !== 'all'
  )

  function actionColor(a) {
    const s = String(a || '')
    if (s === 'Command') return 'orange'
    if (/^(Ack|Silence)/.test(s)) return 'info'
    if (/^(Remove|delete|prune)/i.test(s)) return 'error'
    if (/^(create|import)/i.test(s)) return 'success'
    if (/^(update|save|userAdd|userRemove)/i.test(s)) return 'primary'
    if (/^(restart|sync)/.test(s)) return 'warning'
    return undefined
  }

  function scalarText(v) {
    if (v === null || v === undefined) return ''
    if (typeof v === 'object') return JSON.stringify(v)
    return String(v)
  }
  // one line about the action: command value, changed properties or the
  // identity of the configured object
  function summaryOf(item) {
    const p = item.properties
    if (!p || typeof p !== 'object') return ''
    if (item.action === 'Command') {
      const v = p.valueString ? p.valueString : scalarText(p.value)
      return v !== '' ? '= ' + v : ''
    }
    if (item.action === 'Update Properties') return Object.keys(p).join(', ')
    const keys = ['name', 'tag', 'username', 'protocolDriver', 'protocolConnectionNumber', 'protocolDriverInstanceNumber', '_id']
    return keys
      .filter((k) => p[k] !== undefined && p[k] !== null && p[k] !== '')
      .slice(0, 3)
      .map((k) => k + ': ' + scalarText(p[k]))
      .join(' · ')
  }
  function propertyEntries(item) {
    const p = item.properties
    if (!p || typeof p !== 'object') return []
    return Object.entries(p)
      .slice(0, 40)
      .map(([k, v]) => {
        const s = scalarText(v)
        return [k, s.length > 200 ? s.slice(0, 200) + '…' : s]
      })
  }

  function buildFilter() {
    const filter = {}
    const re = (v) => ({ $regex: v.trim(), $options: 'i' })
    if (searchUsername.value?.trim()) filter.username = re(searchUsername.value)
    const act = typeof searchAction.value === 'string' ? searchAction.value.trim() : ''
    if (act) filter.action = knownActions.includes(act) ? act : re(act)
    if (searchTag.value?.trim()) filter.tag = re(searchTag.value)

    if (period.value in periodMs) {
      filter.timeTag = { $gte: new Date(Date.now() - periodMs[period.value]).toISOString() }
    } else if (period.value === 'custom' && dates.value.length) {
      // whole local days, from the first to the last picked day
      const dts = dates.value.map((d) => new Date(d)).sort((a, b) => a - b)
      const start = new Date(dts[0])
      start.setHours(0, 0, 0, 0)
      const end = new Date(dts[dts.length - 1])
      end.setHours(0, 0, 0, 0)
      end.setDate(end.getDate() + 1)
      filter.timeTag = { $gte: start.toISOString(), $lt: end.toISOString() }
    }
    return filter
  }

  function applyFilters() {
    // a page change makes the watcher below read the list
    if (page.value !== 1) page.value = 1
    else fetchUserActions()
  }
  function clearAndApply(name) {
    const refs = { searchUsername, searchTag }
    refs[name].value = ''
    applyFilters()
  }
  function clearFilters() {
    searchUsername.value = ''
    searchAction.value = ''
    searchTag.value = ''
    dates.value = []
    period.value = 'all'
    applyFilters()
  }
  function filterBy(field, value) {
    if (field === 'username') searchUsername.value = value
    else if (field === 'action') searchAction.value = value
    else if (field === 'tag') searchTag.value = value
    applyFilters()
  }
  watch(period, (p) => {
    if (p === 'custom') {
      menu.value = true
      if (!dates.value.length) return
    }
    applyFilters()
  })

  watch(
    [page, itemsPerPage, sortBy],
    () => {
      savePref('user-actions-page-size', itemsPerPage.value)
      fetchUserActions()
    },
    { deep: true }
  )

  onMounted(() => {
    fetchUserActions()
  })

  const withTimes = (action) => {
    const d = action.timeTag ? new Date(action.timeTag) : null
    return {
      ...action,
      isoTime: d ? d.toISOString() : '',
      localTime: d ? d.toLocaleString() : '',
    }
  }

  // answers of older requests (filters changed meanwhile) are dropped
  let fetchSeq = 0
  const fetchUserActions = async (quiet = false) => {
    const seq = ++fetchSeq
    if (quiet !== true) loading.value = true
    try {
      const json = await postJson('/Invoke/auth/listUserActions', {
        itemsPerPage: itemsPerPage.value,
        sortBy: sortBy.value,
        page: page.value,
        filter: buildFilter(),
      })
      if (seq !== fetchSeq) return
      if (json.error) {
        notify(t('common.error') + ' ' + json.error, 'error')
        return
      }
      userActions.value = (json.userActions || []).map(withTimes)
      totalUserActions.value = json.countTotal || 0
    } catch (err) {
      console.warn(err)
      totalUserActions.value = 0
    } finally {
      if (seq === fetchSeq) loading.value = false
    }
  }

  const snackbar = ref({ show: false, text: '', color: undefined })
  function notify(text, color) {
    snackbar.value = { show: true, text, color }
  }
  async function copyJson(item) {
    if (await copyText(JSON.stringify(item.properties ?? {}, null, 2))) notify(t('admin.common.copied'))
  }

  const exporting = ref(false)
  const EXPORT_MAX = 100000
  async function exportCsv() {
    exporting.value = true
    try {
      const { items, truncated } = await fetchAllPages(
        '/Invoke/auth/listUserActions',
        { sortBy: sortBy.value, filter: buildFilter() },
        'userActions',
        EXPORT_MAX
      )
      const rows = items.map(withTimes)
      const cols = [
        { title: 'timeTag', value: (r) => r.isoTime },
        { title: 'localTime', value: (r) => r.localTime },
        { title: 'username', value: (r) => r.username },
        { title: 'action', value: (r) => r.action },
        { title: 'tag', value: (r) => r.tag },
        { title: 'pointKey', value: (r) => r.pointKey },
        { title: 'details', value: (r) => summaryOf(r) },
        { title: 'properties', value: (r) => JSON.stringify(r.properties ?? {}) },
      ]
      downloadText('user_actions_' + fileStamp() + '.csv', toCsv(rows, cols))
      if (truncated) notify(t('admin.common.exportLimited', { n: EXPORT_MAX.toLocaleString() }), 'warning')
    } catch (err) {
      notify(t('common.error') + ' ' + err.message, 'error')
    } finally {
      exporting.value = false
    }
  }

  // new actions appear every 5 s while the section is shown
  const autoRefresh = ref(false)
  let refreshTimer = null
  watch([autoRefresh, () => props.active], ([on, active]) => {
    clearInterval(refreshTimer)
    refreshTimer = null
    if (on && active)
      refreshTimer = setInterval(() => {
        if (!document.hidden && !loading.value) fetchUserActions(true)
      }, 5000)
  })
  onUnmounted(() => clearInterval(refreshTimer))

  defineExpose({ fetchUserActions })
</script>

<style scoped>
  .ua-periods {
    flex-wrap: wrap;
    height: auto !important;
  }
  .ua-time {
    white-space: nowrap;
  }
  .ua-summary {
    display: inline-block;
    max-width: 420px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    vertical-align: middle;
  }
  .ua-details {
    background: rgba(var(--v-theme-on-surface), 0.03);
    padding: 12px 16px !important;
  }
  .ua-details-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 16px;
    align-items: start;
  }
  .ua-json {
    position: relative;
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 6px;
    padding: 8px 10px;
    max-height: 320px;
    overflow: auto;
  }
  .ua-json pre {
    margin: 0;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .ua-copy {
    position: absolute;
    top: 4px;
    right: 4px;
  }
</style>
