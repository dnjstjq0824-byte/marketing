<template>
  <div class="modal-back" @click.self="$emit('cancel')">
    <div class="modal" role="alertdialog" :aria-label="title">
      <h3>{{ title }}</h3>
      <p class="muted">{{ message }}</p>
      <div class="btnrow">
        <button class="ghost" @click="$emit('cancel')">취소</button>
        <button :class="{ danger }" @click="$emit('confirm')">{{ confirmLabel }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, required: true },
  message: { type: String, default: '' },
  confirmLabel: { type: String, default: '확인' },
  danger: { type: Boolean, default: false },
})
defineEmits(['confirm', 'cancel'])
</script>

<style scoped>
.modal-back {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 150;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--s4);
}
.modal {
  background: var(--panel);
  border-radius: var(--r-lg);
  box-shadow: var(--shadow-lg);
  padding: var(--s5);
  max-width: 440px;
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: var(--s3);
}
.modal h3 { margin: 0; }
.btnrow { display: flex; gap: var(--s3); justify-content: flex-end; margin-top: var(--s2); }
.btnrow button { border-radius: var(--r-md); padding: var(--s2) var(--s4); border: 1px solid var(--accent-dark); background: var(--accent); color: var(--accent-ink); font-weight: 700; transition: background var(--t), border-color var(--t); }
.btnrow button:hover { background: var(--accent-dark); }
.btnrow button.danger { background: var(--warn); border-color: var(--warn); color: #fff; }
.btnrow button.danger:hover { background: #8f1b13; }
.btnrow button.ghost { border-color: var(--line); background: var(--panel); color: var(--ink); font-weight: 500; }
.btnrow button.ghost:hover { background: var(--soft); }
</style>
