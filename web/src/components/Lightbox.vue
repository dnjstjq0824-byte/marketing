<template>
  <div class="back" @click.self="$emit('close')">
    <div class="box" role="dialog" aria-label="미디어 보기">
      <div class="stage">
        <video v-if="isVideo" :src="viewUrl" controls autoplay muted playsinline />
        <img v-else-if="isImage" :src="viewUrl" :alt="alt" />
        <div v-else class="doc">
          <p class="art" aria-hidden="true">▤</p>
          <p class="muted">미리보기 없음 — 내려받아서 보세요.</p>
        </div>
        <button v-if="hasPrev" class="nav prev" @click="$emit('prev')" aria-label="이전">‹</button>
        <button v-if="hasNext" class="nav next" @click="$emit('next')" aria-label="다음">›</button>
        <button class="x" @click="$emit('close')" aria-label="닫기">×</button>
      </div>
      <div class="meta">
        <div>
          <strong>{{ kindLabel }}</strong>
          <p class="muted small">{{ detailLine }}</p>
          <p v-if="props.item.prompt" class="muted small">프롬프트: {{ props.item.prompt }}</p>
          <p v-if="props.item.accountName || props.item.contentTitle" class="muted small">{{ genLine }}</p>
          <p v-if="dlMsg" :class="dlOk ? 'muted small' : 'error'">{{ dlMsg }}</p>
        </div>
        <div class="btnrow">
          <button class="ghost" :disabled="downloading" @click="download">{{ downloading ? '받는 중…' : '내려받기' }}</button>
          <button v-if="canDelete" class="ghost danger" @click="$emit('delete', item.id)">삭제</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { auth } from '../stores/auth.js'
import { MEDIA_KIND } from '../stores/labels.js'

const props = defineProps({
  item: { type: Object, required: true },
  src: { type: String, default: '' },
  hasPrev: { type: Boolean, default: false },
  hasNext: { type: Boolean, default: false },
  canDelete: { type: Boolean, default: true },
})
const emit = defineEmits(['close', 'prev', 'next', 'delete'])
const downloading = ref(false)
const dlMsg = ref('')
const dlOk = ref(false)

function onKey(e) {
  if (e.key === 'Escape') emit('close')
  if (e.key === 'ArrowLeft') emit('prev')
  if (e.key === 'ArrowRight') emit('next')
}

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))

const fileUrl = computed(() => `/api/media/${props.item.id}/file`)
const viewUrl = computed(() => props.src || '')
const alt = computed(() => (props.item.kind === 'upload' ? '업로드 이미지' : '미디어'))
const isVideo = computed(() => (props.item.mime || '').startsWith('video/'))
const isImage = computed(() => {
  const m = props.item.mime || ''
  return m === '' || m.startsWith('image/')
})
const kindLabel = computed(() => MEDIA_KIND[props.item.kind] || props.item.kind)

const detailLine = computed(() => {
  const parts = []
  if (props.item.business) parts.push(props.item.business)
  if (props.item.createdAt) parts.push(props.item.createdAt.slice(0, 10))
  if (props.item.bytes) parts.push(fmtBytes(props.item.bytes))
  if (props.item.sha256) parts.push('#' + props.item.sha256.slice(0, 8))
  return parts.join(' · ')
})

const genLine = computed(() => {
  const parts = []
  if (props.item.accountName) parts.push(`AI ${props.item.accountName}`)
  if (props.item.contentTitle) parts.push(`게시물: ${props.item.contentTitle}`)
  else if (props.item.contentId) parts.push(`게시물 #${props.item.contentId}`)
  return parts.join(' · ')
})

function fmtBytes(n) {
  if (n < 1024) return `${n}B`
  if (n < 1048576) return `${(n / 1024).toFixed(1)}KB`
  if (n < 1073741824) return `${(n / 1048576).toFixed(1)}MB`
  return `${(n / 1073741824).toFixed(2)}GB`
}

function fileName(res) {
  const disp = res.headers.get('Content-Disposition') || ''
  const m = disp.match(/filename="?([^";]+)"?/)
  if (m) return m[1]
  if (props.item.download) return props.item.download
  return `somun-${props.item.id}`
}

async function download() {
  if (downloading.value) return
  downloading.value = true
  dlMsg.value = ''
  try {
    const headers = {}
    if (auth.token) headers['Authorization'] = `Bearer ${auth.token}`
    const res = await fetch(fileUrl.value, { headers })
    if (!res.ok) throw new Error(`request failed (${res.status})`)
    const buf = await res.arrayBuffer()
    const digest = await crypto.subtle.digest('SHA-256', buf)
    const hex = [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, '0')).join('')
    if (props.item.sha256 && hex !== props.item.sha256) {
      dlOk.value = false
      dlMsg.value = '해시가 다릅니다. 다시 받으세요.'
      return
    }
    const a = document.createElement('a')
    a.href = URL.createObjectURL(new Blob([buf]))
    a.download = fileName(res)
    a.click()
    setTimeout(() => URL.revokeObjectURL(a.href), 10000)
    dlOk.value = true
    dlMsg.value = `받았습니다 (${fmtBytes(buf.byteLength)}, 해시 일치)`
  } catch (e) {
    dlOk.value = false
    dlMsg.value = e.message || '받기 실패'
  } finally {
    downloading.value = false
  }
}
</script>

<style scoped>
.back {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75);
  z-index: 160;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--s4);
}
.box { width: min(880px, 100%); display: flex; flex-direction: column; gap: 0; }
.stage {
  position: relative;
  background: var(--media-bg);
  border-radius: var(--r-lg) var(--r-lg) 0 0;
  min-height: 240px;
  max-height: 70vh;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
.stage img, .stage video { max-width: 100%; max-height: 70vh; }
.doc { text-align: center; padding: var(--s6); }
.doc .art { font-size: 56px; color: var(--ink-soft); }
.nav, .x {
  position: absolute;
  background: rgba(255, 255, 255, 0.9);
  border: none;
  color: var(--ink);
  width: 44px;
  height: 44px;
  border-radius: 50%;
  font-size: 24px;
}
.prev { left: var(--s3); }
.next { right: var(--s3); }
.x { top: var(--s3); right: var(--s3); }
.meta {
  background: var(--panel);
  border-radius: 0 0 var(--r-lg) var(--r-lg);
  padding: var(--s4);
  display: flex;
  gap: var(--s4);
  align-items: flex-start;
}
.meta > div:first-child { flex: 1; min-width: 0; }
.btnrow { display: flex; gap: var(--s2); flex-shrink: 0; }
.btnrow button, .ghost {
  border-radius: var(--r-md);
  padding: var(--s2) var(--s4);
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
}
.ghost.danger { background: var(--warn); border-color: var(--warn); color: #fff; }
.error { color: var(--warn); }
</style>
