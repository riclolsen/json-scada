<template>
  <div class="about-root" @contextmenu.prevent>
    <h1 class="about-logo">
      {json:scada}<sub>Next Generation SCADA/IoT Software</sub>
    </h1>
    <div class="about-stage">
      <div
        class="about-crawl"
        :class="{ running: notes !== '' }"
        :style="{ animationDuration: crawlSeconds + 's' }"
      >{{ notes }}</div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

// Release notes (public/release_notes.txt) scrolling in perspective after the
// logo intro; scroll speed scales with the length of the notes.
const notes = ref('')
const crawlSeconds = computed(() => Math.max(60, Math.round(notes.value.split('\n').length * 1.0)))

onMounted(() => {
  setTimeout(async () => {
    try {
      const resp = await fetch('/release_notes.txt')
      if (resp.ok) notes.value = await resp.text()
    } catch (e) {
      /* notes unavailable: the logo intro still shows */
    }
  }, 3000)
})
</script>

<style scoped>
.about-root {
  position: relative;
  height: calc(100vh - 48px);
  overflow: hidden;
  background-color: #000;
  color: #ff6;
  font-family: arial, verdana, sans-serif;
  font-weight: 700;
  user-select: none;
  -webkit-user-select: none;
}
.about-logo {
  position: absolute;
  left: 50%;
  top: 25%;
  width: 2.6em;
  margin-left: -1.3em;
  font-size: min(10em, 18vw);
  line-height: 0.8em;
  letter-spacing: -0.05em;
  text-align: center;
  color: #000;
  text-shadow: -2px -2px 0 #ff6, 2px -2px 0 #ff6, -2px 2px 0 #ff6, 2px 2px 0 #ff6;
  opacity: 0;
  z-index: 1;
  pointer-events: none;
  animation: about-logo 5s ease-out 0.1s;
}
.about-logo sub {
  display: block;
  font-size: 0.3em;
  letter-spacing: 0;
  line-height: 0.8em;
}
@keyframes about-logo {
  0% {
    transform: scale(1);
    opacity: 1;
  }
  50% {
    opacity: 1;
  }
  100% {
    transform: scale(0.1);
    opacity: 0;
  }
}
.about-stage {
  position: absolute;
  inset: 0;
  perspective: 300px;
  overflow: hidden;
}
.about-stage::after {
  content: ' ';
  position: absolute;
  inset: 0;
  background: url('/images/json-scada.svg') center / contain no-repeat;
  opacity: 0.2;
  pointer-events: none;
}
.about-crawl {
  position: absolute;
  left: 50%;
  top: 100%;
  width: min(18em, 90vw);
  margin-left: max(-9em, -45vw);
  font-size: min(350%, 6vw);
  text-align: justify;
  white-space: pre-wrap;
  transform-origin: 50% 0;
  transform: rotateX(25deg);
}
.about-crawl.running {
  animation-name: about-crawl;
  animation-timing-function: linear;
  animation-iteration-count: infinite;
}
@keyframes about-crawl {
  from {
    transform: rotateX(25deg) translateY(0);
  }
  to {
    transform: rotateX(25deg) translateY(calc(-100% - 100vh));
  }
}
</style>
