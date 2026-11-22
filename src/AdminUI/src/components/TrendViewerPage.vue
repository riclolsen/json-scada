<template>
  <v-container fluid class="pa-0 trend-root">
    <!-- Toolbar (hidden with ?controls=0, e.g. the Display Viewer hover preview) -->
    <v-toolbar v-if="showControls" density="compact" color="surface" class="px-2 trend-bar" flat>
      <v-btn-toggle
        v-model="rangeMin"
        mandatory
        density="compact"
        variant="outlined"
        divided
        class="mr-2"
        @update:model-value="setRange"
      >
        <v-btn v-for="r in ranges" :key="r.min" :value="r.min" size="small">{{ r.label }}</v-btn>
      </v-btn-toggle>

      <v-btn icon size="small" variant="text" :title="$t('trendViewer.panLeft')" @click="panX(-1)">
        <v-icon>mdi-arrow-left</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('trendViewer.panRight')" @click="panX(1)">
        <v-icon>mdi-arrow-right</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('trendViewer.zoomIn')" @click="zoomX(0.5)">
        <v-icon>mdi-magnify-plus</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('trendViewer.zoomOut')" @click="zoomX(2)">
        <v-icon>mdi-magnify-minus</v-icon>
      </v-btn>

      <v-divider vertical class="mx-1"></v-divider>
      <v-btn icon size="small" variant="text" :title="$t('trendViewer.yUp')" @click="panY(1)">
        <v-icon>mdi-arrow-up</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('trendViewer.yDown')" @click="panY(-1)">
        <v-icon>mdi-arrow-down</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('trendViewer.yZoomIn')" @click="zoomY(0.5)">
        <v-icon>mdi-arrow-collapse-vertical</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('trendViewer.yZoomOut')" @click="zoomY(2)">
        <v-icon>mdi-arrow-expand-vertical</v-icon>
      </v-btn>

      <v-divider vertical class="mx-1"></v-divider>
      <v-btn icon size="small" variant="text" :title="$t('trendViewer.markers')" @click="cycleMarkers">
        <v-icon>{{ markerIcon }}</v-icon>
      </v-btn>
      <v-btn
        icon
        size="small"
        variant="text"
        :color="follow ? 'primary' : undefined"
        :title="$t('trendViewer.reset')"
        @click="resetView"
      >
        <v-icon>mdi-restore</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('trendViewer.downloadCsv')" @click="downloadCsv">
        <v-icon>mdi-download</v-icon>
      </v-btn>

      <v-spacer></v-spacer>
      <v-progress-circular v-if="loading" indeterminate size="18" width="2" class="mr-2"></v-progress-circular>
    </v-toolbar>

    <!-- Point identification and current value -->
    <div class="trend-status px-2">
      <span class="font-weight-medium">{{ pointTitle }}</span>
      <span v-if="info" class="ml-3">
        {{ $t('trendViewer.current') }}: <strong>{{ currentText }}</strong>
        <span v-if="qualityText" class="ml-2 text-medium-emphasis">{{ qualityText }}</span>
      </span>
    </div>

    <!-- Chart -->
    <div
      ref="chartBox"
      class="trend-chart"
      @wheel.prevent="onWheel"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
      @pointerleave="hoverX = null"
      @dblclick="resetView"
    >
      <svg v-if="width > 0 && height > 0" :width="width" :height="height" class="trend-svg">
        <!-- grid + axes -->
        <g class="grid">
          <line
            v-for="tk in yTicks"
            :key="'gy' + tk.v"
            :x1="M.left"
            :x2="width - M.right"
            :y1="yPx(tk.v)"
            :y2="yPx(tk.v)"
          />
          <line
            v-for="tk in xTicks"
            :key="'gx' + tk.t"
            :x1="xPx(tk.t)"
            :x2="xPx(tk.t)"
            :y1="M.top"
            :y2="height - M.bottom"
          />
        </g>
        <rect
          class="frame"
          :x="M.left"
          :y="M.top"
          :width="plotW"
          :height="plotH"
        />
        <g class="ticks">
          <text
            v-for="tk in yTicks"
            :key="'ty' + tk.v"
            :x="M.left - 6"
            :y="yPx(tk.v) + 4"
            text-anchor="end"
          >{{ tk.label }}</text>
          <text
            v-for="tk in xTicks"
            :key="'tx' + tk.t"
            :x="xPx(tk.t)"
            :y="height - M.bottom + 16"
            text-anchor="middle"
          >{{ tk.label }}</text>
          <text v-if="xDateLabel" :x="M.left" :y="height - 4" class="date-label">{{ xDateLabel }}</text>
        </g>

        <svg :x="M.left" :y="M.top" :width="plotW" :height="plotH" overflow="hidden">
          <g :transform="`translate(${-M.left},${-M.top})`">
            <!-- now marker -->
            <line
              v-if="nowMs >= xMin && nowMs <= xMax"
              class="now-line"
              :x1="xPx(nowMs)"
              :x2="xPx(nowMs)"
              :y1="M.top"
              :y2="height - M.bottom"
            />
            <!-- limits -->
            <line
              v-for="l in limitLines"
              :key="l.key"
              :class="['limit', l.side, 'lvl' + l.level]"
              :x1="M.left"
              :x2="width - M.right"
              :y1="yPx(l.value)"
              :y2="yPx(l.value)"
            />
            <!-- series -->
            <polyline v-if="markers !== 'points'" class="series" :points="seriesPath" />
            <polyline v-if="markers !== 'points' && liveTail" class="series tail" :points="liveTail" />
            <template v-if="markers !== 'lines'">
              <circle
                v-for="(p, i) in visibleMarkers"
                :key="'m' + i"
                class="marker"
                :cx="xPx(p.t)"
                :cy="yPx(p.v)"
                r="2.5"
              />
            </template>
            <!-- hover crosshair -->
            <template v-if="hover">
              <line class="cross" :x1="hover.x" :x2="hover.x" :y1="M.top" :y2="height - M.bottom" />
              <circle v-if="hover.y !== null" class="cross-dot" :cx="hover.x" :cy="hover.y" r="4" />
            </template>
          </g>
        </svg>

        <!-- legend -->
        <g class="legend" :transform="`translate(${M.left + 8},${M.top + 6})`">
          <rect class="legend-bg" x="-4" y="-2" :width="legendW" :height="legendLines.length * 16 + 6" rx="3" />
          <text v-for="(l, i) in legendLines" :key="'lg' + i" :x="0" :y="13 + i * 16" :class="l.cls">
            {{ l.text }}
          </text>
        </g>

        <text v-if="message" class="message" :x="M.left + plotW / 2" :y="M.top + plotH / 2" text-anchor="middle">
          {{ message }}
        </text>
      </svg>
    </div>
  </v-container>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { loadViewersConfig, getViewersConfig } from '../lib/viewersConfig'
