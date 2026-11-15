<template>
  <div class="teamdetail">
    <div class="page-head">
      <RouterLink to="/teams" class="back">← 팀</RouterLink>
      <h1 class="page-title">{{ team.name }} <span v-if="bizLine" class="muted sub">· {{ bizLine }}</span></h1>
      <span class="sp"></span>
      <label class="switch"><input type="checkbox" :checked="team.enabled" @change="flipEnabled" /> {{ team.enabled ? '가동' : '멈춤' }}</label>
      <div class="menuwrap">
        <button class="ghost" @click="menuOpen = !menuOpen" aria-label="팀 메뉴">⋯</button>
        <div v-if="menuOpen" class="menu" role="menu">
          <button role="menuitem" @click="menuOpen = false; renameOpen = true">이름 변경</button>
          <button v-if="!team.isDefault" role="menuitem" class="danger-t" @click="menuOpen = false; delOpen = true">팀 삭제</button>
        </div>
      </div>
    </div>
    <p v-if="loadMsg" class="error">{{ loadMsg }}</p>
    <p class="muted summary">에이전트 {{ members.length }}기 · 일하는 중 {{ working }} · 대기 {{ idle }} · 동시 실행 최대 {{ team.maxConcurrency || 0 }}</p>
    <p class="muted summary">오늘 처리 {{ stats.todayDone }} · 발행 {{ stats.todayPublished }} · 실패 {{ stats.todayFailed }} · 이번 주 조회 {{ weekViews }}</p>
    <div class="tabbar" role="tablist" aria-label="팀 정보">
      <button v-for="t in tabs" :key="t.key" role="tab" :aria-selected="tab === t.key" :class="{ on: tab === t.key }" @click="tab = t.key">{{ t.label }}</button>
    </div>
    <div class="panels">
      <section class="card panel" :class="{ active: tab === 'agents' }" aria-label="에이전트">
        <div class="sect-head">
          <h2 class="section-title">에이전트</h2>
          <span class="sp"></span>
          <button class="ghost small" @click="openAdd">＋ 에이전트 추가</button>
        </div>
        <ul class="rows">
          <li v-for="m in members" :key="m.id" class="mrow">
            <span class="ava sm" aria-hidden="true">{{ faceLetter(m.platform) }}</span>
            <div class="minfo">
              <p class="mname">{{ m.name }} <span v-if="m.isLeader" class="leader">팀장</span></p>
              <p class="muted small">{{ memberStatus(m) }} · 오늘 {{ m.todayDone }}</p>
              <div v-if="team.roleMode === 'manual'" class="rolechips">
                <button v-for="r in roleKeys" :key="r" class="chip" :class="{ on: (m.fixedRole || '').split(',').includes(r) }" @click="toggleRole(m, r)">{{ roleLabel(r) }}</button>
              </div>
            </div>
            <div class="menuwrap">
              <button class="ghost small" @click="rowMenu = rowMenu === m.id ? null : m.id" :aria-label="m.name + ' 메뉴'">⋯</button>
              <div v-if="rowMenu === m.id" class="menu" role="menu">
                <button v-if="!m.isLeader" role="menuitem" @click="setLeader(m.id)">팀장으로 지정</button>
                <button v-else role="menuitem" @click="setLeader(null)">팀장 해제</button>
                <button role="menuitem" @click="openMove(m)">팀 옮기기</button>
                <button v-if="!team.isDefault" role="menuitem" @click="sendToDefault(m)">기본 팀으로 보내기</button>
              </div>
            </div>
          </li>
        </ul>
        <p v-if="!members.length" class="muted small">에이전트가 없습니다. 추가해 보세요.</p>
      </section>
      <section class="card panel" :class="{ active: tab === 'works' }" aria-label="진행 중인 작업">
        <div class="sect-head">
          <h2 class="section-title">진행 중인 작업</h2>
          <span class="sp"></span>
          <RouterLink :to="assignLink" class="ghost small link">이 팀에 맡기기</RouterLink>
          <RouterLink to="/works" class="small">전체 →</RouterLink>
        </div>
        <ul class="rows">
          <li v-for="w in works" :key="w.id">
            <RouterLink :to="'/works/' + w.id" class="wrow">
              <img v-if="w.thumb" :src="w.thumb" alt="" class="thumb" />
              <div class="minfo">
                <p class="mname">{{ w.title }}</p>
                <p class="muted small">{{ stageLabel(w.stage) }} · {{ w.detail }} · {{ w.ago }}</p>
              </div>
            </RouterLink>
          </li>
        </ul>
        <p v-if="!works.length" class="muted small">진행 중인 작업이 없습니다.</p>
      </section>
      <section class="card panel" :class="{ active: tab === 'brand' }" aria-label="브랜드 정보">
        <div class="sect-head">
          <h2 class="section-title">브랜드 정보</h2>
          <span class="sp"></span>
          <button v-if="!brandEdit" class="ghost small" @click="startBrandEdit">수정</button>
        </div>
        <div v-if="!brandEdit" class="brandview">
          <p>소개: {{ brand.intro || '—' }}</p>
          <p class="muted">말투: {{ brand.tone || '—' }} · 금칙어: {{ brand.bannedWords || '—' }}</p>
          <p class="muted small">기본 구매 링크: {{ brand.buyLink || '—' }}</p>
        </div>
        <div v-else class="form">
          <label>소개<textarea v-model.trim="brandForm.intro" rows="2" /></label>
          <label>말투<input v-model.trim="brandForm.tone" /></label>
          <label>금칙어<input v-model.trim="brandForm.banned" /></label>
          <label>기본 구매 링크<input v-model.trim="brandForm.buyLink" /></label>
          <div class="btnrow">
            <button class="ghost" @click="brandEdit = false">취소</button>
            <button @click="saveBrand">저장</button>
          </div>
        </div>
      </section>
      <section class="card panel" :class="{ active: tab === 'brand' }" aria-label="발행 채널">
        <div class="sect-head">
          <h2 class="section-title">발행 채널</h2>
          <span class="sp"></span>
          <button class="ghost small" @click="openChannels">변경</button>
        </div>
        <ul class="rows">
          <li v-for="c in channels" :key="c.id" class="mrow">
            <span class="dot" :class="{ on: c.sessionState === 'connected' }" aria-hidden="true"></span>
            <div class="minfo">
              <p class="mname">{{ c.name }}</p>
              <p class="muted small">{{ platformLabel(c.platform) }} · {{ modeLabel(c.postingMode) }}</p>
            </div>
          </li>
        </ul>
        <p v-if="!channels.length" class="muted small">연결된 전 채널로 발행합니다.</p>
      </section>
      <section class="card panel wide" :class="{ active: tab === 'settings' }" aria-label="팀 설정">
        <h2 class="section-title">팀 설정</h2>
        <div class="setrow">
          <span>업무 분담</span>
          <label class="chk"><input type="radio" value="auto" v-model="settingsForm.roleMode" @change="saveSettings" /> 자동</label>
          <label class="chk"><input type="radio" value="manual" v-model="settingsForm.roleMode" @change="saveSettings" /> 직접 지정</label>
        </div>
        <div class="setrow">
          <label>동시 실행 최대 <input v-model.number="settingsForm.maxConcurrency" type="number" min="0" max="99" class="num" /></label>
          <button class="ghost small" @click="saveSettings">저장</button>
        </div>
        <div class="setrow">
          <span>계정 분배</span>
          <label class="chk"><input type="radio" value="all" v-model="settingsForm.distributionMode" @change="saveSettings" /> 계정마다 다른 글로 모두 올리기</label>
          <label class="chk"><input type="radio" value="one" v-model="settingsForm.distributionMode" @change="saveSettings" /> 계정 1개에만 올리기</label>
        </div>
        <div class="setrow">
          <label>분배 상한 <input v-model.number="settingsForm.distributionCap" type="number" min="1" max="50" class="num" /></label>
          <button class="ghost small" @click="saveSettings">저장</button>
        </div>
      </section>
    </div>
    <div v-if="renameOpen" class="modal-back" @click.self="renameOpen = false">
      <div class="modal" role="dialog" aria-label="이름 변경">
        <h3>이름 변경</h3>
        <div class="form">
          <label>팀 이름<input v-model.trim="renameForm.name" /></label>
          <label>전담 사업<input v-model.trim="renameForm.business" placeholder="비우면 전담 해제" /></label>
          <label>설명<input v-model.trim="renameForm.description" /></label>
        </div>
        <p v-if="renameMsg" class="error">{{ renameMsg }}</p>
        <div class="btnrow">
          <button class="ghost" @click="renameOpen = false">취소</button>
          <button @click="saveRename">저장</button>
        </div>
      </div>
    </div>
    <ConfirmModal v-if="delOpen" title="팀을 삭제할까요?" :message="delMessage" confirmLabel="삭제" danger @confirm="doDelete" @cancel="delOpen = false" />
    <ConfirmModal v-if="stopOpen" title="이 팀을 멈출까요?" :message="stopMessage" confirmLabel="멈추기" danger @confirm="doFlip(false)" @cancel="stopOpen = false" />
    <div v-if="addOpen" class="modal-back" @click.self="addOpen = false">
      <div class="modal wide" role="dialog" aria-label="에이전트 추가">
        <h3>에이전트 추가</h3>
        <ul class="picklist">
          <li v-for="a in addableAgents" :key="a.id">
            <label class="chk">
              <input type="checkbox" :value="a.id" v-model="addIds" />
              <span class="pname">{{ a.name }}</span>
              <span class="muted small">{{ platformLabel(a.platform) }}</span>
              <span v-if="memberTeamName(a.id)" class="muted small">{{ memberTeamName(a.id) }} → 이동</span>
            </label>
          </li>
        </ul>
        <p class="muted small"><RouterLink to="/connect">새 AI 계정 로그인 →</RouterLink></p>
        <p v-if="addMsg" class="error">{{ addMsg }}</p>
        <div class="btnrow">
          <button class="ghost" @click="addOpen = false">취소</button>
          <button @click="submitAdd">추가</button>
        </div>
      </div>
    </div>
    <div v-if="moveTarget" class="modal-back" @click.self="moveTarget = null">
      <div class="modal" role="dialog" aria-label="팀 옮기기">
        <h3>{{ moveTarget.name }} 옮기기</h3>
        <div class="form">
          <label>이동할 팀
            <select v-model.number="moveTeamId">
              <option v-for="t in otherTeams" :key="t.id" :value="t.id">{{ t.name }}</option>
            </select>
          </label>
        </div>
        <p class="muted small">하던 작업은 끝내고 넘어옵니다.</p>
        <p v-if="moveMsg" class="error">{{ moveMsg }}</p>
        <div class="btnrow">
          <button class="ghost" @click="moveTarget = null">취소</button>
          <button @click="submitMove">옮기기</button>
        </div>
      </div>
    </div>
    <div v-if="chanOpen" class="modal-back" @click.self="chanOpen = false">
      <div class="modal" role="dialog" aria-label="발행 채널 변경">
        <h3>발행 채널</h3>
        <ul class="picklist">
          <li v-for="a in channelAccounts" :key="a.id">
            <label class="chk">
              <input type="checkbox" :value="a.id" v-model="chanIds" />
              <span class="pname">{{ a.name }}</span>
              <span class="muted small">{{ platformLabel(a.platform) }}</span>
            </label>
          </li>
        </ul>
        <p v-if="!channelAccounts.length" class="muted small">연결된 채널이 없습니다.</p>
        <p v-if="chanMsg" class="error">{{ chanMsg }}</p>
        <div class="btnrow">
          <button class="ghost" @click="chanOpen = false">취소</button>
          <button @click="submitChannels">저장</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiFetch } from '../stores/auth.js'
