<template>
  <div class="meter" aria-live="polite">
    <div class="bars"><i v-for="i in 4" :key="i" :class="{ on: score >= i, hot: score >= 3 && score >= i }" /></div>
    <span class="lbl">강도: {{ label }}</span>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { passwordScore, STRENGTH_LABEL } from '../../lib/signupValidate.js'

const props = defineProps({ password: { type: String, default: '' } })
const score = computed(() => passwordScore(props.password))
const label = computed(() => STRENGTH_LABEL[score.value])
</script>

<style scoped>
.meter { display: flex; align-items: center; gap: 8px; }
.bars { display: flex; gap: 4px; }
.bars i { width: 28px; height: 6px; border-radius: 3px; background: var(--pu-line); }
.bars i.on { background: var(--pu-hold); }
.bars i.hot { background: var(--pu-ok); }
.lbl { font-size: 12px; color: var(--pu-muted); font-weight: 400; }
</style>
