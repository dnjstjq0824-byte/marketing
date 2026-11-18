<template>
  <div class="pu-stats">
    <div class="pu-stat"><p class="pu-muted pu-small">제품 수</p><p class="pu-num">{{ n(ov.products) }}</p></div>
    <div class="pu-stat"><p class="pu-muted pu-small">승인 대기</p><p class="pu-num">{{ n(ov.pending) }}</p></div>
    <div class="pu-stat"><p class="pu-muted pu-small">승인 홍보자</p><p class="pu-num">{{ n(ov.promoters) }}</p></div>
    <div class="pu-stat"><p class="pu-muted pu-small">지급 예정 수수료</p><p class="pu-num">{{ won(ov.payableTotal) }}</p></div>
  </div>
</template>

<script setup>
import { formatWon } from '../../stores/member.js'

defineProps({ ov: { type: Object, default: () => ({}) } })

function n(v) {
  return Number(v || 0).toLocaleString('ko-KR')
}
function won(v) {
  return formatWon(v || 0)
}
</script>
