<template>
  <a class="bd-skip" href="#bd-main">본문으로 건너뛰기</a>
  <header class="bd-top" :class="{ scrolled }">
    <div class="bd-top-in">
      <RouterLink to="/brand" class="bd-brand" aria-label="SOMUN 브랜드 홈">
        <LogoMark :size="32" />
        <strong>SOMUN</strong>
      </RouterLink>
      <nav class="bd-nav" aria-label="브랜드 메뉴">
        <a href="#how">작동 방식</a>
        <a href="#features">기능</a>
        <a href="#start">시작하기</a>
      </nav>
      <RouterLink to="/login" class="bd-cta">콘솔 열기</RouterLink>
      <button class="bd-burger" aria-label="메뉴 열기" :aria-expanded="open.toString()" @click="open = !open">☰</button>
    </div>
    <nav v-if="open" class="bd-mnav" aria-label="모바일 메뉴">
      <a href="#how" @click="open = false">작동 방식</a>
      <a href="#features" @click="open = false">기능</a>
      <a href="#start" @click="open = false">시작하기</a>
      <RouterLink to="/login">콘솔 열기</RouterLink>
    </nav>
  </header>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import LogoMark from '../LogoMark.vue'

const open = ref(false)
const scrolled = ref(false)
function onKey(e) {
  if (e.key === 'Escape') open.value = false
}
function onScroll() {
  scrolled.value = window.scrollY > 8
}
onMounted(() => {
  document.addEventListener('keydown', onKey)
  window.addEventListener('scroll', onScroll, { passive: true })
  onScroll()
})
onUnmounted(() => {
  document.removeEventListener('keydown', onKey)
  window.removeEventListener('scroll', onScroll)
})
</script>

<style scoped>
.bd-skip { position: absolute; left: -9999px; }
.bd-skip:focus { left: 8px; top: 8px; background: var(--panel); padding: 8px; z-index: 50; }
.bd-top { position: sticky; top: 0; z-index: 40; background: var(--panel); border-bottom: 1px solid var(--line-soft); transition: box-shadow var(--t); }
.bd-top.scrolled { box-shadow: 0 8px 24px -16px rgba(0, 0, 0, 0.25); }
.bd-top-in { max-width: 1120px; margin: 0 auto; padding: 0 20px; height: 64px; display: flex; align-items: center; gap: 20px; }
.bd-brand { display: flex; align-items: center; gap: 9px; text-decoration: none; color: var(--ink); font-size: 17px; letter-spacing: 0.5px; }
.bd-nav { display: flex; gap: 4px; }
.bd-nav a { text-decoration: none; color: var(--ink-soft); font-weight: 700; font-size: 15px; padding: 8px 12px; border-radius: 6px; transition: background var(--t); }
.bd-nav a:hover { background: var(--soft); }
.bd-cta { margin-left: auto; background: var(--accent); color: var(--accent-ink); font-weight: 800; font-size: 14px; text-decoration: none; padding: 9px 18px; border-radius: 999px; transition: background var(--t); white-space: nowrap; }
.bd-cta:hover { background: var(--accent-dark); }
.bd-burger { display: none; margin-left: auto; border: 1px solid var(--line); background: var(--panel); color: var(--ink); border-radius: 8px; width: 40px; height: 40px; font-size: 18px; }
.bd-mnav { display: flex; flex-direction: column; padding: 8px 16px 14px; gap: 2px; border-top: 1px solid var(--line-soft); }
.bd-mnav a { padding: 11px 8px; text-decoration: none; color: var(--ink); font-weight: 700; border-radius: 8px; }
.bd-mnav a:hover { background: var(--soft); }
@media (max-width: 760px) {
  .bd-nav, .bd-top-in > .bd-cta { display: none; }
  .bd-burger { display: inline-flex; align-items: center; justify-content: center; }
}
</style>
