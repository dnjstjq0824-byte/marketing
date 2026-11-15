<template>
  <div class="media">
    <div class="head">
      <h1 class="page-title">미디어 보관함</h1>
      <div class="btnrow">
        <button v-if="!selectMode" class="ghost" @click="showDrop = !showDrop">올리기</button>
        <button v-if="!selectMode" class="ghost" @click="selectMode = true">선택</button>
        <template v-else>
          <button class="ghost" :disabled="!selected.size" @click="zipSel">묶어 받기 ({{ selected.size }})</button>
          <button class="ghost" @click="selectMode = false; selected.clear()">취소</button>
        </template>
      </div>
    </div>
    <Dropzone v-if="showDrop" />
    <div class="seg" role="tablist" aria-label="종류">
      <button v-for="t in tabs" :key="t.key" role="tab" :aria-selected="kind === t.key" :class="{ on: kind === t.key }" @click="setKind(t.key)">{{ t.label }}</button>
    </div>
    <Skeleton v-if="loading && !items.length" :rows="3" />
    <div v-else-if="loadError" class="card center">
      <p>불러오지 못했습니다.</p>
      <button @click="load">다시 시도</button>
    </div>
    <EmptyState v-else-if="!items.length" title="아직 없습니다" :hint="kind === '' ? '올리기로 첫 파일을 올리세요' : '이 종류는 아직 비어 있습니다'" />
    <div v-else class="tiles">
      <MediaTile v-for="m in items" :key="m.id" :item="tileOf(m)" action="open" :selectMode="selectMode" :selected="selected.has(m.id)" @open="openBox(m.id)" @toggle="toggle" />
    </div>
    <div v-if="hasMore" class="center"><button class="ghost" @click="more" :disabled="loading">더 보기</button></div>
    <Lightbox v-if="boxId !== null && current" :item="current" :src="blobs[boxId] || ''" :hasPrev="boxIdx > 0" :hasNext="boxIdx < items.length - 1"
      @close="boxId = null" @prev="step(-1)" @next="step(1)" @delete="remove" />
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { apiFetch, auth } from '../stores/auth.js'
import { toast } from '../stores/toast.js'
import { uploads } from '../stores/upload.js'
import Dropzone from '../components/Dropzone.vue'
import EmptyState from '../components/EmptyState.vue'
import Lightbox from '../components/Lightbox.vue'
import MediaTile from '../components/MediaTile.vue'
import Skeleton from '../components/Skeleton.vue'

const tabs = [
  { key: '', label: '전체' },
  { key: 'upload', label: '업로드' },
  { key: 'gen_image', label: 'AI 이미지' },
  { key: 'gen_video', label: 'AI 영상' },
  { key: 'card', label: '카드뉴스' },
]
const kind = ref('')
const items = ref([])
const page = ref(1)
const hasMore = ref(false)
const loading = ref(true)
const loadError = ref(false)
const showDrop = ref(false)
const selectMode = ref(false)
const selected = ref(new Set())
const boxId = ref(null)
const blobs = ref({})
const boxIdx = computed(() => items.value.findIndex((m) => m.id === boxId.value))
const current = computed(() => items.value[boxIdx.value] || null)

function tileOf(m) {
  return { id: m.id, kind: m.kind, thumb: blobs.value[m.id] || '', url: blobs.value[m.id] || '' }
}

async function fetchBlob(id) {
  if (blobs.value[id]) return blobs.value[id]
  const headers = {}
  if (auth.token) headers['Authorization'] = `Bearer ${auth.token}`
  const res = await fetch(`/api/media/${id}/file`, { headers })
  if (!res.ok) throw new Error(`request failed (${res.status})`)
  const url = URL.createObjectURL(await res.blob())
  blobs.value[id] = url
  return url
}

async function primeBlobs(list) {
  await Promise.all(list.map((m) => fetchBlob(m.id).catch(() => '')))
}

function revokeBlobs() {
  for (const url of Object.values(blobs.value)) URL.revokeObjectURL(url)
  blobs.value = {}
}

