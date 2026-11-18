<template>
  <RouterLink v-if="action === 'link'" class="tile" to="/media" :aria-label="alt">
    <AuthImg :src="thumb" :alt="alt" />
    <span v-if="badge" class="badge">{{ badge }}</span>
  </RouterLink>
  <button v-else class="tile" :class="{ selected }" :aria-label="alt" :aria-pressed="selectMode ? selected : undefined" @click="onClick">
    <AuthImg :src="thumb" :alt="alt" />
    <span v-if="selectMode" class="box" :class="{ on: selected }" aria-hidden="true">{{ selected ? '✓' : '' }}</span>
    <span v-if="badge" class="badge">{{ badge }}</span>
  </button>
</template>

<script setup>
import { computed } from 'vue'
import AuthImg from './AuthImg.vue'

const props = defineProps({
  item: { type: Object, required: true },
  action: { type: String, default: 'link' },
  selectMode: { type: Boolean, default: false },
  selected: { type: Boolean, default: false },
  badge: { type: String, default: '' },
})
const emit = defineEmits(['open', 'toggle'])

const thumb = computed(() => props.item.thumb || props.item.url || '')
const alt = computed(() => (props.item.kind === 'upload' ? '업로드 이미지' : '미디어'))

function onClick() {
  if (props.selectMode) emit('toggle', props.item.id)
  else emit('open', props.item.id)
}
</script>

<style scoped>
.tile {
  display: block;
  aspect-ratio: 1;
  border-radius: var(--r-md);
  overflow: hidden;
  background: var(--surface-2);
  border: 1px solid var(--line-soft);
  padding: 0;
  position: relative;
  width: 100%;
  min-width: 0;
  transition: border-color var(--t), box-shadow var(--t);
}
button.tile { cursor: pointer; color: var(--ink); }
button.tile:hover { border-color: var(--accent); box-shadow: var(--shadow); }
button.tile:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
button.tile.selected { outline: 3px solid var(--accent); }
a.tile:hover { border-color: var(--accent); }
.tile img { width: 100%; height: 100%; object-fit: cover; display: block; }
.box {
  position: absolute;
  top: var(--s1);
  left: var(--s1);
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid var(--line);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
}
.box.on { background: var(--accent); border-color: var(--accent); color: var(--accent-ink); }
.badge {
  position: absolute;
  bottom: var(--s1);
  left: var(--s1);
  background: rgba(0, 0, 0, 0.65);
  color: #fff;
  font-size: 11px;
  border-radius: 4px;
  padding: 0 6px;
  max-width: calc(100% - 8px);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
