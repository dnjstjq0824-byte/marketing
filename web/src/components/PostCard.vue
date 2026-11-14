<template>
  <component :is="post.url ? 'a' : 'div'" class="card" v-bind="linkAttrs">
    <span v-if="post.thumb" class="thumb"><img :src="post.thumb" alt="" loading="lazy" /></span>
    <span class="body">
      <strong class="t">{{ post.title || '(제목 없음)' }}</strong>
      <span class="meta"><StatusChip kind="info">{{ channelLabel(post.channel) }}</StatusChip><span class="muted small">{{ statText }}</span></span>
    </span>
  </component>
</template>

<script setup>
import { computed } from 'vue'
import StatusChip from './StatusChip.vue'
import { label, PLATFORM } from '../stores/labels.js'

const props = defineProps({ post: { type: Object, required: true } })

const linkAttrs = computed(() => (props.post.url ? { href: props.post.url, target: '_blank', rel: 'noopener' } : {}))

function channelLabel(ch) {
  return label(PLATFORM, ch, ch)
}

const statText = computed(() => {
  const parts = []
  if (props.post.publishedAt) parts.push(props.post.publishedAt.slice(0, 10))
  if (props.post.views !== undefined && props.post.views !== null) parts.push(`조회 ${props.post.views}`)
  return parts.join(' · ')
})
</script>

<style scoped>
.card {
  display: flex;
  gap: var(--s3);
  background: var(--panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-lg);
  padding: var(--s3);
  text-decoration: none;
  color: var(--ink);
  box-shadow: var(--shadow);
}
.thumb { width: 56px; height: 56px; border-radius: var(--r-md); overflow: hidden; flex-shrink: 0; background: var(--soft); }
.thumb img { width: 100%; height: 100%; object-fit: cover; }
.body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: var(--s1); }
.t { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; }
.meta { display: flex; align-items: center; gap: var(--s2); }
</style>
