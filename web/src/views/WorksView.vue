<template>
  <div class="works">
    <h1 class="page-title">작업</h1>
    <div class="seg" role="tablist" aria-label="보기">
      <button role="tab" :aria-selected="view === 'list'" :class="{ on: view === 'list' }" @click="setView('list')">목록</button>
      <button role="tab" :aria-selected="view === 'cal'" :class="{ on: view === 'cal' }" @click="setView('cal')">달력</button>
    </div>
    <template v-if="view === 'list'">
    <div class="seg" role="tablist" aria-label="상태">
      <button v-for="t in tabs" :key="t.key" role="tab" :aria-selected="tab === t.key" :class="{ on: tab === t.key }" @click="setTab(t.key)">{{ t.label }}</button>
    </div>
    <PeriodFilter v-model="period" />
    <div class="filters">
      <input v-model="q" placeholder="검색 (제목·주소·사업)" aria-label="검색" @keyup.enter="reload" />
      <select v-model="sort" aria-label="정렬">
        <option value="new">최신순</option>
        <option value="old">오래된순</option>
        <option value="title">제목순</option>
      </select>
      <button class="ghost" @click="reload">찾기</button>
    </div>
    <p class="muted small count">전체 {{ filtered.length }}건</p>
    <Skeleton v-if="loading && !pool.length" :rows="4" />
    <div v-else-if="loadError" class="card center">
      <p>불러오지 못했습니다.</p>
      <button @click="reload">다시 시도</button>
    </div>
    <template v-else>
      <div v-if="pageItems.length" class="cards">
        <WorkCard v-for="w in pageItems" :key="w.id" :work="w" />
      </div>
      <EmptyState
        v-else :title="emptyTitle" hint="조건을 바꾸거나 새 작업을 만드세요"
        to="/new" actionLabel="새 작업 만들기"
      />
      <WorksPager :page="page" :pages="pages" @select="page = $event" />
    </template>
    </template>
    <WorksCalendar v-else />
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiFetch } from '../stores/auth.js'
import { usePoll } from '../stores/poll.js'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'
import WorkCard from '../components/WorkCard.vue'
import PeriodFilter from '../components/console/PeriodFilter.vue'
import WorksPager from '../components/console/WorksPager.vue'
import WorksCalendar from '../components/console/WorksCalendar.vue'

const route = useRoute()
const router = useRouter()
const tabs = [
  { key: 'running', label: '진행 중' },
  { key: 'queued', label: '발행 대기' },
  { key: 'published', label: '발행됨' },
  { key: 'held', label: '멈춤' },
  { key: 'failed', label: '실패' },
]
const tabKeys = tabs.map((t) => t.key)

const tab = ref(tabKeys.includes(route.query.tab) ? route.query.tab : 'running')
const view = ref(route.query.view === 'cal' ? 'cal' : 'list')
function setView(key) {
  if (view.value === key) return
  view.value = key
  router.replace({ query: { ...route.query, view: key === 'cal' ? 'cal' : undefined } })
}
const q = ref(typeof route.query.q === 'string' ? route.query.q : '')
const sort = ref('new')
const period = ref({ preset: '30d', from: '', to: '' })
const pool = ref([])
const page = ref(1)
const pageSize = 12
const loading = ref(true)
const loadError = ref(false)

function parseAgo(ago) {
  if (!ago) return new Date()
  const d = new Date(String(ago).endsWith('Z') ? ago : ago + 'Z')
  return Number.isNaN(d.getTime()) ? null : d
}

function inPeriod(w) {
  const d = parseAgo(w.ago)
  if (!d) return false
  const p = period.value
  if (p.preset === 'today') return d.toDateString() === new Date().toDateString()
  if (p.preset === '7d' || p.preset === '30d') {
    const days = p.preset === '7d' ? 7 : 30
    return Date.now() - d.getTime() <= days * 864e5
  }
  if (p.preset === 'custom' && p.from && p.to) {
    const day = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    return day >= p.from && day <= p.to
  }
  return true
}

const filtered = computed(() => {
  const list = pool.value.filter(inPeriod)
  const by = (w) => parseAgo(w.ago)?.getTime() || 0
  if (sort.value === 'old') list.sort((a, b) => by(a) - by(b))
  else if (sort.value === 'title') list.sort((a, b) => String(a.title).localeCompare(String(b.title), 'ko'))
  else list.sort((a, b) => by(b) - by(a))
  return list
})
const pages = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize)))
const pageItems = computed(() => filtered.value.slice((page.value - 1) * pageSize, page.value * pageSize))
const emptyTitle = computed(() => (pool.value.length ? '조건에 맞는 작업이 없습니다' : '작업이 없습니다'))

async function fetchPool() {
  const out = []
  for (let p = 1; p <= 5; p++) {
    const params = new URLSearchParams({ tab: tab.value, page: String(p), size: '100' })
    if (q.value.trim()) params.set('q', q.value.trim())
    const data = await apiFetch(`/api/works?${params.toString()}`)
    const got = data.items || []
    out.push(...got)
    if (got.length < 100) break
  }
  return out
}

async function load() {
  loadError.value = false
  try {
    pool.value = await fetchPool()
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

function reload() {
  page.value = 1
  router.replace({ query: { ...route.query, tab: tab.value, q: q.value || undefined } })
  load()
}

function setTab(key) {
  if (tab.value === key) return
  tab.value = key
  reload()
}

watch(() => route.query.tab, (v) => {
  if (typeof v === 'string' && tabKeys.includes(v) && v !== tab.value) {
    tab.value = v
    page.value = 1
    load()
  }
})
watch(() => route.query.view, (v) => {
  view.value = v === 'cal' ? 'cal' : 'list'
})
watch([() => period.value, sort], () => { page.value = 1 })
watch(page, () => window.scrollTo({ top: 0 }))

const poll = usePoll(load, { isBusy: () => tab.value === 'running' && pool.value.length > 0 })
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
.filters { display: flex; gap: var(--s2); flex-wrap: wrap; }
.filters input {
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: var(--s2) var(--s3);
  background: var(--panel);
  color: var(--ink);
  flex: 1;
  min-width: 160px;
}
.filters select {
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: var(--s2) var(--s3);
  background: var(--panel);
  color: var(--ink);
}
.count { margin: 0; }
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
@media (min-width: 1150px) {
  .cards { display: grid; grid-template-columns: 1fr 1fr; align-items: start; }
}
@media (min-width: 1700px) {
  .cards { grid-template-columns: 1fr 1fr 1fr; }
}
</style>
