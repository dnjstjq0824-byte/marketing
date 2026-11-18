<template>
  <div class="pg">
    <button class="pu-btn small" :disabled="page <= 1" @click="$emit('goto', page - 1)">이전</button>
    <span class="pu-small pu-muted pu-num">{{ page }} / {{ pages }} · 총 {{ total }}건</span>
    <button class="pu-btn small" :disabled="page >= pages" @click="$emit('goto', page + 1)">다음</button>
  </div>
</template>

<script setup>
defineProps({
  page: { type: Number, default: 1 },
  pages: { type: Number, default: 1 },
  total: { type: Number, default: 0 },
})
defineEmits(['goto'])
</script>

<style scoped>
.pg { display: flex; align-items: center; gap: 12px; justify-content: center; padding: 14px 0 4px; }
</style>