import { toast } from '../stores/toast.js'
import { label, PLATFORM, POSTING_MODE, TEAM_ROLE, WORK_STAGE } from '../stores/labels.js'
import ConfirmModal from '../components/ConfirmModal.vue'

const route = useRoute()
const router = useRouter()
const team = ref({})
const brand = ref({})
const members = ref([])
const works = ref([])
const channels = ref([])
const stats = ref({ todayDone: 0, todayPublished: 0, todayFailed: 0, runningWorks: 0, weekViews: 0 })
const teams = ref([])
const accounts = ref([])
const memberTeams = ref({})
const loadMsg = ref('')
const tab = ref('agents')
const tabs = [
  { key: 'agents', label: '에이전트' },
  { key: 'works', label: '작업' },
  { key: 'brand', label: '브랜드·채널' },
  { key: 'settings', label: '설정' },
]
const menuOpen = ref(false)
const rowMenu = ref(null)
const renameOpen = ref(false)
const renameForm = ref({ name: '', business: '', description: '' })
const renameMsg = ref('')
const delOpen = ref(false)
const stopOpen = ref(false)
const addOpen = ref(false)
const addIds = ref([])
const addMsg = ref('')
const moveTarget = ref(null)
const moveTeamId = ref(0)
const moveMsg = ref('')
const chanOpen = ref(false)
const chanIds = ref([])
const chanMsg = ref('')
const brandEdit = ref(false)
const brandForm = ref({ intro: '', tone: '', banned: '', buyLink: '' })
const settingsForm = ref({ roleMode: 'auto', maxConcurrency: 0, distributionMode: 'all', distributionCap: 5 })
const roleKeys = ['analyze', 'strategy', 'generate', 'review']

