<template>
  <div class="pu-page">
    <a class="pu-skip" href="#pu-main">본문으로 건너뛰기</a>
    <header class="pu-top">
      <div class="pu-top-in">
        <RouterLink to="/promo" class="pu-brand">
          <LogoMark :size="32" />
          <span class="pu-brand-t"><strong>SOMUN</strong><small>홍보 위탁</small></span>
        </RouterLink>
        <nav class="pu-nav" aria-label="홍보 위탁 메뉴">
          <RouterLink to="/promo/products" :aria-current="isActive('/promo/products') ? 'page' : undefined">홍보 위탁 제품</RouterLink>
          <div class="pu-drop" @mouseleave="openMenu = null">
            <button
              class="pu-dropbtn"
              :class="{ on: isActive('/promo/guide') || isActive('/promo/work') }"
              :aria-expanded="(openMenu === 'guide').toString()"
              aria-haspopup="true"
              @click="openMenu = openMenu === 'guide' ? null : 'guide'"
            >
              이용 안내 ▾
            </button>
            <div v-if="openMenu === 'guide'" class="pu-menu" role="menu">
              <RouterLink to="/promo/guide" role="menuitem" @click="openMenu = null">사용 가이드</RouterLink>
              <RouterLink to="/promo/work" role="menuitem" @click="openMenu = null">마케팅 시작</RouterLink>
              <RouterLink to="/promo/me" role="menuitem" @click="openMenu = null">내 홍보함</RouterLink>
            </div>
          </div>
          <RouterLink to="/promo/me" :aria-current="isActive('/promo/me') ? 'page' : undefined">내 홍보함</RouterLink>
        </nav>
        <div class="pu-auth">
          <template v-if="!member.user">
            <RouterLink to="/promo/login" class="pu-login">로그인/회원가입</RouterLink>
          </template>
          <div v-else class="pu-drop" @mouseleave="openMenu = null">
            <button
              class="pu-dropbtn nick"
              :aria-expanded="(openMenu === 'me').toString()"
              aria-haspopup="true"
              @click="openMenu = openMenu === 'me' ? null : 'me'"
            >
              {{ member.user.nickname }}님 ▾
            </button>
            <div v-if="openMenu === 'me'" class="pu-menu right" role="menu">
              <RouterLink to="/promo/me" role="menuitem" @click="openMenu = null">내 홍보함</RouterLink>
              <RouterLink to="/promo/work" role="menuitem" @click="openMenu = null">마케팅 시작</RouterLink>
              <button role="menuitem" @click="doLogout">로그아웃</button>
            </div>
          </div>
          <RouterLink to="/promo/work" class="pu-btn pu-btn-primary small">마케팅 시작</RouterLink>
          <button class="pu-burger" @click="drawerOpen = true" aria-label="메뉴 열기">☰</button>
        </div>
      </div>
    </header>
    <div v-if="drawerOpen" class="pu-drawer-back" @click.self="drawerOpen = false">
      <div class="pu-drawer" role="dialog" aria-label="전체 메뉴">
        <div class="pu-drawer-head">
          <strong>SOMUN 홍보 위탁</strong>
          <button class="pu-x" @click="drawerOpen = false" aria-label="메뉴 닫기">✕</button>
        </div>
        <nav aria-label="모바일 메뉴">
          <RouterLink to="/promo" :aria-current="isActive('/promo', true) ? 'page' : undefined" @click="drawerOpen = false">홍보 메인</RouterLink>
          <RouterLink to="/brand" :aria-current="isActive('/brand') ? 'page' : undefined" @click="drawerOpen = false">SOMUN 소개</RouterLink>
          <RouterLink to="/promo/products" :aria-current="isActive('/promo/products') ? 'page' : undefined" @click="drawerOpen = false">홍보 위탁 제품</RouterLink>
          <RouterLink to="/promo/guide" :aria-current="isActive('/promo/guide') ? 'page' : undefined" @click="drawerOpen = false">사용 가이드</RouterLink>
          <RouterLink to="/promo/me" :aria-current="isActive('/promo/me') ? 'page' : undefined" @click="drawerOpen = false">내 홍보함</RouterLink>
          <RouterLink to="/promo/work" :aria-current="isActive('/promo/work') ? 'page' : undefined" @click="drawerOpen = false">마케팅 시작</RouterLink>
        </nav>
        <div class="pu-drawer-auth">
          <template v-if="!member.user">
            <RouterLink to="/promo/login" class="pu-btn block" @click="drawerOpen = false">로그인/회원가입</RouterLink>
            <RouterLink to="/promo/signup" class="pu-btn pu-btn-primary block" @click="drawerOpen = false">회원가입</RouterLink>
          </template>
          <template v-else>
            <span class="pu-nick">{{ member.user.nickname }}님</span>
            <button class="pu-btn block" @click="doLogout">로그아웃</button>
          </template>
        </div>
      </div>
    </div>
    <main id="pu-main" class="pu-wrap" :class="{ wide }">
      <slot />
    </main>
    <footer class="pu-foot">
      <div class="pu-foot-in cols">
        <div class="pu-fcol">
          <strong>SOMUN 홍보 위탁</strong>
          <span>수수료는 주문 1건당 고정 금액으로 지급됩니다.</span>
        </div>
        <nav class="pu-fcol" aria-label="서비스">
          <span class="pu-fcap">서비스</span>
          <RouterLink to="/brand">SOMUN 소개</RouterLink>
          <RouterLink to="/promo/products">홍보 위탁 제품</RouterLink>
          <RouterLink to="/promo/guide">사용 가이드</RouterLink>
          <RouterLink to="/promo/me">내 홍보함</RouterLink>
        </nav>
        <nav class="pu-fcol" aria-label="계정">
          <span class="pu-fcap">계정</span>
          <RouterLink to="/promo/login">로그인</RouterLink>
          <RouterLink to="/promo/signup">회원가입</RouterLink>
          <RouterLink to="/promo/work">마케팅 시작</RouterLink>
        </nav>
        <nav class="pu-fcol" aria-label="운영자">
          <span class="pu-fcap">운영자</span>
          <RouterLink to="/">콘솔 홈</RouterLink>
          <RouterLink to="/promo/admin">위탁 관리</RouterLink>
        </nav>
      </div>
      <div class="pu-foot-in base">
        <span>© SOMUN</span>
        <span class="pu-legal"><RouterLink to="/promo/terms">이용약관</RouterLink> · <RouterLink to="/promo/privacy">개인정보처리방침</RouterLink></span>
      </div>
    </footer>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { member, memberLogout, memberRefresh } from '../stores/member.js'
