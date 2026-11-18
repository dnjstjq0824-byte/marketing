<template>
  <PublicShell>
    <h1 class="pu-h1">내 홍보함</h1>
    <p class="pu-lead">신청 상태와 승인된 제품의 비공개 링크를 확인합니다.</p>
    <div v-if="loading" class="pu-muted">불러오는 중...</div>
    <template v-else>
      <div v-if="items.length" class="pu-tabs" role="tablist" aria-label="상태 필터">
        <button
          v-for="f in FILTERS"
          :key="f.id"
          class="pu-tab"
          :class="{ on: filter === f.id }"
          role="tab"
          :aria-selected="filter === f.id"
          @click="filter = f.id"
        >
          {{ f.label }}
        </button>
      </div>
      <p v-if="!items.length" class="pu-card">아직 신청이 없습니다. <RouterLink to="/promo/products">제품 보러 가기</RouterLink></p>
      <p v-else-if="!shown.length" class="pu-card pu-empty">해당 상태의 신청이 없습니다.</p>
      <div v-else class="pu-grid">
        <MemberAppCard v-for="a in shown" :key="a.id" :app="a" />
      </div>
    </template>
  </PublicShell>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import PublicShell from '../components/PublicShell.vue'
import MemberAppCard from '../components/promo/MemberAppCard.vue'
import { promoFetch } from '../stores/member.js'

const FILTERS = [
  { id: 'all', label: '전체' },
  { id: 'pending', label: '승인 대기' },
  { id: 'approved', label: '승인' },
  { id: 'rejected', label: '반려' },
]

const items = ref([])
const loading = ref(true)
const filter = ref('all')

function match(a) {
  if (filter.value === 'all') return true
  if (filter.value === 'pending') return a.status !== 'approved' && a.status !== 'rejected'
  return a.status === filter.value
}
const shown = computed(() => items.value.filter(match))

onMounted(async () => {
  try {
    const data = await promoFetch('/api/promo/applications')
    items.value = data.applications || []
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
})
</script>
