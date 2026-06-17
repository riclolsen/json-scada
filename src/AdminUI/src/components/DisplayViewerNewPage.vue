<template>
  <v-container fluid class="pa-0 display-root">
    <!-- Toolbar -->
    <v-toolbar density="compact" color="surface" class="px-2 display-bar" flat>
      <v-select
        v-model="selectedScreen"
        :items="screenItems"
        :label="$t('displayViewer.screen')"
        density="compact"
        variant="outlined"
        hide-details
        style="max-width: 280px"
        class="mr-2"
        @update:model-value="onSelectScreen"
      ></v-select>
      <v-btn icon size="small" variant="text" :title="$t('displayViewer.prev')" @click="navAdjacent(-1)">
        <v-icon>mdi-chevron-left</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('displayViewer.next')" @click="navAdjacent(1)">
        <v-icon>mdi-chevron-right</v-icon>
      </v-btn>

      <v-divider vertical class="mx-1"></v-divider>
      <v-btn icon size="small" variant="text" :title="$t('displayViewer.zoomIn')" @click="zp('in')">
        <v-icon>mdi-magnify-plus</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('displayViewer.zoomOut')" @click="zp('out')">
        <v-icon>mdi-magnify-minus</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('displayViewer.panUp')" @click="zp('up')">
        <v-icon>mdi-arrow-up</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('displayViewer.panDown')" @click="zp('down')">
        <v-icon>mdi-arrow-down</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('displayViewer.panLeft')" @click="zp('left')">
        <v-icon>mdi-arrow-left</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('displayViewer.panRight')" @click="zp('right')">
        <v-icon>mdi-arrow-right</v-icon>
      </v-btn>
      <v-btn icon size="small" variant="text" :title="$t('displayViewer.center')" @click="zp('center')">
        <v-icon>mdi-image-filter-center-focus</v-icon>
      </v-btn>

      <v-divider vertical class="mx-1"></v-divider>
      <v-btn icon size="small" variant="text" :title="$t('displayViewer.slideshow')" @click="toggleSlideshow">
        <v-icon>{{ slideshow ? 'mdi-pause' : 'mdi-play' }}</v-icon>
      </v-btn>
      <v-btn
        icon
        size="small"
        variant="text"
        :color="tmActive ? 'warning' : undefined"
        :title="$t('displayViewer.timeMachine')"
        @click="toggleTimeMachine"
      >
        <v-icon>mdi-history</v-icon>
      </v-btn>
      <v-btn
        v-if="alarmBeep"
        icon
        size="small"
        variant="text"
        color="error"
        :title="$t('displayViewer.silence')"
        @click="silenceBeep"
      >
        <v-icon>mdi-bell-off</v-icon>
      </v-btn>

      <v-spacer></v-spacer>
      <span class="text-caption mr-2">{{ statusMsg }}</span>
      <span class="text-caption" :class="{ 'tm-clock': tmActive }">{{ tmActive ? tmDate + ' ' + tmTimeStr : clock }}</span>
    </v-toolbar>

    <!-- Time Machine controls -->
    <div v-if="tmActive" class="tm-bar px-2">
      <v-icon size="small" class="mr-2">mdi-history</v-icon>
      <span class="text-caption mr-2">{{ $t('displayViewer.timeMachine') }}</span>
      <input type="date" v-model="tmDate" :max="todayStr" min="2013-01-01" class="tm-date" @change="tmChanged" />
      <span class="tm-time mx-3">{{ tmTimeStr }}</span>
      <input
        type="range"
        min="0"
        :max="tmSliderMax"
        v-model.number="tmSeconds"
        class="tm-slider"
        @change="tmChanged"
      />
      <v-btn size="small" variant="tonal" class="ml-3" @click="exitTimeMachine">
        {{ $t('displayViewer.timeMachineExit') }}
      </v-btn>
    </div>

    <!-- SVG mount -->
    <div ref="svgContainer" class="svg-container" @wheel.prevent="onWheel"></div>

    <!-- Point Info + Command dialogs (shared component) -->
    <PointInfoDialog v-model="infoOpen" :point-key="infoKey" @saved="onPointSaved" />

    <!-- Beep sounds -->
    <audio ref="criticalSound" src="/sounds/critical.wav"></audio>
    <audio ref="nonCriticalSound" src="/sounds/noncritical.wav"></audio>
  </v-container>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { loadViewersConfig, getViewersConfig } from '../lib/viewersConfig'
