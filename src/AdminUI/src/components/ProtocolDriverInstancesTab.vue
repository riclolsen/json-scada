<template>
  <div class="driver-instances-tab">
    <AdminSummary v-model="tileFilter" :tiles="summaryTiles" />

    <div class="admin-toolbar">
      <v-btn color="primary" variant="flat" prepend-icon="mdi-plus" @click="openAddProtocolDriverInstanceDialog">
        {{ $t('admin.protocolDriverInstances.addProtocolDriverInstance') }}
      </v-btn>
      <v-text-field
        v-model="search"
        class="admin-search"
        density="compact"
        variant="outlined"
        hide-details
        clearable
        prepend-inner-icon="mdi-magnify"
        :label="$t('admin.protocolDriverInstances.search')"
      ></v-text-field>
      <v-select
        v-model="filterDrivers"
        class="admin-search"
        :items="driversInUse"
        :label="$t('admin.protocolConnections.filterDriver')"
        multiple
        chips
        closable-chips
        clearable
        density="compact"
        variant="outlined"
        hide-details
      ></v-select>
      <v-select
        v-model="filterNode"
        class="node-select"
        :items="nodeNames"
        :label="$t('admin.protocolDriverInstances.filterNode')"
        clearable
        density="compact"
        variant="outlined"
        hide-details
      ></v-select>
      <v-spacer></v-spacer>
      <span class="admin-count">{{ $t('admin.common.results', { n: filteredInstances.length }) }}</span>
      <v-btn icon variant="text" size="small" :title="$t('common.refresh')" :loading="loading" @click="refreshAll">
        <v-icon>mdi-refresh</v-icon>
      </v-btn>
      <v-btn
        v-if="processMgmtEnabled"
        icon
        variant="text"
        size="small"
        :loading="syncing"
        :title="$t('admin.protocolDriverInstances.syncServices')"
        @click="syncDriverServices"
      >
        <v-icon>mdi-cog-sync-outline</v-icon>
      </v-btn>
      <v-btn icon variant="text" size="small" :title="$t('admin.common.exportCsv')" @click="exportCsv">
        <v-icon>mdi-file-delimited-outline</v-icon>
      </v-btn>
    </div>

    <v-data-table
      v-model:expanded="expanded"
      v-model:items-per-page="itemsPerPage"
      :headers="headers"
      :items="filteredInstances"
      :items-per-page-options="[10, 25, 50, 100, -1]"
      :items-per-page-text="$t('common.itemsPerPageText')"
      :sort-by="[{ key: 'protocolDriver', order: 'asc' }]"
      :loading="loading"
      item-value="_id"
      show-expand
      hover
      density="compact"
      class="admin-table"
    >
      <template #[`item.status`]="{ item }">
        <v-chip size="small" label variant="tonal" :color="aliveOf(item).color" class="inst-status">
          <v-icon start size="10">mdi-circle</v-icon>
          {{ aliveOf(item).label }}
        </v-chip>
      </template>
      <template #[`item.protocolDriver`]="{ item }">
        <span class="font-weight-medium">{{ item.protocolDriver }}</span>
        <span class="text-medium-emphasis"> · #{{ item.protocolDriverInstanceNumber }}</span>
        <div v-if="item.softwareVersion" class="text-body-small text-medium-emphasis">v{{ item.softwareVersion }}</div>
      </template>
      <template #[`item.activeNodeName`]="{ item }">
        <div v-if="item.activeNodeName" class="text-body-small">
          <v-icon size="14" class="me-1">mdi-server-outline</v-icon>{{ item.activeNodeName }}
        </div>
        <div class="text-body-small text-medium-emphasis" :title="keepAliveText(item)">
          {{ keepAliveAgeText(item) }}
        </div>
      </template>
      <template #[`item.nodesText`]="{ item }">
        <div class="d-flex flex-wrap ga-1">
          <v-chip
            v-for="n in item.nodeNames || []"
            :key="n"
            size="x-small"
            label
            :variant="n === item.activeNodeName ? 'tonal' : 'outlined'"
            :color="n === item.activeNodeName ? 'primary' : undefined"
          >{{ n }}</v-chip>
        </div>
      </template>
      <template #[`item.logLevel`]="{ item }">
        <span class="text-body-small">{{ logLevelLabel(item.logLevel) }}</span>
      </template>
      <template #[`item.connectionCount`]="{ item }">
        <v-chip
          size="x-small"
          label
          variant="tonal"
          :color="item.connectionCount ? 'primary' : undefined"
          :title="$t('admin.protocolDriverInstances.showConnections')"
          @click="showConnections(item)"
        >
          <v-icon start size="14">mdi-lan</v-icon>{{ item.connectionCount }}
        </v-chip>
      </template>
      <template #[`item.serviceState`]="{ item }">
        <v-tooltip
          v-if="!statusOf(item).manageable"
          :text="statusOf(item).reason || $t('admin.protocolDriverInstances.notManageable')"
        >
          <template #activator="{ props }">
            <v-chip v-bind="props" size="x-small" label variant="outlined" color="grey">
              {{ $t('admin.protocolDriverInstances.serviceStates.notManageable') }}
            </v-chip>
          </template>
        </v-tooltip>
        <template v-else>
          <v-chip size="x-small" label variant="tonal" :color="serviceStateColor(statusOf(item).state, statusOf(item).installed)">
            {{ serviceStateLabel(statusOf(item)) }}
          </v-chip>
          <v-chip v-if="statusOf(item).restartPending" size="x-small" label variant="tonal" color="orange" class="ms-1">
            {{ $t('admin.protocolDriverInstances.serviceStates.restartPending') }}
          </v-chip>
        </template>
      </template>
      <template #[`item.process`]="{ item }">
        <div class="d-flex align-center">
          <v-progress-circular v-if="busyKey === keyOf(item)" indeterminate size="18" width="2" class="mx-2" />
          <template v-else>
            <v-btn
              v-if="canStart(statusOf(item))"
              icon
              variant="text"
              size="x-small"
              color="success"
              :disabled="!statusOf(item).manageable"
              :title="$t('common.start')"
              @click="startProtocolDriverInstance(item)"
            >
              <v-icon size="18">mdi-play</v-icon>
            </v-btn>
            <v-btn
              v-if="canStop(statusOf(item))"
              icon
              variant="text"
              size="x-small"
              color="error"
              :disabled="!statusOf(item).manageable"
              :title="$t('common.stop')"
              @click="stopProtocolDriverInstance(item)"
            >
              <v-icon size="18">mdi-stop</v-icon>
            </v-btn>
            <v-btn
              icon
              variant="text"
              size="x-small"
              :disabled="!statusOf(item).manageable"
              :title="$t('admin.protocolDriverInstances.restart')"
              @click="restartProtocolDriverInstance(item)"
            >
              <v-icon size="18">mdi-restart</v-icon>
            </v-btn>
          </template>
        </div>
      </template>
      <template #[`item.enabled`]="{ item }">
        <v-switch
          :model-value="!!item.enabled"
          color="primary"
          density="compact"
          hide-details
          inset
          :loading="toggling === item._id"
          @update:model-value="(v) => setEnabled(item, v)"
        ></v-switch>
      </template>
      <template #[`item.actions`]="{ item }">
        <div class="admin-row-actions">
          <v-btn
            icon
            variant="text"
            size="x-small"
            :title="$t('admin.protocolDriverInstances.editInstance')"
            @click="openEditProtocolDriverInstanceDialog(item)"
          >
            <v-icon size="18">mdi-pencil-outline</v-icon>
          </v-btn>
          <v-menu location="bottom end">
            <template #activator="{ props }">
              <v-btn icon variant="text" size="x-small" v-bind="props"><v-icon size="18">mdi-dots-vertical</v-icon></v-btn>
            </template>
            <v-list density="compact">
              <v-list-item prepend-icon="mdi-lan" @click="showConnections(item)">
                <v-list-item-title>{{ $t('admin.protocolDriverInstances.showConnections') }}</v-list-item-title>
              </v-list-item>
              <v-divider></v-divider>
              <v-list-item prepend-icon="mdi-delete-outline" base-color="error" @click="openDeleteProtocolDriverInstanceDialog(item)">
                <v-list-item-title>{{ $t('common.delete') }}</v-list-item-title>
              </v-list-item>
            </v-list>
          </v-menu>
        </div>
      </template>
      <template #expanded-row="{ columns, item }">
        <tr>
          <td :colspan="columns.length" class="admin-details">
            <div class="admin-details-grid">
              <div>
                <div class="text-label-medium text-uppercase">{{ $t('admin.protocolDriverInstances.headers.stats') }}</div>
                <dl v-if="statsEntries(item).length" class="admin-kv">
                  <template v-for="[k, v] in statsEntries(item)" :key="k">
                    <dt>{{ k }}</dt>
                    <dd class="admin-mono">{{ v }}</dd>
                  </template>
                </dl>
                <div v-else class="text-body-small text-medium-emphasis">{{ $t('admin.protocolConnections.noStats') }}</div>
              </div>
              <div>
                <div class="text-label-medium text-uppercase">{{ $t('admin.protocolDriverInstances.details') }}</div>
                <dl class="admin-kv">
                  <dt>{{ $t('admin.protocolDriverInstances.headers.activeNodeKeepAliveTimeTag') }}</dt>
                  <dd>{{ keepAliveText(item) || '—' }}</dd>
                  <dt>{{ $t('admin.protocolDriverInstances.version') }}</dt>
                  <dd class="admin-mono">{{ item.softwareVersion || '—' }}</dd>
                  <dt>keepProtocolRunningWhileInactive</dt>
                  <dd>{{ item.keepProtocolRunningWhileInactive ? $t('common.true') : $t('common.false') }}</dd>
                  <template v-if="statusOf(item).manageable">
                    <dt>{{ $t('admin.protocolDriverInstances.serviceName') }}</dt>
                    <dd class="admin-mono">{{ statusOf(item).serviceName || '—' }}</dd>
                  </template>
                  <dt>{{ $t('admin.protocolDriverInstances.startMode') }}</dt>
                  <dd>{{ $t('admin.protocolDriverInstances.startModes.' + (item.processManagement?.startMode === 'manual' ? 'manual' : 'auto')) }}</dd>
                </dl>
              </div>
            </div>
          </td>
        </tr>
      </template>
    </v-data-table>

    <!-- add / edit instance -->
    <v-dialog v-model="dialogEditInstance" scrollable max-width="600px">
      <v-card>
        <v-card-title class="d-flex align-center ga-2">
          <v-icon>mdi-cpu-64-bit</v-icon>
          {{ editedInstance._id ? $t('admin.protocolDriverInstances.editInstance') : $t('admin.protocolDriverInstances.addNewProtocolDriverInstance') }}
        </v-card-title>
        <v-divider></v-divider>
        <v-card-text>
          <div class="admin-dialog-section">{{ $t('admin.rolesManagement.sectionGeneral') }}</div>
          <div class="inst-general">
            <v-autocomplete
              v-model="editedInstance.protocolDriver"
              :items="driverNameItems"
              variant="outlined"
              density="comfortable"
              prepend-inner-icon="mdi-cpu-64-bit"
              :label="$t('admin.protocolDriverInstances.headers.protocolDriver')"
              @update:model-value="onDriverNameChange"
            ></v-autocomplete>
            <v-text-field
              v-model="editedInstance.protocolDriverInstanceNumber"
              variant="outlined"
              density="comfortable"
              :label="$t('admin.protocolDriverInstances.headers.protocolDriverInstanceNumber')"
              type="number"
              disabled
            ></v-text-field>
          </div>
          <div class="inst-general">
            <v-select
              v-model="editedInstance.logLevel"
              :items="logLevelItems"
              item-title="text"
              item-value="value"
              :label="$t('admin.protocolDriverInstances.headers.logLevel')"
              prepend-inner-icon="mdi-text-box-outline"
              variant="outlined"
              density="comfortable"
            ></v-select>
            <v-switch
              v-model="editedInstance.enabled"
              inset
              color="primary"
              :label="$t('admin.protocolDriverInstances.headers.enabled')"
              class="mt-1"
            ></v-switch>
          </div>

          <div class="admin-dialog-section mt-2">{{ $t('admin.protocolDriverInstances.headers.allowedNodesList') }}</div>
          <v-combobox
            v-model="editedInstance.nodeNames"
            :items="nodeNames"
            :label="$t('admin.protocolDriverInstances.allowedNodesList')"
            :hint="$t('admin.protocolDriverInstances.nodesHint')"
            persistent-hint
            prepend-inner-icon="mdi-server-network"
            variant="outlined"
            density="comfortable"
            chips
            closable-chips
            multiple
          ></v-combobox>

          <div class="admin-dialog-section mt-4">{{ $t('admin.protocolDriverInstances.serviceManagement') }}</div>
          <v-switch
            v-model="editedInstance.processManagement.managed"
            inset
            color="primary"
            hide-details
            :label="$t('admin.protocolDriverInstances.managedService')"
          ></v-switch>
          <v-select
            v-model="editedInstance.processManagement.startMode"
            :items="[
              { text: $t('admin.protocolDriverInstances.startModes.auto'), value: 'auto' },
              { text: $t('admin.protocolDriverInstances.startModes.manual'), value: 'manual' },
            ]"
            item-title="text"
            item-value="value"
            variant="outlined"
            density="comfortable"
            class="mt-2"
            :disabled="!editedInstance.processManagement.managed"
            :label="$t('admin.protocolDriverInstances.startMode')"
          ></v-select>
          <v-switch
            v-model="editedInstance.processManagement.autoRestartOnConfigChange"
            inset
            color="primary"
            hide-details
            :disabled="!editedInstance.processManagement.managed"
            :label="$t('admin.protocolDriverInstances.autoRestartOnConfigChange')"
          ></v-switch>
          <v-alert v-if="dialogError" type="error" variant="tonal" density="compact" class="mt-4">{{ dialogError }}</v-alert>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="dialogEditInstance = false">{{ $t('common.cancel') }}</v-btn>
          <v-btn
            color="primary"
            variant="flat"
            :loading="saving"
            :disabled="!editedInstance.protocolDriver"
            @click="updateOrCreateProtocolDriverInstance"
          >
            {{ $t('common.save') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- delete instance -->
    <v-dialog v-model="dialogDeleteInstance" max-width="440px">
      <v-card>
        <v-card-title class="d-flex align-center ga-2">
          <v-icon color="error">mdi-delete-outline</v-icon>
          {{ $t('admin.protocolDriverInstances.deleteInstance') }}
        </v-card-title>
        <v-card-text>
          {{ $t('admin.protocolDriverInstances.deleteInstanceConfirm') }}
          <div class="mt-2 font-weight-medium">
            {{ editedInstance.protocolDriver + ' / ' + editedInstance.protocolDriverInstanceNumber }}
          </div>
          <div v-if="connectionsOf(editedInstance)" class="mt-2 text-body-small text-warning">
            {{ $t('admin.protocolDriverInstances.deleteHasConnections', { n: connectionsOf(editedInstance) }) }}
          </div>
          <div
            v-if="statusOf(editedInstance).manageable && statusOf(editedInstance).installed"
            class="mt-2 text-body-small text-orange"
          >
            {{
              $t('admin.protocolDriverInstances.deleteServiceNote', {
                service: statusOf(editedInstance).serviceName,
                node: statusOf(editedInstance).node,
              })
            }}
          </div>
          <v-alert v-if="dialogError" type="error" variant="tonal" density="compact" class="mt-4">{{ dialogError }}</v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="dialogDeleteInstance = false">{{ $t('common.cancel') }}</v-btn>
          <v-btn color="error" variant="flat" :loading="saving" @click="deleteDriverInstance(editedInstance)">
            {{ $t('common.delete') }}
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
  import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import AdminSummary from './AdminSummary.vue'
  import { postJson, fetchAllPages, toCsv, downloadText, fileStamp, loadPref, savePref } from '../lib/adminList'

  const props = defineProps({
    // the section is shown (lists are polled only then)
    active: { type: Boolean, default: false },
  })
  const emit = defineEmits(['show-connections'])
  const { t } = useI18n()

  const headers = computed(() => [
    { title: t('admin.protocolConnections.headers.status'), key: 'status', sortable: false, width: 120 },
    { title: t('admin.protocolDriverInstances.headers.protocolDriver'), key: 'protocolDriver' },
    { title: t('admin.protocolDriverInstances.headers.activeNode'), key: 'activeNodeName' },
    { title: t('admin.protocolDriverInstances.headers.allowedNodesList'), key: 'nodesText' },
    { title: t('admin.protocolDriverInstances.headers.logLevel'), key: 'logLevel' },
    { title: t('admin.protocolDriverInstances.connections'), key: 'connectionCount', align: 'end' },
    { title: t('admin.protocolDriverInstances.headers.serviceState'), key: 'serviceState', sortable: false },
    { title: t('admin.protocolDriverInstances.headers.process'), key: 'process', sortable: false },
    { title: t('admin.protocolDriverInstances.headers.enabled'), key: 'enabled', width: 90 },
    { title: '', key: 'actions', sortable: false, align: 'end', width: 90 },
    { title: '', key: 'data-table-expand', width: 40 },
  ])

  const logLevelItems = computed(() => [
    { text: t('common.logLevels.minimum'), value: 0 },
    { text: t('common.logLevels.basic'), value: 1 },
    { text: t('common.logLevels.detailed'), value: 2 },
    { text: t('common.logLevels.maximum'), value: 3 },
  ])
  const logLevelLabel = (v) => logLevelItems.value.find((x) => x.value === v)?.text ?? String(v ?? '')

  const dialogEditInstance = ref(false)
  const editedInstanceDefault = {
    protocolDriver: '',
    protocolDriverInstanceNumber: '',
    enabled: true,
    logLevel: 1,
    activeNodeKeepAliveTimeTag: new Date(0),
    keepProtocolRunningWhileInactive: false,
    softwareVersion: '',
    nodeNames: [],
    activeNodeName: '',
    stats: '',
    processManagement: {
      managed: true,
      startMode: 'auto',
      autoRestartOnConfigChange: true,
    },
    processExecutableVariant: 'plc4j',
  }
  const editedInstance = ref(JSON.parse(JSON.stringify(editedInstanceDefault)))

  const dialogDeleteInstance = ref(false)
  const nodeNames = ref([])
  const driverNameItems = [
    'IEC60870-5-104',
    'IEC60870-5-104_SERVER',
    'IEC60870-5-101',
    'IEC60870-5-101_SERVER',
    'IEC60870-5-103',
    'IEC61850',
    'IEC61850_SERVER',
    'DNP3',
    'DNP3_SERVER',
    'MQTT-SPARKPLUG-B',
    'MODBUS',
    'MODBUS_SERVER',
    'OPC-UA',
    'OPC-UA_SERVER',
    'OPC-DA',
    'OPC-DA_SERVER',
    'PLCTAG',
    'PLC4X',
    'TELEGRAF-LISTENER',
    'NODE-RED',
    'N8N',
    'ICCP',
    'ICCP_SERVER',
    'I104M',
    'PI_DATA_ARCHIVE_INJECTOR',
    'PI_DATA_ARCHIVE_CLIENT',
    'ONVIF',
    'CIM-EMS',
  ]
  const driverInstances = ref([])
  const loading = ref(false)
  const saving = ref(false)
  const dialogError = ref('')
  const expanded = ref([])
  const itemsPerPage = ref(loadPref('drivers-page-size', 25))
  watch(itemsPerPage, (n) => savePref('drivers-page-size', n))

  // ---- list filters (client side) ----
  const search = ref('')
  const filterDrivers = ref([])
  const filterNode = ref(null)
  const tileFilter = ref(null)
  const driversInUse = computed(() =>
    [...new Set(driverInstances.value.map((d) => d.protocolDriver).filter(Boolean))].sort()
  )
  const filteredInstances = computed(() => {
    const q = (search.value || '').trim().toLowerCase()
    return driverInstances.value.filter(
      (d) =>
        (!q ||
          String(d.protocolDriver || '').toLowerCase().includes(q) ||
          String(d.activeNodeName || '').toLowerCase().includes(q) ||
          (d.nodeNames || []).some((n) => String(n).toLowerCase().includes(q))) &&
        (!filterDrivers.value.length || filterDrivers.value.includes(d.protocolDriver)) &&
        (!filterNode.value || (d.nodeNames || []).includes(filterNode.value)) &&
        (tileFilter.value === null || aliveOf(d).key === tileFilter.value)
    )
  })

  // ---- liveness: _System.Driver.<driver>.<instance>.Alive tags of the data
  // processor (docs/schema.md); without them, a keep-alive younger than 30 s
  // (compared with this browser's clock) ----
  const driverComm = ref({})
  const now = ref(Date.now())
  async function fetchDriverComm() {
    try {
      const { items } = await fetchAllPages(
        '/Invoke/auth/listTags',
        { filter: { tag: { $regex: '^_System\\.Driver\\.' } } },
        'tags',
        20000
      )
      const map = {}
      for (const tg of items) {
        const m = /^_System\.Driver\.(.+)\.(\d+)\.([A-Za-z]+)$/.exec(tg.tag || '')
        if (!m) continue
        const k = m[1] + '#' + m[2]
        const e = (map[k] = map[k] || {})
        e[m[3]] = tg.invalid ? undefined : tg.type === 'string' ? tg.valueString : tg.value
      }
      driverComm.value = map
    } catch (err) {
      console.warn(err)
    }
  }
  const keepAliveAge = (item) => {
    const ts = item.activeNodeKeepAliveTimeTag ? Date.parse(item.activeNodeKeepAliveTimeTag) : NaN
    return Number.isFinite(ts) && ts > 0 ? Math.max(0, (now.value - ts) / 1000) : null
  }
  function aliveOf(item) {
    if (!item.enabled) return { key: 'disabled', color: 'grey', label: t('admin.protocolConnections.status.disabled') }
    const c = driverComm.value[keyOf(item)]
    let alive
    if (c && c.Alive !== undefined) alive = !!c.Alive
    else {
      const age = keepAliveAge(item)
      alive = age !== null && age < 30
    }
    return alive
      ? { key: 'alive', color: 'success', label: t('admin.protocolDriverInstances.alive') }
      : { key: 'stopped', color: 'error', label: t('admin.protocolDriverInstances.notAlive') }
  }
  function ageText(s) {
    if (s < 90) return t('admin.common.secondsAgo', { n: Math.round(s) })
    if (s < 5400) return t('admin.common.minutesAgo', { n: Math.round(s / 60) })
    if (s < 172800) return t('admin.common.hoursAgo', { n: Math.round(s / 3600) })
    return t('admin.common.daysAgo', { n: Math.round(s / 86400) })
  }
  function keepAliveAgeText(item) {
    const age = keepAliveAge(item)
    return age === null ? t('admin.protocolDriverInstances.noKeepAlive') : ageText(age)
  }
  function keepAliveText(item) {
    const ts = item.activeNodeKeepAliveTimeTag ? new Date(item.activeNodeKeepAliveTimeTag) : null
    return ts && ts.getTime() > 0 ? ts.toLocaleString() : ''
  }
  function statsEntries(item) {
    const st = item.stats
    if (!st || typeof st !== 'object') return []
    return Object.entries(st).map(([k, v]) => [k, v && typeof v === 'object' ? JSON.stringify(v) : String(v)])
  }

  // ---- connections of each instance ----
  const connCount = ref({})
  async function fetchConnectionCounts() {
    try {
      const json = await (await fetch('/Invoke/auth/listProtocolConnections')).json()
      if (!Array.isArray(json)) return
      const m = {}
      for (const c of json) {
        const k = c.protocolDriver + '#' + c.protocolDriverInstanceNumber
        m[k] = (m[k] || 0) + 1
      }
      connCount.value = m
    } catch (err) {
      console.warn(err)
    }
  }
  const connectionsOf = (item) => connCount.value[keyOf(item)] || 0
  function showConnections(item) {
    emit('show-connections', { driver: item.protocolDriver, instance: item.protocolDriverInstanceNumber })
  }

  const summaryTiles = computed(() => {
    const list = driverInstances.value
    const count = (k) => list.filter((d) => aliveOf(d).key === k).length
    return [
      { key: 'total', icon: 'mdi-cpu-64-bit', color: 'primary', value: list.length, label: t('admin.protocolDriverInstances.summary.total') },
      { key: 'alive', filter: 'alive', icon: 'mdi-heart-pulse', color: 'success', value: count('alive'), label: t('admin.protocolDriverInstances.alive') },
      { key: 'stopped', filter: 'stopped', icon: 'mdi-heart-off-outline', color: 'error', value: count('stopped'), label: t('admin.protocolDriverInstances.notAlive') },
      { key: 'disabled', filter: 'disabled', icon: 'mdi-minus-circle-outline', color: 'grey', value: count('disabled'), label: t('admin.protocolConnections.status.disabled') },
      { key: 'connections', icon: 'mdi-lan', color: 'primary', value: Object.values(connCount.value).reduce((a, n) => a + n, 0), label: t('admin.protocolDriverInstances.connections') },
    ]
  })

  // OS-service status (keyed by driver#instance), refreshed by polling.
  const statusMap = ref({})
  const processMgmtEnabled = ref(false)
  const syncing = ref(false)
  const busyKey = ref('')
  const toggling = ref(null)
  const snackbar = ref({ show: false, text: '', color: 'primary' })
  let pollTimer = null

  function keyOf(item) {
    return item.protocolDriver + '#' + item.protocolDriverInstanceNumber
  }
  function statusOf(item) {
    return statusMap.value[keyOf(item)] || { manageable: false, state: 'UNKNOWN' }
  }
  function serviceStateColor(state, installed) {
    if (!installed) return 'grey'
    switch (state) {
      case 'RUNNING':
        return 'green'
      case 'STOPPED':
        return 'blue-grey'
      case 'STARTING':
      case 'STOPPING':
        return 'amber'
      case 'FATAL':
      case 'BACKOFF':
        return 'red'
      default:
        return 'grey'
    }
  }
  function serviceStateLabel(st) {
    if (!st.installed) return t('admin.protocolDriverInstances.serviceStates.notInstalled')
    const key = 'admin.protocolDriverInstances.serviceStates.' + st.state
    const label = t(key)
    return label === key ? st.state : label
  }
  // Show start when the service is stopped/failed/not-installed; show stop whenever
  // it is active in any form, including PAUSED (a paused service can still be stopped).
  function canStart(st) {
    return !st.installed || ['STOPPED', 'FATAL', 'BACKOFF', 'UNKNOWN'].includes(st.state)
  }
  function canStop(st) {
    return ['RUNNING', 'PAUSED', 'STARTING', 'STOPPING'].includes(st.state)
  }
  function showSnack(text, color = 'primary') {
    snackbar.value = { show: true, text, color }
  }

  async function fetchDriverProcessStatus() {
    try {
      const res = await fetch('/Invoke/auth/listDriverProcessStatus')
      const json = await res.json()
      if (json.error) {
        console.warn(json)
        return
      }
      processMgmtEnabled.value = !!json.enabled
      const map = {}
      ;(json.statuses || []).forEach((s) => {
        map[s.protocolDriver + '#' + s.protocolDriverInstanceNumber] = s
      })
      statusMap.value = map
    } catch (err) {
      console.warn(err)
    }
  }

  // keep-alives, liveness and service states every 5 s while the section is shown
  watch(
    () => props.active,
    (active) => {
      clearInterval(pollTimer)
      pollTimer = null
      if (active)
        pollTimer = setInterval(() => {
          if (document.hidden) return
          now.value = Date.now()
          fetchDriverProcessStatus()
          if (!dialogEditInstance.value) {
            fetchProtocolDriverInstances(true)
            fetchDriverComm()
          }
        }, 5000)
    },
    { immediate: true }
  )

  onMounted(() => {
    refreshAll()
  })
  onUnmounted(() => clearInterval(pollTimer))

  async function refreshAll() {
    now.value = Date.now()
    await Promise.all([
      fetchProtocolDriverInstances(),
      fetchDriverProcessStatus(),
      fetchDriverComm(),
      fetchConnectionCounts(),
    ])
  }

  async function fetchNodes() {
    try {
      const res = await fetch('/Invoke/auth/listNodes')
      const json = await res.json()
      if (Array.isArray(json)) nodeNames.value = json
    } catch (err) {
      console.warn(err)
    }
  }

  async function fetchProtocolDriverInstances(quiet = false) {
    if (quiet !== true) loading.value = true
    try {
      await fetchNodes()
      const res = await fetch('/Invoke/auth/listProtocolDriverInstances')
      const json = await res.json()
      if (json.error || !Array.isArray(json)) {
        showSnack(t('common.error') + ' ' + (json.error || ''), 'error')
        return
      }
      json.forEach((item) => {
        item.nodesText = (item.nodeNames || []).join(', ')
        item.connectionCount = connCount.value[keyOf(item)] || 0
      })
      driverInstances.value = json
    } catch (err) {
      console.warn(err)
    } finally {
      loading.value = false
    }
  }
  watch(connCount, () => {
    for (const d of driverInstances.value) d.connectionCount = connCount.value[keyOf(d)] || 0
  })

  // Sends a start/stop/restart action for one instance and refreshes status.
  async function driverProcessAction(item, endpoint, successKey) {
    busyKey.value = keyOf(item)
    try {
      const json = await postJson('/Invoke/auth/' + endpoint, {
        protocolDriver: item.protocolDriver,
        protocolDriverInstanceNumber: item.protocolDriverInstanceNumber,
      })
      if (json.error) {
        showSnack(String(json.error), 'error')
      } else {
        showSnack(item.protocolDriver + ' / ' + item.protocolDriverInstanceNumber + ': ' + t(successKey), 'success')
      }
    } catch (err) {
      console.warn(err)
      showSnack(String(err), 'error')
    }
    // give the service manager a moment, then refresh state
    setTimeout(() => {
      fetchDriverProcessStatus()
      busyKey.value = ''
    }, 2500)
  }

  const startProtocolDriverInstance = (item) =>
    driverProcessAction(item, 'startProtocolDriverInstance', 'admin.protocolDriverInstances.actionResults.started')
  const stopProtocolDriverInstance = (item) =>
    driverProcessAction(item, 'stopProtocolDriverInstance', 'admin.protocolDriverInstances.actionResults.stopped')
  const restartProtocolDriverInstance = (item) =>
    driverProcessAction(item, 'restartProtocolDriverInstance', 'admin.protocolDriverInstances.actionResults.restarted')

  async function syncDriverServices() {
    syncing.value = true
    try {
      const json = await postJson('/Invoke/auth/syncDriverServices', { removeOrphans: false })
      if (json.error) {
        showSnack(String(json.error), 'error')
      } else {
        const r = json.result || {}
        const ensured = (r.ensured || []).length
        const errors = (r.errors || []).length
        const orphans = (r.orphans || []).length
        showSnack(t('admin.protocolDriverInstances.syncResult', { ensured, errors, orphans }), errors ? 'warning' : 'success')
      }
    } catch (err) {
      console.warn(err)
      showSnack(String(err), 'error')
    }
    syncing.value = false
    fetchDriverProcessStatus()
  }

  // make sure the protocol driver instance number is unique for the protocol driver
  function onDriverNameChange(name) {
    const used = driverInstances.value
      .filter((i) => i.protocolDriver === name && i._id !== editedInstance.value._id)
      .map((i) => Number(i.protocolDriverInstanceNumber))
    editedInstance.value.protocolDriverInstanceNumber = used.length ? Math.max(...used) + 1 : 1
  }

  // instance document for the update route: no list-only fields
  function instanceBody(inst) {
    const dup = { ...inst }
    for (const k of ['id', 'nodesText', 'localTimeUpdate', 'process', 'connectionCount']) delete dup[k]
    return dup
  }

  async function setEnabled(item, enabled) {
    toggling.value = item._id
    try {
      const json = await postJson('/Invoke/auth/updateProtocolDriverInstance', instanceBody({ ...item, enabled }))
      if (json.error) showSnack(t('common.error') + ' ' + json.error, 'error')
    } catch (err) {
      showSnack(t('common.error') + ' ' + err.message, 'error')
    } finally {
      toggling.value = null
    }
    fetchProtocolDriverInstances(true)
  }

  async function deleteDriverInstance(item) {
    saving.value = true
    dialogError.value = ''
    try {
      const json = await postJson('/Invoke/auth/deleteProtocolDriverInstance', {
        protocolDriver: item.protocolDriver,
        protocolDriverInstanceNumber: item.protocolDriverInstanceNumber,
        _id: item._id,
      })
      if (json.error) throw new Error(json.error)
      dialogDeleteInstance.value = false
    } catch (err) {
      dialogError.value = err.message
    } finally {
      saving.value = false
    }
    refreshAll()
  }

  async function updateProtocolDriverInstance() {
    const json = await postJson('/Invoke/auth/updateProtocolDriverInstance', instanceBody(editedInstance.value))
    if (json.error) throw new Error(json.error)
  }

  async function updateOrCreateProtocolDriverInstance() {
    if (!editedInstance.value.protocolDriver) return
    saving.value = true
    dialogError.value = ''
    try {
      if (!editedInstance.value._id) {
        const json = await postJson('/Invoke/auth/createProtocolDriverInstance', {})
        if (json.error || !json._id) throw new Error(json.error || t('common.error'))
        editedInstance.value._id = json._id
      }
      await updateProtocolDriverInstance()
      dialogEditInstance.value = false
      showSnack(t('admin.common.saved'), 'success')
    } catch (err) {
      dialogError.value = err.message
    } finally {
      saving.value = false
    }
    refreshAll()
  }

  // Ensures the nested processManagement object exists with sane defaults so the
  // dialog switches bind correctly (older instance docs may not have it).
  function normalizeProcessManagement(inst) {
    const pm = inst.processManagement || {}
    inst.processManagement = {
      managed: pm.managed !== false,
      startMode: pm.startMode === 'manual' ? 'manual' : 'auto',
      autoRestartOnConfigChange: pm.autoRestartOnConfigChange !== false,
    }
    // plc4j is the only PLC4X implementation offered; older docs may carry
    // 'plc4x' or nothing at all
    inst.processExecutableVariant = 'plc4j'
    inst.nodeNames = [...(inst.nodeNames || [])]
    return inst
  }

  const openAddProtocolDriverInstanceDialog = async () => {
    dialogError.value = ''
    await fetchNodes()
    editedInstance.value = normalizeProcessManagement(JSON.parse(JSON.stringify(editedInstanceDefault)))
    dialogEditInstance.value = true
  }

  const openEditProtocolDriverInstanceDialog = async (item) => {
    dialogError.value = ''
    await fetchNodes()
    editedInstance.value = normalizeProcessManagement(JSON.parse(JSON.stringify(item)))
    dialogEditInstance.value = true
  }

  const openDeleteProtocolDriverInstanceDialog = (item) => {
    dialogError.value = ''
    editedInstance.value = { ...item }
    dialogDeleteInstance.value = true
  }

  function exportCsv() {
    const cols = [
      { title: 'protocolDriver', value: (d) => d.protocolDriver },
      { title: 'protocolDriverInstanceNumber', value: (d) => d.protocolDriverInstanceNumber },
      { title: 'enabled', value: (d) => !!d.enabled },
      { title: 'status', value: (d) => aliveOf(d).key },
      { title: 'activeNodeName', value: (d) => d.activeNodeName },
      { title: 'activeNodeKeepAliveTimeTag', value: (d) => d.activeNodeKeepAliveTimeTag },
      { title: 'nodeNames', value: (d) => (d.nodeNames || []).join(' ') },
      { title: 'logLevel', value: (d) => d.logLevel },
      { title: 'softwareVersion', value: (d) => d.softwareVersion },
      { title: 'connections', value: (d) => connectionsOf(d) },
      { title: 'serviceState', value: (d) => (statusOf(d).manageable ? statusOf(d).state : '') },
    ]
    downloadText('driver_instances_' + fileStamp() + '.csv', toCsv(filteredInstances.value, cols))
  }

  defineExpose({ fetchProtocolDriverInstances: refreshAll })
</script>

<style scoped>
  .inst-status {
    min-width: 92px;
  }
  .node-select {
    flex: 0 1 180px;
    min-width: 140px;
  }
  .inst-general {
    display: grid;
    grid-template-columns: 2fr 1fr;
    gap: 12px;
  }
  @media (max-width: 600px) {
    .inst-general {
      grid-template-columns: 1fr;
    }
  }
</style>
