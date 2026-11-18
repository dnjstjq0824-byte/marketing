<template>
  <section id="how" class="bd-sec" aria-labelledby="bd-how-h">
    <p class="bd-eyebrow">작동 방식</p>
    <h2 id="bd-how-h">네 단계면 발행됩니다</h2>
    <p class="bd-sub">사람이 손댈 곳은 방향뿐입니다.</p>
    <ol class="bd-steps">
      <li v-for="(s, i) in STEPS" :key="s.title">
        <span class="bd-ghost" aria-hidden="true">0{{ i + 1 }}</span>
        <strong>{{ s.title }}</strong>
        <span>{{ s.desc }}</span>
      </li>
    </ol>
  </section>
</template>

<script setup>
const STEPS = [
  { title: '연결', desc: 'AI 계정과 발행 채널을 연결합니다.' },
  { title: '생성', desc: 'AI 팀이 채널에 맞는 원고를 씁니다.' },
  { title: '검수', desc: '팀장 검수에 가드레일 검사를 더합니다.' },
  { title: '발행', desc: '예약대로 내보내고 성과를 모읍니다.' },
]
</script>

<style scoped>
.bd-sec { margin: 0; padding: 72px 0; scroll-margin-top: 64px; border-top: 1px solid var(--line-soft); }
.bd-eyebrow { color: var(--accent-dark); font-weight: 800; font-size: 13px; letter-spacing: 0.06em; margin: 0 0 10px; }
.bd-sec h2 { font-size: clamp(1.75rem, 3vw, 2.25rem); font-weight: 800; letter-spacing: -0.02em; margin: 0 0 8px; }
.bd-sub { color: var(--muted); margin: 0 0 32px; font-size: 16px; }
.bd-steps { list-style: none; margin: 0; padding: 26px 0 0; display: grid; grid-template-columns: repeat(4, 1fr); gap: 20px; border-top: 2px solid var(--line); }
.bd-steps li { display: flex; flex-direction: column; gap: 8px; position: relative; }
.bd-steps li::before { content: ""; position: absolute; top: -33px; left: 0; width: 12px; height: 12px; border-radius: 50%; background: var(--accent); }
.bd-ghost { font-size: 40px; font-weight: 800; letter-spacing: -0.02em; color: var(--line); line-height: 1; }
.bd-steps strong { font-size: 16px; }
.bd-steps span:last-child { color: var(--ink-soft); font-size: 14px; line-height: 1.6; }
@media (max-width: 960px) {
  .bd-steps { grid-template-columns: repeat(2, 1fr); gap: 28px 20px; }
}
@media (max-width: 560px) {
  .bd-steps { grid-template-columns: 1fr; }
  .bd-sec { padding: 52px 0; }
}
</style>
