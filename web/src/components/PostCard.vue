<template>
  <component :is="post.url ? 'a' : 'div'" class="pcard" v-bind="linkAttrs">
    <span v-if="post.thumb" class="thumb"><AuthImg :src="post.thumb" alt="" /></span>
    <span v-else class="thumb empty" aria-hidden="true">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/>
      </svg>
    </span>
    <span class="body">
      <strong class="t">{{ post.title || '(제목 없음)' }}</strong>
      <span class="meta">
        <StatusChip kind="info">{{ channelLabel(post.channel) }}</StatusChip>
        <span class="muted small num">{{ statText }}</span>
      </span>
    </span>
    <span v-if="post.url" class="ext" aria-hidden="true">↗</span>
  </component>
</template>

<script setup>
import { computed } from 'vue'
import AuthImg from './AuthImg.vue'
import StatusChip from './StatusChip.vue'
import { label, PLATFORM } from '../stores/labels.js'

const props = defineProps({ post: { type: Object, required: true } })

const linkAttrs = computed(() => (props.post.url ? { href: props.post.url, target: '_blank', rel: 'noopener' } : {}))

function channelLabel(ch) {
  return label(PLATFORM, ch, ch)
}

const statText = computed(() => {
  const parts = []
  if (props.post.publishedAt) parts.push(props.post.publishedAt.slice(5, 10))
  if (props.post.views !== undefined && props.post.views !== null) parts.push(`조회 ${props.post.views}`)
  if (props.post.likes) parts.push(`공감 ${props.post.likes}`)
  if (props.post.comments) parts.push(`댓글 ${props.post.comments}`)
  return parts.join(' · ')
})
</script>

<style scoped>
.pcard {
  display: flex;
  align-items: center;
  gap: 12px;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--r-lg);
  padding: 12px 14px;
  text-decoration: none;
  color: var(--ink);
  box-shadow: var(--shadow);
  transition: border-color var(--t), box-shadow var(--t);
}
a.pcard:hover { border-color: var(--accent); box-shadow: var(--shadow-lg); }
.thumb { width: 52px; height: 52px; border-radius: var(--r-md); overflow: hidden; flex-shrink: 0; background: var(--soft); border: 1px solid var(--line-soft); }
.thumb img { width: 100%; height: 100%; object-fit: cover; }
.thumb.empty { display: flex; align-items: center; justify-content: center; color: var(--muted); }
.body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 5px; }
.t { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; font-weight: 700; }
.meta { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.ext { color: var(--muted); font-size: 15px; font-weight: 700; flex-shrink: 0; }
a.pcard:hover .ext { color: var(--accent); }
</style>
