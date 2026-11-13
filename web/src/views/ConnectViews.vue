<template>
  <section v-if="view === 'agents'" class="page">
    <h2>에이전트 연결</h2>
    <p class="muted">상주 에이전트 등록·상태·일괄조작. 토큰은 발급·순환 시 1회만 보인다.</p>
    <p v-if="error" class="banner err">{{ error }}</p>

    <div class="card">
      <div class="sect-head"><h3>에이전트</h3><span class="count">{{ agentTotal }}기</span></div>
      <div class="toolbar">
        <label>레인
          <select v-model="laneFilter" @change="agentPage = 0; loadAgents()">
            <option value="">전체</option>
            <option value="A">A · 판독·전략</option>
            <option value="B">B · 생성</option>
            <option value="C">C · 검수</option>
            <option value="P">P · 발행</option>
          </select>
        </label>
        <label>상태
          <select v-model="agentStateFilter" @change="agentPage = 0; loadAgents()">
            <option value="">전체</option>
            <option value="on">온라인</option>
            <option value="off">오프라인</option>
            <option value="paused">중지</option>
          </select>
        </label>
        <label>검색 <input v-model="agentQuery" placeholder="이름" @keyup.enter="agentPage = 0; loadAgents()" /></label>
        <button class="ghost" @click="agentPage = 0; loadAgents()">새로고침</button>
      </div>
      <div class="btnrow">
        <button class="ghost small" @click="bulkAgents('pause')" :disabled="!agentSel.length">선택 일시중지</button>
        <button class="ghost small" @click="bulkAgents('resume')" :disabled="!agentSel.length">선택 재개</button>
        <select v-model="bulkLane" aria-label="일괄 레인">
          <option value="">레인 선택</option>
          <option value="A">A</option><option value="B">B</option>
          <option value="C">C</option><option value="P">P</option>
        </select>
        <button class="ghost small" @click="bulkAgents('lane')" :disabled="!agentSel.length || !bulkLane">선택 레인 변경</button>
      </div>
      <div v-if="agents.length" class="table-wrap">
        <table>
          <thead><tr><th></th><th>이름</th><th>레인</th><th>호스트</th><th>사업</th><th>상태</th><th>현재 작업</th><th class="num">24h 완료</th><th class="num">24h 실패</th><th></th></tr></thead>
          <tbody>
            <tr v-for="a in agents" :key="a.id">
              <td><input type="checkbox" :value="a.id" v-model="agentSel" :aria-label="a.label + ' 선택'" /></td>
              <td><strong>{{ a.label }}</strong><br /><span class="muted small">…{{ a.tokenPrefix }}</span></td>
              <td><span class="pill p-chief">{{ a.lane }}</span></td>
              <td>{{ a.host || '—' }}</td>
              <td>{{ a.businesses || '(전체)' }}</td>
              <td><span :class="agentStateClass(a)">{{ agentStateLabel(a) }}</span></td>
              <td>{{ a.currentJobId ? '#' + a.currentJobId : '—' }}</td>
              <td class="num">{{ a.done24h || 0 }}</td>
              <td class="num">{{ a.failed24h || 0 }}</td>
              <td>
                <button class="ghost small" @click="openAgent(a.id)">상세</button>
                <button class="ghost small" @click="confirmRotate = a.id">순환</button>
                <button class="ghost small" @click="confirmDelAgent = a.id">삭제</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyArt v-else art="lens" title="조건에 맞는 에이전트가 없습니다" />
      <div class="btnrow">
        <button class="ghost small" :disabled="agentPage === 0" @click="agentPage--; loadAgents()">이전</button>
        <span class="muted small">{{ agentPage + 1 }} / {{ agentPages }} 페이지</span>
        <button class="ghost small" :disabled="agentPage + 1 >= agentPages" @click="agentPage++; loadAgents()">다음</button>
      </div>
    </div>

    <div class="card narrow">
      <h3>에이전트 등록</h3>
      <div class="form-grid">
        <label>이름 <span class="req">*</span> <input v-model="regForm.label" placeholder="예: alpha-1" /></label>
        <label>레인
          <select v-model="regForm.lane"><option value="A">A · 판독·전략</option><option value="B">B · 생성</option><option value="C">C · 검수</option><option value="P">P · 발행</option></select>
        </label>
        <label>호스트 <input v-model="regForm.host" placeholder="예: studio-pc" /></label>
        <label>용량 <input v-model.number="regForm.capacity" type="number" min="1" /></label>
        <label>사업(쉼표) <input v-model="regForm.businesses" placeholder="비우면 전체" /></label>
        <label>연결 계정
          <select v-model="regForm.accountId">
            <option :value="null">없음</option>
            <option v-for="a in accounts" :key="a.id" :value="a.id">{{ a.name }}</option>
          </select>
        </label>
        <div class="full"><button @click="registerAgent">등록</button></div>
      </div>
      <p v-if="regToken" class="banner warn">토큰 1회 표시: <code>{{ regToken }}</code><br />지금 복사하세요. 다시 볼 수 없습니다.</p>
    </div>

    <div v-if="agentDetail" class="modal-back" @click.self="agentDetail = null">
      <div class="modal">
        <div class="sect-head"><h3>{{ agentDetail.agent.label }}</h3><span :class="agentStateClass(agentDetail.agent)">{{ agentStateLabel(agentDetail.agent) }}</span></div>
        <ul class="speclist">
          <li><span class="k">레인</span><span>{{ agentDetail.agent.lane }}</span></li>
          <li><span class="k">호스트</span><span>{{ agentDetail.agent.host || '—' }}</span></li>
          <li><span class="k">사업</span><span>{{ agentDetail.agent.businesses || '(전체)' }}</span></li>
          <li><span class="k">용량</span><span>{{ agentDetail.agent.capacity }}</span></li>
          <li><span class="k">마지막 신호</span><span>{{ shortDate(agentDetail.agent.lastSeenAt) }}</span></li>
        </ul>
        <h4>최근 작업</h4>
        <ul v-if="agentDetail.jobs.length" class="speclist">
          <li v-for="j in agentDetail.jobs" :key="j.id">
            <span class="k">#{{ j.id }} {{ jobTypeLabel(j.type) }}</span>
            <span><span :class="'pill p-' + j.status">{{ jobStatusLabel(j.status) }}</span></span>
          </li>
        </ul>
        <p v-else class="muted small">작업 이력이 없습니다.</p>
        <div class="btnrow"><button class="ghost" @click="agentDetail = null">닫기</button></div>
      </div>
    </div>

    <div v-if="confirmDelAgent !== null" class="modal-back" @click.self="confirmDelAgent = null">
      <div class="modal">
        <h3>에이전트 삭제</h3>
        <p>처리 중인 작업이 있으면 삭제할 수 없습니다. 되돌릴 수 없습니다.</p>
        <div class="btnrow">
          <button class="danger" @click="doDeleteAgent">삭제</button>
          <button class="ghost" @click="confirmDelAgent = null">취소</button>
        </div>
      </div>
    </div>

    <div v-if="confirmRotate !== null" class="modal-back" @click.self="confirmRotate = null; newToken = ''">
      <div class="modal">
        <h3>토큰 순환</h3>
        <p v-if="!newToken">기존 토큰은 즉시 무효가 됩니다. 계속할까요?</p>
        <p v-if="newToken" class="banner warn">새 토큰 1회 표시: <code>{{ newToken }}</code><br />지금 복사하세요. 다시 볼 수 없습니다.</p>
        <div class="btnrow">
          <button v-if="!newToken" class="danger" @click="doRotate">순환</button>
          <button class="ghost" @click="confirmRotate = null; newToken = ''">{{ newToken ? '닫기' : '취소' }}</button>
        </div>
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
      <div v-if="filtered.length" class="table-wrap">
        <table>
          <thead><tr><th>계정</th><th>플랫폼</th><th>종류</th><th>역할</th><th>담당 사업</th><th>세션 상태</th><th>ID·비번</th><th>사용량</th><th>마지막 작업</th><th>변경</th></tr></thead>
          <tbody>
            <tr v-for="a in filtered" :key="a.id">
              <td>{{ a.name }}</td>
              <td>{{ platformLabel(a.platform) }}</td>
              <td>{{ kindLabel(a.kind) }}</td>
              <td><span :class="'pill p-' + a.role">{{ roleLabel(a.role) }}</span></td>
              <td>{{ a.business || '—' }}</td>
              <td><span :class="'pill p-' + (a.sessionState || 'pending')">{{ sessionLabel(a.sessionState) }}</span></td>
              <td>{{ a.hasCredentials ? '저장됨' : '—' }}</td>
              <td>{{ a.usage || '—' }}</td>
              <td>{{ shortDate(a.lastJobAt) }}</td>
              <td>
                <button class="ghost small" @click="openLogin(a)">로그인</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyArt v-else art="lens" title="조건에 맞는 계정이 없습니다" />
    </div>

    <div class="card">
      <h3>계정 등록</h3>
      <p class="muted small">ID·비밀번호는 AES-256으로 암호화 저장되며 화면에 다시 보이지 않습니다.</p>
      <div class="form-grid">
        <label>계정명 <input v-model="accForm.name" /></label>
        <label>플랫폼
          <select v-model="accForm.platform">
            <option value="">선택</option>
            <optgroup v-for="g in PLATFORM_GROUPS" :key="g.label" :label="g.label">
              <option v-for="p in g.options" :key="p" :value="p">{{ platformLabel(p) }}</option>
            </optgroup>
          </select>
        </label>
        <label>주인 <input v-model="accForm.owner" /></label>
        <label>역할
          <select v-model="accForm.role"><option value="chief">총괄</option><option value="worker">작업자</option></select>
        </label>
        <label>담당 사업 <input v-model="accForm.business" /></label>
        <label>로그인 ID <input v-model="accForm.loginId" autocomplete="off" /></label>
        <label>비밀번호 <input v-model="accForm.password" type="password" autocomplete="new-password" /></label>
        <div class="full"><button @click="addAccount">등록</button></div>
      </div>
    </div>

    <div class="card">
      <h3>계정 정보 수정</h3>
      <p class="muted small">계정명은 큐 연결 키라 바꿀 수 없습니다. 주인·역할·사업만 수정합니다.</p>
      <div class="form-grid">
        <label>계정
          <select v-model="accEdit.id" @change="prefillAccount">
            <option value="">선택</option>
            <option v-for="a in accounts" :key="a.id" :value="a.id">{{ a.name }}</option>
          </select>
        </label>
        <label>주인 <input v-model="accEdit.owner" /></label>
        <label>역할
          <select v-model="accEdit.role"><option value="chief">총괄</option><option value="worker">작업자</option></select>
        </label>
        <label>담당 사업 <input v-model="accEdit.business" /></label>
        <label>종류
          <select v-model="accEdit.kind"><option value="channel">channel</option><option value="agent_service">agent_service</option></select>
        </label>
        <label>플랫폼 사용자 ID <input v-model="accEdit.platformUserId" placeholder="(없음)" /></label>
        <label>로그인 ID 변경 <input v-model="accEdit.loginId" autocomplete="off" placeholder="(유지)" /></label>
        <label>비밀번호 변경 <input v-model="accEdit.password" type="password" autocomplete="new-password" placeholder="(유지)" /></label>
        <div class="full"><button @click="saveAccount">저장</button></div>
      </div>
    </div>

    <div v-if="loginDlg" class="modal-back" @click.self="closeLogin">
      <div class="modal">
        <div class="sect-head"><h3>{{ loginDlg.name }} 로그인</h3><span class="pill p-pending">{{ platformLabel(loginDlg.platform) }}</span></div>
        <p v-if="loginDlg.phase === 'ask'" class="muted">{{ loginHint(loginDlg.platform) }}</p>
        <ol v-if="loginDlg.phase === 'ask'" class="steps">
          <li class="now">브라우저 열기</li>
          <li>뜬 창에서 직접 로그인</li>
          <li>완료 확인</li>
        </ol>
        <label v-if="loginDlg.phase === 'ask'" class="check"><input v-model="loginDlg.realProfile" type="checkbox" /> 내 Chrome 프로필로 열기 (Google 로그인용, Chrome 완전 종료 필요)</label>
        <p v-if="loginDlg.phase === 'ask'" class="muted small">브라우저가 안 뜨면: 파워셸에서 <code>adapter\.venv\Scripts\python -m adapter.login {{ loginDlg.name }} {{ loginDlg.platform }}</code> 실행 후 직접 로그인하고, 아래 바로 확인을 누르세요.</p>
        <p v-if="loginDlg.phase === 'waiting'" class="banner warn">브라우저가 열렸습니다. 로그인을 끝낸 뒤 아래 완료 확인을 누르세요.</p>
        <p v-if="loginDlg.phase === 'busy'" class="banner warn">확인 중입니다. 잠시 기다리세요.</p>
        <p v-if="loginDlg.msg" :class="loginDlg.ok ? 'banner okk' : 'error'">{{ loginDlg.msg }}</p>
        <div class="btnrow">
          <button v-if="loginDlg.phase === 'ask'" @click="startLogin">브라우저 열기</button>
          <button v-if="loginDlg.phase === 'ask'" class="ghost" @click="finishLogin">터미널 로그인 후 바로 확인</button>
          <button v-if="loginDlg.phase === 'waiting'" @click="finishLogin">로그인 완료 확인</button>
          <button class="ghost" @click="closeLogin">{{ loginDlg.phase === 'waiting' || loginDlg.phase === 'busy' ? '취소' : '닫기' }}</button>
        </div>
      </div>
    </div>
  </section>

  <section v-else-if="view === 'channels'" class="page">
    <h2>채널 계정</h2>
    <p class="muted">SNS·블로그 계정의 세션 상태와 자동 발행 가능 여부를 봅니다. 로그인 ID·비밀번호는 AES-256으로 암호화 저장되며 다시 표시되지 않습니다.</p>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="card">
      <div class="toolbar">
        <button @click="checkNow">가벼운 점검 실행</button>
        <span class="muted small">점검은 가볍게 주기로 돌고, 실제 작업이 배정된 세션만 깨웁니다.</span>
      </div>
      <div v-if="accounts.length" class="table-wrap">
        <table>
          <thead><tr><th>계정</th><th>플랫폼</th><th>세션 상태</th><th>발행 모드</th><th>자동 발행</th><th>마지막 확인</th><th>변경</th></tr></thead>
          <tbody>
            <tr v-for="a in accounts" :key="a.id">
              <td>{{ a.name }}</td>
              <td>{{ platformLabel(a.platform) }}</td>
              <td><span :class="'pill p-' + (a.sessionState || 'pending')">{{ sessionLabel(a.sessionState) }}</span></td>
              <td><span :class="a.postingMode === 'auto' ? 'pill p-auto' : 'pill p-manual'">{{ modeLabel(a.postingMode) }}</span></td>
              <td>{{ a.sessionState === 'connected' ? '가능 (트랜치 2)' : '불가 · 발행 정지' }}</td>
              <td>{{ shortDate(a.lastCheckedAt) }}</td>
              <td>
                <button class="ghost small" @click="openLogin(a)">로그인</button>
                <button class="ghost small" @click="setSession(a.id, 'expired')">만료</button>
                <button class="ghost small" @click="setSession(a.id, 'limited')">한도</button>
                <button class="ghost small" @click="toggleMode(a)">모드 전환</button>
                <button class="ghost small" @click="confirmUnreg = a.id">해제</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyArt v-else art="plant" title="등록된 채널 계정이 없습니다" hint="에이전트 연결 화면에서 등록하세요." />
    </div>
    <div class="card">
      <div class="sect-head"><h3>재로그인 필요 큐</h3><span class="count">{{ relogin.length }}건</span></div>
      <ul v-if="relogin.length" class="speclist">
        <li v-for="r in relogin" :key="r.accountId">
          <span class="k">{{ r.accountName }}</span>
          <span>{{ r.owner || '주인 미지정' }} · 보류 {{ r.heldCount }}건 · <span :class="'pill p-' + r.sessionState">{{ sessionLabel(r.sessionState) }}</span></span>
        </li>
      </ul>
      <p v-else class="empty">없음</p>
    </div>

    <div v-if="confirmUnreg !== null" class="modal-back" @click.self="confirmUnreg = null">
      <div class="modal">
        <h3>계정 해제</h3>
        <p>보류 중인 예약이 있으면 해제할 수 없습니다.</p>
        <div class="btnrow">
          <button class="danger" @click="doUnregister">해제</button>
          <button class="ghost" @click="confirmUnreg = null">취소</button>
        </div>
      </div>
    </div>

    <div v-if="loginDlg" class="modal-back" @click.self="closeLogin">
      <div class="modal">
        <div class="sect-head"><h3>{{ loginDlg.name }} 로그인</h3><span class="pill p-pending">{{ platformLabel(loginDlg.platform) }}</span></div>
        <p v-if="loginDlg.phase === 'ask'" class="muted">{{ loginHint(loginDlg.platform) }}</p>
        <ol v-if="loginDlg.phase === 'ask'" class="steps">
          <li class="now">브라우저 열기</li>
          <li>뜬 창에서 직접 로그인</li>
          <li>완료 확인</li>
        </ol>
        <label v-if="loginDlg.phase === 'ask'" class="check"><input v-model="loginDlg.realProfile" type="checkbox" /> 내 Chrome 프로필로 열기 (Google 로그인용, Chrome 완전 종료 필요)</label>
        <p v-if="loginDlg.phase === 'ask'" class="muted small">브라우저가 안 뜨면: 파워셸에서 <code>adapter\.venv\Scripts\python -m adapter.login {{ loginDlg.name }} {{ loginDlg.platform }}</code> 실행 후 직접 로그인하고, 아래 바로 확인을 누르세요.</p>
        <p v-if="loginDlg.phase === 'waiting'" class="banner warn">브라우저가 열렸습니다. 로그인을 끝낸 뒤 아래 완료 확인을 누르세요.</p>
        <p v-if="loginDlg.phase === 'busy'" class="banner warn">확인 중입니다. 잠시 기다리세요.</p>
        <p v-if="loginDlg.msg" :class="loginDlg.ok ? 'banner okk' : 'error'">{{ loginDlg.msg }}</p>
        <div class="btnrow">
          <button v-if="loginDlg.phase === 'ask'" @click="startLogin">브라우저 열기</button>
          <button v-if="loginDlg.phase === 'ask'" class="ghost" @click="finishLogin">터미널 로그인 후 바로 확인</button>
          <button v-if="loginDlg.phase === 'waiting'" @click="finishLogin">로그인 완료 확인</button>
          <button class="ghost" @click="closeLogin">{{ loginDlg.phase === 'waiting' || loginDlg.phase === 'busy' ? '취소' : '닫기' }}</button>
        </div>
      </div>
    </div>
  </section>

  <section v-else class="page">
    <h2>자체 채널</h2>
    <p class="muted">SOMUN이 직접 운영하는 채널 자리입니다.</p>
    <div class="card rule-teal">
      <div class="sect-head"><h3>체크인데일리</h3><span class="pill p-connected">운영 중</span></div>
      <p>상세 지표는 지표 화면에서 확인합니다. (트랜치 1: 자리 표시)</p>
    </div>
    <div class="card">
      <div class="sect-head"><h3>주식·코인 뉴스 확장 슬롯</h3><span class="pill p-pending">확장 예정</span></div>
      <p class="muted">확장 예정 슬롯입니다. (트랜치 1: 자리 표시)</p>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { apiFetch } from '../stores/auth.js'
