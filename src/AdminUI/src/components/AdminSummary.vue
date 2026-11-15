<template>
  <div class="admin-summary">
    <div
      v-for="s in tiles"
      :key="s.key"
      class="admin-tile"
      :class="{ 'admin-tile-active': s.filter !== undefined && modelValue === s.filter }"
      :role="s.filter !== undefined ? 'button' : undefined"
      :tabindex="s.filter !== undefined ? 0 : undefined"
      @click="select(s)"
      @keyup.enter="select(s)"
    >
      <v-icon :color="s.color" size="22">{{ s.icon }}</v-icon>
      <div>
        <div class="admin-tile-value">{{ s.value }}</div>
        <div class="admin-tile-label">{{ s.label }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
  // Row of summary tiles of an Admin section. A tile with `filter` toggles
  // that value as the section's filter (v-model; null when cleared).
  // tiles: [{ key, icon, color, value, label, filter? }]
  const props = defineProps({
    tiles: { type: Array, required: true },
    modelValue: { type: [String, Number], default: null },
  })
  const emit = defineEmits(['update:modelValue'])
  function select(s) {
    if (s.filter === undefined) return
    emit('update:modelValue', props.modelValue === s.filter ? null : s.filter)
  }
</script>
