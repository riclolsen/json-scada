<template>
  <v-container fluid class="pa-0 fill-height">
    <v-row no-gutters class="fill-height">
      <v-col cols="12" class="fill-height">
        <iframe
          src="/tabular.html"
          width="100%"
          height="94%"
          frameborder="0"
          style="
            position: absolute;
            top: 120;
            left: 0;
            width: 100%;
            height: 94%;
          "
        ></iframe>
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup>
  // No additional setup needed for this component
</script>

<style scoped>
  .fill-height {
    height: 94%;
  }
</style>

<style>
  html {
    overflow-y: hidden;
  }
</style>
