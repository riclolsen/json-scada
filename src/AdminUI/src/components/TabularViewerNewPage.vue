<template>
  <v-container fluid class="pa-1 tabular-root" :style="rootStyle">
    <!-- Toolbar -->
    <v-toolbar density="compact" color="surface" class="mb-1 px-2" flat>
      <template v-if="!isAlarmsViewer">
        <span class="text-caption mr-1">{{ $t('tabularViewer.station') }}</span>
        <v-select
          v-model="filterStation"
          :items="group1List"
          density="compact"
          variant="outlined"
          hide-details
          class="mr-2"
          style="max-width: 200px"
          @update:model-value="onStationChange"
        ></v-select>
        <span class="text-caption mr-1">{{ $t('tabularViewer.bay') }}</span>
        <v-select
          v-model="filterBay"
          :items="bayList"
          density="compact"
          variant="outlined"
          hide-details
          class="mr-2"
          style="max-width: 200px"
          @update:model-value="onFilterChange"
        ></v-select>
        <v-checkbox
          v-model="showOnlyCommandable"
          :label="$t('tabularViewer.showCommandable')"
          density="compact"
          hide-details
          class="mr-2"
        ></v-checkbox>
        <v-checkbox
          v-model="showOnlyAbnormal"
          :label="$t('tabularViewer.showAbnormal')"
          density="compact"
          hide-details
          class="mr-2"
        ></v-checkbox>
      </template>

      <v-btn
        v-if="isAlarmsViewer"
        size="small"
        variant="tonal"
        color="warning"
        class="mr-2"
        :disabled="!userHasRight('ackAlarms')"
        @click="ackAll"
      >
        {{ $t('tabularViewer.ackAll') }} (F8)
      </v-btn>

      <v-btn icon size="small" variant="text" :title="$t('tabularViewer.toggleKeyCols')" @click="showKeyCols = !showKeyCols">
        <v-icon>mdi-key</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('tabularViewer.fontUp')" @click="fontInc(1)">
        <v-icon>mdi-format-font-size-increase</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('tabularViewer.fontDown')" @click="fontInc(-1)">
        <v-icon>mdi-format-font-size-decrease</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('tabularViewer.copy')" @click="copyGrid">
        <v-icon>mdi-content-copy</v-icon>
      </v-btn>
      <v-btn
        v-if="isAlarmsViewer"
        icon
        size="small"
        variant="text"
        :title="$t('tabularViewer.beep')"
        @click="toggleBeep"
      >
        <v-icon>{{ beepOn ? 'mdi-volume-high' : 'mdi-volume-off' }}</v-icon>
      </v-btn>
      <v-btn
        v-if="alarmBeep && isAlarmsViewer"
        icon
        size="small"
        variant="text"
        color="error"
        :title="$t('tabularViewer.silenceBeep')"
        @click="silenceBeep"
      >
        <v-icon>mdi-bell-off</v-icon>
      </v-btn>

      <v-spacer></v-spacer>
      <v-checkbox
        v-model="blockUpdates"
        :label="$t('tabularViewer.blockUpdates')"
        density="compact"
        hide-details
        class="mr-2"
      ></v-checkbox>
      <span class="text-caption mr-2">
        <b>{{ visibleRows.length }}</b> / {{ rows.length }} — {{ lastUpdate }}
      </span>
      <span class="text-caption fan">{{ fanState }}</span>
    </v-toolbar>

    <!-- Alarms-viewer filter chips -->
    <div v-if="isAlarmsViewer" class="chips-bar mb-1">
      <v-btn size="x-small" variant="tonal" class="mr-1" @click="selectAllStations(true)">
        {{ $t('tabularViewer.selectAll') }}
      </v-btn>
      <v-btn size="x-small" variant="tonal" class="mr-2" @click="selectAllStations(false)">
        {{ $t('tabularViewer.unselectAll') }}
      </v-btn>
      <v-btn
        v-for="sel in cfg.TabularViewer_CustomFiltersSelectors"
        :key="sel.name"
        size="x-small"
        variant="tonal"
        class="mr-1"
        @click="applyCustomSelector(sel)"
      >
        {{ sel.name }}
      </v-btn>
      <span class="chip-group">
        <span
          v-for="pri in priorityChips"
          :key="'pri' + pri.id"
          class="alm-chip"
          :style="{ opacity: filterOut[pri.id] ? 0.25 : 1 }"
          @click="toggleFilterOut(pri.id)"
        >
          {{ pri.id }}
          <span class="badge" :style="{ backgroundColor: colorOfPriority(pri.minNack), borderColor: colorOfPriority(pri.minNack) }">{{ pri.nack }}</span>
          <span class="badge ack" :style="{ backgroundColor: colorOfPriority(pri.minAck), borderColor: colorOfPriority(pri.minAck) }">{{ pri.ack }}</span>
        </span>
      </span>
      <span class="chip-group">
        <span
          v-for="st in stationChips"
          :key="'st' + st.id"
          class="alm-chip"
          :style="{ opacity: filterOut[st.id] ? 0.25 : 1 }"
          @click="toggleFilterOut(st.id)"
        >
          {{ st.id }}
          <span class="badge" :style="{ backgroundColor: colorOfPriority(st.minNack), borderColor: colorOfPriority(st.minNack) }">{{ st.nack }}</span>
          <span class="badge ack" :style="{ backgroundColor: colorOfPriority(st.minAck), borderColor: colorOfPriority(st.minAck) }">{{ st.ack }}</span>
        </span>
      </span>
    </div>

    <!-- Data grid -->
    <v-data-table-virtual
      :headers="headers"
      :items="visibleRows"
      :no-data-text="failed ? $t('tabularViewer.serverFail') : $t('tabularViewer.noData')"
      density="compact"
      fixed-header
      :height="gridHeight"
      :item-height="rowHeight"
      item-value="key"
      class="tabular-grid"
      :style="{ '--row-height': rowHeight + 'px' }"
      :row-props="rowProps"
      @click:row="onRowClick"
    >
      <template #[`item.value`]="{ item }">
        <span :style="{ display: 'block', textAlign: (item.flags & 0x20) ? 'right' : 'left', paddingRight: (item.flags & 0x20) ? '20px' : '0' }">
          {{ item.valueStr }}
        </span>
      </template>
      <template #[`item.qualifier`]="{ item }">
        <span :class="['qualif', qualifPillClass(item)]" :style="qualifPillStyle(item)">
          {{ item.qualifier }}
        </span>
      </template>
      <template #[`item.station`]="{ item }">
        <span :style="isAlarmsViewer ? { color: colorOfStation(item.station) } : {}">
          {{ item.station }}
        </span>
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
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { loadViewersConfig, getViewersConfig } from '../lib/viewersConfig'
import * as opc from '../lib/opcClient'
import { onAccessDenied } from '../lib/opcClient'
import PointInfoDialog from './PointInfoDialog.vue'
import {
  OpcAcknowledge,
  Flags,
  BEEP_POINTKEY,
  CNTUPDATES_POINTKEY,
} from '../lib/opcCodes'
import {
  userHasRight,
  storageAvailable,
  copyRowsToClipboard,
  getUserGroup1List,
} from '../lib/viewerHelpers'

