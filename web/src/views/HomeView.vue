<template>
  <div class="home">
    <!-- Urgent Action Banner (top priority, hidden when calm) -->
    <section v-if="needs.length" class="action-banner" role="alert" aria-label="조치 필요">
      <div class="ab-head">
        <span class="ab-badge">조치 필요 {{ needs.length }}건</span>
        <span class="small muted">자동화가 멈추지 않도록 아래 항목을 확인해 주세요</span>
      </div>
      <ul class="ab-list">
        <li v-for="(n, i) in needs" :key="i">
          <span class="ab-dot" aria-hidden="true"></span>
          <span class="ab-msg">{{ n.text }}</span>
          <RouterLink :to="n.to" class="ab-btn">{{ n.action }}</RouterLink>
        </li>
      </ul>
    </section>

    <!-- Quick Launch Command Bar -->
    <section class="launch-bar" aria-label="빠른 작업 시작">
      <div class="lb-copy">
        <strong class="lb-title">무엇을 마케팅할까요?</strong>
        <span class="muted small">상품 링크나 주제를 붙여넣으면 AI 팀이 분석부터 발행까지 자동으로 끝냅니다.</span>
      </div>
      <form class="lb-form" @submit.prevent="quickLaunch">
        <input
          v-model.trim="quickInput"
          type="text"
          placeholder="https://smartstore.naver.com/... 또는 마케팅 주제 입력"
          aria-label="상품 링크 또는 주제 빠른 입력"
        />
        <button type="submit">AI에게 맡기기 →</button>
      </form>
    </section>

    <!-- First-run Onboarding Checklist -->
    <section v-if="showChecklist" class="card check" aria-label="시작 체크리스트">
      <div class="sect-head">
        <div>
          <h2 class="section-title">시작하기</h2>
          <p class="muted small">3단계만 연결하면 AI 마케팅 엔진이 24시간 자동으로 돌아갑니다.</p>
        </div>
        <span class="prog-pill num">{{ checkDone }} / 3 완료</span>
      </div>
      <div class="prog-track" aria-hidden="true"><div class="prog-fill" :style="{ width: (checkDone / 3) * 100 + '%' }"></div></div>
      <ol class="check-list">
        <li>
          <span class="step-num" :class="{ ok: data.firstRun.aiConnected }">{{ data.firstRun.aiConnected ? '✓' : '1' }}</span>
          <RouterLink to="/connect">AI 에이전트 계정 연결하기</RouterLink>
          <StatusChip :kind="data.firstRun.aiConnected ? 'ok' : 'hold'">{{ data.firstRun.aiConnected ? '연결됨' : '연결 필요' }}</StatusChip>
        </li>
        <li>
          <span class="step-num" :class="{ ok: data.firstRun.channelConnected }">{{ data.firstRun.channelConnected ? '✓' : '2' }}</span>
          <RouterLink to="/connect">발행할 블로그·SNS 채널 연결하기</RouterLink>
          <StatusChip :kind="data.firstRun.channelConnected ? 'ok' : 'hold'">{{ data.firstRun.channelConnected ? '연결됨' : '연결 필요' }}</StatusChip>
        </li>
        <li>
          <span class="step-num" :class="{ ok: data.firstRun.hasWork }">{{ data.firstRun.hasWork ? '✓' : '3' }}</span>
          <RouterLink to="/new">첫 마케팅 작업 투입하기</RouterLink>
          <StatusChip :kind="data.firstRun.hasWork ? 'ok' : 'hold'">{{ data.firstRun.hasWork ? '완료' : '대기' }}</StatusChip>
        </li>
      </ol>
    </section>

    <!-- Live Teams Status Strip -->
    <section v-if="teams.length" class="teams-strip" aria-label="가동 중인 AI 팀">
      <span class="ts-label muted small">AI 팀</span>
      <div class="ts-chips">
        <RouterLink v-for="t in teams" :key="t.team.id" :to="'/teams/' + t.team.id" class="tchip" :class="{ off: !t.team.enabled }">
          <span class="tdot" :class="{ on: t.team.enabled && t.memberCount > 0 }" aria-hidden="true"></span>
          <strong>{{ t.team.name }}</strong>
          <span v-if="t.businesses && t.businesses.length" class="muted micro">{{ t.businesses[0] }}</span>
          <span class="tcount num">{{ t.memberCount }}기</span>
        </RouterLink>
        <RouterLink to="/teams" class="tchip add">＋ 팀 관리</RouterLink>
      </div>
    </section>

    <Skeleton v-if="loading && !data.today" :rows="4" />
    <div v-else-if="loadError" class="card center">
      <p>현황을 불러오지 못했습니다.</p>
      <button @click="load">다시 시도</button>
    </div>

    <template v-else-if="data.today">
      <!-- 4-Column KPI Strip -->
      <section class="kpis" aria-label="오늘 현황">
        <KpiCard label="진행 중" :value="data.today.running" unit="건" tone="accent" :delta="null" to="/works?tab=running" />
        <KpiCard label="오늘 발행" :value="data.today.publishedToday" unit="건" tone="ok" :delta="pubDelta" to="/works?tab=published" />
        <KpiCard label="발행 대기" :value="data.today.queued" unit="건" tone="hold" :delta="null" to="/works?tab=queued" />
        <KpiCard label="실패 · 확인 필요" :value="data.today.failed" unit="건" :tone="data.today.failed > 0 ? 'warn' : ''" :delta="failDelta" to="/works?tab=failed" invert />
      </section>

      <!-- Publishing Trend Chart + Weekly Views -->
      <section class="card" aria-label="발행 추이">
        <div class="sect-head wrap-head">
          <div>
            <h2 class="section-title">발행 추이</h2>
            <span class="muted small">이번 주 누적 조회 <b class="num views-val">{{ (data.today.weekViews || 0).toLocaleString() }}</b></span>
          </div>
          <div class="period-seg" role="tablist" aria-label="기간">
            <button v-for="d in [7, 14, 30]" :key="d" role="tab" :aria-selected="trendDays === d" :class="{ on: trendDays === d }" @click="setTrendDays(d)">{{ d }}일</button>
          </div>
        </div>
        <TrendChart :days="trend" />
      </section>

      <!-- Running Works + Recent Published Posts -->
      <div class="main-grid">
        <section class="col-sec" aria-label="진행 중인 작업">
          <div class="sect-head">
            <h2 class="section-title">진행 중인 작업 <span class="muted small num">({{ data.running.length }})</span></h2>
            <RouterLink to="/works?tab=running" class="more">전체 보기 →</RouterLink>
          </div>
          <div v-if="data.running.length" class="stack">
            <WorkCard v-for="w in data.running" :key="w.id" :work="w" />
          </div>
          <EmptyState v-else title="지금 AI가 쉬고 있습니다" hint="상품 링크나 주제를 넣으면 바로 시작합니다" to="/new" actionLabel="새 작업 시작" />
        </section>

        <section class="col-sec" aria-label="최근 발행">
          <div class="sect-head">
            <h2 class="section-title">최근 발행 <span class="muted small num">({{ data.recentPosts.length }})</span></h2>
            <RouterLink to="/works?tab=published" class="more">전체 보기 →</RouterLink>
          </div>
          <div v-if="data.recentPosts.length" class="stack">
            <PostCard v-for="p in data.recentPosts" :key="p.id" :post="p" />
          </div>
          <EmptyState v-else title="아직 발행된 게시물이 없습니다" hint="승인된 게시물이 예약 시각에 자동 발행됩니다" to="/works?tab=queued" actionLabel="발행 대기 보기" />
        </section>
      </div>

      <!-- Recent Media -->
      <section v-if="data.recentMedia.length" class="col-sec" aria-label="최근 만든 미디어">
        <div class="sect-head">
          <h2 class="section-title">최근 미디어</h2>
          <RouterLink to="/media" class="more">보관함 전체 →</RouterLink>
        </div>
        <div class="tiles">
          <MediaTile v-for="m in data.recentMedia" :key="m.id" :item="m" action="open" @open="openMedia(m)" />
        </div>
      </section>
    </template>

    <Lightbox
      v-if="boxItem"
      :item="boxItem"
      :src="boxSrc"
      :hasPrev="false"
      :hasNext="false"
      @close="closeBox"
    />
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch, auth } from '../stores/auth.js'
import { usePoll } from '../stores/poll.js'
import EmptyState from '../components/EmptyState.vue'
import Lightbox from '../components/Lightbox.vue'
import MediaTile from '../components/MediaTile.vue'
import PostCard from '../components/PostCard.vue'
import Skeleton from '../components/Skeleton.vue'
import StatusChip from '../components/StatusChip.vue'
import WorkCard from '../components/WorkCard.vue'
import KpiCard from '../components/console/KpiCard.vue'
import TrendChart from '../components/console/TrendChart.vue'

