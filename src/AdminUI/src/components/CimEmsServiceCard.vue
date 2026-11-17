<template>
  <v-card variant="outlined" class="cim-card">
    <v-card-item prepend-icon="mdi-transmission-tower">
      <v-card-title>{{ $t('admin.cimEmsService.title') }}</v-card-title>
      <v-card-subtitle class="cim-wrap">{{ $t('admin.cimEmsService.subtitle') }}</v-card-subtitle>
      <template #append>
        <v-chip size="small" label variant="tonal" :color="envInfo.color" :prepend-icon="envInfo.icon">
          {{ envInfo.label }}
        </v-chip>
      </template>
    </v-card-item>

    <v-card-text>
      <!-- services: driver and demo field simulator -->
      <div class="text-label-medium text-uppercase text-medium-emphasis mb-2">{{ $t('admin.cimEmsService.services') }}</div>
      <div v-for="row in serviceRows" :key="row.key" class="cim-service">
        <div class="cim-service-name">
          <div class="text-body-medium font-weight-medium">{{ row.label }}</div>
          <div class="text-body-small text-medium-emphasis">
            <span class="admin-mono">{{ row.st.serviceName || '—' }}</span>
            <span v-if="row.st.node"> · {{ row.st.node }}</span>
          </div>
        </div>
        <v-chip size="small" label variant="tonal" :color="row.info.color" :prepend-icon="row.info.icon">
          {{ row.info.label }}
        </v-chip>
        <div class="d-flex ga-1">
          <v-btn
            icon
            size="small"
            variant="text"
            color="success"
            :title="$t('common.start')"
            :disabled="!canControl(row.st) || isRunning(row.st) || !envUsable"
            :loading="busy === row.key + ':start'"
            @click="control(row.key, 'start')"
          >
            <v-icon>mdi-play</v-icon>
          </v-btn>
          <v-btn
            icon
            size="small"
            variant="text"
            color="error"
            :title="$t('common.stop')"
            :disabled="!canControl(row.st) || !isRunning(row.st)"
            :loading="busy === row.key + ':stop'"
            @click="control(row.key, 'stop')"
          >
            <v-icon>mdi-stop</v-icon>
          </v-btn>
          <v-btn
            icon
            size="small"
            variant="text"
            color="warning"
            :title="$t('admin.systemSettings.restart')"
            :disabled="!canControl(row.st) || !isRunning(row.st) || !envUsable"
            :loading="busy === row.key + ':restart'"
            @click="control(row.key, 'restart')"
          >
            <v-icon>mdi-restart</v-icon>
          </v-btn>
        </div>
      </div>
      <v-alert v-if="loaded && !services.main.manageable" type="info" variant="tonal" density="compact" class="mt-2">
        {{ $t('admin.cimEmsService.controlUnavailable', { reason: services.main.reason || '—' }) }}
      </v-alert>
      <v-alert v-else-if="loaded && notInstalled.length" type="warning" variant="tonal" density="compact" class="mt-2">
        {{ $t('admin.cimEmsService.notInstalledHint', { service: notInstalled.join(', ') }) }}
      </v-alert>
      <div class="text-body-small text-medium-emphasis mt-2 cim-wrap">
        <v-icon size="14" class="me-1">mdi-information-outline</v-icon>{{ $t('admin.cimEmsService.instanceNote') }}
      </div>

      <!-- Python environment -->
      <v-divider class="my-4"></v-divider>
      <div class="text-label-medium text-uppercase text-medium-emphasis mb-2">{{ $t('admin.cimEmsService.environment') }}</div>
      <dl class="admin-kv">
        <dt>{{ $t('admin.cimEmsService.interpreter') }}</dt>
        <dd class="admin-mono cim-break">{{ env.interpreter || '—' }}</dd>
        <dt>{{ $t('admin.cimEmsService.pythonVersion') }}</dt>
        <dd>{{ env.python || '—' }}</dd>
      </dl>
      <v-table v-if="env.packages && env.packages.length" density="compact" class="cim-packages mt-2">
        <thead>
          <tr>
            <th>{{ $t('admin.cimEmsService.package') }}</th>
            <th>{{ $t('admin.cimEmsService.required') }}</th>
            <th>{{ $t('admin.cimEmsService.installed') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in env.packages" :key="p.name">
            <td class="admin-mono">{{ p.name }}</td>
            <td class="admin-mono">{{ p.required || $t('admin.cimEmsService.anyVersion') }}</td>
            <td>
              <span class="d-inline-flex align-center ga-1">
                <v-icon size="16" :color="pkgOk(p) ? 'success' : p.installed ? 'warning' : 'error'">
                  {{ pkgOk(p) ? 'mdi-check-circle-outline' : p.installed ? 'mdi-alert-outline' : 'mdi-close-circle-outline' }}
                </v-icon>
                <span class="admin-mono">{{ p.installed || '—' }}</span>
              </span>
            </td>
          </tr>
        </tbody>
      </v-table>
      <v-alert v-if="envAlert" :type="envAlert.type" variant="tonal" density="compact" class="mt-3">{{ envAlert.text }}</v-alert>
      <v-alert v-if="installBlocked" type="info" variant="tonal" density="compact" class="mt-3">
        {{ $t('admin.cimEmsService.stopFirst', { service: runningNames.join(', ') }) }}
      </v-alert>

      <div class="d-flex flex-wrap align-center ga-2 mt-3">
        <v-btn
          variant="text"
          prepend-icon="mdi-refresh"
          :disabled="installing || !!busy"
          :loading="busy === 'recheck'"
          @click="recheck"
        >
          {{ $t('admin.cimEmsService.checkAgain') }}
        </v-btn>
        <v-spacer></v-spacer>
        <v-btn
          :color="env.state === 'ok' ? undefined : 'primary'"
          :variant="env.state === 'ok' ? 'tonal' : 'flat'"
          prepend-icon="mdi-package-down"
          :disabled="!canInstall"
          :loading="installing || busy === 'install'"
          @click="confirmDialog = true"
        >
          {{ env.state === 'ok' ? $t('admin.cimEmsService.reinstall') : $t('admin.cimEmsService.install') }}
        </v-btn>
      </div>

      <!-- install output -->
      <div v-if="install.state !== 'idle'" class="mt-3">
        <div class="d-flex align-center ga-2 mb-1">
          <v-icon size="16" :color="jobInfo.color">{{ jobInfo.icon }}</v-icon>
          <span class="text-body-small">{{ jobInfo.label }}</span>
          <v-spacer></v-spacer>
          <v-btn size="x-small" variant="text" @click="showLog = !showLog">
            {{ showLog ? $t('admin.cimEmsService.hideOutput') : $t('admin.cimEmsService.showOutput') }}
          </v-btn>
        </div>
        <v-progress-linear v-if="installing" indeterminate color="primary" class="mb-2"></v-progress-linear>
        <pre v-if="showLog" ref="logRef" class="admin-mono cim-log">{{ install.log }}</pre>
      </div>
    </v-card-text>

    <v-dialog v-model="confirmDialog" max-width="480">
      <v-card>
        <v-card-title>{{ $t('admin.cimEmsService.installConfirmTitle') }}</v-card-title>
        <v-card-text class="cim-wrap">{{ $t('admin.cimEmsService.installConfirm') }}</v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="confirmDialog = false">{{ $t('common.cancel') }}</v-btn>
          <v-btn color="primary" variant="flat" @click="startInstall">{{ $t('admin.cimEmsService.install') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-snackbar v-model="snackbar.show" :color="snackbar.color" :timeout="4000" location="bottom right">
      {{ snackbar.text }}
    </v-snackbar>
  </v-card>
</template>

<script setup>
  import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { postJson } from '../lib/adminList'

  // Admin > System card for the CIM-EMS driver (src/cim-ems): start / stop of its
  // platform services (cim_ems, cim_ems_demo_field) and its Python environment,
  // installed by the component's install script (server_realtime_auth
  // app/services/cim-ems-env.js).
  const props = defineProps({ active: { type: Boolean, default: true } })

  const { t } = useI18n()
  const snackbar = ref({ show: false, text: '', color: undefined })
  const notify = (text, color) => (snackbar.value = { show: true, text, color })
  const errText = (e) => (typeof e === 'string' ? e : e?.error || e?.message || String(e))

  const NO_SERVICE = { manageable: false, installed: false, state: 'UNKNOWN', serviceName: '', reason: '', node: '' }
  const services = ref({ main: { ...NO_SERVICE }, demoField: { ...NO_SERVICE } })
  const env = ref({ state: 'unknown', interpreter: '', python: '', packages: [], reason: '' })
  const install = ref({ state: 'idle', log: '' })
  const loaded = ref(false)
  const busy = ref('')
  const confirmDialog = ref(false)
  const showLog = ref(false)
  const logRef = ref(null)

  const installing = computed(() => install.value.state === 'running')
  const isRunning = (st) => st.state === 'RUNNING'
  const canControl = (st) => st.manageable && st.installed && !busy.value && !installing.value
  // a missing package stops the driver at once; an other version may still run
  const envUsable = computed(() => !['missing', 'unsupported'].includes(env.value.state))
  const pkgOk = (p) => !!p.installed && (!p.required || p.installed === p.required)

  const BUSY_STATES = ['RUNNING', 'STARTING', 'STOPPING', 'BACKOFF']
  const runningNames = computed(() =>
    Object.values(services.value)
      .filter((s) => s.manageable && BUSY_STATES.includes(s.state))
      .map((s) => s.serviceName)
  )
  const installBlocked = computed(() => !installing.value && runningNames.value.length > 0)
  const canInstall = computed(
    () => loaded.value && !installing.value && !busy.value && env.value.state !== 'unsupported' && !installBlocked.value
  )
  const notInstalled = computed(() =>
    Object.values(services.value)
      .filter((s) => s.manageable && !s.installed)
      .map((s) => s.serviceName)
  )

  function serviceInfo(s) {
    if (!loaded.value) return { label: '…', color: undefined, icon: 'mdi-help-circle-outline' }
    if (!s.manageable) return { label: t('admin.mcpServer.stateUnknown'), color: undefined, icon: 'mdi-help-circle-outline' }
    if (!s.installed)
      return { label: t('admin.mcpServer.stateNotInstalled'), color: undefined, icon: 'mdi-package-variant-closed-remove' }
    const map = {
      RUNNING: ['stateRunning', 'success', 'mdi-play-circle-outline'],
      STARTING: ['stateStarting', 'info', 'mdi-progress-clock'],
      STOPPING: ['stateStopping', 'info', 'mdi-progress-clock'],
      STOPPED: ['stateStopped', undefined, 'mdi-stop-circle-outline'],
      BACKOFF: ['stateFailed', 'error', 'mdi-alert-circle-outline'],
      FATAL: ['stateFailed', 'error', 'mdi-alert-circle-outline'],
    }
    const m = map[s.state]
    if (!m) return { label: t('admin.mcpServer.stateUnknown'), color: undefined, icon: 'mdi-help-circle-outline' }
    return { label: t('admin.mcpServer.' + m[0]), color: m[1], icon: m[2] }
  }
  const serviceRows = computed(() => [
    { key: 'main', label: t('admin.cimEmsService.driver'), st: services.value.main, info: serviceInfo(services.value.main) },
    {
      key: 'demoField',
      label: t('admin.cimEmsService.demoField'),
      st: services.value.demoField,
      info: serviceInfo(services.value.demoField),
    },
  ])

  const envInfo = computed(() => {
    if (!loaded.value) return { label: '…', color: undefined, icon: 'mdi-help-circle-outline' }
    if (installing.value) return { label: t('admin.cimEmsService.envInstalling'), color: 'info', icon: 'mdi-progress-download' }
    const map = {
      ok: ['envOk', 'success', 'mdi-language-python'],
      outdated: ['envOutdated', 'warning', 'mdi-alert-outline'],
      missing: ['envMissing', 'error', 'mdi-package-variant-closed-remove'],
      unsupported: ['envUnsupported', undefined, 'mdi-cancel'],
    }
    const m = map[env.value.state] || ['envUnknown', undefined, 'mdi-help-circle-outline']
    return { label: t('admin.cimEmsService.' + m[0]), color: m[1], icon: m[2] }
  })
  const envAlert = computed(() => {
    if (!loaded.value || installing.value) return null
    const s = env.value.state
    if (s === 'missing') return { type: 'warning', text: t('admin.cimEmsService.missingHint') }
    if (s === 'outdated') return { type: 'warning', text: t('admin.cimEmsService.outdatedHint') }
    if (s === 'unsupported') return { type: 'info', text: t('admin.cimEmsService.unsupportedHint', { reason: env.value.reason }) }
    if (s === 'unknown') return { type: 'warning', text: t('admin.cimEmsService.unknownHint', { reason: env.value.reason || '—' }) }
    return null
  })
  const jobInfo = computed(() => {
    const j = install.value
    if (j.state === 'running') return { label: t('admin.cimEmsService.jobRunning'), color: 'info', icon: 'mdi-progress-download' }
    const at = j.endedAt ? new Date(j.endedAt).toLocaleString() : ''
    if (j.state === 'ok') return { label: t('admin.cimEmsService.jobOk', { at }), color: 'success', icon: 'mdi-check-circle-outline' }
    return { label: t('admin.cimEmsService.jobFailed', { at }), color: 'error', icon: 'mdi-alert-circle-outline' }
  })

  async function fetchStatus({ recheck = false, quiet = true } = {}) {
    try {
      const json = await (await fetch('/Invoke/auth/getCimEmsService' + (recheck ? '?recheck=true' : ''))).json()
      if (json.error) throw json
      services.value = { main: { ...NO_SERVICE, ...json.services?.main }, demoField: { ...NO_SERVICE, ...json.services?.demoField } }
      // during an install the server skips the check: keep the last result
      if (json.environment) env.value = json.environment
      const wasRunning = installing.value
      install.value = json.install || { state: 'idle', log: '' }
      if (wasRunning && !installing.value) {
        notify(install.value.state === 'ok' ? t('admin.cimEmsService.installOk') : t('admin.cimEmsService.installFailed'),
          install.value.state === 'ok' ? 'success' : 'error')
        if (install.value.state !== 'ok') showLog.value = true
      }
      loaded.value = true
    } catch (err) {
      if (!quiet) notify(t('common.error') + ' ' + errText(err), 'error')
    }
    schedulePoll()
  }

  async function recheck() {
    busy.value = 'recheck'
    await fetchStatus({ recheck: true, quiet: false })
    busy.value = ''
  }

  async function control(key, action) {
    busy.value = key + ':' + action
    try {
      const json = await postJson('/Invoke/auth/controlCimEmsService', { service: key, action })
      if (json.error) throw json
      notify(t('admin.cimEmsService.actionDone', { action: t('admin.mcpServer.action_' + action) }), 'success')
    } catch (err) {
      notify(t('common.error') + ' ' + errText(err), 'error')
    } finally {
      busy.value = ''
      fetchStatus()
    }
  }

  async function startInstall() {
    confirmDialog.value = false
    busy.value = 'install'
    try {
      const json = await postJson('/Invoke/auth/installCimEmsDependencies', {})
      if (json.error) throw json
      install.value = json.install
      showLog.value = true
    } catch (err) {
      notify(t('common.error') + ' ' + errText(err), 'error')
    } finally {
      busy.value = ''
      fetchStatus()
    }
  }

  // keep the newest install output in view
  watch(
    () => install.value.log,
    async () => {
      await nextTick()
      if (logRef.value) logRef.value.scrollTop = logRef.value.scrollHeight
    }
  )

  // poll while the System section is shown: every 2 s during an install, else 5 s
  let timer = null
  function stopPolling() {
    if (timer) clearTimeout(timer)
    timer = null
  }
  function schedulePoll() {
    stopPolling()
    if (!props.active) return
    timer = setTimeout(() => fetchStatus(), installing.value ? 2000 : 5000)
  }
  watch(
    () => props.active,
    (on) => (on ? fetchStatus() : stopPolling())
  )
  onMounted(() => fetchStatus({ quiet: false }))
  onUnmounted(stopPolling)

  defineExpose({ refresh: () => fetchStatus() })
</script>

<style scoped>
  .cim-card {
    border-color: rgba(var(--v-border-color), var(--v-border-opacity));
  }
  .cim-wrap {
    white-space: normal;
  }
  .cim-break {
    word-break: break-all;
  }
  .cim-service {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 4px 0;
  }
  .cim-service-name {
    flex: 1 1 auto;
    min-width: 0;
  }
  .cim-packages {
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 8px;
  }
  .cim-log {
    margin: 0;
    max-height: 240px;
    overflow: auto;
    font-size: 12px;
    white-space: pre-wrap;
    word-break: break-all;
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 8px;
    padding: 8px 12px;
  }
  @media (max-width: 600px) {
    .cim-service {
      flex-wrap: wrap;
    }
  }
</style>