import { onAccessDenied, writeAck } from '../lib/opcClient'
import { OpcAcknowledge } from '../lib/opcCodes'
import { SageEngine } from '../lib/sage/sageEngine'
import { loadScreenList, screenUrl } from '../lib/screenList'
import PointInfoDialog from './PointInfoDialog.vue'

const router = useRouter()
const route = router.currentRoute

const svgContainer = ref(null)
const screenItems = ref([])
const selectedScreen = ref('')
const alarmBeep = ref(0)
const statusMsg = ref('')
const clock = ref('')
const slideshow = ref(false)

const infoOpen = ref(false)
const infoKey = ref(0)

// --- Time Machine (historical replay) ---
const tmActive = ref(false)
const tmDate = ref('')
const tmSeconds = ref(0)
const todayStr = ref('')
let tmDebounce = null

const tmTimeStr = computed(() => fmtHMS(tmSeconds.value))
const tmSliderMax = computed(() => (tmDate.value === todayStr.value ? nowSeconds() : 86399))

function nowSeconds() {
  const d = new Date()
  return d.getHours() * 3600 + d.getMinutes() * 60 + d.getSeconds()
}
function fmtHMS(s) {
  s = Math.max(0, Math.floor(s))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const ss = s % 60
  const p = (n) => String(n).padStart(2, '0')
  return `${p(h)}:${p(m)}:${p(ss)}`
}
function localDateStr(d) {
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}

function toggleTimeMachine() {
  if (tmActive.value) {
    exitTimeMachine()
    return
  }
  if (!engine) return
  if (slideshow.value) toggleSlideshow()
  tmActive.value = true
  todayStr.value = localDateStr(new Date())
  tmDate.value = todayStr.value
  tmSeconds.value = nowSeconds()
  engine.enterTimeMachine()
  tmChanged()
}

function exitTimeMachine() {
  tmActive.value = false
  clearTimeout(tmDebounce)
  if (engine) engine.exitTimeMachine()
}

function tmChanged() {
  clearTimeout(tmDebounce)
  tmDebounce = setTimeout(() => {
    if (!engine || !tmActive.value || !tmDate.value) return
    const [y, mo, d] = tmDate.value.split('-').map(Number)
    const dt = new Date(y, mo - 1, d, 0, 0, 0, 0)
    dt.setSeconds(tmSeconds.value)
    if (dt > new Date()) dt.setTime(Date.now())
    engine.gotoTime(dt)
  }, 500)
}

const criticalSound = ref(null)
const nonCriticalSound = ref(null)

let engine = null
let clockTimer = null
let beepTimer = null
let slideTimer = null

function openPoint(key) {
  infoKey.value = key
  infoOpen.value = true
}

// after the dialog writes properties (e.g. a blocking annotation), re-read that
// point's full info so the pinned-annotation badge appears without a reload
function onPointSaved(key) {
  if (engine) engine.refreshPoint(key)
}

function loadScreenByRef(ref) {
  const url = screenUrl(ref)
  if (!url || !engine) return
  selectedScreen.value = matchItemValue(ref)
  engine.loadScreen(url).catch((e) => (statusMsg.value = e.message))
  // reflect in route without reload
  router.replace({ path: '/display-viewer-new', query: { screen: ref } })
}

