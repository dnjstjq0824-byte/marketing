<template>
  <div class="works">
    <h1 class="page-title">작업</h1>
    <div class="seg" role="tablist" aria-label="상태">
      <button v-for="t in tabs" :key="t.key" role="tab" :aria-selected="tab === t.key" :class="{ on: tab === t.key }" @click="setTab(t.key)">{{ t.label }}</button>
    </div>
    <div class="seg small" role="tablist" aria-label="보기">
      <button v-for="v in views" :key="v.key" role="tab" :aria-selected="view === v.key" :class="{ on: view === v.key }" @click="setView(v.key)">{{ v.label }}</button>
    </div>
    <div v-if="view === 'cal'" class="calwrap">
      <div class="calhead">
        <button class="ghost" @click="moveMonth(-1)" aria-label="이전 달">‹</button>
        <strong>{{ calYear }}년 {{ calMonth + 1 }}월</strong>
        <button class="ghost" @click="moveMonth(1)" aria-label="다음 달">›</button>
        <span class="sp"></span>
        <span class="legend"><i class="dot sched"></i>예약 <i class="dot done"></i>발행</span>
        <button v-if="unsched" class="ghost small" @click="setTab('queued'); setView('cards')">일정 없는 대기 {{ unsched }}건</button>
      </div>
      <p v-if="calError" class="error">달력을 불러오지 못했습니다. <button class="ghost small" @click="loadCalendar">다시 시도</button></p>
      <div v-else class="cal" role="grid" aria-label="발행 달력">
        <span v-for="d in ['일', '월', '화', '수', '목', '금', '토']" :key="d" class="dow">{{ d }}</span>
        <button v-for="c in calCells" :key="c.key" role="gridcell" class="day" :class="{ out: !c.inMonth, today: c.today, sel: c.key === selDay }"
          :aria-selected="c.key === selDay" @click="selDay = c.key">
          <span class="dnum">{{ c.num }}</span>
          <span class="dots" aria-hidden="true">
            <i v-for="(k, i) in dayDots(c.key).slice(0, 3)" :key="i" class="dot" :class="k"></i>
            <em v-if="dayDots(c.key).length > 3">+{{ dayDots(c.key).length - 3 }}</em>
          </span>
        </button>
      </div>
      <div v-if="selDay" class="card daylist">
        <h3 class="section-title">{{ selDayLabel }}</h3>
        <p v-if="dayLoading" class="muted small">불러오는 중…</p>
        <ul v-else-if="dayItems.length" class="rows">
          <li v-for="it in dayItems" :key="it.key">
            <span class="dot" :class="it.kind" aria-hidden="true"></span>
            <RouterLink v-if="it.to" :to="it.to" class="t">{{ it.title }}</RouterLink>
            <span v-else class="t">{{ it.title }}</span>
            <span class="muted small">{{ it.meta }}</span>
          </li>
        </ul>
        <p v-else class="muted small">이날 발행 일정이 없습니다.</p>
      </div>
    </div>
    <template v-else>
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
        <div v-if="items.length && view === 'cards'" class="cards">
          <WorkCard v-for="w in items" :key="w.id" :work="w" />
        </div>
        <ul v-else-if="items.length" class="denselist" aria-label="밀도 목록">
          <li v-for="w in items" :key="w.id">
            <RouterLink :to="'/works/' + w.id" class="t">{{ w.title }}</RouterLink>
            <span class="muted small">{{ w.business || '공용' }}</span>
            <span class="muted small">{{ w.detail || w.stage || w.state }}</span>
            <span class="muted small num">{{ agoText(w.ago) }}</span>
          </li>
        </ul>
        <EmptyState v-else title="작업이 없습니다" hint="새 작업을 만들면 여기 뜹니다" to="/new" actionLabel="새 작업" />
        <div v-if="hasMore" class="center"><button class="ghost" @click="more" :disabled="loading">더 보기</button></div>
      </template>
    </template>
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

