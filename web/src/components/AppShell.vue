<template>
  <div class="shell" :class="{ 'lnb-min': collapsed }">
    <a class="skip" href="#main">본문으로 건너뛰기</a>
    <aside class="side" aria-label="작업 공간 메뉴">
      <div class="side-head">
        <RouterLink to="/" class="brand">
          <LogoMark :size="34" />
          <span class="brand-t"><strong>SOMUN</strong><small>마케팅 코어</small></span>
        </RouterLink>
        <button class="fold" :class="{ flip: collapsed }" @click="toggleCollapse" :aria-label="collapsed ? '메뉴 펼치기' : '메뉴 접기'" :aria-expanded="(!collapsed).toString()" v-html="icon('chevL', 16)"></button>
      </div>
      <RouterLink to="/new" class="newbtn" :aria-label="'새 작업'"><span class="mi" v-html="icon('plus')" aria-hidden="true"></span> <span class="lbl">새 작업</span></RouterLink>
      <nav aria-label="주 메뉴">
        <RouterLink to="/" :aria-current="isActive('/') ? 'page' : undefined"><span class="mi" v-html="icon('home')" aria-hidden="true"></span><span class="lbl">홈</span></RouterLink>
        <RouterLink to="/works" :aria-current="isActive('/works') ? 'page' : undefined"><span class="mi" v-html="icon('works')" aria-hidden="true"></span><span class="lbl">작업</span></RouterLink>
        <RouterLink to="/media" :aria-current="isActive('/media') ? 'page' : undefined"><span class="mi" v-html="icon('media')" aria-hidden="true"></span><span class="lbl">미디어</span></RouterLink>
        <RouterLink to="/teams" :aria-current="isActive('/teams') ? 'page' : undefined"><span class="mi" v-html="icon('teams')" aria-hidden="true"></span><span class="lbl">팀</span></RouterLink>
        <RouterLink to="/connect" :aria-current="isActive('/connect') ? 'page' : undefined"><span class="mi" v-html="icon('connect')" aria-hidden="true"></span><span class="lbl">연결</span><span v-if="connectAlert" class="dot-alert" aria-label="연결 문제 있음"></span></RouterLink>
        <RouterLink to="/settings" :aria-current="isActive('/settings') ? 'page' : undefined"><span class="mi" v-html="icon('settings')" aria-hidden="true"></span><span class="lbl">설정</span></RouterLink>
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
        <button class="iconbtn menu" @click="drawerOpen = true" aria-label="메뉴 열기" v-html="icon('menu')"></button>
        <div class="crumbbox">
          <nav class="crumbs" aria-label="현재 위치">
            <template v-for="(c, i) in crumbs" :key="i">
              <span v-if="i > 0" class="sep" aria-hidden="true">/</span>
              <RouterLink v-if="c.to" :to="c.to">{{ c.label }}</RouterLink>
              <span v-else class="cur" aria-current="page">{{ c.label }}</span>
            </template>
          </nav>
          <strong class="title">{{ title }}</strong>
        </div>
        <div class="top-right">
          <button v-if="automation !== null" class="auto-chip" :class="{ off: !automation }" @click="confirmStop = true" :aria-label="automation ? '자동화 켜짐, 멈추려면 클릭' : '자동화 멈춤, 켜려면 클릭'">
            <span class="dot" :class="{ on: automation }" aria-hidden="true"></span>{{ automation ? '자동화 켜짐' : '멈춤' }}
          </button>
          <button class="iconbtn" @click="bellOpen = !bellOpen; if (bellOpen) loadNotes()" aria-label="알림">
            <span v-html="icon('bell')" aria-hidden="true"></span><span v-if="unread > 0" class="bdg">{{ unread > 99 ? '99+' : unread }}</span>
          </button>
          <div class="opwrap">
            <button class="iconbtn" @click="opOpen = !opOpen" aria-label="운영자 메뉴" v-html="icon('user')"></button>
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
        <RouterLink to="/" :aria-current="isActive('/') ? 'page' : undefined"><span v-html="icon('home', 22)" aria-hidden="true"></span><small>홈</small></RouterLink>
        <RouterLink to="/works" :aria-current="isActive('/works') ? 'page' : undefined"><span v-html="icon('works', 22)" aria-hidden="true"></span><small>작업</small></RouterLink>
        <RouterLink to="/new" class="fab" aria-label="새 작업" v-html="icon('plus', 26)"></RouterLink>
        <RouterLink to="/teams" :aria-current="isActive('/teams') ? 'page' : undefined"><span v-html="icon('teams', 22)" aria-hidden="true"></span><small>팀</small></RouterLink>
        <RouterLink to="/media" :aria-current="isActive('/media') ? 'page' : undefined"><span v-html="icon('media', 22)" aria-hidden="true"></span><small>미디어</small></RouterLink>
      </nav>
      <div v-if="drawerOpen" class="drawer-back" @click.self="drawerOpen = false">
        <div class="drawer" role="dialog" aria-label="전체 메뉴">
          <div class="drawer-head">
            <strong>SOMUN</strong>
            <button class="iconbtn" @click="drawerOpen = false" aria-label="메뉴 닫기" v-html="icon('close')"></button>
          </div>
          <RouterLink to="/new" class="newbtn" @click="drawerOpen = false"><span v-html="icon('plus', 18)" aria-hidden="true"></span> 새 작업</RouterLink>
          <nav aria-label="전체 메뉴 목록">
            <RouterLink to="/" :aria-current="isActive('/') ? 'page' : undefined" @click="drawerOpen = false"><span v-html="icon('home')" aria-hidden="true"></span> 홈</RouterLink>
            <RouterLink to="/works" :aria-current="isActive('/works') ? 'page' : undefined" @click="drawerOpen = false"><span v-html="icon('works')" aria-hidden="true"></span> 작업</RouterLink>
            <RouterLink to="/media" :aria-current="isActive('/media') ? 'page' : undefined" @click="drawerOpen = false"><span v-html="icon('media')" aria-hidden="true"></span> 미디어</RouterLink>
            <RouterLink to="/teams" :aria-current="isActive('/teams') ? 'page' : undefined" @click="drawerOpen = false"><span v-html="icon('teams')" aria-hidden="true"></span> 팀</RouterLink>
            <RouterLink to="/connect" :aria-current="isActive('/connect') ? 'page' : undefined" @click="drawerOpen = false"><span v-html="icon('connect')" aria-hidden="true"></span> 연결<span v-if="connectAlert" class="dot-alert" aria-label="연결 문제 있음"></span></RouterLink>
            <RouterLink to="/settings" :aria-current="isActive('/settings') ? 'page' : undefined" @click="drawerOpen = false"><span v-html="icon('settings')" aria-hidden="true"></span> 설정</RouterLink>
          </nav>
          <div class="golinks" aria-label="홍보 위탁 바로가기">
            <span class="go-cap muted micro">홍보 위탁</span>
            <RouterLink to="/promo" @click="drawerOpen = false">홍보 메인</RouterLink>
            <RouterLink to="/promo/admin" @click="drawerOpen = false">위탁 관리</RouterLink>
          </div>
          <button class="logout" @click="doLogout">로그아웃</button>
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
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiFetch, auth, logout } from '../stores/auth.js'
import { toast } from '../stores/toast.js'
import Toast from './Toast.vue'
import LogoMark from './LogoMark.vue'

