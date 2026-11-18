<template>
  <section class="pu-band cta" aria-label="시작하기">
    <h2 class="pu-sec-title cta-title">지금 모집 중인 제품을 만나보세요</h2>
    <p class="pu-muted cta-desc">가입은 무료, 신청은 1분이면 됩니다.</p>
    <div class="pu-rowline">
      <RouterLink to="/promo/login" class="pu-btn pu-btn-primary">무료로 시작하기</RouterLink>
      <RouterLink to="/promo/products" class="pu-btn">제품 둘러보기</RouterLink>
    </div>
  </section>
</template>

<style scoped>
.cta { text-align: center; margin-bottom: 0; }
.cta-title { font-size: 24px; margin-bottom: 6px; }
.cta-desc { margin: 0 0 16px; }
.cta .pu-rowline { justify-content: center; }
</style>
