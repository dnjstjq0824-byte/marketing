<template>
  <div class="shell">
    <a class="skip" href="#main">본문으로 건너뛰기</a>
    <aside class="side">
      <RouterLink to="/" class="brand">
        <span class="mark" aria-hidden="true">●</span>
        <span class="brand-t"><strong>SOMUN</strong><small>마케팅 코어</small></span>
      </RouterLink>
      <RouterLink to="/new" class="newbtn"><span class="mi" aria-hidden="true">＋</span> <span class="lbl">새 작업</span></RouterLink>
      <nav aria-label="주 메뉴">
        <RouterLink to="/" :aria-current="isActive('/') ? 'page' : undefined"><span class="mi" aria-hidden="true">⌂</span><span class="lbl">홈</span></RouterLink>
        <RouterLink to="/works" :aria-current="isActive('/works') ? 'page' : undefined"><span class="mi" aria-hidden="true">▤</span><span class="lbl">작업</span></RouterLink>
        <RouterLink to="/media" :aria-current="isActive('/media') ? 'page' : undefined"><span class="mi" aria-hidden="true">▣</span><span class="lbl">미디어</span></RouterLink>
        <RouterLink to="/teams" :aria-current="isActive('/teams') ? 'page' : undefined"><span class="mi" aria-hidden="true">◎</span><span class="lbl">팀</span></RouterLink>
        <RouterLink to="/connect" :aria-current="isActive('/connect') ? 'page' : undefined"><span class="mi" aria-hidden="true">⇄</span><span class="lbl">연결</span><span v-if="connectAlert" class="dot-alert" aria-label="연결 문제 있음"></span></RouterLink>
        <RouterLink to="/settings" :aria-current="isActive('/settings') ? 'page' : undefined"><span class="mi" aria-hidden="true">⚙</span><span class="lbl">설정</span></RouterLink>
      </nav>
      <div class="golinks" aria-label="홍보 위탁 바로가기">
        <span class="go-cap muted micro">홍보 위탁</span>
        <RouterLink to="/promo">홍보 메인</RouterLink>
        <RouterLink to="/promo/admin">위탁 관리</RouterLink>
      </div>
      <p class="side-foot muted micro">{{ todayLine }}</p>
    </aside>
    <div class="body">
      <header class="topbar">
        <strong class="title">{{ title }}</strong>
        <div class="top-right">
          <button v-if="automation !== null" class="auto-chip" :class="{ off: !automation }" @click="confirmStop = true" :aria-label="automation ? '자동화 켜짐, 멈추려면 클릭' : '자동화 멈춤, 켜려면 클릭'">
            <span class="dot" :class="{ on: automation }" aria-hidden="true"></span>{{ automation ? '자동화 켜짐' : '멈춤' }}
          </button>
          <button class="iconbtn" @click="bellOpen = !bellOpen; if (bellOpen) loadNotes()" aria-label="알림">
            🔔<span v-if="unread > 0" class="bdg">{{ unread > 99 ? '99+' : unread }}</span>
          </button>
          <div class="opwrap">
            <button class="iconbtn" @click="opOpen = !opOpen" aria-label="운영자 메뉴">👤</button>
            <div v-if="opOpen" class="opdrop" role="menu">
              <RouterLink to="/settings" role="menuitem" @click="opOpen = false">비밀번호 변경</RouterLink>
              <button role="menuitem" @click="doLogout">로그아웃</button>
            </div>
          </div>
        </div>
        <div v-if="bellOpen" class="belldrop" role="dialog" aria-label="알림 목록">
          <div class="sect-head"><h3>알림</h3><button v-if="unread > 0" class="ghost small" @click="readAll">모두 읽음</button></div>
          <ul v-if="notes.length" class="notelist">
            <li v-for="n in notes" :key="n.id">
              <button class="note" @click="openNote(n)">
                <span class="dot" :class="{ on: !n.readAt && n.severity !== 'info' }" aria-hidden="true"></span>
                <span class="nmsg">{{ n.message }}</span>
                <span class="ntime">{{ shortTime(n.createdAt) }}</span>
              </button>
            </li>
          </ul>
          <p v-else class="empty">알림이 없습니다.</p>
        </div>
      </header>
      <div v-if="!online" class="offlineband">연결이 끊겼습니다 · 재연결 중</div>
      <main id="main" class="main">
        <slot />
      </main>
      <nav class="tabs" aria-label="하단 메뉴">
        <RouterLink to="/">⌂<small>홈</small></RouterLink>
        <RouterLink to="/works">▤<small>작업</small></RouterLink>
        <RouterLink to="/new" class="fab" aria-label="새 작업">＋</RouterLink>
        <RouterLink to="/teams">◎<small>팀</small></RouterLink>
        <button @click="moreOpen = true" aria-label="더보기">☰<small>더보기</small><span v-if="connectAlert" class="dot-alert" aria-hidden="true"></span></button>
      </nav>
      <div v-if="moreOpen" class="sheet-back" @click.self="moreOpen = false">
        <div class="sheet" role="dialog" aria-label="더보기">
          <RouterLink to="/media" @click="moreOpen = false">미디어</RouterLink>
          <RouterLink to="/connect" @click="moreOpen = false">연결<span v-if="connectAlert" class="dot-alert" aria-label="연결 문제 있음"></span></RouterLink>
          <RouterLink to="/settings" @click="moreOpen = false">설정</RouterLink>
          <RouterLink to="/promo/admin" @click="moreOpen = false">위탁 관리</RouterLink>
          <button @click="doLogout">로그아웃</button>
        </div>
      </div>
      <div v-if="confirmStop" class="modal-back" @click.self="confirmStop = false">
        <div class="modal" role="alertdialog" aria-label="자동화 멈추기 확인">
          <h3>자동화를 멈출까요?</h3>
          <p class="muted">새 작업 진행과 발행이 모두 멈춥니다. 진행 중인 건은 끝까지 처리됩니다.</p>
          <div class="btnrow">
            <button class="ghost" @click="confirmStop = false">취소</button>
            <button class="danger" @click="setAutomation(false)">멈추기</button>
          </div>
        </div>
      </div>
      <Toast />
    </div>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiFetch, auth, logout } from '../stores/auth.js'