import { onAccessDenied, readPoints, readHistory, HiLimitFields, LoLimitFields } from '../lib/opcClient'
import { Flags } from '../lib/opcCodes'
import { decodeQuality } from '../lib/viewerHelpers'

// Trend Viewer: one point's history plus live values.
// Query: point=<key or tag> (legacy NPONTO/TAG accepted), embed=1 (no side panel),
// controls=0 (no toolbar, for the Display Viewer hover preview).

const { t, locale } = useI18n()
const router = useRouter()
const route = router.currentRoute

const q = route.value.query
const pointRef = String(q.point || q.NPONTO || q.TAG || '').trim()
const showControls = q.controls !== '0' && q.HIDECTRLS !== '1'

const MIN = 60 * 1000
const POLL_MS = 5000
const MAX_SAMPLES = 20000
const ranges = [
  { min: 15, label: '15m' },
  { min: 60, label: '1h' },
  { min: 120, label: '2h' },
  { min: 360, label: '6h' },
  { min: 1440, label: '24h' },
  { min: 10080, label: '7d' },
]

const info = ref(null) // normalized point (readPoints)
const samples = ref([]) // [{ t, v }] ascending time
const current = ref(null) // latest live point read
const loading = ref(false)
const message = ref('')
const nowMs = ref(Date.now())

// view
const rangeMin = ref(60)
const follow = ref(true)
const xMin = ref(0)
const xMax = ref(0)
const yRange = ref(null) // null = auto scale, else { min, max }
const markers = ref('lines') // lines | both | points
let histFrom = Infinity // earliest instant already requested from the historian

// layout
const chartBox = ref(null)
const width = ref(0)
const height = ref(0)
const M = { left: 64, right: 14, top: 10, bottom: 36 }
const plotW = computed(() => Math.max(10, width.value - M.left - M.right))
const plotH = computed(() => Math.max(10, height.value - M.top - M.bottom))

