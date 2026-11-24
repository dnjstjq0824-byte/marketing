<template>
  <div class="settings">
    <div class="page-head sticky-head">
      <h1 class="page-title">설정</h1>
      <span class="muted small ctx">자동화 {{ form.enabled ? '켜짐' : '꺼짐' }} · 운영자 <b class="num">{{ users.length }}</b>명</span>
    </div>
    <Skeleton v-if="loading" :rows="6" />
    <ErrorRetry v-else-if="loadError" @retry="load" />
    <template v-else>
      <section class="card" aria-label="자동화">
        <h2 class="section-title">자동화</h2>
        <div class="frow">
          <span id="auto-label">자동화 켜기</span>
          <button role="switch" :aria-checked="form.enabled" aria-labelledby="auto-label" class="switch" :class="{ on: form.enabled }" @click="form.enabled = !form.enabled">
            <span class="knob" aria-hidden="true"></span>{{ form.enabled ? '켜짐' : '꺼짐' }}
          </button>
        </div>
        <div class="grid">
          <label>계정당 발행 간격
            <span class="inrow"><input v-model.number="form.publishIntervalMin" type="number" min="0" max="1440" /> 분</span>
            <FieldError :message="errors.interval" hint="0이면 간격 제한 없음" />
          </label>
          <label>계정당 하루 최대
            <span class="inrow"><input v-model.number="form.dailyCap" type="number" min="0" max="999" /> 건</span>
            <FieldError :message="errors.cap" hint="0이면 수량 제한 없음" />
          </label>
          <label class="span2">발행 시간대
            <span class="inrow"><input v-model="form.windowStart" type="time" aria-label="시작 시각" /> ~ <input v-model="form.windowEnd" type="time" aria-label="종료 시각" /></span>
            <span class="hint">밖이면 다음 날로 미룸</span>
            <FieldError :message="errors.window" hint="시작과 종료를 다르게 입력하세요" />
          </label>
          <label>블로그 최소 분량
            <span class="inrow"><input v-model.number="form.blogMinLen" type="number" min="0" max="20000" /> 자</span>
            <FieldError :message="errors.blog" hint="0이면 분량 제한 없음" />
          </label>
          <label>동시 실행 최대
            <span class="inrow"><input v-model.number="form.maxConcurrent" type="number" min="1" max="50" /> 건</span>
            <span class="hint">AI 작업 동시 진행 상한</span>
            <FieldError :message="errors.conc" />
          </label>
        </div>
        <fieldset>
          <legend>만들 것 기본값</legend>
          <label class="check"><input v-model="form.outputs.text" type="checkbox" /> 글</label>
          <label class="check"><input v-model="form.outputs.image" type="checkbox" /> 이미지</label>
          <label class="check"><input v-model="form.outputs.video" type="checkbox" /> 숏폼 영상</label>
        </fieldset>
        <div class="frow wrap">
          <span>미디어 작업 배정</span>
          <label class="check"><input v-model="form.preferPaidMedia" type="checkbox" /> 유료 요금제 계정 우선</label>
        </div>
        <div class="btnrow">
          <button :disabled="savingAuto" @click="saveAutomation">{{ savingAuto ? '저장 중…' : '저장' }}</button>
          <span v-if="savedAuto" class="ok" role="status">저장됨 ✔</span>
        </div>
        <p v-if="errors.form" class="banner err">{{ errors.form }}</p>
      </section>

      <section class="card" aria-label="내 계정">
        <h2 class="section-title">내 계정</h2>
        <div class="frow">
          <span>{{ me.username }}</span>
          <button class="ghost" @click="pwOpen = true">비밀번호 변경</button>
        </div>
        <div class="frow">
          <span>운영자 {{ users.length }}명</span>
          <button class="ghost" :aria-expanded="opsOpen" @click="opsOpen = !opsOpen">운영자 관리 {{ opsOpen ? '▾' : '▸' }}</button>
        </div>
        <div v-if="opsOpen" class="ops">
          <ul v-if="users.length" class="userlist">
            <li v-for="u in users" :key="u.id">
              <span>{{ u.username }}</span>
              <span><span class="role">{{ u.role }}</span> <button v-if="u.id !== me.id" class="ghost small" @click="confirmDelUser = u.id">삭제</button></span>
            </li>
          </ul>
          <div class="addrow">
            <label>새 아이디 <input v-model="newUser.username" autocomplete="off" /></label>
            <label>비밀번호(8자+) <input v-model="newUser.password" type="password" autocomplete="new-password" /></label>
            <button @click="addUser">운영자 추가</button>
          </div>
          <FieldError :message="userError" hint="아이디가 겹치면 다른 이름으로 시도하세요" />
        </div>
      </section>

      <section class="card" aria-label="고급">
        <button class="advhead" :aria-expanded="advOpen" @click="advOpen = !advOpen">
          <span class="section-title">시스템 · 백업 · 고급</span><span aria-hidden="true">{{ advOpen ? '▾' : '▸' }}</span>
        </button>
        <template v-if="advOpen">
          <div class="health-box" :class="{ low: healthLow }">
            <span class="micro muted">엔진 상태 진단</span>
            <strong class="small">{{ healthLine }}</strong>
          </div>
          <div class="frow">
            <span>백업</span>
            <span class="muted small">마지막 {{ lastBackup }} · {{ backupKeep }}세대 보관</span>
            <button class="ghost small" :disabled="backupBusy" @click="runBackup">{{ backupBusy ? '백업 중…' : '지금 백업' }}</button>
          </div>
          <p v-if="backupMsg" :class="backupOk ? 'banner okk' : 'banner err'">{{ backupMsg }}</p>
          <div class="frow wrap">
            <span>백업 포함</span>
            <label class="check"><input v-model="form.backupIncludeSessions" type="checkbox" /> 로그인 세션 포함</label>
          </div>
          <div class="frow wrap">
            <span>미디어 보관</span>
            <label class="inline">안 쓰인 생성물 <input v-model.number="form.mediaRetentionDays" class="narrow" type="number" min="1" max="365" /> 일 뒤 정리</label>
            <label class="check"><input v-model="form.backupIncludeVideo" type="checkbox" /> 영상 백업 포함</label>
          </div>
          <FieldError :message="errors.retention" />
          <p class="hint">자동 정리는 추후 연결됩니다. 값은 미리 저장됩니다.</p>
          <div class="btnrow">
            <button :disabled="savingAdv" @click="saveAdvanced">{{ savingAdv ? '저장 중…' : '저장' }}</button>
            <span v-if="savedAdv" class="ok" role="status">저장됨 ✔</span>
          </div>
          <div class="frow">
            <span>네트워크 경로</span>
            <span>직결</span>
          </div>
          <div class="frow">
            <span>버전</span>
            <span class="muted small">v{{ version.app }} · DB v{{ version.schema }}</span>
          </div>
        </template>
      </section>
    </template>

    <div v-if="pwOpen" class="modal-back" @click.self="pwOpen = false">
      <div class="modal" role="dialog" aria-label="비밀번호 변경">
        <h3>비밀번호 변경</h3>
        <label>현재 비밀번호 <input v-model="pwForm.current" type="password" autocomplete="current-password" /></label>
        <label>새 비밀번호(8자+) <input v-model="pwForm.next" type="password" autocomplete="new-password" /></label>
        <p v-if="pwMsg" class="banner okk">{{ pwMsg }}</p>
        <FieldError :message="pwError" hint="현재 비밀번호를 확인하고 8자 이상으로 입력하세요" />
        <div class="btnrow">
          <button class="ghost" @click="pwOpen = false">닫기</button>
          <button @click="changePassword">변경</button>
        </div>
      </div>
    </div>

    <div v-if="confirmDelUser !== null" class="modal-back" @click.self="confirmDelUser = null">
      <div class="modal" role="alertdialog" aria-label="운영자 삭제 확인">
        <h3>운영자 삭제</h3>
        <p>이 운영자는 다시 로그인할 수 없습니다. 되돌릴 수 없습니다.</p>
        <div class="btnrow">
          <button class="danger" @click="doRemoveUser">삭제</button>
          <button class="ghost" @click="confirmDelUser = null">취소</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { apiFetch } from '../stores/auth.js'
