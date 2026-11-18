<template>
  <nav class="crumbs" aria-label="위치">
    <RouterLink to="/">홈</RouterLink>
    <span aria-hidden="true">›</span>
    <RouterLink to="/works">작업</RouterLink>
    <span aria-hidden="true">›</span>
    <strong>{{ title }}</strong>
  </nav>
</template>

<script setup>
defineProps({ title: { type: String, default: '' } })
</script>

<style scoped>
.crumbs {
  display: flex;
  gap: var(--s2);
  align-items: center;
  font-size: 13px;
  color: var(--muted);
  min-width: 0;
}
.crumbs strong {
  color: var(--ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