import LogoMark from './LogoMark.vue'

defineProps({
  wide: { type: Boolean, default: false },
})

const router = useRouter()
const route = useRoute()
const drawerOpen = ref(false)
const openMenu = ref(null)

function isActive(to, exact = false) {
  return exact ? route.path === to : route.path === to || route.path.startsWith(to + '/')
}

async function doLogout() {
  openMenu.value = null
  drawerOpen.value = false
  await memberLogout()
  router.push('/promo')
}

watch(() => route.path, () => {
  openMenu.value = null
  drawerOpen.value = false
})

function onKey(e) {
  if (e.key === 'Escape') {
    drawerOpen.value = false
    openMenu.value = null
  }
}

onMounted(() => {
  if (member.token && !member.user) memberRefresh()
  document.addEventListener('keydown', onKey)
})
onUnmounted(() => {
  document.removeEventListener('keydown', onKey)
})
</script>

<style scoped>
.pu-skip { position: absolute; left: -9999px; }
.pu-skip:focus { left: 8px; top: 8px; background: var(--pu-card); padding: 8px; z-index: 50; }
.pu-top { border-bottom: 1px solid var(--pu-line); background: var(--pu-card); position: sticky; top: 0; z-index: 40; }
.pu-top-in { max-width: var(--pu-max); margin: 0 auto; padding: 0 20px; height: 64px; display: flex; align-items: center; gap: 20px; }
.pu-brand { display: flex; align-items: center; gap: 9px; text-decoration: none; color: var(--pu-ink); }
.pu-brand-t { display: flex; flex-direction: column; line-height: 1.15; }
.pu-brand-t strong { font-size: 16px; letter-spacing: 0.5px; }
.pu-brand-t small { font-size: 11px; color: var(--pu-muted); }
.pu-nav { display: flex; gap: 4px; align-items: center; }
.pu-nav > a { text-decoration: none; color: var(--pu-soft); font-size: 15px; font-weight: 700; padding: 8px 12px; border-radius: 6px; transition: background var(--pu-t); }
.pu-nav > a:hover { background: var(--pu-line-soft); }
.pu-nav > a[aria-current="page"] { color: var(--pu-accent-dark); }
.pu-drop { position: relative; }
.pu-dropbtn {
  border: none;
  background: none;
  color: var(--pu-soft);
  font-size: 15px;
  font-weight: 700;
  padding: 8px 12px;
  border-radius: 6px;
  cursor: pointer;
  font-family: inherit;
}
.pu-dropbtn:hover { background: var(--pu-line-soft); }
.pu-dropbtn.on, .pu-dropbtn[aria-expanded="true"] { color: var(--pu-accent-dark); }
.pu-dropbtn.nick { font-size: 14px; color: var(--pu-ink); }
.pu-menu {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  min-width: 170px;
  background: var(--pu-card);
  border: 1px solid var(--pu-line);
  border-radius: var(--pu-r-sm);
  box-shadow: var(--pu-shadow);
  padding: 6px;
  display: flex;
  flex-direction: column;
  z-index: 60;
}
.pu-menu.right { left: auto; right: 0; }
.pu-menu a, .pu-menu button {
  text-align: left;
  background: none;
  border: none;
  padding: 9px 12px;
  border-radius: 6px;
  text-decoration: none;
  color: var(--pu-ink);
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
  font-family: inherit;
}
.pu-menu a:hover, .pu-menu button:hover { background: var(--pu-line-soft); }
.pu-auth { margin-left: auto; display: flex; align-items: center; gap: 10px; }
.pu-login { font-size: 14px; font-weight: 700; color: var(--pu-soft); text-decoration: none; white-space: nowrap; }
.pu-login:hover { color: var(--pu-accent-dark); }
.pu-nick { font-size: 14px; font-weight: 700; }
.pu-burger { display: none; border: 1px solid var(--pu-line); background: var(--pu-card); color: var(--pu-ink); border-radius: 8px; width: 40px; height: 40px; font-size: 18px; }
.pu-drawer-back { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.4); z-index: 100; }
.pu-drawer {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 300px;
  max-width: 86vw;
  background: var(--pu-card);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow-y: auto;
}
.pu-drawer-head { display: flex; align-items: center; justify-content: space-between; color: var(--pu-ink); }
.pu-x { border: 1px solid var(--pu-line); background: var(--pu-card); color: var(--pu-ink); border-radius: 8px; width: 36px; height: 36px; font-size: 16px; }
.pu-drawer nav { display: flex; flex-direction: column; gap: 2px; }
.pu-drawer nav a { padding: 12px; text-decoration: none; color: var(--pu-soft); border-radius: 8px; font-size: 16px; font-weight: 700; }
.pu-drawer nav a:hover { background: var(--pu-line-soft); }
.pu-drawer nav a[aria-current="page"] { background: var(--pu-accent-soft); color: var(--pu-accent-dark); }
.pu-drawer-auth { margin-top: auto; display: flex; flex-direction: column; gap: 8px; }
.pu-foot-in.cols { display: grid; grid-template-columns: 2fr 1fr 1fr 1fr; gap: 20px; align-items: start; }
.pu-fcol { display: flex; flex-direction: column; gap: 6px; }
.pu-fcol strong { color: var(--pu-ink); font-size: 15px; }
.pu-fcap { font-size: 12px; font-weight: 800; color: var(--pu-muted); margin-bottom: 2px; }
.pu-fcol a { color: var(--pu-soft); text-decoration: none; font-size: 14px; }
.pu-fcol a:hover { color: var(--pu-accent-dark); }
.pu-foot-in.base { padding-top: 0; }
@media (max-width: 760px) {
  .pu-nav { display: none; }
  .pu-burger { display: inline-flex; align-items: center; justify-content: center; }
  .pu-auth .pu-btn { display: none; }
  .pu-top-in { padding: 0 14px; gap: 10px; }
  .pu-foot-in.cols { grid-template-columns: 1fr 1fr; }
}
</style>
