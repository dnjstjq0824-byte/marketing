<template>
  <PublicShell>
    <p class="backrow"><RouterLink to="/promo/products" class="pu-sec-link">← 목록으로</RouterLink></p>
    <div v-if="loading" class="pu-muted">불러오는 중...</div>
    <p v-else-if="!product" class="pu-card">제품을 찾을 수 없습니다.</p>
    <template v-else>
      <h1 class="pu-h1">{{ product.name }}</h1>
      <p class="pu-lead">{{ product.store }} · {{ formatWon(product.price) }}</p>
      <div class="pu-detail">
        <div>
          <img v-if="product.photo" :src="product.photo" :alt="product.name" class="pu-img" style="margin-bottom: 12px" />
          <div v-for="s in sections" :key="s.title" class="pu-card" style="margin-bottom: 12px">
            <h2 class="pu-sec-title">{{ s.title }}</h2>
            <p style="white-space: pre-line; margin: 8px 0 0">{{ s.body }}</p>
          </div>
        </div>
        <aside class="pu-card dside">
          <span class="pu-chip" :class="product.status === 'recruiting' ? 'ok' : 'neutral'">{{ product.status === 'recruiting' ? '모집 중' : '마감' }}</span>
          <p class="pu-muted pu-small" style="margin: 8px 0 0">{{ product.store }}</p>
          <p style="font-size: 20px; font-weight: 800; margin: 0">{{ formatWon(product.price) }}</p>
          <div class="pu-feebox">
            <div class="pu-small">홍보 수수료</div>
            <div class="pu-fee">{{ formatFee(product.feePerOrder) }}</div>
          </div>
          <p class="pu-small" style="margin: 0 0 12px"><span class="pu-dday" :class="{ over: ddayUntil(product.recruitUntil).over }">{{ ddayUntil(product.recruitUntil).text }}</span> · <span class="pu-muted">신청 {{ product.applicants || 0 }}명</span></p>
          <div class="qacts">
            <a v-if="product.status === 'recruiting' && !mine" href="#apply" class="pu-btn pu-btn-primary block">신청하기</a>
            <RouterLink v-if="mine" to="/promo/me" class="pu-btn block">내 홍보함 보기</RouterLink>
            <button class="pu-btn block" @click="copyLink">링크 복사</button>
            <RouterLink to="/promo/products" class="pu-btn block">목록으로</RouterLink>
          </div>
          <p class="pu-muted pu-small" style="margin-bottom: 0">승인된 분에게만 실제 구매 링크가 열립니다.</p>
        </aside>
      </div>
      <div v-if="product.status !== 'recruiting'" class="pu-card">
        <p style="margin: 0"><strong>모집이 마감됐습니다.</strong></p>
      </div>
      <div v-else-if="!member.user" id="apply" class="pu-card">
        <p>신청하려면 로그인이 필요합니다.</p>
        <RouterLink :to="{ path: '/promo/login', query: { next: route.fullPath } }" class="pu-btn pu-btn-primary">로그인하러 가기</RouterLink>
      </div>
      <div v-else-if="mine" class="pu-card">
        <p style="margin: 0">신청 완료 — <strong>{{ statusText(mine.status) }}</strong><span v-if="mine.status === 'rejected' && mine.reason"> ({{ mine.reason }})</span></p>
      </div>
      <div v-else id="apply" class="pu-card">
        <h2 class="pu-sec-title">홍보 신청</h2>
        <div class="pu-form" style="margin-top: 12px">
          <label>홍보 채널·계정 <input v-model="form.channels" placeholder="예: 네이버 블로그 @손끝일기" /></label>
          <label>홍보 계획 <textarea v-model="form.plan" placeholder="어떻게 홍보할지 적어주세요"></textarea></label>
          <label class="pu-check"><input v-model="form.agreed" type="checkbox" /> {{ formatFee(product.feePerOrder) }} 수수료 조건을 확인했습니다.</label>
        </div>
        <p v-if="msg" class="pu-error">{{ msg }}</p>
        <p style="margin-top: 12px; margin-bottom: 0"><button class="pu-btn pu-btn-primary" @click="submit">신청하기</button></p>
      </div>
    </template>
  </PublicShell>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import PublicShell from '../components/PublicShell.vue'
import { ddayUntil, formatFee, formatWon, member, promoFetch } from '../stores/member.js'
import { toast } from '../stores/toast.js'

const route = useRoute()
const product = ref(null)
const loading = ref(true)
const mine = ref(null)
const form = ref({ channels: '', plan: '', agreed: false })
const msg = ref('')

const sections = computed(() => {
  const p = product.value || {}
  return [
    { title: '제품 사실 정보', body: p.facts || '등록된 정보가 없습니다.' },
    { title: '홍보 포인트', body: p.points || '등록된 정보가 없습니다.' },
    { title: '하면 안 되는 표현', body: p.bannedWords || '등록된 정보가 없습니다.' },
    { title: '제공 자료 안내', body: p.materials || '등록된 정보가 없습니다.' },
    { title: '수수료·정산 조건', body: p.settleTerms || '수수료는 주문 1건당 고정 금액으로 지급됩니다.' },
  ]
})

function statusText(s) {
  return s === 'approved' ? '승인' : s === 'rejected' ? '반려' : '승인 대기'
}

async function copyLink() {
  try {
    await navigator.clipboard.writeText(window.location.href)
    toast('링크를 복사했습니다')
  } catch {
    toast('복사에 실패했습니다')
  }
}

onMounted(async () => {
  try {
    product.value = await promoFetch(`/api/promo/products/${route.params.id}`)
    if (member.token && product.value.status === 'recruiting') {
      try {
        const data = await promoFetch('/api/promo/applications')
        mine.value = (data.applications || []).find((a) => a.productId === product.value.id) || null
      } catch {
        mine.value = null
      }
    }
  } catch {
    product.value = null
  } finally {
    loading.value = false
  }
})

async function submit() {
  msg.value = ''
  try {
    const a = await promoFetch(`/api/promo/products/${product.value.id}/apply`, {
      method: 'POST',
      body: JSON.stringify({ channels: form.value.channels, plan: form.value.plan, feeAgreed: form.value.agreed }),
    })
    mine.value = a
    toast('신청했습니다')
  } catch (e) {
    if (e.status === 409) {
      msg.value = '이미 신청한 제품입니다.'
      mine.value = mine.value || { status: 'pending', reason: '' }
    } else {
      msg.value = e.message || '실패했습니다'
    }
  }
}
</script>

<style scoped>
.backrow { margin: 0 0 10px; }
.dside { position: sticky; top: 84px; }
.dside .pu-feebox { background: var(--pu-accent-soft); border-radius: var(--pu-r-sm); padding: 14px; margin: 12px 0; }
.dside .pu-feebox .pu-fee { font-size: 19px; }
.qacts { display: flex; flex-direction: column; gap: 8px; margin-bottom: 12px; }
@media (max-width: 900px) { .dside { position: static; } }
</style>