const isDigital = computed(() => {
  const p = info.value
  if (!p) return false
  return p.type ? p.type === 'digital' : !(p.flags & Flags.ANALOG)
})

// --- point info / values ------------------------------------------------------

function toNumber(v) {
  if (typeof v === 'boolean') return v ? 1 : 0
  const n = typeof v === 'number' ? v : parseFloat(v)
  return isFinite(n) ? n : null
}

const pointTitle = computed(() => {
  if (!info.value) return pointRef
  const p = info.value
  return [p.station, p.tag, p.descr].filter((s) => s && String(s).trim() !== '').join(' — ')
})

function formatValue(v) {
  if (v === null || v === undefined) return '?'
  const p = info.value
  if (isDigital.value && p)
    return v ? p.stateTextTrue || '1' : p.stateTextFalse || '0'
  return Number(v).toFixed(3) + (p && p.unit ? ' ' + p.unit : '')
}

const currentText = computed(() => {
  if (!current.value) return '?'
  return formatValue(toNumber(current.value.valueRaw))
})

const qualityText = computed(() => {
  if (!current.value) return ''
  const txt = decodeQuality(current.value.flags, {
    failed: t('tabularViewer.quality.failed'),
    substituted: t('tabularViewer.quality.substituted'),
    calculated: t('tabularViewer.quality.calculated'),
    manual: t('tabularViewer.quality.manual'),
    neverUpdated: t('tabularViewer.quality.neverUpdated'),
    alarmed: t('tabularViewer.quality.alarmed'),
    inhibited: t('tabularViewer.quality.inhibited'),
    persistent: t('tabularViewer.quality.persistent'),
    frozen: t('tabularViewer.quality.frozen'),
    normal: t('tabularViewer.quality.normal'),
  }).trim()
  return txt === t('tabularViewer.quality.normal') ? '' : txt
})

function insertSamples(list) {
  if (!list.length) return
  const all = samples.value.concat(list)
  all.sort((a, b) => a.t - b.t)
  // drop exact duplicates (history overlapping live samples)
  const out = []
  for (const s of all) {
    const last = out[out.length - 1]
    if (last && last.t === s.t) continue
    out.push(s)
  }
  samples.value = out.length > MAX_SAMPLES ? out.slice(out.length - MAX_SAMPLES) : out
}

async function loadHistory(from, to) {
  if (!info.value || !(from < to)) return
  loading.value = true
  try {
    const hist = await readHistory(info.value.tag, new Date(from), new Date(to))
    const list = []
    for (const h of hist) {
      const v = toNumber(h.value)
      if (v !== null && isFinite(h.time)) list.push({ t: h.time, v })
    }
    insertSamples(list)
  } catch (e) {
    /* historian unavailable: live samples still accumulate */
  } finally {
    loading.value = false
  }
  if (!samples.value.length && !message.value) message.value = t('trendViewer.noData')
  else if (samples.value.length && message.value === t('trendViewer.noData')) message.value = ''
}

// fetch older history when the view moves before what has been requested
let histTimer = 0
function ensureHistory() {
  clearTimeout(histTimer)
  if (xMin.value >= histFrom) return
  histTimer = setTimeout(() => {
    const from = xMin.value - (xMax.value - xMin.value) * 0.25
    const to = histFrom
    histFrom = from
    loadHistory(from, to)
  }, 400)
}

async function pollLive() {
  if (!info.value) return
  try {
    const pts = await readPoints([info.value.key], true, getViewersConfig())
    if (!pts.length) return
    const p = pts[0]
    current.value = p
    const v = toNumber(p.valueRaw)
    const ts = Date.parse(pts.serverTime)
    const tm = isFinite(ts) ? ts : Date.now()
    nowMs.value = Math.max(nowMs.value, tm)
    const last = samples.value[samples.value.length - 1]
    // store on change, or every 5 minutes to keep the line anchored
    if (v !== null && (!last || last.v !== v || tm - last.t > 5 * MIN) && (!last || tm > last.t)) {
      insertSamples([{ t: tm, v }])
      if (message.value === t('trendViewer.noData')) message.value = ''
    }
  } catch (e) {
    /* transient read error */
  }
}

// --- view control -------------------------------------------------------------

function followWindow() {
  const span = rangeMin.value * MIN
  xMax.value = nowMs.value + span * 0.08
  xMin.value = xMax.value - span
}

