<template>
  <v-container fluid class="pa-1 sec-root">
    <!-- Query -->
    <v-toolbar density="compact" color="surface" class="mb-1 px-2 sec-toolbar" flat>
      <v-icon class="mr-2" color="primary">mdi-shield-alert-outline</v-icon>
      <span class="text-title-medium mr-4">{{ $t('secEvents.title') }}</span>
      <span class="text-body-small mr-1">{{ $t('secEvents.from') }}</span>
      <v-text-field v-model="from" type="datetime-local" step="1" density="compact" variant="outlined" hide-details
        class="mr-2" style="max-width: 230px"></v-text-field>
      <span class="text-body-small mr-1">{{ $t('secEvents.to') }}</span>
      <v-text-field v-model="to" type="datetime-local" step="1" density="compact" variant="outlined" hide-details
        class="mr-2" style="max-width: 230px" :placeholder="$t('secEvents.now')"></v-text-field>
      <v-select v-model="minSeverity" :items="severityItems" density="compact" variant="outlined" hide-details
        class="mr-2" style="max-width: 170px" :label="$t('secEvents.minSeverity')"></v-select>
      <v-select v-model="categories" :items="categoryItems" multiple chips closable-chips density="compact"
        variant="outlined" hide-details class="mr-2" style="max-width: 260px"
        :label="$t('secEvents.category')"></v-select>
      <v-text-field v-model="text" density="compact" variant="outlined" hide-details clearable class="mr-2"
        style="max-width: 220px" :placeholder="$t('secEvents.filterHint')" @keydown.enter="search"></v-text-field>
      <v-btn size="small" color="primary" variant="tonal" class="mr-2" :loading="loading" @click="search">
        {{ $t('secEvents.find') }}
      </v-btn>
      <v-checkbox v-model="autoRefresh" :label="$t('secEvents.autoRefresh')" density="compact" hide-details
        class="flex-grow-0 mr-2"></v-checkbox>
      <v-btn icon size="small" variant="text" :title="$t('secEvents.exportCsv')" :disabled="!rows.length"
        @click="exportCsv">
        <v-icon>mdi-download</v-icon>
      </v-btn>
    </v-toolbar>

    <!-- Counts per severity (of everything matching the query) -->
    <div class="sev-bar mb-1 px-2">
      <span v-for="(s, i) in SEVERITIES" :key="s" :class="['sev-chip', 'sev-' + s]"
        :style="{ opacity: i < minSeverity ? 0.35 : 1 }" :title="$t('secEvents.showFrom', { s: severityName(i) })"
        @click="setMin(i)">
        {{ severityName(i) }} <b>{{ bySeverity[s] || 0 }}</b>
      </span>
      <v-spacer></v-spacer>
      <span class="text-body-small">
        <b>{{ rows.length }}</b> / {{ total }}{{ totalCapped ? '+' : '' }} {{ $t('secEvents.events') }}
        <template v-if="lastUpdate"> — {{ lastUpdate }}</template>
      </span>
      <span v-if="truncated" class="text-body-small text-warning ml-2">
        {{ $t('secEvents.limited', { n: rows.length }) }}
      </span>
    </div>

    <v-alert v-if="errorText" type="error" density="compact" variant="tonal" class="mb-1">{{ errorText }}</v-alert>

    <v-data-table-virtual :headers="headers" :items="rows" density="compact" fixed-header height="100%"
      item-value="_id" class="sec-grid" :no-data-text="$t('secEvents.noData')" @click:row="openRow">
      <template #[`item.timeTag`]="{ item }">
        <span class="text-no-wrap">{{ fmtTime(item.timeTag) }}</span>
      </template>
      <template #[`item.severity`]="{ item }">
        <span :class="['sev-chip', 'small', 'sev-' + item.severity]">{{ severityName(item.severityLevel) }}</span>
      </template>
      <template #[`item.kind`]="{ item }">
        <span class="text-no-wrap" :title="kindTitle(item)">{{ item.kind }}</span>
      </template>
      <template #[`item.connection`]="{ item }">
        <span class="text-no-wrap" :title="item.protocolDriver + ' ' + item.protocolDriverInstanceNumber">
          {{ item.protocolConnectionNumber }} {{ item.connectionName }}
        </span>
      </template>
      <template #[`item.peer`]="{ item }">
        <span class="text-no-wrap">{{ item.peerAddress || item.peerIp }}</span>
        <span v-if="item.peerMac" class="text-medium-emphasis text-no-wrap"> {{ item.peerMac }}</span>
      </template>
      <template #[`item.detail`]="{ item }">
        <span class="d-inline-block text-truncate" style="max-width: 460px" :title="detailText(item)">
          {{ detailText(item) }}
        </span>
      </template>
      <template #[`item.count`]="{ item }">
        <span v-if="item.count > 1">×{{ item.count }}</span>
      </template>
    </v-data-table-virtual>

    <!-- Event detail -->
    <v-dialog v-model="dialog" max-width="760" scrollable>
      <v-card v-if="selected">
        <v-card-title class="d-flex align-center">
          <span :class="['sev-chip', 'sev-' + selected.severity, 'mr-2']">{{ severityName(selected.severityLevel) }}</span>
          {{ selected.kind }}
          <v-spacer></v-spacer>
          <v-btn icon variant="text" size="small" @click="dialog = false"><v-icon>mdi-close</v-icon></v-btn>
        </v-card-title>
        <v-card-text>
          <pre class="sec-json">{{ JSON.stringify(selected, null, 2) }}</pre>
        </v-card-text>
      </v-card>
    </v-dialog>
  </v-container>
