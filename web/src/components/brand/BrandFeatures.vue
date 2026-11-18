<template>
  <section id="features" class="bd-sec" aria-labelledby="bd-feat-h">
    <p class="bd-eyebrow">기능</p>
    <h2 id="bd-feat-h">콘솔에서 오늘 바로 쓰는 것들</h2>
    <p class="bd-sub">전부 이어져 있습니다. 따로 붙일 게 없습니다.</p>
    <div class="bd-grid">
      <article v-for="f in FEATURES" :key="f.title" class="bd-card">
        <span class="bd-ic" aria-hidden="true" v-html="f.icon"></span>
        <h3>{{ f.title }}</h3>
        <p>{{ f.desc }}</p>
        <RouterLink :to="f.to">{{ f.link }} →</RouterLink>
      </article>
    </div>
  </section>
</template>

<script setup>
const S = (inner) => `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${inner}</svg>`
const FEATURES = [
  { title: 'AI 팀 편성', desc: '로그인된 AI 계정을 팀으로 묶고 팀장을 세웁니다. 쓰는 글마다 담당과 검수가 정해집니다.', to: '/teams', link: '팀 보러 가기', icon: S('<circle cx="9" cy="8" r="3.2"/><path d="M3.5 19c.6-3.2 2.8-5 5.5-5s4.9 1.8 5.5 5"/><circle cx="17" cy="9" r="2.4"/><path d="M15.5 14.2c2.9.1 4.6 1.7 5 4.8"/>') },
  { title: '무인 자동화', desc: '예약 시간·일일 상한·비상 정지로 밤새 돌아가는 발행을 사람이 정한 선 안에서만 움직입니다.', to: '/settings', link: '자동화 설정하기', icon: S('<circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3.2 2"/>') },
  { title: '안전 가드레일', desc: '금칙어·원격 이미지·표시 의무를 발행 전에 검사합니다. 걸리면 막히고 기록이 남습니다.', to: '/works', link: '작업 흐름 보기', icon: S('<path d="M12 3.5l7.5 3v5.5c0 4.6-3.2 7.6-7.5 8.5-4.3-.9-7.5-3.9-7.5-8.5V6.5z"/><path d="M9 12l2.2 2.2L15.5 10"/>') },
  { title: '위탁 마켓', desc: '사람 홍보자에게 맡기는 일은 공개 마켓에서 모집·신청·정산까지 이어집니다.', to: '/promo', link: '마켓 구경하기', icon: S('<path d="M4 9.5h16l-1.5 9.5H5.5z"/><path d="M8.5 9.5V7.8a3.5 3.5 0 017 0v1.7"/>') },
]
</script>

<style scoped>
.bd-sec { margin: 0; padding: 72px 0; scroll-margin-top: 64px; border-top: 1px solid var(--line-soft); }
.bd-eyebrow { color: var(--accent-dark); font-weight: 800; font-size: 13px; letter-spacing: 0.06em; margin: 0 0 10px; }
.bd-sec h2 { font-size: clamp(1.75rem, 3vw, 2.25rem); font-weight: 800; letter-spacing: -0.02em; margin: 0 0 8px; }
.bd-sub { color: var(--muted); margin: 0 0 32px; font-size: 16px; }
.bd-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 20px; }
.bd-card { background: var(--panel); border: 1px solid var(--line-soft); border-radius: 16px; padding: 30px 28px; box-shadow: var(--shadow); display: flex; flex-direction: column; gap: 10px; transition: border-color var(--t), transform var(--t); }
.bd-card:hover { border-color: var(--line); transform: translateY(-2px); }
.bd-ic { width: 44px; height: 44px; border-radius: 12px; background: var(--accent-soft); color: var(--accent-dark); display: inline-flex; align-items: center; justify-content: center; }
.bd-ic :deep(svg) { width: 24px; height: 24px; }
.bd-card h3 { margin: 4px 0 0; font-size: 18px; }
.bd-card p { margin: 0; color: var(--ink-soft); font-size: 14.5px; line-height: 1.65; flex: 1; }
.bd-card a { font-weight: 800; font-size: 14px; }
@media (max-width: 760px) {
  .bd-grid { grid-template-columns: 1fr; }
  .bd-sec { padding: 52px 0; }
}
@media (prefers-reduced-motion: reduce) {
  .bd-card:hover { transform: none; }
}
</style>
