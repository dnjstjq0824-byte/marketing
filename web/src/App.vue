<template>
  <div class="shell">
    <a class="skip" href="#main">본문으로 건너뛰기</a>
    <aside class="side">
      <RouterLink to="/" class="brand">
        <span class="mark" aria-hidden="true"><svg viewBox="0 0 24 24" width="22" height="22"><path d="M4 19V9l8-5 8 5v10" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/><path d="M9 19v-6h6v6" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/></svg></span>
        <span class="brand-t"><strong>SOMUN</strong><small>마케팅 코어</small></span>
      </RouterLink>
      <nav aria-label="주 메뉴">
        <div v-for="g in menuGroups" :key="g.label" class="group">
          <h4>{{ g.label }}</h4>
          <RouterLink v-for="it in g.items" :key="it.to" :to="it.to" :aria-current="isActive(it.to) ? 'page' : undefined">
            <span class="mi" aria-hidden="true" v-html="icon(it.to)"></span>{{ it.label }}
          </RouterLink>
        </div>
      </nav>
      <RouterLink to="/settings" class="who" :class="{ off: !auth.user }">
        <span class="dot" :class="{ on: !!auth.user }" aria-hidden="true"></span>
        {{ auth.user ? auth.user.username + ' 로그인 중' : '미로그인 · 설정에서 로그인' }}
      </RouterLink>
    </aside>
    <div class="body">
      <header class="topbar">
        <nav class="crumbs" aria-label="현재 위치">
          <span class="muted">{{ crumb.group }}</span>
          <span aria-hidden="true" class="sep">/</span>
          <strong>{{ crumb.item }}</strong>
        </nav>
        <div class="top-right">
          <span class="muted small hide-m">비용 0원 · 로컬 운영</span>
          <div v-if="auth.user" class="bellwrap">
            <button class="iconbtn" @click="toggleBell" aria-label="알림 발신함" :aria-expanded="bellOpen">
              <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path d="M6 9a6 6 0 0112 0c0 5 2 6 2 6H4s2-1 2-6zM10 20a2 2 0 004 0" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
              <span v-if="unread > 0" class="bdg">{{ unread > 99 ? '99+' : unread }}</span>
            </button>
            <div v-if="bellOpen" class="belldrop" role="dialog" aria-label="알림 목록">
              <div class="sect-head"><h3>알림</h3><button v-if="unread > 0" class="ghost small" @click="readAll">모두 읽음</button></div>
              <ul v-if="notes.length" class="notelist">
                <li v-for="n in notes" :key="n.id" :class="{ read: n.readAt }">
                  <button class="note" @click="readOne(n)">
                    <span :class="n.severity === 'error' ? 'pill p-failed' : (n.severity === 'info' ? 'pill p-approved' : 'pill p-limited')">{{ kindLabel(n.kind) }}</span>
                    <span class="nmsg">{{ n.message }}</span>
                    <span class="ntime">{{ shortTime(n.createdAt) }}</span>
                  </button>
                </li>
              </ul>
              <p v-else class="empty"><strong>알림이 없습니다</strong>발신함이 비어 있습니다.</p>
            </div>
          </div>
        </div>
      </header>
      <main id="main" class="main">
        <RouterView :key="$route.path" />
      </main>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { menuGroups } from './router/index.js'
import { apiFetch, auth, refreshMe } from './stores/auth.js'

const route = useRoute()

const KIND_LABEL = {
  job_failed: '작업 실패', job_stuck: '작업 지연', lease_expired: '리스 만료',
  session_expired: '세션 만료', session_limited: '세션 한도', publish_failed: '발행 실패',
  backup_failed: '백업 실패', overdue_hold: '장기 보류', review_human: '사람 검수',
}
const bellOpen = ref(false)
const notes = ref([])
const unread = ref(0)
let bellTimer = null

function kindLabel(k) {
  return KIND_LABEL[k] || k
}

