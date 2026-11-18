<template>
  <div class="media">
    <div class="head sticky-head">
      <h1 class="page-title">미디어 보관함</h1>
      <span class="muted small ctx"><b class="num">{{ items.length }}</b>개<span v-if="selected.size"> · 선택 <b class="num">{{ selected.size }}</b></span></span>
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
    <div class="filters">
      <label>작업 <input v-model="product" class="narrow" inputmode="numeric" placeholder="번호" @change="applyFilters" /></label>
      <label>부터 <input v-model="from" type="date" @change="applyFilters" /></label>
      <label>까지 <input v-model="to" type="date" @change="applyFilters" /></label>
      <button v-if="product || from || to" class="ghost" @click="clearDates">지우기</button>
    </div>
    <FieldError :message="filterMsg" :hint="filterHint" />
    <Skeleton v-if="loading && !items.length" :rows="3" />
    <ErrorRetry v-else-if="loadError" hint="필터를 확인하고 다시 시도하세요. 계속되면 새로고침하세요." @retry="reload" />
    <div v-else-if="!items.length" class="emptywrap">
      <EmptyState title="아직 없습니다" :hint="hasFilter ? '조건에 맞는 파일이 없습니다' : '올리기로 첫 파일을 올리세요'" />
      <button v-if="!hasFilter" @click="showDrop = true">올리기</button>
      <button v-else class="ghost" @click="clearFilters">필터 지우기</button>
    </div>
    <div v-else class="tiles">
      <MediaTile v-for="m in items" :key="m.id" :item="tileOf(m)" :badge="badgeOf(m)" action="open" :selectMode="selectMode" :selected="selected.has(m.id)" @open="openBox(m.id)" @toggle="toggle" />
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
import ErrorRetry from '../components/console/ErrorRetry.vue'
import FieldError from '../components/console/FieldError.vue'

const tabs = [
  { key: '', label: '전체' },
  { key: 'upload', label: '업로드' },
  { key: 'gen_image', label: 'AI 이미지' },
  { key: 'gen_video', label: 'AI 영상' },
  { key: 'card', label: '카드뉴스' },
]
const kind = ref('')
const product = ref('')
const from = ref('')
const to = ref('')
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
const filterMsg = ref('')
const filterHint = ref('')
const boxIdx = computed(() => items.value.findIndex((m) => m.id === boxId.value))
const current = computed(() => items.value[boxIdx.value] || null)
const hasFilter = computed(() => kind.value !== '' || product.value !== '' || from.value !== '' || to.value !== '')

function tileOf(m) {
  return { id: m.id, kind: m.kind, thumb: `/api/media/${m.id}/thumb`, url: blobs.value[m.id] || '' }
}

function badgeOf(m) {
  if ((m.mime || '').startsWith('video/')) return '▶ 영상'
  if (m.kind === 'gen_image') return 'AI'
  if (m.kind === 'gen_video') return 'AI 영상'
  if (m.kind === 'card') return '카드'
  return ''
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



function revokeBlobs() {
  for (const url of Object.values(blobs.value)) URL.revokeObjectURL(url)
  blobs.value = {}
}

async function fetchPage(p) {
  const params = new URLSearchParams({ page: String(p), size: '48' })
  if (kind.value) params.set('kind', kind.value)
  if (product.value) params.set('product', product.value)
  if (from.value) params.set('from', from.value)
  if (to.value) params.set('to', to.value)
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

function applyFilters() {
  filterMsg.value = ''
  filterHint.value = ''
  if (product.value !== '' && !/^\d+$/.test(product.value.trim())) {
    filterMsg.value = '작업 번호는 숫자로 입력하세요'
    filterHint.value = '작업 화면 주소의 번호를 쓰세요'
    return
  }
  if (from.value && to.value && from.value > to.value) {
    filterMsg.value = '시작일이 종료일보다 늦습니다'
    filterHint.value = '부터·까지를 바꿔 보세요'
    return
  }
  load(false)
}

function clearDates() {
  product.value = ''
  from.value = ''
  to.value = ''
  filterMsg.value = ''
  filterHint.value = ''
  load(false)
}

function clearFilters() {
  kind.value = ''
  selected.value.clear()
  clearDates()
}

function reload() {
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
.media { display: flex; flex-direction: column; gap: var(--s4); min-width: 0; }
.head { display: flex; align-items: center; gap: var(--s3); flex-wrap: wrap; }
.sticky-head {
  position: sticky;
  top: 60px;
  z-index: 30;
  background: var(--bg);
  padding: var(--s2) 0;
}
.head h1 { white-space: nowrap; }
.ctx { flex: 1; min-width: 0; }
.btnrow { display: flex; gap: var(--s2); flex-wrap: wrap; }
.emptywrap { display: flex; flex-direction: column; align-items: center; }
.emptywrap button {
  border-radius: var(--r-md);
  padding: var(--s2) var(--s4);
  font-weight: 700;
  border: 1px solid var(--accent-dark);
  background: var(--accent);
  color: var(--accent-ink);
}
.emptywrap button:hover { background: var(--accent-dark); }
.emptywrap button.ghost { border-color: var(--line); background: var(--panel); color: var(--ink); font-weight: 500; }
.emptywrap button.ghost:hover { background: var(--soft); }
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
.seg button.on { background: var(--accent); border-color: var(--accent); color: var(--accent-ink); font-weight: 700; }
.seg button:not(.on):hover { background: var(--soft); }
.filters { display: flex; gap: var(--s3); align-items: center; flex-wrap: wrap; color: var(--muted); }
.filters label { min-width: 0; }
.filters input { border: 1px solid var(--line); border-radius: var(--r-md); padding: var(--s1) var(--s2); background: var(--panel); color: var(--ink); max-width: 100%; }
.filters input.narrow { width: 90px; }
.tiles { display: grid; grid-template-columns: repeat(6, 1fr); gap: var(--s2); min-width: 0; }
@media (max-width: 900px) {
  .sticky-head { top: 52px; }
  .tiles { grid-template-columns: repeat(3, 1fr); }
}
@media (max-width: 700px) {
  .tiles { grid-template-columns: repeat(2, 1fr); }
}
@media (min-width: 1600px) {
  .tiles { grid-template-columns: repeat(8, 1fr); }
}
</style>