defineProps({ title: { type: String, default: '' } })

const ICON_PATHS = {
  home: '<path d="M4.5 11.5 12 4.5l7.5 7M6.5 10.5V19.5h11v-9"/>',
  works: '<path d="M12 3.5 20 7.5l-8 4-8-4z"/><path d="M4.5 12.5l7.5 3.7 7.5-3.7"/><path d="M4.5 16.5 12 20.2l7.5-3.7"/>',
  media: '<rect x="4" y="5" width="16" height="14" rx="2.5"/><circle cx="9" cy="10" r="1.6"/><path d="M4.5 17.5 10 12.5l3.5 3 2.5-2 3.5 2.5"/>',
  teams: '<circle cx="9" cy="8" r="3"/><path d="M3.5 19c.6-3.2 2.8-5 5.5-5s4.9 1.8 5.5 5"/><circle cx="17" cy="9" r="2.4"/><path d="M15.7 14.3c2.7.2 4.3 1.7 4.8 4.7"/>',
  connect: '<path d="M4 8.5h11M13 5.5l3 3-3 3"/><path d="M20 15.5H9M11 12.5l-3 3 3 3"/>',
  settings: '<circle cx="12" cy="12" r="3.2"/><path d="M12 2.8v2.6M12 18.6v2.6M2.8 12h2.6M18.6 12h2.6M5.5 5.5l1.8 1.8M16.7 16.7l1.8 1.8M18.5 5.5l-1.8 1.8M7.3 16.7l-1.8 1.8"/>',
  plus: '<path d="M12 5.5v13M5.5 12h13"/>',
  bell: '<path d="M6 16.5v-5.2a6 6 0 0 1 12 0v5.2l1.5 2.5h-15z"/><path d="M10.2 21a2.1 2.1 0 0 0 3.6 0"/>',
  user: '<circle cx="12" cy="8" r="3.6"/><path d="M5.3 19.8c1-3.4 3.6-5.3 6.7-5.3s5.7 1.9 6.7 5.3"/>',
  menu: '<path d="M5 7.5h14M5 12h14M5 16.5h14"/>',
  close: '<path d="M6.5 6.5l11 11M17.5 6.5l-11 11"/>',
  chevL: '<path d="M14.5 6 8.5 12l6 6"/>',
}
function icon(name, size = 20) {
  return `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICON_PATHS[name] || ''}</svg>`
}