const router = useRouter()
const data = ref({ actions: [], today: null, running: [], recentPosts: [], recentMedia: [], firstRun: {} })
const teams = ref([])
const trend = ref([])
const trendDays = ref(14)
const loading = ref(true)
const loadError = ref(false)
const quickInput = ref('')
const boxItem = ref(null)
const boxSrc = ref('')

function quickLaunch() {
  const v = quickInput.value.trim()
  if (!v) {
    router.push('/new')
    return
  }
  const isUrl = /^https?:\/\//i.test(v) || /^[a-z0-9-]+\.[a-z]{2,}/i.test(v)
  if (isUrl) {
    const full = /^https?:\/\//i.test(v) ? v : `https://${v}`
    router.push({ path: '/new', query: { kind: 'url', url: full } })
  } else {
    router.push({ path: '/new', query: { kind: 'topic', topic: v } })
  }
}

function actionLabel(kind) {
  if (kind === 'login') return '다시 로그인'
  if (kind === 'account' || kind === 'reviewer') return '계정 연결'
  if (kind === 'failed') return '게시물 확인'
  return '확인하기'
}

const showChecklist = computed(() => {
  const f = data.value.firstRun
  return f && (!f.aiConnected || !f.channelConnected || !f.hasWork)
})
const checkDone = computed(() => {
  const f = data.value.firstRun || {}
  return (f.aiConnected ? 1 : 0) + (f.channelConnected ? 1 : 0) + (f.hasWork ? 1 : 0)
})

