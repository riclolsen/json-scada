<template>
  <v-container fluid class="pa-1 events-root" :style="rootStyle">
    <!-- Toolbar -->
    <v-toolbar density="compact" color="surface" class="mb-1 px-2" flat>
      <v-btn-toggle v-model="mode" mandatory density="compact" variant="outlined" divided class="mr-2" @update:model-value="onModeChange">
        <v-btn :value="0" size="small">{{ $t('eventsViewer.modes.normal') }}</v-btn>
        <v-btn :value="1" size="small">{{ $t('eventsViewer.modes.aggregated') }}</v-btn>
        <v-btn :value="2" size="small">{{ $t('eventsViewer.modes.panic') }}</v-btn>
        <v-btn :value="3" size="small">{{ $t('eventsViewer.modes.frozen') }}</v-btn>
        <v-btn :value="4" size="small">{{ $t('eventsViewer.modes.historical') }}</v-btn>
      </v-btn-toggle>

      <v-btn
        v-if="gpsToggleVisible"
        size="small"
        variant="tonal"
        class="mr-2"
        :title="$t('eventsViewer.toolbar.gpsTime')"
        @click="toggleGpsTime"
      >
        <v-icon start>{{ gpsTime ? 'mdi-satellite-variant' : 'mdi-clock-outline' }}</v-icon>
        {{ gpsTime ? $t('eventsViewer.toolbar.gpsTime') : $t('eventsViewer.toolbar.localTime') }}
      </v-btn>

      <!-- Station filter -->
      <v-menu v-if="cfg.EventsViewer_AllowFilter && mode !== 4" :close-on-content-click="false">
        <template #activator="{ props }">
          <v-btn icon size="small" variant="text" v-bind="props" :title="$t('eventsViewer.toolbar.filter')">
            <v-icon :color="hasStationFilter ? 'primary' : undefined">mdi-filter</v-icon>
          </v-btn>
        </template>
        <v-card max-height="400" style="overflow-y: auto">
          <v-list density="compact">
            <v-list-item v-for="st in group1List" :key="st">
              <v-checkbox v-model="stationChecked[st]" :label="st" density="compact" hide-details></v-checkbox>
            </v-list-item>
          </v-list>
        </v-card>
      </v-menu>

      <v-btn size="small" variant="tonal" color="warning" class="mr-1" :disabled="!userHasRight('ackEvents')" @click="ackAll">
        {{ $t('eventsViewer.toolbar.ackAll') }} (F8)
      </v-btn>
      <v-btn size="small" variant="tonal" color="error" class="mr-1" :disabled="!userHasRight('ackEvents')" @click="removeAll">
        {{ $t('eventsViewer.toolbar.removeAll') }} (F2)
      </v-btn>
      <v-btn
        v-if="alarmBeep"
        icon
        size="small"
        variant="text"
        color="error"
        :title="$t('eventsViewer.toolbar.silence')"
        @click="silenceBeep"
      >
        <v-icon>mdi-bell-off</v-icon>
      </v-btn>

      <v-btn icon size="small" variant="text" :title="$t('eventsViewer.toolbar.toggleKeyCols')" @click="showKeyCols = !showKeyCols">
        <v-icon>mdi-key</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('eventsViewer.toolbar.fontUp')" @click="fontInc(1)">
        <v-icon>mdi-format-font-size-increase</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('eventsViewer.toolbar.fontDown')" @click="fontInc(-1)">
        <v-icon>mdi-format-font-size-decrease</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('eventsViewer.toolbar.copy')" @click="copyGrid">
        <v-icon>mdi-content-copy</v-icon>
      </v-btn>

      <v-spacer></v-spacer>
      <span class="text-caption mr-2"><b>{{ rows.length }}</b> {{ $t('eventsViewer.events') }}{{ filterText }}</span>
      <span class="text-caption fan">{{ fanState }}</span>
    </v-toolbar>

    <!-- Historical controls -->
    <v-toolbar v-if="mode === 4" density="compact" color="surface" class="mb-1 px-2" flat>
      <span class="text-caption mr-1">{{ $t('eventsViewer.hist.date') }}</span>
      <v-text-field v-model="histDate" type="date" density="compact" variant="outlined" hide-details class="mr-2" style="max-width: 170px"></v-text-field>
      <span class="text-caption mr-1">{{ $t('eventsViewer.hist.time') }}</span>
      <v-text-field v-model="histTime" type="time" step="1" density="compact" variant="outlined" hide-details class="mr-2" style="max-width: 140px"></v-text-field>
      <span class="text-caption mr-1">{{ $t('eventsViewer.hist.filter') }}</span>
      <v-text-field v-model="histFilter" density="compact" variant="outlined" hide-details class="mr-2" style="max-width: 160px"></v-text-field>
      <v-btn size="small" color="primary" variant="tonal" @click="histSearch">{{ $t('eventsViewer.toolbar.find') }}</v-btn>
    </v-toolbar>

    <!-- Grid -->
    <v-data-table-virtual
      :headers="headers"
      :items="visibleRows"
      :no-data-text="failed ? $t('eventsViewer.serverFail') : $t('eventsViewer.noData')"
      density="compact"
      fixed-header
      :height="gridHeight"
      :item-height="rowHeight"
      item-value="rowId"
      class="events-grid"
      :style="{ '--row-height': rowHeight + 'px' }"
      :row-props="rowProps"
      @click:row="onRowClick"
    >
      <template #[`item.qualifier`]="{ item }">
        <span class="qualif" :style="qualifPillStyle(item)">{{ item.qualifier }}</span>
      </template>
      <template #[`item.station`]="{ item }">
        <span :style="{ color: colorOfStation(item.station) }">{{ item.station }}</span>
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
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useTheme } from 'vuetify'
import { loadViewersConfig, getViewersConfig } from '../lib/viewersConfig'
import * as opc from '../lib/opcClient'
import { onAccessDenied } from '../lib/opcClient'
import PointInfoDialog from './PointInfoDialog.vue'
import { OpcAcknowledge, BEEP_POINTKEY, CNTUPDATES_POINTKEY } from '../lib/opcCodes'
import {
  userHasRight,
  storageAvailable,
  readCookie,
  isInt,
  copyRowsToClipboard,
  getUserGroup1List,
} from '../lib/viewerHelpers'

