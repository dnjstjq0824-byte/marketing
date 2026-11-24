<template>
  <span class="chip" :class="'c-' + kind"><slot /></span>
</template>

<script setup>
defineProps({ kind: { type: String, default: 'neutral' } })
</script>

<style scoped>
.chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11.5px;
  font-weight: 700;
  line-height: 20px;
  padding: 1px 9px;
  border-radius: 999px;
  background: var(--soft);
  color: var(--ink-soft);
  white-space: nowrap;
  flex-shrink: 0;
}
.chip.c-ok { background: var(--ok-soft); color: var(--ok); }
.chip.c-warn { background: var(--warn-soft); color: var(--warn); }
.chip.c-hold { background: var(--hold-soft); color: var(--hold); }
.chip.c-info { background: var(--accent-soft); color: var(--accent-dark); }
.chip.c-gold { background: var(--gold-soft); color: var(--gold); }
</style>
