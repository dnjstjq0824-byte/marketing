<template>
  <section class="page">
    <h2>대화</h2>
    <p class="muted">에이전트 슬롯이 늘어도 의장님의 대화 상대는 총괄 1자리로 고정됩니다.</p>

    <OfficeStrip :people="stripPeople" :selected-id="selectedId" @select="jumpToCard" />

    <div class="tabs">
      <button :class="{ active: peer === 'chief' }" @click="peer = 'chief'">총괄 (판정·기준·지표)</button>
      <button :class="{ active: peer === 'worker' }" @click="peer = 'worker'">작업자 (에이전트별 카드)</button>
    </div>

    <!-- Chief tab: fixed seat over the real chief account. -->
    <template v-if="peer === 'chief'">
      <div v-if="!chiefReady" class="card">
        <div class="sect-head"><h3>총괄 AI 계정이 연결되지 않았습니다</h3><span class="pill p-pending">대기</span></div>
        <p class="muted">대화보다 로그인이 먼저입니다. 에이전트 연결 화면에서 AI 서비스 계정(Meta AI 권장)을 등록·로그인하면 대화가 활성화됩니다.</p>
        <div class="btnrow">
          <button @click="$router.push('/agents')">에이전트 연결로 이동</button>
          <button class="ghost" @click="loadAll">새로고침</button>
        </div>
        <p v-if="accError" class="error">{{ accError }}</p>
      </div>
      <div v-else :id="'chatcard-' + chiefAccount.id" class="card chatcard">
        <div class="sect-head">
          <h3>{{ chiefAccount.name }} · {{ platformLabel(chiefAccount.platform) }}</h3>
          <span :class="'pill p-' + (chiefAccount.sessionState || 'pending')">{{ sessionLabel(chiefAccount.sessionState) }}</span>
        </div>
        <div class="chatlog">
          <div v-for="(m, i) in cardMsgs(chiefAccount.id)" :key="i" :class="m.me ? 'me' : 'them'"><p>{{ m.text }}</p></div>
        </div>
        <div class="formrow">
          <label class="grow">메시지 <input v-model="cardDrafts[chiefAccount.id]" @keyup.enter="sendCard(chiefAccount)" :disabled="sending[chiefAccount.id]" /></label>
          <button @click="sendCard(chiefAccount)" :disabled="sending[chiefAccount.id]">보내기</button>
        </div>
      </div>
    </template>

    <!-- Worker tab: one live card per worker account. Never gated by chief. -->
    <template v-else>
      <p v-if="accError" class="error">{{ accError }}</p>
      <div v-if="workerAccounts.length" class="cardgrid">
        <div v-for="a in workerAccounts" :key="a.id" :id="'chatcard-' + a.id" class="card chatcard" :class="{ selected: a.id === selectedId }">
          <div class="sect-head">
            <h3>{{ a.name }} · {{ platformLabel(a.platform) }}</h3>
            <span :class="'pill p-' + (a.sessionState || 'pending')">{{ sessionLabel(a.sessionState) }}</span>
          </div>
          <div class="chatlog">
            <div v-for="(m, i) in cardMsgs(a.id)" :key="i" :class="m.me ? 'me' : 'them'"><p>{{ m.text }}</p></div>
          </div>
          <div v-if="a.sessionState === 'connected'" class="formrow">
            <label class="grow">메시지 <input v-model="cardDrafts[a.id]" @keyup.enter="sendCard(a)" :disabled="sending[a.id]" /></label>
            <button @click="sendCard(a)" :disabled="sending[a.id]">보내기</button>
          </div>
          <p v-else class="empty">연결되지 않은 계정입니다. 에이전트 연결 화면에서 로그인하세요.</p>
        </div>
      </div>
      <EmptyArt v-else-if="!accError" art="plant" title="작업자 계정이 없습니다" hint="에이전트 연결 화면에서 등록하세요." />
    </template>

  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiFetch } from '../stores/auth.js'
import { PLATFORM, SESSION_STATE, label } from '../stores/labels.js'
import EmptyArt from '../components/EmptyArt.vue'
import OfficeStrip from '../components/OfficeStrip.vue'

const peer = ref('chief')
const accounts = ref([])
const agents = ref([])
const accError = ref('')
const selectedId = ref(null)
const msgStore = ref({})
const cardDrafts = ref({})
const sending = ref({})

const STATE_LABEL = { working: '일함', blocked: '막힘', done: '완료', idle: '쉼' }

function sessionLabel(s) {
  return label(SESSION_STATE, s)
}

function platformLabel(p) {
  return label(PLATFORM, p)
}

function initialOf(name) {
  return (name || '?').trim().charAt(0).toUpperCase() || '?'
}

function agentFor(accountId) {
  return agents.value.find((g) => g.accountId === accountId) || null
}

function stripState(a) {
  if (a.sessionState === 'expired' || a.sessionState === 'limited') return 'blocked'
  const g = agentFor(a.id)
  if (g && g.currentJobId) return 'working'
  return 'idle'
}