function setRange(min) {
  rangeMin.value = min
  if (follow.value) followWindow()
  else {
    const c = (xMin.value + xMax.value) / 2
    xMin.value = c - (min * MIN) / 2
    xMax.value = c + (min * MIN) / 2
  }
  ensureHistory()
}

function panX(dir) {
  follow.value = false
  const d = ((xMax.value - xMin.value) / 4) * dir
  xMin.value += d
  xMax.value += d
  ensureHistory()
}

function zoomX(factor, centerT) {
  let span = (xMax.value - xMin.value) * factor
  span = Math.min(Math.max(span, 10 * 1000), 366 * 24 * 60 * MIN)
  if (centerT === undefined && follow.value) {
    // toolbar zoom while following: keep "now" at the right edge
    rangeMin.value = span / MIN
    followWindow()
  } else {
    const c = centerT !== undefined ? centerT : (xMin.value + xMax.value) / 2
    const r = (c - xMin.value) / (xMax.value - xMin.value)
    follow.value = false
    xMin.value = c - span * r
    xMax.value = xMin.value + span
  }
  ensureHistory()
}

function panY(dir) {
  const y = yScale.value
  const d = ((y.max - y.min) / 4) * dir
  yRange.value = { min: y.min + d, max: y.max + d }
}

function zoomY(factor, centerV) {
  const y = yScale.value
  const c = centerV !== undefined ? centerV : (y.min + y.max) / 2
  const span = (y.max - y.min) * factor
  const r = (c - y.min) / (y.max - y.min)
  yRange.value = { min: c - span * r, max: c - span * r + span }
}

function resetView() {
  follow.value = true
  yRange.value = null
  if (!ranges.some((r) => r.min === rangeMin.value)) rangeMin.value = 60
  followWindow()
  ensureHistory()
}

function cycleMarkers() {
  markers.value = { lines: 'both', both: 'points', points: 'lines' }[markers.value]
}
const markerIcon = computed(
  () => ({ lines: 'mdi-chart-line', both: 'mdi-chart-line-variant', points: 'mdi-chart-scatter-plot' })[markers.value]
)

// --- scales -------------------------------------------------------------------

function niceStep(span, count) {
  const raw = span / Math.max(1, count)
  const mag = Math.pow(10, Math.floor(Math.log10(raw)))
  const n = raw / mag
  return (n <= 1 ? 1 : n <= 2 ? 2 : n <= 5 ? 5 : 10) * mag
}

const visibleSamples = computed(() => {
  const s = samples.value
  if (!s.length) return []
  // include one sample on each side so lines run to the plot edges
  let i0 = lowerBound(s, xMin.value) - 1
  let i1 = lowerBound(s, xMax.value) + 1
  i0 = Math.max(0, i0)
  i1 = Math.min(s.length, i1)
  return s.slice(i0, i1)
})

function lowerBound(arr, tm) {
  let lo = 0
  let hi = arr.length
  while (lo < hi) {
    const mid = (lo + hi) >> 1
    if (arr[mid].t < tm) lo = mid + 1
    else hi = mid
  }
  return lo
}

const yScale = computed(() => {
  if (yRange.value) return yRange.value
  if (isDigital.value) return { min: -0.2, max: 1.2 }
  let min = Infinity
  let max = -Infinity
  for (const p of visibleSamples.value) {
    if (p.v < min) min = p.v
    if (p.v > max) max = p.v
  }
  const cv = current.value ? toNumber(current.value.valueRaw) : null
  if (cv !== null && nowMs.value >= xMin.value && nowMs.value <= xMax.value) {
    min = Math.min(min, cv)
    max = Math.max(max, cv)
  }
  if (!isFinite(min)) return { min: 0, max: 1 }
  if (min === max) {
    const d = Math.abs(min) * 0.1 || 1
    return { min: min - d, max: max + d }
  }
  const pad = (max - min) * 0.08
  return { min: min - pad, max: max + pad }
})

function xPx(tm) {
  return M.left + ((tm - xMin.value) / (xMax.value - xMin.value)) * plotW.value
}
function yPx(v) {
  const y = yScale.value
  return M.top + plotH.value - ((v - y.min) / (y.max - y.min)) * plotH.value
}
function tFromPx(px) {
  return xMin.value + ((px - M.left) / plotW.value) * (xMax.value - xMin.value)
}
function vFromPx(py) {
  const y = yScale.value
  return y.min + ((M.top + plotH.value - py) / plotH.value) * (y.max - y.min)
}