const needs = computed(() => {
  const out = (data.value.actions || []).map((a) => ({
    text: a.message || '확인 필요',
    to: a.href || '/',
    action: a.label || actionLabel(a.kind),
  }))
  const failed = data.value.today?.failed || 0
  const hasFailed = out.some((n) => n.to === '/works' || n.text.includes('실패'))
  if (failed > 0 && !hasFailed) {
    out.push({ text: `발행 실패 ${failed}건 확인이 필요합니다`, to: '/works?tab=failed', action: '확인하기' })
  }
  return out.slice(0, 6)
})

function dayDelta(key) {
  const d = trend.value
  if (d.length < 2) return null
  return d[d.length - 1][key] - d[d.length - 2][key]
}
const pubDelta = computed(() => dayDelta('published'))
const failDelta = computed(() => dayDelta('failed'))

async function setTrendDays(days) {
  trendDays.value = days
  try {
    const tr = await apiFetch(`/api/publish-trend?days=${days}`)
    trend.value = tr.days || []
  } catch { /* keep existing */ }
}

async function openMedia(m) {
  boxItem.value = m
  boxSrc.value = ''
  try {
    const headers = {}
    if (auth.token) headers['Authorization'] = `Bearer ${auth.token}`
    const res = await fetch(`/api/media/${m.id}/file`, { headers })
    if (res.ok) boxSrc.value = URL.createObjectURL(await res.blob())
  } catch { /* ignore */ }
}

function closeBox() {
  if (boxSrc.value) URL.revokeObjectURL(boxSrc.value)
  boxSrc.value = ''
  boxItem.value = null
}

