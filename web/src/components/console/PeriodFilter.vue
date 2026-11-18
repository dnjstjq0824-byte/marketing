<template>
  <div class="period" role="group" aria-label="기간">
    <button
      v-for="p in presets" :key="p.key" :class="{ on: modelValue.preset === p.key }"
      @click="pick(p.key)"
    >
      {{ p.label }}
    </button>
    <span v-if="modelValue.preset === 'custom'" class="custom">
      <input type="date" :value="modelValue.from" aria-label="시작일" @change="set('from', $event.target.value)" />
      <span aria-hidden="true">~</span>
      <input type="date" :value="modelValue.to" aria-label="종료일" @change="set('to', $event.target.value)" />
    </span>
  </div>
</template>

<script setup>
const props = defineProps({ modelValue: { type: Object, required: true } })
const emit = defineEmits(['update:modelValue'])
const presets = [
  { key: 'today', label: '오늘' },
  { key: '7d', label: '7일' },
  { key: '30d', label: '30일' },
  { key: 'custom', label: '직접' },
]

function iso(d) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function pick(key) {
  const m = { ...props.modelValue, preset: key }
  if (key === 'custom' && !m.from) {
    const t = new Date()
    m.to = iso(t)
    m.from = iso(new Date(t.getTime() - 6 * 864e5))
  }
  emit('update:modelValue', m)
}

function set(k, v) {
  emit('update:modelValue', { ...props.modelValue, [k]: v })
}
</script>

<style scoped>
.period { display: flex; gap: var(--s2); flex-wrap: wrap; align-items: center; }
.period button {
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
  border-radius: 999px;
  padding: var(--s1) var(--s3);
  font-size: 13px;
}
.period button.on { background: var(--ink); border-color: var(--ink); color: var(--bg); font-weight: 700; }
.custom { display: flex; gap: var(--s1); align-items: center; color: var(--muted); font-size: 13px; }
.custom input {
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: var(--s1) var(--s2);
  background: var(--panel);
  color: var(--ink);
}
</style>