import { AI_PLATFORMS, JOB_STATUS, JOB_TYPE, KIND, LOGIN_HINT, PLATFORM, PLATFORM_GROUPS, POSTING_MODE, ROLE, SESSION_STATE, label } from '../stores/labels.js'
import EmptyArt from '../components/EmptyArt.vue'

const props = defineProps({ view: { type: String, default: 'agents' } })

const accounts = ref([])
const relogin = ref([])
const error = ref('')
const roleFilter = ref('')
const bizFilter = ref('')
const alertOnly = ref(false)
const accForm = ref({ name: '', platform: '', owner: '', role: 'worker', business: '', loginId: '', password: '' })
const accEdit = ref({ id: '', owner: '', role: 'worker', business: '', kind: 'channel', platformUserId: '', loginId: '', password: '' })
const confirmUnreg = ref(null)
const loginDlg = ref(null) // { id, name, platform, phase, msg, ok }

const agents = ref([])
const agentTotal = ref(0)
const agentPage = ref(0)
const agentLimit = 20
const laneFilter = ref('')
const agentStateFilter = ref('')
const agentQuery = ref('')
const agentSel = ref([])
const bulkLane = ref('')
const regForm = ref({ label: '', lane: 'A', host: '', capacity: 1, businesses: '', accountId: null })
const regToken = ref('')
const agentDetail = ref(null)
const confirmDelAgent = ref(null)
const confirmRotate = ref(null)
const newToken = ref('')