import { toast } from '../stores/toast.js'
import Skeleton from '../components/Skeleton.vue'
import ErrorRetry from '../components/console/ErrorRetry.vue'
import FieldError from '../components/console/FieldError.vue'

const loading = ref(true)
const loadError = ref(false)
const form = ref({ enabled: true, publishIntervalMin: 120, dailyCap: 3, windowStart: '09:00', windowEnd: '22:00', blogMinLen: 1500, outputs: { text: true, image: true, video: true }, mediaRetentionDays: 30, backupIncludeVideo: false, backupIncludeSessions: true, maxConcurrent: 3, preferPaidMedia: true })
const errors = ref({})
const savingAuto = ref(false)
const savingAdv = ref(false)
const savedAuto = ref(false)
const savedAdv = ref(false)
const me = ref({ id: 0, username: '' })
const users = ref([])
const opsOpen = ref(false)
const newUser = ref({ username: '', password: '' })
const userError = ref('')
const confirmDelUser = ref(null)
const pwOpen = ref(false)
const pwForm = ref({ current: '', next: '' })
const pwMsg = ref('')
const pwError = ref('')
const advOpen = ref(false)
const backups = ref([])
const backupKeep = ref(7)
const backupBusy = ref(false)
const backupMsg = ref('')
const backupOk = ref(true)
const version = ref({ app: '', schema: '' })
const health = ref(null)

