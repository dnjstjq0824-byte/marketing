<template>
  <div class="trend">
    <svg viewBox="0 0 600 190" role="img" :aria-label="summary">
      <line v-for="y in [150, 95, 40]" :key="y" :y1="y" :y2="y" x1="30" x2="596" class="grid" />
      <rect
        v-for="(d, i) in days" :key="d.date" :x="34 + i * step" :y="150 - h(d.published)"
        :width="barW" :height="h(d.published)" rx="2" class="bar"
      >
        <title>{{ d.date }} 발행 {{ d.published }}건 · 실패 {{ d.failed }}건</title>
      </rect>
      <circle
        v-for="(d, i) in days" v-show="d.failed > 0" :key="'f' + d.date"
        :cx="34 + i * step + barW / 2" cy="160" r="3.5" class="fail"
      />
      <text v-for="t in ticks" :key="t.x" :x="t.x" y="180" class="tick">{{ t.label }}</text>
    </svg>
    <p class="muted small sum">{{ summary }} <span v-if="hasFail" class="failkey">● 실패 있음</span></p>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({ days: { type: Array, default: () => [] } })
const max = computed(() => Math.max(1, ...props.days.map((d) => d.published)))
const step = computed(() => (props.days.length > 1 ? 562 / props.days.length : 562))
const barW = computed(() => Math.max(4, Math.min(28, step.value * 0.55)))
const h = (v) => Math.max(2, Math.round((v / max.value) * 110))
const hasFail = computed(() => props.days.some((d) => d.failed > 0))
const ticks = computed(() => {
  const n = props.days.length
  if (!n) return []
  const k = Math.max(1, Math.ceil(n / 5))
  return props.days
    .map((d, i) => ({ d, i }))
    .filter(({ i }) => i % k === 0 || i === n - 1)
    .map(({ d, i }) => ({ x: 34 + i * step.value, label: d.date.slice(5) }))
})
const summary = computed(() => {
  const p = props.days.reduce((a, d) => a + d.published, 0)
  const f = props.days.reduce((a, d) => a + d.failed, 0)
  return props.days.length ? `최근 ${props.days.length}일 발행 ${p}건 · 실패 ${f}건` : '추이 자료 없음'
})
</script>

<style scoped>
.trend { display: flex; flex-direction: column; gap: var(--s2); }
svg { width: 100%; height: auto; display: block; }
.grid { stroke: var(--line-soft); stroke-width: 1; }
.bar { fill: var(--accent); }
.fail { fill: var(--warn); }
.tick { fill: var(--muted); font-size: 11px; }
.sum { margin: 0; }
.failkey { color: var(--warn); }
</style>
