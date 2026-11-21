<template>
  <div class="system-tab">
    <AdminSummary :tiles="overviewTiles" />

    <div class="sys-grid">
      <!-- project backup: export and import -->
      <v-card variant="outlined" class="sys-card">
        <v-card-item prepend-icon="mdi-archive-arrow-down-outline">
          <v-card-title>{{ $t('admin.systemSettings.backupTitle') }}</v-card-title>
          <v-card-subtitle class="sys-wrap">{{ $t('admin.systemSettings.exportHint') }}</v-card-subtitle>
        </v-card-item>
        <v-card-text>
          <v-form ref="exportForm" class="sys-row" @submit.prevent="exportProject">
            <v-text-field
              v-model="projectName"
              :label="$t('admin.systemSettings.projectName')"
              :rules="[rules.projectName]"
              suffix=".zip"
              prepend-inner-icon="mdi-folder-zip-outline"
              variant="outlined"
              density="comfortable"
            ></v-text-field>
            <v-btn
              type="submit"
              color="primary"
              variant="flat"
              size="large"
              prepend-icon="mdi-download"
              :loading="exporting"
              :disabled="!projectNameValid"
            >
              {{ $t('admin.systemSettings.exportProject') }}
            </v-btn>
          </v-form>

          <v-divider class="my-4"></v-divider>

          <div class="text-title-small mb-1">{{ $t('admin.systemSettings.importProject') }}</div>
          <div class="text-body-small text-medium-emphasis mb-3 sys-wrap">{{ $t('admin.systemSettings.importHint') }}</div>
          <div
            class="drop-zone"
            :class="{ 'drop-zone-over': isDragging, 'drop-zone-file': !!file }"
            role="button"
            tabindex="0"
            @click="fileInput?.click()"
            @keyup.enter="fileInput?.click()"
            @dragenter.prevent="isDragging = true"
            @dragleave.prevent="isDragging = false"
            @dragover.prevent
            @drop.prevent="handleDrop"
          >
            <v-icon size="32" :color="file ? 'primary' : undefined">
              {{ file ? 'mdi-folder-zip' : 'mdi-cloud-upload-outline' }}
            </v-icon>
            <div v-if="file">
              <div class="font-weight-medium">{{ file.name }}</div>
              <div class="text-body-small text-medium-emphasis">{{ formatSize(file.size) }}</div>
            </div>
            <div v-else class="text-body-medium">{{ $t('admin.systemSettings.dragDropHere') }}</div>
            <v-btn v-if="file" icon variant="text" size="small" class="drop-clear" :title="$t('common.cancel')" @click.stop="file = null">
              <v-icon>mdi-close</v-icon>
            </v-btn>
            <input ref="fileInput" type="file" accept=".zip" hidden @change="handleFileUpload" />
          </div>
          <v-alert v-if="fileError" type="error" variant="tonal" density="compact" class="mt-3">{{ fileError }}</v-alert>
          <v-alert v-if="importSuccessful" type="success" variant="tonal" density="compact" class="mt-3">
            {{ $t('admin.systemSettings.importSuccessful') }}
          </v-alert>
          <div class="d-flex mt-3">
            <v-spacer></v-spacer>
            <v-btn
              color="warning"
              variant="flat"
              prepend-icon="mdi-upload"
              :disabled="!file"
              :loading="importing"
              @click="importDialog = true"
            >
              {{ $t('admin.systemSettings.importProject') }}
            </v-btn>
          </div>
        </v-card-text>
      </v-card>

      <!-- OS service management -->
      <v-card variant="outlined" class="sys-card">
        <v-card-item prepend-icon="mdi-cog-sync-outline">
          <v-card-title>{{ $t('admin.systemSettings.processManagement') }}</v-card-title>
          <template #append>
            <v-chip size="small" label variant="tonal" :color="processManagementEnabled ? 'success' : undefined">
              {{ processManagementEnabled ? $t('admin.systemSettings.on') : $t('admin.systemSettings.off') }}
            </v-chip>
          </template>
        </v-card-item>
        <v-card-text>
          <v-alert v-if="!processManagementEnabled" type="info" variant="tonal" density="compact" class="mb-3">
            {{ $t('admin.systemSettings.processManagementDisabled') }}
          </v-alert>
          <div class="sys-setting">
            <div>
              <div class="text-body-medium font-weight-medium">{{ $t('admin.systemSettings.autoManageServices') }}</div>
              <div class="text-body-small text-medium-emphasis">{{ $t('admin.systemSettings.autoManageServicesHint') }}</div>
            </div>
            <v-switch
              v-model="settings.autoManageServices"
              inset
              color="primary"
              hide-details
              :loading="savingSettings"
              @update:model-value="saveSystemSettings"
            ></v-switch>
          </div>
          <v-divider class="my-2"></v-divider>
          <div class="sys-setting">
            <div>
              <div class="text-body-medium font-weight-medium">{{ $t('admin.systemSettings.autoRestartOnConnectionChange') }}</div>
              <div class="text-body-small text-medium-emphasis">{{ $t('admin.systemSettings.autoRestartHint') }}</div>
            </div>
            <v-switch
              v-model="settings.autoRestartOnConnectionChange"
              inset
              color="primary"
              hide-details
              :loading="savingSettings"
              @update:model-value="saveSystemSettings"
            ></v-switch>
          </div>
          <div v-if="localNode" class="text-body-small text-medium-emphasis mt-3">
            <v-icon size="14" class="me-1">mdi-server-outline</v-icon>
            {{ $t('admin.systemSettings.localNode') }}: <span class="admin-mono">{{ localNode }}</span>
          </div>
        </v-card-text>
      </v-card>

      <!-- data retention: soeData, alarmsData and hist, cleaned once a day by cs_data_processor -->
      <v-card variant="outlined" class="sys-card">
        <v-card-item prepend-icon="mdi-database-clock-outline">
          <v-card-title>{{ $t('admin.systemSettings.retentionTitle') }}</v-card-title>
          <v-card-subtitle class="sys-wrap">{{ $t('admin.systemSettings.retentionHint') }}</v-card-subtitle>
        </v-card-item>
        <v-card-text>
          <v-form ref="retentionForm" @submit.prevent="saveRetention">
            <v-text-field
              v-for="f in retentionFieldList"
              :key="f.key"
              v-model.number="retention[f.key]"
              type="number"
              :min="retentionLimits.min"
              :max="retentionLimits.max"
              step="1"
              :label="f.label"
              :suffix="$t('admin.systemSettings.retentionDays')"
              :prepend-inner-icon="f.icon"
              :rules="[rules.retention]"
              variant="outlined"
              density="comfortable"
              class="mb-1"
            ></v-text-field>
            <div class="d-flex">
              <v-spacer></v-spacer>
              <v-btn
                type="submit"
                color="primary"
                variant="flat"
                prepend-icon="mdi-content-save-outline"
                :loading="savingRetention"
                :disabled="!retentionValid"
              >
                {{ $t('common.save') }}
              </v-btn>
            </div>
          </v-form>
        </v-card-text>
      </v-card>

      <!-- restarts -->
      <v-card variant="outlined" class="sys-card">
        <v-card-item prepend-icon="mdi-restart">
          <v-card-title>{{ $t('admin.systemSettings.maintenanceTitle') }}</v-card-title>
        </v-card-item>
        <v-card-text>
          <div v-for="a in restartActions" :key="a.key" class="sys-setting">
            <div>
              <div class="text-body-medium font-weight-medium">{{ a.title }}</div>
              <div class="text-body-small text-medium-emphasis">{{ a.hint }}</div>
            </div>
            <v-btn color="warning" variant="tonal" :prepend-icon="a.icon" :loading="restarting === a.key" @click="confirmAction = a">
              {{ $t('admin.systemSettings.restart') }}
            </v-btn>
          </div>
        </v-card-text>
      </v-card>

      <!-- configuration clean-up (pruneConfiguration) -->
      <v-card variant="outlined" class="sys-card sys-danger">
        <v-card-item prepend-icon="mdi-broom">
          <v-card-title>{{ $t('admin.systemSettings.pruneTitle') }}</v-card-title>
          <v-card-subtitle class="sys-wrap">{{ $t('admin.systemSettings.pruneHint') }}</v-card-subtitle>
        </v-card-item>
        <v-card-text>
          <div class="prune-parts">
            <v-checkbox
              v-for="p in pruneParts"
              :key="p.key"
              v-model="pruneSel[p.key]"
              :label="p.label"
              density="compact"
              hide-details
              color="error"
              @update:model-value="pruneReport = null"
            ></v-checkbox>
          </div>
          <div v-if="pruneReport" class="prune-report mt-3">
            <div class="text-label-medium text-uppercase">
              {{ pruneReport.dryRun ? $t('admin.systemSettings.pruneWillDelete') : $t('admin.systemSettings.pruneDeleted') }}
            </div>
            <dl class="admin-kv">
              <template v-for="row in pruneRows" :key="row.label">
                <dt>{{ row.label }}</dt>
                <dd>
                  <span class="font-weight-medium">{{ row.count }}</span>
                  <span v-if="row.detail" class="text-body-small text-medium-emphasis"> — {{ row.detail }}</span>
                </dd>
              </template>
            </dl>
            <v-alert v-for="w in pruneReport.warnings || []" :key="w" type="warning" variant="tonal" density="compact" class="mt-2">
              {{ w }}
            </v-alert>
          </div>
          <div class="d-flex flex-wrap ga-2 mt-3">
            <v-spacer></v-spacer>
            <v-btn variant="tonal" prepend-icon="mdi-eye-outline" :loading="pruning === 'preview'" :disabled="!pruneAny" @click="prunePreview">
              {{ $t('admin.systemSettings.prunePreview') }}
            </v-btn>
            <v-btn
              color="error"
              variant="flat"
              prepend-icon="mdi-delete-sweep-outline"
              :disabled="!pruneReport || !pruneReport.dryRun || !pruneAny"
              @click="openPruneConfirm"
            >
              {{ $t('admin.systemSettings.pruneExecute') }}
            </v-btn>
          </div>
        </v-card-text>
      </v-card>

      <!-- MCP server (AI assistants) and CIM-EMS services -->
      <McpServerCard ref="mcpRef" :active="active" />
      <CimEmsServiceCard ref="cimRef" :active="active" />
    </div>

    <!-- confirm a restart -->
    <v-dialog :model-value="!!confirmAction" max-width="460" @update:model-value="(v) => !v && (confirmAction = null)">
      <v-card v-if="confirmAction">
        <v-card-title class="d-flex align-center ga-2">
          <v-icon color="warning">mdi-alert-outline</v-icon>{{ confirmAction.title }}
        </v-card-title>
        <v-card-text>{{ confirmAction.hint }}</v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="confirmAction = null">{{ $t('common.cancel') }}</v-btn>
          <v-btn color="warning" variant="flat" @click="runRestart(confirmAction)">{{ $t('common.confirm') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- confirm an import -->
    <v-dialog v-model="importDialog" max-width="500">
      <v-card>
        <v-card-title class="d-flex align-center ga-2">
          <v-icon color="warning">mdi-alert-outline</v-icon>{{ $t('admin.systemSettings.importConfirmTitle') }}
        </v-card-title>
        <v-card-text>{{ $t('admin.systemSettings.importConfirmText', { file: file?.name || '' }) }}</v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="importDialog = false">{{ $t('common.cancel') }}</v-btn>
          <v-btn color="warning" variant="flat" @click="importProject">{{ $t('admin.systemSettings.importProject') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- confirm a clean-up: the word must be typed -->
    <v-dialog v-model="pruneDialog" max-width="500">
      <v-card>
        <v-card-title class="d-flex align-center ga-2">
          <v-icon color="error">mdi-delete-alert-outline</v-icon>{{ $t('admin.systemSettings.pruneTitle') }}
        </v-card-title>
        <v-card-text>
          <p class="mb-3">{{ $t('admin.systemSettings.pruneConfirmText') }}</p>
          <v-text-field
            v-model="pruneWord"
            :label="$t('admin.systemSettings.pruneTypeToConfirm', { word: PRUNE_WORD })"
            variant="outlined"
            density="comfortable"
            autofocus
            @keyup.enter="pruneWord === PRUNE_WORD && pruneExecute()"
          ></v-text-field>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="pruneDialog = false">{{ $t('common.cancel') }}</v-btn>
          <v-btn color="error" variant="flat" :disabled="pruneWord !== PRUNE_WORD" :loading="pruning === 'execute'" @click="pruneExecute">
            {{ $t('admin.systemSettings.pruneExecute') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-snackbar v-model="snackbar.show" :color="snackbar.color" :timeout="4000" location="bottom right">
      {{ snackbar.text }}
    </v-snackbar>
  </div>
</template>

<script setup>
  import { ref, computed, onMounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import AdminSummary from './AdminSummary.vue'
  import McpServerCard from './McpServerCard.vue'
  import CimEmsServiceCard from './CimEmsServiceCard.vue'
  import { postJson, fileStamp } from '../lib/adminList'

  const { t } = useI18n()
  defineProps({ active: { type: Boolean, default: true } })
  const mcpRef = ref(null)
  const cimRef = ref(null)

  const snackbar = ref({ show: false, text: '', color: undefined })
  function notify(text, color) {
    snackbar.value = { show: true, text, color }
  }
  const errText = (e) => (typeof e === 'string' ? e : e?.error || e?.message || String(e))

  // ---- overview ----
  const counts = ref({})
  const overviewTiles = computed(() => {
    const c = counts.value
    const v = (k) => (c[k] === undefined ? '—' : Number(c[k]).toLocaleString())
    return [
      { key: 'tags', icon: 'mdi-tag-multiple-outline', color: 'primary', value: v('tags'), label: t('admin.tabs.tags') },
      { key: 'connections', icon: 'mdi-lan', color: 'primary', value: v('connections'), label: t('admin.tabs.protocolConnections') },
      { key: 'drivers', icon: 'mdi-cpu-64-bit', color: 'primary', value: v('drivers'), label: t('admin.tabs.protocolDriverInstances') },
      { key: 'users', icon: 'mdi-account-multiple-outline', color: 'primary', value: v('users'), label: t('admin.tabs.userManagement') },
      { key: 'roles', icon: 'mdi-shield-account-outline', color: 'primary', value: v('roles'), label: t('admin.tabs.rolesManagement') },
      { key: 'displays', icon: 'mdi-monitor-dashboard', color: 'primary', value: v('displays'), label: t('admin.rolesManagement.headers.displays') },
    ]
  })
  async function fetchCounts() {
    const len = async (url) => {
      try {
        const j = await (await fetch(url)).json()
        return Array.isArray(j) ? j.length : undefined
      } catch (e) {
        return undefined
      }
    }
    const [tags, connections, drivers, users, roles, displays] = await Promise.all([
      postJson('/Invoke/auth/listTags', { itemsPerPage: 1, page: 1, filter: {} })
        .then((j) => j.countTotal)
        .catch(() => undefined),
      len('/Invoke/auth/listProtocolConnections'),
      len('/Invoke/auth/listProtocolDriverInstances'),
      len('/Invoke/auth/listUsers'),
      len('/Invoke/auth/listRoles'),
      len('/Invoke/auth/listDisplays'),
    ])
    counts.value = { tags, connections, drivers, users, roles, displays }
  }

  // ---- service management settings ----
  const settings = ref({ autoManageServices: true, autoRestartOnConnectionChange: true })
  const processManagementEnabled = ref(false)
  const localNode = ref('')
  const savingSettings = ref(false)

  async function fetchSystemSettings() {
    try {
      const json = await (await fetch('/Invoke/auth/getSystemSettings')).json()
      if (json.error) throw json
      if (json.settings) settings.value = json.settings
      if (json.retentionLimits) retentionLimits.value = json.retentionLimits
      for (const f of RETENTION_FIELDS)
        if (typeof json.settings?.[f] === 'number') retention.value[f] = json.settings[f]
      processManagementEnabled.value = !!json.processManagementEnabled
      localNode.value = json.localNode || ''
    } catch (err) {
      notify(t('common.error') + ' ' + errText(err), 'error')
    }
  }

  async function saveSystemSettings() {
    savingSettings.value = true
    try {
      const json = await postJson('/Invoke/auth/updateSystemSettings', {
        autoManageServices: settings.value.autoManageServices,
        autoRestartOnConnectionChange: settings.value.autoRestartOnConnectionChange,
      })
      if (json.error) throw json
      if (json.settings) settings.value = { ...settings.value, ...json.settings }
      notify(t('admin.common.saved'), 'success')
    } catch (err) {
      notify(t('common.error') + ' ' + errText(err), 'error')
      fetchSystemSettings()
    } finally {
      savingSettings.value = false
    }
  }

  // ---- data retention (days) ----
  const RETENTION_FIELDS = ['soeRetentionDays', 'alarmsRetentionDays', 'histRetentionDays']
  const retention = ref({ soeRetentionDays: 60, alarmsRetentionDays: 60, histRetentionDays: 60 })
  const retentionLimits = ref({ min: 1, max: 36500 })
  const savingRetention = ref(false)
  const retentionFieldList = computed(() => [
    { key: 'soeRetentionDays', icon: 'mdi-format-list-bulleted', label: t('admin.systemSettings.retentionSoe') },
    { key: 'alarmsRetentionDays', icon: 'mdi-bell-outline', label: t('admin.systemSettings.retentionAlarms') },
    { key: 'histRetentionDays', icon: 'mdi-chart-line', label: t('admin.systemSettings.retentionHist') },
  ])
  const retentionOk = (v) =>
    Number.isInteger(v) && v >= retentionLimits.value.min && v <= retentionLimits.value.max
  const retentionValid = computed(() => RETENTION_FIELDS.every((f) => retentionOk(retention.value[f])))

  async function saveRetention() {
    if (!retentionValid.value) return
    savingRetention.value = true
    try {
      const json = await postJson(
        '/Invoke/auth/updateSystemSettings',
        Object.fromEntries(RETENTION_FIELDS.map((f) => [f, retention.value[f]]))
      )
      if (json.error) throw json
      notify(t('admin.common.saved'), 'success')
    } catch (err) {
      notify(t('common.error') + ' ' + errText(err), 'error')
      fetchSystemSettings()
    } finally {
      savingRetention.value = false
    }
  }

  function refresh() {
    fetchSystemSettings()
    fetchCounts()
    mcpRef.value?.refresh?.()
    cimRef.value?.refresh?.()
  }
  onMounted(refresh)
  defineExpose({ refresh })

  // ---- export ----
  // the server accepts a plain zip file name only (exportProject in auth.controller.js)
  const PROJECT_NAME_RE = /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,246}$/
  const projectName = ref('jsonscada_project_' + fileStamp().slice(0, 10))
  const projectNameValid = computed(() => PROJECT_NAME_RE.test(projectName.value || ''))
  const rules = {
    projectName: (v) => PROJECT_NAME_RE.test(v || '') || t('admin.systemSettings.projectNameRule'),
    retention: (v) =>
      retentionOk(v) ||
      t('admin.systemSettings.retentionRule', { min: retentionLimits.value.min, max: retentionLimits.value.max }),
  }
  const exporting = ref(false)
  async function exportProject() {
    if (!projectNameValid.value) return
    exporting.value = true
    const fileName = projectName.value + '.zip'
    try {
      const resp = await fetch('/Invoke/auth/exportProject', {
        method: 'post',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ project: { fileName } }),
      })
      // a failure answers JSON { error }, a success the zip file (starts with "PK")
      const blob = await resp.blob()
      const head = await blob.slice(0, 2).text()
      if (head !== 'PK') {
        let msg = resp.status + ' ' + resp.statusText
        try {
          msg = JSON.parse(await blob.text()).error || msg
        } catch (e) {
          // not JSON
        }
        throw new Error(msg)
      }
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = fileName
      document.body.appendChild(a)
      a.click()
      a.remove()
      setTimeout(() => URL.revokeObjectURL(a.href), 1000)
      notify(t('admin.systemSettings.exportDone', { file: fileName }), 'success')
    } catch (err) {
      notify(t('common.error') + ' ' + errText(err), 'error')
    } finally {
      exporting.value = false
    }
  }

  // ---- import ----
  const file = ref(null)
  const fileInput = ref(null)
  const fileError = ref('')
  const isDragging = ref(false)
  const importDialog = ref(false)
  const importing = ref(false)
  const importSuccessful = ref(false)
  const MAX_UPLOAD = 100 * 1024 * 1024 // express-fileupload limit of server_realtime_auth

  function pickFile(f) {
    importSuccessful.value = false
    if (!f) return
    if (!f.name.toLowerCase().endsWith('.zip')) {
      file.value = null
      fileError.value = t('admin.systemSettings.invalidFileType')
    } else if (f.size > MAX_UPLOAD) {
      file.value = null
      fileError.value = t('admin.systemSettings.fileTooLarge', { size: formatSize(MAX_UPLOAD) })
    } else {
      file.value = f
      fileError.value = ''
    }
  }
  function handleFileUpload(event) {
    pickFile(event?.target?.files?.[0])
    if (event?.target) event.target.value = ''
  }
  function handleDrop(event) {
    isDragging.value = false
    pickFile(event.dataTransfer?.files?.[0])
  }
  function formatSize(n) {
    if (n < 1024) return n + ' B'
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
    return (n / 1024 / 1024).toFixed(1) + ' MB'
  }

  async function importProject() {
    importDialog.value = false
    if (!file.value) return
    importing.value = true
    importSuccessful.value = false
    fileError.value = ''
    try {
      const formData = new FormData()
      formData.append('projectFileName', file.value.name)
      formData.append('projectFileData', file.value)
      const resp = await fetch('/Invoke/auth/importProject', { method: 'POST', body: formData })
      const json = await resp.json()
      if (json.error) throw json
      importSuccessful.value = true
      file.value = null
      fetchCounts()
    } catch (err) {
      fileError.value = errText(err)
    } finally {
      importing.value = false
    }
  }

  // ---- restarts ----
  const restartActions = computed(() => [
    {
      key: 'restartProtocols',
      icon: 'mdi-lan-pending',
      title: t('admin.systemSettings.restartProtocols'),
      hint: t('admin.systemSettings.restartProtocolsHint'),
    },
    {
      key: 'restartProcesses',
      icon: 'mdi-server-network',
      title: t('admin.systemSettings.restartProcesses'),
      hint: t('admin.systemSettings.restartProcessesHint'),
    },
  ])
  const confirmAction = ref(null)
  const restarting = ref('')
  async function runRestart(a) {
    confirmAction.value = null
    restarting.value = a.key
    try {
      const json = await postJson('/Invoke/auth/' + a.key, {})
      if (json.error) throw json
      notify(a.title + ': ' + t('admin.systemSettings.restartRequested'), 'success')
    } catch (err) {
      notify(t('common.error') + ' ' + errText(err), 'error')
    } finally {
      restarting.value = ''
    }
  }

  // ---- configuration clean-up: dry run first, then the typed confirmation ----
  const PRUNE_WORD = 'PRUNE'
  const pruneParts = computed(() => [
    { key: 'connections', label: t('admin.systemSettings.pruneConnections') },
    { key: 'tags', label: t('admin.systemSettings.pruneTags') },
    { key: 'cimModels', label: t('admin.systemSettings.pruneCim') },
    { key: 'displays', label: t('admin.systemSettings.pruneDisplays') },
  ])
  const pruneSel = ref({ connections: true, tags: true, cimModels: true, displays: true })
  const pruneAny = computed(() => Object.values(pruneSel.value).some(Boolean))
  const pruneReport = ref(null)
  const pruning = ref('')
  const pruneDialog = ref(false)
  const pruneWord = ref('')

  const pruneBody = (confirm) => ({ ...pruneSel.value, confirm })
  async function prunePreview() {
    pruning.value = 'preview'
    try {
      const json = await postJson('/Invoke/auth/pruneConfiguration', pruneBody(false))
      if (json.error) throw json
      pruneReport.value = json
    } catch (err) {
      notify(t('common.error') + ' ' + errText(err), 'error')
    } finally {
      pruning.value = ''
    }
  }
  function openPruneConfirm() {
    pruneWord.value = ''
    pruneDialog.value = true
  }
  async function pruneExecute() {
    if (pruneWord.value !== PRUNE_WORD) return
    pruning.value = 'execute'
    try {
      const json = await postJson('/Invoke/auth/pruneConfiguration', pruneBody(true))
      if (json.error) throw json
      pruneReport.value = json
      pruneDialog.value = false
      notify(t('admin.systemSettings.pruneDone'), 'success')
      fetchCounts()
    } catch (err) {
      notify(t('common.error') + ' ' + errText(err), 'error')
    } finally {
      pruning.value = ''
    }
  }
  const pruneRows = computed(() => {
    const r = pruneReport.value
    if (!r) return []
    const n = (part, key) => (r.dryRun ? part?.count : part?.[key] ?? part?.count)
    const rows = []
    if (r.connections) {
      const names = r.connections.names || []
      rows.push({
        label: t('admin.systemSettings.pruneConnections'),
        count: n(r.connections, 'deleted'),
        detail: names.slice(0, 6).join(', ') + (names.length > 6 ? '…' : ''),
      })
    }
    if (r.tags) rows.push({ label: t('admin.systemSettings.pruneTags'), count: n(r.tags, 'deleted') })
    if (r.cimModels) {
      const src = r.dryRun ? r.cimModels : r.cimModels.deleted || r.cimModels
      const total = Object.entries(src)
        .filter(([k, v]) => k !== 'modelIds' && typeof v === 'number')
        .reduce((a, [, v]) => a + v, 0)
      rows.push({ label: t('admin.systemSettings.pruneCim'), count: total })
    }
    if (r.displays) {
      const files = r.dryRun ? r.displays.files || [] : r.displays.deleted || []
      rows.push({
        label: t('admin.systemSettings.pruneDisplays'),
        count: files.length,
        detail: files.slice(0, 6).join(', ') + (files.length > 6 ? '…' : ''),
      })
    }
    return rows
  })
</script>

<style scoped>
  .sys-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 440px), 1fr));
    gap: 16px;
    align-items: start;
  }
  .sys-card {
    border-color: rgba(var(--v-border-color), var(--v-border-opacity));
  }
  .sys-danger {
    border-color: rgba(var(--v-theme-error), 0.45);
  }
  .sys-wrap {
    white-space: normal;
  }
  .sys-row {
    display: flex;
    gap: 12px;
    align-items: flex-start;
  }
  .sys-row .v-text-field {
    flex: 1 1 auto;
  }
  .sys-row .v-btn {
    margin-top: 2px;
  }
  .sys-setting {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 6px 0;
  }
  .drop-zone {
    position: relative;
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 18px 16px;
    border: 2px dashed rgba(var(--v-border-color), 0.35);
    border-radius: 10px;
    cursor: pointer;
    transition: background 0.15s, border-color 0.15s;
  }
  .drop-zone:hover,
  .drop-zone-over {
    border-color: rgb(var(--v-theme-primary));
    background: rgba(var(--v-theme-primary), 0.06);
  }
  .drop-zone-file {
    border-style: solid;
    border-color: rgba(var(--v-theme-primary), 0.6);
  }
  .drop-clear {
    position: absolute;
    top: 6px;
    right: 6px;
  }
  .prune-parts {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  }
  .prune-report {
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 8px;
    padding: 8px 12px;
  }
  @media (max-width: 600px) {
    .sys-row {
      flex-direction: column;
      align-items: stretch;
    }
  }
</style>
