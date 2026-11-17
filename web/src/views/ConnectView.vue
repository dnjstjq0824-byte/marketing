<template>
  <div class="connect">
    <h1 class="page-title">연결</h1>
    <div v-if="queueTotal > 0" class="banner" role="alert">
      <span>로그인 대기 {{ queueTotal }}건</span>
      <button @click="startQueue">이어서 로그인</button>
    </div>
    <section class="card" aria-label="AI 계정">
      <div class="sect-head">
        <h2 class="section-title">AI 계정</h2>
        <span class="sp"></span>
        <button class="ghost small" @click="openAdd('agent_service')">계정 추가</button>
        <button class="ghost small" @click="openBulk('agent_service')">여러 개 한 번에</button>
      </div>
      <div v-for="g in aiGroups" :key="g.platform" class="group">
        <h3 class="muted small">{{ g.label }} · {{ g.items.length }}기
          <span v-if="g.custom"> · 내가 추가한 AI</span>
          <span v-if="g.chatStatus === 'ok'"> · 대화 연결됨</span>
          <span v-else-if="g.chatStatus === 'unlinked'"> · 대화 미연결</span>
          <button v-if="g.custom" class="ghost small" @click="openAIEdit(g.platform)">관리</button>
        </h3>
        <AccountRow v-for="a in g.items" :key="a.id" :account="a" :held="heldOf(a.id)" :team="teamOf(a.id)" ai @chat="openChat(a)" @login="openLogin(a)" @unregister="askUnregister(a)" @probe="runProbe(a)" @plan="openPlan(a)" />
        <p v-if="!g.items.length" class="muted small">아직 없습니다.</p>
      </div>
    </section>
    <section class="card" aria-label="발행 채널">
      <div class="sect-head">
        <h2 class="section-title">발행 채널</h2>
        <span class="sp"></span>
        <button class="ghost small" @click="openAdd('channel')">채널 추가</button>
        <button class="ghost small" @click="openBulk('channel')">여러 개 한 번에</button>
      </div>
      <div v-for="g in channelGroups" :key="g.platform" class="group">
        <h3 class="muted small">{{ g.label }} · {{ g.items.length }}개</h3>
        <AccountRow v-for="a in g.items" :key="a.id" :account="a" :held="heldOf(a.id)" :team="teamOf(a.id)" @login="openLogin(a)" @unregister="askUnregister(a)" @mode="flipMode(a)" @backend="openBackend(a)" />
        <p v-if="!g.items.length" class="muted small">아직 없습니다.</p>
      </div>
    </section>
    <LoginDialog v-if="loginDlg" :key="loginDlg.id" :account="loginDlg" :next="nextPending" :real-holder="realHolderName" @done="onLoginDone" @close="loginDlg = null" @next="openLogin" />
    <div v-if="addDlg" class="modal-back" @click.self="addDlg = null">
      <div class="modal" role="dialog" :aria-label="addDlg.kind === 'agent_service' ? 'AI 계정 추가' : '발행 채널 추가'">
        <h3>{{ addDlg.kind === 'agent_service' ? 'AI 계정 추가' : '발행 채널 추가' }}</h3>
        <div class="form">
          <label>{{ addDlg.kind === 'agent_service' ? 'AI 종류' : '플랫폼' }}
            <select v-model="addForm.platform">
              <option value="">선택</option>
              <option v-for="p in addDlg.kind === 'agent_service' ? AI_OPTIONS : CHANNEL_PLATFORMS" :key="p" :value="p">{{ platformOptionLabel(p, addDlg.kind) }}</option>
              <option v-if="addDlg.kind === 'agent_service'" value="__new__">＋ 새 AI 등록</option>
            </select>
          </label>
          <template v-if="addDlg.kind === 'agent_service' && addForm.platform === '__new__'">
            <label>AI 키(영문 소문자) <input v-model="newAI.key" placeholder="예: perplexity" /></label>
            <label>이름 <input v-model="newAI.name" placeholder="예: Perplexity" /></label>
            <label>로그인 주소(https) <input v-model="newAI.loginUrl" placeholder="https://" /></label>
          </template>
          <p v-if="addDlg.kind === 'channel' && addForm.platform && registryStatus(addForm.platform) === 'preparing'" class="muted small">아직 발행 준비중입니다. 계정 등록·로그인은 먼저 할 수 있습니다.</p>
          <label>이름 <input v-model="addForm.name" :placeholder="addDlg.kind === 'agent_service' ? '예: meta-2' : '예: luno-blog-1'" /></label>
          <label>주인 <input v-model="addForm.owner" /></label>
          <label v-if="addDlg.kind === 'agent_service'">역할
            <select v-model="addForm.role"><option value="worker">작업자</option><option value="chief">총괄</option></select>
          </label>
          <label v-else>방식
            <select v-model="addForm.mode"><option value="manual">수동</option><option value="auto">자동</option></select>
          </label>
          <label>담당 사업 <input v-model="addForm.business" /></label>
        </div>
        <p v-if="addMsg" class="error">{{ addMsg }}</p>
        <div class="btnrow">
          <button class="ghost" @click="addDlg = null">취소</button>
          <button @click="submitAdd">추가</button>
        </div>
      </div>
    </div>
    <div v-if="bulkDlg" class="modal-back" @click.self="bulkDlg = null">
      <div class="modal" role="dialog" aria-label="여러 개 한 번에">
        <h3>여러 개 한 번에</h3>
        <div class="form">
          <label>이름 앞부분 <input v-model="bulkForm.prefix" placeholder="예: luno_meta_" /></label>
          <label>{{ bulkDlg.kind === 'agent_service' ? 'AI 종류' : '플랫폼' }}
            <select v-model="bulkForm.platform">
              <option value="">선택</option>
              <option v-for="p in bulkDlg.kind === 'agent_service' ? AI_OPTIONS : CHANNEL_PLATFORMS" :key="p" :value="p">{{ platformOptionLabel(p, bulkDlg.kind) }}</option>
            </select>
          </label>
          <p v-if="bulkDlg.kind === 'channel' && bulkForm.platform && registryStatus(bulkForm.platform) === 'preparing'" class="muted small">아직 발행 준비중입니다. 계정 등록·로그인은 먼저 할 수 있습니다.</p>
          <label>개수 (1~50) <input v-model.number="bulkForm.count" type="number" min="1" max="50" /></label>
          <label>주인 <input v-model="bulkForm.owner" /></label>
        </div>
        <p class="muted small">{{ bulkPreview }}</p>
        <p v-if="bulkMsg" :class="bulkOk ? 'banner okk' : 'error'">{{ bulkMsg }}</p>
        <div class="btnrow">
          <button class="ghost" @click="bulkDlg = null">닫기</button>
          <button v-if="bulkOk && bulkMade > 0" @click="bulkToQueue">이어서 로그인</button>
          <button v-else @click="submitBulk">만들기</button>
        </div>
      </div>
    </div>
    <div v-if="aiEdit" class="modal-back" @click.self="aiEdit = null">
      <div class="modal" role="dialog" :aria-label="aiEdit.name + ' 관리'">
        <h3>{{ aiEdit.name }} 관리</h3>
        <p class="muted small">대화 상태: {{ aiEdit.chatStatus === 'ok' ? '연결됨' : '미연결' }} · 로그인 주소만 바꾸면 https로 시작해야 합니다.</p>
        <div class="form">
          <label>대화 주소 <input v-model="aiEditForm.chatUrl" /></label>
          <label>입력창 선택자 <input v-model="aiEditForm.composerSelector" /></label>
          <label>보내기 선택자 <input v-model="aiEditForm.sendSelector" /></label>
          <label>답장 선택자 <input v-model="aiEditForm.replySelector" /></label>
          <label>로그아웃 표시 문구(쉼표) <input v-model="aiEditForm.markers" /></label>
        </div>
        <p v-if="aiEditMsg" :class="aiEditOk ? 'banner okk' : 'error'">{{ aiEditMsg }}</p>
        <div class="btnrow">
          <button class="ghost danger" @click="deleteAI">삭제</button>
          <span class="sp"></span>
          <button class="ghost" @click="aiEdit = null">닫기</button>
          <button @click="saveAIEdit">저장</button>
        </div>
      </div>
    </div>
    <div v-if="planDlg" class="modal-back" @click.self="planDlg = null">
      <div class="modal" role="dialog" :aria-label="planDlg.name + ' 요금제·기능'">
        <h3>{{ planDlg.name }} 요금제·기능</h3>
        <div class="form">
          <label>요금제 표시 <input v-model="planForm.tier" placeholder="예: 무료, Pro" /></label>
          <div class="checks">
            <label v-for="c in CAPS" :key="c.key" class="check"><input v-model="planForm.caps" type="checkbox" :value="c.key" /> {{ c.label }}</label>
          </div>
        </div>
        <p v-if="planMsg" class="error">{{ planMsg }}</p>
        <div class="btnrow">
          <button class="ghost" @click="planDlg = null">취소</button>
          <button @click="savePlan">저장</button>
        </div>
      </div>
    </div>
    <div v-if="backendDlg" class="modal-back" @click.self="backendDlg = null">
      <div class="modal" role="dialog" :aria-label="backendDlg.name + ' 실행 방식'">
        <h3>{{ backendDlg.name }} 실행 방식</h3>
        <p class="muted small">직접 발행은 이 계정의 세션으로 올리고, 에이전트 경유는 지정한 AI가 대신 발행합니다.</p>
        <div class="form">
          <label>실행 방식
            <select v-model="backendForm.backend">
              <option v-for="b in readyBackends" :key="b.name" :value="b.name">{{ b.name === 'direct' ? '직접 발행' : `${b.name} 경유` }}</option>
            </select>
          </label>
          <label v-if="backendForm.backend !== 'direct'">중계 AI
            <select v-model="backendForm.relay">
              <option value="">선택</option>
              <option v-for="r in relayOptions" :key="r.name" :value="r.name">{{ r.name }} · {{ r.sessionState }}</option>
            </select>
          </label>
        </div>
        <p v-if="backendMsg" class="error">{{ backendMsg }}</p>
        <div class="btnrow">
          <button class="ghost" @click="backendDlg = null">취소</button>
          <button @click="saveBackend">저장</button>
        </div>
      </div>
    </div>
    <ConfirmModal v-if="unregTarget" title="계정을 해제할까요?" :message="unregTarget.name" confirmLabel="해제" danger @confirm="doUnregister" @cancel="unregTarget = null" />
    <div v-if="chatTarget" class="drawer-back" @click.self="chatTarget = null">
      <div class="drawer" role="dialog" :aria-label="chatTarget.name + ' 대화'">
        <ChatPanel :accountId="chatTarget.id" :name="chatTarget.name" :canChat="chatTarget.sessionState === 'connected'" @close="chatTarget = null" />
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiFetch } from '../stores/auth.js'
import { toast } from '../stores/toast.js'
import { AI_PLATFORMS, label, PLATFORM, PLATFORM_GROUPS } from '../stores/labels.js'
import AccountRow from '../components/AccountRow.vue'
import ChatPanel from '../components/ChatPanel.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import LoginDialog from '../components/LoginDialog.vue'

