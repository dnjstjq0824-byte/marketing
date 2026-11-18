<template>
  <section class="pu-sec" aria-label="시작 방법">
    <div class="pu-sec-head">
      <h2 class="pu-sec-title">시작은 이렇게 쉬워요</h2>
      <RouterLink to="/promo/guide" class="pu-sec-link">자세한 가이드 →</RouterLink>
    </div>
    <ol class="pu-steps steps4">
      <li v-for="s in STEPS" :key="s.title"><strong>{{ s.title }}</strong><span>{{ s.desc }}</span></li>
    </ol>
  </section>
</template>

<script setup>
const STEPS = [
  { title: '무료 가입', desc: '이메일로 가입하면 바로 시작됩니다.' },
  { title: '제품 신청', desc: '채널·계획을 적고 수수료 조건에 동의합니다.' },
  { title: '링크 홍보', desc: '승인되면 비공개 링크가 열립니다.' },
  { title: '수수료 적립', desc: '확인된 주문 1건마다 쌓입니다.' },
]
</script>

<style scoped>
.steps4 { grid-template-columns: repeat(4, 1fr); }
@media (max-width: 900px) { .steps4 { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 700px) { .steps4 { grid-template-columns: 1fr; } }
</style>
