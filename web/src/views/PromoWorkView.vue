<template>
  <PublicShell>
    <h1 class="pu-h1">마케팅 작업</h1>
    <div v-if="loading" class="pu-muted">불러오는 중...</div>
    <template v-else-if="!items.length">
      <p class="pu-lead">아직 승인된 제품이 없습니다.</p>
      <div v-if="apps.length" class="pu-grid">
        <div v-for="a in apps" :key="a.id" class="pu-card">
          <div class="pu-rowline">
            <strong>{{ a.product }}</strong>
            <span class="pu-chip" :class="a.status === 'rejected' ? 'warn' : 'hold'">{{ statusText(a.status) }}</span>
          </div>
          <p v-if="a.status === 'rejected' && a.reason" class="pu-small">사유: {{ a.reason }}</p>
        </div>
      </div>
      <p><RouterLink to="/promo/products" class="pu-btn pu-btn-primary">홍보할 제품 신청하기</RouterLink></p>
    </template>
    <template v-else>
      <div class="pu-card" style="margin-bottom: 12px">
        <strong>내 실적</strong>
        <span class="pu-muted"> · 확인된 주문 {{ orders }}건 × 건당 수수료 = </span>
        <strong class="pu-fee">{{ formatWon(total) }}</strong>
      </div>
      <div v-for="it in items" :key="it.id" class="pu-card" style="margin-bottom: 12px">
        <div class="pu-rowline">
          <strong>{{ it.product }}</strong>
          <span class="pu-fee pu-small">{{ formatFee(it.feeAgreed) }}</span>
        </div>
        <p class="pu-muted pu-small">홍보 채널: {{ it.channels }} · 확인된 주문 {{ it.confirmedOrders }}건</p>
        <img v-if="it.photo" :src="it.photo" :alt="it.product" class="pu-img" style="max-width: 360px" />
        <h3 class="pu-sec-title" style="margin-top: 12px">홍보 포인트</h3>
        <p class="pu-small" style="white-space: pre-line">{{ it.points || '없음' }}</p>
        <h3 class="pu-sec-title">하면 안 되는 표현</h3>
        <p class="pu-small" style="white-space: pre-line">{{ it.bannedWords || '없음' }}</p>
        <h3 class="pu-sec-title">제공 자료</h3>
        <p class="pu-small" style="white-space: pre-line">{{ it.materials || '없음' }}</p>
        <h3 class="pu-sec-title">비공개 링크</h3>
        <p v-if="links[it.id]" class="pu-small" style="word-break: break-all">{{ links[it.id] }}</p>
        <p>
          <button v-if="!links[it.id]" class="pu-btn small" @click="openLink(it)">링크 보기</button>
          <button v-else class="pu-btn small" @click="copyText(links[it.id])">복사</button>
        </p>
        <h3 class="pu-sec-title">홍보한 게시물</h3>
        <ul v-if="it.posts && it.posts.length" class="pu-small">
          <li v-for="p in it.posts" :key="p.id"><a :href="p.url" target="_blank" rel="noopener">{{ p.url }}</a></li>
        </ul>
        <p v-else class="pu-muted pu-small">아직 등록된 게시물이 없습니다.</p>
        <div class="pu-rowline">
          <input v-model="postUrls[it.id]" placeholder="https:// 게시물 주소" style="flex: 1; min-width: 200px; border: 1px solid var(--pu-line); border-radius: var(--pu-r); padding: 8px 10px; font-size: 14px" />
          <button class="pu-btn small" @click="addPost(it)">등록</button>
        </div>
        <p v-if="postMsg[it.id]" class="pu-error pu-small">{{ postMsg[it.id] }}</p>
      </div>
    </template>
  </PublicShell>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import PublicShell from '../components/PublicShell.vue'
import { formatFee, formatWon, promoFetch } from '../stores/member.js'
import { toast } from '../stores/toast.js'

const items = ref([])
const apps = ref([])
const orders = ref(0)
const total = ref(0)
const links = ref({})
const postUrls = ref({})
const postMsg = ref({})
const loading = ref(true)

function statusText(s) {
  return s === 'approved' ? '승인' : s === 'rejected' ? '반려' : '승인 대기'
}

async function reload() {
  const data = await promoFetch('/api/promo/work')
  items.value = data.items || []
  orders.value = data.orders || 0
  total.value = data.total || 0
  if (!items.value.length) {
    const mine = await promoFetch('/api/promo/applications')
    apps.value = mine.applications || []
  }
}

onMounted(async () => {
  try {
    await reload()
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
})

async function openLink(it) {
  try {
    const data = await promoFetch(`/api/promo/applications/${it.id}/private-link`)
    links.value = { ...links.value, [it.id]: data.url }
  } catch (e) {
    toast(e.message || '실패했습니다', 'error')
  }
}

async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text || '')
    toast('복사했습니다')
  } catch {
    toast('복사에 실패했습니다', 'error')
  }
}

async function addPost(it) {
  postMsg.value = { ...postMsg.value, [it.id]: '' }
  try {
    await promoFetch(`/api/promo/applications/${it.id}/posts`, {
      method: 'POST',
      body: JSON.stringify({ url: postUrls.value[it.id] || '' }),
    })
    postUrls.value = { ...postUrls.value, [it.id]: '' }
    toast('등록했습니다')
    await reload()
  } catch (e) {
    postMsg.value = { ...postMsg.value, [it.id]: e.message || '실패했습니다' }
  }
}
</script>
