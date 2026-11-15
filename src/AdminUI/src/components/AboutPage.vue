<template>
  <div class="about-root">
    <v-toolbar density="compact" color="surface" flat class="px-2 about-bar">
      <v-toolbar-title class="text-subtitle-1 font-weight-medium">{{ $t('dashboard.about') }}</v-toolbar-title>
      <v-progress-circular v-if="loading" indeterminate size="18" width="2" class="mr-2"></v-progress-circular>
    </v-toolbar>
    <v-divider></v-divider>
    <div class="about-scroll">
      <v-alert v-if="error" type="error" density="compact" variant="tonal" class="ma-4">
        {{ $t('common.error') }}: {{ error }}
      </v-alert>
      <pre v-else class="about-text">{{ notes }}</pre>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'

// Plain viewer of the release notes (public/release_notes.txt).
const notes = ref('')
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    const resp = await fetch('/release_notes.txt', { cache: 'no-cache' })
    if (!resp.ok) throw new Error(resp.status + ' ' + resp.statusText)
    notes.value = await resp.text()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.about-root {
  height: 100vh;
  display: flex;
  flex-direction: column;
}
.about-bar {
  flex: 0 0 auto;
}
.about-scroll {
  flex: 1 1 auto;
  overflow: auto;
}
.about-text {
  margin: 0;
  padding: 16px 20px 32px;
  font-family: ui-monospace, 'Cascadia Mono', Consolas, 'DejaVu Sans Mono', monospace;
  font-size: 0.85rem;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  color: rgb(var(--v-theme-on-surface));
}
</style>
