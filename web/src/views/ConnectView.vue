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
        <h3 class="muted small">{{ g.label }}</h3>
        <AccountRow v-for="a in g.items" :key="a.id" :account="a" :held="heldOf(a.id)" ai @chat="openChat(a)" @login="openLogin(a)" @unregister="askUnregister(a)" />
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
        <h3 class="muted small">{{ g.label }}</h3>
        <AccountRow v-for="a in g.items" :key="a.id" :account="a" :held="heldOf(a.id)" @login="openLogin(a)" @unregister="askUnregister(a)" @mode="flipMode(a)" />
        <p v-if="!g.items.length" class="muted small">아직 없습니다.</p>
      </div>
    </section>
    <LoginDialog v-if="loginDlg" :account="loginDlg" :next="nextPending" @done="onLoginDone" @close="loginDlg = null" @next="openLogin" />
    <div v-if="addDlg" class="modal-back" @click.self="addDlg = null">
      <div class="modal" role="dialog" :aria-label="addDlg.kind === 'agent_service' ? 'AI 계정 추가' : '발행 채널 추가'">
        <h3>{{ addDlg.kind === 'agent_service' ? 'AI 계정 추가' : '발행 채널 추가' }}</h3>
        <div class="form">
          <label>{{ addDlg.kind === 'agent_service' ? 'AI 종류' : '플랫폼' }}
            <select v-model="addForm.platform">
              <option value="">선택</option>
              <option v-for="p in addDlg.kind === 'agent_service' ? AI_PLATFORMS : CHANNEL_PLATFORMS" :key="p" :value="p">{{ platformLabel(p) }}</option>
            </select>
          </label>
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
              <option v-for="p in bulkDlg.kind === 'agent_service' ? AI_PLATFORMS : CHANNEL_PLATFORMS" :key="p" :value="p">{{ platformLabel(p) }}</option>
            </select>
          </label>
          <label>개수 (1~50) <input v-model.number="bulkForm.count" type="number" min="1" max="50" /></label>
          <label>주인 <input v-model="bulkForm.owner" /></label>
        </div>
        <p class="muted small">{{ bulkPreview }}</p>
        <p v-if="bulkMsg" :class="bulkOk ? 'banner okk' : 'error'">{{ bulkMsg }}</p>
        <div class="btnrow">
          <button class="ghost" @click="bulkDlg = null">닫기</button>
          <button @click="submitBulk">만들기</button>
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
const loginDlg = ref(null)
const addDlg = ref(null)
const bulkDlg = ref(null)
const unregTarget = ref(null)
const chatTarget = ref(null)
const addForm = ref({ platform: '', name: '', owner: '', role: 'worker', business: '', mode: 'manual' })
const addMsg = ref('')
const bulkForm = ref({ prefix: '', platform: '', owner: '', count: 10 })
const bulkMsg = ref('')
const bulkOk = ref(false)

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
  return label(PLATFORM, p, p)
}

function isAI(a) {
  return a.kind === 'agent_service' || AI_PLATFORMS.includes(a.platform)
}

const aiGroups = computed(() => groupAccounts(true))
const channelGroups = computed(() => groupAccounts(false))

function groupAccounts(ai) {
  const platforms = ai ? AI_PLATFORMS : CHANNEL_PLATFORMS.value
  return platforms.map((p) => ({
    platform: p,
    label: platformLabel(p),
    items: accounts.value.filter((a) => a.platform === p && isAI(a) === ai),
  })).filter((g) => g.items.length || ai)
}

const queueTotal = computed(() => queue.value.length)

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

async function load() {
  try {
    const [acc, q] = await Promise.all([
      apiFetch('/api/session/accounts'),
      apiFetch('/api/session/relogin-queue'),
    ])
    accounts.value = acc.accounts || []
    queue.value = q.items || []
  } catch (e) {
    toast(e.message || '불러오기 실패', 'error')
  }
}

function openLogin(a) {
  loginDlg.value = { id: a.id, name: a.name, platform: a.platform }
}

async function onLoginDone() {
  await load()
  try {
    const data = await apiFetch('/api/session/login-queue/next', { method: 'POST', body: JSON.stringify({}) })
    if (data.account) {
      loginDlg.value = data.account
      return
    }
  } catch { /* queue empty */
  }
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
  addMsg.value = ''
}

async function submitAdd() {
  addMsg.value = ''
  try {
    const body = {
      name: addForm.value.name.trim(), platform: addForm.value.platform,
      owner: addForm.value.owner.trim(), business: addForm.value.business.trim(),
      kind: addDlg.value.kind,
    }
    if (addDlg.value.kind === 'agent_service') body.role = addForm.value.role
    const a = await apiFetch('/api/session/accounts', { method: 'POST', body: JSON.stringify(body) })
    if (addDlg.value.kind === 'channel' && addForm.value.mode === 'auto') {
      await apiFetch(`/api/session/accounts/${a.id}`, { method: 'PATCH', body: JSON.stringify({ postingMode: 'auto' }) })
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
</style>
