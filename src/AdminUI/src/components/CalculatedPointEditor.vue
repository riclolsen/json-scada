<template>
  <div class="calc-editor">
    <!-- formula: catalog search and the code itself (any code, user processes too) -->
    <div class="calc-row">
      <v-autocomplete
        :model-value="catalogValue"
        :items="formulaItems"
        v-model:search="formulaSearch"
        no-filter
        item-value="code"
        item-title="title"
        :label="$t('admin.tags.calc.formula')"
        :placeholder="$t('admin.tags.calc.formulaSearch')"
        prepend-inner-icon="mdi-function-variant"
        variant="outlined"
        density="comfortable"
        clearable
        hide-details="auto"
        @update:model-value="setFormula"
      >
        <template #item="{ props: itemProps, item }">
          <v-list-item v-bind="itemProps" :title="item.title">
            <v-list-item-subtitle class="calc-mono">{{ item.expression }}</v-list-item-subtitle>
            <template #append>
              <v-chip v-if="item.legacy" size="x-small" label variant="tonal" class="me-1">
                {{ $t('admin.tags.calc.legacy') }}
              </v-chip>
              <v-chip size="x-small" label variant="tonal" color="primary">{{ ruleText(item.parcels) }}</v-chip>
            </template>
          </v-list-item>
        </template>
      </v-autocomplete>
      <v-text-field
        :model-value="formula"
        type="number"
        min="0"
        :label="$t('admin.tags.calc.code')"
        variant="outlined"
        density="comfortable"
        hide-details="auto"
        class="calc-code"
        @update:model-value="setFormula"
      ></v-text-field>
    </div>
    <v-checkbox
      v-model="showLegacy"
      :label="$t('admin.tags.calc.showLegacy')"
      density="compact"
      hide-details
      class="mt-1"
    ></v-checkbox>

    <!-- what the chosen formula computes -->
    <v-sheet v-if="def" border rounded class="pa-3 mt-2">
      <div class="text-body-medium font-weight-medium">{{ def.code }} – {{ def.name }}</div>
      <div class="calc-mono text-body-small mt-1">{{ def.expression }}</div>
      <div class="text-body-small text-medium-emphasis mt-1">
        {{ $t('admin.tags.calc.parcels') }}: {{ ruleText(def.parcels) }} ·
        {{ $t('admin.tags.calc.result') }}: {{ resultText(def.result) }} · {{ def.quality }}
      </div>
      <ul v-if="def.notes" class="text-body-small text-medium-emphasis mt-1 calc-notes">
        <li v-for="n in def.notes" :key="n">{{ n }}</li>
      </ul>
    </v-sheet>
    <v-alert v-else-if="formula >= catalog.userFormulaMin" type="info" variant="tonal" density="compact" class="mt-2">
      {{ $t('admin.tags.calc.userFormula', { min: catalog.userFormulaMin }) }}
    </v-alert>
    <v-alert v-else-if="formula > 0" type="warning" variant="tonal" density="compact" class="mt-2">
      {{ $t('admin.tags.calc.unknownFormula', { code: formula }) }}
    </v-alert>

    <!-- parcels, in formula order -->
    <template v-if="formula > 0">
      <div class="d-flex align-center ga-2 mt-4 mb-1">
        <div class="text-label-medium text-uppercase text-medium-emphasis">{{ $t('admin.tags.calc.parcelList') }}</div>
        <v-chip v-if="countProblem" size="small" label variant="tonal" color="warning" prepend-icon="mdi-alert-outline">
          {{ countProblem }}
        </v-chip>
        <v-chip v-else-if="def" size="small" label variant="tonal" color="success" prepend-icon="mdi-check">
          {{ $t('admin.tags.calc.countOk') }}
        </v-chip>
      </div>
      <v-table v-if="parcels.length" density="compact" class="calc-table">
        <tbody>
          <tr v-for="(key, i) in parcels" :key="i + ':' + key">
            <td class="calc-p">P{{ i + 1 }}</td>
            <td class="calc-mono">{{ key }}</td>
            <td class="calc-tag">
              <template v-if="info[key]">
                <div class="calc-ellipsis">{{ info[key].tag }}</div>
                <div class="text-body-small text-medium-emphasis calc-ellipsis">{{ info[key].description }}</div>
              </template>
              <span v-else-if="lookedUp[key]" class="text-error text-body-small">{{ $t('admin.tags.calc.notFound') }}</span>
            </td>
            <td class="calc-value text-body-small">
              <template v-if="info[key]">
                <span :class="{ 'text-medium-emphasis': info[key].invalid }">{{ valueText(info[key]) }}</span>
                <v-icon v-if="info[key].invalid" size="14" color="warning" :title="$t('admin.tags.calc.invalid')">mdi-alert-outline</v-icon>
              </template>
            </td>
            <td class="calc-actions">
              <v-btn icon size="x-small" variant="text" :disabled="i === 0" :title="$t('admin.tags.calc.moveUp')" @click="move(i, -1)">
                <v-icon>mdi-arrow-up</v-icon>
              </v-btn>
              <v-btn icon size="x-small" variant="text" :disabled="i === parcels.length - 1" :title="$t('admin.tags.calc.moveDown')" @click="move(i, 1)">
                <v-icon>mdi-arrow-down</v-icon>
              </v-btn>
              <v-btn icon size="x-small" variant="text" color="error" :title="$t('common.delete')" @click="remove(i)">
                <v-icon>mdi-close</v-icon>
              </v-btn>
            </td>
          </tr>
        </tbody>
      </v-table>
      <div v-else class="text-body-small text-medium-emphasis">{{ $t('admin.tags.calc.noParcels') }}</div>

      <!-- add a parcel: search tags by name, description or key -->
      <div class="calc-row mt-3">
        <v-autocomplete
          v-model="pick"
          v-model:search="search"
          :items="results"
          :loading="searching"
          item-value="_id"
          item-title="tag"
          no-filter
          return-object
          :label="$t('admin.tags.calc.addParcel')"
          :placeholder="$t('admin.tags.calc.addParcelHint')"
          :no-data-text="search ? $t('admin.tags.calc.noMatch') : $t('admin.tags.calc.typeToSearch')"
          prepend-inner-icon="mdi-magnify"
          variant="outlined"
          density="comfortable"
          hide-details
          @update:model-value="addPicked"
        >
          <template #item="{ props: itemProps, item }">
            <v-list-item v-bind="itemProps" :title="item.tag">
              <v-list-item-subtitle>{{ item.description }}</v-list-item-subtitle>
              <template #append>
                <span class="text-body-small text-medium-emphasis me-2">{{ valueText(item) }}</span>
                <v-chip size="x-small" label variant="tonal">{{ item.type }} · {{ item._id }}</v-chip>
              </template>
            </v-list-item>
          </template>
        </v-autocomplete>
        <v-select
          v-model="typeFilter"
          :items="typeItems"
          :label="$t('admin.tags.calc.typeFilter')"
          variant="outlined"
          density="comfortable"
          hide-details
          class="calc-type"
        ></v-select>
      </div>
    </template>
  </div>