const accounts = ref([])
const queue = ref([])
const teams = ref([])
const registry = ref({})
const aiPlatforms = ref([])
const backends = ref([])
const loginDlg = ref(null)
const addDlg = ref(null)
const bulkDlg = ref(null)
const unregTarget = ref(null)
const chatTarget = ref(null)
const aiEdit = ref(null)
const planDlg = ref(null)
const backendDlg = ref(null)
const addForm = ref({ platform: '', name: '', owner: '', role: 'worker', business: '', mode: 'manual' })
const addMsg = ref('')
const newAI = ref({ key: '', name: '', loginUrl: '' })
const bulkForm = ref({ prefix: '', platform: '', owner: '', count: 10 })
const bulkMsg = ref('')
const bulkOk = ref(false)
const bulkMade = ref(0)
const aiEditForm = ref({ chatUrl: '', composerSelector: '', sendSelector: '', replySelector: '', markers: '' })
const aiEditMsg = ref('')
const aiEditOk = ref(false)
const planForm = ref({ tier: '', caps: [] })
const planMsg = ref('')
const backendForm = ref({ backend: 'direct', relay: '' })
const backendMsg = ref('')
const CAPS = [
  { key: 'text', label: '텍스트' },
  { key: 'image', label: '이미지 생성' },
  { key: 'video', label: '영상 생성' },
  { key: 'file', label: '파일 첨부' },
]

