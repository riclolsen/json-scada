<template>
  <v-container fluid class="pa-2">
    <v-alert
      v-if="!canEngineer"
      type="info"
      variant="tonal"
      density="compact"
      class="mb-2"
    >
      {{ t('admin.cimEms.readOnly') }}
    </v-alert>

    <v-tabs v-model="sub" density="compact" color="primary">
      <v-tab value="models"
        ><v-icon start>mdi-transmission-tower</v-icon
        >{{ t('admin.cimEms.models') }}</v-tab
      >
      <v-tab value="bindings"
        ><v-icon start>mdi-link-variant</v-icon
        >{{ t('admin.cimEms.bindings') }}</v-tab
      >
      <v-tab value="runs" @click="fetchRuns"
        ><v-icon start>mdi-history</v-icon>{{ t('admin.cimEms.runs') }}</v-tab
      >
      <v-tab value="requests" @click="fetchRequests"
        ><v-icon start>mdi-tray-full</v-icon
        >{{ t('admin.cimEms.requests') }}</v-tab
      >
    </v-tabs>

    <v-window v-model="sub" class="mt-2">
      <!-- MODELS -->
      <v-window-item value="models">
        <v-toolbar flat density="compact">
          <v-btn color="primary" variant="tonal" @click="fetchModels">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
          <v-btn
            v-if="canEngineer"
            color="primary"
            variant="tonal"
            class="ml-2"
            @click="uploadDialog = true"
          >
            <v-icon start>mdi-upload</v-icon>{{ t('admin.cimEms.upload') }}
          </v-btn>
        </v-toolbar>
        <v-data-table
          :headers="modelHeaders"
          :items="models"
          item-value="_id"
          show-expand
          v-model:expanded="modelsExpanded"
          density="compact"
          class="elevation-1"
          :items-per-page-text="t('common.itemsPerPageText')"
        >
          <template #[`item.status`]="{ item }">
            <v-chip size="small" :color="statusColor(item.status)">{{
              item.status
            }}</v-chip>
          </template>
          <template #[`item.importedAt`]="{ item }">{{
            fmtTime(item.importedAt)
          }}</template>
          <template #[`item.validation`]="{ item }">
            <span class="text-caption"
              >{{ item.validation?.errors?.length || 0 }} /
              {{ item.validation?.warnings?.length || 0 }} /
              {{ item.validation?.unsupported?.length || 0 }}</span
            >
          </template>
          <template #[`item.actions`]="{ item }">
            <template v-if="canEngineer">
              <v-btn
                v-if="['validated', 'retired'].includes(item.status)"
                size="small"
                variant="text"
                color="green"
                :title="t('admin.cimEms.activate')"
                @click="confirmAction('activate', item)"
                ><v-icon>mdi-play-circle</v-icon></v-btn
              >
              <v-btn
                v-if="item.status === 'active'"
                size="small"
                variant="text"
                color="orange"
                :title="t('admin.cimEms.rollback')"
                @click="confirmAction('rollback', item)"
                ><v-icon>mdi-undo</v-icon></v-btn
              >
              <v-btn
                size="small"
                variant="text"
                :title="t('admin.cimEms.download')"
                @click="download(item.sourceArtifactId)"
                ><v-icon>mdi-download</v-icon></v-btn
              >
            </template>
            <v-btn
              size="small"
              variant="text"
              :title="t('admin.cimEms.bindings')"
              @click="openBindings(item)"
              ><v-icon>mdi-link-variant</v-icon></v-btn
            >
          </template>
          <template #expanded-row="{ columns, item }">
            <tr>
              <td :colspan="columns.length" class="py-2">
                <div
                  v-for="k in ['errors', 'warnings', 'unsupported']"
                  :key="k"
                >
                  <div
                    v-if="item.validation?.[k]?.length"
                    class="text-subtitle-2"
                  >
                    {{ t('admin.cimEms.' + k) }}
                  </div>
                  <ul class="ml-6 text-caption">
                    <li v-for="(m, i) in item.validation?.[k] || []" :key="i">
                      {{ m }}
                    </li>
                  </ul>
                </div>
                <div class="text-caption mt-1">
                  SHA-256: {{ item.sourceSha256 }} · pypowsybl
                  {{ item.engine?.pypowsybl }} ·
                  {{ t('admin.cimEms.loadFlow') }}:
                  {{ item.validation?.load_flow?.status || '-' }}
                </div>
                <div class="text-caption">
                  {{
                    Object.entries(item.counts || {})
                      .filter(([, v]) => v)
                      .map(([k, v]) => k + ' ' + v)
                      .join(' · ')
                  }}
                </div>
              </td>
            </tr>
          </template>
        </v-data-table>
      </v-window-item>

      <!-- BINDINGS -->
      <v-window-item value="bindings">
        <v-toolbar flat density="compact" class="flex-wrap">
          <v-select
            v-model="bModelId"
            :items="modelIds"
            :label="t('admin.cimEms.modelId')"
            density="compact"
            hide-details
            style="max-width: 220px"
            class="mr-2"
            @update:model-value="fetchBindings(null)"
          />
          <v-select
            v-model="bRevision"
            :items="revisionItems"
            :label="t('admin.cimEms.bindingRevision')"
            density="compact"
            hide-details
            style="max-width: 260px"
            class="mr-2"
            @update:model-value="fetchBindings(bRevision)"
          />
          <v-btn
            v-if="canEngineer"
            :disabled="!bModelId || busy"
            color="primary"
            variant="tonal"
            class="mr-2"
            @click="propose"
          >
            <v-icon start>mdi-auto-fix</v-icon>{{ t('admin.cimEms.propose') }}
          </v-btn>
          <v-btn
            v-if="canEngineer"
            :disabled="!bindings.length || busy"
            color="green"
            variant="tonal"
            @click="saveBindings"
          >
            <v-icon start>mdi-content-save</v-icon
            >{{ t('admin.cimEms.saveRevision') }}
          </v-btn>
          <v-chip v-if="dirty" color="orange" size="small" class="ml-2">{{
            t('admin.cimEms.unsaved')
          }}</v-chip>
          <v-progress-circular
            v-if="busy"
            indeterminate
            size="24"
            class="ml-3"
          />
        </v-toolbar>

        <v-alert
          v-if="proposalInfo"
          type="info"
          variant="tonal"
          density="compact"
          class="my-2"
        >
          {{
            t('admin.cimEms.proposalSummary', {
              tags: proposalInfo.tags,
              sw: proposalInfo.switches.proposed,
              swTotal: proposalInfo.switches.total,
              an: proposalInfo.analogs.proposed,
              unmatched:
                proposalInfo.switches.unmatched +
                proposalInfo.analogs.unmatched,
              ambiguous:
                proposalInfo.switches.ambiguous +
                proposalInfo.analogs.ambiguous,
            })
          }}
          <v-btn
            size="x-small"
            variant="text"
            @click="showLists = !showLists"
            >{{ t('admin.cimEms.details') }}</v-btn
          >
          <div v-if="showLists" class="text-caption mt-1">
            <b>{{ t('admin.cimEms.ambiguous') }}:</b>
            {{ proposalInfo.ambiguous.join(', ') || '-' }}<br />
            <b>{{ t('admin.cimEms.unmatched') }}:</b>
            {{ proposalInfo.unmatched.join(', ') || '-' }}
          </div>
        </v-alert>

        <div class="d-flex flex-wrap align-center my-2 ga-2">
          <v-chip
            v-for="c in coverage"
            :key="c.kind"
            size="small"
            variant="outlined"
            >{{ c.kind }}: {{ c.approved }} / {{ c.count }}</v-chip
          >
          <v-spacer />
          <v-text-field
            v-model="bSearch"
            density="compact"
            hide-details
            prepend-inner-icon="mdi-magnify"
            :label="t('admin.cimEms.search')"
            style="max-width: 260px"
          />
          <v-select
            v-model="bKind"
            :items="['', 'switch', 'tap', 'analog']"
            density="compact"
            hide-details
            :label="t('admin.cimEms.kind')"
            style="max-width: 140px"
          />
          <template v-if="canEngineer">
            <v-btn size="small" variant="tonal" @click="setApproved(true)">{{
              t('admin.cimEms.approveShown')
            }}</v-btn>
            <v-btn size="small" variant="tonal" @click="setApproved(false)">{{
              t('admin.cimEms.rejectShown')
            }}</v-btn>
          </template>
        </div>

        <v-data-table
          :headers="bindingHeaders"
          :items="filteredBindings"
          density="compact"
          class="elevation-1"
          :items-per-page="25"
          :items-per-page-text="t('common.itemsPerPageText')"
        >
          <template #[`item.approved`]="{ item }">
            <v-checkbox-btn
              v-model="item.approved"
              :disabled="!canEngineer"
              density="compact"
              @update:model-value="dirty = true"
            />
          </template>
          <template #[`item.tag`]="{ item }">
            <v-text-field
              v-if="canEngineer"
              v-model="item.tag"
              density="compact"
              variant="plain"
              hide-details
              @update:model-value="dirty = true"
            />
            <span v-else>{{ item.tag }}</span>
          </template>
          <template #[`item.sigma`]="{ item }">
            <v-text-field
              v-if="canEngineer && item.kind === 'analog'"
              v-model.number="item.sigma"
              type="number"
              density="compact"
              variant="plain"
              hide-details
              style="width: 90px"
              @update:model-value="dirty = true"
            />
            <span v-else-if="item.kind === 'analog'">{{ item.sigma }}</span>
          </template>
          <template #[`item.confidence`]="{ item }">{{
            Number(item.confidence || 0).toFixed(2)
          }}</template>
          <template #[`item.remove`]="{ item }">
            <v-btn
              v-if="canEngineer"
              size="x-small"
              variant="text"
              color="red"
              @click="removeBinding(item)"
              ><v-icon>mdi-delete</v-icon></v-btn
            >
          </template>
        </v-data-table>
      </v-window-item>

      <!-- RUNS -->
      <v-window-item value="runs">
        <v-toolbar flat density="compact">
          <v-btn color="primary" variant="tonal" @click="fetchRuns">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
          <v-text-field
            v-model="runConn"
            type="number"
            density="compact"
            hide-details
            :label="t('admin.cimEms.connectionNumber')"
            style="max-width: 200px"
            class="ml-2"
            @keyup.enter="fetchRuns"
          />
          <v-btn
            :disabled="!runConn || busy"
            color="primary"
            variant="tonal"
            class="ml-2"
            @click="rerun"
          >
            <v-icon start>mdi-play</v-icon>{{ t('admin.cimEms.rerun') }}
          </v-btn>
        </v-toolbar>
        <v-data-table
          :headers="runHeaders"
          :items="runs"
          item-value="_id"
          show-expand
          v-model:expanded="runsExpanded"
          density="compact"
          class="elevation-1"
          :items-per-page="25"
          :items-per-page-text="t('common.itemsPerPageText')"
        >
          <template #[`item.createdAt`]="{ item }">{{
            fmtTime(item.createdAt)
          }}</template>
          <template #[`item.status`]="{ item }">
            <v-chip size="small" :color="statusColor(item.status)">{{
              item.status
            }}</v-chip>
          </template>
          <template #[`item.se`]="{ item }">
            <span v-if="item.summary?.estimation">
              <v-chip
                size="small"
                :color="statusColor(item.summary.estimation.status)"
                >{{ item.summary.estimation.status }}</v-chip
              >
              <span
                v-if="item.summary.estimation.badData?.length"
                class="text-caption ml-1"
                >{{ item.summary.estimation.badData.length }}
                {{ t('admin.cimEms.badData') }}</span
              >
            </span>
          </template>
          <template #[`item.ca`]="{ item }">
            <span v-if="item.summary?.contingency">
              <v-chip
                size="small"
                :color="statusColor(item.summary.contingency.status)"
                >{{ item.summary.contingency.status }}</v-chip
              >
              <span
                v-if="item.summary.contingency.status === 'done'"
                class="text-caption ml-1"
                >{{ item.summary.contingency.insecure?.length || 0 }} /
                {{ item.summary.contingency.contingencies }}</span
              >
            </span>
          </template>
          <template #expanded-row="{ columns, item }">
            <tr>
              <td :colspan="columns.length" class="py-2">
                <div v-if="item.summary?.estimation?.badData?.length">
                  <div class="text-subtitle-2">
                    {{ t('admin.cimEms.badData') }}
                  </div>
                  <ul class="ml-6 text-caption">
                    <li
                      v-for="(b, i) in item.summary.estimation.badData"
                      :key="i"
                    >
                      {{ b.element }} {{ b.quantity }}
                      {{ b.side ? '(' + b.side + ')' : '' }}: r<sub>N</sub> =
                      {{ b.normalized }}
                    </li>
                  </ul>
                </div>
                <div v-if="item.summary?.contingency?.insecure?.length">
                  <div class="text-subtitle-2">
                    {{ t('admin.cimEms.insecure') }}
                  </div>
                  <div class="text-caption ml-6">
                    {{ item.summary.contingency.insecure.join(', ') }}
                  </div>
                </div>
                <v-btn
                  v-if="canEngineer && item.summary?.contingency?.artifactId"
                  size="small"
                  variant="tonal"
                  class="my-1"
                  @click="download(item.summary.contingency.artifactId)"
                  ><v-icon start>mdi-download</v-icon
                  >{{ t('admin.cimEms.caResult') }}</v-btn
                >
                <pre class="text-caption">{{
                  JSON.stringify(
                    {
                      runId: item.runId,
                      model: item.modelId + ' rev ' + item.modelRevision,
                      bindingRevision: item.bindingRevision,
                      topologyHash: item.topologyHash,
                      snapshotTime: item.snapshotTime,
                      nodeName: item.nodeName,
                      published: item.published,
                      summary: item.summary,
                    },
                    null,
                    2
                  )
                }}</pre>
              </td>
            </tr>
          </template>
        </v-data-table>
      </v-window-item>

      <!-- REQUESTS -->
      <v-window-item value="requests">
        <v-toolbar flat density="compact">
          <v-btn color="primary" variant="tonal" @click="fetchRequests">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
        </v-toolbar>
        <v-data-table
          :headers="requestHeaders"
          :items="requests"
          item-value="_id"
          show-expand
          v-model:expanded="requestsExpanded"
          density="compact"
          class="elevation-1"
          :items-per-page="25"
          :items-per-page-text="t('common.itemsPerPageText')"
        >
          <template #[`item.createdAt`]="{ item }">{{
            fmtTime(item.createdAt)
          }}</template>
          <template #[`item.finishedAt`]="{ item }">{{
            fmtTime(item.finishedAt)
          }}</template>
          <template #[`item.status`]="{ item }">
            <v-chip size="small" :color="statusColor(item.status)">{{
              item.status
            }}</v-chip>
          </template>
          <template #expanded-row="{ columns, item }">
            <tr>
              <td :colspan="columns.length">
                <pre class="text-caption">{{
                  JSON.stringify(
                    {
                      params: item.params,
                      result: item.result,
                      error: item.error,
                    },
                    null,
                    2
                  )
                }}</pre>
              </td>
            </tr>
          </template>
        </v-data-table>
      </v-window-item>
    </v-window>

    <!-- upload dialog -->
    <v-dialog v-model="uploadDialog" max-width="520">
      <v-card>
        <v-card-title>{{ t('admin.cimEms.upload') }}</v-card-title>
        <v-card-text>
          <v-combobox
            v-model="upModelId"
            :items="modelIds"
            :label="t('admin.cimEms.modelId')"
            :rules="[
              (v) =>
                modelIdRe.test(v || '') || t('admin.cimEms.invalidModelId'),
            ]"
          />
          <v-file-input
            v-model="upFile"
            accept=".zip"
            :label="t('admin.cimEms.packageFile')"
            prepend-icon="mdi-folder-zip"
          />
          <div class="text-caption">{{ t('admin.cimEms.uploadHint') }}</div>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn @click="uploadDialog = false">{{ t('common.cancel') }}</v-btn>
          <v-btn
            color="primary"
            :disabled="!upFile || !modelIdRe.test(upModelId || '') || busy"
            @click="upload"
            >{{ t('admin.cimEms.upload') }}</v-btn
          >
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- confirmation -->
    <v-dialog v-model="confirmDialog" max-width="480">
      <v-card>
        <v-card-title>{{ t('common.confirm') }}</v-card-title>
        <v-card-text>{{ confirmText }}</v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn @click="confirmDialog = false">{{ t('common.cancel') }}</v-btn>
          <v-btn color="primary" @click="runConfirmed">{{
            t('common.confirm')
          }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-snackbar v-model="snack" :color="snackColor" timeout="6000">{{
      snackText
    }}</v-snackbar>
  </v-container>
</template>

<script setup>
  import { ref, computed, onMounted } from 'vue'
  import { useI18n } from 'vue-i18n'

  const { t } = useI18n()
  const API = '/Invoke/auth/cimEms/'
  const modelIdRe = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/
  const POLL_MS = 2000
  const POLL_LIMIT_MS = 30 * 60 * 1000

  const sub = ref('models')
  const canEngineer = ref(false)
  const busy = ref(false)
  const models = ref([])
  const modelsExpanded = ref([])
  const runs = ref([])
  const runsExpanded = ref([])
  const runConn = ref(null)
  const requests = ref([])
  const requestsExpanded = ref([])
  const uploadDialog = ref(false)
  const upModelId = ref('')
  const upFile = ref(null)
  const confirmDialog = ref(false)
  const confirmText = ref('')
  let confirmed = null
  const snack = ref(false)
  const snackText = ref('')
  const snackColor = ref('info')

  const bModelId = ref(null)
  const bRevision = ref(null)
  const revisions = ref([])
  const bindings = ref([])
  const dirty = ref(false)
  const proposalInfo = ref(null)
  const showLists = ref(false)
  const bSearch = ref('')
  const bKind = ref('')

  const modelHeaders = computed(() => [
    { title: t('admin.cimEms.modelId'), key: 'modelId' },
    { title: t('admin.cimEms.revision'), key: 'revision', align: 'end' },
    { title: t('admin.cimEms.status'), key: 'status' },
    { title: 'CGMES', key: 'cgmesVersion' },
    { title: t('admin.cimEms.importedBy'), key: 'importedBy' },
    { title: t('admin.cimEms.importedAt'), key: 'importedAt' },
    {
      title: t('admin.cimEms.validation'),
      key: 'validation',
      sortable: false,
    },
    { title: '', key: 'actions', sortable: false, align: 'end' },
  ])
  const bindingHeaders = computed(() => [
    { title: t('admin.cimEms.approved'), key: 'approved', width: 60 },
    { title: t('admin.cimEms.kind'), key: 'kind' },
    { title: t('admin.cimEms.element'), key: 'element_id' },
    { title: 'Q', key: 'quantity' },
    { title: t('admin.cimEms.side'), key: 'side', align: 'end' },
    { title: 'Tag', key: 'tag', minWidth: 220 },
    { title: 'σ', key: 'sigma' },
    { title: t('admin.cimEms.confidence'), key: 'confidence', align: 'end' },
    { title: t('admin.cimEms.source'), key: 'proposed_from' },
    { title: '', key: 'remove', sortable: false },
  ])
  const runHeaders = computed(() => [
    { title: t('admin.cimEms.time'), key: 'createdAt' },
    { title: '#', key: 'connectionNumber', align: 'end' },
    { title: t('admin.cimEms.trigger'), key: 'trigger' },
    { title: t('admin.cimEms.status'), key: 'status' },
    { title: 'ms', key: 'durationMs', align: 'end' },
    { title: t('admin.cimEms.revision'), key: 'modelRevision', align: 'end' },
    { title: 'SE', key: 'se', sortable: false },
    { title: 'CA', key: 'ca', sortable: false },
  ])
  const requestHeaders = computed(() => [
    { title: t('admin.cimEms.time'), key: 'createdAt' },
    { title: t('admin.cimEms.type'), key: 'type' },
    { title: t('admin.cimEms.user'), key: 'createdBy' },
    { title: t('admin.cimEms.status'), key: 'status' },
    { title: t('admin.cimEms.node'), key: 'nodeName' },
    { title: t('admin.cimEms.finished'), key: 'finishedAt' },
    { title: t('admin.cimEms.error'), key: 'error' },
  ])

  const modelIds = computed(() => [
    ...new Set(models.value.map((m) => m.modelId)),
  ])
  const revisionItems = computed(() =>
    revisions.value.map((r) => ({
      value: r.bindingRevision,
      title:
        r.bindingRevision +
        ' — ' +
        r.approved +
        '/' +
        r.count +
        ' · ' +
        (r.createdBy || '') +
        ' ' +
        fmtTime(r.createdAt),
    }))
  )
  const filteredBindings = computed(() => {
    const s = bSearch.value.trim().toLowerCase()
    return bindings.value.filter(
      (b) =>
        (!bKind.value || b.kind === bKind.value) &&
        (!s ||
          String(b.tag).toLowerCase().includes(s) ||
          String(b.element_id).toLowerCase().includes(s))
    )
  })
  const coverage = computed(() => {
    const c = {}
    for (const b of bindings.value) {
      c[b.kind] = c[b.kind] || { kind: b.kind, count: 0, approved: 0 }
      c[b.kind].count++
      if (b.approved) c[b.kind].approved++
    }
    return Object.values(c)
  })

  const fmtTime = (v) => (v ? new Date(v).toLocaleString() : '')
  const statusColor = (s) =>
    ({
      active: 'green',
      validated: 'blue',
      converged: 'green',
      done: 'green',
      retired: 'grey',
      skipped: 'grey',
      pending: 'grey',
      running: 'blue',
      rejected: 'red',
      failed: 'red',
      diverged: 'red',
      unsupported: 'orange',
      'not-observable': 'orange',
      'rejected-snapshot': 'orange',
      'no-load-flow': 'orange',
    })[s] || 'default'

  const notify = (text, color = 'info') => {
    snackText.value = text
    snackColor.value = color
    snack.value = true
  }

  const getJson = async (url) => {
    const res = await fetch(url)
    const json = await res.json()
    if (json.error) throw new Error(json.error)
    return json
  }

  const fetchModels = async () => {
    try {
      const json = await getJson(API + 'listModels')
      models.value = json.models || []
      canEngineer.value = !!json.canEngineer
      if (!bModelId.value && modelIds.value.length)
        bModelId.value = modelIds.value[0]
    } catch (err) {
      notify(String(err.message || err), 'error')
    }
  }

  const fetchRuns = async () => {
    try {
      const q = runConn.value ? '?connectionNumber=' + runConn.value : ''
      runs.value = (await getJson(API + 'listRuns' + q)).runs || []
    } catch (err) {
      notify(String(err.message || err), 'error')
    }
  }

  const fetchRequests = async () => {
    try {
      requests.value = (await getJson(API + 'listRequests')).requests || []
    } catch (err) {
      notify(String(err.message || err), 'error')
    }
  }

  const fetchBindings = async (revision) => {
    if (!bModelId.value) return
    try {
      let url =
        API + 'listBindings?modelId=' + encodeURIComponent(bModelId.value)
      if (revision) url += '&revision=' + revision
      const json = await getJson(url)
      revisions.value = json.revisions || []
      bRevision.value = json.bindingRevision
      bindings.value = json.bindings || []
      proposalInfo.value = null
      dirty.value = false
    } catch (err) {
      notify(String(err.message || err), 'error')
    }
  }

  // Posts a study request and waits for the driver to finish it.
  const request = async (body) => {
    const res = await fetch(API + 'request', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    const json = await res.json()
    if (json.error) throw new Error(json.error)
    return waitFor(json.requestId)
  }

  const waitFor = async (id) => {
    const t0 = Date.now()
    while (Date.now() - t0 < POLL_LIMIT_MS) {
      await new Promise((r) => setTimeout(r, POLL_MS))
      const list = (await getJson(API + 'listRequests?limit=100')).requests
      const r = list.find((x) => x._id === id)
      if (r && (r.status === 'done' || r.status === 'failed')) {
        if (r.status === 'failed') throw new Error(r.error)
        return r
      }
    }
    throw new Error(t('admin.cimEms.timeout'))
  }

  const withBusy = async (fn) => {
    busy.value = true
    try {
      await fn()
    } catch (err) {
      notify(String(err.message || err), 'error')
    } finally {
      busy.value = false
    }
  }

  const upload = () =>
    withBusy(async () => {
      const fd = new FormData()
      const f = Array.isArray(upFile.value) ? upFile.value[0] : upFile.value
      fd.append('modelId', upModelId.value)
      fd.append('packageFile', f)
      const res = await fetch(API + 'uploadModel', { method: 'POST', body: fd })
      const json = await res.json()
      if (json.error) throw new Error(json.error)
      uploadDialog.value = false
      notify(t('admin.cimEms.importQueued'))
      const r = await waitFor(json.requestId)
      notify(
        t('admin.cimEms.imported', {
          revision: r.result.revision,
          status: r.result.status,
        }),
        r.result.status === 'rejected' ? 'warning' : 'success'
      )
      await fetchModels()
    })

  const confirmAction = (type, item) => {
    confirmText.value = t('admin.cimEms.confirm_' + type, {
      model: item.modelId,
      revision: item.revision,
    })
    confirmed = () =>
      withBusy(async () => {
        const r = await request({
          type,
          modelId: item.modelId,
          revision: item.revision,
        })
        notify(
          t('admin.cimEms.activeNow', { revision: r.result.activeRevision }),
          'success'
        )
        await fetchModels()
      })
    confirmDialog.value = true
  }

  const runConfirmed = () => {
    confirmDialog.value = false
    if (confirmed) confirmed()
  }

  const openBindings = (item) => {
    bModelId.value = item.modelId
    sub.value = 'bindings'
    fetchBindings(null)
  }

  const propose = () =>
    withBusy(async () => {
      const r = await request({
        type: 'proposeBindings',
        modelId: bModelId.value,
      })
      const res = await fetch(API + 'artifact?id=' + r.result.artifactId)
      const list = await res.json()
      if (!Array.isArray(list)) throw new Error(list.error || 'proposal')
      // keep the approval state of tags already approved in the shown revision
      const approved = new Set(
        bindings.value
          .filter((b) => b.approved)
          .map(
            (b) =>
              b.kind +
              '|' +
              b.element_id +
              '|' +
              b.side +
              '|' +
              b.quantity +
              '|' +
              b.tag
          )
      )
      for (const b of list)
        b.approved = approved.has(
          b.kind +
            '|' +
            b.element_id +
            '|' +
            b.side +
            '|' +
            b.quantity +
            '|' +
            b.tag
        )
      bindings.value = list
      proposalInfo.value = r.result
      dirty.value = true
    })

  const saveBindings = () =>
    withBusy(async () => {
      const r = await request({
        type: 'saveBindings',
        modelId: bModelId.value,
        bindings: bindings.value,
      })
      notify(
        t('admin.cimEms.saved', {
          revision: r.result.bindingRevision,
          approved: r.result.approved,
          count: r.result.count,
        }),
        'success'
      )
      await fetchBindings(r.result.bindingRevision)
    })

  const setApproved = (v) => {
    for (const b of filteredBindings.value) b.approved = v
    dirty.value = true
  }

  const removeBinding = (item) => {
    bindings.value = bindings.value.filter((b) => b !== item)
    dirty.value = true
  }

  const rerun = () =>
    withBusy(async () => {
      await request({ type: 'rerun', connectionNumber: Number(runConn.value) })
      notify(t('admin.cimEms.rerunQueued'), 'success')
      setTimeout(fetchRuns, 3000)
    })

  const download = (id) => {
    if (id) window.open(API + 'artifact?id=' + id, '_blank')
  }

  onMounted(fetchModels)

  defineExpose({ fetchModels })
</script>