function shortTime(iso) {
  return (iso || '').slice(5, 16).replace('T', ' ')
}

async function loadNotes() {
  if (!auth.user) {
    notes.value = []
    unread.value = 0
    return
  }
  try {
    const data = await apiFetch('/api/notifications?limit=20')
    notes.value = data.items || []
    unread.value = data.unread || 0
  } catch {
    notes.value = []
    unread.value = 0
  }
}

function toggleBell() {
  bellOpen.value = !bellOpen.value
  if (bellOpen.value) loadNotes()
}

async function readOne(n) {
  if (!n.readAt) {
    try {
      await apiFetch(`/api/notifications/${n.id}/read`, { method: 'POST' })
    } catch { /* badge refreshes below regardless */ }
    await loadNotes()
  }
}

async function readAll() {
  try {
    await apiFetch('/api/notifications/read-all', { method: 'POST' })
  } catch { /* badge refreshes below regardless */ }
  await loadNotes()
}

watch(() => route.path, () => {
  bellOpen.value = false
  loadNotes()
})
watch(() => auth.user, loadNotes)

onMounted(async () => {
  await refreshMe()
  loadNotes()
  bellTimer = setInterval(loadNotes, 60000)
})
onUnmounted(() => {
  if (bellTimer) clearInterval(bellTimer)
})

const ICONS = {
  '/': '<svg viewBox="0 0 24 24"><path d="M4 4h7v7H4zM13 4h7v4h-7zM13 11h7v9h-7zM4 14h7v6H4z" fill="none" stroke="currentColor" stroke-width="1.8"/></svg>',
  '/chat': '<svg viewBox="0 0 24 24"><path d="M4 6h16v9H9l-5 4z" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/></svg>',
  '/products': '<svg viewBox="0 0 24 24"><path d="M12 3l8 4v10l-8 4-8-4V7z" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/><path d="M12 3v8m8-4l-8 4-8-4" fill="none" stroke="currentColor" stroke-width="1.8"/></svg>',
  '/readings': '<svg viewBox="0 0 24 24"><path d="M5 4h11a3 3 0 013 3v13H8a3 3 0 01-3-3z" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/><path d="M9 9h6M9 13h6" stroke="currentColor" stroke-width="1.8"/></svg>',
  '/review': '<svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="7" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M16.5 16.5L21 21" stroke="currentColor" stroke-width="1.8"/></svg>',
  '/queue': '<svg viewBox="0 0 24 24"><path d="M4 6h16M4 12h16M4 18h10" stroke="currentColor" stroke-width="1.8"/><circle cx="19" cy="18" r="2.4" fill="none" stroke="currentColor" stroke-width="1.8"/></svg>',
  '/metrics': '<svg viewBox="0 0 24 24"><path d="M4 20V10M10 20V4M16 20v-8M21 20H3" fill="none" stroke="currentColor" stroke-width="1.8"/></svg>',
  '/agents': '<svg viewBox="0 0 24 24"><circle cx="8" cy="8" r="3.4" fill="none" stroke="currentColor" stroke-width="1.8"/><circle cx="16.5" cy="9.5" r="2.6" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M2.5 20c.6-3.4 2.8-5 5.5-5s4.9 1.6 5.5 5M14 15.4c2.9-.4 5.9 1 6.5 4.1" fill="none" stroke="currentColor" stroke-width="1.8"/></svg>',
  '/channels': '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M19.1 4.9L17 7M7 17l-2.1 2.1" stroke="currentColor" stroke-width="1.8"/></svg>',
  '/owned': '<svg viewBox="0 0 24 24"><path d="M4 11l8-7 8 7v9a1 1 0 01-1 1h-5v-6h-4v6H5a1 1 0 01-1-1z" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/></svg>',
  '/criteria': '<svg viewBox="0 0 24 24"><path d="M6 3h12v18l-3-2-3 2-3-2-3 2z" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/><path d="M9 9l2 2 4-4" fill="none" stroke="currentColor" stroke-width="1.8"/></svg>',
  '/resident': '<svg viewBox="0 0 24 24"><rect x="4" y="7" width="16" height="10" rx="2" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M9 7V4M15 7V4M9 4h6M8 12h.01M12 12h.01M16 12h.01" stroke="currentColor" stroke-width="1.8"/></svg>',
  '/system': '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.4 2.4M16.7 16.7l2.4 2.4M19.1 4.9l-2.4 2.4M7.3 16.7l-2.4 2.4" stroke="currentColor" stroke-width="1.8"/></svg>',
  '/mobile': '<svg viewBox="0 0 24 24"><rect x="7" y="2" width="10" height="20" rx="2" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M11 18.5h2" stroke="currentColor" stroke-width="1.8"/></svg>',
  '/settings': '<svg viewBox="0 0 24 24"><path d="M4 8h16M4 16h16" stroke="currentColor" stroke-width="1.8"/><circle cx="9" cy="8" r="2.4" fill="var(--panel)" stroke="currentColor" stroke-width="1.8"/><circle cx="15" cy="16" r="2.4" fill="var(--panel)" stroke="currentColor" stroke-width="1.8"/></svg>',
}