const CHANNEL_PLATFORMS = computed(() => {
  const out = []
  for (const g of PLATFORM_GROUPS) {
    for (const p of g.options) {
      if (!AI_PLATFORMS.includes(p)) out.push(p)
    }
  }
  return [...new Set(out)]
})

function platformLabel(p) {
  const ai = aiPlatforms.value.find((x) => x.key === p)
  if (ai && ai.name) return ai.name
  return label(PLATFORM, p, p)
}

// Server AI list drives the UI; the bundled list is the offline fallback.
const AI_OPTIONS = computed(() => {
  if (aiPlatforms.value.length) return aiPlatforms.value.map((x) => x.key)
  return AI_PLATFORMS
})

function registryStatus(p) {
  return registry.value[p] || ''
}

function platformOptionLabel(p, kind) {
  if (kind !== 'channel') return platformLabel(p)
  const st = registryStatus(p)
  if (st === 'ready') return `${platformLabel(p)} (사용가능)`
  if (st === 'preparing') return `${platformLabel(p)} (준비중)`
  return platformLabel(p)
}

function isAI(a) {
  if (a.kind === 'agent_service') return true
  if (aiPlatforms.value.length) return aiPlatforms.value.some((x) => x.key === a.platform)
  return AI_PLATFORMS.includes(a.platform)
}