const id = computed(() => route.params.id)
const bizLine = computed(() => {
  const raw = team.value.businesses || ''
  const list = raw.split(',').map((s) => s.trim()).filter(Boolean)
  return list.length ? list.join(' · ') + ' 전담' : ''
})
const assignLink = computed(() => {
  const raw = team.value.businesses || ''
  const first = raw.split(',').map((s) => s.trim()).filter(Boolean)[0] || ''
  const query = { team: String(team.value.id || '') }
  if (first) query.business = first
  return { path: '/new', query }
})
const working = computed(() => Math.min(stats.value.runningWorks || 0, members.value.length))
const idle = computed(() => members.value.length - working.value)
const weekViews = computed(() => {
  const v = stats.value.weekViews || 0
  return v >= 1000 ? (v / 1000).toFixed(1) + 'k' : String(v)
})
const defaultTeam = computed(() => teams.value.find((t) => t.isDefault))
const otherTeams = computed(() => teams.value.filter((t) => String(t.id) !== String(id.value)))
const agentAccounts = computed(() => accounts.value.filter((a) => a.kind === 'agent_service'))
const channelAccounts = computed(() => accounts.value.filter((a) => a.kind === 'channel'))
const addableAgents = computed(() => {
  const mine = new Set(members.value.map((m) => m.id))
  return agentAccounts.value.filter((a) => !mine.has(a.id))
})
const delMessage = computed(() => `에이전트 ${members.value.length}기와 진행 작업 ${stats.value.runningWorks}건이 기본 팀으로 옮겨집니다`)
const stopMessage = computed(() => `${team.value.name}의 진행 작업 ${stats.value.runningWorks}건이 멈춥니다. 다른 팀은 계속 일합니다.`)

