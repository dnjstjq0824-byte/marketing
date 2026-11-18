<template>
  <svg :width="size" :height="size" viewBox="0 0 64 64" class="logo-mark" aria-hidden="true" focusable="false">
    <rect x="2" y="2" width="60" height="60" rx="15" fill="#FF6F0F" />
    <path d="M45 23 C45 16.5 39.5 13 32 13 C24.5 13 19 17.5 19 23.5 C19 33 45 29 45 40.5 C45 46.5 39.5 51 32 51 C25.5 51 20.5 47.5 18.5 42" fill="none" stroke="#ffffff" stroke-width="8" stroke-linecap="round" />
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