const aiGroups = computed(() => groupAccounts(true))
const channelGroups = computed(() => groupAccounts(false))

function groupAccounts(ai) {
  const platforms = ai ? AI_OPTIONS.value : CHANNEL_PLATFORMS.value
  return platforms.map((p) => {
    const meta = aiPlatforms.value.find((x) => x.key === p) || {}
    return {
      platform: p,
      label: platformLabel(p),
      custom: ai && meta.builtin === false,
      chatStatus: ai ? meta.chatStatus || '' : '',
      items: accounts.value.filter((a) => a.platform === p && isAI(a) === ai),
    }
  }).filter((g) => g.items.length || ai)
}

const realHolderName = computed(() => {
  if (!loginDlg.value) return ''
  const holder = accounts.value.find((a) => a.profileMode === 'real' && a.id !== loginDlg.value.id)
  return holder ? holder.name : ''
})

const queueTotal = computed(() => accounts.value.filter((a) => ['pending', 'expired', 'limited'].includes(a.sessionState)).length)

const nextPending = computed(() => {
  if (!loginDlg.value) return null
  const pool = accounts.value.filter((a) => a.id !== loginDlg.value.id && ['pending', 'expired', 'limited'].includes(a.sessionState))
  return pool.find((a) => a.platform === loginDlg.value.platform) || pool[0] || null
})

const bulkPreview = computed(() => {
  const p = bulkForm.value.prefix
  const n = bulkForm.value.count
  if (!p || !n || n < 1) return ''
  const pad = (i) => String(i).padStart(2, '0')
  return n === 1 ? `→ ${p}01` : `→ ${p}01 ~ ${p}${pad(Math.min(n, 50))}`
})

const readyBackends = computed(() => {
  const ready = backends.value.filter((b) => b.ready)
  return ready.length ? ready : [{ name: 'direct', aiPlatform: '', ready: true }]
})

