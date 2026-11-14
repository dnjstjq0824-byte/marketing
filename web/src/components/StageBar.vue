<template>
  <ol class="stages" :class="{ compact }" aria-label="진행 단계">
    <li v-for="(s, i) in STAGES" :key="s" :class="cls(i)" :aria-current="i === idx ? 'step' : undefined">
      <span class="dot" aria-hidden="true"></span><span v-if="!compact" class="lbl">{{ WORK_STAGE[s] }}</span>
    </li>
  </ol>
</template>

<script setup>
import { computed } from 'vue'
import { WORK_STAGE } from '../stores/labels.js'

const STAGES = ['intake', 'analyze', 'research', 'strategy', 'generate', 'review', 'publish', 'collect']
const props = defineProps({ current: { type: String, default: 'intake' }, compact: { type: Boolean, default: false } })
const idx = computed(() => Math.max(0, STAGES.indexOf(props.current)))

function cls(i) {
  if (i < idx.value) return 'done'
  if (i === idx.value) return 'now'
  return 'todo'
}
</script>

<style scoped>
.stages {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  align-items: center;
  gap: 0;
}
.stages li { display: flex; align-items: center; gap: var(--s1); flex: 1; }
.stages li:last-child { flex: none; }
.stages li:not(:last-child)::after {
  content: '';
  flex: 1;
  height: 2px;
  background: var(--line);
  margin: 0 var(--s1);
  border-radius: 2px;
}
.stages li.done:not(:last-child)::after { background: var(--ok); }
.dot { width: 10px; height: 10px; border-radius: 50%; background: var(--line); flex-shrink: 0; }
li.done .dot { background: var(--ok); }
li.now .dot { background: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
.lbl { font-size: 12px; color: var(--muted); white-space: nowrap; }
li.now .lbl { color: var(--ink); font-weight: 700; }
.compact .lbl { display: none; }
</style>
