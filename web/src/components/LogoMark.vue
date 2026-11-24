<template>
  <svg :width="size" :height="size" viewBox="0 0 64 64" class="logo-mark" aria-hidden="true" focusable="false">
    <defs>
      <linearGradient id="somun-g" x1="0" y1="0" x2="1" y2="1">
        <stop offset="0%" stop-color="#06B6D4" />
        <stop offset="100%" stop-color="#0E7490" />
      </linearGradient>
    </defs>
    <rect x="2" y="2" width="60" height="60" rx="16" fill="url(#somun-g)" />
    <path d="M44.5 23 C44.5 16.8 39.2 13.5 32 13.5 C24.8 13.5 19.5 17.8 19.5 23.5 C19.5 32.5 44.5 29 44.5 40 C44.5 46 39.2 50.5 32 50.5 C25.5 50.5 20.5 47 18.8 41.8" fill="none" stroke="#ffffff" stroke-width="7.5" stroke-linecap="round" />
  </svg>
</template>

<script setup>
defineProps({
  size: { type: [Number, String], default: 32 },
})
</script>

<style scoped>
.logo-mark { flex: none; display: block; }
</style>