const stripPeople = computed(() =>
  accounts.value.map((a) => {
    const st = stripState(a)
    return {
      id: a.id,
      name: a.name,
      initial: initialOf(a.name),
      sub: `${platformLabel(a.platform)} · ${a.role === 'chief' ? '총괄' : '작업자'}`,
      state: st,
      stateLabel: a.sessionState === 'pending' ? '대기' : (STATE_LABEL[st] || st),
    }
  }),
)

const chiefAccount = computed(() =>
  accounts.value.find((a) => a.role === 'chief' && a.sessionState === 'connected') || null,
)
const chiefReady = computed(() => chiefAccount.value !== null)

const workerAccounts = computed(() => accounts.value.filter((a) => a.role !== 'chief'))

function cardMsgs(id) {
  return msgStore.value[id] || []
}

function seedCard(a) {
  if (msgStore.value[a.id]) return
  msgStore.value[a.id] = [{ me: false, text: `연결 상태: ${sessionLabel(a.sessionState)}.` }]
  if (a.sessionState === 'expired' || a.sessionState === 'limited') {
    msgStore.value[a.id].push({ me: false, text: '재로그인이 필요합니다. 채널 계정 화면에서 확인하세요.' })
  }
}

async function loadAll() {
  accError.value = ''
  try {
    const [acc, ag] = await Promise.all([
      apiFetch('/api/session/accounts'),
      apiFetch('/api/agents?limit=200'),
    ])
    accounts.value = acc.accounts || []
    agents.value = (ag.items || []).filter((g) => g.accountId)
    for (const a of accounts.value) {
      seedCard(a)
      if (!(a.id in cardDrafts.value)) cardDrafts.value[a.id] = ''
    }
    // Best-effort history; older servers lack the endpoint.
    for (const a of accounts.value) {
      if (a.sessionState !== 'connected') continue
      try {
        const h = await apiFetch(`/api/chat/history?accountId=${a.id}&limit=30`)
        if (h && Array.isArray(h.items) && h.items.length) {
          msgStore.value[a.id] = h.items.map((m) => ({ me: m.role !== 'assistant', text: m.text }))
        }
      } catch { /* history unavailable: keep the status seed */ }
    }
  } catch (e) {
    accError.value = e.message
  }
}

function jumpToCard(id) {
  selectedId.value = id
  const a = accounts.value.find((x) => x.id === id)
  if (a) peer.value = a.role === 'chief' ? 'chief' : 'worker'
  requestAnimationFrame(() => {
    const el = document.getElementById('chatcard-' + id)
    if (!el) return
    const calm = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches
    el.scrollIntoView({ behavior: calm ? 'auto' : 'smooth', block: 'nearest' })
  })
}

async function sendCard(a) {
  const text = (cardDrafts.value[a.id] || '').trim()
  if (!text || sending.value[a.id]) return
  sending.value[a.id] = true
  cardDrafts.value[a.id] = ''
  msgStore.value[a.id].push({ me: true, text })
  try {
    const res = await apiFetch('/api/chat/send', {
      method: 'POST',
      body: JSON.stringify({ accountId: a.id, message: text }),
    })
    if (res && res.pending) {
      sending.value[a.id] = false
      pollPending(a, res.afterId)
      return
    }
    msgStore.value[a.id].push({ me: false, text: (res && res.reply) || '(빈 응답)' })
  } catch (e) {
    const missing = e && (e.status === 404 || e.status === 501)
    msgStore.value[a.id].push({
      me: false,
      text: missing ? '실채팅 연결 준비 중입니다. 다음 단계에서 실제 답장이 옵니다.' : ('전송 실패: ' + e.message),
    })
  } finally {
    sending.value[a.id] = false
  }
}

async function pollPending(a, afterId) {
  const note = { me: false, text: '답장 기다리는 중…' }
  msgStore.value[a.id].push(note)
  const deadline = Date.now() + 6 * 60 * 1000
  while (Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, 4000))
    if (!msgStore.value[a.id]) return
    try {
      const st = await apiFetch(`/api/chat/pending?afterId=${afterId}`)
      if (!st.done) continue
      note.text = st.failed ? ('전송 실패: ' + (st.message || '알 수 없음')) : (st.reply || '(빈 응답)')
      return
    } catch (e) {
      note.text = '전송 실패: ' + e.message
      return
    }
  }
  note.text = '시간 초과: 답장이 늦어지고 있습니다. 기록 새로고침으로 확인하세요.'
}

onMounted(() => {
  loadAll()
})
</script>

<style scoped>
.cardgrid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 12px;
}
.chatcard.selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--accent-soft);
}
.chatlog {
  max-height: 260px;
  overflow-y: auto;
}
@media (max-width: 640px) {
  .cardgrid { grid-template-columns: 1fr; }
}
</style>