function icon(to) {
  return ICONS[to] || ''
}

function isActive(to) {
  return route.path === to
}

const crumb = computed(() => {
  for (const g of menuGroups) {
    for (const it of g.items) {
      if (it.to === route.path) return { group: g.label, item: it.label }
    }
  }
  return { group: '홈', item: '대시보드' }
})
</script>

<style>
:root {
  --bg: #f4f1ea;
  --panel: #fffdf8;
  --ink: #1d2733;
  --ink-soft: #43525f;
  --muted: #6d7a72;
  --line: #ddd5c2;
  --line-soft: #eae4d3;
  --soft: #faf7ef;
  --accent: #0e7490;
  --accent-dark: #0b5d74;
  --accent-soft: #e2f1f6;
  --accent-ink: #ffffff;
  --gold: #b97f00;
  --gold-soft: #f7e8c8;
  --pine: #0f2b34;
  --pine-soft: #16424f;
  --paper-ink: #ece5d3;
  --warn: #b42318;
  --warn-soft: #fbe4e0;
  --ok: #067647;
  --ok-soft: #d9f0e3;
  --hold: #8a5a00;
  --hold-soft: #fdf3dd;
  --r-sm: 6px;
  --r-md: 10px;
  --r-lg: 14px;
  --shadow: 0 1px 2px rgba(29, 39, 51, 0.07), 0 10px 26px -18px rgba(29, 39, 51, 0.35);
  --shadow-lg: 0 18px 50px -12px rgba(29, 39, 51, 0.35);
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background-color: var(--bg);
  background-image: url('data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="140" height="140"><filter id="n"><feTurbulence type="fractalNoise" baseFrequency="0.85" numOctaves="2" stitchTiles="stitch"/><feColorMatrix type="saturate" values="0"/><feComponentTransfer><feFuncA type="linear" slope="0.055"/></feComponentTransfer></filter><rect width="140" height="140" filter="url(%23n)"/></svg>');
  color: var(--ink);
  font-family: "Malgun Gothic", "Apple SD Gothic Neo", "Noto Sans KR", system-ui, sans-serif;
  font-size: 14px;
  line-height: 1.6;
  -webkit-font-smoothing: antialiased;
}
a { color: var(--accent-dark); }
a:hover { color: var(--accent); }
:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; border-radius: 4px; }
.skip {
  position: absolute;
  left: -999px;
  top: 8px;
  background: var(--ink);
  color: #fff;
  padding: 8px 14px;
  border-radius: var(--r-sm);
  z-index: 50;
}
.skip:focus { left: 8px; color: #fff; }
.shell { display: flex; min-height: 100vh; }
.side {
  width: 236px;
  flex: none;
  background: var(--pine);
  color: var(--paper-ink);
  padding: 16px 12px 12px;
  display: flex;
  flex-direction: column;
  position: sticky;
  top: 0;
  height: 100vh;
  overflow: auto;
}
.brand { display: flex; align-items: center; gap: 10px; text-decoration: none; border-radius: var(--r-md); padding: 6px 8px 12px; }
.mark {
  flex: none;
  width: 36px;
  height: 36px;
  border-radius: 9px;
  background: var(--gold-soft);
  color: var(--pine);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.brand-t { display: flex; flex-direction: column; line-height: 1.25; }
.brand-t strong { font-size: 17px; letter-spacing: 2.5px; color: #fff; }
.brand-t small { font-size: 12px; color: #a9bda9; }
.side nav { flex: 1; }
.group h4 { margin: 14px 8px 4px; font-size: 11px; letter-spacing: 0.6px; color: #8fa3a0; font-weight: 700; }
.side nav a {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 7px 10px;
  border-radius: 8px;
  color: #cfd8cd;
  text-decoration: none;
  font-weight: 500;
}
.side nav a:hover { background: var(--pine-soft); color: #fff; }
.side nav a.router-link-active { background: var(--pine-soft); color: #fff; font-weight: 700; box-shadow: inset 3px 0 0 var(--gold-soft); }
.side nav a.router-link-active .mi { color: var(--gold-soft); }
.mi { flex: none; width: 17px; height: 17px; display: inline-flex; color: #8fa3a0; }
.mi svg { width: 100%; height: 100%; }
.who {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: #a9bda9;
  padding: 9px 10px;
  margin-top: 10px;
  border-top: 1px solid var(--pine-soft);
  text-decoration: none;
  border-radius: 8px;
}
.who:hover { background: var(--pine-soft); color: #fff; }
.dot { flex: none; width: 8px; height: 8px; border-radius: 50%; background: #5a6e6b; }
.dot.on { background: #4cc38a; }
.body { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.topbar {
  background: rgba(244, 241, 234, 0.92);
  border-bottom: 1px solid var(--line);
  padding: 11px 28px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  position: sticky;
  top: 0;
  z-index: 5;
}
.crumbs { display: flex; align-items: baseline; gap: 8px; font-size: 14px; }
.crumbs .sep { color: var(--line); }
.crumbs strong { font-size: 15px; }
.top-right { display: flex; align-items: center; gap: 12px; }
.bellwrap { position: relative; }
.iconbtn {
  position: relative;
  background: transparent;
  border: 1px solid transparent;
  color: var(--ink-soft);
  border-radius: 8px;
  padding: 6px 8px;
  margin: 0;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
}
.iconbtn:hover { background: var(--soft); border-color: var(--line); }
.bdg {
  position: absolute;
  top: -4px;
  right: -4px;
  min-width: 19px;
  height: 19px;
  padding: 0 5px;
  border-radius: 999px;
  background: var(--warn);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-variant-numeric: tabular-nums;
}
.belldrop {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  width: 380px;
  max-width: 86vw;
  max-height: 420px;
  overflow: auto;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  box-shadow: var(--shadow-lg);
  padding: 14px 16px;
  z-index: 30;
}
.notelist { list-style: none; margin: 8px 0 0; padding: 0; }
.notelist li { border-top: 1px solid var(--line-soft); }
.note {
  width: 100%;
  text-align: left;
  background: transparent;
  border: none;
  padding: 9px 2px;
  margin: 0;
  cursor: pointer;
  display: block;
  color: var(--ink);
  font-weight: 400;
}
.note:hover { background: var(--soft); }
.notelist li.read .note { opacity: 0.62; }
.nmsg { display: block; font-size: 13px; margin: 3px 0 1px; }
.ntime { display: block; font-size: 11px; color: var(--muted); }
.main { flex: 1; padding: 24px 28px 64px; max-width: 1180px; width: 100%; margin: 0 auto; }
.page-head { margin: 2px 0 4px; }
.page h2 { margin: 0 0 2px; font-size: 23px; letter-spacing: -0.4px; }
.page > .muted, .page-head > .muted { margin-top: 0; max-width: 72ch; }
.num { font-variant-numeric: tabular-nums; text-align: right; }
th.num { text-align: right; }
.hero {
  background-color: var(--pine);
  background-image: radial-gradient(rgba(236, 229, 211, 0.13) 1px, transparent 1px);
  background-size: 18px 18px;
  border: 1px solid var(--pine);
  border-top: 3px solid var(--gold-soft);
  border-radius: var(--r-lg);
  color: var(--paper-ink);
  padding: 20px 22px 16px;
  margin: 12px 0 14px;
  box-shadow: var(--shadow-lg);
}
.hero .kick { font-size: 12px; color: #a9bda9; letter-spacing: 0.3px; }
.hero h3 { margin: 2px 0 4px; font-size: 21px; letter-spacing: -0.3px; color: #fff; }
.hero .lede { margin: 0 0 12px; color: #cfd8cd; font-size: 13px; }
.hero-stats { display: flex; flex-wrap: wrap; gap: 8px 26px; margin: 0; padding: 12px 0 2px; border-top: 1px solid var(--pine-soft); list-style: none; }
.hero-stats b { display: block; font-size: 24px; line-height: 1.15; color: #fff; font-variant-numeric: tabular-nums; }
.hero-stats span { font-size: 12px; color: #a9bda9; }
.hero-stats li.alert b { color: #f2b8ac; }
.hero-stats li.gold b { color: var(--gold-soft); }
.muted { color: var(--muted); }
.small { font-size: 12px; }
.hide-m { white-space: nowrap; }
.card {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: 18px 20px;
  margin: 14px 0;
  box-shadow: var(--shadow);
}
.card.narrow { max-width: 560px; }
.card.hi { border-color: var(--accent); }
.card.warnline { border-color: var(--warn); }
.card.rule-pine { border-top: 3px solid var(--pine); }
.card.rule-gold { border-top: 3px solid var(--gold); }
.card.rule-teal { border-top: 3px solid var(--accent); }
.card h3 { margin: 0 0 10px; font-size: 16px; letter-spacing: -0.2px; }
.card h4 { margin: 18px 0 8px; font-size: 14px; }
.card h4:first-child { margin-top: 0; }
.row { display: flex; gap: 14px; align-items: flex-start; flex-wrap: wrap; }
.row > .card { margin-top: 0; }
.grow { flex: 1; min-width: 270px; }
.grow2 { flex: 2; min-width: 330px; }
label { display: block; margin: 10px 0; font-weight: 600; font-size: 13px; }
label .req { color: var(--warn); font-weight: 700; }
.hint { display: block; font-weight: 400; font-size: 12px; color: var(--muted); margin-top: 3px; }
input, select, textarea {
  display: block;
  width: 100%;
  margin-top: 5px;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  font: inherit;
  font-size: 14px;
  font-weight: 400;
  background: var(--panel);
  color: var(--ink);
}
input:hover, select:hover, textarea:hover { border-color: #c4b995; }
input:focus, select:focus, textarea:focus { outline: 2px solid var(--accent-soft); border-color: var(--accent); }
input::placeholder, textarea::placeholder { color: #a39e8b; }
input[type="radio"], input[type="checkbox"] { display: inline; width: auto; margin: 0 6px 0 0; accent-color: var(--accent-dark); }
input[type="number"] { font-variant-numeric: tabular-nums; }
textarea { resize: vertical; line-height: 1.6; }
fieldset { border: 1px solid var(--line); border-radius: var(--r-sm); margin: 10px 0; padding: 10px 14px 12px; }
fieldset legend { font-size: 13px; font-weight: 600; padding: 0 6px; }
fieldset label { display: inline-block; margin-right: 18px; font-weight: 400; }
button {
  padding: 8px 16px;
  border: 1px solid var(--accent-dark);
  border-radius: var(--r-sm);
  background: var(--accent);
  color: var(--accent-ink);
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  margin: 4px 4px 4px 0;
}
button:hover { background: var(--accent-dark); }
button:active { transform: translateY(1px); }
button:disabled { opacity: 0.55; cursor: default; transform: none; }
button.ghost { background: var(--panel); color: var(--ink); border-color: var(--line); font-weight: 500; }
button.ghost:hover { background: var(--soft); border-color: #c4b995; }
button.danger { background: var(--warn); border-color: var(--warn); color: #fff; }
button.danger:hover { background: #8f1b13; }
button.small { padding: 4px 10px; font-size: 12px; font-weight: 500; }
.error { color: var(--warn); }
.ok { color: var(--ok); }
.banner { border-radius: var(--r-sm); padding: 10px 14px; margin: 10px 0; font-size: 13px; }
.banner.err { background: var(--warn-soft); color: #7c1a12; }
.banner.okk { background: var(--ok-soft); color: #0a4a2e; }
.banner.warn { background: var(--hold-soft); color: #5f3f00; }
.banner.info { background: var(--accent-soft); color: var(--accent-dark); }
table { width: 100%; border-collapse: collapse; margin-top: 10px; font-size: 13.5px; }
th, td { text-align: left; padding: 8px 10px; border-bottom: 1px solid var(--line-soft); vertical-align: middle; }
thead th {
  font-size: 12px;
  color: var(--muted);
  background: var(--soft);
  position: sticky;
  top: 0;
  white-space: nowrap;
}
tbody tr:hover { background: var(--soft); }
tbody tr.sel { background: var(--accent-soft); }
td.id, th.id { font-variant-numeric: tabular-nums; color: var(--muted); width: 1%; white-space: nowrap; }
.ellipsis { max-width: 340px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.toolbar { display: flex; gap: 12px; align-items: flex-end; flex-wrap: wrap; margin-bottom: 4px; }
.toolbar label { min-width: 170px; margin-bottom: 0; }
.toolbar .spacer { flex: 1; }
.picklist { list-style: none; margin: 0; padding: 0; max-height: 480px; overflow: auto; }
.picklist button {
  width: 100%;
  text-align: left;
  background: var(--panel);
  color: var(--ink);
  border: 1px solid var(--line);
  margin: 0 0 5px;
  font-weight: 500;
  line-height: 1.5;
}
.picklist button:hover { background: var(--soft); border-color: #c4b995; }
.picklist button.active { border-color: var(--accent); background: var(--accent-soft); font-weight: 700; box-shadow: inset 3px 0 0 var(--accent); }
.picklist .sub { display: block; font-size: 12px; color: var(--muted); font-weight: 400; }
.badge {
  display: inline-block;
  font-size: 11px;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--hold-soft);
  color: var(--hold);
  margin-left: 6px;
}
.btnrow { display: flex; gap: 4px; flex-wrap: wrap; align-items: center; }
.btnrow select { width: auto; margin: 0; }
.formrow { display: flex; gap: 12px; flex-wrap: wrap; align-items: flex-end; }
.formrow label { flex: 1; min-width: 160px; }
.check { display: flex; align-items: center; gap: 4px; font-weight: 400; }
.disclosure, .ailabel { border: 1px dashed var(--line); border-radius: 6px; padding: 10px 12px; margin: 12px 0; background: var(--soft); }
.disclosure strong, .ailabel strong { font-size: 12px; color: var(--muted); }
.disclosure p, .ailabel p { margin: 4px 0 0; }
.thumbs { display: flex; gap: 8px; flex-wrap: wrap; margin: 8px 0; }
.thumbs img { max-height: 120px; border: 1px solid var(--line); border-radius: 6px; }
.thumbs figure { margin: 0; text-align: center; }
.thumbs figcaption { margin-top: 4px; }
.thumbs figcaption button { font-size: 12px; padding: 3px 10px; }
.reading { margin: 8px 0; }
.reading > div { display: flex; gap: 12px; padding: 6px 0; border-bottom: 1px solid var(--line); }
.reading dt { flex: none; width: 110px; color: var(--muted); }
.reading dd { margin: 0; white-space: pre-wrap; }
.reading .unknowns dt, .reading .unknowns dd { color: var(--hold); }
.jobs { list-style: none; margin: 4px 0; padding: 0; font-size: 13px; }
.jobs li { padding: 4px 0; border-bottom: 1px dotted var(--line); }
.modal-back { position: fixed; inset: 0; background: rgba(23, 34, 46, 0.5); display: flex; align-items: center; justify-content: center; z-index: 40; padding: 16px; }
.modal { background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); padding: 22px 26px; max-width: 480px; width: 100%; box-shadow: var(--shadow-lg); max-height: 86vh; overflow: auto; }
.modal h3 { margin-top: 0; }
.modal .btnrow { margin-top: 14px; justify-content: flex-end; }
.chatlog { border: 1px solid var(--line); border-radius: var(--r-md); padding: 14px; min-height: 220px; max-height: 360px; overflow: auto; background: var(--soft); }
.chatlog .me { text-align: right; margin: 8px 0; }
.chatlog .them { text-align: left; margin: 8px 0; }
.chatlog p { display: inline-block; padding: 8px 13px; border-radius: 14px; background: #ece5d3; margin: 0; max-width: 82%; text-align: left; line-height: 1.55; }
.chatlog .me p { background: var(--accent-dark); color: #fff; }
.chatlog .sys { text-align: center; margin: 10px 0; }
.chatlog .sys span { display: inline-block; font-size: 12px; color: var(--muted); background: #ece5d3; border-radius: 999px; padding: 2px 12px; }
.tabs { display: inline-flex; gap: 4px; margin-bottom: 12px; background: var(--soft); border: 1px solid var(--line); border-radius: 9px; padding: 3px; }
.tabs button { background: transparent; color: var(--muted); border-color: transparent; margin: 0; font-weight: 600; }
.tabs button:hover { background: #ece5d3; color: var(--ink); }
.tabs button.active { background: var(--panel); color: var(--accent-dark); border-color: var(--line); box-shadow: var(--shadow); }
.statgrid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; margin: 12px 0; }
.stat { border: 1px solid var(--line); border-radius: var(--r-md); padding: 13px 16px 12px; background: var(--panel); box-shadow: var(--shadow); }
.stat b { display: block; font-size: 27px; line-height: 1.2; color: var(--ink); font-variant-numeric: tabular-nums; }
.stat span { font-size: 12px; color: var(--muted); }
.stat.tone b { color: var(--accent-dark); }
.stat.bad { border-color: var(--warn); }
.stat.bad b { color: var(--warn); }
.stat.good b { color: var(--ok); }
.stat.warn b { color: var(--hold); }
.notice { border: 1px solid var(--line); border-radius: var(--r-sm); padding: 10px 14px; margin: 10px 0; background: var(--soft); }
.empty { border: 1px dashed var(--line); border-radius: var(--r-md); padding: 26px 20px; text-align: center; color: var(--muted); background: var(--soft); margin: 10px 0; }
.empty strong { display: block; color: var(--ink-soft); font-size: 14px; margin-bottom: 2px; }
.empty-art { text-align: center; padding: 22px 12px 20px; color: var(--muted); }
.empty-art svg { display: block; margin: 0 auto 8px; }
.empty-art strong { display: block; color: var(--ink-soft); font-size: 14px; }
.empty-art span { display: block; font-size: 12px; margin-top: 2px; }
.skel { border-radius: var(--r-sm); background: #ece5d3; color: transparent; user-select: none; }
@media (prefers-reduced-motion: no-preference) {
  .skel { animation: skel 1.4s ease-in-out infinite; }
  @keyframes skel { 50% { opacity: 0.55; } }
}
.steps { list-style: none; margin: 12px 0; padding: 0; counter-reset: step; }
.steps li { position: relative; padding: 0 0 14px 34px; }
.steps li::before {
  counter-increment: step;
  content: counter(step);
  position: absolute;
  left: 0;
  top: 2px;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--soft);
  border: 1px solid var(--line);
  color: var(--muted);
  font-size: 12px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.steps li:not(:last-child)::after { content: ""; position: absolute; left: 11px; top: 26px; bottom: 0; width: 1px; background: var(--line); }
.steps li.done::before { background: var(--ok); border-color: var(--ok); color: #fff; }
.steps li.now::before { background: var(--accent); border-color: var(--accent); color: #fff; }
.steps li.now { font-weight: 600; }
.form-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0 14px; }
.form-grid .full { grid-column: 1 / -1; }
.table-wrap { overflow-x: auto; margin: 0 -4px; padding: 0 4px; }
.pill { display: inline-block; font-size: 11px; font-weight: 700; padding: 2px 10px; border-radius: 999px; background: #ece5d3; color: #5c5645; white-space: nowrap; }
.p-draft, .p-pending { background: #ece5d3; color: #5c5645; }
.p-review, .p-claimed { background: #fdf3dd; color: #8a5a00; }
.p-approved { background: #e2f1f6; color: #0b5d74; }
.p-queued { background: #d9f0e3; color: #067647; }
.p-published, .p-done, .p-connected { background: #17222e; color: #fff; }
.p-failed, .p-expired { background: #fbe4e0; color: #b42318; }
.p-limited { background: #fdf3dd; color: #8a5a00; }
.p-held { background: #fbe4e0; color: #b42318; }
.p-inactive_hold { background: #ece5d3; color: #5c5645; }
.p-chief { background: #e2f1f6; color: #0b5d74; }
.p-worker { background: #ece5d3; color: #5c5645; }
.p-scheduled { background: #e2f1f6; color: #0b5d74; }
.p-auto { background: #d9f0e3; color: #067647; }
.p-manual { background: #ece5d3; color: #5c5645; }
.p-rejected, .p-error { background: #fbe4e0; color: #b42318; }
.declist { margin: 6px 0; padding-left: 22px; }
.declist li { padding: 5px 0; border-bottom: 1px dotted var(--line); }
.declist li:last-child { border-bottom: none; }
.speclist { list-style: none; margin: 6px 0; padding: 0; }
.speclist li { display: flex; gap: 10px; padding: 7px 0; border-bottom: 1px solid var(--line); }
.speclist li:last-child { border-bottom: none; }
.speclist .k { flex: none; width: 130px; color: var(--muted); }
.speclist input, .speclist select { width: auto; flex: 1; margin: 0; min-width: 120px; }
.speclist button { flex: none; margin: 0 0 0 4px; }
.speclist li { align-items: center; }
.sect-head { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; }
.sect-head .count { font-size: 12px; color: var(--muted); }
@media (max-width: 900px) {
  .shell { flex-direction: column; }
  .side { width: auto; height: auto; position: static; padding: 12px 12px 8px; }
  .brand { padding-bottom: 8px; }
  .side nav { display: flex; flex-wrap: wrap; gap: 0 16px; }
  .group h4 { margin: 8px 4px 2px; }
  .side nav a { padding: 5px 6px; }
  .mi { width: 15px; height: 15px; }
  .who { margin-top: 6px; }
  .topbar { position: static; padding: 10px 16px; }
  .hide-m { display: none; }
  .main { padding: 16px 16px 48px; }
  .row { flex-direction: column; }
  .grow, .grow2 { min-width: 0; }
  .toolbar label { min-width: 140px; flex: 1; }
}
</style>
