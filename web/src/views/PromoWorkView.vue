<template>
  <PublicShell>
    <h1 class="pu-h1">마케팅 작업</h1>
    <div v-if="loading" class="pu-muted">불러오는 중...</div>
    <template v-else-if="!items.length">
      <p class="pu-lead">아직 승인된 제품이 없습니다.</p>
      <div v-if="apps.length" class="pu-grid wapps">
        <MemberAppCard v-for="a in apps" :key="a.id" :app="a" />
      </div>
      <p><RouterLink to="/promo/products" class="pu-btn pu-btn-primary">홍보할 제품 신청하기</RouterLink></p>
    </template>
    <template v-else>
      <div class="pu-card wband"><strong>내 실적</strong><span class="pu-muted"> · 확인된 주문 <span class="pu-num">{{ orders }}</span>건 × 건당 수수료 = </span><strong class="pu-fee pu-num">{{ formatWon(total) }}</strong></div>
      <div v-for="it in items" :key="it.id" class="pu-card wcard">
        <div class="pu-rowline">
          <strong>{{ it.product }}</strong>
          <span class="pu-fee pu-small pu-num">{{ formatFee(it.feeAgreed) }}</span>
        </div>
        <p class="pu-muted pu-small">홍보 채널: {{ it.channels }} · 확인된 주문 <span class="pu-num">{{ it.confirmedOrders }}</span>건</p>
        <img v-if="it.photo" :src="it.photo" :alt="it.product" class="pu-img" />
        <h3 class="pu-sec-title wsec">홍보 포인트</h3>
        <p class="pu-small wpre">{{ it.points || '없음' }}</p>
        <h3 class="pu-sec-title wsec">하면 안 되는 표현</h3>
        <p class="pu-small wpre">{{ it.bannedWords || '없음' }}</p>
        <h3 class="pu-sec-title wsec">제공 자료</h3>
        <p class="pu-small wpre">{{ it.materials || '없음' }}</p>
        <h3 class="pu-sec-title wsec">비공개 링크</h3>
        <PrivateLinkBox :app-id="it.id" />
        <h3 class="pu-sec-title wsec">홍보한 게시물</h3>
        <ul v-if="it.posts && it.posts.length" class="pu-small wposts">
          <li v-for="p in it.posts" :key="p.id"><a :href="p.url" target="_blank" rel="noopener">{{ p.url }}</a></li>
        </ul>
        <p v-else class="pu-muted pu-small">아직 등록된 게시물이 없습니다.</p>
        <WorkPostForm :app-id="it.id" @posted="reload" />
      </div>
    </template>
  </PublicShell>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import PublicShell from '../components/PublicShell.vue'
import MemberAppCard from '../components/promo/MemberAppCard.vue'
import PrivateLinkBox from '../components/promo/PrivateLinkBox.vue'
import WorkPostForm from '../components/promo/WorkPostForm.vue'
import { formatFee, formatWon, promoFetch } from '../stores/member.js'

const items = ref([])
const apps = ref([])
const orders = ref(0)
const total = ref(0)
const loading = ref(true)

async function reload() {
  const data = await promoFetch('/api/promo/work')
  items.value = data.items || []
  orders.value = data.orders || 0
  total.value = data.total || 0
  if (!items.value.length) { const mine = await promoFetch('/api/promo/applications'); apps.value = mine.applications || [] }
}

onMounted(async () => {
  try { await reload() } catch { items.value = [] } finally { loading.value = false }
})
</script>

<style scoped>
.wband { margin-bottom: 12px; }
.wapps { margin-bottom: 16px; }
.wcard { margin-bottom: 12px; }
.wsec { margin-top: 12px; font-size: 16px; }
.wpre { white-space: pre-line; margin-top: 4px; }
.wposts { word-break: break-all; }
.pu-img { max-width: 360px; margin-top: 8px; }
</style>
