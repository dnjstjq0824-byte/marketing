<template>
  <div class="works">
    <h1 class="page-title">작업</h1>
    <div class="seg" role="tablist" aria-label="상태">
      <button v-for="t in tabs" :key="t.key" role="tab" :aria-selected="tab === t.key" :class="{ on: tab === t.key }" @click="setTab(t.key)">{{ t.label }}</button>
    </div>
    <div class="filters">
      <input v-model="q" placeholder="검색 (제목·주소·사업)" aria-label="검색" @keyup.enter="reload" />
      <input v-model="business" placeholder="사업 (비우면 전체)" aria-label="사업" @keyup.enter="reload" />
      <button class="ghost" @click="reload">찾기</button>
    </div>
    <Skeleton v-if="loading && !items.length" :rows="4" />
    <div v-else-if="loadError" class="card center">
      <p>불러오지 못했습니다.</p>
      <button @click="reload">다시 시도</button>
    </div>
    <template v-else>
      <div v-if="items.length" class="cards">
        <WorkCard v-for="w in items" :key="w.id" :work="w" />
      </div>
      <EmptyState v-else title="작업이 없습니다" hint="새 작업을 만들면 여기 뜹니다" to="/new" actionLabel="새 작업" />
      <div v-if="hasMore" class="center"><button class="ghost" @click="more" :disabled="loading">더 보기</button></div>
    </template>
  </div>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiFetch } from '../stores/auth.js'
import { usePoll } from '../stores/poll.js'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'
import WorkCard from '../components/WorkCard.vue'

const route = useRoute()
const router = useRouter()
const tabs = [
  { key: 'running', label: '진행 중' },
  { key: 'queued', label: '발행 대기' },
  { key: 'published', label: '발행됨' },
  { key: 'held', label: '멈춤' },
  { key: 'failed', label: '실패' },
]
const tab = ref(typeof route.query.tab === 'string' ? route.query.tab : 'running')
const q = ref(typeof route.query.q === 'string' ? route.query.q : '')
const business = ref('')
const items = ref([])
const page = ref(1)
const hasMore = ref(false)
const loading = ref(true)
const loadError = ref(false)

async function fetchPage(p) {
  const params = new URLSearchParams({ tab: tab.value, page: String(p), size: '20' })
  if (q.value.trim()) params.set('q', q.value.trim())
  if (business.value.trim()) params.set('business', business.value.trim())
  return apiFetch(`/api/works?${params.toString()}`)
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
      items.value = got
      page.value = 1
    }
    hasMore.value = got.length >= 20
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

function reload() {
  router.replace({ query: { ...route.query, tab: tab.value, q: q.value || undefined } })
  load(false)
}

function setTab(key) {
  if (tab.value === key) return
  tab.value = key
  reload()
}

async function more() {
  loading.value = true
  await load(true)
}

watch(() => route.query.tab, (v) => {
  if (typeof v === 'string' && v !== tab.value) {
    tab.value = v
    load(false)
  }
})

const poll = usePoll(() => load(false), { isBusy: () => tab.value === 'running' && items.value.length > 0 })
onMounted(() => poll.start())
</script>

<style scoped>
.works { display: flex; flex-direction: column; gap: var(--s4); }
.seg { display: flex; gap: var(--s2); flex-wrap: wrap; }
.seg button {
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
  border-radius: 999px;
  padding: var(--s2) var(--s4);
}
.seg button.on { background: var(--accent); border-color: var(--accent); color: #fff; font-weight: 700; }
.filters { display: flex; gap: var(--s2); }
.filters input {
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: var(--s2) var(--s3);
  background: var(--panel);
  flex: 1;
}
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
.cards { display: flex; flex-direction: column; gap: var(--s3); }
</style>
