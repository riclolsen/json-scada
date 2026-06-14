<template>
  <div>
    <!-- Point Info dialog -->
    <v-dialog v-model="open" max-width="520" scrollable>
      <v-card v-if="info.point">
        <v-card-title class="text-subtitle-1">
          {{ info.point.key }} : {{ info.point.tag }}
        </v-card-title>
        <v-card-subtitle>{{ info.point.station }} - {{ info.point.descr }}</v-card-subtitle>
        <v-card-text>
          <div class="mb-2">
            <b>{{ $t('tabularViewer.dlg.value') }}:</b> {{ info.point.valueStr }}
          </div>
          <div class="mb-2">
            <b>{{ $t('tabularViewer.dlg.quality') }}:</b> {{ info.qualityText }}
          </div>

          <template v-if="info.isAnalog">
            <v-text-field v-model="info.hiLimit" :label="$t('tabularViewer.dlg.hiLimit')" type="number" density="compact" variant="outlined" hide-details class="mb-2" :disabled="!userHasRight('enterLimits')"></v-text-field>
            <v-text-field v-model="info.loLimit" :label="$t('tabularViewer.dlg.loLimit')" type="number" density="compact" variant="outlined" hide-details class="mb-2" :disabled="!userHasRight('enterLimits')"></v-text-field>
            <v-text-field v-model="info.hysteresis" :label="$t('tabularViewer.dlg.hysteresis')" type="number" density="compact" variant="outlined" hide-details class="mb-2" :disabled="!userHasRight('enterLimits')"></v-text-field>
          </template>

          <v-textarea v-model="info.annotation" :label="$t('tabularViewer.dlg.annotation')" rows="2" density="compact" variant="outlined" hide-details class="mb-2" :disabled="!userHasRight('enterAnnotations')" @update:model-value="updateCommandBlocked"></v-textarea>
          <v-textarea v-model="info.notes" :label="$t('tabularViewer.dlg.notes')" rows="2" density="compact" variant="outlined" hide-details class="mb-2" :disabled="!userHasRight('enterNotes')"></v-textarea>
          <v-checkbox v-model="info.alarmDisabled" :label="$t('tabularViewer.dlg.alarmDisabled')" density="compact" hide-details :disabled="!userHasRight('disableAlarms')"></v-checkbox>

          <!-- Manual value entry / substitution -->
          <template v-if="info.canSubstitute">
            <v-text-field v-if="info.isAnalog" v-model="info.newValue" :label="$t('tabularViewer.dlg.manualValue')" type="number" density="compact" variant="outlined" hide-details class="mb-2"></v-text-field>
            <v-radio-group v-else v-model="info.newValue" inline hide-details density="compact">
              <v-radio :label="info.point.stateTextTrue" :value="1"></v-radio>
              <v-radio :label="info.point.stateTextFalse" :value="0"></v-radio>
            </v-radio-group>
          </template>

          <div v-if="info.commandBlocked" class="text-error text-caption mt-1">
            {{ $t('tabularViewer.dlg.blockedByAnnotation') }}
          </div>
        </v-card-text>
        <v-card-actions>
          <v-btn
            v-if="info.point.commandKey"
            color="primary"
            variant="tonal"
            :disabled="!userHasRight('sendCommands') || info.commandBlocked"
            @click="openCommand(info.point.commandKey)"
          >
            {{ $t('tabularViewer.dlg.command') }}
          </v-btn>
          <v-btn variant="text" @click="openTrend(info.point)">{{ $t('tabularViewer.dlg.trend') }}</v-btn>
          <v-btn variant="text" @click="openCurves(info.point)">{{ $t('tabularViewer.dlg.curves') }}</v-btn>
          <v-spacer></v-spacer>
          <v-btn color="success" variant="tonal" @click="saveProperties">{{ $t('common.save') }}</v-btn>
          <v-btn variant="text" @click="open = false">{{ $t('common.cancel') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- Command dialog -->
    <v-dialog v-model="command.open" max-width="440" scrollable content-class="cmd-dialog">
      <v-card v-if="command.point">
        <v-card-title class="text-subtitle-1">{{ $t('tabularViewer.cmd.title') }}</v-card-title>
        <v-card-subtitle>{{ command.point.station }} - {{ command.point.descr }}</v-card-subtitle>
        <v-card-text>
          <div class="mb-2 text-caption">{{ command.cmdTag }}</div>
          <template v-if="command.cmdType === 'digital'">
            <v-radio-group v-model="command.value" hide-details density="compact">
              <v-radio :label="command.stateTextTrue" :value="1"></v-radio>
              <v-radio :label="command.stateTextFalse" :value="0"></v-radio>
            </v-radio-group>
          </template>
          <template v-else-if="command.cmdType === 'analog'">
            <v-text-field v-model="command.value" :label="$t('tabularViewer.cmd.value')" type="number" density="compact" variant="outlined" hide-details></v-text-field>
          </template>
          <template v-else>
            <v-text-field v-model="command.value" :label="$t('tabularViewer.cmd.value')" density="compact" variant="outlined" hide-details></v-text-field>
          </template>
          <div class="mt-2 text-caption">{{ $t('tabularViewer.cmd.ackStatus') }}: <b>{{ command.ackText }}</b></div>
        </v-card-text>
        <v-card-actions>
          <v-btn color="error" variant="tonal" :disabled="!userHasRight('sendCommands') || commandCooldown || (command.cmdType === 'digital' && command.value === undefined)" @click="executeCommand">
            {{ $t('tabularViewer.cmd.execute') }}
          </v-btn>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="command.open = false">{{ $t('common.cancel') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script setup>
import { reactive, ref, computed, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import * as opc from '../lib/opcClient'
import { getViewersConfig } from '../lib/viewersConfig'
import { Flags } from '../lib/opcCodes'
import { userHasRight, storageAvailable, decodeQuality } from '../lib/viewerHelpers'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  pointKey: { type: [Number, String], default: 0 },
})
const emit = defineEmits(['update:modelValue'])

const { t } = useI18n()
const cfg = () => getViewersConfig()
let infoTimer = null
let logCnt = 0

const open = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const qmsg = () => ({
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
})

const info = reactive({
  point: null,
  qualityText: '',
  isAnalog: false,
  canSubstitute: false,
  commandBlocked: false,
  hiLimit: '',
  loLimit: '',
  hysteresis: '',
  annotation: '',
  notes: '',
  alarmDisabled: false,
  newValue: undefined,
})
const command = reactive({
  open: false,
  point: null,
  cmdKey: 0,
  cmdTag: '',
  cmdType: 'digital',
  stateTextTrue: 'ON',
  stateTextFalse: 'OFF',
  value: undefined,
  ackText: '',
  handle: null,
})
const commandCooldown = ref(false)
let cooldownTimer = null

// Load the point whenever the dialog opens with a key
watch(
  () => [props.modelValue, props.pointKey],
  ([isOpen, key]) => {
    if (isOpen && key) loadPoint(key)
    if (!isOpen) clearInterval(infoTimer)
  }
)

async function loadPoint(pointKey) {
  const pts = await opc.readPoints([pointKey], true, cfg())
  if (pts.length === 0) return
  const p = pts[0]
  info.point = p
  info.isAnalog = (p.flags & Flags.ANALOG) !== 0
  info.qualityText = decodeQuality(p.flags, qmsg())
  info.hiLimit = isFinite(p.hiLimit) ? p.hiLimit : ''
  info.loLimit = isFinite(p.loLimit) ? p.loLimit : ''
  info.hysteresis = p.hysteresis ?? ''
  info.annotation = p.annotation || ''
  info.notes = p.notes || ''
  info.alarmDisabled = p.alarmDisabled
  info.newValue = undefined
  info.canSubstitute =
    p.manual && (userHasRight('enterManuals') || userHasRight('substituteValues'))
  updateCommandBlocked()
  clearInterval(infoTimer)
  infoTimer = setInterval(async () => {
    if (!props.modelValue || !info.point) return
    const upd = await opc.readPoints([info.point.key], true, cfg())
    if (upd.length) {
      info.point.valueStr = upd[0].valueStr
      info.qualityText = decodeQuality(upd[0].flags, qmsg())
    }
  }, 2000)
}

function updateCommandBlocked() {
  info.commandBlocked = !!(info.point && info.point.commandKey) && info.annotation.trim() !== ''
}

async function saveProperties() {
  const p = info.point
  const li = parseFloat(info.loLimit)
  const ls = parseFloat(info.hiLimit)
  const hs = parseFloat(info.hysteresis)
  const propsToWrite = {
    alarmDisabled: !!info.alarmDisabled,
    annotation: info.annotation,
    loLimit: isNaN(li) ? -Number.MAX_VALUE : li,
    hiLimit: isNaN(ls) ? Number.MAX_VALUE : ls,
    hysteresis: isNaN(hs) ? 0 : hs,
    notes: info.notes,
  }
  if (info.newValue !== undefined && info.newValue !== '') {
    const nv = parseFloat(info.newValue)
    if (!isNaN(nv)) {
      propsToWrite.newValue = nv
      propsToWrite.substituted = true
    }
  }
  await opc.writeProperties({ pointKey: p.key, props: propsToWrite })
  updateCommandBlocked()
  open.value = false
}

async function openCommand(cmdKey) {
  const pts = await opc.readPoints([cmdKey], true, cfg())
  if (pts.length === 0) return
  const c = pts[0]
  command.point = info.point || c
  command.cmdKey = cmdKey
  command.cmdTag = c.tag
  command.cmdType = c.type
  command.stateTextTrue = c.stateTextTrue || 'ON'
  command.stateTextFalse = c.stateTextFalse || 'OFF'
  command.value = c.type === 'digital' ? undefined : c.valueRaw
  command.ackText = ''
  command.handle = null
  clearTimeout(cooldownTimer)
  commandCooldown.value = false
  command.open = true
}

async function executeCommand() {
  const isString = command.cmdType !== 'digital' && command.cmdType !== 'analog'
  let value = command.value
  if (!isString) value = parseFloat(value)
  command.ackText = '...'
  const res = await opc.writeCommand({ pointKey: command.cmdKey, value, isString })
  if (!res.ok) {
    command.ackText = t('common.error')
    return
  }
  command.handle = res.handle
  commandCooldown.value = true
  cooldownTimer = setTimeout(() => { commandCooldown.value = false }, 5000)
  logCommand(command.cmdKey, value)
  pollCommandAck()
}

async function pollCommandAck() {
  if (!command.open || !command.cmdKey) return
  const r = await opc.readCommandAck({ cmdKey: command.cmdKey, clientHandle: command.handle })
  switch (r.status) {
    case 'ok':
      command.ackText = t('tabularViewer.cmd.ok')
      break
    case 'waiting':
      command.ackText = '???'
      setTimeout(pollCommandAck, 1000)
      break
    case 'rejected':
      command.ackText = r.message
        ? t('tabularViewer.cmd.canceled') + ': ' + r.message
        : t('tabularViewer.cmd.rejected')
      break
    default:
      setTimeout(pollCommandAck, 1000)
  }
}

function logCommand(point, value) {
  if (!storageAvailable('localStorage')) return
  const stored = localStorage.getItem('lastlogcnt')
  if (stored !== null) logCnt = parseInt(stored)
  logCnt = (logCnt + 1) % 1000
  const idx = ('00' + logCnt).slice(-3)
  localStorage[idx] = new Date().toString() + ' Point:' + point + ' Value:' + value
  localStorage['lastlogcnt'] = logCnt
}

function openTrend(p) {
  window.open('/trend.html?NPONTO=' + p.key, 'Trend ' + p.key, 'height=400,width=700,resizable=yes')
}
function openCurves(p) {
  const url =
    p.flags & Flags.ANALOG
      ? '/grafana/d/78X6BmvMk/json-scada-history-analog?var-point_tag=' + p.tag
      : '/grafana/d/LsXOaz47z/json-scada-history-digital?var-point_tag=' + p.tag
  window.open(url, 'History ' + p.tag, 'height=600,width=1000,resizable=yes')
}

// Release cooldown when command dialog closes
watch(() => command.open, (isOpen) => {
  if (!isOpen) {
    clearTimeout(cooldownTimer)
    commandCooldown.value = false
  }
})

onUnmounted(() => {
  clearInterval(infoTimer)
  clearTimeout(cooldownTimer)
})
</script>

<style>
/* Shift command dialog down so it doesn't cover info dialog's manual value field */
.cmd-dialog {
  transform: translateY(60px) !important;
}
</style>
