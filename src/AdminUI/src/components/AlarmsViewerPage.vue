<template>
  <v-container fluid class="pa-1 alarms-root" :style="rootStyle">
    <!-- Toolbar -->
    <v-toolbar density="compact" color="surface" class="mb-1 px-2" flat>
      <v-btn-toggle :model-value="mode" mandatory density="compact" variant="outlined" divided class="mr-2"
        @update:model-value="onModeChange">
        <v-btn value="realtime" size="small">{{ $t('alarmsViewer.modes.normal') }}</v-btn>
        <v-btn value="aggregated" size="small">{{ $t('alarmsViewer.modes.aggregated') }}</v-btn>
        <v-btn value="historical" size="small">{{ $t('alarmsViewer.modes.historical') }}</v-btn>
      </v-btn-toggle>

      <template v-if="isRealtime">
        <v-btn size="small" variant="tonal" color="warning" class="mr-2" :disabled="!userHasRight('ackAlarms')"
          @click="ackAll">
          {{ $t('tabularViewer.ackAll') }} (F8)
        </v-btn>
        <v-checkbox v-model="unackedOnly" :label="$t('alarmsViewer.unackedOnly')" density="compact" hide-details
          class="mr-2 flex-grow-0"></v-checkbox>
        <v-btn icon size="small" variant="text" :title="$t('tabularViewer.beep')" @click="toggleBeep">
          <v-icon>{{ beepOn ? 'mdi-volume-high' : 'mdi-volume-off' }}</v-icon>
        </v-btn>
        <v-btn v-if="alarmBeep" icon size="small" variant="text" color="error"
          :title="$t('tabularViewer.silenceBeep')" @click="silenceBeep">
          <v-icon>mdi-bell-off</v-icon>
        </v-btn>
      </template>

      <v-menu :close-on-content-click="false" location="bottom end">
        <template #activator="{ props }">
          <v-btn v-bind="props" icon size="small" variant="text" :disabled="!prefsReady"
            :title="$t('admin.common.columns')" :aria-label="$t('admin.common.columns')">
            <v-icon>mdi-view-column-outline</v-icon>
          </v-btn>
        </template>
        <v-list density="compact" min-width="220" role="menu" :aria-label="$t('admin.common.columns')">
          <v-list-item v-for="column in optionalColumns" :key="column.key" :title="column.title" link
            :prepend-icon="visibleColumns.includes(column.key) ? 'mdi-checkbox-marked' : 'mdi-checkbox-blank-outline'"
            role="menuitemcheckbox" :aria-checked="visibleColumns.includes(column.key)"
            @click="toggleColumn(column.key)" />
        </v-list>
      </v-menu>
      <v-btn icon size="small" variant="text" :title="$t('tabularViewer.fontUp')" @click="fontInc(1)">
        <v-icon>mdi-format-font-size-increase</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('tabularViewer.fontDown')" @click="fontInc(-1)">
        <v-icon>mdi-format-font-size-decrease</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('tabularViewer.copy')" @click="copyGrid">
        <v-icon>mdi-content-copy</v-icon>
      </v-btn>

      <v-spacer></v-spacer>
      <span class="text-body-small mr-2">
        <b>{{ visibleRows.length }}</b> / {{ modeRows.length }} {{ $t('alarmsViewer.alarms') }}
        <template v-if="lastUpdate"> — {{ lastUpdate }}</template>
      </span>
      <span class="text-body-small fan">{{ fanState }}</span>
    </v-toolbar>

    <!-- Historical query -->
    <v-toolbar v-if="mode === 'historical'" density="compact" color="surface" class="mb-1 px-2" flat>
      <span class="text-body-small mr-1">{{ $t('alarmsViewer.hist.from') }}</span>
      <v-text-field v-model="histFrom" type="datetime-local" step="1" density="compact" variant="outlined"
        hide-details class="mr-2" style="max-width: 230px"></v-text-field>
      <span class="text-body-small mr-1">{{ $t('alarmsViewer.hist.to') }}</span>
      <v-text-field v-model="histTo" type="datetime-local" step="1" density="compact" variant="outlined"
        hide-details class="mr-2" style="max-width: 230px"></v-text-field>
      <span class="text-body-small mr-1">{{ $t('alarmsViewer.hist.filter') }}</span>
      <v-text-field v-model="histText" density="compact" variant="outlined" hide-details clearable class="mr-2"
        style="max-width: 200px" :placeholder="$t('alarmsViewer.hist.filterHint')"
        @keydown.enter="histSearch"></v-text-field>
      <v-btn size="small" color="primary" variant="tonal" :loading="histLoading" @click="histSearch">
        {{ $t('alarmsViewer.hist.find') }}
      </v-btn>
      <span v-if="histLimited" class="text-body-small text-warning ml-2">
        {{ $t('alarmsViewer.hist.limited', { n: rows.length }) }}
      </span>
    </v-toolbar>

    <!-- Station / priority filter chips -->
    <div class="chips-bar mb-1">
      <v-btn size="x-small" variant="tonal" class="mr-1" @click="selectAllStations(false)">
        {{ $t('tabularViewer.selectAll') }}
      </v-btn>
      <v-btn size="x-small" variant="tonal" class="mr-2" @click="selectAllStations(true)">
        {{ $t('tabularViewer.unselectAll') }}
      </v-btn>
      <v-btn v-for="sel in cfg.TabularViewer_CustomFiltersSelectors" :key="sel.name" size="x-small" variant="tonal"
        class="mr-1" @click="applyCustomSelector(sel)">
        {{ sel.name }}
      </v-btn>
      <span class="chip-group">
        <span v-for="pri in priorityChips" :key="'pri' + pri.id" class="alm-chip"
          :style="{ opacity: filterOut[pri.id] ? 0.25 : 1 }" @click="toggleFilterOut(pri.id)">
          {{ pri.id }}
          <span class="badge" :style="badgeStyle(pri.minNack)">{{ pri.nack }}</span>
          <span class="badge ack" :style="badgeStyle(pri.minAck)">{{ pri.ack }}</span>
        </span>
      </span>
      <span class="chip-group">
        <span v-for="st in stationChips" :key="'st' + st.id" class="alm-chip"
          :style="{ opacity: filterOut[st.id] ? 0.25 : 1 }" @click="toggleFilterOut(st.id)">
          {{ st.id }}
          <span class="badge" :style="badgeStyle(st.minNack)">{{ st.nack }}</span>
          <span class="badge ack" :style="badgeStyle(st.minAck)">{{ st.ack }}</span>
        </span>
      </span>
    </div>

    <!-- Alarm list -->
    <v-data-table-virtual :headers="headers" :items="visibleRows"
      :no-data-text="failed ? $t('tabularViewer.serverFail') : $t('tabularViewer.noData')" density="compact"
      fixed-header :height="'100%'" :item-height="rowHeight" item-value="rowId" class="alarms-grid"
      :style="{ '--row-height': rowHeight + 'px' }" :row-props="rowProps" @click:row="onRowClick">
      <template #[`item.descr`]="{ item }">
        <span class="d-block text-truncate" :title="item.descr">{{ item.descr }}</span>
      </template>
      <template #[`item.displayValue`]="{ item }">
        <span class="d-inline-block text-truncate" :title="String(item.displayValue)"
          style="max-width: 240px">{{ item.displayValue }}</span>
      </template>
      <template #[`item.text`]="{ item }">
        <span class="d-inline-block text-truncate" :title="item.text" style="max-width: 320px">{{ item.text }}</span>
      </template>
      <template #[`item.station`]="{ item }">
        <span class="d-inline-block text-truncate" :style="{ maxWidth: '100px', color: colorOfStation(item.station) }">
          {{ item.station }}
        </span>
      </template>
      <template #[`item.state`]="{ item }">
        <span :class="['state', item.active ? 'state-active' : 'state-normal']">
          {{ item.active ? $t('alarmsViewer.state.active') : $t('alarmsViewer.state.normal') }}
        </span>
      </template>
      <template #[`item.priority`]="{ item }">
        <span class="qualif pill" :style="priorityPillStyle(item)">{{ item.priority }}{{ isRowAcked(item) ? '' : 'L' }}</span>
      </template>
    </v-data-table-virtual>

    <!-- Point Info + Command dialogs (shared component) -->
    <PointInfoDialog v-model="infoOpen" :point-key="infoKey" />

    <!-- Beep sounds -->
    <audio ref="criticalSound" src="/sounds/critical.wav"></audio>
    <audio ref="nonCriticalSound" src="/sounds/noncritical.wav"></audio>
  </v-container>