const route = useRoute()
const router = useRouter()
const tabs = [
  { key: 'running', label: '진행 중' },
  { key: 'queued', label: '발행 대기' },
  { key: 'published', label: '발행됨' },
  { key: 'held', label: '멈춤' },
  { key: 'failed', label: '실패' },
]

function agoText(ago) {
  if (!ago) return '방금'
  const d = new Date(String(ago).endsWith('Z') ? ago : ago + 'Z')
  if (Number.isNaN(d.getTime())) return '방금'
  const mins = Math.max(0, Math.round((Date.now() - d.getTime()) / 60000))
  if (mins < 1) return '방금'
  if (mins < 60) return `${mins}분 전`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}시간 전`
  return `${Math.floor(hours / 24)}일 전`
}
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
onMounted(() => {
  poll.start()
  if (view.value === 'cal') loadCalendar()
})

const views = [
  { key: 'cards', label: '카드' },
  { key: 'dense', label: '밀도' },
  { key: 'cal', label: '캘린더' },
]
const view = ref(['cards', 'dense', 'cal'].includes(route.query.view) ? route.query.view : 'cards')

function setView(key) {
  view.value = key
  router.replace({ query: { ...route.query, view: key } })
  if (key === 'cal' && !calLoaded.value) loadCalendar()
}

const calYear = ref(new Date().getFullYear())
const calMonth = ref(new Date().getMonth())
const calItems = ref([])
const calLoaded = ref(false)
const calError = ref(false)
const unsched = ref(0)
const selDay = ref('')
const dayItems = ref([])
const dayLoading = ref(false)

function dayKey(d) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

const calCells = computed(() => {
  const first = new Date(calYear.value, calMonth.value, 1)
  const start = new Date(first)
  start.setDate(start.getDate() - start.getDay())
  const today = dayKey(new Date())
  const cells = []
  for (let i = 0; i < 42; i++) {
    const d = new Date(start)
    d.setDate(start.getDate() + i)
    const key = dayKey(d)
    cells.push({ key, num: d.getDate(), inMonth: d.getMonth() === calMonth.value, today: key === today })
  }
  return cells
})

const dayDots = (key) => calItems.value.filter((it) => it.day === key).map((it) => it.kind)
const selDayLabel = computed(() => {
  if (!selDay.value) return ''
  const [y, m, d] = selDay.value.split('-').map(Number)
  return `${y}년 ${m}월 ${d}일`
})

function moveMonth(delta) {
  const d = new Date(calYear.value, calMonth.value + delta, 1)
  calYear.value = d.getFullYear()
  calMonth.value = d.getMonth()
  selDay.value = ''
  dayItems.value = []
}

async function loadCalendar() {
  calError.value = false
  try {
    const [qc, logs] = await Promise.all([
      apiFetch('/api/contents?status=queued'),
      apiFetch('/api/publish-logs'),
    ])
    const items = []
    let lost = 0
    for (const c of qc.contents || []) {
      if (!c.scheduledAt) { lost++; continue }
      const when = new Date(c.scheduledAt)
      if (Number.isNaN(when.getTime())) { lost++; continue }
      items.push({ day: dayKey(when), kind: 'sched', at: when, contentId: c.id, title: c.title || '(제목 없음)', meta: `${c.channel || ''} · ${c.targetAccount || ''}`.trim(), productId: c.productId })
    }
    for (const l of logs.logs || []) {
      if (l.status !== 'done' || !l.executedAt) continue
      const when = new Date(l.executedAt)
      if (Number.isNaN(when.getTime())) continue
      items.push({ day: dayKey(when), kind: 'done', at: when, contentId: l.contentId, title: '', meta: `${l.channel || ''} · ${l.account || ''}`.trim(), productId: 0 })
    }
    calItems.value = items
    unsched.value = lost
    calLoaded.value = true
  } catch {
    calError.value = true
  }
}

watch(selDay, async (key) => {
  dayItems.value = []
  if (!key) return
  dayLoading.value = true
  const rows = calItems.value.filter((it) => it.day === key).sort((a, b) => a.at - b.at).slice(0, 20)
  const out = []
  for (const r of rows) {
    if (selDay.value !== key) return
    let title = r.title
    let to = r.productId ? `/works/${r.productId}` : ''
    if (!title && r.contentId) {
      try {
        const c = await apiFetch(`/api/contents/${r.contentId}`)
        title = c.title || '(제목 없음)'
        if (c.productId) to = `/works/${c.productId}`
      } catch {
        title = '(제목 없음)'
      }
    }
    const hh = String(r.at.getHours()).padStart(2, '0')
    const mm = String(r.at.getMinutes()).padStart(2, '0')
    out.push({ key: `${r.kind}-${r.contentId}-${r.at.getTime()}`, kind: r.kind, title, meta: `${hh}:${mm} ${r.meta}`.trim(), to })
  }
  if (selDay.value !== key) return
  dayItems.value = out
  dayLoading.value = false
})

watch(() => route.query.view, (v) => {
  if (typeof v === 'string' && ['cards', 'dense', 'cal'].includes(v) && v !== view.value) setView(v)
})
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
.seg.small button { padding: var(--s1) var(--s3); font-size: 13px; }
.calwrap { display: flex; flex-direction: column; gap: var(--s3); }
.calhead { display: flex; align-items: center; gap: var(--s2); }
.calhead strong { font-size: 17px; }
.sp { flex: 1; }
.legend { display: flex; align-items: center; gap: var(--s1); font-size: 13px; color: var(--muted); }
.dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
.dot.sched { background: var(--accent); }
.dot.done { background: var(--ok); }
.cal { display: grid; grid-template-columns: repeat(7, 1fr); gap: 4px; }
.dow { text-align: center; font-size: 12px; color: var(--muted); padding: var(--s1) 0; }
.day {
  display: flex; flex-direction: column; align-items: center; gap: 2px;
  min-height: 52px; padding: var(--s1);
  background: var(--panel); border: 1px solid var(--line-soft); border-radius: var(--r-md);
  color: var(--ink); margin: 0; box-shadow: none; font-weight: 400;
}
.day:hover { background: var(--soft); border-color: var(--line); }
.day.out { opacity: 0.45; }
.day.today .dnum { font-weight: 800; color: var(--accent-dark); }
.day.sel { border-color: var(--accent); box-shadow: 0 0 0 1px var(--accent); }
.dnum { font-size: 13px; }
.dots { display: flex; gap: 2px; align-items: center; min-height: 10px; }
.dots em { font-style: normal; font-size: 10px; color: var(--muted); }
.daylist ul.rows, .denselist { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
.daylist ul.rows li { display: flex; align-items: center; gap: var(--s2); padding: var(--s2) 0; border-bottom: 1px solid var(--line-soft); }
.daylist ul.rows li:last-child { border-bottom: none; }
.daylist .t, .denselist .t { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; text-decoration: none; color: var(--ink); font-weight: 600; }
.daylist .t:hover, .denselist .t:hover { text-decoration: underline; }
.denselist { background: var(--panel); border: 1px solid var(--line-soft); border-radius: var(--r-lg); box-shadow: var(--shadow); }
.denselist li { display: flex; align-items: center; gap: var(--s3); padding: var(--s2) var(--s4); border-bottom: 1px solid var(--line-soft); }
.denselist li:last-child { border-bottom: none; }
@media (max-width: 900px) {
  .day { min-height: 44px; }
  .denselist li { flex-wrap: wrap; gap: var(--s1) var(--s3); }
  .denselist .t { flex-basis: 100%; }
}
.filters { display: flex; gap: var(--s2); flex-wrap: wrap; }
.filters input {
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: var(--s2) var(--s3);
  background: var(--panel);
  flex: 1;
  min-width: 160px;
}
.denselist .num { margin-left: auto; font-variant-numeric: tabular-nums; white-space: nowrap; }
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