const props = defineProps({
  mode: { type: Number, default: 0 },
})

const { t } = useI18n()
const router = useRouter()
const route = router.currentRoute
const vuetifyTheme = useTheme()

// --- state -------------------------------------------------------------------
const cfg = ref(getViewersConfig())
const rows = ref([])
const group1List = ref([])
const stationChecked = reactive({})
const mode = ref(props.mode || 0)
const gpsTime = ref(1)
const histDate = ref('')
const histTime = ref('00:00:00')
const histFilter = ref('')
const showKeyCols = ref(false)
const fontScale = ref(13)
const failed = ref(false)
const fanState = ref('(!)')
const gridHeight = ref(window.innerHeight - 110)
const alarmBeep = ref(0)
const alarmBeepType = ref(0)
const removedIds = reactive(new Set())

const infoOpen = ref(false)
const infoKey = ref(0)

const criticalSound = ref(null)
const nonCriticalSound = ref(null)

let pollTimer = null
let statusTimer = null
let beepTimer = null
let lastSuccess = Date.now()
let lastCntUpdates = -1
const notifState = {}

const gpsToggleVisible = computed(
  () => cfg.value.EventsViewer_TimeGPSorLocal === 2 || cfg.value.EventsViewer_TimeGPSorLocal === 3
)
const hasStationFilter = computed(() => group1List.value.some((s) => !stationChecked[s]))

// --- headers -----------------------------------------------------------------
const headers = computed(() => {
  const h = []
  h.push({ title: t('eventsViewer.columns.date'), key: 'dateStr', width: 90, sortable: false })
  h.push({ title: t('eventsViewer.columns.time'), key: 'timeStr', width: 90, sortable: false })
  if (gpsTime.value)
    h.push({ title: t('eventsViewer.columns.ms'), key: 'ms', width: 45, sortable: false })
  if (showKeyCols.value) {
    h.push({ title: t('eventsViewer.columns.pointNum'), key: 'pointKey', width: 90, sortable: false })
    h.push({ title: t('eventsViewer.columns.id'), key: 'tag', width: 250, sortable: false })
  }
  h.push({ title: t('eventsViewer.columns.location'), key: 'station', width: 80, sortable: false })
  h.push({ title: t('eventsViewer.columns.description'), key: 'descr', sortable: false })
  h.push({ title: t('eventsViewer.columns.event'), key: 'event', width: 180, sortable: false })
  h.push({ title: t('eventsViewer.columns.qualifier'), key: 'qualifier', width: 70, sortable: false })
  return h
})

