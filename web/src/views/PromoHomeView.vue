<template>
  <PublicShell>
    <PromoHero />
    <PromoStats />
    <section class="pu-sec" aria-label="인기 모집">
      <div class="pu-sec-head">
        <h2 class="pu-sec-title">인기 모집</h2>
        <RouterLink to="/promo/products" class="pu-sec-link">더보기 →</RouterLink>
      </div>
      <div v-if="loading" class="pu-muted">불러오는 중...</div>
      <div v-else-if="!shown.length" class="pu-card pu-empty">현재 모집 중인 제품이 없습니다.</div>
      <div v-else class="pu-cgrid">
        <PromoProductCard v-for="p in shown" :key="p.id" :product="p" />
      </div>
    </section>
    <PromoFeatureCards />
    <PromoTimeline />
    <PromoFaq />
    <PromoCtaBanner />
  </PublicShell>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import PublicShell from '../components/PublicShell.vue'
import PromoHero from '../components/promo/PromoHero.vue'
import PromoStats from '../components/promo/PromoStats.vue'
import PromoFeatureCards from '../components/promo/PromoFeatureCards.vue'
import PromoTimeline from '../components/promo/PromoTimeline.vue'
import PromoFaq from '../components/promo/PromoFaq.vue'
import PromoCtaBanner from '../components/promo/PromoCtaBanner.vue'
import PromoProductCard from '../components/promo/PromoProductCard.vue'
import { promoFetch } from '../stores/member.js'

const products = ref([])
const loading = ref(true)

const shown = computed(() =>
  products.value
    .filter((p) => p.status === 'recruiting')
    .sort((a, b) => (b.applicants || 0) - (a.applicants || 0))
    .slice(0, 10),
)

onMounted(async () => {
  try {
    const data = await promoFetch('/api/promo/products')
    products.value = data.products || []
  } catch {
    products.value = []
  } finally {
    loading.value = false
  }
})
</script>
