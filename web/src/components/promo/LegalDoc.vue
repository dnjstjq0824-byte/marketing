<template>
  <div class="pu-card legal">
    <p class="pu-small pu-muted">{{ version }} · {{ notice }}</p>
    <section v-for="s in article" :key="s.h">
      <h2>{{ s.h }}</h2>
      <p v-for="(line, i) in s.lines" :key="i">{{ line }}</p>
    </section>
  </div>
</template>

<script setup>
defineProps({
  article: { type: Array, required: true },
  version: { type: String, required: true },
  notice: { type: String, required: true },
})
</script>

<style scoped>
.legal { display: flex; flex-direction: column; gap: 20px; }
.legal h2 { font-size: 16px; margin: 0 0 8px; }
.legal p { margin: 0 0 6px; font-size: 14px; color: var(--pu-soft); line-height: 1.7; }
.legal section { margin: 0; }
</style>