</template>

<script setup>
/*
 * Security events viewer: the secEvents collection written by the protocol
 * drivers, through POST /Invoke/auth/listSecEvents (needs a role with the
 * viewSecEvents right; the menu entry is shown only to those users).
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 */
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getUserSetting, setUserSetting } from '../lib/viewerHelpers'

const { t } = useI18n()

const SEVERITIES = ['info', 'notice', 'warning', 'alert', 'critical']
const CATEGORIES = [
  'connection',
  'tls',
  'peer',
  'association',
  'auth',
  'identity',
  'protocol',
  'sequence',
  'access',
  'control',
  'configuration',
  'unsolicited',
  'flood',
  'resource',
  'file',
  'meta',
]
const REFRESH_MS = 10000
const SETTINGS_KEY = 'sec_settings'

const pad = (n) => String(n).padStart(2, '0')
function toLocalInput(d) {
  return (
    d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + 'T' +
    pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds())
  )
}

const from = ref(toLocalInput(new Date(Date.now() - 24 * 3600 * 1000)))
const to = ref('')
const minSeverity = ref(1)
const categories = ref([])
const text = ref('')
const autoRefresh = ref(true)

const rows = ref([])
const total = ref(0)
const totalCapped = ref(false)
const truncated = ref(false)
const bySeverity = ref({})
const loading = ref(false)
const errorText = ref('')
const lastUpdate = ref('')
const dialog = ref(false)
const selected = ref(null)
let timer = null

try {
  const s = JSON.parse(getUserSetting(SETTINGS_KEY) || '{}')
  if (Number.isInteger(s.minSeverity)) minSeverity.value = s.minSeverity
  if (Array.isArray(s.categories)) categories.value = s.categories.filter((c) => CATEGORIES.includes(c))
  if (typeof s.autoRefresh === 'boolean') autoRefresh.value = s.autoRefresh
} catch (e) {
  // keep the defaults
}
watch([minSeverity, categories, autoRefresh], () => {
  setUserSetting(
    SETTINGS_KEY,
    JSON.stringify({ minSeverity: minSeverity.value, categories: categories.value, autoRefresh: autoRefresh.value })
  )
  schedule()
})

const severityName = (i) => t('secEvents.severities.' + (SEVERITIES[i] || 'info'))
const severityItems = computed(() => SEVERITIES.map((s, i) => ({ title: severityName(i), value: i })))
const categoryItems = computed(() => CATEGORIES.map((c) => ({ title: c, value: c })))

const headers = computed(() => [
  { title: t('secEvents.columns.time'), key: 'timeTag', width: 175 },
  { title: t('secEvents.columns.severity'), key: 'severity', width: 95 },
  { title: t('secEvents.columns.kind'), key: 'kind' },
  { title: t('secEvents.columns.connection'), key: 'connection' },
  { title: t('secEvents.columns.peer'), key: 'peer' },
  { title: t('secEvents.columns.object'), key: 'objectRef' },
  { title: t('secEvents.columns.action'), key: 'action' },
  { title: t('secEvents.columns.detail'), key: 'detail' },
  { title: t('secEvents.columns.source'), key: 'source' },
  { title: '#', key: 'count', width: 50 },
])

