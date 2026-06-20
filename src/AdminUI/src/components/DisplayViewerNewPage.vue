<template>
  <v-container fluid class="pa-0 display-root">
    <!-- Toolbar -->
    <v-toolbar v-show="!barHidden" density="compact" color="surface" class="px-2 display-bar" flat>
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
        <v-progress-circular
          v-if="slideshow"
          :model-value="slideProgress"
          size="24"
          width="2.5"
          color="primary"
        >
          <v-icon size="14">mdi-pause</v-icon>
        </v-progress-circular>
        <v-icon v-else>mdi-play</v-icon>
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
    </v-toolbar>

    <!-- Display update date/time, stacked (date over time); sits left of the alarm
         box when it is shown, otherwise moves all the way to the right -->
    <div class="display-datetime" :class="{ 'tm-clock': tmActive, 'dt-no-alarmbox': !alarmBoxShown }">
      <div class="dd-date">{{ dateStr }}</div>
      <div class="dd-time">{{ timeStr }}</div>
    </div>

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
    <PointInfoDialog ref="pointInfoRef" v-model="infoOpen" :point-key="infoKey" :anchor="infoAnchor" @saved="onPointSaved" />

    <!-- Beep sounds -->
    <audio ref="criticalSound" src="/sounds/critical.wav"></audio>
    <audio ref="nonCriticalSound" src="/sounds/noncritical.wav"></audio>
  </v-container>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
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
const clockDate = ref('')
const clockTime = ref('')
const alarmBoxShown = ref(false)
const slideshow = ref(false)
const slideProgress = ref(0) // 0-100, time elapsed toward the next slideshow advance
const barHidden = ref(false)

const infoOpen = ref(false)
const infoKey = ref(0)
const infoAnchor = ref(null)
const pointInfoRef = ref(null)

// --- Time Machine (historical replay) ---
const tmActive = ref(false)
const tmDate = ref('')
const tmSeconds = ref(0)
const todayStr = ref('')
let tmDebounce = null

const tmTimeStr = computed(() => fmtHMS(tmSeconds.value))
const tmSliderMax = computed(() => (tmDate.value === todayStr.value ? nowSeconds() : 86399))

// date/time shown top-right (left of the alarm box): replay time in Time Machine,
// otherwise the live wall clock
const dateStr = computed(() => (tmActive.value ? tmDate.value : clockDate.value))
const timeStr = computed(() => (tmActive.value ? tmTimeStr.value : clockTime.value))

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
let beepTimer = null
let slideTimer = null
let slideStart = 0

function openPoint(key, pos) {
  infoKey.value = key
  infoAnchor.value = pos || null
  infoOpen.value = true
}

// clear the selected-object highlight when the dialog closes
watch(infoOpen, (open) => {
  if (!open && engine) engine.clearHighlight()
})

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

// advance to the next screen, wrapping to the first at the end (loop)
function slideAdvance() {
  if (screenItems.value.length === 0) return
  const idx = screenItems.value.findIndex((it) => it.value === selectedScreen.value)
  const next = (idx + 1) % screenItems.value.length
  loadScreenByRef(screenItems.value[next].value)
}

// drive the slideshow: fill the ring over the interval, then load the next screen
function slideTick() {
  const interval = (getViewersConfig().ScreenViewer_SlideShowInterval || 10) * 1000
  const elapsed = Date.now() - slideStart
  if (elapsed >= interval) {
    slideStart = Date.now()
    slideProgress.value = 0
    slideAdvance()
  } else {
    slideProgress.value = Math.min(100, (elapsed / interval) * 100)
  }
}

function toggleSlideshow() {
  slideshow.value = !slideshow.value
  clearInterval(slideTimer)
  slideProgress.value = 0
  if (slideshow.value) {
    slideStart = Date.now()
    slideTimer = setInterval(slideTick, 100)
  }
}

function silenceBeep() {
  writeAck({ pointId: 0, action: OpcAcknowledge.SilenceBeep })
  alarmBeep.value = 0
}

// Jump to the Nth screen in the list (1-based, as in the legacy 1..0 shortcuts).
function selectScreenByIndex(n) {
  const it = screenItems.value[n - 1]
  if (it) loadScreenByRef(it.value)
}

// Jump to the screen whose title carries a "{X}" mnemonic matching the pressed key.
function selectScreenByLetter(ch) {
  const up = ch.toUpperCase()
  const it = screenItems.value.find((i) => {
    const p = i.title.indexOf('{')
    return p !== -1 && (i.title.charAt(p + 1) || '').toUpperCase() === up
  })
  if (it) {
    loadScreenByRef(it.value)
    return true
  }
  return false
}

// Toggle the top toolbar to maximize the screen area (legacy hideShowBar / F10 / numpad*).
function hideShowBar() {
  barHidden.value = !barHidden.value
}

// Serialize the current SVG and open it in a new window so it can be saved (legacy shift+enter).
function snapshotSvg() {
  if (!engine || !engine.svgEl) return
  const blob = new Blob([new XMLSerializer().serializeToString(engine.svgEl)], {
    type: 'image/svg+xml',
  })
  const url = URL.createObjectURL(blob)
  window.open(url, 'SVG Snapshot')
}