const agentPages = computed(() => Math.max(1, Math.ceil(agentTotal.value / agentLimit)))

function agentOnline(a) {
  if (!a.lastSeenAt) return false
  return Date.now() - new Date(a.lastSeenAt + 'Z').getTime() < 3 * 60 * 1000
}

function agentStateLabel(a) {
  if (!a.enabled) return '중지'
  return agentOnline(a) ? '온라인' : '오프라인'
}

function agentStateClass(a) {
  if (!a.enabled) return 'pill p-pending'
  return agentOnline(a) ? 'pill p-connected' : 'pill p-limited'
}

async function loadAgents() {
  error.value = ''
  try {
    const params = new URLSearchParams({ limit: String(agentLimit), offset: String(agentPage.value * agentLimit) })
    if (laneFilter.value) params.set('lane', laneFilter.value)
    if (agentQuery.value.trim()) params.set('q', agentQuery.value.trim())
    const data = await apiFetch('/api/agents?' + params.toString())
    let items = data.items || []
    if (agentStateFilter.value === 'paused') items = items.filter((a) => !a.enabled)
    else if (agentStateFilter.value === 'on') items = items.filter((a) => a.enabled && agentOnline(a))
    else if (agentStateFilter.value === 'off') items = items.filter((a) => a.enabled && !agentOnline(a))
    agents.value = items
    agentTotal.value = data.total || 0
    agentSel.value = []
  } catch (e) {
    error.value = e.message
  }
}