const props = defineProps({
  mode: { type: String, default: '' },
})

const { t } = useI18n()
const router = useRouter()
const route = router.currentRoute

const isAlarmsViewer = computed(
  () => props.mode === 'ALARMS_VIEWER' || route.value.query.mode === 'ALARMS_VIEWER'
)

// --- config & state ----------------------------------------------------------
const cfg = ref(getViewersConfig())
const rows = ref([])
const pointsById = new Map()
const group1List = ref([''])
const bayList = ref([''])
const filterStation = ref('')
const filterBay = ref('')
const showOnlyCommandable = ref(false)
const showOnlyAbnormal = ref(false)
const blockUpdates = ref(false)
const filterOut = reactive({})
const alarmInfo = ref({})
const beepOn = ref(false)
const alarmBeep = ref(0)
const alarmBeepType = ref(0)
const lastUpdate = ref('')
const fanState = ref('(!)')
const fontScale = ref(13)
const showKeyCols = ref(false)
const failed = ref(false)
const gridHeight = ref(window.innerHeight - 110)

const criticalSound = ref(null)
const nonCriticalSound = ref(null)

let pollTimer = null
let beepTimer = null
let lastSuccess = Date.now()

// point info/command dialog (shared component)
const infoOpen = ref(false)
const infoKey = ref(0)