const healthLow = computed(() => !!health.value?.disk?.low || health.value?.db?.ok === false)

const healthLine = computed(() => {
  const h = health.value
  if (!h) return '불러오는 중…'
  const db = h.db?.ok ? '정상' : '오류'
  const disk = h.disk?.totalBytes ? `여유 ${h.disk.freePct.toFixed(0)}%` : '여유 확인 불가'
  const ex = h.executors ? `${h.executors.online}/${h.executors.total}대 연결` : ''
  const wait = (h.teamQueue || []).reduce((n, t) => n + (t.pending || 0), 0)
  const errs = (h.recentErrors || []).length
  return `DB ${db} · 디스크 ${disk} · 실행기 ${ex} · 대기 ${wait}건 · 최근 오류 ${errs}건`
})

async function loadHealth() {
  try {
    health.value = await apiFetch('/api/system/health')
  } catch {
    health.value = null
  }
}

watch(advOpen, (open) => { if (open) loadHealth() })

let savedTimer = null
function flash(which) {
  if (which === 'auto') savedAuto.value = true
  else savedAdv.value = true
  if (savedTimer) clearTimeout(savedTimer)
  savedTimer = setTimeout(() => { savedAuto.value = false; savedAdv.value = false }, 2000)
}

const lastBackup = computed(() => {
  const name = backups.value[0] || ''
  const m = name.match(/^\d{8}-(\d{2})(\d{2})/)
  return m ? `${m[1]}:${m[2]}` : '없음'
})

function validInt(v, min, max) {
  return Number.isInteger(v) && v >= min && v <= max
}

function validate(section) {
  const e = {}
  const f = form.value
  if (section === 'auto') {
    if (!validInt(f.publishIntervalMin, 0, 1440)) e.interval = '0~1440분 사이로 입력하세요.'
    if (!validInt(f.dailyCap, 0, 999)) e.cap = '0~999건 사이로 입력하세요.'
    if (!/^\d{2}:\d{2}$/.test(f.windowStart || '') || !/^\d{2}:\d{2}$/.test(f.windowEnd || '')) e.window = '시각을 HH:MM으로 입력하세요.'
    else if (f.windowStart === f.windowEnd) e.window = '시작과 종료가 같을 수 없습니다.'
    if (!validInt(f.blogMinLen, 0, 20000)) e.blog = '0~20000자 사이로 입력하세요.'
    if (!validInt(f.maxConcurrent, 1, 50)) e.conc = '1~50건 사이로 입력하세요.'
  } else {
    if (!validInt(f.mediaRetentionDays, 1, 365)) e.retention = '1~365일 사이로 입력하세요.'
  }
  errors.value = e
  return !Object.keys(e).length
}