function fmtTime(v) {
  const d = new Date(v)
  return isNaN(d.getTime()) ? '' : d.toLocaleString()
}
function detailText(e) {
  return [e.detail, e.value && 'value: ' + e.value, e.error && 'error: ' + e.error].filter(Boolean).join(' — ')
}
function kindTitle(e) {
  return [e.category, e.technique && 'MITRE ' + e.technique, e.std62351].filter(Boolean).join(' · ')
}
function setMin(i) {
  minSeverity.value = i
  search()
}

async function search() {
  loading.value = true
  try {
    const body = { minSeverity: minSeverity.value, categories: categories.value, limit: 5000 }
    const b = new Date(from.value)
    if (!isNaN(b.getTime())) body.timeBegin = b.toISOString()
    if (to.value) {
      const e = new Date(to.value)
      if (!isNaN(e.getTime())) body.timeEnd = e.toISOString()
    }
    if (text.value) body.text = text.value
    const resp = await fetch('/Invoke/auth/listSecEvents', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify(body),
    })
    const json = await resp.json().catch(() => null)
    if (resp.status === 403) throw new Error(t('secEvents.noRight'))
    if (!json || json.error) throw new Error(json?.unavailable ? t('secEvents.unavailable') : json?.error || 'invalid answer')
    rows.value = json.events || []
    total.value = json.total || 0
    totalCapped.value = !!json.totalCapped
    truncated.value = !!json.truncated
    bySeverity.value = json.bySeverity || {}
    errorText.value = ''
    lastUpdate.value = new Date().toLocaleTimeString()
  } catch (err) {
    errorText.value = err.message
  } finally {
    loading.value = false
  }
}

function schedule() {
  clearInterval(timer)
  timer = null
  // auto refresh only follows the present: a closed period does not change
  if (autoRefresh.value) timer = setInterval(() => !to.value && !loading.value && search(), REFRESH_MS)
}

function openRow(_, { item }) {
  selected.value = item
  dialog.value = true
}

function exportCsv() {
  const cols = ['timeTag', 'severity', 'kind', 'category', 'action', 'source', 'protocolDriver',
    'protocolConnectionNumber', 'connectionName', 'peerAddress', 'peerMac', 'objectRef', 'value',
    'detail', 'error', 'count', 'technique', 'std62351']
  const q = (v) => '"' + String(v ?? '').replace(/"/g, '""') + '"'
  const lines = [cols.join(',')].concat(rows.value.map((r) => cols.map((c) => q(r[c])).join(',')))
  const blob = new Blob([lines.join('\r\n')], { type: 'text/csv' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = 'secEvents-' + toLocalInput(new Date()).replace(/[:T]/g, '-') + '.csv'
  a.click()
  setTimeout(() => URL.revokeObjectURL(a.href), 1000)
}

onMounted(() => {
  search()
  schedule()
})
onBeforeUnmount(() => clearInterval(timer))
</script>

<style scoped>
.sec-root {
  height: 100vh;
  display: flex;
  flex-direction: column;
}
.sec-toolbar :deep(.v-toolbar__content) {
  flex-wrap: wrap;
  height: auto !important;
  row-gap: 4px;
  padding: 4px 0;
}
.sec-grid {
  flex: 1 1 auto;
  min-height: 0;
  cursor: pointer;
}
.sev-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}
.sev-chip {
  display: inline-block;
  padding: 1px 8px;
  border-radius: 10px;
  font-size: 0.8rem;
  cursor: pointer;
  white-space: nowrap;
  color: #fff;
}
.sev-chip.small {
  cursor: default;
  font-size: 0.75rem;
}
.sev-info {
  background: #607d8b;
}
.sev-notice {
  background: #1e88e5;
}
.sev-warning {
  background: #f9a825;
  color: #222;
}
.sev-alert {
  background: #e65100;
}
.sev-critical {
  background: #b71c1c;
}
.sec-json {
  font-size: 0.8rem;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
