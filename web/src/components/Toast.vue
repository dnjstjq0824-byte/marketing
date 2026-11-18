<template>
  <div class="toasts" aria-live="polite">
    <div v-for="t in toasts" :key="t.id" :class="'toast t-' + t.kind">
      <span>{{ t.message }}</span>
      <button v-if="t.kind === 'error'" class="ghost small" @click="dismiss(t.id)" aria-label="닫기">닫기</button>
    </div>
  </div>
</template>

<script setup>
import { dismiss, toasts } from '../stores/toast.js'
</script>

<style scoped>
.toasts {
  position: fixed;
  bottom: var(--s5);
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  flex-direction: column;
  gap: var(--s2);
  z-index: 200;
  max-width: min(480px, calc(100vw - 32px));
}
.toast {
  background: var(--ink);
  color: #fff;
  border-radius: var(--r-md);
  padding: var(--s3) var(--s4);
  box-shadow: var(--shadow-lg);
  display: flex;
  align-items: center;
  gap: var(--s3);
  font-size: 13px;
  transition: opacity var(--t), transform var(--t);
}
.toast.t-error { background: var(--warn); }
.toast button { color: #fff; border-color: rgba(255, 255, 255, 0.5); }
@media (max-width: 900px) {
  .toasts { bottom: 76px; }
}
</style>