import { toast } from '../stores/toast.js'
import Toast from './Toast.vue'

defineProps({ title: { type: String, default: '' } })

const route = useRoute()
const router = useRouter()
const bellOpen = ref(false)
const opOpen = ref(false)
const moreOpen = ref(false)
const confirmStop = ref(false)
const notes = ref([])
const unread = ref(0)
const automation = ref(null)
const connectAlert = ref(false)
const todayLine = ref('')
const online = ref(typeof navigator === 'undefined' ? true : navigator.onLine)

function isActive(to) {
  return to === '/' ? route.path === '/' : route.path.startsWith(to)
}

function shortTime(iso) {
  return (iso || '').slice(5, 16).replace('T', ' ')
}

async function loadNotes() {
  try {
    const data = await apiFetch('/api/notifications?limit=20')
    notes.value = data.items || []
    unread.value = data.unread || 0
  } catch {
    notes.value = []
    unread.value = 0
  }
}

async function readAll() {
  try {
    await apiFetch('/api/notifications/read-all', { method: 'POST' })
  } catch { /* refresh below */ }
  await loadNotes()
}

async function openNote(n) {
  if (!n.readAt) {
    try {
      await apiFetch(`/api/notifications/${n.id}/read`, { method: 'POST' })
    } catch { /* navigate anyway */ }
    await loadNotes()
  }
  bellOpen.value = false
  if ((n.kind || '').startsWith('session')) router.push('/connect')
  else if (n.kind === 'publish_failed') router.push('/works')
  else router.push('/')
}

async function loadAutomation() {
  try {
    const data = await apiFetch('/api/automation')
    automation.value = !!data.enabled
  } catch {
    automation.value = null
  }
}

async function setAutomation(on) {
  confirmStop.value = false
  if (on === automation.value) return
  try {
    await apiFetch('/api/automation', { method: 'PUT', body: JSON.stringify({ enabled: on }) })
    automation.value = on
    toast(on ? '자동화를 다시 켰습니다' : '자동화를 멈췄습니다')
  } catch (e) {
    toast(e.message || '저장 실패', 'error')
  }
}

async function loadSide() {
  try {
    const data = await apiFetch('/api/overview')
    const n = (data.today && data.today.publishedToday) || 0
    todayLine.value = n ? `오늘 발행 ${n}건` : ''
    connectAlert.value = (data.actions || []).some((a) => a.kind === 'login' || a.kind === 'account')
  } catch {
    todayLine.value = ''
  }
}