// --- headers -----------------------------------------------------------------
const headers = computed(() => {
  const h = []
  if (showKeyCols.value) {
    h.push({ title: t('tabularViewer.columns.pointNum'), key: 'key', width: 70, sortable: false })
    h.push({ title: t('tabularViewer.columns.id'), key: 'tag', width: 230, sortable: false })
  }
  h.push({ title: t('tabularViewer.columns.location'), key: 'station', width: 90, sortable: false })
  h.push({ title: t('tabularViewer.columns.description'), key: 'descr', sortable: false })
  h.push({ title: t('tabularViewer.columns.value'), key: 'value', width: 180, sortable: false })
  h.push({ title: t('tabularViewer.columns.qualifier'), key: 'qualifier', width: 70, sortable: false })
  h.push({ title: t('tabularViewer.columns.alarmTime'), key: 'alarmTime', width: 170, sortable: false })
  h.push({ title: t('tabularViewer.columns.sourceTime'), key: 'fieldTime', width: 200, sortable: false })
  return h
})

// --- visible rows (client-side filters) --------------------------------------
const visibleRows = computed(() => {
  return rows.value.filter((r) => {
    if (isAlarmsViewer.value) {
      if (filterOut[r.station]) return false
      if (filterOut[String(r.priority)]) return false
    }
    if (showOnlyCommandable.value && !r.commandKey) return false
    if (showOnlyAbnormal.value && !(r.flags & (Flags.ABNORMAL | Flags.ALARMED))) return false
    return true
  })
})

// --- row & cell styling ------------------------------------------------------
function brighten(color) {
  // lighten an rgb()/named color toward white (~40%); fallback returns input
  const ctx = document.createElement('canvas').getContext('2d')
  ctx.fillStyle = color
  const hex = ctx.fillStyle // normalized to #rrggbb
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  const mix = (c) => Math.round(c + (255 - c) * 0.45)
  return `rgb(${mix(r)},${mix(g)},${mix(b)})`
}

function rowProps({ item }) {
  const q = item.qualifier
  const pri = parseInt(q.charAt(0))
  let color
  const acked = q.indexOf('L') === -1
  if (acked) {
    color = q.indexOf('I') !== -1 ? brighten(cfg.value.TabularViewer_AckTxtColor) : cfg.value.TabularViewer_AckTxtColor
  } else {
    color = pri === 0 ? cfg.value.ColorOfPriority[0] : cfg.value.TabularViewer_AlmTxtColor
  }
  if (q.indexOf('F') !== -1) color = cfg.value.TabularViewer_FailTxtColor
  return {
    style: {
      color,
      fontWeight: pri === 0 ? 'bold' : 'normal',
      cursor: 'pointer',
    },
  }
}

function qualifPillClass(item) {
  const q = item.qualifier
  return q.indexOf('L') !== -1 || q.indexOf('P') !== -1 ? 'pill' : ''
}
function qualifPillStyle(item) {
  const q = item.qualifier
  const pri = parseInt(q.charAt(0))
  if (q.indexOf('L') !== -1 || q.indexOf('P') !== -1) {
    return {
      backgroundColor: colorOfPriority(pri),
      borderRadius: '10px',
      color: 'black',
      textAlign: 'center',
      padding: '0 6px',
      opacity: q.indexOf('L') !== -1 ? 1 : 0.3,
    }
  }
  return {}
}

function colorOfPriority(pri) {
  const arr = cfg.value.ColorOfPriority
  const i = isNaN(pri) ? arr.length - 1 : pri
  return arr[Math.min(i, arr.length - 1)]
}
const stationIndex = computed(() => {
  const m = {}
  ;[...group1List.value].sort().forEach((s, i) => (m[s] = i))
  return m
})
function colorOfStation(station) {
  const arr = cfg.value.ColorOfSubstation
  const idx = stationIndex.value[station] || 0
  return arr[idx % arr.length]
}

const rowHeight = computed(() => Math.round(fontScale.value + 5))

const rootStyle = computed(() => ({
  '--tab-font-size': fontScale.value + 'px',
  '--row-height': rowHeight.value + 'px',
  fontFamily: cfg.value.TabularViewer_Font,
}))