async function save(section) {
  errors.value = { ...errors.value, form: '' }
  if (!validate(section)) return
  const flag = section === 'auto' ? savingAuto : savingAdv
  flag.value = true
  try {
    const saved = await apiFetch('/api/settings/automation', { method: 'PUT', body: JSON.stringify(form.value) })
    form.value = { ...saved, outputs: { ...saved.outputs } }
    flash(section)
    toast('저장했습니다')
  } catch (e) {
    errors.value = { ...errors.value, form: e.message || '저장 실패' }
    toast(e.message || '저장 실패', 'error')
  } finally {
    flag.value = false
  }
}

const saveAutomation = () => save('auto')
const saveAdvanced = () => save('adv')

async function load() {
  loading.value = true
  loadError.value = false
  try {
    const [s, u, bl, v] = await Promise.all([
      apiFetch('/api/settings/automation'),
      apiFetch('/api/users'),
      apiFetch('/api/system/backups'),
      apiFetch('/api/system/version'),
    ])
    form.value = { ...s, outputs: { ...(s.outputs || {}) } }
    if (form.value.backupIncludeSessions === undefined) form.value.backupIncludeSessions = true
    users.value = u.users || []
    backups.value = bl.backups || []
    backupKeep.value = bl.keep || 7
    version.value = v
    const self = await apiFetch('/api/auth/me')
    me.value = { id: self.id, username: self.username }
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

async function runBackup() {
  backupBusy.value = true
  backupMsg.value = ''
  try {
    await apiFetch('/api/system/backup', { method: 'POST' })
    const bl = await apiFetch('/api/system/backups')
    backups.value = bl.backups || []
    backupKeep.value = bl.keep || backupKeep.value
    backupOk.value = true
    backupMsg.value = '백업했습니다.'
  } catch (e) {
    backupOk.value = false
    backupMsg.value = e.message || '백업 실패'
  } finally {
    backupBusy.value = false
  }
}

async function addUser() {
  userError.value = ''
  try {
    await apiFetch('/api/users', { method: 'POST', body: JSON.stringify(newUser.value) })
    newUser.value = { username: '', password: '' }
    const u = await apiFetch('/api/users')
    users.value = u.users || []
    toast('운영자를 추가했습니다')
  } catch (e) {
    userError.value = e.message
  }
}

async function doRemoveUser() {
  const id = confirmDelUser.value
  confirmDelUser.value = null
  userError.value = ''
  try {
    await apiFetch(`/api/users/${id}`, { method: 'DELETE' })
    const u = await apiFetch('/api/users')
    users.value = u.users || []
  } catch (e) {
    userError.value = e.message
  }
}

async function changePassword() {
  pwMsg.value = ''
  pwError.value = ''
  try {
    await apiFetch('/api/auth/password', { method: 'PATCH', body: JSON.stringify(pwForm.value) })
    pwForm.value = { current: '', next: '' }
    pwMsg.value = '비밀번호를 변경했습니다.'
  } catch (e) {
    pwError.value = e.message
  }
}

function onKey(e) {
  if (e.key === 'Escape') {
    pwOpen.value = false
    confirmDelUser.value = null
  }
}

onMounted(() => {
  load()
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<style scoped>
.settings { display: flex; flex-direction: column; gap: var(--s4); min-width: 0; }
.page-head { display: flex; align-items: baseline; gap: var(--s3); flex-wrap: wrap; }
.sticky-head {
  position: sticky;
  top: 60px;
  z-index: 30;
  background: var(--bg);
  padding: var(--s2) 0;
}
.ctx { overflow-wrap: anywhere; }
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
.center { align-items: center; text-align: center; }
.frow { display: flex; align-items: center; gap: var(--s3); min-width: 0; }
.frow > span:first-child { flex: none; min-width: 120px; font-weight: 700; }
.frow > span, .frow .muted { min-width: 0; overflow-wrap: anywhere; }
.frow button:last-child, .frow .muted { margin-left: auto; }
.warnline { color: var(--warn); font-weight: 700; }
.health-box {
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.health-box.low { background: var(--warn-soft); border-color: var(--warn); color: var(--warn); }
.frow.wrap { flex-wrap: wrap; }
.grid { display: grid; grid-template-columns: 1fr; gap: var(--s3); }
.grid > label { min-width: 0; }
@media (min-width: 700px) {
  .grid { grid-template-columns: repeat(2, 1fr); }
  .grid > .span2 { grid-column: span 2; }
}
label { display: flex; flex-direction: column; gap: var(--s1); font-weight: 700; font-size: 13px; }
.inrow { display: flex; align-items: center; gap: var(--s2); font-weight: 400; flex-wrap: wrap; }
.inrow input { width: 90px; }
input[type="time"] { width: auto; min-width: 0; max-width: 100%; }
label.inline { flex-direction: row; align-items: center; font-weight: 400; }
input.narrow { width: 70px; }
.hint { font-weight: 400; font-size: 12px; color: var(--muted); }
fieldset { border: 1px solid var(--line); border-radius: var(--r-md); padding: var(--s2) var(--s3) var(--s3); display: flex; gap: var(--s4); }
legend { font-size: 13px; font-weight: 700; padding: 0 var(--s1); }
.check { flex-direction: row; align-items: center; font-weight: 400; }
.switch {
  display: inline-flex;
  align-items: center;
  gap: var(--s2);
  border: 1px solid var(--line);
  background: var(--soft);
  color: var(--ink);
  border-radius: 999px;
  padding: var(--s1) var(--s3) var(--s1) var(--s1);
  font-weight: 700;
}
.switch .knob { width: 28px; height: 18px; border-radius: 999px; background: var(--muted); transition: background var(--t); }
.switch.on { border-color: var(--ok); }
.switch.on .knob { background: var(--ok); }
.btnrow { display: flex; align-items: center; gap: var(--s3); }
.ops { display: flex; flex-direction: column; gap: var(--s3); border-top: 1px solid var(--line-soft); padding-top: var(--s3); }
.userlist { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
.userlist li { display: flex; align-items: center; justify-content: space-between; gap: var(--s2); padding: var(--s2) 0; border-bottom: 1px solid var(--line-soft); min-width: 0; }
.userlist li > span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.role { background: var(--soft); border-radius: 999px; padding: 0 10px; font-size: 12px; }
.addrow { display: flex; gap: var(--s3); align-items: flex-end; flex-wrap: wrap; }
.addrow label { flex: 1; min-width: 160px; }
.advhead {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  background: none;
  border: none;
  color: var(--ink);
  padding: 0;
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
  border: 1px solid var(--line);
  border-radius: var(--r-lg);
  box-shadow: var(--shadow-lg);
  padding: var(--s5);
  width: 100%;
  max-width: 440px;
  display: flex;
  flex-direction: column;
  gap: var(--s3);
}
.modal h3 { margin: 0; }
.modal .btnrow { justify-content: flex-end; }
input {
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: var(--s2) var(--s3);
  background: var(--panel);
  font-weight: 400;
}
input[type="checkbox"] { width: auto; accent-color: var(--accent-dark); }
@media (min-width: 1150px) {
  .settings { display: grid; grid-template-columns: 7fr 5fr; align-items: start; }
  .settings > .page-head { grid-column: 1 / -1; }
  .settings > section:nth-of-type(1) { grid-row: span 2; }
}
@media (max-width: 900px) {
  .sticky-head { top: 52px; }
}
</style>
