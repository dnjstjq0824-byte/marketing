<template>
  <RouterLink class="wcard" :to="`/works/${work.id}`">
    <span v-if="work.thumb" class="thumb"><AuthImg :src="work.thumb" alt="" /></span>
    <span v-else class="thumb empty" aria-hidden="true">
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
        <rect x="3" y="4" width="18" height="16" rx="3" />
        <path d="M8 9h8M8 13h5" />
      </svg>
    </span>
    <span class="body">
      <span class="toprow">
        <strong class="t">{{ work.title }}</strong>
        <StatusChip :kind="stageKind">{{ stageLabel }}</StatusChip>
      </span>
      <span class="meta muted small">
        <span v-if="work.business" class="biz">{{ work.business }}</span>
        <span v-if="work.business && work.detail"> · </span>
        <span>{{ work.detail }}</span>
        <span> · {{ agoText }}</span>
      </span>
      <StageBar :current="work.stage" compact />
    </span>
  </RouterLink>
</template>

<script setup>
import { computed } from 'vue'
import AuthImg from './AuthImg.vue'
import StageBar from './StageBar.vue'
import StatusChip from './StatusChip.vue'
import { WORK_STAGE } from '../stores/labels.js'

const props = defineProps({ work: { type: Object, required: true } })

const stageLabel = computed(() => WORK_STAGE[props.work.stage] || props.work.stage || '진행')
const stageKind = computed(() => {
  if (props.work.state === 'failed') return 'warn'
  if (props.work.state === 'held') return 'hold'
  if (props.work.stage === 'collect' || props.work.state === 'published') return 'ok'
  return 'info'
})

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
.wcard {
  display: flex;
  gap: 14px;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--r-lg);
  padding: 14px 16px;
  text-decoration: none;
  color: var(--ink);
  box-shadow: var(--shadow);
  align-items: center;
  transition: border-color var(--t), box-shadow var(--t), transform var(--t);
}
.wcard:hover { border-color: var(--accent); box-shadow: var(--shadow-lg); transform: translateY(-1px); }
.thumb {
  width: 60px;
  height: 60px;
  border-radius: var(--r-md);
  overflow: hidden;
  flex-shrink: 0;
  background: var(--soft);
  border: 1px solid var(--line-soft);
}
.thumb img { width: 100%; height: 100%; object-fit: cover; }
.thumb.empty { display: flex; align-items: center; justify-content: center; color: var(--muted); }
.body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 6px; }
.toprow { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-width: 0; }
.t { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14.5px; font-weight: 700; }
.meta { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12.5px; }
.biz { color: var(--ink-soft); font-weight: 600; }
</style>