const filterText = computed(() => {
  if (mode.value === 4) {
    return histFilter.value.trim() !== '' ? ' — ' + histFilter.value.trim() : ''
  }
  const unchecked = group1List.value.filter((s) => !stationChecked[s])
  if (unchecked.length === 0 || unchecked.length === group1List.value.length) return ''
  const checked = group1List.value.filter((s) => stationChecked[s])
  return ' — ' + checked.slice(0, 4).join(' ') + (checked.length > 4 ? ' ...' : '')
})

// --- visible rows: apply station filter + compute time-gap border ------------
const visibleRows = computed(() => {
  const checkedStations = group1List.value.filter((s) => stationChecked[s])
  const allChecked = checkedStations.length === group1List.value.length
  let prevDiff = 0
  return rows.value
    .filter((r) => mode.value === 4 || allChecked || stationChecked[r.station])
    .map((r) => {
      let gap = (r.timeDiffSec - prevDiff) / 60
      prevDiff = r.timeDiffSec
      if (gap > 30) gap = 30
      if (gap < 1) gap = 0
      return { ...r, gapPx: Math.round(gap) }
    })
})

// --- styling -----------------------------------------------------------------
const isDark = computed(() => vuetifyTheme.global.current.value.dark)

// theme-aware text colors for row states
const almTxtColor = computed(() => isDark.value ? '#82B1FF' : cfg.value.EventsViewer_AlmTxtColor)
const failTxtColor = computed(() => isDark.value ? '#FF8A80' : cfg.value.EventsViewer_FailTxtColor)
const ackTxtColor = computed(() => isDark.value ? '#BDBDBD' : cfg.value.EventsViewer_AckTxtColor)
const elimTxtColor = computed(() => isDark.value ? '#9E9E9E' : cfg.value.EventsViewer_ElimTxtColor)

// shared canvas context for color parsing (avoids per-call allocations)
let _colorCtx = null
function _getColorCtx() {
  if (!_colorCtx) _colorCtx = document.createElement('canvas').getContext('2d')
  return _colorCtx
}
const _colorCache = new Map()

function hexFromColor(color) {
  if (!color) return '#000000'
  const cached = _colorCache.get(color)
  if (cached) return cached
  const ctx = _getColorCtx()
  ctx.fillStyle = color
  const hex = ctx.fillStyle
  _colorCache.set(color, hex)
  return hex
}

function lightenColor(color, amount = 0.4) {
  if (!color) return color
  const hex = hexFromColor(color)
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  const mix = (c) => Math.round(c + (255 - c) * amount)
  return `rgb(${mix(r)}, ${mix(g)}, ${mix(b)})`
}

function getPillTextColor(bgColor) {
  if (!bgColor) return '#000'
  const hex = hexFromColor(bgColor)
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  const lum = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  return lum > 0.55 ? '#000' : '#fff'
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
  const color = arr[idx % arr.length]
  return isDark.value ? lightenColor(color) : color
}

function rowProps({ item }) {
  const q = item.qualifier
  const pri = parseInt(q.charAt(0))
  const acked = q.indexOf('L') === -1
  let color = ''
  let fontWeight = 'normal'
  if (removedIds.has(item.rowId)) {
    color = elimTxtColor.value
  } else if (acked) {
    color = ackTxtColor.value
  } else if (pri === 0) {
    color = colorOfPriority(0)
    fontWeight = 'bold'
  } else {
    color = almTxtColor.value
  }
  if (q.indexOf('F') !== -1) color = failTxtColor.value
  if (pri === 0) fontWeight = 'bold'
  const style = { color, fontWeight, cursor: 'pointer' }
  if (item.gapPx > 0)
    style.borderTop = item.gapPx + 'px solid ' + cfg.value.EventsViewer_GridColor
  return { style }
}

function qualifPillStyle(item) {
  const pri = parseInt(item.qualifier.charAt(0))
  const bgColor = colorOfPriority(pri)
  return {
    backgroundColor: bgColor,
    borderRadius: '10px',
    color: getPillTextColor(bgColor),
    fontWeight: 'bold',
    textAlign: 'center',
    padding: '0 6px',
    opacity: item.qualifier.indexOf('L') !== -1 ? 1 : 0.55,
  }
}

const rowHeight = computed(() => Math.round(fontScale.value + 5))

