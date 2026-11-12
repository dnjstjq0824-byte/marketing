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
      <h3>이상 알림</h3>
      <ul v-if="alerts.length">
        <li v-for="(a, i) in alerts" :key="i">{{ a }}</li>
      </ul>
      <p v-else class="muted">이상이 없습니다.</p>
    </div>

    <div class="row">
      <div class="card grow">
        <h3>큐 요약</h3>
        <ul>
          <li>실패한 작업 {{ counts.jobsFailed }}건 — 상주 관리 화면에서 확인</li>
          <li>보류된 발행 {{ counts.heldTotal }}건 — 발행 큐에서 확인</li>
        </ul>
      </div>
      <div class="card grow">
        <h3>재로그인 필요</h3>
        <ul v-if="relogin.length">
          <li v-for="r in relogin" :key="r.accountId">계정 {{ r.accountName }} · 보류 {{ r.heldCount }}건</li>
        </ul>
        <p v-else class="muted">없음</p>
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
const error = ref('')

async function load() {
  error.value = ''
  try {
    const data = await apiFetch('/api/dashboard')
    counts.value = data.counts || counts.value
    relogin.value = data.reloginNeeded || []
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
