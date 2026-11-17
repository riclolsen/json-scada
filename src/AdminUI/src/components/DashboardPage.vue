<template>
  <div class="home-root">
    <div class="home-card">
      <img src="/images/json-scada.svg" alt="{json:scada}" class="home-logo" />
      <div class="text-title-large font-weight-medium mt-4">{{ $t('nav.welcomeTitle', { product: '{json:scada}' }) }}</div>
      <div v-if="user" class="text-body-medium text-medium-emphasis mt-1">
        {{ $t('nav.signedInAs', { user }) }}
      </div>
      <div class="home-hint text-body-medium mt-6">
        <div class="hint-row">
          <NavIcon name="openHere" :size="18" />
          <span>{{ $t('nav.hintSameWindow') }}</span>
        </div>
        <div class="hint-row">
          <NavIcon name="openNew" :size="18" />
          <span>{{ $t('nav.hintNewWindow') }}</span>
        </div>
        <div class="hint-row">
          <NavIcon name="collapse" :size="18" />
          <span>{{ $t('nav.hintCollapse') }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import NavIcon from './NavIcon.vue'
import { getUserObj } from '../lib/viewerHelpers'

// Landing page after sign-in; navigation lives in the side panel (AppSidebar).
const user = computed(() => (getUserObj() || {}).username || '')
</script>

<style scoped>
.home-root {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px 16px;
}
.home-card {
  max-width: 460px;
  text-align: center;
}
.home-logo {
  width: min(320px, 70vw);
  height: auto;
}
.home-hint {
  display: inline-flex;
  flex-direction: column;
  gap: 10px;
  text-align: start;
  opacity: 0.8;
}
.hint-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
</style>
