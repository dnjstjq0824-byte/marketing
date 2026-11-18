<template>
  <div class="empty">
    <p class="art" aria-hidden="true">{{ art }}</p>
    <p><strong>{{ title }}</strong></p>
    <p class="muted small">{{ hint }}</p>
    <RouterLink v-if="to" class="btn" :to="to">{{ actionLabel }}</RouterLink>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, default: '비어 있습니다' },
  hint: { type: String, default: '' },
  to: { type: String, default: '' },
  actionLabel: { type: String, default: '만들기' },
  art: { type: String, default: '○' },
})
</script>

<style scoped>
.empty {
  text-align: center;
  padding: var(--s6) var(--s4);
  display: flex;
  flex-direction: column;
  gap: var(--s2);
  align-items: center;
}
.art { font-size: 40px; color: var(--line); margin: 0; }
.btn {
  background: var(--accent);
  color: var(--accent-ink);
  border-radius: var(--r-md);
  padding: var(--s2) var(--s4);
  text-decoration: none;
  font-weight: 700;
  margin-top: var(--s2);
  transition: background var(--t);
}
.btn:hover { background: var(--accent-dark); }
</style>