const yTicks = computed(() => {
  const y = yScale.value
  if (isDigital.value && !yRange.value) {
    const p = info.value || {}
    return [
      { v: 0, label: shorten(p.stateTextFalse || '0') },
      { v: 1, label: shorten(p.stateTextTrue || '1') },
    ]
  }
  const step = niceStep(y.max - y.min, Math.max(2, Math.floor(plotH.value / 40)))
  const dec = Math.max(0, -Math.floor(Math.log10(step)))
  const out = []
  for (let v = Math.ceil(y.min / step) * step; v <= y.max + step * 1e-9; v += step)
    out.push({ v, label: v.toFixed(dec) })
  return out
})

function shorten(s) {
  s = String(s)
  return s.length > 8 ? s.slice(0, 7) + '…' : s
}

const TIME_STEPS = [1, 2, 5, 10, 15, 30, 60, 120, 300, 600, 900, 1800, 3600, 7200, 10800, 21600, 43200, 86400, 172800, 604800].map(
  (s) => s * 1000
)

const xTicks = computed(() => {
  const span = xMax.value - xMin.value
  const want = Math.max(2, Math.floor(plotW.value / 110))
  const step = TIME_STEPS.find((s) => span / s <= want) || TIME_STEPS[TIME_STEPS.length - 1]
  // align ticks to local time
  const tz = new Date(xMin.value).getTimezoneOffset() * MIN
  const out = []
  const first = Math.ceil((xMin.value - tz) / step) * step + tz
  const opts =
    step >= 86400000 ? { day: '2-digit', month: '2-digit' }
    : step < MIN ? { hour: '2-digit', minute: '2-digit', second: '2-digit' }
    : { hour: '2-digit', minute: '2-digit' }
  for (let tm = first; tm <= xMax.value; tm += step)
    out.push({ t: tm, label: new Date(tm).toLocaleString(locale.value, opts) })
  return out
})

const xDateLabel = computed(() => {
  const a = new Date(xMin.value).toLocaleDateString(locale.value)
  const b = new Date(xMax.value).toLocaleDateString(locale.value)
  return a === b ? a : a + ' – ' + b
})

// --- series geometry ----------------------------------------------------------

function stepPoints(list) {
  let pts = ''
  let prev = null
  for (const p of list) {
    const x = xPx(p.t).toFixed(1)
    const y = yPx(p.v).toFixed(1)
    if (prev !== null && isDigital.value) pts += x + ',' + prev + ' '
    pts += x + ',' + y + ' '
    prev = y
  }
  return pts
}

const seriesPath = computed(() => stepPoints(visibleSamples.value))

// hold the last value up to "now" (the historian stores changes only)
const liveTail = computed(() => {
  const s = samples.value
  if (!s.length) return ''
  const last = s[s.length - 1]
  if (last.t >= nowMs.value || last.t > xMax.value) return ''
  const y = yPx(last.v).toFixed(1)
  return xPx(last.t).toFixed(1) + ',' + y + ' ' + xPx(nowMs.value).toFixed(1) + ',' + y
})

const visibleMarkers = computed(() => {
  const v = visibleSamples.value
  return v.length > 3000 ? [] : v
})

// --- hover / legend -----------------------------------------------------------

const hoverX = ref(null)

function valueAt(tm) {
  const s = samples.value
  if (!s.length) return null
  const i = lowerBound(s, tm)
  if (i === 0) return tm === s[0].t ? s[0].v : null
  if (i >= s.length) return tm <= nowMs.value ? s[s.length - 1].v : null
  const p1 = s[i - 1]
  const p2 = s[i]
  if (isDigital.value) return p1.v
  return p1.v + ((p2.v - p1.v) * (tm - p1.t)) / (p2.t - p1.t)
}

const hover = computed(() => {
  if (hoverX.value === null || drag) return null
  const x = Math.min(Math.max(hoverX.value, M.left), M.left + plotW.value)
  const tm = tFromPx(x)
  const v = valueAt(tm)
  return { x, tm, v, y: v === null ? null : yPx(v) }
})