</template>

<script setup>
// Alarms Viewer: Normal shows unacknowledged or active alarms. Aggregated
// shows the latest of these per tag. Historical shows occurrences in a period.
// A click acknowledges one occurrence in Normal and the whole point in
// Aggregated. F8 acknowledges all realtime alarms.
// Shift/Alt click opens the point.
import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useTheme } from 'vuetify'
import { loadViewersConfig, getViewersConfig } from '../lib/viewersConfig'
import * as opc from '../lib/opcClient'
import { onAccessDenied } from '../lib/opcClient'
import PointInfoDialog from './PointInfoDialog.vue'
import { OpcAcknowledge, BEEP_POINTKEY } from '../lib/opcCodes'
import {
  userHasRight,
  getUserSetting,
  setUserSetting,
  copyRowsToClipboard,
  getUserGroup1List,
} from '../lib/viewerHelpers'

const { t } = useI18n()
const router = useRouter()
const route = router.currentRoute
const vuetifyTheme = useTheme()

// --- state -------------------------------------------------------------------
const cfg = ref(getViewersConfig())
const rows = ref([])
const group1List = ref([])
const mode = ref('realtime')
const isRealtime = computed(() => mode.value !== 'historical')
const unackedOnly = ref(false)
const fontScale = ref(13)
const filterOut = reactive({}) // station names and priorities (as strings) filtered out
const beepOn = ref(false)
const alarmBeep = ref(0)
const alarmBeepType = ref(0)
const lastUpdate = ref('')
const fanState = ref('(!)')
const failed = ref(false)
const histFrom = ref('')
const histTo = ref('')
const histText = ref('')
const histLoading = ref(false)
const histLimited = ref(false)

