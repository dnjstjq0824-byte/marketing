<template>
  <section v-if="view === 'agents'" class="page">
    <h2>에이전트 연결</h2>
    <p class="muted">슬롯 구조입니다. A·B·C 칸은 예시이며 특정 에이전트에 종속되지 않습니다. 필요하면 슬롯을 추가하세요.</p>
    <p v-if="error" class="error">{{ error }}</p>

    <div class="card">
      <h3>슬롯</h3>
      <div class="statgrid">
        <div v-for="s in slots" :key="s.id" class="stat">
          <b>{{ s.label }}</b>
          <span>{{ s.agentName || '(빈 슬롯)' }}</span>
        </div>
      </div>
      <div class="formrow">
        <label>칸 이름 <input v-model="slotForm.label" placeholder="예: D" /></label>
        <label>에이전트명 <input v-model="slotForm.agentName" placeholder="예: 새 에이전트" /></label>
        <button @click="addSlot">슬롯 추가</button>
      </div>
    </div>

    <div class="card">
      <h3>연결된 계정 요약 (카드 보기)</h3>
      <div class="statgrid">
        <div class="stat"><b>{{ accounts.length }}</b><span>전체 계정</span></div>
        <div class="stat"><b>{{ counts.chief }}</b><span>총괄</span></div>
        <div class="stat"><b>{{ counts.worker }}</b><span>작업자</span></div>
        <div class="stat"><b>{{ counts.alert }}</b><span>이상 계정</span></div>
      </div>
    </div>

    <div class="card">
      <h3>계정 목록</h3>
      <div class="toolbar">
        <label>역할
          <select v-model="roleFilter">
            <option value="">전체</option>
            <option value="chief">총괄</option>
            <option value="worker">작업자</option>
          </select>
        </label>
        <label>사업
          <select v-model="bizFilter">
            <option value="">전체 사업</option>
            <option v-for="b in businesses" :key="b" :value="b">{{ b }}</option>
          </select>
        </label>
        <label class="check"><input v-model="alertOnly" type="checkbox" /> 이상 계정만</label>
        <button class="ghost" @click="load">새로고침</button>
      </div>
      <table v-if="filtered.length">
        <thead><tr><th>계정</th><th>역할</th><th>담당 사업</th><th>세션 상태</th><th>사용량</th><th>마지막 작업</th></tr></thead>
        <tbody>
          <tr v-for="a in filtered" :key="a.id">
            <td>{{ a.name }}</td>
            <td>{{ a.role === 'chief' ? '총괄' : '작업자' }}</td>
            <td>{{ a.business || '—' }}</td>
            <td>{{ sessionLabel(a.sessionState) }}</td>
            <td>{{ a.usage || '—' }}</td>
            <td>{{ shortDate(a.lastJobAt) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-else class="muted">조건에 맞는 계정이 없습니다.</p>
    </div>

    <div class="card">
      <h3>계정 등록</h3>
      <div class="formrow">
        <label>계정명 <input v-model="accForm.name" /></label>
        <label>플랫폼 <input v-model="accForm.platform" placeholder="예: claude-app" /></label>
        <label>주인 <input v-model="accForm.owner" /></label>
        <label>역할
          <select v-model="accForm.role"><option value="chief">총괄</option><option value="worker">작업자</option></select>
        </label>
        <label>담당 사업 <input v-model="accForm.business" /></label>
        <button @click="addAccount">등록</button>
      </div>
    </div>
  </section>

  <section v-else-if="view === 'channels'" class="page">
    <h2>채널 계정</h2>
    <p class="muted">SNS·블로그 계정의 세션 상태와 자동 발행 가능 여부를 봅니다. 아이디·비밀번호는 저장하지 않습니다.</p>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="card">
      <div class="toolbar">
        <button @click="checkNow">가벼운 점검 실행</button>
        <span class="muted small">점검은 가볍게 주기로 돌고, 실제 작업이 배정된 세션만 깨웁니다.</span>
      </div>
      <table v-if="accounts.length">
        <thead><tr><th>계정</th><th>플랫폼</th><th>세션 상태</th><th>자동 발행</th><th>마지막 확인</th><th>변경</th></tr></thead>
        <tbody>
          <tr v-for="a in accounts" :key="a.id">
            <td>{{ a.name }}</td>
            <td>{{ a.platform }}</td>
            <td>{{ sessionLabel(a.sessionState) }}</td>
            <td>{{ a.sessionState === 'connected' ? '가능 (트랜치 2)' : '불가 · 발행 정지' }}</td>
            <td>{{ shortDate(a.lastCheckedAt) }}</td>
            <td>
              <button class="ghost" @click="setSession(a.id, 'connected')">연결</button>
              <button class="ghost" @click="setSession(a.id, 'expired')">만료</button>
              <button class="ghost" @click="setSession(a.id, 'limited')">한도</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-else class="muted">등록된 채널 계정이 없습니다. 에이전트 연결 화면에서 등록하세요.</p>
    </div>
    <div class="card">
      <h3>재로그인 필요 큐</h3>
      <ul v-if="relogin.length">
        <li v-for="r in relogin" :key="r.accountId">
          {{ r.accountName }} ({{ r.owner || '주인 미지정' }}) · 보류 {{ r.heldCount }}건 · {{ sessionLabel(r.sessionState) }}
        </li>
      </ul>
      <p v-else class="muted">없음</p>
    </div>
  </section>

  <section v-else class="page">
    <h2>자체 채널</h2>
    <p class="muted">SOMUN이 직접 운영하는 채널 자리입니다.</p>
    <div class="card">
      <h3>체크인데일리</h3>
      <p>운영 중 · 상세 지표는 지표 화면에서 확인합니다. (트랜치 1: 자리 표시)</p>
    </div>
    <div class="card">
      <h3>주식·코인 뉴스 확장 슬롯</h3>
      <p class="muted">확장 예정 슬롯입니다. (트랜치 1: 자리 표시)</p>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { apiFetch } from '../stores/auth.js'

const props = defineProps({ view: { type: String, default: 'agents' } })

const accounts = ref([])
const slots = ref([])
const relogin = ref([])
const error = ref('')
const roleFilter = ref('')
const bizFilter = ref('')
const alertOnly = ref(false)
const slotForm = ref({ label: '', agentName: '' })
const accForm = ref({ name: '', platform: '', owner: '', role: 'worker', business: '' })

const ALERT = new Set(['expired', 'limited', 'inactive_hold'])

const businesses = computed(() => {
  const set = new Set()
  for (const a of accounts.value) if (a.business) set.add(a.business)
  return [...set]
})

const counts = computed(() => ({
  chief: accounts.value.filter((a) => a.role === 'chief').length,
  worker: accounts.value.filter((a) => a.role === 'worker').length,
  alert: accounts.value.filter((a) => ALERT.has(a.sessionState)).length,
}))

const filtered = computed(() =>
  accounts.value.filter((a) => {
    if (roleFilter.value && a.role !== roleFilter.value) return false
    if (bizFilter.value && a.business !== bizFilter.value) return false
    if (alertOnly.value && !ALERT.has(a.sessionState)) return false
    return true
  }),
)

function sessionLabel(s) {
  return { pending: '대기', connected: '연결', expired: '만료', limited: '한도', inactive_hold: '비활성 보류' }[s] || s || '—'
}

function shortDate(iso) {
  return iso ? iso.slice(0, 16).replace('T', ' ') : '—'
}

async function load() {
  error.value = ''
  try {
    const [acc, sl, rl] = await Promise.all([
      apiFetch('/api/session/accounts'),
      apiFetch('/api/session/slots'),
      apiFetch('/api/session/relogin-queue'),
    ])
    accounts.value = acc.accounts || []
    slots.value = sl.slots || []
    relogin.value = rl.items || []
  } catch (e) {
    error.value = e.message
  }
}

async function addSlot() {
  error.value = ''
  try {
    await apiFetch('/api/session/slots', { method: 'POST', body: JSON.stringify(slotForm.value) })
    slotForm.value = { label: '', agentName: '' }
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function addAccount() {
  error.value = ''
  try {
    await apiFetch('/api/session/accounts', { method: 'POST', body: JSON.stringify(accForm.value) })
    accForm.value = { name: '', platform: '', owner: '', role: 'worker', business: '' }
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function setSession(id, state) {
  error.value = ''
  try {
    await apiFetch(`/api/session/accounts/${id}`, { method: 'PATCH', body: JSON.stringify({ sessionState: state }) })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function checkNow() {
  error.value = ''
  try {
    await apiFetch('/api/session/check', { method: 'POST' })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

watch(() => props.view, load)
onMounted(load)
</script>
