<template>
  <div class="cd-root">
    <!-- one app opened inside the page (?app=<folder>) -->
    <template v-if="current">
      <v-toolbar density="compact" color="surface" flat class="px-1 cd-bar">
        <v-btn icon variant="text" size="small" :title="$t('customDev.back')" @click="closeApp">
          <v-icon size="20">mdi-arrow-left</v-icon>
        </v-btn>
        <div class="cd-bar-title">
          <span class="text-body-large font-weight-medium text-truncate">{{ currentInfo.title || current }}</span>
          <span class="cd-folder text-truncate">{{ current }}</span>
        </div>
        <v-spacer></v-spacer>
        <v-btn icon variant="text" size="small" :title="$t('common.refresh')" @click="frameKey++">
          <v-icon size="20">mdi-refresh</v-icon>
        </v-btn>
        <v-btn icon variant="text" size="small" :title="$t('customDev.openNewWindow')" :href="appUrl(current)" target="_blank">
          <NavIcon name="openNew" :size="20" />
        </v-btn>
      </v-toolbar>
      <v-divider></v-divider>
      <iframe :key="frameKey" :src="appUrl(current)" class="cd-frame" :title="currentInfo.title || current"></iframe>
    </template>

    <!-- list of the apps -->
    <template v-else>
      <v-toolbar density="compact" color="surface" flat class="px-2 cd-bar">
        <v-toolbar-title class="text-body-large font-weight-medium">{{ $t('dashboard.customDevelopments') }}</v-toolbar-title>
        <v-chip v-if="apps.length" size="small" variant="tonal" class="mr-2">{{ apps.length }}</v-chip>
        <v-text-field
          v-if="apps.length > 3"
          v-model="search"
          :placeholder="$t('customDev.search')"
          prepend-inner-icon="mdi-magnify"
          density="compact"
          variant="outlined"
          hide-details
          clearable
          single-line
          class="cd-search mr-2"
        ></v-text-field>
        <v-btn icon variant="text" size="small" :title="$t('common.refresh')" :loading="loading" @click="load">
          <v-icon size="20">mdi-refresh</v-icon>
        </v-btn>
      </v-toolbar>
      <v-divider></v-divider>

      <div class="cd-scroll">
        <div class="cd-content">
          <p class="text-body-medium text-medium-emphasis mb-5">{{ $t('customDev.subtitle') }}</p>

          <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-4">
            {{ $t('common.error') }} {{ error }}
            <template #append>
              <v-btn size="small" variant="text" @click="load">{{ $t('customDev.retry') }}</v-btn>
            </template>
          </v-alert>

          <div v-if="loading && !apps.length" class="cd-grid">
            <v-skeleton-loader v-for="n in 3" :key="n" type="article" class="cd-card"></v-skeleton-loader>
          </div>

          <div v-else-if="!error && !apps.length" class="cd-empty">
            <NavIcon name="customDevelopments" :size="40" />
            <div class="text-title-medium mt-3">{{ $t('customDev.empty') }}</div>
            <div class="text-body-medium text-medium-emphasis mt-1">{{ $t('customDev.emptyHint') }}</div>
          </div>

          <div v-else-if="!filtered.length && apps.length" class="cd-empty">
            <div class="text-body-medium text-medium-emphasis">{{ $t('customDev.noMatch') }}</div>
          </div>

          <div v-else class="cd-grid">
            <v-card
              v-for="a in filtered"
              :key="a.folder"
              variant="outlined"
              class="cd-card"
              @click="openApp(a.folder, $event)"
              @auxclick.prevent="openNew(a.folder)"
            >
              <div class="cd-card-head">
                <div class="cd-avatar"><NavIcon name="customDevelopments" :size="22" /></div>
                <div class="cd-card-titles">
                  <div class="text-title-medium cd-title" :title="a.title || a.folder">{{ a.title || a.folder }}</div>
                  <div class="cd-folder text-truncate">{{ a.folder }}</div>
                </div>
              </div>
              <div class="cd-desc text-body-medium text-medium-emphasis">
                {{ a.description || $t('customDev.noDescription') }}
              </div>
              <div class="cd-card-foot">
                <span v-if="a.modified" class="text-body-small text-medium-emphasis">
                  {{ $t('customDev.built', { date: fmtDate(a.modified) }) }}
                </span>
                <v-spacer></v-spacer>
                <v-btn
                  icon
                  variant="text"
                  size="small"
                  :title="$t('customDev.openNewWindow')"
                  @click.stop.prevent="openNew(a.folder)"
                >
                  <NavIcon name="openNew" :size="18" />
                </v-btn>
                <v-btn variant="tonal" size="small" color="primary" @click.stop.prevent="openApp(a.folder)">
                  {{ $t('customDev.open') }}
                </v-btn>
              </div>
            </v-card>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
  import { computed, onMounted, onUnmounted, ref } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  import { useI18n } from 'vue-i18n'
  import NavIcon from './NavIcon.vue'

  // Custom developments (src/custom-developments): the apps served by
  // server_realtime_auth under /custom-developments/<folder>/, listed from
  // GET /custom-developments?format=json; an app opens inside the page
  // (?app=<folder>) or in a new window.
  const route = useRoute()
  const router = useRouter()
  const { locale } = useI18n()

  const apps = ref([])
  const loading = ref(false)
  const error = ref('')
  const search = ref('')
  const frameKey = ref(0)
  let disposed = false

  const FOLDER_RE = /^[^/\\?#]+$/
  const current = computed(() => {
    const a = route.query.app
    return typeof a === 'string' && FOLDER_RE.test(a) ? a : ''
  })
  const currentInfo = computed(() => apps.value.find((a) => a.folder === current.value) || {})

  const filtered = computed(() => {
    const q = (search.value || '').trim().toLowerCase()
    if (!q) return apps.value
    return apps.value.filter((a) =>
      [a.folder, a.title, a.description].some((s) => (s || '').toLowerCase().includes(q))
    )
  })

  const appUrl = (folder) => '/custom-developments/' + encodeURIComponent(folder) + '/'

  function openApp(folder, ev) {
    if (ev && (ev.ctrlKey || ev.metaKey || ev.shiftKey)) return openNew(folder)
    router.push({ path: '/custom-developments', query: { app: folder } })
  }
  function openNew(folder) {
    window.open(appUrl(folder), '_blank', 'noopener')
  }
  function closeApp() {
    router.push({ path: '/custom-developments' })
  }

  function fmtDate(iso) {
    const d = new Date(iso)
    if (isNaN(d.getTime())) return ''
    return d.toLocaleDateString(locale.value, { year: 'numeric', month: 'short', day: 'numeric' })
  }

  async function load() {
    loading.value = true
    error.value = ''
    try {
      const resp = await fetch('/custom-developments?format=json', { cache: 'no-store' })
      if (disposed) return
      if (!resp.ok) throw new Error(resp.status + ' ' + resp.statusText)
      const data = await resp.json()
      if (disposed) return
      apps.value = Array.isArray(data?.apps) ? data.apps : []
    } catch (e) {
      if (!disposed) error.value = e.message
    } finally {
      if (!disposed) loading.value = false
    }
  }

  onMounted(load)
  onUnmounted(() => {
    disposed = true
  })
</script>

<style scoped>
  .cd-root {
    height: 100vh;
    display: flex;
    flex-direction: column;
  }
  .cd-bar {
    flex: 0 0 auto;
  }
  .cd-bar-title {
    display: flex;
    flex-direction: column;
    min-width: 0;
    line-height: 1.2;
    margin-left: 4px;
  }
  .cd-search {
    max-width: 260px;
  }
  .cd-frame {
    flex: 1 1 auto;
    width: 100%;
    border: 0;
    display: block;
  }
  .cd-scroll {
    flex: 1 1 auto;
    overflow-y: auto;
  }
  .cd-content {
    max-width: 1200px;
    margin: 0 auto;
    padding: 24px 20px 32px;
  }
  .cd-grid {
    display: grid;
    gap: 16px;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  }
  .cd-card {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px;
    border-radius: 12px;
    transition:
      border-color 0.15s,
      transform 0.15s,
      box-shadow 0.15s;
  }
  .cd-card:hover,
  .cd-card:focus-visible {
    border-color: rgb(var(--v-theme-primary));
    transform: translateY(-2px);
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.12);
  }
  .cd-card-head {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
  }
  .cd-card-titles {
    min-width: 0;
  }
  .cd-title {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    line-height: 1.3;
  }
  .cd-avatar {
    flex: 0 0 auto;
    width: 40px;
    height: 40px;
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: rgb(var(--v-theme-primary));
    background: rgba(var(--v-theme-primary), 0.12);
  }
  .cd-folder {
    font-family: ui-monospace, Consolas, monospace;
    font-size: 12px;
    opacity: 0.7;
  }
  .cd-desc {
    flex: 1 1 auto;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
    min-height: 3em;
  }
  .cd-card-foot {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .cd-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: 48px 24px;
    border: 1px dashed rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 12px;
    opacity: 0.9;
  }
</style>
