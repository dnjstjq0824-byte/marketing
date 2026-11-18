<template>
  <div class="card center" role="alert">
    <p><strong>불러오지 못했습니다</strong></p>
    <p v-if="message" class="muted small wrap">{{ message }}</p>
    <p class="muted small">{{ hint }}</p>
    <button @click="$emit('retry')">다시 시도</button>
  </div>
</template>

<script setup>
defineProps({
  message: { type: String, default: '' },
  hint: { type: String, default: '잠시 후 다시 시도하세요. 계속되면 새로고침하세요.' },
})
defineEmits(['retry'])
</script>

<style scoped>
.center { text-align: center; align-items: center; }
.wrap { overflow-wrap: anywhere; }
.center button {
  border-radius: var(--r-md);
  padding: var(--s2) var(--s4);
  border: 1px solid var(--accent-dark);
  background: var(--accent);
  color: var(--accent-ink);
  font-weight: 700;
}
.center button:hover { background: var(--accent-dark); }
</style>
