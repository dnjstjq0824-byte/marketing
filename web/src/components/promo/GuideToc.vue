<template>
  <nav class="gtoc" aria-label="이 페이지 목차">
    <p class="gtoc-t">이 페이지 목차</p>
    <button
      v-for="it in items"
      :key="it.id"
      :class="{ on: it.id === activeId }"
      @click="$emit('jump', it.id)"
    >
      {{ it.title }}
    </button>
  </nav>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, watch } from 'vue'

const props = defineProps({
  items: { type: Array, default: () => [] },
  activeId: { type: String, default: '' },
})
const emit = defineEmits(['jump', 'update:activeId'])

let obs = null
function observe() {
  if (obs) obs.disconnect()
  obs = new IntersectionObserver((entries) => {
    entries.forEach((e) => {
      if (e.isIntersecting) emit('update:activeId', e.target.dataset.guideArticle)
    })
  }, { rootMargin: '-20% 0px -70% 0px' })
  nextTick(() => document.querySelectorAll('[data-guide-article]').forEach((el) => obs.observe(el)))
}
onMounted(observe)
watch(() => props.items, observe)
onBeforeUnmount(() => { if (obs) obs.disconnect() })
</script>

<style scoped>
.gtoc {
  position: sticky;
  top: 84px;
  border: 1px solid var(--pu-line);
  border-radius: var(--pu-r);
  background: var(--pu-card);
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.gtoc-t { margin: 0 0 6px; font-size: 13px; font-weight: 800; color: var(--pu-muted); }
.gtoc button {
  border: none;
  background: none;
  text-align: left;
  padding: 8px 10px;
  border-radius: 8px;
  font-size: 13px;
  color: var(--pu-soft);
  cursor: pointer;
  font-family: inherit;
}
.gtoc button:hover { background: var(--pu-line-soft); }
.gtoc button.on { background: var(--pu-accent-soft); color: var(--pu-accent-dark); font-weight: 800; }
@media (max-width: 900px) {
  .gtoc { position: static; flex-direction: row; flex-wrap: wrap; align-items: center; }
  .gtoc-t { width: 100%; }
}
</style>
