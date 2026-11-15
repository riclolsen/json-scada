<template>
  <v-card variant="outlined" class="mcp-card">
    <v-card-item prepend-icon="mdi-robot-outline">
      <v-card-title>{{ $t('admin.mcpServer.title') }}</v-card-title>
      <v-card-subtitle class="mcp-wrap">{{ $t('admin.mcpServer.subtitle') }}</v-card-subtitle>
      <template #append>
        <v-chip size="small" label variant="tonal" :color="stateInfo.color" :prepend-icon="stateInfo.icon">
          {{ stateInfo.label }}
        </v-chip>
      </template>
    </v-card-item>

    <v-card-text>
      <!-- service: state, address, start / stop / restart -->
      <div class="mcp-status">
        <dl class="admin-kv">
          <dt>{{ $t('admin.mcpServer.service') }}</dt>
          <dd>
            <span class="admin-mono">{{ service.serviceName || '—' }}</span>
            <span v-if="service.node" class="text-medium-emphasis"> · {{ service.node }}</span>
          </dd>
          <dt>{{ $t('admin.mcpServer.endpoint') }}</dt>
          <dd class="d-flex align-center ga-1">
            <v-icon size="14" :color="listening ? 'success' : undefined" :title="listenTitle">
              {{ listening ? 'mdi-lan-connect' : 'mdi-lan-disconnect' }}
            </v-icon>
            <span class="admin-mono mcp-break">{{ endpoint }}</span>
            <v-btn icon size="x-small" variant="text" :title="$t('admin.common.copy')" @click="copy(endpoint)">
              <v-icon size="16">mdi-content-copy</v-icon>
            </v-btn>
          </dd>
        </dl>
        <div class="d-flex flex-wrap ga-2">
          <v-btn
            color="success"
            variant="tonal"
            prepend-icon="mdi-play"
            :disabled="!canControl || isRunning"
            :loading="busy === 'start'"
            @click="control('start')"
          >
            {{ $t('common.start') }}
          </v-btn>
          <v-btn
            color="error"
            variant="tonal"
            prepend-icon="mdi-stop"
            :disabled="!canControl || !isRunning"
            :loading="busy === 'stop'"
            @click="control('stop')"
          >
            {{ $t('common.stop') }}
          </v-btn>
          <v-btn
            color="warning"
            variant="tonal"
            prepend-icon="mdi-restart"
            :disabled="!canControl || !isRunning"
            :loading="busy === 'restart'"
            @click="control('restart')"
          >
            {{ $t('admin.systemSettings.restart') }}
          </v-btn>
        </div>
      </div>
      <v-alert v-if="loaded && !service.manageable" type="info" variant="tonal" density="compact" class="mt-3">
        {{ $t('admin.mcpServer.controlUnavailable', { reason: service.reason || '—' }) }}
      </v-alert>
      <v-alert v-else-if="loaded && !service.installed" type="warning" variant="tonal" density="compact" class="mt-3">
        {{ $t('admin.mcpServer.notInstalledHint', { service: service.serviceName }) }}
      </v-alert>

      <!-- settings -->
      <v-form ref="formRef" class="mt-4" @submit.prevent="save(false)">
        <div class="text-label-medium text-uppercase text-medium-emphasis mb-2">{{ $t('admin.mcpServer.connection') }}</div>
        <div class="mcp-row">
          <v-combobox
            v-model="form.ipBind"
            :items="bindItems"
            :label="$t('admin.mcpServer.bindAddress')"
            :hint="$t('admin.mcpServer.bindHint')"
            :rules="[rules.bind]"
            persistent-hint
            variant="outlined"
            density="comfortable"
          ></v-combobox>
          <v-text-field
            v-model.number="form.httpPort"
            type="number"
            min="1"
            max="65535"
            :label="$t('admin.mcpServer.port')"
            :rules="[rules.port]"
            variant="outlined"
            density="comfortable"
            class="mcp-port"
          ></v-text-field>
        </div>
        <v-combobox
          v-model="form.allowedHosts"
          :label="$t('admin.mcpServer.allowedHosts')"
          :hint="$t('admin.mcpServer.allowedHostsHint')"
          :rules="[rules.hosts]"
          persistent-hint
          multiple
          chips
          closable-chips
          variant="outlined"
          density="comfortable"
          class="mt-3"
        ></v-combobox>
        <v-select
          v-model="form.logLevel"
          :items="logLevelItems"
          :label="$t('admin.mcpServer.logLevel')"
          variant="outlined"
          density="comfortable"
          class="mt-3"
          hide-details
        ></v-select>
        <v-alert v-if="exposed" type="warning" variant="tonal" density="compact" class="mt-3">
          {{ $t('admin.mcpServer.exposedWarning', { port: form.httpPort }) }}
          <span v-if="!form.allowedHosts.length"> {{ $t('admin.mcpServer.noHostCheck') }}</span>
        </v-alert>

        <v-divider class="my-4"></v-divider>
        <div class="text-label-medium text-uppercase text-medium-emphasis mb-1">{{ $t('admin.mcpServer.tools') }}</div>
        <div class="sys-setting">
          <div>
            <div class="text-body-medium font-weight-medium">{{ $t('admin.mcpServer.adminTools') }}</div>
            <div class="text-body-small text-medium-emphasis">{{ $t('admin.mcpServer.adminToolsHint') }}</div>
          </div>
          <v-switch v-model="form.adminTools" inset color="primary" hide-details></v-switch>
        </div>
        <template v-if="form.adminTools">
          <div class="mcp-row mt-2">
            <v-combobox
              v-model="form.adminUsername"
              :items="userNames"
              :label="$t('admin.mcpServer.adminUsername')"
              prepend-inner-icon="mdi-account-key-outline"
              variant="outlined"
              density="comfortable"
              hide-details="auto"
            ></v-combobox>
            <v-text-field
              v-model="form.adminPassword"
              :type="showPassword ? 'text' : 'password'"
              :label="$t('admin.mcpServer.adminPassword')"
              :placeholder="settings.adminPasswordSet ? $t('admin.mcpServer.passwordKeep') : ''"
              :persistent-placeholder="settings.adminPasswordSet"
              :append-inner-icon="showPassword ? 'mdi-eye-off' : 'mdi-eye'"
              autocomplete="new-password"
              variant="outlined"
              density="comfortable"
              hide-details="auto"
              @click:append-inner="showPassword = !showPassword"
            ></v-text-field>
          </div>
          <v-alert v-if="userProblem" type="warning" variant="tonal" density="compact" class="mt-3">{{ userProblem }}</v-alert>
          <v-text-field
            v-model="form.adminUrl"
            :label="$t('admin.mcpServer.adminUrl')"
            :hint="$t('admin.mcpServer.adminUrlHint')"
            :rules="[rules.url]"
            persistent-hint
            variant="outlined"
            density="comfortable"
            class="mt-3"
          ></v-text-field>
          <div class="sys-setting mt-1">
            <div>
              <div class="text-body-medium font-weight-medium">{{ $t('admin.mcpServer.cimEmsTools') }}</div>
              <div class="text-body-small text-medium-emphasis">{{ $t('admin.mcpServer.cimEmsToolsHint') }}</div>
            </div>
            <v-switch v-model="form.cimEmsTools" inset color="primary" hide-details></v-switch>
          </div>
        </template>

        <div class="text-body-small text-medium-emphasis mt-3 mcp-wrap">
          <v-icon size="14" class="me-1">mdi-information-outline</v-icon>{{ $t('admin.mcpServer.appliesNote') }}
        </div>
        <div class="d-flex flex-wrap align-center ga-2 mt-3">
          <v-chip v-if="dirty" size="small" label variant="tonal" color="warning">{{ $t('admin.mcpServer.unsaved') }}</v-chip>
          <v-spacer></v-spacer>
          <v-btn variant="text" :disabled="!dirty" @click="resetForm">{{ $t('common.cancel') }}</v-btn>
          <v-btn
            type="submit"
            :color="isRunning ? undefined : 'primary'"
            :variant="isRunning ? 'tonal' : 'flat'"
            prepend-icon="mdi-content-save-outline"
            :disabled="!dirty"
            :loading="busy === 'save'"
          >
            {{ $t('common.save') }}
          </v-btn>
          <v-btn
            v-if="isRunning && canControl"
            color="primary"
            variant="flat"
            prepend-icon="mdi-content-save-move-outline"
            :disabled="!dirty"
            :loading="busy === 'saveRestart'"
            @click="save(true)"
          >
            {{ $t('admin.mcpServer.saveRestart') }}
          </v-btn>
        </div>
      </v-form>

      <!-- client configuration snippet -->
      <v-expansion-panels variant="accordion" class="mt-4">
        <v-expansion-panel>
          <v-expansion-panel-title>
            <v-icon size="18" class="me-2">mdi-code-json</v-icon>{{ $t('admin.mcpServer.clientConfig') }}
          </v-expansion-panel-title>
          <v-expansion-panel-text>
            <div class="text-body-small text-medium-emphasis mb-2 mcp-wrap">{{ $t('admin.mcpServer.clientConfigHint') }}</div>
            <div class="mcp-snippet">
              <pre class="admin-mono">{{ clientConfig }}</pre>
              <v-btn icon size="x-small" variant="text" class="mcp-snippet-copy" :title="$t('admin.common.copy')" @click="copy(clientConfig)">
                <v-icon size="16">mdi-content-copy</v-icon>
              </v-btn>
            </div>
          </v-expansion-panel-text>
        </v-expansion-panel>
      </v-expansion-panels>
    </v-card-text>

    <v-snackbar v-model="snackbar.show" :color="snackbar.color" :timeout="4000" location="bottom right">
      {{ snackbar.text }}
    </v-snackbar>
  </v-card>
