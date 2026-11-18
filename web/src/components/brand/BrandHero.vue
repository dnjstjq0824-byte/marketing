<template>
  <section class="bd-hero" aria-labelledby="bd-h1">
    <div class="bd-hero-copy">
      <p class="bd-kicker">AI 마케팅 엔진</p>
      <h1 id="bd-h1">마케팅팀을<br />AI로 꾸리세요</h1>
      <p class="bd-lead">기획자·작가·검수자·발행 담당까지. SOMUN의 AI 에이전트 팀이 콘텐츠가 나가는 전 과정을 맡습니다.</p>
      <div class="bd-ctas">
        <RouterLink to="/login" class="bd-btn primary">콘솔 열기</RouterLink>
        <a href="#how" class="bd-btn">작동 방식 보기</a>
      </div>
      <ul class="bd-mini">
        <li>밤새 도는 예약 발행</li>
        <li>팀장 검수 내장</li>
        <li>발행 전 가드레일</li>
      </ul>
    </div>
    <BrandVisual />
  </section>
</template>

<script setup>
import BrandVisual from './BrandVisual.vue'
</script>

<style scoped>
.bd-hero { display: grid; grid-template-columns: 1.02fr 0.98fr; gap: 48px; align-items: center; padding: 64px 0 72px; }
.bd-kicker { display: inline-block; color: var(--accent-dark); background: var(--accent-soft); font-weight: 800; font-size: 13px; margin: 0 0 18px; padding: 6px 14px; border-radius: 999px; }
.bd-hero h1 { font-size: clamp(2.5rem, 5vw, 3.75rem); font-weight: 800; letter-spacing: -0.03em; line-height: 1.18; margin: 0 0 18px; }
.bd-lead { color: var(--ink-soft); font-size: 18px; line-height: 1.65; margin: 0 0 30px; max-width: 480px; }
.bd-ctas { display: flex; gap: 12px; flex-wrap: wrap; margin-bottom: 30px; }
.bd-btn { text-decoration: none; font-weight: 800; font-size: 15px; padding: 14px 28px; border-radius: 999px; border: 1px solid var(--line); background: var(--panel); color: var(--ink); transition: background var(--t), transform var(--t); }
.bd-btn:hover { background: var(--soft); transform: translateY(-1px); }
.bd-btn.primary { background: var(--accent); border-color: var(--accent); color: var(--accent-ink); box-shadow: 0 10px 24px -10px var(--accent); }
.bd-btn.primary:hover { background: var(--accent-dark); }
.bd-mini { list-style: none; margin: 0; padding: 18px 0 0; display: flex; gap: 20px; flex-wrap: wrap; border-top: 1px solid var(--line-soft); }
.bd-mini li { font-size: 13.5px; font-weight: 700; color: var(--ink-soft); padding-left: 20px; position: relative; }
.bd-mini li::before { content: ""; position: absolute; left: 0; top: 7px; width: 10px; height: 6px; border-left: 2.5px solid var(--ok); border-bottom: 2.5px solid var(--ok); transform: rotate(-45deg); border-radius: 1px; }
@media (max-width: 960px) {
  .bd-hero { grid-template-columns: 1fr; gap: 36px; padding: 44px 0 52px; }
}
@media (prefers-reduced-motion: reduce) {
  .bd-btn:hover { transform: none; }
}
</style>
