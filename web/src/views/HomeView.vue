<template>
  <div class="home">
    <section v-if="showChecklist" class="card check" aria-label="시작 체크리스트">
      <h2 class="section-title">시작하기</h2>
      <ol>
        <li><RouterLink to="/connect">AI 계정 연결하기</RouterLink><StatusChip :kind="data.firstRun.aiConnected ? 'ok' : 'hold'">{{ data.firstRun.aiConnected ? '됨' : '안 됨' }}</StatusChip></li>
        <li><RouterLink to="/connect">발행 채널 연결하기</RouterLink><StatusChip :kind="data.firstRun.channelConnected ? 'ok' : 'hold'">{{ data.firstRun.channelConnected ? '됨' : '안 됨' }}</StatusChip></li>
        <li><RouterLink to="/new">첫 작업 만들기</RouterLink><StatusChip :kind="data.firstRun.hasWork ? 'ok' : 'hold'">{{ data.firstRun.hasWork ? '됨' : '안 됨' }}</StatusChip></li>
      </ol>
    </section>
    <Skeleton v-if="loading && !data.today" :rows="4" />
    <div v-else-if="loadError" class="card center">
      <p>불러오지 못했습니다.</p>
      <button @click="load">다시 시도</button>
    </div>
    <template v-else-if="data.today">
      <section class="kpis" aria-label="오늘 현황">
        <KpiCard label="진행 중" :value="data.today.running" :delta="null" to="/works" />
        <KpiCard label="오늘 발행" :value="data.today.publishedToday" :delta="pubDelta" to="/works?tab=published" />
        <KpiCard label="발행 대기" :value="data.today.queued" :delta="null" to="/works?tab=queued" />
        <KpiCard label="실패" :value="data.today.failed" :delta="failDelta" to="/works?tab=failed" invert />
      </section>
      <section class="card" aria-label="발행 추이">
        <div class="sect-head">
          <h2 class="section-title">발행 추이</h2>
          <span class="muted small">이번 주 조회 <b class="num">{{ data.today.weekViews }}</b></span>
        </div>
        <TrendChart :days="trend" />
      </section>
      <div class="topgrid">
        <section class="card today" aria-label="오늘 한 일">
          <h2 class="section-title">오늘 한 일</h2>
          <p class="today-line">발행 <strong class="num">{{ data.today.publishedToday }}</strong>건 · 진행 중 <strong class="num">{{ data.today.running }}</strong>건 · 대기 <strong class="num">{{ data.today.queued }}</strong>건</p>
          <ul v-if="todayPosts.length" class="donelist">
            <li v-for="p in todayPosts" :key="p.id">
              <a v-if="p.url" :href="p.url" target="_blank" rel="noopener">{{ p.title }}</a>
              <span v-else>{{ p.title }}</span>
              <span class="muted small">{{ p.channel }}</span>
            </li>
          </ul>
          <p v-else class="muted small">아직 발행된 글이 없습니다.</p>
        </section>
        <section class="card needs" :class="{ calm: needs.length === 0 }" aria-label="개입 필요">
          <h2 class="section-title">개입 필요 {{ needs.length }}건</h2>
          <p v-if="!needs.length" class="calm-msg">볼 일 없습니다 ✔</p>
          <ul v-else class="needlist">
            <li v-for="(n, i) in needs" :key="i">
              <span class="nmsg">{{ n.text }}</span>
              <RouterLink :to="n.to" class="ghost small link">{{ n.action }}</RouterLink>
            </li>
          </ul>
        </section>
      </div>
      <section aria-label="진행 중인 작업">
        <div class="sect-head"><h2 class="section-title">진행 중</h2><RouterLink to="/works" class="more">전체 보기</RouterLink></div>
        <div v-if="data.running.length" class="cards">
          <WorkCard v-for="w in data.running" :key="w.id" :work="w" />
        </div>
        <EmptyState v-else title="진행 중인 작업이 없습니다" hint="새 작업을 만들면 여기서 진행됩니다" to="/new" actionLabel="새 작업" />
      </section>
      <section v-if="data.recentPosts.length" aria-label="최근 발행">
        <div class="sect-head"><h2 class="section-title">최근 발행</h2><RouterLink to="/works?tab=published" class="more">전체 보기</RouterLink></div>
        <div class="cards">
          <PostCard v-for="p in data.recentPosts" :key="p.id" :post="p" />
        </div>
      </section>
      <section v-if="data.recentMedia.length" aria-label="최근 미디어">
        <div class="sect-head"><h2 class="section-title">최근 미디어</h2><RouterLink to="/media" class="more">전체 보기</RouterLink></div>
        <div class="tiles">
          <MediaTile v-for="m in data.recentMedia" :key="m.id" :item="m" />
        </div>
      </section>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiFetch } from '../stores/auth.js'
