<template>
  <PublicShell>
    <h1 class="pu-h1">홍보 위탁 제품</h1>
    <p class="pu-lead">건당 수수료를 확인하고 신청하세요. 실제 구매 링크는 승인된 분에게만 열립니다.</p>
    <div class="pu-cats" role="list">
      <button class="pu-cat" :class="{ on: !cat }" @click="cat = ''"><i>전</i><span>전체</span></button>
      <button v-for="c in CATEGORIES" :key="c" class="pu-cat" :class="{ on: cat === c }" @click="cat = c"><i>{{ c.slice(0, 1) }}</i><span>{{ c }}</span></button>
    </div>
    <div v-if="loading" class="pu-muted">불러오는 중...</div>
    <div v-else-if="!shown.length" class="pu-card pu-empty">해당 제품이 없습니다.</div>
    <div v-else class="pu-cgrid">
      <RouterLink v-for="p in shown" :key="p.id" :to="'/promo/products/' + p.id" class="pu-card pu-pcard pu-ccard">
        <img v-if="p.photo" :src="p.photo" :alt="p.name" class="pu-img" />
        <div class="pu-pbody">
          <p class="pu-pname">{{ p.name }} <span v-if="p.status !== 'recruiting'" class="pu-chip neutral">마감</span></p>
          <p class="pu-pmeta">{{ p.store }} · {{ formatWon(p.price) }}</p>
          <p class="pu-pfee pu-fee">{{ formatFee(p.feePerOrder) }}</p>
          <div class="pu-pfoot">
            <span class="pu-dday" :class="{ over: ddayUntil(p.recruitUntil).over }">{{ ddayUntil(p.recruitUntil).text }}</span>
            <span class="pu-applied">신청 {{ p.applicants || 0 }}명</span>
          </div>
        </div>
      </RouterLink>
    </div>
  </PublicShell>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import PublicShell from '../components/PublicShell.vue'
import { CATEGORIES, ddayUntil, formatFee, formatWon, promoFetch } from '../stores/member.js'

const products = ref([])
const loading = ref(true)
const cat = ref('')

const shown = computed(() => {
  const list = cat.value ? products.value.filter((p) => p.category === cat.value) : products.value
  return [...list].sort((a, b) => {
    if ((a.status === 'recruiting') !== (b.status === 'recruiting')) return a.status === 'recruiting' ? -1 : 1
    return (b.applicants || 0) - (a.applicants || 0)
  })
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
