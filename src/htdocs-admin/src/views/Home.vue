<template>
  <div class="home">
    <TabbedOptions />
  </div>
</template>

<script>
// @ is an alias to /src
import TabbedOptions from '@/components/TabbedOptions.vue'

export default {
  name: 'Home',
  components: {
    TabbedOptions
  }
}
</script>