</template>

<script setup>
  import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { postJson, copyText } from '../lib/adminList'

  // Admin > System card for the MCP server's HTTP service (src/mcp-json-scada-db).
  // The settings live in systemSettings { key: 'mcpServer' } (server_realtime_auth
  // app/services/mcp-settings.js); the MCP server reads them when it starts.
  const props = defineProps({ active: { type: Boolean, default: true } })

  const { t } = useI18n()
  const snackbar = ref({ show: false, text: '', color: undefined })
  const notify = (text, color) => (snackbar.value = { show: true, text, color })
  const errText = (e) => (typeof e === 'string' ? e : e?.error || e?.message || String(e))

  const EMPTY = {
    ipBind: '127.0.0.1',
    httpPort: 6001,
    allowedHosts: [],
    logLevel: 1,
    adminTools: true,
    adminUsername: '',
    adminUrl: 'http://127.0.0.1:8080',
    cimEmsTools: true,
    adminPasswordSet: false,
  }
  const settings = ref({ ...EMPTY })
  const form = ref({ ...EMPTY, adminPassword: '' })
  const service = ref({ manageable: false, installed: false, state: 'UNKNOWN', serviceName: '', reason: '', node: '' })
  const listening = ref(false)
  const adminUser = ref('notSet')
  const loaded = ref(false)
  const busy = ref('')
  const showPassword = ref(false)
  const formRef = ref(null)
  const userNames = ref([])

  const isRunning = computed(() => service.value.state === 'RUNNING')
  const canControl = computed(() => service.value.manageable && service.value.installed && !busy.value)

  const stateInfo = computed(() => {
    const s = service.value
    if (!loaded.value) return { label: '…', color: undefined, icon: 'mdi-help-circle-outline' }
    if (s.manageable && !s.installed)
      return { label: t('admin.mcpServer.stateNotInstalled'), color: undefined, icon: 'mdi-package-variant-closed-remove' }
    const map = {
      RUNNING: ['stateRunning', 'success', 'mdi-play-circle-outline'],
      STARTING: ['stateStarting', 'info', 'mdi-progress-clock'],
      STOPPING: ['stateStopping', 'info', 'mdi-progress-clock'],
      STOPPED: ['stateStopped', undefined, 'mdi-stop-circle-outline'],
      BACKOFF: ['stateFailed', 'error', 'mdi-alert-circle-outline'],
      FATAL: ['stateFailed', 'error', 'mdi-alert-circle-outline'],
    }
    // without service control, whether something answers is all we know
    if (!s.manageable)
      return listening.value
        ? { label: t('admin.mcpServer.stateListening'), color: 'success', icon: 'mdi-lan-connect' }
        : { label: t('admin.mcpServer.stateUnknown'), color: undefined, icon: 'mdi-help-circle-outline' }
    const m = map[s.state]
    if (!m) return { label: t('admin.mcpServer.stateUnknown'), color: undefined, icon: 'mdi-help-circle-outline' }
    return { label: t('admin.mcpServer.' + m[0]), color: m[1], icon: m[2] }
  })

  // the address an AI client uses: a wildcard bind is reached through this site's host name
  const endpoint = computed(() => {
    const b = settings.value.ipBind
    let host = b === '0.0.0.0' || b === '::' ? window.location.hostname || 'localhost' : b
    if (host.includes(':') && !host.startsWith('[')) host = '[' + host + ']'
    return 'http://' + host + ':' + settings.value.httpPort + '/mcp'
  })
  const listenTitle = computed(() =>
    listening.value ? t('admin.mcpServer.listening') : t('admin.mcpServer.notListening')
  )
  const clientConfig = computed(() =>
    JSON.stringify({ mcpServers: { 'json-scada': { type: 'http', url: endpoint.value } } }, null, 2)
  )

  const bindItems = ['127.0.0.1', '0.0.0.0', '::1', '::']
  const logLevelItems = computed(() => [
    { value: 0, title: '0 – ' + t('common.logLevels.minimum') },
    { value: 1, title: '1 – ' + t('common.logLevels.basic') },
    { value: 2, title: '2 – ' + t('common.logLevels.detailed') },
    { value: 3, title: '3 – ' + t('common.logLevels.maximum') },
  ])
  const LOOPBACK = ['127.0.0.1', '::1', 'localhost']
  const exposed = computed(() => !LOOPBACK.includes(String(form.value.ipBind || '').trim()))

  // same rules as the server (mcp-settings.js); it validates again
  const IPV4 = /^(25[0-5]|2[0-4]\d|1?\d?\d)(\.(25[0-5]|2[0-4]\d|1?\d?\d)){3}$/
  const IPV6 = /^[0-9a-fA-F:.]*:[0-9a-fA-F:.]*$/
  const HOSTNAME = /^[A-Za-z0-9]([A-Za-z0-9-]{0,62})(\.[A-Za-z0-9]([A-Za-z0-9-]{0,62}))*$/
  const validHost = (h) =>
    (h.startsWith('[') && h.endsWith(']') && IPV6.test(h.slice(1, -1))) || IPV4.test(h) || HOSTNAME.test(h)
  const rules = {
    bind: (v) => {
      const s = String(v || '').trim()
      return s === 'localhost' || IPV4.test(s) || IPV6.test(s) || t('admin.mcpServer.invalidBind')
    },
    port: (v) => (Number.isInteger(Number(v)) && v >= 1 && v <= 65535) || t('admin.mcpServer.invalidPort'),
    hosts: (list) => {
      const bad = (list || []).find((h) => !validHost(String(h).trim().toLowerCase()))
      return bad === undefined || t('admin.mcpServer.invalidHost', { host: bad })
    },
    url: (v) => {
      try {
        const u = new URL(v)
        return ['http:', 'https:'].includes(u.protocol) || t('admin.mcpServer.invalidUrl')
      } catch (e) {
        return t('admin.mcpServer.invalidUrl')
      }
    },
  }

  const userProblem = computed(() => {
    if (!form.value.adminTools) return ''
    // judged for the saved user; an edited name is checked after saving
    if (form.value.adminUsername !== settings.value.adminUsername) return ''
    if (!settings.value.adminUsername || !settings.value.adminPasswordSet) return t('admin.mcpServer.userNotSet')
    if (adminUser.value === 'notFound') return t('admin.mcpServer.userNotFound', { user: settings.value.adminUsername })
    if (adminUser.value === 'notAdmin') return t('admin.mcpServer.userNotAdmin', { user: settings.value.adminUsername })
    return ''
  })

  const FIELDS = ['ipBind', 'httpPort', 'allowedHosts', 'logLevel', 'adminTools', 'adminUsername', 'adminUrl', 'cimEmsTools']
  const dirty = computed(
    () =>
      form.value.adminPassword !== '' ||
      FIELDS.some((k) => JSON.stringify(form.value[k]) !== JSON.stringify(settings.value[k]))
  )

  function resetForm() {
    form.value = { ...settings.value, allowedHosts: [...(settings.value.allowedHosts || [])], adminPassword: '' }
    showPassword.value = false
  }

  async function fetchStatus({ withSettings = false } = {}) {
    try {
      const json = await (await fetch('/Invoke/auth/getMcpServer')).json()
      if (json.error) throw json
      service.value = json.service || service.value
      listening.value = !!json.listening
      adminUser.value = json.adminUser || 'notSet'
      // a poll keeps the admin's unsaved edits
      const keepEdits = !withSettings && dirty.value
      settings.value = { ...EMPTY, ...json.settings }
      if (!keepEdits) resetForm()
      loaded.value = true
    } catch (err) {
      if (withSettings) notify(t('common.error') + ' ' + errText(err), 'error')
    }
  }

  async function fetchUserNames() {
    try {
      const list = await (await fetch('/Invoke/auth/listUsers')).json()
      if (Array.isArray(list)) userNames.value = list.map((u) => u.username).filter(Boolean)
    } catch (e) {
      // suggestions only
    }
  }

  async function save(restart) {
    const { valid } = (await formRef.value?.validate()) || { valid: true }
    if (!valid) return
    busy.value = restart ? 'saveRestart' : 'save'
    try {
      const f = form.value
      const body = {
        ipBind: String(f.ipBind || '').trim(),
        httpPort: Number(f.httpPort),
        allowedHosts: (f.allowedHosts || []).map((h) => String(h).trim().toLowerCase()).filter(Boolean),
        logLevel: f.logLevel,
        adminTools: f.adminTools,
        adminUsername: String(f.adminUsername || '').trim(),
        adminUrl: String(f.adminUrl || '').trim(),
        cimEmsTools: f.cimEmsTools,
        restart,
      }
      if (f.adminPassword) body.adminPassword = f.adminPassword
      const json = await postJson('/Invoke/auth/updateMcpServer', body)
      if (json.error) throw json
      settings.value = { ...EMPTY, ...json.settings }
      resetForm()
      notify(json.restarted ? t('admin.mcpServer.savedRestarted') : t('admin.mcpServer.saved'), 'success')
      await fetchStatus()
    } catch (err) {
      notify(t('common.error') + ' ' + errText(err), 'error')
    } finally {
      busy.value = ''
    }
  }

  async function control(action) {
    busy.value = action
    try {
      const json = await postJson('/Invoke/auth/controlMcpServer', { action })
      if (json.error) throw json
      notify(t('admin.mcpServer.actionDone', { action: t('admin.mcpServer.action_' + action) }), 'success')
    } catch (err) {
      notify(t('common.error') + ' ' + errText(err), 'error')
    } finally {
      busy.value = ''
      // the service needs a moment to start listening
      fetchStatus()
      setTimeout(fetchStatus, 1500)
    }
  }

  async function copy(text) {
    if (await copyText(text)) notify(t('admin.common.copied'), 'success')
  }

  // poll the state every 5 s while the System section is shown
  let timer = null
  function startPolling() {
    stopPolling()
    timer = setInterval(() => fetchStatus(), 5000)
  }
  function stopPolling() {
    if (timer) clearInterval(timer)
    timer = null
  }
  watch(
    () => props.active,
    (on) => {
      if (on) {
        fetchStatus()
        startPolling()
      } else stopPolling()
    }
  )
  onMounted(() => {
    fetchStatus({ withSettings: true })
    fetchUserNames()
    if (props.active) startPolling()
  })
  onUnmounted(stopPolling)

  function refresh() {
    fetchStatus()
    fetchUserNames()
  }
  defineExpose({ refresh })
</script>

<style scoped>
  .mcp-card {
    border-color: rgba(var(--v-border-color), var(--v-border-opacity));
  }
  .mcp-wrap {
    white-space: normal;
  }
  .mcp-break {
    word-break: break-all;
  }
  .mcp-status {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }
  .mcp-row {
    display: flex;
    gap: 12px;
    align-items: flex-start;
  }
  .mcp-row > * {
    flex: 1 1 0;
    min-width: 0;
  }
  .mcp-row > .mcp-port {
    flex: 0 0 130px;
  }
  .sys-setting {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 6px 0;
  }
  .mcp-snippet {
    position: relative;
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 8px;
    padding: 8px 36px 8px 12px;
    overflow-x: auto;
  }
  .mcp-snippet pre {
    margin: 0;
    font-size: 12px;
  }
  .mcp-snippet-copy {
    position: absolute;
    top: 4px;
    right: 4px;
  }
  @media (max-width: 600px) {
    .mcp-row {
      flex-direction: column;
      align-items: stretch;
    }
    .mcp-row > .mcp-port {
      flex: 1 1 auto;
    }
  }
</style>