const route = useRoute()
const router = useRouter()
const bellOpen = ref(false)
const opOpen = ref(false)
const drawerOpen = ref(false)
const confirmStop = ref(false)
const notes = ref([])
const unread = ref(0)
const automation = ref(null)
const connectAlert = ref(false)
const todayLine = ref('')
const online = ref(typeof navigator === 'undefined' ? true : navigator.onLine)
const collapsed = ref(false)

const SECTIONS = {
  '/': '홈',
  '/new': '새 작업',
  '/works': '작업',
  '/media': '미디어',
  '/teams': '팀',
  '/connect': '연결',
  '/settings': '설정',
}
const crumbs = computed(() => {
  const list = [{ to: '/', label: '홈' }]
  if (route.path === '/') return [{ label: '홈' }]
  if (route.path.startsWith('/works/')) {
    list.push({ to: '/works', label: '작업' }, { label: '상세' })
    return list
  }
  if (route.path.startsWith('/teams/')) {
    list.push({ to: '/teams', label: '팀' }, { label: '상세' })
    return list
  }
  const seg = '/' + route.path.split('/')[1]
  list.push({ label: SECTIONS[seg] || '작업 공간' })
  return list
})

function toggleCollapse() {
  collapsed.value = !collapsed.value
  try {
    localStorage.setItem('somun.lnb', collapsed.value ? 'min' : 'full')
  } catch { /* private mode */ }
}

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
  drawerOpen.value = false
  await logout()
  router.push('/login')
}

function onOnline() { online.value = true }
function onOffline() { online.value = false }
function onKey(e) {
  if (e.key === 'Escape') {
    drawerOpen.value = false
    bellOpen.value = false
    opOpen.value = false
  }
}

