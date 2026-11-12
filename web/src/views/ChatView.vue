<template>
  <section class="page">
    <h2>대화</h2>
    <p class="muted">에이전트 슬롯이 늘어도 의장님의 대화 상대는 총괄 1자리로 고정됩니다.</p>

    <div class="tabs">
      <button :class="{ active: peer === 'chief' }" @click="peer = 'chief'">총괄 (판정·기준·지표)</button>
      <button :class="{ active: peer === 'worker' }" @click="peer = 'worker'">작업자 (에이전트별 스레드)</button>
    </div>

    <!-- Re-login notice takes precedence over the conversation when expired. -->
    <div v-if="session === 'expired'" class="card">
      <div class="sect-head"><h3>세션이 만료됐습니다</h3><span class="pill p-expired">만료</span></div>
      <p>대화보다 재로그인이 먼저입니다. 아래 버튼으로 다시 로그인하면 대화가 활성화됩니다.</p>
      <button @click="relogin">Claude 로그인</button>
    </div>

    <div v-else-if="session !== 'connected'" class="card">
      <template v-if="loginStep === 0">
        <p class="muted">아직 로그인 전입니다. 로그인하면 대화가 활성화됩니다. (트랜치 1: 화면·상태까지만, 실제 세션 연결 없음)</p>
        <button @click="loginStep = 1">Claude 로그인</button>
        <button class="ghost" @click="session = 'expired'">만료 상태 미리보기</button>
      </template>
      <template v-else-if="loginStep === 1">
        <h3>Claude 로그인 안내</h3>
        <ol class="declist">
          <li>아래 버튼을 누르면 공식 로그인 화면이 열립니다.</li>
          <li>의장님 계정으로 로그인합니다.</li>
          <li>완료되면 대화가 활성화됩니다.</li>
        </ol>
        <div class="btnrow">
          <button @click="loginStep = 2">공식 로그인 화면 열기</button>
          <button class="ghost" @click="loginStep = 0">뒤로</button>
        </div>
      </template>
      <template v-else>
        <h3>공식 로그인 (시연)</h3>
        <p class="muted">트랜치 1에서는 실제 세션을 연결하지 않습니다. 아래 완료 버튼으로 동선만 확인합니다.</p>
        <div class="btnrow">
          <button @click="finishLogin">로그인 완료</button>
          <button class="ghost" @click="loginStep = 1">뒤로</button>
        </div>
      </template>
    </div>

    <!-- Chief: single fixed window (mock). -->
    <div v-else-if="peer === 'chief'" class="card">
      <div class="chatlog">
        <div v-for="(m, i) in chiefMsgs" :key="i" :class="m.me ? 'me' : 'them'"><p>{{ m.text }}</p></div>
        <p v-if="!chiefMsgs.length" class="muted">총괄과 연결됐습니다. 메시지를 입력하세요. (트랜치 1 화면 시연용)</p>
      </div>
      <div class="formrow">
        <label class="grow">메시지 <input v-model="draft" @keyup.enter="sendChief" /></label>
        <button @click="sendChief">보내기</button>
        <button class="ghost" @click="session = 'expired'">세션 만료 시연</button>
      </div>
    </div>

    <!-- Worker: per-agent threads (mock messages over real account list). -->
    <div v-else class="row">
      <div class="card grow">
        <div class="sect-head"><h3>에이전트 스레드</h3><span class="count">{{ accounts.length }}명</span></div>
        <p v-if="accError" class="error">{{ accError }}</p>
        <ul class="picklist">
          <li v-for="a in accounts" :key="a.id">
            <button :class="{ active: threadId === a.id }" @click="openThread(a.id)">
              <span :class="'pill p-' + (a.sessionState || 'pending')">{{ sessionLabel(a.sessionState) }}</span>
              {{ a.name }} · {{ a.platform || '미지정' }}
              <span v-if="unreadOf(a)" class="badge">새 소식</span>
            </button>
          </li>
        </ul>
        <p v-if="!accounts.length && !accError" class="empty">로그인된 에이전트가 없습니다. 에이전트 연결 화면에서 등록하세요.</p>
      </div>
      <div class="card grow2">
        <template v-if="thread">
          <div class="sect-head"><h3>{{ thread.name }}</h3><span :class="'pill p-' + (thread.sessionState || 'pending')">{{ sessionLabel(thread.sessionState) }}</span></div>
          <div class="chatlog">
            <div v-for="(m, i) in threadMsgs" :key="i" :class="m.me ? 'me' : 'them'"><p>{{ m.text }}</p></div>
          </div>
          <div v-if="thread.sessionState === 'connected'" class="formrow">
            <label class="grow">메시지 <input v-model="threadDraft" @keyup.enter="sendThread" /></label>
            <button @click="sendThread">보내기</button>
          </div>
          <p v-else class="empty">로그아웃된 에이전트입니다. 스레드는 읽기 전용으로 남습니다.</p>
        </template>
        <p v-else class="muted">왼쪽에서 에이전트를 선택하세요.</p>
      </div>
    </div>

  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiFetch } from '../stores/auth.js'

const peer = ref('chief')
const session = ref('idle') // idle | connected | expired
const loginStep = ref(0) // 0 button, 1 guidance, 2 login screen
const draft = ref('')
const chiefMsgs = ref([])
const accounts = ref([])
const accError = ref('')
const threadId = ref(null)
const threadDraft = ref('')
const threadStore = ref({})

const thread = computed(() => accounts.value.find((a) => a.id === threadId.value) || null)
const threadMsgs = computed(() => threadStore.value[threadId.value] || [])

function sessionLabel(s) {
  return { pending: '대기', connected: '연결', expired: '만료', limited: '한도', inactive_hold: '비활성 보류' }[s] || s || '—'
}

// Deterministic mock seed: expired accounts show unread news.
function unreadOf(a) {
  return a.sessionState === 'expired' || a.sessionState === 'limited'
}

function seedThread(a) {
  if (threadStore.value[a.id]) return
  const lines = [
    { me: false, text: `[${a.name} 시연] 연결 상태: ${sessionLabel(a.sessionState)}. (트랜치 1 목업)` },
  ]
  if (unreadOf(a)) {
    lines.push({ me: false, text: `[${a.name} 시연] 재로그인이 필요합니다. 채널 계정 화면에서 확인하세요.` })
  }
  threadStore.value[a.id] = lines
}

async function loadAccounts() {
  accError.value = ''
  try {
    const data = await apiFetch('/api/session/accounts')
    accounts.value = data.accounts || []
    for (const a of accounts.value) seedThread(a)
  } catch (e) {
    accError.value = e.message
  }
}

function openThread(id) {
  threadId.value = id
  threadDraft.value = ''
}

function sendThread() {
  const text = threadDraft.value.trim()
  if (!text || !thread.value) return
  threadStore.value[thread.value.id].push({ me: true, text })
  threadStore.value[thread.value.id].push({ me: false, text: `[${thread.value.name} 시연 응답] "${text}" 확인했습니다.` })
  threadDraft.value = ''
}

function relogin() {
  session.value = 'idle'
  loginStep.value = 1
}

function finishLogin() {
  loginStep.value = 0
  session.value = 'connected'
  chiefMsgs.value = []
  loadAccounts()
}

function sendChief() {
  const text = draft.value.trim()
  if (!text) return
  chiefMsgs.value.push({ me: true, text })
  chiefMsgs.value.push({ me: false, text: `[총괄 시연 응답] "${text}" 확인했습니다. (트랜치 1: 실제 에이전트 미연결)` })
  draft.value = ''
}

onMounted(() => {
  if (session.value === 'connected') loadAccounts()
})
</script>
