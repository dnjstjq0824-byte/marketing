<template>
  <div v-if="voted" class="gfb done">
    <p class="pu-small">의견 감사합니다. 더 좋은 가이드로 보답할게요.</p>
  </div>
  <div v-else class="gfb">
    <span class="pu-small pu-muted">도움이 되었나요?</span>
    <button class="pu-btn small" @click="voted = true">예</button>
    <button class="pu-btn small" @click="voted = true">아니요</button>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'

const props = defineProps({ articleId: { type: String, required: true } })
const voted = ref(false)
watch(() => props.articleId, () => { voted.value = false })
</script>

<style scoped>
.gfb { display: flex; align-items: center; gap: 8px; margin-top: 6px; padding-top: 12px; border-top: 1px dashed var(--pu-line); }
.gfb.done { border-top-style: solid; }
.gfb p { margin: 0; color: var(--pu-soft); }
</style>