const infoOpen = ref(false)
const infoKey = ref(0)
const criticalSound = ref(null)
const nonCriticalSound = ref(null)

let pollTimer = null
let beepTimer = null
// set on unmount: async work still running then must not start timers again
let disposed = false
let lastSuccess = Date.now()
let requestId = 0

// --- columns -----------------------------------------------------------------
const nowrapCol = { cellProps: { class: 'nowrap-cell' }, headerProps: { class: 'nowrap-cell' } }
const allHeaders = computed(() => [
  { title: t('alarmsViewer.columns.time'), key: 'timeStr', optional: true, ...nowrapCol },
  { title: t('tabularViewer.columns.pointNum'), key: 'pointKey', width: 70, optional: true },
  { title: t('tabularViewer.columns.id'), key: 'tag', width: 230, optional: true },
  { title: t('tabularViewer.columns.location'), key: 'station', width: 90, optional: true },
  {
    title: t('tabularViewer.columns.description'),
    key: 'descr',
    width: '100%',
    cellProps: { class: 'descr-cell' },
    headerProps: { class: 'descr-cell' },
  },
  { title: t('tabularViewer.dlg.value'), key: 'displayValue', ...nowrapCol },
  { title: t('alarmsViewer.columns.alarm'), key: 'text', optional: true, ...nowrapCol },
  { title: t('alarmsViewer.columns.state'), key: 'state', optional: true, ...nowrapCol },
  { title: t('alarmsViewer.columns.priority'), key: 'priority', ...nowrapCol },
  { title: t('alarmsViewer.columns.normalTime'), key: 'normalTimeStr', optional: true, ...nowrapCol },
  { title: t('alarmsViewer.columns.ackTime'), key: 'ackTimeStr', optional: true, ...nowrapCol },
  { title: t('alarmsViewer.columns.ackUser'), key: 'ackUser', optional: true, ...nowrapCol },
  { title: t('tabularViewer.columns.sourceTime'), key: 'sourceTimeStr', optional: true, ...nowrapCol },
].map((header) => ({ ...header, sortable: false })))
const optionalColumns = computed(() => allHeaders.value.filter((header) => header.optional))
const realtimeColumns = ['timeStr', 'station', 'text', 'state']
const historicalColumns = [...realtimeColumns, 'normalTimeStr', 'ackTimeStr', 'ackUser']
const selectedColumns = ref(null)
const visibleColumns = computed(() =>
  selectedColumns.value ?? (isRealtime.value ? realtimeColumns : historicalColumns)
)
const headers = computed(() =>
  allHeaders.value.filter((header) => !header.optional || visibleColumns.value.includes(header.key))
)