</template>

<script setup>
  import { ref, computed, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { postJson, escapeRegex } from '../lib/adminList'
  import catalog from '../lib/calcFormulas.json'

  // Vuetify 4 item slots receive the raw item as `item` (the wrapper is `internalItem`).
  // Formula and parcels of a calculated point (Admin > Tags edit dialog). The
  // formula catalog is generated from the calculations engine table
  // (tools/gen-calc-formulas.mjs); parcels are point keys in formula order.
  const props = defineProps({
    formula: { type: [Number, String], default: 0 },
    parcels: { type: Array, default: () => [] },
    selfKey: { type: [Number, String], default: null }, // the point being edited
  })
  const emit = defineEmits(['update:formula', 'update:parcels'])
  const { t } = useI18n()

  const byCode = new Map(catalog.formulas.map((f) => [f.code, f]))
  const formulaNum = computed(() => Number(props.formula) || 0)
  const def = computed(() => byCode.get(formulaNum.value) || null)
  const catalogValue = computed(() => (def.value ? def.value.code : null))

  const showLegacy = ref(false)
  watch(def, (d) => {
    if (d?.legacy) showLegacy.value = true
  }, { immediate: true })
  // filtered here on code, name and expression (the box shows the chosen title
  // as its text: that alone does not filter)
  const formulaSearch = ref('')
  const allFormulaItems = computed(() =>
    catalog.formulas
      .filter((f) => showLegacy.value || !f.legacy || f.code === formulaNum.value)
      .map((f) => ({
        ...f,
        title: f.code + ' – ' + f.name,
        search: (f.code + ' ' + f.name + ' ' + f.expression).toLowerCase(),
      }))
  )
  const formulaItems = computed(() => {
    const q = String(formulaSearch.value || '').trim().toLowerCase()
    if (!q || (def.value && q === (def.value.code + ' – ' + def.value.name).toLowerCase())) return allFormulaItems.value
    return allFormulaItems.value.filter((f) => f.search.includes(q))
  })
  function setFormula(v) {
    const n = Math.max(0, Math.trunc(Number(v) || 0))
    emit('update:formula', n)
  }

  function ruleText(r) {
    if (!r) return ''
    if ('exact' in r) return t('admin.tags.calc.ruleExact', { n: r.exact })
    if ('max' in r) return t('admin.tags.calc.ruleRange', { min: r.min, max: r.max })
    return t('admin.tags.calc.ruleMin', { n: r.min })
  }
  function resultText(r) {
    return r === 'analog' ? t('admin.tags.calc.analog') : r === 'digital' ? t('admin.tags.calc.digital') : t('admin.tags.calc.any')
  }
  function fits(r, n) {
    if ('exact' in r) return n === r.exact
    return n >= r.min && (!('max' in r) || n <= r.max)
  }
  const countProblem = computed(() => {
    if (!def.value) return ''
    const n = props.parcels.length
    if (fits(def.value.parcels, n)) return ''
    return t('admin.tags.calc.countBad', { rule: ruleText(def.value.parcels), n })
  })

  // --- parcel list ---------------------------------------------------------------
  function setParcels(list) {
    emit('update:parcels', list)
  }
  function move(i, d) {
    const list = [...props.parcels]
    ;[list[i], list[i + d]] = [list[i + d], list[i]]
    setParcels(list)
  }
  function remove(i) {
    const list = [...props.parcels]
    list.splice(i, 1)
    setParcels(list)
  }

  // tag, description and current value of the parcels
  const info = ref({})
  const lookedUp = ref({})
  const PROJ = (p) => ({
    _id: p._id,
    tag: p.tag,
    description: p.description || '',
    type: p.type,
    value: p.value,
    valueString: p.valueString,
    invalid: !!p.invalid,
  })
  async function lookup(keys) {
    const missing = [...new Set(keys.map(Number))].filter((k) => Number.isFinite(k) && !lookedUp.value[k])
    if (!missing.length) return
    try {
      const json = await postJson('/Invoke/auth/listTags', {
        itemsPerPage: missing.length,
        filter: { _id: { $in: missing } },
      })
      if (json.error) return
      const next = { ...info.value }
      for (const p of json.tags || []) next[p._id] = PROJ(p)
      info.value = next
      const seen = { ...lookedUp.value }
      for (const k of missing) seen[k] = true
      lookedUp.value = seen
    } catch (e) {
      // names are a help only
    }
  }
  watch(() => props.parcels, (list) => lookup(list || []), { immediate: true, deep: true })

  function valueText(p) {
    if (p.type === 'string' || p.type === 'json') return String(p.valueString ?? '').slice(0, 24)
    if (p.type === 'digital') return p.valueString || String(p.value ?? '')
    const v = Number(p.value)
    return Number.isFinite(v) ? String(Math.round(v * 1000) / 1000) : ''
  }

  // --- add parcel: server search -------------------------------------------------
  const pick = ref(null)
  const search = ref('')
  const results = ref([])
  const searching = ref(false)
  const typeFilter = ref('')
  const typeItems = computed(() => [
    { value: '', title: t('admin.tags.calc.anyType') },
    { value: 'analog', title: t('admin.tags.calc.analog') },
    { value: 'digital', title: t('admin.tags.calc.digital') },
  ])
  let timer = null
  let seq = 0
  watch([search, typeFilter], () => {
    clearTimeout(timer)
    timer = setTimeout(runSearch, 300)
  })
  async function runSearch() {
    const q = String(search.value || '').trim()
    const my = ++seq
    if (!q) {
      results.value = []
      return
    }
    searching.value = true
    const base = typeFilter.value ? { type: typeFilter.value } : {}
    const re = { $regex: escapeRegex(q), $options: 'i' }
    const queries = [{ ...base, tag: re }, { ...base, description: re }]
    if (/^-?\d+$/.test(q)) queries.unshift({ ...base, _id: Number(q) })
    try {
      const answers = await Promise.all(
        queries.map((filter) => postJson('/Invoke/auth/listTags', { itemsPerPage: 25, filter }))
      )
      if (my !== seq) return
      const out = []
      const seen = new Set()
      for (const a of answers)
        for (const p of a?.tags || [])
          if (!seen.has(p._id) && String(p._id) !== String(props.selfKey)) {
            seen.add(p._id)
            out.push(PROJ(p))
          }
      results.value = out.slice(0, 40)
    } catch (e) {
      if (my === seq) results.value = []
    } finally {
      if (my === seq) searching.value = false
    }
  }
  function addPicked(p) {
    if (!p) return
    info.value = { ...info.value, [p._id]: p }
    lookedUp.value = { ...lookedUp.value, [p._id]: true }
    setParcels([...props.parcels, p._id])
    // ready for the next one
    setTimeout(() => {
      pick.value = null
      search.value = ''
    })
  }
</script>

<style scoped>
  .calc-row {
    display: flex;
    gap: 12px;
    align-items: flex-start;
  }
  .calc-row > :first-child {
    flex: 1 1 0;
    min-width: 0;
  }
  .calc-code {
    flex: 0 0 120px;
  }
  .calc-type {
    flex: 0 0 150px;
  }
  .calc-mono {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    white-space: normal;
    word-break: break-word;
  }
  .calc-notes {
    padding-left: 18px;
  }
  .calc-table {
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 8px;
  }
  .calc-table td {
    padding: 2px 8px !important;
  }
  .calc-p {
    width: 40px;
    font-weight: 600;
  }
  .calc-tag {
    max-width: 260px;
  }
  .calc-ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .calc-value {
    white-space: nowrap;
  }
  .calc-actions {
    width: 100px;
    white-space: nowrap;
    text-align: right;
  }
  @media (max-width: 600px) {
    .calc-row {
      flex-wrap: wrap;
    }
    .calc-code,
    .calc-type {
      flex: 1 1 100%;
    }
  }
</style>
