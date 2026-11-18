<template>
  <RouterLink class="kpi" :to="to">
    <span class="k-label">{{ label }}</span>
    <span class="k-row">
      <b class="num">{{ value }}</b>
      <i v-if="delta !== null" class="delta" :class="cls">{{ arrow }}{{ Math.abs(delta) }}</i>
    </span>
    <span class="k-hint">{{ delta === null ? '실시간' : '어제 대비' }}</span>
  </RouterLink>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  label: { type: String, required: true },
  value: { type: [Number, String], required: true },
  delta: { type: Number, default: null },
  to: { type: String, required: true },
  invert: { type: Boolean, default: false },
})

const arrow = computed(() => (props.delta > 0 ? '▲' : props.delta < 0 ? '▼' : '–'))
const cls = computed(() => {
  if (props.delta === 0) return 'flat'
  const up = props.delta > 0
  return up !== props.invert ? 'good' : 'bad'
})
</script>

<style scoped>
.kpi {
  background: var(--panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-lg);
  padding: var(--s4);
  box-shadow: var(--shadow);
  text-decoration: none;
  color: var(--ink);
  display: flex;
  flex-direction: column;
  gap: var(--s1);
  transition: border-color var(--t), box-shadow var(--t);
}
.kpi:hover { border-color: var(--line); box-shadow: var(--shadow-lg); }
.k-label { font-size: 13px; color: var(--muted); }
.k-row { display: flex; align-items: baseline; gap: var(--s2); }
.k-row b { font-size: 28px; }
.k-hint { font-size: 12px; color: var(--muted); }
.delta { font-style: normal; font-size: 12px; font-weight: 700; border-radius: 999px; padding: 1px 8px; }
.delta.good { color: var(--ok); background: var(--ok-soft); }
.delta.bad { color: var(--warn); background: var(--warn-soft); }
.delta.flat { color: var(--muted); background: var(--soft); }
</style>
