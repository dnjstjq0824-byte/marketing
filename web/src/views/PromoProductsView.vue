<template>
  <PublicShell>
    <h1 class="pu-h1">홍보 위탁 제품</h1>
    <p class="pu-lead">건당 수수료를 확인하고 신청하세요. 실제 구매 링크는 승인된 분에게만 열립니다.</p>
    <PromoFilters :filters="filters" @change="onFilters" />
    <p class="pu-muted pu-small" aria-live="polite">총 {{ shown.length }}개</p>
    <div v-if="loading" class="pu-muted">불러오는 중...</div>
    <div v-else-if="!shown.length" class="pu-card pu-empty">조건에 맞는 제품이 없습니다.</div>
    <div v-else class="pu-cgrid">
      <PromoProductCard v-for="p in shown" :key="p.id" :product="p" />
    </div>
  </PublicShell>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PublicShell from '../components/PublicShell.vue'
import PromoFilters from '../components/promo/PromoFilters.vue'
import PromoProductCard from '../components/promo/PromoProductCard.vue'
import { CATEGORIES, promoFetch } from '../stores/member.js'

const route = useRoute()
const router = useRouter()
const products = ref([])
const loading = ref(true)

const SORTS = ['popular', 'deadline', 'fee', 'new']

function fromQuery(q) {
  return {
    q: typeof q.q === 'string' ? q.q : '',
    cat: typeof q.cat === 'string' && CATEGORIES.includes(q.cat) ? q.cat : '',
    sort: typeof q.sort === 'string' && SORTS.includes(q.sort) ? q.sort : 'popular',
    openOnly: q.open === '1',
  }
}

const filters = ref(fromQuery(route.query))

function onFilters(next) {
  filters.value = next
}

watch(
  filters,
  (f) => {
    const query = {}
    if (f.q) query.q = f.q
    if (f.cat) query.cat = f.cat
    if (f.sort !== 'popular') query.sort = f.sort
    if (f.openOnly) query.open = '1'
    router.replace({ query })
  },
  { deep: true },
)

function deadlineOf(p) {
  const t = new Date((p.recruitUntil || '') + 'T00:00:00').getTime()
  return Number.isNaN(t) ? Infinity : t
}

const shown = computed(() => {
  const kw = filters.value.q.trim().toLowerCase()
  let list = products.value.filter((p) => {
    if (filters.value.cat && p.category !== filters.value.cat) return false
    if (filters.value.openOnly && p.status !== 'recruiting') return false
    if (kw && !`${p.name} ${p.store}`.toLowerCase().includes(kw)) return false
    return true
  })
  const by = {
    popular: (a, b) => (b.applicants || 0) - (a.applicants || 0),
    deadline: (a, b) => deadlineOf(a) - deadlineOf(b),
    fee: (a, b) => (b.feePerOrder || 0) - (a.feePerOrder || 0),
    new: (a, b) => b.id - a.id,
  }
  list = [...list].sort(by[filters.value.sort] || by.popular)
  if (filters.value.sort === 'popular') {
    list.sort((a, b) => (a.status === 'recruiting' ? 0 : 1) - (b.status === 'recruiting' ? 0 : 1))
  }
  return list
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
