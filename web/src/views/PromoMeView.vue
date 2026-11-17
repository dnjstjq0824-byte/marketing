<template>
  <PublicShell>
    <h1 class="pu-h1">내 홍보함</h1>
    <p class="pu-lead">신청 상태와 승인된 제품의 비공개 링크를 확인합니다.</p>
    <div v-if="loading" class="pu-muted">불러오는 중...</div>
    <p v-else-if="!items.length" class="pu-card">아직 신청이 없습니다. <RouterLink to="/promo/products">제품 보러 가기</RouterLink></p>
    <div v-else class="pu-grid">
      <div v-for="a in items" :key="a.id" class="pu-card">
        <div class="pu-rowline">
          <strong>{{ a.product }}</strong>
          <span class="pu-chip" :class="a.status === 'approved' ? 'ok' : a.status === 'rejected' ? 'warn' : 'hold'">{{ statusText(a.status) }}</span>
        </div>
        <p class="pu-muted pu-small">{{ formatFee(a.feeAgreed) }} · {{ shortDate(a.createdAt) }}</p>
        <p v-if="a.status === 'rejected' && a.reason" class="pu-small">반려 사유: {{ a.reason }}</p>
        <div v-if="a.status === 'approved'">
          <p v-if="links[a.id]" class="pu-small" style="word-break: break-all">{{ links[a.id] }}</p>
          <p>
            <button v-if="!links[a.id]" class="pu-btn small" @click="openLink(a)">링크 보기</button>
            <button v-else class="pu-btn small" @click="copyLink(a)">복사</button>
          </p>
        </div>
        <p v-else><span class="pu-chip neutral">승인 후 열림</span></p>
      </div>
    </div>
  </PublicShell>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import PublicShell from '../components/PublicShell.vue'
import { formatFee, promoFetch } from '../stores/member.js'
import { toast } from '../stores/toast.js'

const items = ref([])
const links = ref({})
const loading = ref(true)

function statusText(s) {
  return s === 'approved' ? '승인' : s === 'rejected' ? '반려' : '승인 대기'
}

function shortDate(iso) {
  return (iso || '').slice(0, 10)
}

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

async function openLink(a) {
  try {
    const data = await promoFetch(`/api/promo/applications/${a.id}/private-link`)
    links.value = { ...links.value, [a.id]: data.url }
  } catch (e) {
    toast(e.message || '실패했습니다', 'error')
  }
}

async function copyLink(a) {
  try {
    await navigator.clipboard.writeText(links.value[a.id] || '')
    toast('복사했습니다')
  } catch {
    toast('복사에 실패했습니다', 'error')
  }
}
</script>