const rootStyle = computed(() => ({
  '--eve-font-size': fontScale.value + 'px',
  '--row-height': rowHeight.value + 'px',
  fontFamily: cfg.value.EventsViewer_Font,
}))

// --- feed --------------------------------------------------------------------
function modeToAggregate(m) {
  if (m === 1) return 1
  if (m === 2) return 2
  return 0
}

async function pollTick() {
  if (mode.value !== 3 && mode.value !== 4) {
    fanState.value = '.'
    try {
      const data = await opc.readSoe(
        {
          group1Filter: activeStationFilter(),
          useSourceTime: gpsTime.value === 1,
          aggregate: modeToAggregate(mode.value),
          limit: cfg.value.EventsViewer_MaxRealtimeEvents,
          panicPriorityLimit: cfg.value.EventsViewer_PanicModePriorityLimit,
        },
        cfg.value
      )
      rows.value = data
      lastSuccess = Date.now()
      failed.value = false
      if (cfg.value.EventsViewer_Notific) processNotifications(data)
      fanState.value = '.'
    } catch (e) {
      fanState.value = 'E'
    }
  }
  if (Date.now() - lastSuccess > 30000 && mode.value !== 3 && mode.value !== 4) {
    rows.value = []
    failed.value = true
  }
  pollTimer = setTimeout(pollTick, cfg.value.EventsViewer_RefreshTime * 1000)
}

function activeStationFilter() {
  const checked = group1List.value.filter((s) => stationChecked[s])
  if (checked.length === group1List.value.length) return [] // all → no filter
  return checked
}

async function statusTick() {
  try {
    const pts = await opc.readPoints([BEEP_POINTKEY, CNTUPDATES_POINTKEY], true, cfg.value)
    const beep = pts.find((p) => p.key === BEEP_POINTKEY)
    const cnt = pts.find((p) => p.key === CNTUPDATES_POINTKEY)
    if (beep) {
      let on = beep.valueRaw ? 1 : 0
      if (on && Array.isArray(beep.beepGroup1List)) {
        const mine = getUserGroup1List()
        if (mine.length > 0 && beep.beepGroup1List.filter((v) => mine.includes(v)).length === 0)
          on = 0
      }
      alarmBeep.value = on
      alarmBeepType.value = beep.beepType || 0
    }
    if (cnt) {
      const v = cnt.valueRaw
      if (lastCntUpdates !== -1 && v !== lastCntUpdates && mode.value < 3) {
        clearTimeout(pollTimer)
        pollTimer = setTimeout(pollTick, 100)
      }
      lastCntUpdates = v
    }
  } catch (e) {
    /* ignore */
  }
  statusTimer = setTimeout(statusTick, 2000)
}

async function histSearch() {
  const filter = histFilter.value.trim() !== '' ? [histFilter.value.trim()] : []
  const tp = histTime.value.split(':')
  if (tp.length === 2) tp[2] = '0'
  const begin = new Date(histDate.value)
  begin.setTime(
    begin.getTime() +
      (begin.getTimezoneOffset() / 60) * 3600000 +
      tp[0] * 3600000 +
      tp[1] * 60000 +
      tp[2] * 1000
  )
  const end = new Date(histDate.value)
  end.setTime(
    end.getTime() + (begin.getTimezoneOffset() / 60) * 3600000 + 23 * 3600000 + 59 * 60000 + 59 * 1000 + 999
  )
  fanState.value = '.'
  try {
    const data = await opc.readSoe(
      {
        group1Filter: filter,
        useSourceTime: gpsTime.value === 1,
        aggregate: 0,
        limit: cfg.value.EventsViewer_MaxHistoricalEvents,
        timeBegin: begin,
        timeEnd: end,
      },
      cfg.value
    )
    rows.value = data
    // continue-from-last paging seed
    if (data.length >= cfg.value.EventsViewer_MaxHistoricalEvents && data.length > 1)
      histTime.value = data[data.length - 2].timeStr
    fanState.value = '.'
  } catch (e) {
    fanState.value = 'E'
  }
}

