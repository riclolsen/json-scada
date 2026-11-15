<template>
  <iframe src="/svgedit" class="app-frame"></iframe>
</template>

<script setup>
  // Page of a separately served tool, framed full height beside the side panel
</script>

<style scoped>
  .app-frame {
    display: block;
    width: 100%;
    height: 100vh;
    border: 0;
  }
</style>

<style>
  html {
    overflow-y: hidden;
  }
</style>
