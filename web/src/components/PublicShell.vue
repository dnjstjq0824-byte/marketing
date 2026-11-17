<template>
  <div class="pu-page">
    <a class="pu-skip" href="#pu-main">본문으로 건너뛰기</a>
    <header class="pu-top">
      <div class="pu-top-in">
        <RouterLink to="/promo" class="pu-brand">
          <span class="pu-mark" aria-hidden="true">S</span>
          <span class="pu-brand-t"><strong>SOMUN</strong><small>홍보 위탁</small></span>
        </RouterLink>
        <nav class="pu-nav" aria-label="홍보 위탁 메뉴">
          <RouterLink to="/promo/products">홍보 위탁 제품</RouterLink>
          <RouterLink to="/promo/guide">사용 가이드</RouterLink>
          <RouterLink to="/promo/me">내 홍보함</RouterLink>
        </nav>
        <div class="pu-auth">
          <template v-if="!member.user">
            <RouterLink to="/promo/login" class="pu-login">로그인/회원가입</RouterLink>
          </template>
          <template v-else>
            <span class="pu-nick">{{ member.user.nickname }}님</span>
            <button class="pu-logout" @click="doLogout">로그아웃</button>
          </template>
          <RouterLink to="/promo/work" class="pu-btn pu-btn-primary small">마케팅 시작</RouterLink>
        </div>
      </div>
    </header>
    <main id="pu-main" class="pu-wrap" :class="{ wide }">
      <slot />
    </main>
    <footer class="pu-foot">
      <div class="pu-foot-in">
        <strong>SOMUN 홍보 위탁</strong>
        <span>수수료는 주문 1건당 고정 금액으로 지급됩니다.</span>
        <span style="margin-left: auto">© SOMUN</span>
      </div>
    </footer>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { member, memberLogout, memberRefresh } from '../stores/member.js'

defineProps({
  wide: { type: Boolean, default: false },
})

const router = useRouter()

async function doLogout() {
  await memberLogout()
  router.push('/promo')
}

onMounted(() => {
  if (member.token && !member.user) memberRefresh()
})
</script>

<style scoped>
.pu-skip { position: absolute; left: -9999px; }
.pu-skip:focus { left: 8px; top: 8px; background: #fff; padding: 8px; z-index: 50; }
.pu-top { border-bottom: 1px solid var(--pu-line); background: #fff; position: sticky; top: 0; z-index: 40; }
.pu-top-in { max-width: var(--pu-max); margin: 0 auto; padding: 0 20px; height: 64px; display: flex; align-items: center; gap: 20px; }
.pu-brand { display: flex; align-items: center; gap: 9px; text-decoration: none; color: var(--pu-ink); }
.pu-mark {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: var(--pu-accent);
  color: #fff;
  font-weight: 800;
  font-size: 17px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.pu-brand-t { display: flex; flex-direction: column; line-height: 1.15; }
.pu-brand-t strong { font-size: 16px; letter-spacing: 0.5px; }
.pu-brand-t small { font-size: 11px; color: var(--pu-muted); }
.pu-nav { display: flex; gap: 4px; }
.pu-nav a { text-decoration: none; color: var(--pu-soft); font-size: 15px; font-weight: 700; padding: 8px 12px; border-radius: 6px; }
.pu-nav a:hover { background: var(--pu-line-soft); }
.pu-nav a.router-link-active { color: var(--pu-accent-dark); }
.pu-auth { margin-left: auto; display: flex; align-items: center; gap: 10px; }
.pu-login { font-size: 14px; font-weight: 700; color: var(--pu-soft); text-decoration: none; }
.pu-login:hover { color: var(--pu-accent-dark); }
.pu-nick { font-size: 14px; font-weight: 700; }
.pu-logout { border: none; background: none; color: var(--pu-muted); font-size: 13px; cursor: pointer; padding: 8px 4px; }
.pu-logout:hover { color: var(--pu-ink); }
@media (max-width: 700px) {
  .pu-top-in { flex-wrap: wrap; height: auto; padding: 10px 14px; gap: 8px; }
  .pu-nav { order: 3; width: 100%; }
  .pu-auth { margin-left: auto; }
}
</style>
