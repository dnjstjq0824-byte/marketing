<template>
  <RouterLink class="kpi" :class="toneClass" :to="to">
    <div class="k-top">
      <span class="k-label">{{ label }}</span>
      <span v-if="delta !== null" class="delta" :class="cls">{{ arrow }}{{ Math.abs(delta) }}</span>
    </div>
    <div class="k-row">
      <b class="num">{{ value }}</b>
      <span v-if="unit" class="k-unit">{{ unit }}</span>
    </div>
    <span class="k-hint">{{ hint || (delta === null ? '실시간 집계' : '어제 대비') }}</span>
  </RouterLink>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  label: { type: String, required: true },
  value: { type: [Number, String], required: true },
  unit: { type: String, default: '' },
  hint: { type: String, default: '' },
  tone: { type: String, default: '' },
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
const toneClass = computed(() => (props.tone ? `tone-${props.tone}` : ''))
</script>

<style scoped>
.kpi {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--r-lg);
  padding: 18px 20px;
  box-shadow: var(--shadow);
  text-decoration: none;
  color: var(--ink);
  display: flex;
  flex-direction: column;
  gap: 6px;
  position: relative;
  overflow: hidden;
  transition: border-color var(--t), box-shadow var(--t), transform var(--t);
}
.kpi::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 3px;
  background: var(--line);
  transition: background var(--t);
}
.kpi.tone-accent::before { background: var(--accent); }
.kpi.tone-ok::before { background: var(--ok); }
.kpi.tone-warn::before { background: var(--warn); }
.kpi.tone-hold::before { background: var(--hold); }
.kpi:hover { border-color: var(--accent); box-shadow: var(--shadow-lg); transform: translateY(-1px); }
.k-top { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.k-label { font-size: 13px; font-weight: 600; color: var(--muted); }
.k-row { display: flex; align-items: baseline; gap: 4px; }
.k-row b { font-size: 30px; font-weight: 800; line-height: 1.15; letter-spacing: -0.03em; }
.k-unit { font-size: 13px; color: var(--muted); font-weight: 600; }
.k-hint { font-size: 12px; color: var(--muted); }
.delta { font-style: normal; font-size: 11.5px; font-weight: 700; border-radius: 999px; padding: 2px 8px; }
.delta.good { color: var(--ok); background: var(--ok-soft); }
.delta.bad { color: var(--warn); background: var(--warn-soft); }
.delta.flat { color: var(--muted); background: var(--soft); }
</style>