function onKeydown(e) {
  const tag = (e.target && e.target.tagName) || ''
  if (tag === 'INPUT' || tag === 'TEXTAREA' || (e.target && e.target.isContentEditable)) return

  // Escape: close the command dialog first (it's :persistent here, so it won't
  // close itself), then leave Time Machine, then close the point-info dialog.
  if (e.key === 'Escape') {
    if (pointInfoRef.value && pointInfoRef.value.commandOpen) {
      pointInfoRef.value.closeCommand()
    } else if (tmActive.value) {
      exitTimeMachine()
    } else {
      infoOpen.value = false
    }
    return
  }

  // F9: silence the alarm beep.
  if (e.key === 'F9') {
    silenceBeep()
    e.preventDefault()
    return
  }

  // F10 or numpad * : hide/show the toolbar.
  if (e.key === 'F10' || e.code === 'NumpadMultiply') {
    hideShowBar()
    e.preventDefault()
    return
  }

  // Shift+Enter: open a saveable SVG snapshot of the current screen.
  if (e.key === 'Enter' && e.shiftKey) {
    snapshotSvg()
    e.preventDefault()
    return
  }

  // Zoom / pan (numpad + main keys). Skip when a modifier is held so the legacy
  // point-navigation chords (shift/ctrl + arrows) don't get hijacked into panning.
  if (!e.shiftKey && !e.ctrlKey && !e.altKey && !e.metaKey) {
    switch (e.code) {
      case 'NumpadAdd':
      case 'Numpad9':
        zp('in')
        e.preventDefault()
        return
      case 'NumpadSubtract':
      case 'Numpad3':
        zp('out')
        e.preventDefault()
        return
      case 'ArrowUp':
      case 'Numpad8':
        zp('up')
        e.preventDefault()
        return
      case 'ArrowDown':
      case 'Numpad2':
        zp('down')
        e.preventDefault()
        return
      case 'ArrowLeft':
      case 'Numpad4':
        zp('left')
        e.preventDefault()
        return
      case 'ArrowRight':
      case 'Numpad6':
        zp('right')
        e.preventDefault()
        return
      case 'Home':
      case 'Numpad5':
      case 'Numpad7':
        zp('center')
        e.preventDefault()
        return
      default:
        break
    }
    // '+' / '-' on the main row also zoom (numpad handled above via e.code).
    if (e.key === '+') {
      zp('in')
      return
    }
    if (e.key === '-') {
      zp('out')
      return
    }
  }

  // Screen navigation: ',' previous, '.' next (main keys, no modifiers).
  if (!e.shiftKey && !e.ctrlKey && !e.altKey && !e.metaKey) {
    if (e.key === ',') {
      navAdjacent(-1)
      return
    }
    if (e.key === '.') {
      navAdjacent(1)
      return
    }
    // Digits 1..9 / 0 select the 1st..9th / 10th screen in the list.
    if (e.code && e.code.startsWith('Digit')) {
      const d = e.code.slice(5)
      selectScreenByIndex(d === '0' ? 10 : Number(d))
      return
    }
    // Letter mnemonics from "{X}" in the screen titles.
    if (e.key.length === 1 && /[a-zA-Z]/.test(e.key)) {
      selectScreenByLetter(e.key)
    }
  }
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
    // update the displayed date/time only on a data refresh, using the server time
    onUpdateTime: (d) => {
      clockDate.value = d.toLocaleDateString()
      clockTime.value = d.toLocaleTimeString()
    },
    onAlarmBox: (visible) => (alarmBoxShown.value = visible),
  })

  const initial =
    route.value.query.screen ||
    route.value.query.SELTELA ||
    route.value.query.SVGFILE ||
    (screenItems.value[0] && screenItems.value[0].value)
  if (initial) loadScreenByRef(initial)

  window.addEventListener('keydown', onKeydown)
  // date/time is driven by the engine's onUpdateTime (server data timestamp), not a wall clock
  beepTimer = setInterval(() => {
    if (alarmBeep.value && nonCriticalSound.value) nonCriticalSound.value.play().catch(() => {})
  }, 1500)
})

onUnmounted(() => {
  if (engine) engine.dispose()
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
  /* The mimic is operated by mouse/keyboard, not read as a document — keep
     panning, double-clicks and shortcuts from selecting labels/SVG text. */
  user-select: none;
  -webkit-user-select: none;
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
/* Display update date/time — stacked, anchored just left of the alarm box
   (alarm box is fixed at right:0, width 372). */
.display-datetime {
  position: fixed;
  top: 50px;
  right: 380px;
  z-index: 7;
  text-align: right;
  line-height: 1.05;
  pointer-events: none;
  font-family: tahoma, sans-serif;
  /* follow the theme like the rest of the toolbar text */
  color: rgb(var(--v-theme-on-surface));
}
.display-datetime .dd-date {
  font-size: 10px;
  opacity: 0.9;
}
.display-datetime .dd-time {
  font-size: 15px;
  font-weight: 700;
}
/* no alarm box -> use the full right edge */
.display-datetime.dt-no-alarmbox {
  right: 8px;
}
.display-datetime.tm-clock {
  color: #ffb74d;
}
</style>