function platformLabel(p) { return label(PLATFORM, p, p) }
function roleLabel(r) { return label(TEAM_ROLE, r, r) }
function modeLabel(m) { return label(POSTING_MODE, m, m) }
function stageLabel(s) { return label(WORK_STAGE, s, s) }
function faceLetter(p) { return (platformLabel(p) || '?').slice(0, 1) }
function memberTeamName(aid) { return memberTeams.value[aid] || '' }
function memberStatus(m) {
  const fixed = (m.fixedRole || '').split(',').filter(Boolean).map(roleLabel).join('·')
  const state = { connected: '대기', limited: '사용 한도', expired: '로그인 필요', pending: '로그인 필요', inactive_hold: '꺼둠' }[m.sessionState] || m.sessionState
  return (team.value.roleMode === 'manual' && fixed ? fixed + ' · ' : '') + state
}

async function load() {
  loadMsg.value = ''
  try {
    const d = await apiFetch(`/api/teams/${id.value}`)
    team.value = d.team || {}
    brand.value = d.brand || {}
    members.value = d.members || []
    works.value = d.works || []
    channels.value = d.channels || []
    stats.value = d.stats || stats.value
    settingsForm.value = { roleMode: team.value.roleMode || 'auto', maxConcurrency: team.value.maxConcurrency || 0, distributionMode: team.value.distributionMode || 'all', distributionCap: team.value.distributionCap || 5 }
  } catch (e) {
    loadMsg.value = e.message || '팀을 불러오지 못했습니다'
  }
  try {
    const data = await apiFetch('/api/teams')
    teams.value = (data.items || []).map((c) => c.team)
  } catch { /* move targets stay empty */ }
}