const relayOptions = computed(() => {
  const be = backends.value.find((b) => b.name === backendForm.value.backend)
  if (!be || !be.aiPlatform) return []
  return accounts.value.filter((a) => a.kind === 'agent_service' && a.platform === be.aiPlatform)
})

async function load() {
  try {
    const [acc, q, t, reg, ai, be] = await Promise.all([
      apiFetch('/api/session/accounts'),
      apiFetch('/api/session/relogin-queue'),
      apiFetch('/api/teams').catch(() => ({ items: [] })),
      apiFetch('/api/channel-platforms').catch(() => ({ items: [] })),
      apiFetch('/api/ai-platforms').catch(() => ({ items: [] })),
      apiFetch('/api/publish-backends').catch(() => ({ backends: [] })),
    ])
    accounts.value = acc.accounts || []
    queue.value = q.items || []
    teams.value = t.items || []
    backends.value = be.backends || []
    const m = {}
    for (const p of reg.items || []) m[p.key] = p.status
    registry.value = m
    aiPlatforms.value = ai.items || []
  } catch (e) {
    toast(e.message || '불러오기 실패', 'error')
  }
}

function teamOf(id) {
  for (const c of teams.value) {
    const t = c.team || {}
    const inMembers = (c.memberIds || []).includes(id)
    const inChannels = (c.channelIds || []).includes(id)
    if (inMembers || inChannels) {
      return { id: t.id, name: t.name, leader: t.leaderAccountId === id }
    }
  }
  return null
}

function openLogin(a) {
  loginDlg.value = { id: a.id, name: a.name, platform: a.platform }
}

async function onLoginDone() {
  // 목록만 새로고침한다. 다음 계정으로 자동 교체하지 않는다:
  // 완료 다이얼로그가 다른 계정으로 바뀌어 보이는 버그의 원인이었다.
  // 이어서 로그인은 다이얼로그의 [다음 계정 로그인] 버튼이 맡는다.
  await load()
}