// configured out-of-range alarm limits, from the most severe high to the
// most severe low (level 1 = hi/loLimit, 2 = hihi/lolo, 3 = hihihi/lololo)
const limitLines = computed(() => {
  const p = info.value
  if (!p) return []
  const out = []
  HiLimitFields.forEach((key, i) => {
    if (isFinite(p[key])) out.unshift({ key, value: p[key], side: 'hi', level: i + 1 })
  })
  LoLimitFields.forEach((key, i) => {
    if (isFinite(p[key])) out.push({ key, value: p[key], side: 'lo', level: i + 1 })
  })
  return out
})

const legendLines = computed(() => {
  const lines = []
  if (hover.value) {
    lines.push({
      text: new Date(hover.value.tm).toLocaleString(locale.value) + '  =  ' + formatValue(hover.value.v),
      cls: 'lg-main',
    })
  } else if (info.value) {
    lines.push({ text: (info.value.tag || '') + '  =  ' + currentText.value, cls: 'lg-main' })
  }
  for (const l of limitLines.value)
    lines.push({ text: t('trendViewer.' + l.key) + ': ' + formatValue(l.value), cls: 'lg-' + l.side })
  return lines
})
const legendW = computed(() => Math.min(plotW.value - 8, 8 + 7 * Math.max(0, ...legendLines.value.map((l) => l.text.length))))

// --- pointer / wheel / keyboard -----------------------------------------------

let drag = null
function localXY(evt) {
  const r = chartBox.value.getBoundingClientRect()
  return { x: evt.clientX - r.left, y: evt.clientY - r.top }
}

function onPointerDown(evt) {
  if (evt.button !== 0) return
  const p = localXY(evt)
  drag = { x: p.x, y: p.y, xMin: xMin.value, xMax: xMax.value, y0: { ...yScale.value }, moved: false }
  evt.currentTarget.setPointerCapture(evt.pointerId)
}

function onPointerMove(evt) {
  const p = localXY(evt)
  hoverX.value = p.x
  if (!drag) return
  const dx = p.x - drag.x
  const dy = p.y - drag.y
  if (!drag.moved && Math.abs(dx) + Math.abs(dy) < 4) return
  drag.moved = true
  follow.value = false
  const dt = (dx / plotW.value) * (drag.xMax - drag.xMin)
  xMin.value = drag.xMin - dt
  xMax.value = drag.xMax - dt
  if (yRange.value || Math.abs(dy) > 8) {
    const dv = (dy / plotH.value) * (drag.y0.max - drag.y0.min)
    yRange.value = { min: drag.y0.min + dv, max: drag.y0.max + dv }
  }
}

function onPointerUp(evt) {
  if (drag && drag.moved) ensureHistory()
  drag = null
  if (evt.currentTarget.hasPointerCapture && evt.currentTarget.hasPointerCapture(evt.pointerId))
    evt.currentTarget.releasePointerCapture(evt.pointerId)
}

function onWheel(evt) {
  const p = localXY(evt)
  const factor = evt.deltaY > 0 ? 1.25 : 0.8
  if (evt.shiftKey) zoomY(factor, vFromPx(p.y))
  else zoomX(factor, tFromPx(p.x))
}

function onKeydown(evt) {
  if (evt.target && /INPUT|TEXTAREA|SELECT/.test(evt.target.tagName)) return
  const map = {
    ArrowLeft: () => panX(-1),
    ArrowRight: () => panX(1),
    ArrowUp: () => panY(1),
    ArrowDown: () => panY(-1),
    '+': () => zoomX(0.5),
    '-': () => zoomX(2),
    '*': () => zoomY(0.5),
    '/': () => zoomY(2),
    0: resetView,
    p: cycleMarkers,
  }
  const fn = map[evt.key]
  if (fn) {
    evt.preventDefault()
    fn()
  }
}

// --- CSV export ---------------------------------------------------------------