async function registerAgent() {
  error.value = ''
  regToken.value = ''
  try {
    const body = {
      label: regForm.value.label, lane: regForm.value.lane, host: regForm.value.host,
      capacity: regForm.value.capacity, businesses: regForm.value.businesses,
    }
    if (regForm.value.accountId) body.accountId = regForm.value.accountId
    const data = await apiFetch('/api/agents', { method: 'POST', body: JSON.stringify(body) })
    regToken.value = data.token
    regForm.value = { label: '', lane: 'A', host: '', capacity: 1, businesses: '', accountId: null }
    await loadAgents()
  } catch (e) {
    error.value = e.message
  }
}

async function bulkAgents(action) {
  error.value = ''
  try {
    const body = { ids: agentSel.value, action }
    if (action === 'lane') body.lane = bulkLane.value
    await apiFetch('/api/agents/bulk', { method: 'POST', body: JSON.stringify(body) })
    bulkLane.value = ''
    await loadAgents()
  } catch (e) {
    error.value = e.message
  }
}

async function openAgent(id) {
  error.value = ''
  try {
    agentDetail.value = await apiFetch(`/api/agents/${id}`)
  } catch (e) {
    error.value = e.message
  }
}

async function doDeleteAgent() {
  const id = confirmDelAgent.value
  confirmDelAgent.value = null
  error.value = ''
  try {
    await apiFetch(`/api/agents/${id}`, { method: 'DELETE' })
    await loadAgents()
  } catch (e) {
    error.value = e.message
  }
}

