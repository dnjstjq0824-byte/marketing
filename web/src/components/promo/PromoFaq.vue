<template>
  <section class="pu-sec" aria-label="자주 묻는 질문">
    <div class="pu-sec-head">
      <h2 class="pu-sec-title">자주 묻는 질문</h2>
    </div>
    <div class="faqlist">
      <details v-for="f in FAQS" :key="f.q" class="pu-card faq">
        <summary>{{ f.q }}</summary>
        <p class="pu-muted">{{ f.a }}</p>
      </details>
    </div>
  </section>
</template>

<script setup>
const FAQS = [
  { q: '비용이 드나요?', a: '아니요. 가입과 신청은 무료입니다. 승인된 뒤 홍보 링크로 주문이 들어오면 수수료를 받습니다.' },
  { q: '수수료는 어떻게 계산되나요?', a: '판매가의 %가 아니라 주문 1건당 고정 금액입니다. 금액은 제품 카드에서 신청 전에 확인할 수 있습니다.' },
  { q: '누구나 신청할 수 있나요?', a: '가입 후 누구나 신청할 수 있고, 운영자가 승인합니다. 결과는 내 홍보함에서 대기·승인·반려로 확인합니다.' },
  { q: '홍보 링크는 언제 받나요?', a: '승인되면 내 홍보함에서 비공개 구매 링크가 열립니다. 본인만 사용하고 외부 공유는 금지입니다.' },
  { q: '정산은 언제 되나요?', a: '확인이 끝난 주문만 실적에 들어갑니다. 정산 주기·방법은 제품마다 다르니 상세의 정산 조건을 확인하세요.' },
]
</script>

<style scoped>
.faqlist { display: flex; flex-direction: column; gap: 10px; }
.faq { padding: 0; }
.faq summary { padding: 16px 18px; font-weight: 800; font-size: 15px; cursor: pointer; list-style: none; }
.faq summary::-webkit-details-marker { display: none; }
.faq summary::after { content: '+'; float: right; color: var(--pu-accent-dark); font-weight: 800; }
.faq[open] summary::after { content: '−'; }
.faq p { margin: 0; padding: 0 18px 16px; font-size: 14px; white-space: pre-line; }
</style>