// Map a loose ref to a dropdown item value when possible
function matchItemValue(ref) {
  const found = screenItems.value.find(
    (it) => it.value === ref || it.value.endsWith('/' + ref) || it.value.endsWith('/' + ref + '.svg')
  )
  return found ? found.value : ref
}

function onSelectScreen(val) {
  loadScreenByRef(val)
}

function navAdjacent(dir) {
  if (screenItems.value.length === 0) return
  const idx = screenItems.value.findIndex((it) => it.value === selectedScreen.value)
  const next = idx + dir
  if (next >= 0 && next < screenItems.value.length) loadScreenByRef(screenItems.value[next].value)
}

function zp(op) {
  if (engine) engine.zoomPan(op)
}
function onWheel(ev) {
  if (engine) engine.wheelZoom(ev)
}

function toggleSlideshow() {
  slideshow.value = !slideshow.value
  if (slideshow.value) {
    slideTimer = setInterval(
      () => navAdjacent(1),
      (getViewersConfig().ScreenViewer_SlideShowInterval || 10) * 1000
    )
  } else {
    clearInterval(slideTimer)
  }
}

function silenceBeep() {
  writeAck({ pointId: 0, action: OpcAcknowledge.SilenceBeep })
  alarmBeep.value = 0
}

function onKeydown(e) {
  const tag = (e.target && e.target.tagName) || ''
  if (tag === 'INPUT' || tag === 'TEXTAREA') return
  if (e.key === ',') navAdjacent(-1)
  else if (e.key === '.') navAdjacent(1)
  else if (e.key === 'F9') {
    silenceBeep()
    e.preventDefault()
  } else if (e.key === 'Escape') infoOpen.value = false
}

onMounted(async () => {
  onAccessDenied(() => router.push('/login'))
  const cfg = await loadViewersConfig()
  screenItems.value = await loadScreenList()

  engine = new SageEngine({
    container: svgContainer.value,
    cfg,
    onOpenPoint: openPoint,
    onAlarmBeep: (v) => (alarmBeep.value = v),
    onStatus: (m) => (statusMsg.value = m),
    onScreenLink: (screen) => loadScreenByRef(screen),
  })

  const initial =
    route.value.query.screen ||
    route.value.query.SELTELA ||
    route.value.query.SVGFILE ||
    (screenItems.value[0] && screenItems.value[0].value)
  if (initial) loadScreenByRef(initial)

  window.addEventListener('keydown', onKeydown)
  clockTimer = setInterval(() => (clock.value = new Date().toLocaleString()), 1000)
  beepTimer = setInterval(() => {
    if (alarmBeep.value && nonCriticalSound.value) nonCriticalSound.value.play().catch(() => {})
  }, 1500)
})

onUnmounted(() => {
  if (engine) engine.dispose()
  clearInterval(clockTimer)
  clearInterval(beepTimer)
  clearInterval(slideTimer)
  clearTimeout(tmDebounce)
  window.removeEventListener('keydown', onKeydown)
})
</script>

<style scoped>
.display-root {
  height: calc(100vh - 48px);
  display: flex;
  flex-direction: column;
}
.svg-container {
  flex: 1 1 auto;
  overflow: hidden;
  position: relative;
}
.svg-container :deep(svg) {
  width: 100%;
  height: 100%;
}
.tm-bar {
  display: flex;
  align-items: center;
  background-color: var(--tm-bg, #2e7d32);
  color: #fff;
  height: 40px;
  flex: 0 0 auto;
}
.tm-bar .v-icon {
  color: #fff;
}
.tm-date {
  background: #fff;
  color: #000;
  border-radius: 3px;
  padding: 1px 4px;
  font-size: 0.85rem;
}
.tm-time {
  font-family: monospace;
  font-size: 0.95rem;
  min-width: 70px;
}
.tm-slider {
  flex: 1 1 auto;
  accent-color: #fff;
}
.tm-clock {
  color: #ffb74d;
  font-weight: 600;
}
</style>
