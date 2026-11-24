<template>
  <div class="trend">
    <div v-if="hoverItem" class="tipbar">
      <strong>{{ hoverItem.date }}</strong>
      <span>발행 <b class="num">{{ hoverItem.published }}</b>건</span>
      <span v-if="hoverItem.failed" class="failkey">실패 <b class="num">{{ hoverItem.failed }}</b>건</span>
    </div>
    <svg viewBox="0 0 640 190" role="img" :aria-label="summary">
      <defs>
        <linearGradient id="somun-bar" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-color="var(--accent-light)" />
          <stop offset="100%" stop-color="var(--accent)" />
        </linearGradient>
      </defs>
      <line v-for="y in [150, 95, 40]" :key="y" :y1="y" :y2="y" x1="34" x2="628" class="grid" />
      <text x="26" y="44" class="ytick num">{{ max }}</text>
      <text x="26" y="99" class="ytick num">{{ Math.round(max / 2) }}</text>
      <text x="26" y="154" class="ytick num">0</text>
      <g v-for="(d, i) in days" :key="d.date" class="col" @mouseenter="hoverIdx = i" @mouseleave="hoverIdx = -1">
        <rect :x="38 + i * step" y="30" :width="step" height="130" class="hit" />
        <rect
          :x="38 + i * step + (step - barW) / 2"
          :y="150 - h(d.published)"
          :width="barW"
          :height="h(d.published)"
          rx="4"
          class="bar"
          :class="{ active: hoverIdx === i }"
        >
          <title>{{ d.date }} 발행 {{ d.published }}건 · 실패 {{ d.failed }}건</title>
        </rect>
        <circle
          v-show="d.failed > 0"
          :cx="38 + i * step + step / 2"
          cy="160"
          r="3.5"
          class="fail"
        />
      </g>
      <text v-for="t in ticks" :key="t.x" :x="t.x" y="182" class="tick">{{ t.label }}</text>
    </svg>
    <p class="muted small sum">{{ summary }} <span v-if="hasFail" class="failkey">● 실패 발생일</span></p>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'

const props = defineProps({ days: { type: Array, default: () => [] } })
const hoverIdx = ref(-1)
const hoverItem = computed(() => (hoverIdx.value >= 0 ? props.days[hoverIdx.value] : null))
const max = computed(() => Math.max(1, ...props.days.map((d) => d.published)))
const step = computed(() => (props.days.length > 0 ? 586 / props.days.length : 586))
const barW = computed(() => Math.max(6, Math.min(28, step.value * 0.56)))
const h = (v) => (v > 0 ? Math.max(5, Math.round((v / max.value) * 110)) : 2)
const hasFail = computed(() => props.days.some((d) => d.failed > 0))
const ticks = computed(() => {
  const n = props.days.length
  if (!n) return []
  const k = Math.max(1, Math.ceil(n / 6))
  return props.days
    .map((d, i) => ({ d, i }))
    .filter(({ i }) => i % k === 0 || i === n - 1)
    .map(({ d, i }) => ({ x: 38 + i * step.value + step.value / 2, label: d.date.slice(5) }))
})
const summary = computed(() => {
  const p = props.days.reduce((a, d) => a + d.published, 0)
  const f = props.days.reduce((a, d) => a + d.failed, 0)
  return props.days.length ? `최근 ${props.days.length}일 누적 발행 ${p}건 · 실패 ${f}건` : '발행 추이 데이터가 아직 없습니다'
})
</script>

<style scoped>
.trend { display: flex; flex-direction: column; gap: 8px; position: relative; }
.tipbar {
  position: absolute;
  top: 0;
  right: 0;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 999px;
  padding: 3px 12px;
  font-size: 12px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
svg { width: 100%; height: auto; display: block; }
.grid { stroke: var(--line-soft); stroke-width: 1; stroke-dasharray: 3 3; }
.hit { fill: transparent; cursor: pointer; }
.bar { fill: url(#somun-bar); opacity: 0.88; transition: opacity var(--t); }
.bar.active, .col:hover .bar { opacity: 1; }
.fail { fill: var(--warn); }
.tick { fill: var(--muted); font-size: 11px; text-anchor: middle; }
.ytick { fill: var(--muted); font-size: 10.5px; text-anchor: end; }
.sum { margin: 0; }
.failkey { color: var(--warn); font-weight: 600; }
</style>
