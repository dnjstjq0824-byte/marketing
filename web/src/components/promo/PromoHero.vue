<template>
  <section class="pu-band hero" aria-label="소개">
    <p class="pu-chip">홍보 위탁 플랫폼</p>
    <h1 class="pu-h1 hero-title">좋아하는 제품을 홍보하고<br /><em>건당 수수료</em>를 받으세요</h1>
    <p class="pu-lead hero-lead">마음에 드는 제품에 신청하고, 승인되면 비공개 링크로 홍보합니다. 주문 1건마다 고정 금액이 적립됩니다.</p>
    <div class="pu-rowline">
      <RouterLink to="/promo/login" class="pu-btn pu-btn-primary">무료로 시작하기</RouterLink>
      <RouterLink to="/promo/guide" class="pu-btn">이용가이드 보기</RouterLink>
    </div>
  </section>
</template>

<style scoped>
.hero { margin-bottom: 28px; background: linear-gradient(135deg, var(--pu-accent-soft), var(--pu-card)); }
.hero-title { font-size: 34px; line-height: 1.3; margin: 12px 0 8px; }
.hero-title em { font-style: normal; color: var(--pu-accent-dark); }
.hero-lead { margin-bottom: 18px; max-width: 560px; }
@media (max-width: 700px) { .hero-title { font-size: 26px; } }
</style>
