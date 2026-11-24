<template>
  <div class="shell" :class="{ 'lnb-min': collapsed }">
    <a class="skip" href="#main">본문으로 건너뛰기</a>
    <aside class="side" aria-label="작업 공간 메뉴">
      <div class="side-head">
        <RouterLink to="/" class="brand">
          <LogoMark :size="32" />
          <span class="brand-t"><strong>SOMUN</strong><small>AI 마케팅 엔진</small></span>
        </RouterLink>
        <button class="fold" :class="{ flip: collapsed }" @click="toggleCollapse" :aria-label="collapsed ? '메뉴 펼치기' : '메뉴 접기'" :aria-expanded="(!collapsed).toString()" v-html="icon('chevL', 15)"></button>
      </div>
      <RouterLink to="/new" class="newbtn" :aria-label="'새 작업 (단축키 N)'" title="새 작업 (N)">
        <span class="mi" v-html="icon('plus', 18)" aria-hidden="true"></span>
        <span class="lbl">새 작업</span>
        <kbd class="kbd lbl">N</kbd>
      </RouterLink>
      <nav aria-label="주 메뉴">
        <RouterLink to="/" :aria-current="isActive('/') ? 'page' : undefined" title="홈">
          <span class="mi" v-html="icon('home')" aria-hidden="true"></span><span class="lbl">홈</span>
        </RouterLink>
        <RouterLink to="/works" :aria-current="isActive('/works') ? 'page' : undefined" title="작업">
          <span class="mi" v-html="icon('works')" aria-hidden="true"></span><span class="lbl">작업</span>
        </RouterLink>
        <RouterLink to="/media" :aria-current="isActive('/media') ? 'page' : undefined" title="미디어">
          <span class="mi" v-html="icon('media')" aria-hidden="true"></span><span class="lbl">미디어</span>
        </RouterLink>
        <RouterLink to="/teams" :aria-current="isActive('/teams') ? 'page' : undefined" title="팀">
          <span class="mi" v-html="icon('teams')" aria-hidden="true"></span><span class="lbl">팀</span>
        </RouterLink>
        <RouterLink to="/connect" :aria-current="isActive('/connect') ? 'page' : undefined" title="연결">
          <span class="mi" v-html="icon('connect')" aria-hidden="true"></span>
          <span class="lbl">연결</span>
          <span v-if="connectAlert" class="dot-alert" aria-label="연결 문제 있음"></span>
        </RouterLink>
        <RouterLink to="/settings" :aria-current="isActive('/settings') ? 'page' : undefined" title="설정">
          <span class="mi" v-html="icon('settings')" aria-hidden="true"></span><span class="lbl">설정</span>
        </RouterLink>
      </nav>
      <div class="golinks" aria-label="홍보 위탁 바로가기">
        <span class="go-cap micro">홍보 위탁</span>
        <RouterLink to="/promo"><span class="mi" v-html="icon('store', 16)" aria-hidden="true"></span><span class="lbl">홍보관</span></RouterLink>
        <RouterLink to="/promo/admin"><span class="mi" v-html="icon('shield', 16)" aria-hidden="true"></span><span class="lbl">위탁 관리</span></RouterLink>
        <RouterLink to="/brand"><span class="mi" v-html="icon('spark', 16)" aria-hidden="true"></span><span class="lbl">서비스 소개</span></RouterLink>
      </div>
      <div class="side-foot">
        <div class="engine-pill" :class="{ on: automation }">
          <span class="pulse" aria-hidden="true"></span>
          <span class="lbl">{{ todayLine || (automation ? '엔진 가동 중' : '엔진 일시정지') }}</span>
        </div>
      </div>
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
          <button
            v-if="automation !== null"
            class="auto-chip"
            :class="{ off: !automation }"
            @click="onAutoClick"
            :aria-label="automation ? '자동화 켜짐, 멈추려면 클릭' : '자동화 멈춤, 켜려면 클릭'"
          >
            <span class="dot" :class="{ on: automation }" aria-hidden="true"></span>
            <span class="chip-lbl">{{ automation ? '자동화 가동' : '자동화 멈춤' }}</span>
          </button>
          <button class="iconbtn" @click="onThemeToggle" :title="themeTitle" :aria-label="themeTitle" v-html="icon(themeIcon, 18)"></button>
          <div class="opwrap">
            <button class="iconbtn" @click="bellOpen = !bellOpen; opOpen = false; if (bellOpen) loadNotes()" aria-label="알림">
              <span v-html="icon('bell', 19)" aria-hidden="true"></span>
              <span v-if="unread > 0" class="bdg">{{ unread > 99 ? '99+' : unread }}</span>
            </button>
            <div v-if="bellOpen" class="belldrop" role="dialog" aria-label="알림 목록">
              <div class="sect-head">
                <h3>알림 <span v-if="unread > 0" class="muted micro">({{ unread }})</span></h3>
                <button v-if="unread > 0" class="ghost small" @click="readAll">모두 읽음</button>
              </div>
              <ul v-if="notes.length" class="notelist">
                <li v-for="n in notes" :key="n.id">
                  <button class="note" :class="{ unread: !n.readAt }" @click="openNote(n)">
                    <span class="ndot" :class="{ warn: !n.readAt && n.severity !== 'info', read: !!n.readAt }" aria-hidden="true"></span>
                    <span class="nmsg">{{ n.message }}</span>
                    <span class="ntime num">{{ shortTime(n.createdAt) }}</span>
                  </button>
                </li>
              </ul>
              <p v-else class="empty-note muted small">새로운 알림이 없습니다.</p>
            </div>
          </div>
          <div class="opwrap">
            <button class="iconbtn opbtn" @click="opOpen = !opOpen; bellOpen = false" aria-label="운영자 메뉴">
              <span class="avatar" aria-hidden="true">{{ opInitial }}</span>
            </button>
            <div v-if="opOpen" class="opdrop" role="menu">
              <div class="op-head">
                <strong>{{ auth.user?.username || '운영자' }}</strong>
                <span class="muted micro">마케팅 엔진 관리자</span>
              </div>
              <RouterLink to="/settings" role="menuitem" @click="opOpen = false">환경 설정 · 비밀번호</RouterLink>
              <button role="menuitem" class="danger-item" @click="doLogout">로그아웃</button>
            </div>
          </div>
        </div>
      </header>

      <div v-if="!online" class="offlineband">연결이 끊겼습니다 · 재연결 중</div>

      <main id="main" class="main">
        <div class="main-inner">
          <slot />
        </div>
      </main>

      <nav class="tabs" aria-label="하단 메뉴">
        <RouterLink to="/" :aria-current="isActive('/') ? 'page' : undefined"><span v-html="icon('home', 21)" aria-hidden="true"></span><small>홈</small></RouterLink>
        <RouterLink to="/works" :aria-current="isActive('/works') ? 'page' : undefined"><span v-html="icon('works', 21)" aria-hidden="true"></span><small>작업</small></RouterLink>
        <RouterLink to="/new" class="fab" aria-label="새 작업" v-html="icon('plus', 24)"></RouterLink>
        <RouterLink to="/teams" :aria-current="isActive('/teams') ? 'page' : undefined"><span v-html="icon('teams', 21)" aria-hidden="true"></span><small>팀</small></RouterLink>
        <button class="tab-more" @click="drawerOpen = true" aria-label="더보기">
          <span class="more-ic" v-html="icon('menu', 21)" aria-hidden="true"></span>
          <span v-if="connectAlert" class="tab-dot" aria-hidden="true"></span>
          <small>더보기</small>
        </button>
      </nav>

      <div v-if="drawerOpen" class="drawer-back" @click.self="drawerOpen = false">
        <div class="drawer" role="dialog" aria-label="전체 메뉴">
          <div class="drawer-head">
            <span class="d-brand"><LogoMark :size="26" /> <strong>SOMUN</strong></span>
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
          <div class="d-links" aria-label="홍보 위탁 바로가기">
            <span class="muted micro">홍보 위탁</span>
            <RouterLink to="/promo" @click="drawerOpen = false">홍보관</RouterLink>
            <RouterLink to="/promo/admin" @click="drawerOpen = false">위탁 관리</RouterLink>
            <RouterLink to="/brand" @click="drawerOpen = false">서비스 소개</RouterLink>
          </div>
          <button class="ghost logout" @click="doLogout">로그아웃</button>
        </div>
      </div>

      <div v-if="confirmStop" class="modal-back" @click.self="confirmStop = false">
        <div class="modal" role="alertdialog" aria-label="자동화 멈추기 확인">
          <h3>자동화를 멈출까요?</h3>
          <p class="muted small">새 작업 진행과 예약 발행이 모두 일시 중지됩니다. 이미 진행 중인 작업은 끝까지 처리됩니다.</p>
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
import { cycleTheme, theme } from '../stores/theme.js'
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
  menu: '<path d="M5 7.5h14M5 12h14M5 16.5h14"/>',
  close: '<path d="M6.5 6.5l11 11M17.5 6.5l-11 11"/>',
  chevL: '<path d="M14.5 6 8.5 12l6 6"/>',
  store: '<path d="M4 9.5 5.5 5h13L20 9.5M5 10v9h14v-9M9 19v-5h6v5"/>',
  shield: '<path d="M12 3.5 5 6.5v5.5c0 4.5 3 7.5 7 8.5 4-1 7-4 7-8.5V6.5z"/>',
  spark: '<path d="M12 3l2.2 6.8L21 12l-6.8 2.2L12 21l-2.2-6.8L3 12l6.8-2.2z"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2.5v2M12 19.5v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2.5 12h2M19.5 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>',
  moon: '<path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5z"/>',
  monitor: '<rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/>',
}
function icon(name, size = 19) {
  return `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.85" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICON_PATHS[name] || ''}</svg>`
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

const opInitial = computed(() => (auth.user?.username || 'A').slice(0, 1).toUpperCase())
const themeIcon = computed(() => (theme.mode === 'light' ? 'sun' : theme.mode === 'dark' ? 'moon' : 'monitor'))
const themeTitle = computed(() => (theme.mode === 'light' ? '라이트 모드 (클릭해 다크로 전환)' : theme.mode === 'dark' ? '다크 모드 (클릭해 시스템으로 전환)' : '시스템 테마 (클릭해 라이트로 전환)'))

function onThemeToggle() {
  const next = cycleTheme()
  const label = next === 'light' ? '라이트 모드' : next === 'dark' ? '다크 모드' : '시스템 테마'
  toast(`테마: ${label}`)
}

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

function onAutoClick() {
  if (automation.value) confirmStop.value = true
  else setAutomation(true)
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
  else if (n.kind === 'publish_failed') router.push('/works?tab=failed')
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
    confirmStop.value = false
    return
  }
  const tag = (e.target?.tagName || '').toLowerCase()
  if (tag === 'input' || tag === 'textarea' || tag === 'select' || e.target?.isContentEditable || e.metaKey || e.ctrlKey || e.altKey) return
  if (e.key === 'n' || e.key === 'N') {
    e.preventDefault()
    router.push('/new')
  } else if (e.key === '/') {
    const el = document.querySelector('main input[type="search"], main input[aria-label*="검색"], main input[placeholder*="검색"]')
    if (el) {
      e.preventDefault()
      el.focus()
    }
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
  timer = setInterval(() => { loadNotes(); loadSide() }, 60000)
})
onUnmounted(() => {
  window.removeEventListener('online', onOnline)
  window.removeEventListener('offline', onOffline)
  document.removeEventListener('keydown', onKey)
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.shell { display: flex; min-height: 100vh; background: var(--bg); }
.side {
  width: 244px;
  flex-shrink: 0;
  background: var(--side-bg);
  color: var(--side-ink);
  border-right: 1px solid var(--side-line);
  padding: 20px 14px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  position: sticky;
  top: 0;
  height: 100vh;
  transition: width var(--t);
  z-index: 60;
}
.side-head { display: flex; align-items: center; gap: 8px; padding: 0 4px; }
.brand { display: flex; align-items: center; gap: 10px; text-decoration: none; color: var(--side-ink); flex: 1; min-width: 0; }
.brand-t { display: flex; flex-direction: column; line-height: 1.2; }
.brand-t strong { font-size: 17px; font-weight: 800; letter-spacing: 0.08em; color: #fff; }
.brand-t small { font-size: 11px; color: var(--side-muted); }
.fold {
  flex: none;
  border: 1px solid var(--side-line);
  background: transparent;
  color: var(--side-muted);
  border-radius: 7px;
  width: 28px;
  height: 28px;
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: transform var(--t), background var(--t), color var(--t);
}
.fold.flip { transform: rotate(180deg); }
.fold:hover { background: var(--side-surface); color: #fff; }

.newbtn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: linear-gradient(135deg, #06b6d4 0%, #0e7490 100%);
  color: #ffffff;
  border-radius: var(--r-md);
  padding: 11px 14px;
  text-decoration: none;
  font-weight: 700;
  font-size: 14px;
  box-shadow: 0 6px 18px -6px rgba(6, 182, 212, 0.55);
  transition: filter var(--t), transform var(--t);
}
.newbtn:hover { filter: brightness(1.08); color: #fff; transform: translateY(-1px); }
.kbd {
  margin-left: auto;
  font-family: inherit;
  font-size: 11px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 5px;
  background: rgba(255, 255, 255, 0.18);
  color: #fff;
}

.side nav { display: flex; flex-direction: column; gap: 3px; }
.side nav a, .golinks a {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 10px 12px;
  border-radius: 10px;
  text-decoration: none;
  color: var(--side-muted);
  font-size: 14px;
  font-weight: 600;
  transition: background var(--t), color var(--t);
}
.side nav a:hover, .golinks a:hover { background: var(--side-surface); color: #fff; }
.side nav a[aria-current="page"] {
  background: rgba(6, 182, 212, 0.16);
  color: #67e8f9;
  font-weight: 700;
}
.mi { display: inline-flex; align-items: center; justify-content: center; flex: none; width: 20px; height: 20px; }
.dot-alert { width: 8px; height: 8px; border-radius: 50%; background: #f87171; margin-left: auto; box-shadow: 0 0 0 3px rgba(248, 113, 113, 0.2); }

.golinks {
  display: flex;
  flex-direction: column;
  gap: 2px;
  border-top: 1px solid var(--side-line);
  padding-top: 14px;
  margin-top: 4px;
}
.go-cap { padding: 0 12px 6px; color: var(--side-muted); text-transform: uppercase; letter-spacing: 0.06em; font-weight: 700; }
.golinks a { font-size: 13px; padding: 8px 12px; }

.side-foot { margin-top: auto; padding-top: 10px; border-top: 1px solid var(--side-line); }
.engine-pill {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 10px;
  background: var(--side-surface);
  font-size: 12.5px;
  font-weight: 600;
  color: var(--side-muted);
}
.engine-pill .pulse { width: 8px; height: 8px; border-radius: 50%; background: #64748b; flex: none; }
.engine-pill.on { color: #e2e8f0; }
.engine-pill.on .pulse { background: #34d399; box-shadow: 0 0 0 3px rgba(52, 211, 153, 0.25); }

.shell.lnb-min .side { width: 74px; padding: 20px 10px; align-items: stretch; }
.shell.lnb-min .side-head { flex-direction: column; gap: 10px; }
.shell.lnb-min .brand { justify-content: center; }
.shell.lnb-min .brand-t, .shell.lnb-min .lbl, .shell.lnb-min .go-cap { display: none; }
.shell.lnb-min .side nav a, .shell.lnb-min .golinks a { justify-content: center; padding: 10px; }
.shell.lnb-min .newbtn { width: 44px; height: 44px; padding: 0; border-radius: 12px; margin: 0 auto; }
.shell.lnb-min .engine-pill { justify-content: center; padding: 10px; }

.body { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.topbar {
  min-height: 60px;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 8px 28px;
  border-bottom: 1px solid var(--line);
  background: var(--panel);
  position: sticky;
  top: 0;
  z-index: 50;
}
.topbar .menu { display: none; }
.crumbbox { display: flex; flex-direction: column; gap: 1px; min-width: 0; }
.crumbs { display: flex; align-items: center; gap: 6px; font-size: 11.5px; color: var(--muted); }
.crumbs a { color: var(--muted); text-decoration: none; }
.crumbs a:hover { color: var(--accent); }
.crumbs .cur { color: var(--ink-soft); font-weight: 600; }
.crumbs .sep { opacity: 0.5; }
.topbar .title { font-size: 18px; font-weight: 800; letter-spacing: -0.02em; color: var(--ink); }
.top-right { margin-left: auto; display: flex; align-items: center; gap: 8px; }

.auto-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
  border-radius: 999px;
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 700;
  box-shadow: var(--shadow-sm);
}
.auto-chip:hover { background: var(--soft); border-color: var(--muted); }
.auto-chip.off { color: var(--muted); background: var(--soft); }
.dot { width: 8px; height: 8px; border-radius: 50%; background: var(--muted); display: inline-block; }
.dot.on { background: var(--ok); box-shadow: 0 0 0 3px var(--ok-soft); }

.iconbtn {
  border: 1px solid transparent;
  background: transparent;
  color: var(--ink-soft);
  border-radius: 10px;
  padding: 0;
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
}
.iconbtn:hover { background: var(--soft); color: var(--ink); border-color: var(--line); }
.avatar {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: var(--accent-soft);
  color: var(--accent-dark);
  font-size: 13px;
  font-weight: 800;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.bdg {
  position: absolute;
  top: 2px;
  right: 2px;
  background: var(--warn);
  color: #fff;
  font-size: 10.5px;
  font-weight: 800;
  border-radius: 999px;
  padding: 0 5px;
  line-height: 16px;
}

.opwrap { position: relative; }
.opdrop, .belldrop {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--r-lg);
  box-shadow: var(--shadow-lg);
  padding: 10px;
  z-index: 100;
}
.opdrop { display: flex; flex-direction: column; min-width: 196px; gap: 2px; }
.op-head { padding: 8px 10px 10px; border-bottom: 1px solid var(--line-soft); margin-bottom: 4px; display: flex; flex-direction: column; }
.opdrop a, .opdrop button {
  text-align: left;
  justify-content: flex-start;
  background: transparent;
  border: none;
  padding: 9px 10px;
  text-decoration: none;
  color: var(--ink);
  border-radius: var(--r-sm);
  font-size: 13.5px;
  font-weight: 600;
}
.opdrop a:hover, .opdrop button:hover { background: var(--soft); }
.opdrop .danger-item { color: var(--warn); }

.belldrop { width: 380px; max-width: calc(100vw - 24px); padding: 14px; }
.belldrop h3 { margin: 0; font-size: 15px; }
.notelist { list-style: none; margin: 10px 0 0; padding: 0; max-height: 340px; overflow: auto; display: flex; flex-direction: column; gap: 4px; }
.note {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  justify-content: flex-start;
  width: 100%;
  background: transparent;
  border: none;
  color: var(--ink);
  text-align: left;
  padding: 9px 10px;
  border-radius: var(--r-sm);
  font-weight: 400;
}
.note.unread { background: var(--soft); font-weight: 600; }
.note:hover { background: var(--accent-soft); }
.ndot { width: 8px; height: 8px; border-radius: 50%; background: var(--accent); margin-top: 6px; flex: none; }
.ndot.warn { background: var(--warn); }
.ndot.read { background: var(--line); }
.nmsg { flex: 1; font-size: 13px; line-height: 1.45; }
.ntime { color: var(--muted); font-size: 11.5px; white-space: nowrap; }
.empty-note { text-align: center; padding: 24px 0; }

.offlineband { background: var(--warn); color: #fff; text-align: center; padding: 8px; font-size: 13px; font-weight: 600; }
.main { padding: 28px; width: 100%; flex: 1; }
.main-inner { max-width: 1240px; margin: 0 auto; width: 100%; }
.tabs { display: none; }

.drawer-back { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.55); backdrop-filter: blur(2px); z-index: 150; }
.drawer {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 300px;
  max-width: 86vw;
  background: var(--panel);
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  overflow-y: auto;
  box-shadow: var(--shadow-lg);
}
.drawer-head { display: flex; align-items: center; justify-content: space-between; }
.d-brand { display: flex; align-items: center; gap: 8px; font-size: 16px; }
.drawer nav { display: flex; flex-direction: column; gap: 3px; }
.drawer nav a {
  padding: 11px 12px;
  text-decoration: none;
  color: var(--ink-soft);
  border-radius: var(--r-md);
  font-size: 15px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 12px;
}
.drawer nav a > span { display: inline-flex; }
.drawer nav a:hover { background: var(--soft); color: var(--ink); }
.drawer nav a[aria-current="page"] { background: var(--accent-soft); color: var(--accent-dark); font-weight: 700; }
.d-links { display: flex; flex-direction: column; gap: 4px; border-top: 1px solid var(--line-soft); padding-top: 12px; }
.d-links a { padding: 8px 12px; text-decoration: none; color: var(--ink-soft); font-size: 14px; border-radius: var(--r-sm); }
.d-links a:hover { background: var(--soft); }
.drawer .logout { margin-top: auto; width: 100%; }

@media (max-width: 900px) {
  .side { display: none; }
  .topbar { min-height: 54px; padding: 8px 14px; gap: 8px; }
  .topbar .menu { display: inline-flex; }
  .crumbs { display: none; }
  .topbar .title { font-size: 16px; }
  .auto-chip { padding: 6px 10px; font-size: 12px; }
  .chip-lbl { display: none; }
  .main { padding: 16px 14px 88px; }
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
  .tabs a, .tabs .tab-more {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 3px;
    text-decoration: none;
    color: var(--muted);
    background: transparent;
    border: none;
    padding: 0;
    min-height: 44px;
    position: relative;
  }
  .tabs a > span, .tabs .more-ic { display: inline-flex; }
  .tabs small { font-size: 11px; font-weight: 600; }
  .tabs a[aria-current="page"] { color: var(--accent); font-weight: 700; }
  .tabs .fab {
    background: linear-gradient(135deg, #06b6d4 0%, #0e7490 100%);
    color: #fff;
    border-radius: 50%;
    width: 52px;
    height: 52px;
    flex: none;
    margin: -18px 8px 0;
    box-shadow: 0 6px 16px rgba(14, 116, 144, 0.4);
  }
  .tab-dot { position: absolute; top: 8px; right: 24%; width: 7px; height: 7px; border-radius: 50%; background: var(--warn); }
}

@media (min-width: 901px) and (max-width: 1279px) {
  .shell:not(.lnb-min) .side { width: 74px; padding: 20px 10px; }
  .shell:not(.lnb-min) .side-head { flex-direction: column; gap: 10px; }
  .shell:not(.lnb-min) .brand { justify-content: center; }
  .shell:not(.lnb-min) .brand-t, .shell:not(.lnb-min) .lbl, .shell:not(.lnb-min) .go-cap { display: none; }
  .shell:not(.lnb-min) .side nav a, .shell:not(.lnb-min) .golinks a { justify-content: center; padding: 10px; }
  .shell:not(.lnb-min) .newbtn { width: 44px; height: 44px; padding: 0; border-radius: 12px; margin: 0 auto; }
  .shell:not(.lnb-min) .engine-pill { justify-content: center; padding: 10px; }
}
</style>
