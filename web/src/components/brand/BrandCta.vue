<template>
  <section id="start" class="bd-cta-sec" aria-labelledby="bd-cta-h">
    <p class="bd-eyebrow">시작하기</p>
    <h2 id="bd-cta-h">콘솔에서 시작하세요</h2>
    <p class="bd-cta-sub">계정 연결부터 첫 발행까지, 안내는 가이드에 다 있습니다.</p>
    <div class="bd-ctas">
      <RouterLink to="/login" class="bd-btn primary">콘솔 열기</RouterLink>
      <RouterLink to="/promo/guide" class="bd-btn ghost">이용 가이드 보기</RouterLink>
    </div>
  </section>
</template>

<style scoped>
.bd-cta-sec { background: var(--media-bg); border-radius: 20px; padding: 64px 32px; margin: 72px 0; text-align: center; }
.bd-eyebrow { color: #ffb37a; font-weight: 800; font-size: 13px; letter-spacing: 0.06em; margin: 0 0 12px; }
.bd-cta-sec h2 { color: #fff; font-size: clamp(1.75rem, 3vw, 2.25rem); font-weight: 800; letter-spacing: -0.02em; margin: 0 0 10px; }
.bd-cta-sub { color: #b9c4cc; margin: 0 0 30px; font-size: 16px; }
.bd-ctas { display: flex; gap: 12px; justify-content: center; flex-wrap: wrap; }
.bd-btn { text-decoration: none; font-weight: 800; font-size: 15px; padding: 14px 28px; border-radius: 999px; transition: background var(--t), transform var(--t); }
.bd-btn:hover { transform: translateY(-1px); }
.bd-btn.primary { background: var(--accent); color: var(--accent-ink); }
.bd-btn.primary:hover { background: #ff7f2a; }
.bd-btn.ghost { border: 1px solid #3a4653; color: #fff; }
.bd-btn.ghost:hover { background: #1e2833; }
@media (max-width: 760px) {
  .bd-cta-sec { padding: 48px 24px; margin: 52px 0; }
}
@media (prefers-reduced-motion: reduce) {
  .bd-btn:hover { transform: none; }
}
</style>
