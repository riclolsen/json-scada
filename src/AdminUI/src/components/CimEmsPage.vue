<template>
  <v-container fluid class="cim-ems-page ma-0 pa-0">
    <cim-ems-tab />
  </v-container>
</template>

<script setup>
  import { onMounted, onUnmounted } from 'vue'
  import CimEmsTab from './CimEmsTab.vue'

  // DashboardPage hides the document scrollbar when it unmounts; the CIM-EMS
  // lists grow with the page, so page scrolling must be on while it is shown.
  let previousOverflowY = ''
  onMounted(() => {
    previousOverflowY = document.documentElement.style.overflowY
    document.documentElement.style.overflowY = 'auto'
  })
  onUnmounted(() => {
    document.documentElement.style.overflowY = previousOverflowY
  })
</script>

<style scoped>
  .cim-ems-page {
    padding-top: 1rem;
    padding-bottom: 1rem;
  }
</style>
