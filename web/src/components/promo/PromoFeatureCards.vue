<template>
  <section class="pu-sec" aria-label="특징">
    <div class="pu-sec-head">
      <h2 class="pu-sec-title">왜 여기서 홍보하나요?</h2>
    </div>
    <div class="pu-grid">
      <div v-for="f in FEATURES" :key="f.title" class="pu-card">
        <p class="fic">{{ f.icon }}</p>
        <h3 class="ftitle">{{ f.title }}</h3>
        <p class="pu-muted fdesc">{{ f.desc }}</p>
      </div>
    </div>
  </section>
</template>

<script setup>
const FEATURES = [
  { icon: '💰', title: '건당 고정 수수료', desc: '판매가의 %가 아닙니다. 주문 1건당 고정 금액이라 얼마를 버는지 신청 전에 알 수 있습니다.' },
  { icon: '🔒', title: '승인제 링크 보호', desc: '실제 구매 링크는 승인된 본인에게만 열립니다. 내 홍보 실적을 안전하게 지킵니다.' },
  { icon: '📊', title: '투명한 실적 집계', desc: '확인이 끝난 주문만 실적에 들어갑니다. 내 홍보함에서 대기·승인·반려를 바로 봅니다.' },
]
</script>

<style scoped>
.fic { font-size: 28px; margin: 0 0 8px; }
.ftitle { font-size: 17px; font-weight: 800; margin: 0 0 6px; }
.fdesc { font-size: 14px; margin: 0; }
</style>
