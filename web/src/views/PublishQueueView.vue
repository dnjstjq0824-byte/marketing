<template>
  <section class="page">
    <h2>발행 큐</h2>
    <p class="muted">큐·상태·화면까지만 제공합니다. 실제 외부 채널 발행은 트랜치 2입니다. 두 채널군은 분리 표기되며 섞지 않습니다.</p>
    <p v-if="error" class="error">{{ error }}</p>

    <div class="card">
      <h3>예약 등록 (queued 항목 대상)</h3>
      <div class="formrow">
        <label>콘텐츠 ID <input v-model="sched.id" placeholder="예: 3" /></label>
        <label>계정 <input v-model="sched.account" placeholder="예: somun_official" /></label>
        <label>플랫폼
          <select v-model="sched.platform">
            <option value="instagram">instagram (API군)</option>
            <option value="facebook">facebook (API군)</option>
            <option value="threads">threads (API군)</option>
            <option value="tiktok">tiktok (API군)</option>
            <option value="youtube">youtube (API군)</option>
            <option value="blogger">blogger (API군)</option>
            <option value="naver_blog">naver_blog (직접군)</option>
            <option value="tistory">tistory (직접군)</option>
          </select>
        </label>
        <label>예정 시각 <input v-model="sched.at" type="datetime-local" /></label>
        <button @click="reserve">예약</button>
      </div>
      <p class="muted small">과거 시각은 입력 단계에서 거부됩니다. 같은 계정·같은 시각 예약은 적재 순서대로 직렬 실행됩니다.</p>
      <p v-if="schedMsg" class="ok">{{ schedMsg }}</p>
    </div>

    <div class="row">
      <div class="card grow">
        <h3>공식 발행 API군</h3>
        <p class="muted small">Instagram / Facebook · Threads · TikTok · YouTube · Blogger</p>
        <table v-if="official.length">
          <thead><tr><th>ID</th><th>제목</th><th>계정</th><th>예정</th><th>보류</th></tr></thead>
          <tbody>
            <tr v-for="c in official" :key="c.id">
              <td>{{ c.id }}</td>
              <td class="ellipsis">{{ c.title || '(제목 없음)' }}</td>
              <td>{{ c.targetAccount || '—' }}</td>
              <td>{{ shortDate(c.scheduledAt) }}</td>
              <td>{{ c.held ? '보류 · ' + c.holdReason : '—' }}</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="muted">큐가 비어 있습니다.</p>
      </div>
      <div class="card grow">
        <h3>직접 posting군 (API 없음)</h3>
        <p class="muted small">네이버 블로그 · 티스토리 — 처리 방향은 PO 결정 대기</p>
        <table v-if="manual.length">
          <thead><tr><th>ID</th><th>제목</th><th>계정</th><th>예정</th><th>보류</th></tr></thead>
          <tbody>
            <tr v-for="c in manual" :key="c.id">
              <td>{{ c.id }}</td>
              <td class="ellipsis">{{ c.title || '(제목 없음)' }}</td>
              <td>{{ c.targetAccount || '—' }}</td>
              <td>{{ shortDate(c.scheduledAt) }}</td>
              <td>{{ c.held ? '보류 · ' + c.holdReason : '—' }}</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="muted">큐가 비어 있습니다.</p>
      </div>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiFetch } from '../stores/auth.js'

function shortDate(iso) {
  return iso ? iso.slice(0, 16).replace('T', ' ') : '—'
}

const official = ref([])
const manual = ref([])
const error = ref('')
const sched = ref({ id: '', account: '', platform: 'instagram', at: '' })
const schedMsg = ref('')

async function load() {
  error.value = ''
  try {
    const [a, b] = await Promise.all([
      apiFetch('/api/publish-queue?group=official_api'),
      apiFetch('/api/publish-queue?group=manual'),
    ])
    official.value = a.items || []
    manual.value = b.items || []
  } catch (e) {
    error.value = e.message
  }
}

function toRFC3339(local) {
  if (!local) return ''
  const d = new Date(local)
  return d.toISOString()
}

async function reserve() {
  error.value = ''
  schedMsg.value = ''
  try {
    const res = await apiFetch(`/api/contents/${sched.value.id}/schedule`, {
      method: 'POST',
      body: JSON.stringify({
        account: sched.value.account.trim(),
        platform: sched.value.platform,
        scheduledAt: toRFC3339(sched.value.at),
      }),
    })
    schedMsg.value = `예약됨 · 같은 시각 직렬 순서 ${res.queuePosition || '—'}번`
    await load()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)
</script>
