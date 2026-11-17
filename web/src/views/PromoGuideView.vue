<template>
  <PublicShell>
    <h1 class="pu-h1">사용 가이드</h1>
    <p class="pu-lead">처음 오셨다면 이 순서대로 따라오세요.</p>
    <div class="pu-tabs">
      <button class="pu-tab" :class="{ on: guide === 0 }" @click="guide = 0">홍보 시작하기</button>
      <button class="pu-tab" :class="{ on: guide === 1 }" @click="guide = 1">홍보 규칙</button>
      <button class="pu-tab" :class="{ on: guide === 2 }" @click="guide = 2">수수료·정산</button>
    </div>
    <div class="pu-card">
      <ul v-if="guide === 0" class="pu-guide-list">
        <li><strong>1. 회원가입·로그인</strong><span>이메일로 가입하면 바로 시작됩니다.</span></li>
        <li><strong>2. 제품 둘러보기</strong><span>카테고리별로 모집 중 제품을 봅니다.</span></li>
        <li><strong>3. 홍보 신청</strong><span>채널·계획을 적고 수수료 조건에 동의합니다.</span></li>
        <li><strong>4. 승인 확인</strong><span>내 홍보함에서 대기·승인·반려를 봅니다.</span></li>
        <li><strong>5. 링크로 홍보</strong><span>승인되면 비공개 링크가 열립니다.</span></li>
        <li><strong>6. 실적 적립</strong><span>확인된 주문 1건마다 수수료가 쌓입니다.</span></li>
      </ul>
      <ul v-else-if="guide === 1" class="pu-guide-list">
        <li><strong>제품별 금지 표현 준수</strong><span>상세 화면의 금지 표현을 반드시 지킵니다.</span></li>
        <li><strong>허위·과장 금지</strong><span>가격·효과·순위를 지어내지 않습니다.</span></li>
        <li><strong>비공개 링크 본인 사용</strong><span>승인받은 본인만 씁니다. 외부 공유는 금지입니다.</span></li>
        <li><strong>광고 표기 준수</strong><span>관계 법령에 따라 광고임을 밝힙니다.</span></li>
      </ul>
      <ul v-else class="pu-guide-list">
        <li><strong>건당 고정 금액</strong><span>판매가의 %가 아니라 주문 1건당 고정입니다.</span></li>
        <li><strong>제품마다 상이</strong><span>금액은 카드에서 신청 전에 확인합니다.</span></li>
        <li><strong>확인된 주문만 집계</strong><span>확인이 끝나야 실적에 들어갑니다.</span></li>
        <li><strong>정산 조건은 제품별 상이</strong><span>주기·방법은 제품 상세에서 확인합니다.</span></li>
      </ul>
    </div>
  </PublicShell>
</template>

<script setup>
import { ref } from 'vue'
import PublicShell from '../components/PublicShell.vue'

const guide = ref(0)
</script>
