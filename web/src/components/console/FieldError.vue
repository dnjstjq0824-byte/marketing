<template>
  <p v-if="message" class="ferr" role="alert">
    <span class="fmsg">{{ message }}</span>
    <span v-if="hint" class="fhint">{{ hint }}</span>
  </p>
</template>

<script setup>
defineProps({
  message: { type: String, default: '' },
  hint: { type: String, default: '' },
})
</script>

<style scoped>
.ferr {
  margin: var(--s1) 0 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow-wrap: anywhere;
}
.fmsg { color: var(--warn); font-size: 13px; font-weight: 600; }
.fhint { color: var(--muted); font-size: 12px; font-weight: 400; }
</style>
