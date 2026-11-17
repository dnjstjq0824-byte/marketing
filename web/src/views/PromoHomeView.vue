<template>
  <PublicShell>
    <section class="pu-banners" aria-label="안내">
      <RouterLink to="/promo/products" class="pu-banner b1">
        <strong>홍보 위탁 <em>오픈</em></strong>
        <span>마음에 드는 제품에 신청하세요</span>
      </RouterLink>
      <RouterLink to="/promo/products" class="pu-banner b2">
        <strong>수수료는 <em>건당 고정</em></strong>
        <span>%가 아닌 주문 1건당 금액으로 받습니다</span>
      </RouterLink>
      <RouterLink to="/promo/work" class="pu-banner b3">
        <strong>마케팅 <em>시작하기</em></strong>
        <span>승인된 제품으로 바로 일합니다</span>
      </RouterLink>
    </section>
    <section class="pu-sec" aria-label="카테고리">
      <div class="pu-cats" role="list">
        <button class="pu-cat" :class="{ on: !cat }" @click="cat = ''"><i>전</i><span>전체</span></button>
        <button v-for="c in CATEGORIES" :key="c" class="pu-cat" :class="{ on: cat === c }" @click="cat = c"><i>{{ c.slice(0, 1) }}</i><span>{{ c }}</span></button>
      </div>
    </section>
    <section class="pu-sec" aria-label="인기 모집">
      <div class="pu-sec-head">
        <h2 class="pu-sec-title">인기 모집</h2>
        <RouterLink to="/promo/products" class="pu-sec-link">더보기 →</RouterLink>
      </div>
      <div v-if="loading" class="pu-muted">불러오는 중...</div>
      <div v-else-if="!shown.length" class="pu-card pu-empty">현재 모집 중인 제품이 없습니다.</div>
      <div v-else class="pu-cgrid">
        <RouterLink v-for="p in shown" :key="p.id" :to="'/promo/products/' + p.id" class="pu-card pu-pcard pu-ccard">
          <img v-if="p.photo" :src="p.photo" :alt="p.name" class="pu-img" />
          <div class="pu-pbody">
            <p class="pu-pname">{{ p.name }}</p>
            <p class="pu-pmeta">{{ p.store }}</p>
            <p class="pu-pfee pu-fee">{{ formatFee(p.feePerOrder) }}</p>
            <div class="pu-pfoot">
              <span class="pu-dday" :class="{ over: ddayUntil(p.recruitUntil).over }">{{ ddayUntil(p.recruitUntil).text }}</span>
              <span class="pu-applied">신청 {{ p.applicants || 0 }}명</span>
            </div>
          </div>
        </RouterLink>
      </div>
    </section>
    <section class="pu-sec" aria-label="사용 가이드">
      <div class="pu-band">
        <div class="pu-sec-head" style="margin-bottom: 4px">
          <h2 class="pu-sec-title">처음이세요?</h2>
          <RouterLink to="/promo/guide" class="pu-sec-link">사용 가이드 보기 →</RouterLink>
        </div>
        <p style="margin: 0">홍보 시작하기·홍보 규칙·수수료 정산 방법을 한곳에 모았습니다.</p>
      </div>
    </section>
  </PublicShell>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import PublicShell from '../components/PublicShell.vue'
import { CATEGORIES, ddayUntil, formatFee, promoFetch } from '../stores/member.js'

const products = ref([])
const loading = ref(true)
const cat = ref('')

const shown = computed(() => {
  const open = products.value.filter((p) => p.status === 'recruiting')
  const list = cat.value ? open.filter((p) => p.category === cat.value) : open
  return [...list].sort((a, b) => (b.applicants || 0) - (a.applicants || 0)).slice(0, 10)
})

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