// --- data fetch / polling ----------------------------------------------------
async function pollTick() {
  // Tabular mode requires a station filter before querying (legacy mudaFiltro).
  const skip = !isAlarmsViewer.value && filterStation.value === ''
  if (!blockUpdates.value && !skip) {
    fanState.value = '.'
    try {
      const data = await opc.readFiltered(
        {
          station: isAlarmsViewer.value ? '' : filterStation.value,
          bay: isAlarmsViewer.value ? '' : filterBay.value,
          onlyAlarms: isAlarmsViewer.value,
        },
        cfg.value
      )
      rows.value = data
      data.forEach((r) => pointsById.set(r.key, r))
      lastSuccess = Date.now()
      failed.value = false
      if (!isAlarmsViewer.value) recomputeBayList()
      if (isAlarmsViewer.value) recomputeAlarmInfo()
      if (isAlarmsViewer.value) await pollBeepStatus()
      lastUpdate.value = new Date().toLocaleTimeString()
    } catch (e) {
      fanState.value = 'E'
    }
  } else if (skip) {
    rows.value = []
    lastSuccess = Date.now()
  }
  // failsafe: blank grid if server stalled
  if (Date.now() - lastSuccess > 15000) {
    rows.value = []
    failed.value = true
  }
  const base = cfg.value.TabularViewer_RefreshTime * 1000
  const interval = isAlarmsViewer.value ? base / 1.5 : base
  pollTimer = setTimeout(pollTick, interval)
}

function recomputeBayList() {
  const set = new Set([''])
  rows.value.forEach((r) => {
    const m = (r.descr || '').split(' | ')[0]
    if (m) set.add(m)
  })
  bayList.value = [...set].sort()
}

function recomputeAlarmInfo() {
  const info = {}
  const ensure = (id, isSubst) => {
    if (!info[id]) info[id] = { ack: 0, nack: 0, minAck: 10, minNack: 10, isSubst }
    return info[id]
  }
  rows.value.forEach((r) => {
    const pri = r.priority
    const nack = r.qualifier.indexOf('L') !== -1
    const p = ensure(String(pri), false)
    const s = ensure(r.station, true)
    if (nack) {
      p.nack++
      s.nack++
      if (pri < s.minNack) s.minNack = pri
      p.minNack = pri
    } else {
      p.ack++
      s.ack++
      if (pri < s.minAck) s.minAck = pri
      p.minAck = pri
    }
  })
  alarmInfo.value = info
}

const stationChips = computed(() =>
  Object.keys(alarmInfo.value)
    .filter((k) => alarmInfo.value[k].isSubst)
    .sort()
    .map((k) => ({ id: k, ...alarmInfo.value[k] }))
)
const priorityChips = computed(() =>
  Object.keys(alarmInfo.value)
    .filter((k) => !alarmInfo.value[k].isSubst)
    .sort()
    .map((k) => ({ id: k, ...alarmInfo.value[k] }))
)

async function pollBeepStatus() {
  const pts = await opc.readPoints([BEEP_POINTKEY, CNTUPDATES_POINTKEY], true, cfg.value)
  const beep = pts.find((p) => p.key === BEEP_POINTKEY)
  if (beep) {
    alarmBeepType.value = beep.beepType || 0
    let on = beep.valueRaw ? 1 : 0
    if (on && Array.isArray(beep.beepGroup1List)) {
      const mine = getUserGroup1List()
      if (mine.length > 0) {
        const inter = beep.beepGroup1List.filter((v) => mine.includes(v))
        if (inter.length === 0) on = 0
      }
    }
    alarmBeep.value = on
  }
}

// --- row interactions --------------------------------------------------------
function onRowClick(event, { item }) {
  const ctrl = event.ctrlKey || event.button === 1
  const forceInfo = event.altKey || event.shiftKey
  const ack = (isAlarmsViewer.value || ctrl) && !forceInfo
  if (ack) {
    if (item.flags & Flags.ALARMED) {
      opc.writeAck({ pointId: item.key, action: OpcAcknowledge.AckOneAlarm | OpcAcknowledge.SilenceBeep })
      item.qualifier = item.qualifier.replace('L', '')
    }
  } else {
    infoKey.value = item.key
    infoOpen.value = true
  }
}

