<template>
  <div>
    <div class="drop" :class="{ over: dragOver }" @dragover.prevent="dragOver = true" @dragleave="dragOver = false"
      @drop.prevent="onDrop" @click="browse" @keydown.enter="browse" tabindex="0" role="button"
      aria-label="파일 첨부: 클릭하거나 끌어다 놓으세요">
      <p><strong>클릭하거나 끌어다 놓으세요</strong></p>
      <p class="muted small">사진 50MB · 영상 2GB · PDF 100MB</p>
      <input ref="fileInput" type="file" multiple class="hidden" :accept="accept" @change="onPick" />
    </div>
    <ul v-if="rows.length" class="queue">
      <li v-for="r in rows" :key="r.id" class="qrow">
        <span class="qname">{{ r.name }}</span>
        <span v-if="r.state === 'done'" class="qok">✓</span>
        <span v-else-if="r.state === 'error'" class="qerr">
          {{ r.error }} <button class="ghost small" @click.stop="retry(r.id)">다시 시도</button>
        </span>
        <span v-else-if="r.state === 'cancelled'" class="muted small">취소됨</span>
        <span v-else class="qprog"><span class="bar" :style="{ width: pct(r) + '%' }"></span></span>
        <button v-if="r.state !== 'done'" class="ghost small" @click.stop="dismiss(r.id)" aria-label="빼기">×</button>
      </li>
    </ul>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { cancelUpload, queueFiles, removeUpload, retryUpload, uploads } from '../stores/upload.js'

const fileInput = ref(null)
const dragOver = ref(false)
const accept = 'image/jpeg,image/png,image/webp,image/gif,video/mp4,video/webm,video/quicktime,application/pdf'

const rows = computed(() => uploads)

function browse() {
  fileInput.value.click()
}

function onPick(e) {
  if (e.target.files.length) queueFiles(e.target.files)
  e.target.value = ''
}

function onDrop(e) {
  dragOver.value = false
  if (e.dataTransfer.files.length) queueFiles(e.dataTransfer.files)
}

function pct(r) {
  return r.total ? Math.round((r.done / r.total) * 100) : 0
}

function dismiss(id) {
  cancelUpload(id)
  removeUpload(id)
}

function retry(id) {
  retryUpload(id)
}

defineExpose({ rows })
</script>

<style scoped>
.drop {
  border: 2px dashed var(--line);
  border-radius: var(--r-lg);
  padding: var(--s6) var(--s4);
  text-align: center;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: var(--s1);
}
.drop.over { border-color: var(--accent); background: var(--accent-soft); }
.hidden { display: none; }
.queue { list-style: none; margin: var(--s3) 0 0; padding: 0; display: flex; flex-direction: column; gap: var(--s2); }
.qrow { display: flex; align-items: center; gap: var(--s3); background: var(--panel); border: 1px solid var(--line-soft); border-radius: var(--r-md); padding: var(--s2) var(--s3); }
.qname { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.qok { color: var(--ok); font-weight: 700; }
.qerr { color: var(--warn); font-size: 13px; }
.qprog { flex: 1; height: 8px; background: var(--line-soft); border-radius: 4px; overflow: hidden; }
.qprog .bar { display: block; height: 100%; background: var(--accent); }
.ghost { border: 1px solid var(--line); background: var(--panel); color: var(--ink); border-radius: var(--r-md); }
.small { font-size: 13px; padding: var(--s1) var(--s2); }
</style>
