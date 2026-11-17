<template>
  <PublicShell wide>
    <div class="pu-admin">
      <aside class="pu-side" aria-label="위탁 관리 메뉴">
        <h2>위탁 관리</h2>
        <button :class="{ on: tab === 'dash' }" @click="tab = 'dash'">대시보드</button>
        <button :class="{ on: tab === 'products' }" @click="tab = 'products'">제품 관리</button>
        <button :class="{ on: tab === 'apps' }" @click="tab = 'apps'">신청 관리<span v-if="ov.pending"> ({{ ov.pending }})</span></button>
        <div class="pu-back">
          <RouterLink to="/promo">← 공개 화면으로</RouterLink>
          <RouterLink to="/">운영 콘솔로 →</RouterLink>
        </div>
      </aside>
      <div class="pu-adminbody">
        <div v-if="tab === 'dash'">
          <h1 class="pu-h1">대시보드</h1>
          <div class="pu-stats">
            <div class="pu-stat"><p class="pu-muted pu-small">제품 수</p><p class="pu-num">{{ ov.products || 0 }}</p></div>
            <div class="pu-stat"><p class="pu-muted pu-small">승인 대기</p><p class="pu-num">{{ ov.pending || 0 }}</p></div>
            <div class="pu-stat"><p class="pu-muted pu-small">승인 홍보자</p><p class="pu-num">{{ ov.promoters || 0 }}</p></div>
            <div class="pu-stat"><p class="pu-muted pu-small">지급 예정 수수료</p><p class="pu-num">{{ formatWon(ov.payableTotal || 0) }}</p></div>
          </div>
          <h2 class="pu-sec-title">최근 신청</h2>
          <div v-if="!recent.length" class="pu-muted">아직 신청이 없습니다.</div>
          <div v-else class="pu-rows">
            <div v-for="a in recent" :key="a.id" class="pu-card">
              <div class="pu-rowline">
                <strong>{{ a.product }}</strong>
                <span class="pu-chip" :class="a.status === 'approved' ? 'ok' : a.status === 'rejected' ? 'warn' : 'hold'">{{ statusText(a.status) }}</span>
              </div>
              <p class="pu-muted pu-small">{{ a.memberNickname }} · {{ a.memberEmail }}</p>
            </div>
          </div>
        </div>
        <div v-if="tab === 'products'">
          <div class="pu-sec-head">
            <h1 class="pu-h1">제품 관리</h1>
            <span style="flex: 1"></span>
            <button class="pu-btn pu-btn-primary small" @click="openForm(null)">제품 등록</button>
          </div>
          <div class="pu-rows">
            <div v-for="p in products" :key="p.id" class="pu-card pu-arow">
              <img v-if="p.photo" :src="p.photo" :alt="p.name" class="pu-thumb" />
              <span v-else class="pu-thumb blank">{{ p.name.slice(0, 1) }}</span>
              <div>
                <div class="pu-rowline">
                  <strong>{{ p.name }}</strong>
                  <span class="pu-chip" :class="p.status === 'recruiting' ? 'ok' : 'neutral'">{{ p.status === 'recruiting' ? '모집 중' : '마감' }}</span>
                </div>
                <p class="pu-muted pu-small">{{ p.store }} · {{ formatWon(p.price) }} · {{ formatFee(p.feePerOrder) }}</p>
                <p class="pu-muted pu-small">비공개 링크: {{ p.linkMasked || '미등록' }}</p>
                <p v-if="formMsg && formId === p.id" class="pu-error pu-small">{{ formMsg }}</p>
              </div>
              <div class="pu-rowline pu-acts">
                <button class="pu-btn small" @click="openForm(p)">수정</button>
                <button v-if="p.status !== 'recruiting'" class="pu-btn small" @click="setStatus(p, 'recruiting')">모집 시작</button>
                <button v-else class="pu-btn small" @click="setStatus(p, 'closed')">모집 마감</button>
              </div>
            </div>
          </div>
          <div v-if="formOpen" class="pu-card" style="margin-top: 16px">
            <h2 class="pu-sec-title">{{ formId ? '제품 수정' : '제품 등록' }}</h2>
            <div class="pu-form pu-formgrid">
              <label>제품명 <input v-model="form.name" /></label>
              <label>스토어 <input v-model="form.store" /></label>
              <label>판매가(원) <input v-model.number="form.price" type="number" min="0" /></label>
              <label>사진 주소(URL) <input v-model="form.photo" /></label>
              <label>카테고리
                <select v-model="form.category">
                  <option value="">없음</option>
                  <option v-for="c in CATEGORIES" :key="c" :value="c">{{ c }}</option>
                </select>
              </label>
              <label>모집 마감일 <input v-model="form.recruitUntil" type="date" /></label>
              <label class="full">사실 정보 <textarea v-model="form.facts"></textarea></label>
              <label class="full">홍보 포인트 <textarea v-model="form.points"></textarea></label>
              <label class="full">하면 안 되는 표현 <textarea v-model="form.bannedWords"></textarea></label>
              <label class="full">제공 자료 안내 <textarea v-model="form.materials"></textarea></label>
              <label>주문 건당 수수료(원) <input v-model.number="form.feePerOrder" type="number" min="0" /></label>
              <label>수수료·정산 조건 <input v-model="form.settleTerms" /></label>
              <label class="full">스마트스토어 비공개 링크 <input v-model="form.privateUrl" placeholder="수정 시 비워두면 기존 유지" /></label>
            </div>
            <p v-if="formMsg && !formId" class="pu-error">{{ formMsg }}</p>
            <p class="pu-rowline"><button class="pu-btn pu-btn-primary small" @click="saveForm">저장</button><button class="pu-btn small" @click="formOpen = false">취소</button></p>
          </div>
        </div>
        <div v-if="tab === 'apps'">
          <div class="pu-sec-head">
            <h1 class="pu-h1">신청 관리</h1>
            <span style="flex: 1"></span>
            <label class="pu-small">상태
              <select v-model="appFilter" @change="reload">
                <option value="">전체</option>
                <option value="pending">대기</option>
                <option value="approved">승인</option>
                <option value="rejected">반려</option>
              </select>
            </label>
          </div>
          <div class="pu-rows">
            <div v-for="a in apps" :key="a.id" class="pu-card">
              <div class="pu-rowline">
                <strong>{{ a.product }}</strong>
                <span class="pu-chip" :class="a.status === 'approved' ? 'ok' : a.status === 'rejected' ? 'warn' : 'hold'">{{ statusText(a.status) }}</span>
              </div>
              <p class="pu-muted pu-small">{{ a.memberNickname }} · {{ a.memberEmail }} · {{ formatFee(a.feeAgreed) }}</p>
              <p class="pu-small">채널: {{ a.channels }}</p>
              <p class="pu-small" style="white-space: pre-line">계획: {{ a.plan }}</p>
              <p v-if="a.status === 'rejected' && a.reason" class="pu-small">반려 사유: {{ a.reason }}</p>
              <div v-if="a.status === 'pending'" class="pu-rowline">
                <button class="pu-btn small pu-btn-primary" @click="decide(a, true)">승인</button>
                <input v-model="reasons[a.id]" placeholder="반려 사유" style="border: 1px solid var(--pu-line); border-radius: 10px; padding: 6px 8px; font-size: 13px" />
                <button class="pu-btn small" @click="decide(a, false)">반려</button>
              </div>
              <div v-if="a.status === 'approved'" class="pu-rowline">
                <label class="pu-small">확인 주문 수 <input v-model.number="counts[a.id]" type="number" min="0" style="width: 90px; border: 1px solid var(--pu-line); border-radius: 10px; padding: 6px 8px" /></label>
                <button class="pu-btn small" @click="confirm(a)">확인 저장</button>
              </div>
              <p v-if="appMsg[a.id]" class="pu-error pu-small">{{ appMsg[a.id] }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </PublicShell>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import PublicShell from '../components/PublicShell.vue'
import { apiFetch } from '../stores/auth.js'
import { CATEGORIES, formatFee, formatWon } from '../stores/member.js'
import { toast } from '../stores/toast.js'

const tab = ref('dash')
const ov = ref({})
const products = ref([])
const apps = ref([])
const appFilter = ref('')
const formOpen = ref(false)
const formId = ref(null)
const form = ref({})
const formMsg = ref('')
const reasons = ref({})
const counts = ref({})
const appMsg = ref({})

const recent = computed(() => [...apps.value].slice(0, 5))

function statusText(s) {
  return s === 'approved' ? '승인' : s === 'rejected' ? '반려' : '승인 대기'
}

function blankForm() {
  return { name: '', store: '', price: 0, photo: '', facts: '', points: '', bannedWords: '', materials: '', feePerOrder: 0, settleTerms: '', privateUrl: '', category: '', recruitUntil: '' }
}

async function reload() {
  const [o, ps, as] = await Promise.all([
    apiFetch('/api/promo/admin/overview'),
    apiFetch('/api/promo/admin/products'),
    apiFetch('/api/promo/admin/applications' + (appFilter.value ? `?status=${appFilter.value}` : '')),
  ])
  ov.value = o
  products.value = ps.products || []
  apps.value = as.applications || []
  const c = {}
  for (const a of apps.value) c[a.id] = a.confirmedOrders || 0
  counts.value = c
}

onMounted(async () => {
  try {
    await reload()
  } catch (e) {
    toast(e.message || '실패했습니다', 'error')
  }
})

function openForm(p) {
  formOpen.value = true
  formId.value = p ? p.id : null
  formMsg.value = ''
  form.value = p ? { name: p.name, store: p.store, price: p.price, photo: p.photo, facts: p.facts, points: p.points, bannedWords: p.bannedWords, materials: p.materials, feePerOrder: p.feePerOrder, settleTerms: p.settleTerms, privateUrl: '', category: p.category || '', recruitUntil: p.recruitUntil || '' } : blankForm()
}

async function saveForm() {
  formMsg.value = ''
  try {
    if (formId.value) {
      await apiFetch(`/api/promo/admin/products/${formId.value}`, { method: 'PATCH', body: JSON.stringify(form.value) })
    } else {
      await apiFetch('/api/promo/admin/products', { method: 'POST', body: JSON.stringify(form.value) })
    }
    formOpen.value = false
    toast('저장했습니다')
    await reload()
  } catch (e) {
    formMsg.value = e.message || '실패했습니다'
  }
}

async function setStatus(p, status) {
  formMsg.value = ''
  try {
    await apiFetch(`/api/promo/admin/products/${p.id}/recruit`, { method: 'POST', body: JSON.stringify({ status }) })
    toast('저장했습니다')
    await reload()
  } catch (e) {
    formId.value = p.id
    formMsg.value = e.message || '실패했습니다'
  }
}

async function decide(a, approve) {
  appMsg.value = { ...appMsg.value, [a.id]: '' }
  if (approve && !confirm(`"${a.product}" 신청을 승인할까요? 비공개 링크가 열립니다.`)) return
  try {
    await apiFetch(`/api/promo/admin/applications/${a.id}/decision`, {
      method: 'POST',
      body: JSON.stringify({ approve, reason: approve ? '' : (reasons.value[a.id] || '') }),
    })
    toast(approve ? '승인했습니다' : '반려했습니다')
    await reload()
  } catch (e) {
    appMsg.value = { ...appMsg.value, [a.id]: e.message || '실패했습니다' }
  }
}

async function confirm(a) {
  appMsg.value = { ...appMsg.value, [a.id]: '' }
  try {
    await apiFetch(`/api/promo/admin/applications/${a.id}/confirm`, {
      method: 'POST',
      body: JSON.stringify({ orders: counts.value[a.id] || 0 }),
    })
    toast('저장했습니다')
    await reload()
  } catch (e) {
    appMsg.value = { ...appMsg.value, [a.id]: e.message || '실패했습니다' }
  }
}
</script>