// --- alarm ack / beep --------------------------------------------------------
function ackAll() {
  opc.writeAck({ pointId: 0, action: OpcAcknowledge.AckAllAlarms | OpcAcknowledge.SilenceBeep })
}
function silenceBeep() {
  opc.writeAck({ pointId: 0, action: OpcAcknowledge.SilenceBeep })
  alarmBeep.value = 0
}
function toggleBeep() {
  beepOn.value = !beepOn.value
  if (beepOn.value && nonCriticalSound.value) nonCriticalSound.value.play().catch(() => {})
}

// --- alarms-viewer filter chips ----------------------------------------------
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
  if (storageAvailable('localStorage'))
    localStorage.setItem('almFilter', JSON.stringify(filterOut))
}
function loadFilters() {
  if (!storageAvailable('localStorage')) return
  const s = localStorage.getItem('almFilter')
  if (s) {
    try {
      Object.assign(filterOut, JSON.parse(s))
    } catch (e) {
      /* ignore */
    }
  }
}

// --- filters / font ----------------------------------------------------------
function onStationChange() {
  filterBay.value = ''
  onFilterChange()
}
function onFilterChange() {
  rows.value = []
}
function fontInc(dir) {
  const v = fontScale.value + dir
  if (v >= 10 && v <= 30) {
    fontScale.value = v
    if (storageAvailable('localStorage')) localStorage.setItem('tab_fontsize', v)
  }
}
function copyGrid() {
  copyRowsToClipboard(headers.value, visibleRows.value)
}

// --- keyboard ----------------------------------------------------------------
function onKeydown(e) {
  const tag = (e.target && e.target.tagName) || ''
  const inInput = tag === 'INPUT' || tag === 'TEXTAREA'
  if (e.key === 'F8') {
    ackAll()
    e.preventDefault()
  } else if (e.key === 'F9') {
    silenceBeep()
    e.preventDefault()
  } else if (e.key === 'Escape') {
    infoOpen.value = false
  } else if (!inInput && e.key === '1') {
    showOnlyCommandable.value = !showOnlyCommandable.value
  } else if (!inInput && e.key === '2') {
    showOnlyAbnormal.value = !showOnlyAbnormal.value
  } else if (e.key === '+' || e.key === 'Add') {
    fontInc(1)
  } else if (e.key === '-' || e.key === 'Subtract') {
    fontInc(-1)
  }
}

function onResize() {
  gridHeight.value = window.innerHeight - (isAlarmsViewer.value ? 160 : 110)
}

function beforeUnloadGuard(e) {
  e.preventDefault()
  e.returnValue = ''
}

// --- lifecycle ---------------------------------------------------------------
onMounted(async () => {
  onAccessDenied(() => router.push('/login'))
  cfg.value = await loadViewersConfig()
  if (storageAvailable('localStorage')) {
    const fs = parseInt(localStorage.getItem('tab_fontsize'))
    if (!isNaN(fs) && fs >= 10 && fs <= 30) fontScale.value = fs
  }
  group1List.value = await opc.getGroup1List()
  // initial filter from query (tabular mode)
  if (!isAlarmsViewer.value) {
    if (route.value.query.subst) filterStation.value = route.value.query.subst
    if (route.value.query.bay) filterBay.value = route.value.query.bay
  } else {
    loadFilters()
    window.addEventListener('beforeunload', beforeUnloadGuard)
    beepTimer = setInterval(() => {
      if (beepOn.value && alarmBeep.value) {
        const el = alarmBeepType.value === 2 ? criticalSound.value : nonCriticalSound.value
        if (el) el.play().catch(() => {})
      }
    }, 1500)
  }
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('resize', onResize)
  onResize()
  pollTick()
})

onUnmounted(() => {
  clearTimeout(pollTimer)
  clearInterval(beepTimer)
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('resize', onResize)
  window.removeEventListener('beforeunload', beforeUnloadGuard)
})
</script>

<style scoped>
.tabular-root {
  height: calc(100vh - 60px);
  font-size: var(--tab-font-size, 13px);
  user-select: none;
}
.tabular-grid {
  font-size: var(--tab-font-size, 13px);
}
.tabular-grid :deep(td) {
  padding: 0 4px !important;
  line-height: 1;
  height: var(--row-height) !important;
  user-select: none;
}
.tabular-grid :deep(th) {
  padding: 0 4px !important;
  height: var(--row-height) !important;
  user-select: none;
}
.tabular-grid :deep(tr:nth-child(even) td) {
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
  overflow-x: auto;
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
</style>
