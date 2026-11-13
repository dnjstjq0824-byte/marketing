<template>
  <section class="page">
    <h2>대시보드</h2>
    <p class="muted">기능 나열이 아니라 오늘의 상태 요약입니다.</p>
    <p v-if="error" class="banner err">{{ error }} <button class="ghost small" @click="load">다시 시도</button></p>

    <div class="hero">
      <div class="kick">{{ todayLine }}</div>
      <h3>오늘의 판</h3>
      <p class="lede">{{ lede }}</p>
      <ul class="hero-stats">
        <li :class="{ gold: counts.reviewWaiting > 0 }"><b>{{ counts.reviewWaiting }}</b><span>검수 대기</span></li>
        <li><b>{{ counts.jobsPending }}</b><span>대기 작업</span></li>
        <li><b>{{ counts.jobsClaimed }}</b><span>처리 중</span></li>
        <li :class="{ alert: counts.heldTotal > 0 }"><b>{{ counts.queuedTotal }}</b><span>발행 큐</span></li>
        <li><b>{{ counts.scheduledToday }}</b><span>오늘 일정</span></li>
      </ul>
    </div>

    <div class="card">
      <div class="sect-head"><h3>에이전트</h3><RouterLink to="/agents">연결 화면</RouterLink></div>
      <div class="statgrid">
        <div class="stat good"><b>{{ agentStats.online }}</b><span>온라인</span></div>
        <div class="stat" :class="{ bad: agentStats.offline > 0 }"><b>{{ agentStats.offline }}</b><span>오프라인</span></div>
        <div class="stat"><b>{{ agentStats.paused }}</b><span>중지</span></div>
        <div class="stat tone"><b>{{ agentStats.done1h }}</b><span>1시간 완료</span></div>
        <div class="stat" :class="{ bad: agentStats.failed1h > 0 }"><b>{{ agentStats.failed1h }}</b><span>1시간 실패</span></div>
      </div>
      <p v-if="topFailing.length" class="muted small">실패 상위:
        <span v-for="(f, i) in topFailing" :key="f.agentId">{{ i ? ' · ' : '' }}{{ f.label }} {{ f.failed }}건</span>
      </p>
    </div>

    <div class="card">
      <div class="sect-head"><h3>사업별 현황</h3><span class="count">{{ byBusiness.length }}개 사업</span></div>
      <div v-if="byBusiness.length" class="table-wrap">
        <table>
          <thead><tr><th>사업</th><th class="num">상품</th><th class="num">검수 대기</th><th class="num">발행 큐</th></tr></thead>
          <tbody>
            <tr v-for="b in byBusiness" :key="b.business">
              <td><strong>{{ b.business || '(미지정)' }}</strong></td>
              <td class="num">{{ b.products }}</td>
              <td class="num">{{ b.review }}</td>
              <td class="num">{{ b.queued }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else class="empty">등록된 사업이 없습니다.</p>
    </div>

    <div class="card">
      <div class="sect-head"><h3>이상 알림</h3><span class="count">{{ alerts.length }}건</span></div>
      <ul v-if="alerts.length" class="speclist">
        <li v-for="(a, i) in alerts" :key="i">
          <span :class="a.tone === 'warn' ? 'pill p-limited' : 'pill p-failed'">{{ a.tone === 'warn' ? '주의' : '알림' }}</span>
          <span>{{ a.text }} <RouterLink :to="a.to">바로 가기</RouterLink></span>
        </li>
      </ul>
      <EmptyArt v-else art="plant" title="이상이 없습니다" hint="평온합니다." />
    </div>

    <div class="card">
      <div class="sect-head"><h3>오늘 일정</h3><span class="count">{{ today.length }}건</span></div>
      <ul v-if="today.length" class="speclist">
        <li v-for="c in today" :key="c.id">
          <span class="k">{{ shortTime(c.scheduledAt) }}</span>
          <span>#{{ c.id }} {{ c.title || '(제목 없음)' }} · {{ c.targetAccount || '미지정' }}
            <span v-if="c.held" class="pill p-held">보류</span></span>
        </li>
      </ul>
      <EmptyArt v-else art="tray" title="오늘 예정된 발행이 없습니다" />
    </div>

    <div class="row">
      <div class="card grow">
        <h3>큐 요약</h3>
        <ul class="speclist">
          <li><span class="k">실패한 작업</span><span><strong>{{ counts.jobsFailed }}</strong>건 — 상주 관리 화면에서 확인</span></li>
          <li><span class="k">지연된 작업</span><span><strong>{{ counts.jobsStuck }}</strong>건 — 30분 넘게 처리 중</span></li>
          <li><span class="k">보류된 발행</span><span><strong>{{ counts.heldTotal }}</strong>건 — 발행 큐에서 확인</span></li>
        </ul>
      </div>
      <div class="card grow">
        <div class="sect-head"><h3>재로그인 필요</h3><span class="count">{{ relogin.length }}건</span></div>
        <ul v-if="relogin.length" class="speclist">
          <li v-for="r in relogin" :key="r.accountId">
            <span class="k">{{ r.accountName }}</span><span>보류 {{ r.heldCount }}건</span>
          </li>
        </ul>
        <p v-else class="empty">없음</p>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiFetch } from '../stores/auth.js'
