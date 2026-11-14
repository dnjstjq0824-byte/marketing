<template>
  <div v-if="actions.length" class="banner" role="alert">
    <div v-for="(a, i) in actions" :key="i" class="row">
      <span class="ic" aria-hidden="true">{{ icon(a.kind) }}</span>
      <span class="msg">{{ a.message }}</span>
      <RouterLink v-if="a.href" class="btn" :to="a.href">{{ a.label || defaultLabel(a.kind) }}</RouterLink>
    </div>
  </div>
</template>

<script setup>
defineProps({ actions: { type: Array, default: () => [] } })

function icon(kind) {
  if (kind === 'login') return '🔑'
  if (kind === 'account') return '👤'
  if (kind === 'failed') return '⚠'
  if (kind === 'reviewer') return '◎'
  return '•'
}

function defaultLabel(kind) {
  if (kind === 'login') return '다시 로그인'
  if (kind === 'account' || kind === 'reviewer') return '계정 연결'
  if (kind === 'failed') return '게시물 확인'
  return '보기'
}
</script>

<style scoped>
.banner {
  background: var(--hold-soft);
  border: 1px solid var(--gold);
  border-radius: var(--r-lg);
  padding: var(--s3) var(--s4);
  display: flex;
  flex-direction: column;
  gap: var(--s2);
}
.row { display: flex; align-items: center; gap: var(--s3); }
.msg { flex: 1; }
.btn {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: var(--s1) var(--s3);
  text-decoration: none;
  color: var(--ink);
  white-space: nowrap;
  font-size: 13px;
}
</style>