async function ensureAccounts() {
  if (accounts.value.length) return
  const data = await apiFetch('/api/session/accounts')
  accounts.value = data.accounts || []
  const names = {}
  await Promise.all(teams.value.map(async (t) => {
    try {
      const d = await apiFetch(`/api/teams/${t.id}`)
      for (const m of d.members || []) names[m.id] = t.name
    } catch { /* blank */ }
  }))
  memberTeams.value = names
}

function flipEnabled(e) {
  const on = e.target.checked
  e.target.checked = team.value.enabled
  if (!on && (stats.value.runningWorks || 0) > 0) {
    stopOpen.value = true
    return
  }
  doFlip(on)
}

async function doFlip(on) {
  stopOpen.value = false
  try {
    team.value = await apiFetch(`/api/teams/${id.value}`, { method: 'PATCH', body: JSON.stringify({ enabled: on }) })
    toast(on ? '팀을 가동합니다' : '팀을 멈췄습니다')
  } catch (e) {
    toast(e.message || '저장 실패', 'error')
  }
}

async function setLeader(accountId) {
  rowMenu.value = null
  try {
    team.value = await apiFetch(`/api/teams/${id.value}/leader`, { method: 'POST', body: JSON.stringify({ accountId }) })
    await load()
    toast(accountId ? '팀장을 지정했습니다. 팀장은 검수를 맡습니다' : '팀장을 해제했습니다')
  } catch (e) {
    toast(e.message || '저장 실패', 'error')
  }
}

async function toggleRole(m, role) {
  const cur = (m.fixedRole || '').split(',').filter(Boolean)
  const next = cur.includes(role) ? cur.filter((r) => r !== role) : [...cur, role]
  try {
    await apiFetch(`/api/teams/${id.value}/members/${m.id}/roles`, { method: 'POST', body: JSON.stringify({ roles: next }) })
    m.fixedRole = next.join(',')
  } catch (e) {
    toast(e.message || '저장 실패', 'error')
  }
}

async function openAdd() {
  addIds.value = []
  addMsg.value = ''
  addOpen.value = true
  try {
    await ensureAccounts()
  } catch (e) {
    addMsg.value = e.message || '계정 목록을 불러오지 못했습니다'
  }
}

async function submitAdd() {
  addMsg.value = ''
  const list = [
    ...members.value.map((m) => ({ accountId: m.id, roles: (m.fixedRole || '').split(',').filter(Boolean) })),
    ...addIds.value.map((accountId) => ({ accountId, roles: [] })),
  ]
  try {
    await apiFetch(`/api/teams/${id.value}/members`, { method: 'PUT', body: JSON.stringify({ members: list }) })
    toast(`${addIds.value.length}기를 추가했습니다`)
    addOpen.value = false
    await load()
  } catch (e) {
    addMsg.value = e.message || '추가 실패'
  }
}

function openMove(m) {
  rowMenu.value = null
  moveTarget.value = m
  moveMsg.value = ''
  const first = otherTeams.value[0]
  moveTeamId.value = first ? first.id : 0
}

async function submitMove() {
  moveMsg.value = ''
  if (!moveTeamId.value) {
    moveMsg.value = '이동할 팀이 없습니다'
    return
  }
  try {
    await apiFetch(`/api/teams/${moveTeamId.value}/members`, { method: 'POST', body: JSON.stringify({ accountId: moveTarget.value.id }) })
    toast(`${moveTarget.value.name}을(를) 옮겼습니다`)
    moveTarget.value = null
    await load()
  } catch (e) {
    moveMsg.value = e.message || '옮기기 실패'
  }
}