import EmptyArt from '../components/EmptyArt.vue'

const counts = ref({ reviewWaiting: 0, jobsPending: 0, jobsClaimed: 0, jobsFailed: 0, jobsStuck: 0, queuedTotal: 0, heldTotal: 0, scheduledToday: 0 })
const alerts = ref([])
const agentStats = ref({ online: 0, offline: 0, paused: 0, done1h: 0, failed1h: 0 })
const topFailing = ref([])
const relogin = ref([])
const today = ref([])
const byBusiness = ref([])
const error = ref('')

function shortTime(iso) {
  return iso ? iso.slice(11, 16) : '—'
}

const todayLine = computed(() => {
  const d = new Date()
  const week = ['일', '월', '화', '수', '목', '금', '토'][d.getDay()]
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} (${week}) · 1인 운영 중`
})

const lede = computed(() => {
  const c = counts.value
  const bad = c.jobsFailed + c.jobsStuck + c.heldTotal
  if (bad > 0) return `확인할 일 ${bad}건이 있습니다. 아래 이상 알림부터 보세요.`
  if (c.reviewWaiting > 0) return `평온합니다. 검수 ${c.reviewWaiting}건만 보면 됩니다.`
  return '평온합니다. 오늘 볼일은 발행 큐 확인뿐입니다.'
})

async function load() {
  error.value = ''
  try {
    const data = await apiFetch('/api/dashboard')
    counts.value = data.counts || counts.value
    relogin.value = data.reloginNeeded || []
    today.value = data.today || []
    byBusiness.value = data.byBusiness || []
    try {
      const stats = await apiFetch('/api/agents/stats')
      agentStats.value = stats
      topFailing.value = stats.topFailing || []
    } catch {
      agentStats.value = { online: 0, offline: 0, paused: 0, done1h: 0, failed1h: 0 }
      topFailing.value = []
    }
    const a = []
    if (counts.value.jobsFailed > 0) a.push({ tone: 'err', to: '/resident', text: `실패한 작업 ${counts.value.jobsFailed}건이 확인을 기다립니다.` })
    if (counts.value.jobsStuck > 0) a.push({ tone: 'err', to: '/resident', text: `30분 넘게 처리 중인 작업 ${counts.value.jobsStuck}건이 있습니다. 작업자가 멈췄는지 확인하세요.` })
    if (counts.value.heldTotal > 0) a.push({ tone: 'warn', to: '/queue', text: `보류된 발행 ${counts.value.heldTotal}건이 있습니다.` })
    for (const r of relogin.value) a.push({ tone: 'warn', to: '/channels', text: `계정 ${r.accountName}: 세션 ${r.sessionState} — 재로그인 필요` })
    alerts.value = a
  } catch (e) {
    error.value = e.message || '불러오기 실패'
  }
}

onMounted(load)
</script>