function downloadCsv() {
  const rows = visibleSamples.value.filter((p) => p.t >= xMin.value && p.t <= xMax.value)
  const csv =
    'time,value\n' + rows.map((p) => new Date(p.t).toISOString() + ',' + p.v).join('\n') + '\n'
  const blob = new Blob([csv], { type: 'text/csv' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = 'trend-' + ((info.value && info.value.tag) || pointRef).replace(/[^\w.-]+/g, '_') + '.csv'
  a.click()
  setTimeout(() => URL.revokeObjectURL(a.href), 1000)
}

// --- lifecycle ----------------------------------------------------------------

let pollTimer = 0
let clockTimer = 0
let ro = null
// set on unmount: async work still running then must not start timers again
let disposed = false

onMounted(async () => {
  onAccessDenied(() => router.push('/login'))
  window.addEventListener('keydown', onKeydown)
  ro = new ResizeObserver(() => {
    if (!chartBox.value) return
    width.value = chartBox.value.clientWidth
    height.value = chartBox.value.clientHeight
  })
  ro.observe(chartBox.value)

  nowMs.value = Date.now()
  followWindow()

  if (!pointRef) {
    message.value = t('trendViewer.noPoint')
    return
  }
  await loadViewersConfig()
  if (disposed) return
  loading.value = true
  try {
    const pts = await readPoints([pointRef], true, getViewersConfig())
    if (disposed) return
    if (pts.length) {
      info.value = pts[0]
      current.value = pts[0]
    }
  } catch (e) {
    /* reported below */
  }
  loading.value = false
  if (!info.value) {
    message.value = t('trendViewer.notFound')
    return
  }
  document.title = t('trendViewer.title') + ' - ' + info.value.tag

  histFrom = xMin.value - (xMax.value - xMin.value) * 0.25
  await loadHistory(histFrom, Date.now())
  if (disposed) return
  pollLive()
  pollTimer = setInterval(pollLive, POLL_MS)
  clockTimer = setInterval(() => {
    nowMs.value = Math.max(nowMs.value, Date.now())
    if (follow.value) followWindow()
  }, 1000)
})

onUnmounted(() => {
  disposed = true
  clearInterval(pollTimer)
  clearInterval(clockTimer)
  clearTimeout(histTimer)
  window.removeEventListener('keydown', onKeydown)
  if (ro) ro.disconnect()
})
</script>

<style scoped>
.trend-root {
  height: 100vh;
  display: flex;
  flex-direction: column;
  user-select: none;
  -webkit-user-select: none;
}
.trend-bar {
  flex: 0 0 auto;
}
.trend-status {
  flex: 0 0 auto;
  font-size: 0.85rem;
  line-height: 1.6rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.trend-chart {
  flex: 1 1 auto;
  min-height: 120px;
  position: relative;
  overflow: hidden;
  cursor: crosshair;
  touch-action: none;
}
.trend-svg {
  position: absolute;
  inset: 0;
  font-size: 11px;
}
.grid line {
  stroke: rgba(var(--v-theme-on-surface), 0.1);
}
.frame {
  fill: none;
  stroke: rgba(var(--v-theme-on-surface), 0.35);
}
.ticks text {
  fill: rgba(var(--v-theme-on-surface), 0.75);
}
.date-label {
  fill: rgba(var(--v-theme-on-surface), 0.55);
}
.series {
  fill: none;
  stroke: rgb(var(--v-theme-primary));
  stroke-width: 1.6;
  stroke-linejoin: round;
}
.series.tail {
  stroke-dasharray: 4 3;
  opacity: 0.7;
}
.marker {
  fill: rgb(var(--v-theme-primary));
}
.limit {
  stroke-width: 1.2;
  stroke-dasharray: 6 4;
}
.limit.lvl2 {
  stroke-width: 1.6;
  stroke-dasharray: 10 3;
}
.limit.lvl3 {
  stroke-width: 2;
  stroke-dasharray: none;
}
.limit.hi,
.lg-hi {
  stroke: rgb(var(--v-theme-error));
  fill: rgb(var(--v-theme-error));
}
.limit.lo,
.lg-lo {
  stroke: rgb(var(--v-theme-info));
  fill: rgb(var(--v-theme-info));
}
.lg-hi,
.lg-lo {
  stroke: none;
}
.now-line {
  stroke: rgba(var(--v-theme-on-surface), 0.3);
  stroke-dasharray: 2 3;
}
.cross {
  stroke: rgba(var(--v-theme-on-surface), 0.5);
}
.cross-dot {
  fill: rgb(var(--v-theme-surface));
  stroke: rgb(var(--v-theme-primary));
  stroke-width: 2;
}
.legend-bg {
  fill: rgba(var(--v-theme-surface), 0.8);
  stroke: rgba(var(--v-theme-on-surface), 0.15);
}
.lg-main {
  fill: rgb(var(--v-theme-on-surface));
  font-size: 12px;
}
.message {
  fill: rgba(var(--v-theme-on-surface), 0.6);
  font-size: 14px;
}
</style>
