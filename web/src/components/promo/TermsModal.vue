<template>
  <div class="tm-back" @click.self="$emit('close')">
    <div class="tm-modal" role="dialog" :aria-label="title">
      <h3>{{ title }}</h3>
      <div class="tm-body">
        <p v-for="(line, i) in lines" :key="i">{{ line }}</p>
      </div>
      <button class="pu-btn pu-btn-primary block" @click="$emit('close')">닫기</button>
    </div>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, required: true },
  lines: { type: Array, default: () => [] },
})
defineEmits(['close'])
</script>

<style scoped>
.tm-back { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.4); z-index: 150; display: flex; align-items: center; justify-content: center; padding: 20px; }
.tm-modal { background: var(--pu-card); color: var(--pu-ink); border-radius: var(--pu-r); box-shadow: var(--pu-shadow); padding: 22px; max-width: 440px; width: 100%; display: flex; flex-direction: column; gap: 12px; }
.tm-modal h3 { margin: 0; font-size: 17px; }
.tm-body { max-height: 40vh; overflow-y: auto; display: flex; flex-direction: column; gap: 8px; font-size: 14px; color: var(--pu-soft); }
.tm-body p { margin: 0; }
</style>