// --- row interactions --------------------------------------------------------
function onRowClick(event, { item }) {
  const forceInfo = event.altKey || event.shiftKey
  if (forceInfo) {
    silenceBeep()
    infoKey.value = item.pointKey
    infoOpen.value = true
    return
  }
  if (!userHasRight('ackEvents')) return
  const aggregate = mode.value === 1 || gpsTime.value === 0 ? 1 : 0
  const unacked = item.qualifier.indexOf('L') !== -1
  if (unacked) {
    opc.writeEventAck({
      pointId: item.tag,
      eventId: item.eventId,
      action: opc.eventAction(0, aggregate, item.tag),
    })
    item.qualifier = item.qualifier.replace('L', '')
  } else {
    opc.writeEventAck({
      pointId: item.tag,
      eventId: item.eventId,
      action: opc.eventAction(1, aggregate, item.tag),
    })
    removedIds.add(item.rowId)
  }
  scheduleAckRefresh()
}

function ackAll() {
  opc.writeEventAck({ pointId: 0, eventId: 0, action: opc.eventAction(0, 0, 0) })
  scheduleAckRefresh()
}
function removeAll() {
  opc.writeEventAck({ pointId: 0, eventId: 0, action: opc.eventAction(1, 0, 0) })
  scheduleAckRefresh()
}
function silenceBeep() {
  opc.writeEventAck({ pointId: 0, eventId: 0, action: OpcAcknowledge.SilenceBeep })
  alarmBeep.value = 0
}
function scheduleAckRefresh() {
  if (mode.value >= 3) return
  clearTimeout(pollTimer)
  pollTimer = setTimeout(pollTick, 4000)
}

// --- desktop notifications (ported from events.html processaNotific) ---------
function processNotifications(list) {
  if (!('Notification' in window) || Notification.permission !== 'granted') return
  const c = cfg.value
  const cria = {}
  for (let i = list.length - 1; i >= 0; i--) {
    const r = list[i]
    const semod = r.station + ' - ' + (r.descr.split('~')[0] || '')
    const estdj =
      r.tag.indexOf(c.Viewers_NotificTagText) !== -1 &&
      r.descr.indexOf(c.Viewers_NotificDescrText) !== -1
    if (estdj && r.event.indexOf(c.Viewers_NotificCancelEventText) !== -1) {
      delete notifState[semod]
      delete cria[semod]
    }
    if (r.qualifier.indexOf('L') === -1) continue
    if (
      (estdj && r.event.indexOf(c.Viewers_NotificEventText) !== -1) ||
      r.event === c.Viewers_AddToNotificEventText
    ) {
      const aux = notifState[semod]?.info || ''
      const acr = r.descr.substring(r.descr.indexOf('~') + 1) + ' : ' + r.event
      const info = aux.indexOf(acr) === -1 ? aux + (aux === '' ? '' : '\n') + acr : aux
      if (info !== (notifState[semod]?.info || undefined)) {
        cria[semod] = { title: semod, body: info, group1: r.station, time: r.timeStr }
        notifState[semod] = { info, time: r.timeStr, station: r.station }
      }
    }
  }
  for (const key in cria) {
    const item = cria[key]
    if (
      item.body.indexOf(c.Viewers_AlmTxtHighlight1) !== -1 &&
      item.body.indexOf(c.Viewers_DescrTxtHighlight2) !== -1
    ) {
      const nf = new Notification(item.title + ' - ' + item.time, {
        body: item.body,
        tag: item.title + '-' + item.time,
        requireInteraction: true,
      })
      nf.onclick = () => {
        nf.close()
        if (typeof c.EventsViewer_NotificationClick === 'function')
          c.EventsViewer_NotificationClick(0, '', item.group1)
      }
      setTimeout(() => nf.close(), 10 * 60 * 1000)
    }
  }
}

// --- toolbar helpers ---------------------------------------------------------
function onModeChange() {
  if (mode.value !== 3 && mode.value !== 4) {
    clearTimeout(pollTimer)
    pollTimer = setTimeout(pollTick, 100)
  }
  if (mode.value === 4) {
    const now = new Date()
    histDate.value = now.toISOString().slice(0, 10)
    histTime.value = '00:00:00'
    histFilter.value = ''
  }
  setBeforeUnload(mode.value !== 4)
  onResize()
}
function toggleGpsTime() {
  gpsTime.value = gpsTime.value ? 0 : 1
  if (storageAvailable('localStorage')) localStorage.setItem('gpstime', gpsTime.value)
  if (mode.value !== 3 && mode.value !== 4) {
    clearTimeout(pollTimer)
    pollTimer = setTimeout(pollTick, 100)
  }
}
function fontInc(dir) {
  const v = fontScale.value + dir
  if (v >= 10 && v <= 30) {
    fontScale.value = v
    if (storageAvailable('localStorage')) localStorage.setItem('eve_fontsize', v)
  }
}
function copyGrid() {
  copyRowsToClipboard(headers.value, visibleRows.value)
}

