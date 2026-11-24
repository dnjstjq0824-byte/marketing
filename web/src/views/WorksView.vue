<template>
  <div class="works">
    <div class="page-head">
      <div>
        <h1 class="page-title">작업</h1>
        <p class="muted small">투입된 상품·주제별 AI 진행 현황과 발행 결과를 확인합니다.</p>
      </div>
      <div class="head-acts">
        <div class="view-seg" role="tablist" aria-label="보기 방식">
          <button role="tab" :aria-selected="view === 'list'" :class="{ on: view === 'list' }" @click="setView('list')">목록</button>
          <button role="tab" :aria-selected="view === 'cal'" :class="{ on: view === 'cal' }" @click="setView('cal')">달력</button>
        </div>
        <RouterLink to="/new" class="new-cta">＋ 새 작업</RouterLink>
      </div>
    </div>

    <template v-if="view === 'list'">
      <div class="toolbar card">
        <div class="tab-row" role="tablist" aria-label="상태 필터">
          <button v-for="t in tabs" :key="t.key" role="tab" :aria-selected="tab === t.key" :class="{ on: tab === t.key }" @click="setTab(t.key)">
            {{ t.label }}
          </button>
        </div>
        <div class="filters">
          <input v-model="q" type="search" placeholder="검색 (제목·URL·사업) — 단축키 /" aria-label="검색" @keyup.enter="reload" />
          <select v-if="businesses.length" v-model="biz" aria-label="사업 필터" @change="reload">
            <option value="">전체 사업</option>
            <option v-for="b in businesses" :key="b" :value="b">{{ b }}</option>
          </select>
          <select v-model="sort" aria-label="정렬">
            <option value="new">최신순</option>
            <option value="old">오래된순</option>
            <option value="title">제목순</option>
          </select>
          <button class="ghost" @click="reload">검색</button>
        </div>
        <div class="sub-row">
          <PeriodFilter v-model="period" />
          <span class="muted small num">총 <b>{{ filtered.length }}</b>건</span>
        </div>
      </div>

      <Skeleton v-if="loading && !pool.length" :rows="4" />
      <div v-else-if="loadError" class="card center">
        <p>작업 목록을 불러오지 못했습니다.</p>
        <button @click="reload">다시 시도</button>
      </div>
      <template v-else>
        <div v-if="pageItems.length" class="cards">
          <WorkCard v-for="w in pageItems" :key="w.id" :work="w" />
        </div>
        <EmptyState
          v-else
          :title="emptyTitle"
          hint="필터 조건을 바꾸거나 새 작업을 투입해 보세요"
          to="/new"
          actionLabel="새 작업 시작"
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
  { key: 'all', label: '전체' },
  { key: 'running', label: '진행 중' },
  { key: 'queued', label: '발행 대기' },
  { key: 'published', label: '발행됨' },
  { key: 'held', label: '멈춤' },
  { key: 'failed', label: '실패' },
]
const tabKeys = tabs.map((t) => t.key)

const tab = ref(tabKeys.includes(route.query.tab) ? route.query.tab : 'all')
const view = ref(route.query.view === 'cal' ? 'cal' : 'list')
function setView(key) {
  if (view.value === key) return
  view.value = key
  router.replace({ query: { ...route.query, view: key === 'cal' ? 'cal' : undefined } })
}

const q = ref(typeof route.query.q === 'string' ? route.query.q : '')
const biz = ref(typeof route.query.business === 'string' ? route.query.business : '')
const sort = ref('new')
const period = ref({ preset: '30d', from: '', to: '' })
const pool = ref([])
const teamBusinesses = ref([])
const page = ref(1)
const pageSize = 12
const loading = ref(true)
const loadError = ref(false)

const businesses = computed(() => {
  const s = new Set(teamBusinesses.value)
  for (const w of pool.value) if (w.business) s.add(w.business)
  return [...s].sort((a, b) => a.localeCompare(b, 'ko'))
})

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
const emptyTitle = computed(() => (pool.value.length ? '조건에 맞는 작업이 없습니다' : '아직 등록된 작업이 없습니다'))

async function fetchPool() {
  const out = []
  for (let p = 1; p <= 5; p++) {
    const params = new URLSearchParams({ tab: tab.value, page: String(p), size: '100' })
    if (q.value.trim()) params.set('q', q.value.trim())
    if (biz.value) params.set('business', biz.value)
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
  router.replace({ query: { ...route.query, tab: tab.value, q: q.value || undefined, business: biz.value || undefined } })
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

const poll = usePoll(load, { isBusy: () => (tab.value === 'running' || tab.value === 'all') && pool.value.length > 0 })
onMounted(async () => {
  poll.start()
  try {
    const tm = await apiFetch('/api/teams')
    const out = []
    for (const t of tm.items || []) for (const b of t.businesses || []) if (!out.includes(b)) out.push(b)
    teamBusinesses.value = out
  } catch { /* optional */ }
})
</script>

<style scoped>
.works { display: flex; flex-direction: column; gap: 18px; }
.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 14px; flex-wrap: wrap; }
.head-acts { display: flex; align-items: center; gap: 10px; }

.view-seg {
  display: inline-flex;
  background: var(--soft);
  padding: 3px;
  border-radius: 10px;
  gap: 2px;
}
.view-seg button {
  background: transparent;
  color: var(--muted);
  border: none;
  border-radius: 7px;
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 600;
}
.view-seg button.on { background: var(--panel); color: var(--ink); box-shadow: var(--shadow-sm); }

.new-cta {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: var(--accent);
  color: var(--accent-ink);
  text-decoration: none;
  font-weight: 700;
  font-size: 13.5px;
  padding: 8px 16px;
  border-radius: 10px;
  transition: background var(--t);
}
.new-cta:hover { background: var(--accent-dark); color: var(--accent-ink); }

.toolbar { display: flex; flex-direction: column; gap: 12px; padding: 16px 18px; }
.tab-row { display: flex; gap: 6px; flex-wrap: wrap; }
.tab-row button {
  background: var(--soft);
  color: var(--ink-soft);
  border: 1px solid transparent;
  border-radius: 999px;
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 600;
}
.tab-row button:hover { background: var(--surface-2); }
.tab-row button.on { background: var(--accent); color: var(--accent-ink); font-weight: 700; }

.filters { display: flex; gap: 8px; flex-wrap: wrap; }
.filters input { flex: 1; min-width: 180px; }
.filters select { width: auto; min-width: 120px; }

.sub-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; padding-top: 6px; border-top: 1px solid var(--line-soft); }
.center { text-align: center; display: flex; flex-direction: column; align-items: center; gap: 10px; }
.cards { display: grid; grid-template-columns: 1fr; gap: 12px; }
@media (min-width: 980px) {
  .cards { grid-template-columns: 1fr 1fr; }
}
@media (min-width: 1500px) {
  .cards { grid-template-columns: 1fr 1fr 1fr; }
}
</style>