async function sendToDefault(m) {
  rowMenu.value = null
  if (!defaultTeam.value) return
  try {
    await apiFetch(`/api/teams/${defaultTeam.value.id}/members`, { method: 'POST', body: JSON.stringify({ accountId: m.id }) })
    toast(`${m.name}을(를) 기본 팀으로 보냈습니다`)
    await load()
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

function startBrandEdit() {
  brandForm.value = { intro: brand.value.intro || '', tone: brand.value.tone || '', banned: brand.value.bannedWords || '', buyLink: brand.value.buyLink || '' }
  brandEdit.value = true
}

async function saveBrand() {
  try {
    brand.value = await apiFetch(`/api/teams/${id.value}/brand`, {
      method: 'PUT',
      body: JSON.stringify({ intro: brandForm.value.intro, tone: brandForm.value.tone, bannedWords: brandForm.value.banned, buyLink: brandForm.value.buyLink }),
    })
    brandEdit.value = false
    toast('브랜드 정보를 저장했습니다. 새 작업부터 반영됩니다')
  } catch (e) {
    toast(e.message || '저장 실패', 'error')
  }
}

async function openChannels() {
  chanIds.value = channels.value.map((c) => c.id)
  chanMsg.value = ''
  chanOpen.value = true
  try {
    await ensureAccounts()
  } catch (e) {
    chanMsg.value = e.message || '계정 목록을 불러오지 못했습니다'
  }
}

async function submitChannels() {
  chanMsg.value = ''
  try {
    await apiFetch(`/api/teams/${id.value}/channels`, { method: 'PUT', body: JSON.stringify({ channels: chanIds.value }) })
    chanOpen.value = false
    await load()
    toast('발행 채널을 저장했습니다')
  } catch (e) {
    chanMsg.value = e.message || '저장 실패'
  }
}

async function saveSettings() {
  try {
    team.value = await apiFetch(`/api/teams/${id.value}`, {
      method: 'PATCH',
      body: JSON.stringify({ roleMode: settingsForm.value.roleMode, maxConcurrency: settingsForm.value.maxConcurrency || 0, distributionMode: settingsForm.value.distributionMode, distributionCap: settingsForm.value.distributionCap || 5 }),
    })
    toast('팀 설정을 저장했습니다')
  } catch (e) {
    toast(e.message || '저장 실패', 'error')
  }
}

async function saveRename() {
  renameMsg.value = ''
  renameForm.value = renameForm.value.name ? renameForm.value : { name: team.value.name, business: '', description: team.value.description || '' }
  if (!renameForm.value.name) {
    renameMsg.value = '팀 이름을 입력하세요'
    return
  }
  try {
    const biz = (renameForm.value.business || '').split(',').map((s) => s.trim()).filter(Boolean)
    team.value = await apiFetch(`/api/teams/${id.value}`, {
      method: 'PATCH',
      body: JSON.stringify({ name: renameForm.value.name, businesses: biz, description: renameForm.value.description }),
    })
    renameOpen.value = false
    toast('팀 이름을 바꿨습니다')
    await load()
  } catch (e) {
    const msg = e.message || ''
    renameMsg.value = msg.includes('already belongs') ? '다른 팀이 전담 중입니다' : (msg.includes('already exists') ? '같은 이름의 팀이 있습니다' : (msg || '저장 실패'))
  }
}

async function doDelete() {
  delOpen.value = false
  try {
    await apiFetch(`/api/teams/${id.value}`, { method: 'DELETE' })
    toast('팀을 삭제했습니다')
    router.push('/teams')
  } catch (e) {
    toast(e.message || '삭제 실패', 'error')
  }
}

watch(renameOpen, (open) => {
  if (open) {
    const raw = team.value.businesses || ''
    renameForm.value = { name: team.value.name || '', business: raw, description: team.value.description || '' }
    renameMsg.value = ''
  }
})
watch(() => route.params.id, load)
onMounted(load)
</script>

<style scoped>
.page-head { display: flex; align-items: baseline; gap: var(--s3); margin-bottom: var(--s1); }
.back { text-decoration: none; color: var(--ink-soft); white-space: nowrap; }
.link {
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
  border-radius: var(--r-md);
  padding: var(--s1) var(--s2);
  font-size: 13px;
  text-decoration: none;
  white-space: nowrap;
}
.link:hover { background: var(--soft); }
.page-head .page-title { margin: 0; font-size: 20px; }
.sub { font-size: 14px; font-weight: 400; }
.sp { flex: 1; }
.switch { display: flex; align-items: center; gap: var(--s1); font-size: 14px; cursor: pointer; white-space: nowrap; }
.menuwrap { position: relative; }
.menu {
  position: absolute; right: 0; top: calc(100% + 4px); z-index: 60;
  background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-md);
  box-shadow: var(--shadow-lg); min-width: 170px; padding: var(--s1);
  display: flex; flex-direction: column;
}
.menu button { background: none; border: none; text-align: left; padding: var(--s2) var(--s3); border-radius: var(--r-sm); color: var(--ink); margin: 0; }
.menu button:hover { background: var(--accent-soft); }
.menu .danger-t { color: var(--warn-ink, #9a5b00); }
.summary { margin: 2px 0; font-size: 14px; }
.tabbar { display: none; }
.panels { display: grid; grid-template-columns: 1fr 1fr; gap: var(--s4); margin-top: var(--s4); }
.panel.wide { grid-column: 1 / -1; }
.sect-head { display: flex; align-items: center; margin-bottom: var(--s2); }
.section-title { margin: 0; font-size: 16px; }
.rows { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--s2); }
.mrow { display: flex; align-items: center; gap: var(--s2); }
.ava.sm {
  width: 28px; height: 28px; border-radius: 50%; flex-shrink: 0;
  background: var(--accent-soft); display: inline-flex; align-items: center; justify-content: center;
  font-weight: 700; font-size: 13px;
}
.minfo { flex: 1; min-width: 0; }
.mname { margin: 0; font-weight: 600; }
.leader { font-size: 12px; font-weight: 700; background: var(--accent-soft); border-radius: 999px; padding: 1px 8px; }
.dot { width: 8px; height: 8px; border-radius: 50%; background: #bbb; display: inline-block; flex-shrink: 0; }
.dot.on { background: var(--ok); }
.rolechips { display: flex; gap: var(--s1); margin-top: 4px; flex-wrap: wrap; }
.chip { display: inline-flex; align-items: center; border: 1px solid var(--line); background: var(--panel); border-radius: 999px; padding: 2px 10px; font-size: 12px; cursor: pointer; color: var(--muted); margin: 0; }
.chip.on { border-color: var(--accent); color: var(--ink); background: var(--accent-soft); }
.wrow { display: flex; gap: var(--s2); text-decoration: none; color: inherit; align-items: center; }
.thumb { width: 48px; height: 48px; object-fit: cover; border-radius: var(--r-sm); flex-shrink: 0; }
.brandview p { margin: var(--s1) 0; }
.setrow { display: flex; align-items: center; gap: var(--s3); margin-top: var(--s2); flex-wrap: wrap; }
.setrow label { display: inline-flex; align-items: center; gap: var(--s2); margin: 0; }
.setrow .num { display: inline-block; width: 72px; margin-top: 0; }
.chk { display: flex; align-items: center; gap: var(--s1); cursor: pointer; }
.num { width: 72px; }
.picklist { list-style: none; margin: 0 0 var(--s2); padding: 0; max-height: 280px; overflow: auto; border: 1px solid var(--line-soft); border-radius: var(--r-md); }
.picklist li { padding: var(--s2) var(--s3); border-bottom: 1px solid var(--line-soft); }
.picklist li:last-child { border-bottom: none; }
.pname { font-weight: 600; }
.modal.wide { max-width: 560px; }
@media (max-width: 900px) {
  .panels { grid-template-columns: 1fr; }
  .tabbar { display: flex; gap: var(--s1); margin-top: var(--s3); border-bottom: 1px solid var(--line); position: sticky; top: 52px; background: var(--bg); z-index: 40; padding: var(--s1) 0; }
  .tabbar button { flex: 1; background: none; border: none; padding: var(--s2); margin: 0; color: var(--muted); border-bottom: 2px solid transparent; }
  .tabbar button.on { color: var(--ink); font-weight: 700; border-bottom-color: var(--accent); }
  .panel { display: none; }
  .panel.active { display: block; }
}
</style>
