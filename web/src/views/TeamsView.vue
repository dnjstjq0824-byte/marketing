<template>
  <div class="teams">
    <div class="page-head">
      <h1 class="page-title">팀</h1>
      <span class="muted">에이전트 {{ agentTotal }}기 · 팀 {{ teams.length }}개</span>
      <span class="sp"></span>
      <button @click="openCreate">＋ 팀 만들기</button>
    </div>
    <p v-if="onlyDefault" class="hint">브랜드별로 팀을 나눠 보세요. 예: 마케팅1팀 = LUNO 전담</p>
    <p v-if="loadMsg" class="error">{{ loadMsg }}</p>
    <div class="grid">
      <RouterLink v-for="c in teams" :key="c.team.id" class="card teamcard" :to="'/teams/' + c.team.id">
        <div class="t-head">
          <strong class="t-name">{{ c.team.name }}</strong>
          <span class="t-state" :class="{ off: !c.team.enabled }"><span class="dot" :class="{ on: c.team.enabled }" aria-hidden="true"></span>{{ c.team.enabled ? '가동' : '멈춤' }}</span>
        </div>
        <p class="t-biz muted">{{ bizLine(c) }}</p>
        <p class="t-comp">{{ compLine(c) }}</p>
        <div class="faces" aria-hidden="true">
          <span v-for="(f, i) in (c.faces || [])" :key="i" class="ava" :title="platformLabel(f)">{{ faceLetter(f) }}</span>
          <span v-if="c.memberCount > (c.faces || []).length" class="more">+{{ c.memberCount - (c.faces || []).length }}</span>
        </div>
        <p class="t-nums">일하는 중 {{ c.working }} · 대기 {{ c.idle }}</p>
        <p class="t-nums">진행 작업 {{ c.stats.runningWorks }} · 오늘 발행 {{ c.stats.todayPublished }}</p>
        <p class="t-nums muted">채널 {{ c.channelCount }}개</p>
        <p v-for="w in warnings(c)" :key="w" class="warn">⚠ {{ w }}</p>
      </RouterLink>
    </div>
    <div v-if="createOpen" class="modal-back" @click.self="createOpen = false">
      <div class="modal wide" role="dialog" aria-label="팀 만들기">
        <ol class="steps">
          <li :class="{ on: step === 1 }">① 팀 정보</li>
          <li :class="{ on: step === 2 }">② 에이전트 배치</li>
          <li :class="{ on: step === 3 }">③ 브랜드·채널</li>
        </ol>
        <div v-if="step === 1" class="form">
          <label>팀 이름<input v-model.trim="form.name" placeholder="예: 마케팅1팀" /></label>
          <label>전담 사업
            <input v-model.trim="form.business" list="bizlist" placeholder="예: LUNO (새 사업 입력 가능)" />
            <datalist id="bizlist"><option v-for="b in allBusinesses" :key="b" :value="b" /></datalist>
          </label>
          <label>설명<input v-model.trim="form.description" placeholder="예: LUNO 신상 홍보" /></label>
        </div>
        <div v-if="step === 2">
          <p class="muted small">이 팀에서 일할 에이전트를 고르세요</p>
          <div class="pickbar">
            <label class="chk"><input type="checkbox" :checked="allChecked" @change="toggleAll" /> 전체 선택</label>
            <select v-model="agentFilter" aria-label="AI 종류">
              <option value="">AI 전체</option>
              <option v-for="p in AI_PLATFORMS" :key="p" :value="p">{{ platformLabel(p) }}</option>
            </select>
            <input v-model.trim="agentQuery" placeholder="🔍 이름 검색" aria-label="이름 검색" />
          </div>
          <ul class="picklist">
            <li v-for="a in filteredAgents" :key="a.id">
              <label class="chk">
                <input type="checkbox" :value="a.id" v-model="form.memberIds" />
                <span class="ava sm" aria-hidden="true">{{ faceLetter(a.platform) }}</span>
                <span class="pname">{{ a.name }}</span>
                <span class="muted small">{{ platformLabel(a.platform) }}</span>
                <span v-if="memberTeamName(a.id)" class="muted small">{{ memberTeamName(a.id) }}{{ memberTeamName(a.id) !== '(기본 팀)' ? ' → 이동' : '' }}</span>
              </label>
              <div v-if="form.roleMode === 'manual' && form.memberIds.includes(a.id)" class="rolechips">
                <label v-for="r in roleKeys" :key="r" class="chip" :class="{ on: (form.roles[a.id] || []).includes(r) }">
                  <input type="checkbox" :value="r" v-model="form.roles[a.id]" />{{ roleLabel(r) }}
                </label>
              </div>
            </li>
          </ul>
          <p class="muted small">선택 {{ form.memberIds.length }}기</p>
          <div class="moderow">
            <span class="muted small">업무 분담</span>
            <label class="chk"><input type="radio" value="auto" v-model="form.roleMode" /> 자동</label>
            <label class="chk"><input type="radio" value="manual" v-model="form.roleMode" /> 직접 지정</label>
          </div>
        </div>
        <div v-if="step === 3" class="form">
          <label>브랜드 소개<textarea v-model.trim="form.intro" rows="2" placeholder="예: LUNO는 …" /></label>
          <label>말투<input v-model.trim="form.tone" list="tonelist" placeholder="예: 친근한 존댓말" />
            <datalist id="tonelist"><option value="친근한 존댓말" /><option value="정중한 존댓말" /><option value="활기찬 반말" /></datalist>
          </label>
          <label>금칙어<input v-model.trim="form.banned" placeholder="예: 최저가, 1위" /></label>
          <label>기본 구매 링크<input v-model.trim="form.buyLink" placeholder="https://…" /></label>
          <fieldset class="chanbox">
            <legend>발행 채널</legend>
            <p v-if="!channelAccounts.length" class="muted small">연결된 채널이 없습니다. 안 고르면 연결된 전 채널로 발행합니다.</p>
            <label v-for="a in channelAccounts" :key="a.id" class="chk">
              <input type="checkbox" :value="a.id" v-model="form.channelIds" />
              <span class="pname">{{ a.name }}</span>
              <span class="muted small">{{ platformLabel(a.platform) }}</span>
            </label>
            <p v-if="channelAccounts.length" class="muted small">안 고르면 연결된 전 채널로 발행합니다.</p>
          </fieldset>
        </div>
        <p v-if="createMsg" class="error">{{ createMsg }}</p>
        <div class="btnrow">
          <button class="ghost" @click="createOpen = false">취소</button>
          <span class="sp"></span>
          <button v-if="step > 1" class="ghost" @click="step--">이전</button>
          <button v-if="step < 3" class="ghost" @click="step++">건너뛰기</button>
          <button v-if="step < 3" @click="step++">다음</button>
          <button v-if="step === 3" @click="submitCreate" :disabled="creating">{{ creating ? '만드는 중…' : '팀 만들기' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch } from '../stores/auth.js'
import { toast } from '../stores/toast.js'
import { AI_PLATFORMS, label, PLATFORM, TEAM_ROLE } from '../stores/labels.js'

const router = useRouter()
const teams = ref([])
const accounts = ref([])
const memberTeams = ref({})
const loadMsg = ref('')
const createOpen = ref(false)
const step = ref(1)
const creating = ref(false)
const createMsg = ref('')
const agentFilter = ref('')
const agentQuery = ref('')
const form = ref({ name: '', business: '', description: '', memberIds: [], roles: {}, roleMode: 'auto', intro: '', tone: '', banned: '', buyLink: '', channelIds: [] })
const roleKeys = ['analyze', 'strategy', 'generate', 'review']

const agentTotal = computed(() => teams.value.reduce((n, c) => n + (c.memberCount || 0), 0))
const onlyDefault = computed(() => teams.value.length === 1 && teams.value[0].team.isDefault)
const allBusinesses = computed(() => {
  const out = []
  for (const c of teams.value) for (const b of c.businesses || []) if (!out.includes(b)) out.push(b)
  return out
})
const agentAccounts = computed(() => accounts.value.filter((a) => a.kind === 'agent_service'))
const channelAccounts = computed(() => accounts.value.filter((a) => a.kind === 'channel'))
const filteredAgents = computed(() => {
  const q = agentQuery.value.toLowerCase()
  return agentAccounts.value.filter((a) =>
    (!agentFilter.value || a.platform === agentFilter.value) &&
    (!q || a.name.toLowerCase().includes(q)))
})
const allChecked = computed(() => filteredAgents.value.length > 0 && filteredAgents.value.every((a) => form.value.memberIds.includes(a.id)))

function platformLabel(p) { return label(PLATFORM, p, p) }
function roleLabel(r) { return label(TEAM_ROLE, r, r) }
function faceLetter(p) { return (platformLabel(p) || '?').slice(0, 1) }
function memberTeamName(id) { return memberTeams.value[id] || '' }

function bizLine(c) {
  if (!c.businesses || !c.businesses.length) return c.team.isDefault ? '전담 없음' : '전담 사업 없음'
  return c.businesses.join(' · ') + ' 전담'
}

function compLine(c) {
  if (!c.memberCount) return c.team.isDefault ? '에이전트 0기' : '에이전트 없음'
  const plats = []
  for (const f of c.faces || []) {
    const name = platformLabel(f)
    if (!plats.includes(name)) plats.push(name)
  }
  const head = plats.slice(0, 2).join(' · ')
  const rest = plats.length > 2 ? ` 외 ${plats.length - 2}` : ''
  return `${head}${rest} ${c.memberCount}기`
}

function warnings(c) {
  const out = []
  if (c.memberCount === 0) {
    out.push(c.team.isDefault ? '에이전트가 없어 기본 팀 작업은 대기합니다' : '에이전트가 없습니다')
  } else if (c.memberCount === 1) {
    out.push('검수를 위해 1기 더 필요')
  } else if (c.unready >= c.memberCount) {
    out.push('전원 한도·로그인 필요')
  }
  if (!c.team.isDefault && (c.businesses || []).length > 0 && c.channelCount === 0) out.push('발행 채널 없음')
  return out
}

function toggleAll(e) {
  const ids = new Set(form.value.memberIds)
  if (e.target.checked) for (const a of filteredAgents.value) ids.add(a.id)
  else for (const a of filteredAgents.value) ids.delete(a.id)
  form.value.memberIds = [...ids]
}

async function load() {
  loadMsg.value = ''
  try {
    const data = await apiFetch('/api/teams')
    teams.value = data.items || []
  } catch (e) {
    loadMsg.value = e.message || '팀 목록을 불러오지 못했습니다'
  }
}

async function openCreate() {
  step.value = 1
  createMsg.value = ''
  form.value = { name: '', business: '', description: '', memberIds: [], roles: {}, roleMode: 'auto', intro: '', tone: '', banned: '', buyLink: '', channelIds: [] }
  agentFilter.value = ''
  agentQuery.value = ''
  createOpen.value = true
  try {
    const data = await apiFetch('/api/session/accounts')
    accounts.value = data.accounts || []
  } catch (e) {
    createMsg.value = e.message || '계정 목록을 불러오지 못했습니다'
    return
  }
  const names = {}
  await Promise.all(teams.value.map(async (c) => {
    try {
      const d = await apiFetch(`/api/teams/${c.team.id}`)
      for (const m of d.members || []) names[m.id] = c.team.isDefault ? '(기본 팀)' : c.team.name
    } catch { /* team name stays blank */ }
  }))
  memberTeams.value = names
}

async function submitCreate() {
  createMsg.value = ''
  if (!form.value.name) {
    createMsg.value = '팀 이름을 입력하세요'
    step.value = 1
    return
  }
  creating.value = true
  try {
    const body = { name: form.value.name, description: form.value.description, roleMode: form.value.roleMode }
    if (form.value.business) body.businesses = [form.value.business]
    const team = await apiFetch('/api/teams', { method: 'POST', body: JSON.stringify(body) })
    const id = team.id
    if (form.value.memberIds.length) {
      const members = form.value.memberIds.map((accountId) => ({
        accountId, roles: form.value.roleMode === 'manual' ? (form.value.roles[accountId] || []) : [],
      }))
      await apiFetch(`/api/teams/${id}/members`, { method: 'PUT', body: JSON.stringify({ members }) })
    }
    if (form.value.intro || form.value.tone || form.value.banned || form.value.buyLink) {
      await apiFetch(`/api/teams/${id}/brand`, {
        method: 'PUT',
        body: JSON.stringify({ intro: form.value.intro, tone: form.value.tone, bannedWords: form.value.banned, buyLink: form.value.buyLink }),
      })
    }
    if (form.value.channelIds.length) {
      await apiFetch(`/api/teams/${id}/channels`, { method: 'PUT', body: JSON.stringify({ channels: form.value.channelIds }) })
    }
    toast(`팀 ${form.value.name}을 만들었습니다`)
    createOpen.value = false
    router.push(`/teams/${id}`)
  } catch (e) {
    const msg = e.message || ''
    if (msg.includes('already belongs')) createMsg.value = `${form.value.business}은(는) 다른 팀이 전담 중입니다`
    else if (msg.includes('already exists')) createMsg.value = '같은 이름의 팀이 있습니다'
    else createMsg.value = msg || '팀 만들기에 실패했습니다'
  } finally {
    creating.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.page-head { display: flex; align-items: baseline; gap: var(--s3); margin-bottom: var(--s4); }
.page-head .page-title { margin: 0; }
.sp { flex: 1; }
.hint { color: var(--muted); margin: 0 0 var(--s4); }
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: var(--s4); }
.teamcard { text-decoration: none; color: inherit; display: block; }
.teamcard:hover { border-color: var(--accent); }
.t-head { display: flex; align-items: center; gap: var(--s2); }
.t-name { font-size: 17px; flex: 1; }
.t-state { font-size: 13px; color: var(--ink-soft); display: flex; align-items: center; gap: 4px; }
.t-state.off { color: var(--muted); }
.dot { width: 8px; height: 8px; border-radius: 50%; background: #bbb; display: inline-block; }
.dot.on { background: var(--ok); }
.t-biz { margin: var(--s1) 0; }
.t-comp { margin: 0 0 var(--s2); font-weight: 600; }
.faces { display: flex; gap: 4px; margin-bottom: var(--s2); }
.ava {
  width: 28px; height: 28px; border-radius: 50%;
  background: var(--accent-soft); color: var(--ink);
  display: inline-flex; align-items: center; justify-content: center;
  font-weight: 700; font-size: 14px;
}
.ava.sm { width: 24px; height: 24px; font-size: 12px; }
.more { color: var(--muted); font-size: 13px; align-self: center; }
.t-nums { margin: 2px 0; font-size: 14px; }
.warn { margin: var(--s1) 0 0; font-size: 13px; color: var(--warn-ink, #9a5b00); }
.steps { display: flex; gap: var(--s3); list-style: none; margin: 0 0 var(--s4); padding: 0; }
.steps li { color: var(--muted); font-size: 14px; }
.steps li.on { color: var(--ink); font-weight: 700; }
.modal.wide { max-width: 640px; }
.pickbar { display: flex; gap: var(--s2); margin-bottom: var(--s2); flex-wrap: wrap; }
.pickbar input[type="text"], .pickbar input:not([type]) { flex: 1; min-width: 120px; }
.picklist { list-style: none; margin: 0 0 var(--s2); padding: 0; max-height: 280px; overflow: auto; border: 1px solid var(--line-soft); border-radius: var(--r-md); }
.picklist li { padding: var(--s2) var(--s3); border-bottom: 1px solid var(--line-soft); }
.picklist li:last-child { border-bottom: none; }
.chk { display: flex; align-items: center; gap: var(--s2); cursor: pointer; }
.pname { font-weight: 600; }
.rolechips { display: flex; gap: var(--s1); margin: var(--s1) 0 var(--s1) 26px; flex-wrap: wrap; }
.chip { display: inline-flex; align-items: center; border: 1px solid var(--line); border-radius: 999px; padding: 2px 10px; font-size: 13px; cursor: pointer; color: var(--muted); }
.chip input { display: none; }
.chip.on { border-color: var(--accent); color: var(--ink); background: var(--accent-soft); }
.moderow { display: flex; gap: var(--s3); align-items: center; }
.chanbox { border: 1px solid var(--line-soft); border-radius: var(--r-md); padding: var(--s3); margin: var(--s2) 0 0; display: flex; flex-direction: column; gap: var(--s1); }
.chanbox legend { padding: 0 var(--s1); font-size: 14px; }
@media (max-width: 900px) {
  .grid { grid-template-columns: 1fr; }
}
</style>