function toggleColumn(key) {
  const columns = visibleColumns.value
  selectedColumns.value = columns.includes(key) ? columns.filter((column) => column !== key) : [...columns, key]
}

// --- client-side filters -----------------------------------------------------
const modeRows = computed(() => {
  if (mode.value !== 'aggregated') return rows.value
  // The realtime feed is already sorted newest first by timeTag and _id.
  const seen = new Set()
  return rows.value.filter((row) => {
    const key = row.tag || (row.pointKey ?? row.rowId)
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
})

const unackedPoints = computed(() =>
  new Set(rows.value.filter((r) => !r.acked).map((r) => r.pointKey))
)

function isRowAcked(row) {
  return mode.value === 'aggregated' ? !unackedPoints.value.has(row.pointKey) : row.acked
}

const visibleRows = computed(() =>
  modeRows.value.filter((r) => {
    if (filterOut[r.station]) return false
    if (filterOut[String(r.priority)]) return false
    if (isRealtime.value && unackedOnly.value && isRowAcked(r)) return false
    return true
  })
)

// counts of unacknowledged / acknowledged alarms per priority and station
const alarmInfo = computed(() => {
  const info = {}
  const ensure = (id, isStation) => {
    if (!info[id]) info[id] = { ack: 0, nack: 0, minAck: 10, minNack: 10, isStation }
    return info[id]
  }
  for (const r of modeRows.value) {
    for (const e of [ensure(String(r.priority), false), ensure(r.station, true)]) {
      if (isRowAcked(r)) {
        e.ack++
        e.minAck = Math.min(e.minAck, r.priority)
      } else {
        e.nack++
        e.minNack = Math.min(e.minNack, r.priority)
      }
    }
  }
  return info
})
const chipsOf = (isStation) =>
  Object.keys(alarmInfo.value)
    .filter((k) => alarmInfo.value[k].isStation === isStation)
    .sort()
    .map((k) => ({ id: k, ...alarmInfo.value[k] }))
const stationChips = computed(() => chipsOf(true))
const priorityChips = computed(() => chipsOf(false))

// --- styling -----------------------------------------------------------------
const isDark = computed(() => vuetifyTheme.global.current.value.dark)

function colorOfPriority(pri) {
  const arr = cfg.value.ColorOfPriority
  const i = isNaN(pri) ? arr.length - 1 : pri
  return arr[Math.max(0, Math.min(i, arr.length - 1))]
}
function badgeStyle(pri) {
  const c = colorOfPriority(pri)
  return { backgroundColor: c, borderColor: c }
}
const stationIndex = computed(() => {
  const m = {}
  ;[...group1List.value].sort().forEach((s, i) => (m[s] = i))
  return m
})
function colorOfStation(station) {
  const arr = cfg.value.ColorOfSubstation
  return arr[(stationIndex.value[station] || 0) % arr.length]
}

function rowProps({ item }) {
  let color
  if (isRowAcked(item)) color = isDark.value ? '#BDBDBD' : cfg.value.TabularViewer_AckTxtColor
  else if (item.priority === 0) color = colorOfPriority(0)
  else color = isDark.value ? '#82B1FF' : cfg.value.TabularViewer_AlmTxtColor
  return {
    style: {
      color,
      fontWeight: item.priority === 0 ? 'bold' : 'normal',
      cursor: 'pointer',
    },
  }
}
function priorityPillStyle(item) {
  return {
    backgroundColor: colorOfPriority(item.priority),
    color: 'black',
    opacity: isRowAcked(item) ? 0.35 : 1,
  }
}

const rowHeight = computed(() => Math.round(fontScale.value + 5))
const rootStyle = computed(() => ({
  '--alm-font-size': fontScale.value + 'px',
  '--row-height': rowHeight.value + 'px',
  fontFamily: cfg.value.TabularViewer_Font,
}))

// --- realtime feed -----------------------------------------------------------
async function pollTick() {
  if (disposed || !isRealtime.value) return
  const id = ++requestId
  fanState.value = '.'
  try {
    const r = await opc.readAlarms({ mode: 'realtime' }, cfg.value)
    if (disposed || id !== requestId) return
    rows.value = r.alarms
    lastSuccess = Date.now()
    failed.value = false
    lastUpdate.value = new Date().toLocaleTimeString()
    await pollBeepStatus(id)
  } catch (e) {
    if (disposed || id !== requestId) return
    fanState.value = 'E'
  }
  if (disposed || id !== requestId) return
  // failsafe: blank list if the server stalled
  if (Date.now() - lastSuccess > 15000) {
    rows.value = []
    failed.value = true
  }
  clearTimeout(pollTimer) // ack-all may have scheduled a poll while this one ran
  pollTimer = setTimeout(pollTick, (cfg.value.TabularViewer_RefreshTime * 1000) / 1.5)
}

async function pollBeepStatus(id) {
  const pts = await opc.readPoints([BEEP_POINTKEY], true, cfg.value)
  if (disposed || id !== requestId) return
  const beep = pts.find((p) => p.key === BEEP_POINTKEY)
  if (!beep) return
  alarmBeepType.value = beep.beepType || 0
  let on = beep.valueRaw ? 1 : 0
  if (on && Array.isArray(beep.beepGroup1List)) {
    const mine = getUserGroup1List()
    if (mine.length > 0 && beep.beepGroup1List.filter((v) => mine.includes(v)).length === 0) on = 0
  }
  alarmBeep.value = on
}

// --- historical query --------------------------------------------------------
// datetime-local input value (local time) of a date
function toInputValue(d) {
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}
function setDefaultPeriod() {
  const now = new Date()
  histTo.value = toInputValue(now)
  histFrom.value = toInputValue(new Date(now.getTime() - 24 * 3600 * 1000))
}
async function histSearch() {
  const begin = new Date(histFrom.value)
  const end = new Date(histTo.value)
  if (isNaN(begin.getTime()) || isNaN(end.getTime())) return
  const id = ++requestId
  histLoading.value = true
  fanState.value = '.'
  try {
    const r = await opc.readAlarms(
      {
        mode: 'historical',
        timeBegin: begin,
        timeEnd: end,
        text: (histText.value || '').trim(),
        limit: cfg.value.EventsViewer_MaxHistoricalEvents || 10000,
      },
      cfg.value,
      60000
    )
    if (disposed || id !== requestId) return
    rows.value = r.alarms
    histLimited.value = r.limited
    failed.value = false
    lastUpdate.value = ''
  } catch (e) {
    if (disposed || id !== requestId) return
    fanState.value = 'E'
    failed.value = true
  } finally {
    if (!disposed && id === requestId) histLoading.value = false
  }
}

function onModeChange(nextMode) {
  const wasRealtime = isRealtime.value
  mode.value = nextMode
  if (wasRealtime === isRealtime.value) return
  requestId++
  rows.value = []
  failed.value = false
  histLoading.value = false
  histLimited.value = false
  lastUpdate.value = ''
  clearTimeout(pollTimer)
  if (isRealtime.value) {
    pollTimer = setTimeout(pollTick, 50)
  } else {
    if (!histFrom.value || !histTo.value) setDefaultPeriod()
  }
  setBeforeUnload(isRealtime.value)
}

// --- row interactions --------------------------------------------------------
function onRowClick(event, { item }) {
  const forceInfo = event.altKey || event.shiftKey || mode.value === 'historical'
  const acked = isRowAcked(item)
  if (!forceInfo && !acked && userHasRight('ackAlarms')) {
    const aggregated = mode.value === 'aggregated'
    opc.writeAck({
      pointId: item.pointKey,
      alarmId: aggregated ? undefined : item._id,
      action: OpcAcknowledge.AckOneAlarm | OpcAcknowledge.SilenceBeep,
    })
    if (aggregated) {
      for (const row of rows.value) {
        if (row.pointKey === item.pointKey) row.acked = true
      }
    } else {
      item.acked = true
    }
    return
  }
  if (forceInfo || acked) {
    infoKey.value = item.pointKey
    infoOpen.value = true
  }
}
function ackAll() {
  opc.writeAck({ pointId: 0, action: OpcAcknowledge.AckAllAlarms | OpcAcknowledge.SilenceBeep })
  rows.value.forEach((r) => (r.acked = true))
  clearTimeout(pollTimer)
  pollTimer = setTimeout(pollTick, 1500)
}
function silenceBeep() {
  opc.writeAck({ pointId: 0, action: OpcAcknowledge.SilenceBeep })
  alarmBeep.value = 0
}
function toggleBeep() {
  beepOn.value = !beepOn.value
  if (beepOn.value && nonCriticalSound.value) nonCriticalSound.value.play().catch(() => {})
}

// --- filter chips (saved per user, shared with the former viewer: almFilter) --
function toggleFilterOut(id) {
  filterOut[id] = !filterOut[id]
  persistFilters()
}
function selectAllStations(filterIt) {
  stationChips.value.forEach((s) => (filterOut[s.id] = filterIt))
  persistFilters()
}
function applyCustomSelector(sel) {
  stationChips.value.forEach((s) => (filterOut[s.id] = !sel.group1List.includes(s.id)))
  persistFilters()
}
function persistFilters() {
  setUserSetting('almFilter', JSON.stringify(filterOut))
}
function loadFilters() {
  try {
    const f = JSON.parse(getUserSetting('almFilter') || 'null')
    if (f && typeof f === 'object')
      for (const [k, v] of Object.entries(f)) if (typeof v === 'boolean') filterOut[k] = v
  } catch (e) {
    /* ignore */
  }
}

// --- per-user settings: mode, visible columns, historical filter -------------
const PREFS_KEY = 'alm_settings'
const prefsReady = ref(false)
function savePrefs() {
  if (!prefsReady.value) return
  setUserSetting(
    PREFS_KEY,
    JSON.stringify({
      mode: mode.value,
      visibleColumns: selectedColumns.value,
      unackedOnly: unackedOnly.value,
      histText: histText.value || '',
    })
  )
}
watch([mode, selectedColumns, unackedOnly, histText], savePrefs)
function loadPrefs() {
  try {
    const p = JSON.parse(getUserSetting(PREFS_KEY) || 'null')
    if (!p || typeof p !== 'object') return
    if (['realtime', 'aggregated', 'historical'].includes(p.mode)) mode.value = p.mode
    if (Array.isArray(p.visibleColumns)) {
      selectedColumns.value = optionalColumns.value.map(({ key }) => key).filter((key) => p.visibleColumns.includes(key))
    } else if (p.showKeyCols === true) {
      selectedColumns.value = optionalColumns.value.map(({ key }) => key)
    }
    if (typeof p.unackedOnly === 'boolean') unackedOnly.value = p.unackedOnly
    if (typeof p.histText === 'string') histText.value = p.histText.slice(0, 200)
  } catch (e) {
    /* ignore */
  }
}

function fontInc(dir) {
  const v = fontScale.value + dir
  if (v >= 10 && v <= 30) {
    fontScale.value = v
    setUserSetting('tab_fontsize', v)
  }
}
function copyGrid() {
  copyRowsToClipboard(headers.value, visibleRows.value)
}

// --- keyboard ----------------------------------------------------------------
function onKeydown(e) {
  const tag = (e.target && e.target.tagName) || ''
  const inInput = tag === 'INPUT' || tag === 'TEXTAREA'
  if (e.key === 'F8' && isRealtime.value) {
    ackAll()
    e.preventDefault()
  } else if (e.key === 'F9') {
    silenceBeep()
    e.preventDefault()
  } else if (e.key === 'Escape') {
    infoOpen.value = false
  } else if (!inInput && (e.key === '+' || e.key === 'Add')) {
    fontInc(1)
  } else if (!inInput && (e.key === '-' || e.key === 'Subtract')) {
    fontInc(-1)
  }
}

function beforeUnloadGuard(e) {
  e.preventDefault()
  e.returnValue = ''
}
function setBeforeUnload(on) {
  window.removeEventListener('beforeunload', beforeUnloadGuard)
  if (on) window.addEventListener('beforeunload', beforeUnloadGuard)
}

// --- lifecycle ---------------------------------------------------------------
onMounted(async () => {
  onAccessDenied(() => router.push('/login'))
  cfg.value = await loadViewersConfig()
  if (disposed) return
  {
    const fs = parseInt(getUserSetting('tab_fontsize'))
    if (!isNaN(fs) && fs >= 10 && fs <= 30) fontScale.value = fs
  }
  group1List.value = (await opc.getGroup1List()).filter((s) => s !== '')
  if (disposed) return
  loadFilters()
  loadPrefs()
  if (route.value.query.mode === 'historical') mode.value = 'historical'
  setDefaultPeriod()
  prefsReady.value = true
  window.addEventListener('keydown', onKeydown)
  setBeforeUnload(isRealtime.value)
  beepTimer = setInterval(() => {
    if (beepOn.value && alarmBeep.value && isRealtime.value) {
      const el = alarmBeepType.value === 2 ? criticalSound.value : nonCriticalSound.value
      if (el) el.play().catch(() => {})
    }
  }, 1500)
  if (isRealtime.value) pollTick()
  else histSearch()
})

onUnmounted(() => {
  disposed = true
  clearTimeout(pollTimer)
  clearInterval(beepTimer)
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('beforeunload', beforeUnloadGuard)
})
</script>

<style scoped>
.alarms-root {
  height: 100vh;
  display: flex;
  flex-direction: column;
  font-size: var(--alm-font-size, 13px);
  user-select: none;
}
.alarms-root > :not(.alarms-grid) {
  flex: 0 0 auto;
}
.alarms-grid {
  flex: 1 1 0;
  min-height: 0;
  font-size: var(--alm-font-size, 13px);
}
.alarms-grid :deep(td) {
  padding: 0 4px !important;
  line-height: 1;
  height: var(--row-height) !important;
}
/* the virtual scroller's spacer rows size themselves */
.alarms-grid :deep(tbody td[colspan][style]) {
  height: 0 !important;
  padding: 0 !important;
}
.alarms-grid :deep(th) {
  padding: 0 4px !important;
  height: var(--row-height) !important;
}
.alarms-grid :deep(td.descr-cell) {
  max-width: 0;
  overflow: hidden;
}
.alarms-grid :deep(th.descr-cell) {
  min-width: 120px;
}
.alarms-grid :deep(td.nowrap-cell),
.alarms-grid :deep(th.nowrap-cell),
.alarms-grid :deep(th.nowrap-cell .v-data-table-header__content) {
  white-space: nowrap;
}
.alarms-grid :deep(tr:nth-child(even) td) {
  background-color: rgba(128, 128, 128, 0.05) !important;
}
.fan {
  font-family: courier, monospace;
  font-weight: bold;
}
.chips-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 2px;
  max-height: 72px;
  overflow-y: auto;
}
.chip-group {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 3px;
  margin-left: 6px;
}
.alm-chip {
  cursor: pointer;
  box-shadow: 1px 1px 1px #666;
  min-width: 48px;
  border-radius: 4px;
  border: 1px solid #777;
  padding: 2px 4px;
  font-size: 13px;
  text-align: center;
  line-height: 1;
}
.alm-chip .badge {
  display: inline-block;
  font-size: 11px;
  border-radius: 15px;
  border: 2px solid silver;
  background-color: silver;
  margin: 1px;
  min-width: 16px;
  color: black;
}
.alm-chip .badge.ack {
  filter: contrast(0.7);
}
.qualif {
  font-size: smaller;
}
.pill {
  border-radius: 10px;
  padding: 0 6px;
  font-weight: bold;
}
.state {
  font-size: smaller;
  text-transform: uppercase;
}
.state-active {
  font-weight: bold;
}
</style>