async function load() {
  loadError.value = false
  try {
    const [res, tr, tm] = await Promise.all([
      apiFetch('/api/overview'),
      apiFetch(`/api/publish-trend?days=${trendDays.value}`).catch(() => ({ days: [] })),
      apiFetch('/api/teams').catch(() => ({ items: [] })),
    ])
    data.value = {
      actions: res.actions || [],
      today: res.today,
      running: res.running || [],
      recentPosts: res.recentPosts || [],
      recentMedia: res.recentMedia || [],
      firstRun: res.firstRun || {},
    }
    trend.value = tr.days || []
    teams.value = tm.items || []
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

const poll = usePoll(load, { isBusy: () => (data.value.today?.running || 0) > 0 })
onMounted(() => poll.start())
onUnmounted(closeBox)
</script>

<style scoped>
.home { display: flex; flex-direction: column; gap: 22px; }

.action-banner {
  background: var(--warn-soft);
  border: 1px solid rgba(220, 38, 38, 0.25);
  border-radius: var(--r-lg);
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.ab-head { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.ab-badge {
  background: var(--warn);
  color: #fff;
  font-size: 12px;
  font-weight: 800;
  padding: 2px 10px;
  border-radius: 999px;
}
.ab-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
.ab-list li { display: flex; align-items: center; gap: 10px; }
.ab-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--warn); flex: none; }
.ab-msg { flex: 1; font-size: 13.5px; font-weight: 600; color: var(--ink); }
.ab-btn {
  background: var(--panel);
  color: var(--warn);
  border: 1px solid rgba(220, 38, 38, 0.35);
  border-radius: 8px;
  padding: 5px 12px;
  font-size: 12.5px;
  font-weight: 700;
  text-decoration: none;
  white-space: nowrap;
}
.ab-btn:hover { background: var(--warn); color: #fff; }

.launch-bar {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--r-lg);
  padding: 20px 22px;
  box-shadow: var(--shadow);
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.lb-copy { display: flex; flex-direction: column; gap: 2px; }
.lb-title { font-size: 16px; font-weight: 800; color: var(--ink); }
.lb-form { display: flex; gap: 10px; }
.lb-form input { flex: 1; padding: 11px 14px; font-size: 14.5px; border-radius: var(--r-md); }
.lb-form button { white-space: nowrap; padding: 11px 20px; border-radius: var(--r-md); font-weight: 700; }

.check { display: flex; flex-direction: column; gap: 14px; }
.prog-pill { font-size: 12.5px; font-weight: 700; color: var(--accent-dark); background: var(--accent-soft); padding: 3px 10px; border-radius: 999px; }
.prog-track { height: 6px; border-radius: 999px; background: var(--soft); overflow: hidden; }
.prog-fill { height: 100%; background: var(--accent); border-radius: 999px; transition: width 300ms ease; }
.check-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 10px; }
.check-list li { display: flex; align-items: center; gap: 12px; }
.check-list a { flex: 1; text-decoration: none; color: var(--ink); font-weight: 600; }
.check-list a:hover { color: var(--accent); }
.step-num {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--soft);
  color: var(--muted);
  font-size: 12px;
  font-weight: 800;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
}
.step-num.ok { background: var(--ok); color: #fff; }

.teams-strip { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.ts-label { font-weight: 700; flex: none; }
.ts-chips { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.tchip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 6px 12px;
  border-radius: 999px;
  background: var(--panel);
  border: 1px solid var(--line);
  text-decoration: none;
  color: var(--ink);
  font-size: 13px;
  box-shadow: var(--shadow-sm);
  transition: border-color var(--t), background var(--t);
}
.tchip:hover { border-color: var(--accent); background: var(--accent-soft); }
.tchip.off { opacity: 0.6; }
.tchip.add { border-style: dashed; color: var(--muted); font-weight: 600; }
.tdot { width: 7px; height: 7px; border-radius: 50%; background: var(--muted); }
.tdot.on { background: var(--ok); }
.tcount { font-size: 12px; font-weight: 700; color: var(--accent-dark); }

.kpis { display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; }

.wrap-head { flex-wrap: wrap; margin-bottom: 12px; }
.views-val { color: var(--ink); font-weight: 800; }
.period-seg {
  display: inline-flex;
  background: var(--soft);
  padding: 3px;
  border-radius: 999px;
  gap: 2px;
}
.period-seg button {
  background: transparent;
  color: var(--muted);
  border: none;
  border-radius: 999px;
  padding: 4px 12px;
  font-size: 12.5px;
  font-weight: 600;
}
.period-seg button.on { background: var(--panel); color: var(--ink); box-shadow: var(--shadow-sm); }

.main-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; align-items: start; }
.col-sec { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
.stack { display: flex; flex-direction: column; gap: 10px; }
.more { font-size: 13px; font-weight: 600; text-decoration: none; color: var(--accent); }
.more:hover { text-decoration: underline; }

.tiles { display: grid; grid-template-columns: repeat(6, 1fr); gap: 10px; }
.center { text-align: center; display: flex; flex-direction: column; align-items: center; gap: 10px; }

@media (max-width: 1020px) {
  .main-grid { grid-template-columns: 1fr; }
}
@media (max-width: 900px) {
  .kpis { grid-template-columns: repeat(2, 1fr); gap: 10px; }
  .tiles { grid-template-columns: repeat(3, 1fr); }
  .lb-form { flex-direction: column; }
}
@media (max-width: 600px) {
  .tiles { grid-template-columns: repeat(2, 1fr); }
}
</style>