// --- keyboard ----------------------------------------------------------------
function onKeydown(e) {
  const tag = (e.target && e.target.tagName) || ''
  const inInput = tag === 'INPUT' || tag === 'TEXTAREA'
  if (e.key === 'F2') {
    removeAll()
    e.preventDefault()
  } else if (e.key === 'F8') {
    ackAll()
    e.preventDefault()
  } else if (e.key === 'F9') {
    silenceBeep()
    e.preventDefault()
  } else if (e.key === 'Escape') {
    infoOpen.value = false
  } else if (!inInput && e.key >= '1' && e.key <= '5') {
    mode.value = parseInt(e.key) - 1
    onModeChange()
  } else if (e.key === '+' || e.key === 'Add') {
    fontInc(1)
  } else if (e.key === '-' || e.key === 'Subtract') {
    fontInc(-1)
  }
}

function onResize() {
  gridHeight.value = window.innerHeight - (mode.value === 4 ? 160 : 110)
}
function beforeUnloadGuard(e) {
  e.preventDefault()
  e.returnValue = ''
}
function setBeforeUnload(on) {
  window.removeEventListener('beforeunload', beforeUnloadGuard)
  if (on) window.addEventListener('beforeunload', beforeUnloadGuard)
}

watch(showKeyCols, onResize)

// --- lifecycle ---------------------------------------------------------------
onMounted(async () => {
  onAccessDenied(() => router.push('/login'))
  cfg.value = await loadViewersConfig()

  // GPS/local initialization per config
  const tg = cfg.value.EventsViewer_TimeGPSorLocal
  if (tg === 0) gpsTime.value = 1
  else if (tg === 1) gpsTime.value = 0
  else {
    const gps = readCookie('gpstime')
    const lsGps = storageAvailable('localStorage') ? localStorage.getItem('gpstime') : null
    if (lsGps !== null && isInt(lsGps)) gpsTime.value = parseInt(lsGps)
    else if (isInt(gps)) gpsTime.value = parseInt(gps)
    else gpsTime.value = tg === 2 ? 1 : 0
  }

  if (storageAvailable('localStorage')) {
    const fs = parseInt(localStorage.getItem('eve_fontsize'))
    if (!isNaN(fs) && fs >= 10 && fs <= 30) fontScale.value = fs
  }

  group1List.value = (await opc.getGroup1List()).filter((s) => s !== '')
  group1List.value.forEach((s) => (stationChecked[s] = true))

  if (route.value.query.modo === '4' || props.mode === 4) mode.value = 4

  histDate.value = new Date().toISOString().slice(0, 10)
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('resize', onResize)
  setBeforeUnload(mode.value !== 4)
  if ('Notification' in window && cfg.value.EventsViewer_Notific && Notification.permission === 'default')
    Notification.requestPermission()
  onResize()
  pollTick()
  statusTick()
  beepTimer = setInterval(() => {
    if (alarmBeep.value) {
      const el = alarmBeepType.value === 2 ? criticalSound.value : nonCriticalSound.value
      if (el) el.play().catch(() => {})
    }
  }, 1500)
})

onUnmounted(() => {
  clearTimeout(pollTimer)
  clearTimeout(statusTimer)
  clearInterval(beepTimer)
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('resize', onResize)
  window.removeEventListener('beforeunload', beforeUnloadGuard)
})
</script>

<style scoped>
.events-root {
  height: calc(100vh - 60px);
  font-size: var(--eve-font-size, 14px);
  user-select: none;
}
.events-grid {
  font-size: var(--eve-font-size, 14px);
}
.events-grid :deep(td) {
  padding: 0 4px !important;
  line-height: 1;
  height: var(--row-height) !important;
  user-select: none;
}
.events-grid :deep(th) {
  padding: 0 4px !important;
  height: var(--row-height) !important;
  user-select: none;
}
.events-grid :deep(tr:nth-child(even) td) {
  background-color: rgba(128, 128, 128, 0.05) !important;
}
.fan {
  font-family: courier, monospace;
  font-weight: bold;
}
.qualif {
  font-size: smaller;
}
</style>