async function fetchPage(p) {
  const params = new URLSearchParams({ page: String(p), size: '48' })
  if (kind.value) params.set('kind', kind.value)
  return apiFetch(`/api/media?${params.toString()}`)
}

async function load(append) {
  loadError.value = false
  try {
    const data = await fetchPage(append ? page.value + 1 : 1)
    const got = data.items || []
    if (append) {
      items.value = items.value.concat(got)
      page.value += 1
    } else {
      revokeBlobs()
      items.value = got
      page.value = 1
    }
    hasMore.value = got.length >= 48
    await primeBlobs(got)
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

function setKind(key) {
  if (kind.value === key) return
  kind.value = key
  selected.value.clear()
  load(false)
}

async function more() {
  loading.value = true
  await load(true)
}

async function openBox(id) {
  fetchBlob(id).catch(() => '')
  boxId.value = id
  try {
    const d = await apiFetch(`/api/media/${id}`)
    const i = items.value.findIndex((m) => m.id === id)
    if (i >= 0) items.value[i] = { ...items.value[i], ...d }
  } catch {
    /* list row already has the essentials */
  }
}

function step(d) {
  const i = items.value.findIndex((m) => m.id === boxId.value) + d
  if (i >= 0 && i < items.value.length) {
    boxId.value = items.value[i].id
    fetchBlob(boxId.value).catch(() => '')
  }
}

function toggle(id) {
  if (selected.value.has(id)) selected.value.delete(id)
  else selected.value.add(id)
}

async function zipSel() {
  if (!selected.value.size) return
  try {
    const headers = { 'Content-Type': 'application/json' }
    if (auth.token) headers['Authorization'] = `Bearer ${auth.token}`
    const res = await fetch('/api/media/zip', { method: 'POST', headers, body: JSON.stringify({ ids: [...selected.value] }) })
    if (!res.ok) throw new Error(`request failed (${res.status})`)
    const a = document.createElement('a')
    a.href = URL.createObjectURL(await res.blob())
    a.download = 'somun-media.zip'
    a.click()
    setTimeout(() => URL.revokeObjectURL(a.href), 10000)
    toast(`${selected.value.size}개를 묶어 받았습니다`)
    selected.value.clear()
    selectMode.value = false
  } catch (e) {
    toast(e.message || '받기 실패', 'error')
  }
}

async function remove(id) {
  try {
    await apiFetch(`/api/media/${id}`, { method: 'DELETE' })
    toast('삭제했습니다')
    boxId.value = null
    await load(false)
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

const seenDone = ref(0)
watch(uploads, () => {
  const done = uploads.filter((r) => r.state === 'done').length
  if (done > seenDone.value) {
    seenDone.value = done
    load(false)
  }
}, { deep: true })

onMounted(() => load(false))
onUnmounted(revokeBlobs)
</script>

<style scoped>
.media { display: flex; flex-direction: column; gap: var(--s4); }
.head { display: flex; align-items: center; gap: var(--s3); }
.head h1 { flex: 1; }
.btnrow { display: flex; gap: var(--s2); }
.card {
  background: var(--panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-lg);
  padding: var(--s4);
  box-shadow: var(--shadow);
  display: flex;
  flex-direction: column;
  gap: var(--s2);
}
.center { text-align: center; }
.center button, .ghost {
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
  border-radius: var(--r-md);
  padding: var(--s2) var(--s4);
}
.seg { display: flex; gap: var(--s2); flex-wrap: wrap; }
.seg button {
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
  border-radius: 999px;
  padding: var(--s2) var(--s4);
}
.seg button.on { background: var(--accent); border-color: var(--accent); color: #fff; font-weight: 700; }
.tiles { display: grid; grid-template-columns: repeat(6, 1fr); gap: var(--s2); }
@media (max-width: 900px) {
  .tiles { grid-template-columns: repeat(3, 1fr); }
}
@media (min-width: 1600px) {
  .tiles { grid-template-columns: repeat(8, 1fr); }
}
</style>