async function doRotate() {
  const id = confirmRotate.value
  error.value = ''
  try {
    const data = await apiFetch(`/api/agents/${id}/rotate-token`, { method: 'POST' })
    newToken.value = data.token
    await loadAgents()
  } catch (e) {
    error.value = e.message
    confirmRotate.value = null
  }
}

function prefillAccount() {
  const a = accounts.value.find((x) => x.id === accEdit.value.id)
  if (!a) return
  accEdit.value.owner = a.owner || ''
  accEdit.value.role = a.role || 'worker'
  accEdit.value.business = a.business || ''
  accEdit.value.kind = a.kind || 'channel'
  accEdit.value.platformUserId = a.platformUserId || ''
}

async function saveAccount() {
  error.value = ''
  try {
    const body = { owner: accEdit.value.owner, role: accEdit.value.role, business: accEdit.value.business,
      kind: accEdit.value.kind, platformUserId: accEdit.value.platformUserId }
    if (accEdit.value.loginId || accEdit.value.password) {
      body.loginId = accEdit.value.loginId
      body.password = accEdit.value.password
    }
    await apiFetch(`/api/session/accounts/${accEdit.value.id}`, { method: 'PATCH', body: JSON.stringify(body) })
    accEdit.value = { id: '', owner: '', role: 'worker', business: '', kind: 'channel', platformUserId: '', loginId: '', password: '' }
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function doUnregister() {
  const id = confirmUnreg.value
  confirmUnreg.value = null
  error.value = ''
  try {
    await apiFetch(`/api/session/accounts/${id}`, { method: 'DELETE' })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

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
  return label(SESSION_STATE, s)
}

function roleLabel(r) {
  return label(ROLE, r)
}

function modeLabel(m) {
  return label(POSTING_MODE, m)
}

function platformLabel(p) {
  return label(PLATFORM, p)
}

function kindLabel(k) {
  return label(KIND, k)
}

function loginHint(p) {
  return label(LOGIN_HINT, p, '브라우저에서 계정으로 로그인하세요.')
}

function openLogin(a) {
  loginDlg.value = { id: a.id, name: a.name, platform: a.platform, phase: 'ask', msg: '', ok: false, realProfile: false }
}

async function startLogin() {
  const dlg = loginDlg.value
  if (!dlg) return
  dlg.msg = ''
  try {
    await apiFetch(`/api/session/accounts/${dlg.id}/login-start`, { method: 'POST', body: JSON.stringify({ realProfile: !!dlg.realProfile }) })
    dlg.phase = 'waiting'
  } catch (e) {
    dlg.msg = e.message
    dlg.ok = false
  }
}

async function finishLogin() {
  const dlg = loginDlg.value
  if (!dlg) return
  const from = dlg.phase
  dlg.phase = 'busy'
  dlg.msg = ''
  try {
    await apiFetch(`/api/session/accounts/${dlg.id}/login-finish`, { method: 'POST' })
    dlg.msg = '로그인 확인됨. 세션이 연결됐습니다.'
    dlg.ok = true
    dlg.phase = 'done'
    await load()
  } catch (e) {
    dlg.msg = e.message
    dlg.ok = false
    dlg.phase = from === 'ask' ? 'ask' : 'waiting'
  }
}

async function closeLogin() {
  const dlg = loginDlg.value
  loginDlg.value = null
  if (!dlg) return
  if (dlg.phase === 'waiting' || dlg.phase === 'busy') {
    try {
      await apiFetch(`/api/session/accounts/${dlg.id}/login-cancel`, { method: 'POST' })
    } catch (e) {
      error.value = e.message
    }
    await load()
  }
}

function jobTypeLabel(t) {
  return label(JOB_TYPE, t)
}

function jobStatusLabel(s) {
  return label(JOB_STATUS, s)
}

function shortDate(iso) {
  return iso ? iso.slice(0, 16).replace('T', ' ') : '—'
}

async function load() {
  error.value = ''
  try {
    const [acc, rl] = await Promise.all([
      apiFetch('/api/session/accounts'),
      apiFetch('/api/session/relogin-queue'),
    ])
    accounts.value = acc.accounts || []
    relogin.value = rl.items || []
    if (props.view === 'agents') await loadAgents()
  } catch (e) {
    error.value = e.message
  }
}

async function addAccount() {
  error.value = ''
  try {
    const created = await apiFetch('/api/session/accounts', { method: 'POST', body: JSON.stringify(accForm.value) })
    // Kind follows the platform: AI services are agent_service, channels are channel.
    const kind = AI_PLATFORMS.includes(accForm.value.platform) ? 'agent_service' : 'channel'
    if (created && created.id) {
      try {
        await apiFetch(`/api/session/accounts/${created.id}`, { method: 'PATCH', body: JSON.stringify({ kind }) })
      } catch (e2) {
        // The account exists; show it and report the kind step separately.
        error.value = e2.message + ' (계정은 등록됨, 종류는 정보 수정에서 지정)'
      }
    }
    accForm.value = { name: '', platform: '', owner: '', role: 'worker', business: '', loginId: '', password: '' }
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

async function toggleMode(a) {
  error.value = ''
  try {
    const next = a.postingMode === 'auto' ? 'manual' : 'auto'
    await apiFetch(`/api/session/accounts/${a.id}`, { method: 'PATCH', body: JSON.stringify({ postingMode: next }) })
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