async function startQueue() {
  try {
    const data = await apiFetch('/api/session/login-queue/next', { method: 'POST', body: JSON.stringify({}) })
    if (data.account) openLogin(data.account)
    else toast('대기 중인 로그인이 없습니다')
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

function openChat(a) {
  chatTarget.value = a
}

function heldOf(id) {
  const q = queue.value.find((x) => x.accountId === id)
  return q ? q.heldCount || 0 : 0
}

function openAdd(kind) {
  addDlg.value = { kind }
  addForm.value = { platform: '', name: '', owner: '', role: 'worker', business: '', mode: 'manual' }
  newAI.value = { key: '', name: '', loginUrl: '' }
  addMsg.value = ''
}

async function submitAdd() {
  addMsg.value = ''
  try {
    let platform = addForm.value.platform
    if (addDlg.value.kind === 'agent_service' && platform === '__new__') {
      const created = await apiFetch('/api/ai-platforms', {
        method: 'POST',
        body: JSON.stringify({
          key: newAI.value.key.trim(), name: newAI.value.name.trim(),
          loginUrl: newAI.value.loginUrl.trim(),
        }),
      })
      platform = created.key
      await load()
    }
    const body = {
      name: addForm.value.name.trim(), platform,
      owner: addForm.value.owner.trim(), business: addForm.value.business.trim(),
      kind: addDlg.value.kind,
    }
    if (addDlg.value.kind === 'agent_service') body.role = addForm.value.role
    const a = await apiFetch('/api/session/accounts', { method: 'POST', body: JSON.stringify(body) })
    if (addDlg.value.kind === 'channel') {
      // New channels default to auto (W0); honor an explicit manual choice.
      await apiFetch(`/api/session/accounts/${a.id}`, { method: 'PATCH', body: JSON.stringify({ postingMode: addForm.value.mode }) })
    }
    addDlg.value = null
    toast('계정을 만들었습니다. 로그인하세요.')
    await load()
  } catch (e) {
    addMsg.value = e.message
  }
}

function openBulk(kind) {
  bulkDlg.value = { kind }
  bulkForm.value = { prefix: '', platform: '', owner: '', count: 10 }
  bulkMsg.value = ''
  bulkOk.value = false
  bulkMade.value = 0
}

async function bulkToQueue() {
  const platform = bulkForm.value.platform
  bulkDlg.value = null
  try {
    const data = await apiFetch('/api/session/login-queue/next', { method: 'POST', body: JSON.stringify({ platform }) })
    if (data.account) openLogin(data.account)
    else toast('대기 중인 로그인이 없습니다')
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

async function submitBulk() {
  bulkMsg.value = ''
  try {
    const data = await apiFetch('/api/session/accounts/bulk', {
      method: 'POST',
      body: JSON.stringify({
        prefix: bulkForm.value.prefix, platform: bulkForm.value.platform,
        owner: bulkForm.value.owner, count: bulkForm.value.count, kind: bulkDlg.value.kind,
      }),
    })
    const made = (data.accounts || []).length
    const fails = data.failures || []
    bulkOk.value = true
    bulkMade.value = made
    bulkMsg.value = `${made}개 만들었습니다.` + (fails.length ? ` (건너뜀 ${fails.length}개)` : '') + ' 차례로 로그인하세요.'
    await load()
  } catch (e) {
    bulkOk.value = false
    bulkMsg.value = e.message
  }
}

function askUnregister(a) {
  unregTarget.value = a
}

async function doUnregister() {
  const a = unregTarget.value
  unregTarget.value = null
  try {
    await apiFetch(`/api/session/accounts/${a.id}`, { method: 'DELETE' })
    toast('해제했습니다')
    await load()
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

async function flipMode(a) {
  try {
    await apiFetch(`/api/session/accounts/${a.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ postingMode: a.postingMode === 'auto' ? 'manual' : 'auto' }),
    })
    await load()
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

function openAIEdit(key) {
  const p = aiPlatforms.value.find((x) => x.key === key)
  if (!p) return
  aiEdit.value = p
  let markers = ''
  try {
    markers = (JSON.parse(p.loggedOutMarkers || '[]') || []).join(', ')
  } catch { markers = '' }
  aiEditForm.value = {
    chatUrl: p.chatUrl || '', composerSelector: p.composerSelector || '',
    sendSelector: p.sendSelector || '', replySelector: p.replySelector || '',
    markers,
  }
  aiEditMsg.value = ''
  aiEditOk.value = false
}

async function saveAIEdit() {
  aiEditMsg.value = ''
  try {
    const markers = aiEditForm.value.markers.split(',').map((s) => s.trim()).filter(Boolean)
    await apiFetch(`/api/ai-platforms/${aiEdit.value.key}`, {
      method: 'PATCH',
      body: JSON.stringify({
        chatUrl: aiEditForm.value.chatUrl.trim(),
        composerSelector: aiEditForm.value.composerSelector.trim(),
        sendSelector: aiEditForm.value.sendSelector.trim(),
        replySelector: aiEditForm.value.replySelector.trim(),
        loggedOutMarkers: JSON.stringify(markers),
      }),
    })
    aiEditOk.value = true
    aiEditMsg.value = '저장했습니다. 대화 연결 테스트로 확인하세요.'
    await load()
  } catch (e) {
    aiEditOk.value = false
    aiEditMsg.value = e.message
  }
}

async function deleteAI() {
  aiEditMsg.value = ''
  try {
    await apiFetch(`/api/ai-platforms/${aiEdit.value.key}`, { method: 'DELETE' })
    aiEdit.value = null
    toast('삭제했습니다')
    await load()
  } catch (e) {
    aiEditOk.value = false
    aiEditMsg.value = e.message
  }
}

async function runProbe(a) {
  try {
    toast('대화 연결 테스트 중... (최대 3분)')
    const res = await apiFetch(`/api/session/accounts/${a.id}/chat-probe`, { method: 'POST' })
    if (res.ok) toast('대화 연결됨')
    else toast(res.reason || '대화 미연결', 'error')
    await load()
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

function openPlan(a) {
  planDlg.value = a
  let caps = []
  try {
    const arr = JSON.parse(a.capabilities || '[]')
    if (Array.isArray(arr)) caps = arr
  } catch { caps = [] }
  planForm.value = { tier: a.planTier || '', caps }
  planMsg.value = ''
}

async function savePlan() {
  planMsg.value = ''
  try {
    await apiFetch(`/api/session/accounts/${planDlg.value.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ planTier: planForm.value.tier, capabilities: JSON.stringify(planForm.value.caps) }),
    })
    planDlg.value = null
    toast('저장했습니다')
    await load()
  } catch (e) {
    planMsg.value = e.message
  }
}

function openBackend(a) {
  backendDlg.value = a
  backendForm.value = { backend: a.executorBackend || 'direct', relay: a.relayAccount || '' }
  backendMsg.value = ''
}

async function saveBackend() {
  backendMsg.value = ''
  try {
    const be = backendForm.value.backend
    await apiFetch(`/api/session/accounts/${backendDlg.value.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ executorBackend: be, relayAccount: be === 'direct' ? '' : backendForm.value.relay }),
    })
    backendDlg.value = null
    toast('저장했습니다')
    await load()
  } catch (e) {
    backendMsg.value = e.message
  }
}

onMounted(load)
</script>

<style scoped>
.connect { display: flex; flex-direction: column; gap: var(--s4); }
.banner {
  background: var(--hold-soft);
  border: 1px solid var(--gold);
  border-radius: var(--r-lg);
  padding: var(--s3) var(--s4);
  display: flex;
  align-items: center;
  gap: var(--s3);
}
.banner button {
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: var(--r-md);
  padding: var(--s2) var(--s4);
  font-weight: 700;
  margin-left: auto;
}
.card {
  background: var(--panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-lg);
  padding: var(--s4);
  box-shadow: var(--shadow);
  display: flex;
  flex-direction: column;
  gap: var(--s3);
}
.sect-head { display: flex; align-items: center; gap: var(--s2); }
.sect-head h2 { flex: 1; }
.sect-head .sp { flex: 1; }
.group h3 { margin: 0 0 var(--s2); }
.form { display: flex; flex-direction: column; gap: var(--s3); }
.form label { display: flex; flex-direction: column; gap: var(--s1); font-weight: 700; }
.form input, .form select {
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: var(--s2) var(--s3);
  background: var(--panel);
  font-weight: 400;
  width: 100%;
}
.modal-back {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 150;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--s4);
}
.modal {
  background: var(--panel);
  border-radius: var(--r-lg);
  box-shadow: var(--shadow-lg);
  padding: var(--s5);
  max-width: 480px;
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: var(--s3);
  max-height: calc(100vh - 64px);
  overflow-y: auto;
}
.modal h3 { margin: 0; }
.btnrow { display: flex; gap: var(--s2); justify-content: flex-end; }
.btnrow .sp { flex: 1; }
.checks { display: flex; gap: var(--s3); flex-wrap: wrap; }
.check { display: flex; align-items: center; gap: var(--s1); font-weight: 400; }
.check input { width: auto; }
button.danger { color: var(--warn); }
.btnrow button, .ghost {
  border-radius: var(--r-md);
  padding: var(--s2) var(--s4);
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
}
.btnrow button:not(.ghost) { background: var(--accent); border-color: var(--accent); color: #fff; font-weight: 700; }
.small { font-size: 13px; padding: var(--s1) var(--s2); }
.error { color: var(--warn); }
.drawer-back {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 140;
  display: flex;
  justify-content: flex-end;
}
.drawer { width: min(480px, 100%); background: var(--panel); height: 100%; box-shadow: var(--shadow-lg); }
@media (max-width: 900px) {
  .drawer { width: 100%; }
}
@media (min-width: 1150px) {
  .connect { display: grid; grid-template-columns: 1fr 1fr; align-items: start; }
  .connect > h1, .connect > .banner { grid-column: 1 / -1; }
}
</style>
