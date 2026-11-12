<template>
  <section class="page">
    <h2>발행 큐</h2>
    <p class="muted">큐·상태·화면까지만 제공합니다. 실제 외부 채널 발행은 트랜치 2입니다. 두 채널군은 분리 표기되며 섞지 않습니다.</p>
    <p v-if="error" class="error">{{ error }}</p>

    <div class="card">
      <h3>예약 등록 (queued 항목 대상)</h3>
      <div class="form-grid">
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
        <div class="full"><button @click="reserve">예약</button></div>
      </div>
      <p class="muted small">과거 시각은 입력 단계에서 거부됩니다. 같은 계정·같은 시각 예약은 적재 순서대로 직렬 실행됩니다.</p>
      <p v-if="schedMsg" class="ok">{{ schedMsg }}</p>
    </div>

    <div class="card">
      <div class="sect-head"><h3>발행 기록 (plan)</h3><span class="count">{{ logs.length }}건</span></div>
      <p class="muted small">콘텐츠·채널·계정 삼중항의 중복 등록은 거부됩니다. done/failed는 트랜치 2에서만 생깁니다.</p>
      <div class="form-grid">
        <label>콘텐츠 ID <input v-model="logForm.contentId" placeholder="예: 3" /></label>
        <label>채널 <input v-model="logForm.channel" placeholder="예: instagram" /></label>
        <label>계정 <input v-model="logForm.account" placeholder="예: somun_official" /></label>
        <label>상태
          <select v-model="logForm.status">
            <option value="scheduled">scheduled</option>
            <option value="held">held</option>
          </select>
        </label>
        <label class="full">메모 <input v-model="logForm.note" /></label>
        <div class="full"><button @click="addLog">기록 등록</button></div>
      </div>
      <div v-if="logs.length" class="table-wrap">
        <table>
          <thead><tr><th>ID</th><th>콘텐츠</th><th>채널</th><th>계정</th><th>상태</th><th>메모</th><th></th></tr></thead>
          <tbody>
            <tr v-for="l in logs" :key="l.id">
              <td>{{ l.id }}</td>
              <td>{{ l.contentId }}</td>
              <td>{{ l.channel }}</td>
              <td>{{ l.account }}</td>
              <td>
                <select v-if="editingLog === l.id" v-model="logEdit.status">
                  <option value="scheduled">scheduled</option>
                  <option value="held">held</option>
                </select>
                <span v-else>{{ l.status }}</span>
              </td>
              <td>
                <input v-if="editingLog === l.id" v-model="logEdit.note" />
                <span v-else>{{ l.note || '—' }}</span>
              </td>
              <td>
                <template v-if="editingLog === l.id">
                  <button @click="saveLog(l.id)">저장</button>
                  <button class="ghost" @click="editingLog = null">취소</button>
                </template>
                <template v-else>
                  <button class="ghost" @click="startLogEdit(l)">수정</button>
                  <button class="ghost" @click="deleteLog(l.id)">삭제</button>
                </template>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else class="empty">기록이 없습니다.</p>
    </div>

    <div class="row">
      <div class="card grow">
        <div class="sect-head"><h3>공식 발행 API군</h3><span class="count">{{ official.length }}건</span></div>
        <p class="muted small">Instagram / Facebook · Threads · TikTok · YouTube · Blogger</p>
        <div v-if="official.length" class="table-wrap">
          <table>
            <thead><tr><th>ID</th><th>사업</th><th>제목</th><th>계정</th><th>예정</th><th>보류</th></tr></thead>
            <tbody>
              <tr v-for="c in official" :key="c.id">
                <td>{{ c.id }}</td>
                <td>{{ prodBiz[c.productId] || '(미지정)' }}</td>
                <td class="ellipsis">{{ c.title || '(제목 없음)' }}</td>
                <td>{{ c.targetAccount || '—' }}</td>
                <td>{{ shortDate(c.scheduledAt) }}</td>
                <td><span v-if="c.held" class="pill p-held">보류</span><span v-else>—</span> {{ c.held ? c.holdReason : '' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="empty">큐가 비어 있습니다.</p>
      </div>
      <div class="card grow">
        <div class="sect-head"><h3>직접 posting군 (API 없음)</h3><span class="count">{{ manual.length }}건</span></div>
        <p class="muted small">네이버 블로그 · 티스토리 — 처리 방향은 PO 결정 대기</p>
        <div v-if="manual.length" class="table-wrap">
          <table>
            <thead><tr><th>ID</th><th>사업</th><th>제목</th><th>계정</th><th>예정</th><th>보류</th></tr></thead>
            <tbody>
              <tr v-for="c in manual" :key="c.id">
                <td>{{ c.id }}</td>
                <td>{{ prodBiz[c.productId] || '(미지정)' }}</td>
                <td class="ellipsis">{{ c.title || '(제목 없음)' }}</td>
                <td>{{ c.targetAccount || '—' }}</td>
                <td>{{ shortDate(c.scheduledAt) }}</td>
                <td><span v-if="c.held" class="pill p-held">보류</span><span v-else>—</span> {{ c.held ? c.holdReason : '' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="empty">큐가 비어 있습니다.</p>
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
const logs = ref([])
const prodBiz = ref({})
const error = ref('')
const sched = ref({ id: '', account: '', platform: 'instagram', at: '' })
const schedMsg = ref('')
const logForm = ref({ contentId: '', channel: '', account: '', status: 'scheduled', note: '' })
const editingLog = ref(null)
const logEdit = ref({ status: 'scheduled', note: '' })

function startLogEdit(l) {
  logEdit.value = { status: l.status, note: l.note || '' }
  editingLog.value = l.id
}

async function saveLog(id) {
  error.value = ''
  try {
    await apiFetch(`/api/publish-logs/${id}`, { method: 'PATCH', body: JSON.stringify(logEdit.value) })
    editingLog.value = null
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function deleteLog(id) {
  error.value = ''
  try {
    await apiFetch(`/api/publish-logs/${id}`, { method: 'DELETE' })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function load() {
  error.value = ''
  try {
    const [a, b, c, d] = await Promise.all([
      apiFetch('/api/publish-queue?group=official_api'),
      apiFetch('/api/publish-queue?group=manual'),
      apiFetch('/api/publish-logs'),
      apiFetch('/api/products'),
    ])
    official.value = a.items || []
    manual.value = b.items || []
    logs.value = c.logs || []
    const map = {}
    for (const p of d.products || []) map[p.id] = p.business || ''
    prodBiz.value = map
  } catch (e) {
    error.value = e.message
  }
}

async function addLog() {
  error.value = ''
  try {
    await apiFetch('/api/publish-logs', {
      method: 'POST',
      body: JSON.stringify({
        contentId: Number(logForm.value.contentId),
        channel: logForm.value.channel.trim(),
        account: logForm.value.account.trim(),
        status: logForm.value.status,
        note: logForm.value.note,
      }),
    })
    logForm.value = { contentId: '', channel: '', account: '', status: 'scheduled', note: '' }
    await load()
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
