<template>
  <ol class="stages" :class="{ compact }" aria-label="진행 단계">
    <li v-for="(s, i) in STAGES" :key="s" :class="cls(i)" :aria-current="i === idx ? 'step' : undefined" :title="WORK_STAGE[s]">
      <span class="node" aria-hidden="true">
        <svg v-if="i < idx && !compact" width="10" height="10" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M2.5 6.2 5 8.7 9.5 3.5"/></svg>
      </span>
      <span v-if="!compact" class="lbl">{{ WORK_STAGE[s] }}</span>
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
  width: 100%;
}
.stages li { display: flex; align-items: center; gap: 6px; flex: 1; min-width: 0; }
.stages li:last-child { flex: none; }
.stages li:not(:last-child)::after {
  content: '';
  flex: 1;
  height: 2px;
  background: var(--line);
  margin: 0 6px;
  border-radius: 2px;
  transition: background var(--t);
}
.stages li.done:not(:last-child)::after { background: var(--ok); }
.node {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: var(--soft);
  border: 2px solid var(--line);
  color: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all var(--t);
}
li.done .node { background: var(--ok); border-color: var(--ok); }
li.now .node {
  background: var(--accent);
  border-color: var(--accent);
  box-shadow: 0 0 0 4px var(--accent-soft);
  animation: stage-pulse 2s infinite;
}
.lbl { font-size: 12px; font-weight: 500; color: var(--muted); white-space: nowrap; }
li.done .lbl { color: var(--ink-soft); }
li.now .lbl { color: var(--accent-dark); font-weight: 700; }

.compact .node { width: 9px; height: 9px; border-width: 0; background: var(--line); }
.compact li.done .node { background: var(--ok); }
.compact li.now .node { background: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
.compact li:not(:last-child)::after { margin: 0 3px; }

@keyframes stage-pulse {
  0%, 100% { box-shadow: 0 0 0 3px var(--accent-soft); }
  50% { box-shadow: 0 0 0 6px var(--accent-soft); }
}
@media (prefers-reduced-motion: reduce) {
  li.now .node { animation: none; }
}
@media (max-width: 720px) {
  .stages:not(.compact) .lbl { display: none; }
}
</style>
