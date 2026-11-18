<template>
  <section v-if="stats" class="pu-sec" aria-label="현재 모집 현황">
    <div class="pu-stats stats3">
      <div class="pu-stat">
        <p class="pu-muted pu-small">모집 중 상품</p>
        <p class="pu-num">{{ fmt(stats.recruiting) }}<span class="unit">개</span></p>
      </div>
      <div class="pu-stat">
        <p class="pu-muted pu-small">활동 프로모터</p>
        <p class="pu-num">{{ fmt(stats.promoters) }}<span class="unit">명</span></p>
      </div>
      <div class="pu-stat">
        <p class="pu-muted pu-small">등록 홍보 포스트</p>
        <p class="pu-num">{{ fmt(stats.posts) }}<span class="unit">건</span></p>
      </div>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { promoFetch } from '../../stores/member.js'

const stats = ref(null)

function fmt(n) {
  return Number(n || 0).toLocaleString('ko-KR')
}

onMounted(async () => {
  try {
    stats.value = await promoFetch('/api/promo/stats')
  } catch {
    stats.value = null
  }
})
</script>

<style scoped>
.stats3 { grid-template-columns: repeat(3, 1fr); }
.unit { font-size: 15px; font-weight: 700; color: var(--pu-muted); margin-left: 4px; }
@media (max-width: 900px) { .stats3 { grid-template-columns: repeat(3, 1fr); } }
</style>