import { usePoll } from '../stores/poll.js'
import EmptyState from '../components/EmptyState.vue'
import MediaTile from '../components/MediaTile.vue'
import PostCard from '../components/PostCard.vue'
import Skeleton from '../components/Skeleton.vue'
import StatusChip from '../components/StatusChip.vue'
import WorkCard from '../components/WorkCard.vue'
import KpiCard from '../components/console/KpiCard.vue'
import TrendChart from '../components/console/TrendChart.vue'

const data = ref({ actions: [], today: null, running: [], recentPosts: [], recentMedia: [], firstRun: {} })
const trend = ref([])
const loading = ref(true)
const loadError = ref(false)

function actionLabel(kind) {
  if (kind === 'login') return '다시 로그인'
  if (kind === 'account' || kind === 'reviewer') return '계정 연결'
  if (kind === 'failed') return '게시물 확인'
  return '보기'
}

const showChecklist = computed(() => {
  const f = data.value.firstRun
  return f && (!f.aiConnected || !f.channelConnected || !f.hasWork)
})

const todayPosts = computed(() => {
  const day = new Date().toDateString()
  return (data.value.recentPosts || []).filter((p) => {
    try {
      return new Date(p.publishedAt).toDateString() === day
    } catch {
      return false
    }
  }).slice(0, 3)
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
    out.push({ text: `발행 실패 ${failed}건`, to: '/works?tab=failed', action: '확인하기' })
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

async function load() {
  loadError.value = false
  try {
    const [res, tr] = await Promise.all([
      apiFetch('/api/overview'),
      apiFetch('/api/publish-trend?days=14').catch(() => ({ days: [] })),
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
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

const poll = usePoll(load, { isBusy: () => (data.value.today?.running || 0) > 0 })
onMounted(() => poll.start())
</script>

<style scoped>
.home { display: flex; flex-direction: column; gap: var(--s5); }
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
.check { display: block; text-align: left; }
.check ol { margin: var(--s2) 0 0; padding-left: var(--s5); display: flex; flex-direction: column; gap: var(--s2); }
.check li { display: flex; align-items: center; gap: var(--s3); }
.sect-head { display: flex; align-items: center; margin-bottom: var(--s3); }
.sect-head h2 { flex: 1; }
.more { font-size: 13px; }
.today-line { margin: 0; font-size: 15px; }
.today-line strong { font-size: 18px; color: var(--accent-dark); }
.topgrid { display: flex; flex-direction: column; gap: var(--s5); }
@media (min-width: 1100px) {
  .topgrid { display: grid; grid-template-columns: 1.15fr 1fr; align-items: start; }
}
.donelist, .needlist { list-style: none; margin: var(--s2) 0 0; padding: 0; display: flex; flex-direction: column; gap: var(--s2); }
.donelist li { display: flex; gap: var(--s2); align-items: baseline; }
.needlist li { display: flex; gap: var(--s2); align-items: center; justify-content: space-between; }
.nmsg { flex: 1; }
.needs.calm { border-color: var(--ok); }
.calm-msg { margin: 0; color: var(--ok); font-weight: 700; }
.link {
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
  border-radius: var(--r-md);
  padding: var(--s1) var(--s2);
  font-size: 13px;
  text-decoration: none;
  white-space: nowrap;
}
.link:hover { background: var(--soft); }
.center { text-align: center; }
.center button {
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: var(--r-md);
  padding: var(--s2) var(--s4);
  margin-top: var(--s2);
}
.kpis { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--s3); }
.cards { display: flex; flex-direction: column; gap: var(--s3); }
.tiles { display: grid; grid-template-columns: repeat(6, 1fr); gap: var(--s2); }
@media (max-width: 900px) {
  .kpis { grid-template-columns: repeat(2, 1fr); gap: var(--s2); }
  .tiles { grid-template-columns: repeat(3, 1fr); }
}
@media (max-width: 700px) {
  .tiles { grid-template-columns: repeat(2, 1fr); }
}
@media (min-width: 1150px) {
  .cards { display: grid; grid-template-columns: 1fr 1fr; align-items: start; }
}
@media (min-width: 1700px) {
  .cards { grid-template-columns: 1fr 1fr 1fr; }
}
</style>
