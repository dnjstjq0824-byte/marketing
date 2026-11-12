<template>
  <section class="page">
    <h2>대시보드</h2>
    <p class="muted">기능 나열이 아니라 오늘의 상태 요약입니다.</p>
    <p v-if="error" class="error">{{ error }} <button class="ghost" @click="load">다시 시도</button></p>

    <div class="statgrid">
      <div class="stat"><b>{{ counts.reviewWaiting }}</b><span>검수 대기</span></div>
      <div class="stat"><b>{{ counts.jobsPending }}</b><span>대기 작업</span></div>
      <div class="stat"><b>{{ counts.jobsClaimed }}</b><span>처리 중 작업</span></div>
      <div class="stat"><b>{{ counts.queuedTotal }}</b><span>발행 큐</span></div>
      <div class="stat"><b>{{ counts.scheduledToday }}</b><span>오늘 일정</span></div>
    </div>

    <div class="card">
      <div class="sect-head"><h3>사업별 현황</h3><span class="count">{{ byBusiness.length }}개 사업</span></div>
      <div v-if="byBusiness.length" class="table-wrap">
        <table>
          <thead><tr><th>사업</th><th>상품</th><th>검수 대기</th><th>발행 큐</th></tr></thead>
          <tbody>
            <tr v-for="b in byBusiness" :key="b.business">
              <td><strong>{{ b.business || '(미지정)' }}</strong></td>
              <td>{{ b.products }}</td>
              <td>{{ b.review }}</td>
              <td>{{ b.queued }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else class="empty">등록된 사업이 없습니다.</p>
    </div>

    <div class="card">
      <div class="sect-head"><h3>이상 알림</h3><span class="count">{{ alerts.length }}건</span></div>
      <ul v-if="alerts.length" class="speclist">
        <li v-for="(a, i) in alerts" :key="i"><span class="pill p-failed">알림</span><span>{{ a }}</span></li>
      </ul>
      <p v-else class="empty">이상이 없습니다. 평온합니다.</p>
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
      <p v-else class="empty">오늘 예정된 발행이 없습니다.</p>
    </div>

    <div class="row">
      <div class="card grow">
        <h3>큐 요약</h3>
        <ul class="speclist">
          <li><span class="k">실패한 작업</span><span><strong>{{ counts.jobsFailed }}</strong>건 — 상주 관리 화면에서 확인</span></li>
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
import { onMounted, ref } from 'vue'
import { apiFetch } from '../stores/auth.js'

const counts = ref({ reviewWaiting: 0, jobsPending: 0, jobsClaimed: 0, jobsFailed: 0, queuedTotal: 0, heldTotal: 0, scheduledToday: 0 })
const alerts = ref([])
const relogin = ref([])
const today = ref([])
const byBusiness = ref([])
const error = ref('')

function shortTime(iso) {
  return iso ? iso.slice(11, 16) : '—'
}

async function load() {
  error.value = ''
  try {
    const data = await apiFetch('/api/dashboard')
    counts.value = data.counts || counts.value
    relogin.value = data.reloginNeeded || []
    today.value = data.today || []
    byBusiness.value = data.byBusiness || []
    const a = []
    if (counts.value.jobsFailed > 0) a.push(`실패한 작업 ${counts.value.jobsFailed}건이 큐 화면에서 확인을 기다립니다.`)
    if (counts.value.heldTotal > 0) a.push(`보류된 발행 ${counts.value.heldTotal}건이 있습니다.`)
    for (const r of relogin.value) a.push(`계정 ${r.accountName}: 세션 ${r.sessionState} — 재로그인 필요`)
    alerts.value = a
  } catch (e) {
    error.value = e.message || '불러오기 실패'
  }
}

onMounted(load)
</script>