let timer = null
onMounted(() => {
  try {
    collapsed.value = localStorage.getItem('somun.lnb') === 'min'
  } catch { /* private mode */ }
  loadAutomation()
  loadNotes()
  loadSide()
  window.addEventListener('online', onOnline)
  window.addEventListener('offline', onOffline)
  document.addEventListener('keydown', onKey)
  timer = setInterval(() => { loadNotes() }, 60000)
})
onUnmounted(() => {
  window.removeEventListener('online', onOnline)
  window.removeEventListener('offline', onOffline)
  document.removeEventListener('keydown', onKey)
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
  transition: width var(--t);
}
.side-head { display: flex; align-items: center; gap: var(--s2); }
.brand { display: flex; align-items: center; gap: var(--s3); text-decoration: none; color: var(--ink); padding: var(--s1) var(--s2); flex: 1; min-width: 0; }
.brand-t { display: flex; flex-direction: column; line-height: 1.25; }
.brand-t strong { font-size: 19px; letter-spacing: 1.5px; }
.brand-t small { font-size: 12px; color: var(--muted); }
.fold {
  flex: none;
  border: 1px solid var(--line-soft);
  background: none;
  color: var(--muted);
  border-radius: var(--r-sm);
  width: 28px;
  height: 28px;
  padding: 0;
  margin: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: transform var(--t);
}
.fold.flip { transform: rotate(180deg); }
@media (prefers-reduced-motion: reduce) {
  .fold { transition: none; }
}
.fold:hover { background: var(--soft); color: var(--ink); }
.newbtn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
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
.side nav a { transition: background var(--t); }
.side nav a:hover { background: var(--soft); color: var(--ink); }
.side nav a[aria-current="page"] {
  background: var(--accent-soft);
  color: var(--ink);
  font-weight: 800;
  box-shadow: inset 3px 0 0 var(--accent);
}
.mi { display: inline-flex; align-items: center; justify-content: center; flex: none; }
.side nav .mi { width: 22px; height: 22px; }
.shell.lnb-min .side { width: 76px; padding: var(--s5) var(--s2); align-items: stretch; }
.shell.lnb-min .side-head { flex-direction: column; }
.shell.lnb-min .brand { justify-content: center; padding: 0; }
.shell.lnb-min .brand-t, .shell.lnb-min .lbl, .shell.lnb-min .side-foot, .shell.lnb-min .golinks { display: none; }
.shell.lnb-min .side nav a { justify-content: center; padding: var(--s2); }
.shell.lnb-min .side nav a[aria-current="page"] { box-shadow: none; }
.shell.lnb-min .newbtn { width: 44px; height: 44px; padding: 0; border-radius: 50%; font-size: 20px; margin: 0 auto; }
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
  min-height: 60px;
  display: flex;
  align-items: center;
  gap: var(--s4);
  padding: var(--s2) var(--s6);
  border-bottom: 1px solid var(--line-soft);
  box-shadow: 0 1px 2px rgba(29, 39, 51, 0.06);
  background: var(--panel);
  position: sticky;
  top: 0;
  z-index: 50;
}
.topbar .menu { display: none; }
.crumbbox { display: flex; flex-direction: column; gap: 1px; min-width: 0; }
.crumbs { display: flex; align-items: center; gap: 6px; font-size: 12px; color: var(--muted); }
.crumbs a { color: var(--muted); text-decoration: none; }
.crumbs a:hover { color: var(--accent-dark); }
.crumbs .cur { color: var(--ink-soft); font-weight: 700; }
.crumbs .sep { opacity: 0.6; }
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
.dot { width: 8px; height: 8px; border-radius: 50%; background: var(--muted); display: inline-block; }
.dot.on { background: var(--ok); }
.dot-alert { width: 8px; height: 8px; border-radius: 50%; background: var(--warn); display: inline-block; margin-left: var(--s1); }
.iconbtn {
  border: 1px solid transparent;
  background: none;
  color: var(--ink);
  border-radius: var(--r-md);
  padding: var(--s2);
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 40px;
  min-height: 40px;
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
.drawer-back {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 150;
}
.drawer {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 300px;
  max-width: 86vw;
  background: var(--panel);
  padding: var(--s4);
  display: flex;
  flex-direction: column;
  gap: var(--s3);
  overflow-y: auto;
  box-shadow: var(--shadow-lg);
}
.drawer-head { display: flex; align-items: center; justify-content: space-between; }
.drawer nav { display: flex; flex-direction: column; gap: 2px; }
.drawer nav a {
  padding: var(--s3);
  text-decoration: none;
  color: var(--ink-soft);
  border-radius: var(--r-md);
  font-size: 16px;
  display: flex;
  align-items: center;
  gap: 12px;
}
.drawer nav a > span { display: inline-flex; }
.drawer nav a:hover { background: var(--soft); color: var(--ink); }
.drawer nav a[aria-current="page"] { background: var(--accent-soft); color: var(--ink); font-weight: 800; }
.drawer .logout {
  margin-top: auto;
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
  border-radius: var(--r-md);
  padding: var(--s3);
  font-size: 15px;
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
  .topbar { min-height: 52px; padding: var(--s2) var(--s4); gap: var(--s2); }
  .topbar .menu { display: inline-flex; }
  .crumbs { font-size: 11px; }
  .topbar .title { font-size: 16px; }
  .auto-chip { font-size: 12px; padding: var(--s1) var(--s3); }
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
  .tabs a {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 3px;
    text-decoration: none;
    color: var(--ink-soft);
    min-height: 44px;
    min-width: 44px;
  }
  .tabs a > span { display: inline-flex; }
  .tabs small { font-size: 11px; }
  .tabs a[aria-current="page"] { color: var(--accent-dark); font-weight: 800; }
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
  .shell:not(.lnb-min) .side { width: 76px; padding: var(--s5) var(--s2); }
  .shell:not(.lnb-min) .golinks { display: none; }
  .shell:not(.lnb-min) .side .brand { justify-content: center; padding: 0; }
  .shell:not(.lnb-min) .side-head { flex-direction: column; }
  .shell:not(.lnb-min) .brand-t, .shell:not(.lnb-min) .lbl, .shell:not(.lnb-min) .side-foot { display: none; }
  .shell:not(.lnb-min) .side nav a { justify-content: center; padding: var(--s2); }
  .shell:not(.lnb-min) .side nav a[aria-current="page"] { box-shadow: none; }
  .shell:not(.lnb-min) .newbtn { width: 44px; height: 44px; padding: 0; border-radius: 50%; font-size: 20px; margin: 0 auto; }
}
</style>
