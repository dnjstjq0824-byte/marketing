<template>
  <RouterLink class="card" :to="`/works/${work.id}`">
    <span v-if="work.thumb" class="thumb"><img :src="work.thumb" alt="" loading="lazy" /></span>
    <span v-else class="thumb empty" aria-hidden="true">▣</span>
    <span class="body">
      <strong class="t">{{ work.title }}</strong>
      <span class="meta muted small">{{ work.detail }} · {{ agoText }}</span>
      <StageBar :current="work.stage" compact />
    </span>
  </RouterLink>
</template>

<script setup>
import { computed } from 'vue'
import StageBar from './StageBar.vue'

const props = defineProps({ work: { type: Object, required: true } })

const agoText = computed(() => {
  const ago = props.work.ago || ''
  if (!ago) return '방금'
  const d = new Date(ago.endsWith('Z') ? ago : ago + 'Z')
  if (Number.isNaN(d.getTime())) return '방금'
  const mins = Math.max(0, Math.round((Date.now() - d.getTime()) / 60000))
  if (mins < 1) return '방금'
  if (mins < 60) return `${mins}분 전`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}시간 전`
  return `${Math.floor(hours / 24)}일 전`
})
</script>

<style scoped>
.card {
  display: flex;
  gap: var(--s4);
  background: var(--panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-lg);
  padding: var(--s4);
  text-decoration: none;
  color: var(--ink);
  box-shadow: var(--shadow);
}
.thumb {
  width: 64px;
  height: 64px;
  border-radius: var(--r-md);
  overflow: hidden;
  flex-shrink: 0;
  background: var(--soft);
}
.thumb img { width: 100%; height: 100%; object-fit: cover; }
.thumb.empty { display: flex; align-items: center; justify-content: center; color: var(--muted); font-size: 24px; }
.body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: var(--s1); }
.t { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.meta { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
