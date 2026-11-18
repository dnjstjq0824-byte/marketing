<template>
  <section class="bd-sec" aria-labelledby="bd-guard-h">
    <p class="bd-eyebrow">발행 전 자동 검사</p>
    <h2 id="bd-guard-h">나가는 글은 전부 검사합니다</h2>
    <p class="bd-sub">문제가 있으면 발행이 막히고 사유가 기록에 남습니다.</p>
    <dl class="bd-checks">
      <div v-for="g in GUARDS" :key="g[0]">
        <dt>{{ g[0] }}</dt>
        <dd>{{ g[1] }}</dd>
      </div>
    </dl>
  </section>
</template>

<script setup>
const GUARDS = [
  ['금칙어 검사', '업종별 금칙어에 걸리면 발행 중단'],
  ['원격 이미지 차단', '출처 불분명한 외부 이미지는 제외'],
  ['대가·AI 표시', '광고 표시와 AI 생성 표기를 확인'],
  ['분량·링크 기준', '최소 분량과 허용된 링크만 통과'],
]
</script>

<style scoped>
.bd-sec { margin: 0; padding: 72px 0; scroll-margin-top: 64px; border-top: 1px solid var(--line-soft); }
.bd-eyebrow { color: var(--accent-dark); font-weight: 800; font-size: 13px; letter-spacing: 0.06em; margin: 0 0 10px; }
.bd-sec h2 { font-size: clamp(1.75rem, 3vw, 2.25rem); font-weight: 800; letter-spacing: -0.02em; margin: 0 0 8px; }
.bd-sub { color: var(--muted); margin: 0 0 12px; font-size: 16px; }
.bd-checks { margin: 0; display: grid; grid-template-columns: 1fr 1fr; gap: 0 40px; }
.bd-checks div { display: flex; flex-direction: column; gap: 4px; padding: 20px 0; border-bottom: 1px solid var(--line-soft); }
.bd-checks dt { font-weight: 800; font-size: 16px; padding-left: 26px; position: relative; }
.bd-checks dt::before { content: ""; position: absolute; left: 2px; top: 8px; width: 10px; height: 6px; border-left: 2.5px solid var(--ok); border-bottom: 2.5px solid var(--ok); transform: rotate(-45deg); border-radius: 1px; }
.bd-checks dd { margin: 0; padding-left: 26px; color: var(--muted); font-size: 14px; }
@media (max-width: 760px) {
  .bd-checks { grid-template-columns: 1fr; }
  .bd-sec { padding: 52px 0; }
}
</style>