async function doLogout() {
  opOpen.value = false
  moreOpen.value = false
  await logout()
  router.push('/login')
}

function onOnline() { online.value = true }
function onOffline() { online.value = false }

let timer = null
onMounted(() => {
  loadAutomation()
  loadNotes()
  loadSide()
  window.addEventListener('online', onOnline)
  window.addEventListener('offline', onOffline)
  timer = setInterval(() => { loadNotes() }, 60000)
})
onUnmounted(() => {
  window.removeEventListener('online', onOnline)
  window.removeEventListener('offline', onOffline)
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.shell { display: flex; min-height: 100vh; }
.side {
  width: 236px;
  flex-shrink: 0;
  background: var(--panel);
  border-right: 1px solid var(--line-soft);
  box-shadow: 2px 0 12px -8px rgba(29, 39, 51, 0.25);
  padding: var(--s5) var(--s4);
  display: flex;
  flex-direction: column;
  gap: var(--s4);
  position: sticky;
  top: 0;
  height: 100vh;
}
.brand { display: flex; align-items: center; gap: var(--s3); text-decoration: none; color: var(--ink); padding: var(--s1) var(--s2); }
.brand .mark {
  flex: none;
  width: 34px;
  height: 34px;
  border-radius: 10px;
  background: var(--accent);
  color: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 19px;
}
.brand-t { display: flex; flex-direction: column; line-height: 1.25; }
.brand-t strong { font-size: 19px; letter-spacing: 1.5px; }
.brand-t small { font-size: 12px; color: var(--muted); }
.newbtn {
  display: block;
  text-align: center;
  background: var(--accent);
  color: #fff;
  border-radius: var(--r-md);
  padding: var(--s3);
  text-decoration: none;
  font-weight: 700;
  font-size: 15px;
  box-shadow: var(--shadow);
}
.newbtn:hover { background: var(--accent-dark); color: #fff; }
.side nav { display: flex; flex-direction: column; gap: 2px; }
.side nav a {
  display: flex;
  align-items: center;
  gap: var(--s3);
  padding: var(--s3);
  border-radius: var(--r-md);
  text-decoration: none;
  color: var(--ink-soft);
  font-size: 15px;
}
.side nav a { transition: background 0.12s ease; }
.side nav a:hover { background: var(--soft); color: var(--ink); }
.side nav a[aria-current="page"] {
  background: var(--accent-soft);
  color: var(--ink);
  font-weight: 800;
  box-shadow: inset 3px 0 0 var(--accent);
}
.side nav .mi { width: 22px; text-align: center; font-size: 17px; }
.golinks {
  display: flex;
  flex-direction: column;
  gap: 2px;
  border-top: 1px solid var(--line-soft);
  padding-top: var(--s3);
  margin-top: var(--s2);
}
.go-cap { padding: 0 var(--s3); }
.golinks a {
  padding: var(--s2) var(--s3);
  border-radius: var(--r-md);
  text-decoration: none;
  color: var(--ink-soft);
  font-size: 14px;
}
.golinks a:hover { background: var(--soft); color: var(--ink); }
.golinks a.router-link-active { color: var(--accent-dark); font-weight: 800; }
.side-foot { margin-top: auto; }
.body { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.topbar {
  height: 60px;
  display: flex;
  align-items: center;
  gap: var(--s4);
  padding: 0 var(--s6);
  border-bottom: 1px solid var(--line-soft);
  box-shadow: 0 1px 2px rgba(29, 39, 51, 0.06);
  background: var(--panel);
  position: sticky;
  top: 0;
  z-index: 50;
}
.topbar .title { font-size: 20px; font-weight: 800; letter-spacing: -0.3px; }
.top-right { margin-left: auto; display: flex; align-items: center; gap: var(--s3); }
.auto-chip {
  display: flex;
  align-items: center;
  gap: var(--s2);
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
  border-radius: 999px;
  padding: var(--s2) var(--s4);
  font-size: 14px;
  font-weight: 600;
}
.auto-chip:hover { background: var(--soft); }
.auto-chip.off { color: var(--muted); }
.dot { width: 8px; height: 8px; border-radius: 50%; background: #bbb; display: inline-block; }
.dot.on { background: var(--ok); }
.dot-alert { width: 8px; height: 8px; border-radius: 50%; background: var(--warn); display: inline-block; margin-left: var(--s1); }
.iconbtn {
  border: 1px solid transparent;
  background: none;
  color: var(--ink);
  border-radius: var(--r-md);
  padding: var(--s2);
  font-size: 19px;
  position: relative;
}
.iconbtn:hover { background: var(--soft); border-color: var(--line-soft); }
.bdg {
  position: absolute;
  top: 0;
  right: 0;
  background: var(--warn);
  color: #fff;
  font-size: 11px;
  border-radius: 999px;
  padding: 0 5px;
}
.opwrap { position: relative; }
.opdrop, .belldrop {
  position: absolute;
  right: 0;
  top: calc(100% + var(--s2));
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--r-lg);
  box-shadow: var(--shadow-lg);
  padding: var(--s3);
  z-index: 100;
}
.opdrop { display: flex; flex-direction: column; min-width: 160px; }
.opdrop a, .opdrop button {
  text-align: left;
  background: none;
  border: none;
  padding: var(--s2) var(--s3);
  text-decoration: none;
  color: var(--ink);
  border-radius: var(--r-sm);
}
.belldrop { width: 360px; max-width: calc(100vw - 32px); }
.notelist { list-style: none; margin: 0; padding: 0; max-height: 320px; overflow: auto; }
.note { display: flex; gap: var(--s2); align-items: baseline; width: 100%; background: none; border: none; color: var(--ink); text-align: left; padding: var(--s2); }
.nmsg { flex: 1; }
.ntime { color: var(--muted); font-size: 12px; white-space: nowrap; }
.offlineband { background: var(--ink-soft); color: #fff; text-align: center; padding: var(--s2); font-size: 13px; }
.main { padding: var(--s6); width: 100%; }
.tabs { display: none; }
.sheet-back {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 150;
  display: flex;
  align-items: flex-end;
}
.sheet { background: var(--panel); width: 100%; border-radius: var(--r-lg) var(--r-lg) 0 0; padding: var(--s4); display: flex; flex-direction: column; }
.sheet a, .sheet button {
  padding: var(--s4);
  text-decoration: none;
  color: var(--ink);
  background: none;
  background: none;
  border: none;
  border-bottom: 1px solid var(--line-soft);
  text-align: left;
  font-size: 16px;
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
.modal { background: var(--panel); border-radius: var(--r-lg); box-shadow: var(--shadow-lg); padding: var(--s5); max-width: 440px; width: 100%; }
.btnrow { display: flex; gap: var(--s3); justify-content: flex-end; margin-top: var(--s4); }
.btnrow button { border-radius: var(--r-md); padding: var(--s2) var(--s4); border: 1px solid var(--line); background: var(--panel); color: var(--ink); }
.btnrow button.danger { background: var(--warn); border-color: var(--warn); color: #fff; }
@media (max-width: 900px) {
  .side { display: none; }
  .topbar { height: 52px; padding: 0 var(--s4); }
  .topbar .title { font-size: 16px; }
  .main { padding: var(--s4) var(--s4) 88px; }
  .tabs {
    display: flex;
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: calc(64px + env(safe-area-inset-bottom));
    padding-bottom: env(safe-area-inset-bottom);
    background: var(--panel);
    border-top: 1px solid var(--line);
    z-index: 100;
  }
  .tabs a, .tabs button {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-decoration: none;
    color: var(--ink-soft);
    background: none;
    border: none;
    font-size: 20px;
    min-height: 44px;
    min-width: 44px;
  }
  .tabs small { font-size: 11px; }
  .tabs a.router-link-active { color: var(--accent-dark); font-weight: 800; }
  .tabs .fab {
    background: var(--accent);
    color: #fff;
    border-radius: 50%;
    width: 56px;
    height: 56px;
    flex: none;
    margin: -20px auto 0;
    font-size: 28px;
  }
}
@media (min-width: 901px) and (max-width: 1279px) {
  .side { width: 64px; padding: var(--s5) var(--s2); align-items: center; }
  .golinks { display: none; }
  .side .brand { justify-content: center; padding: 0; }
  .side .brand-t, .side .lbl, .side-foot { display: none; }
  .side nav a { justify-content: center; padding: var(--s2); }
  .side nav a[aria-current="page"] { box-shadow: none; }
  .newbtn { width: 44px; height: 44px; padding: 0; border-radius: 50%; font-size: 20px; }
}
</style>
