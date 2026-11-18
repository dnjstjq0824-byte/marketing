<template>
  <nav v-if="pages > 1" class="pager" aria-label="페이지">
    <button :disabled="page <= 1" @click="$emit('select', page - 1)">이전</button>
    <button v-for="p in window" :key="p" :class="{ on: p === page }" @click="$emit('select', p)">
      {{ p }}
    </button>
    <button :disabled="page >= pages" @click="$emit('select', page + 1)">다음</button>
  </nav>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  page: { type: Number, required: true },
  pages: { type: Number, required: true },
})
defineEmits(['select'])

const window = computed(() => {
  const out = []
  const lo = Math.max(1, Math.min(props.page - 3, props.pages - 6))
  for (let p = lo; p <= Math.min(props.pages, lo + 6); p++) out.push(p)
  return out
})
</script>

<style scoped>
.pager { display: flex; gap: var(--s1); justify-content: center; align-items: center; flex-wrap: wrap; }
.pager button {
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
  border-radius: var(--r-md);
  padding: var(--s1) var(--s3);
  min-width: 40px;
}
.pager button.on { background: var(--accent); border-color: var(--accent); color: #fff; font-weight: 700; }
.pager button:disabled { opacity: 0.4; }
</style>
