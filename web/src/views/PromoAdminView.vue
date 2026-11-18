<template>
  <PublicShell wide>
    <div class="pu-admin">
      <aside class="pu-side" aria-label="위탁 관리 메뉴">
        <h2>위탁 관리</h2>
        <button :class="{ on: tab === 'dash' }" @click="tab = 'dash'">대시보드</button>
        <button :class="{ on: tab === 'products' }" @click="tab = 'products'">제품 관리</button>
        <button :class="{ on: tab === 'apps' }" @click="tab = 'apps'">신청 관리<span v-if="ov.pending"> ({{ ov.pending }})</span></button>
        <button :class="{ on: tab === 'invites' }" @click="tab = 'invites'">초대 코드</button>
        <div class="pu-back">
          <RouterLink to="/promo">← 공개 화면으로</RouterLink>
          <RouterLink to="/">운영 콘솔로 →</RouterLink>
        </div>
      </aside>
      <div class="pu-adminbody">
        <div v-if="tab === 'dash'">
          <h1 class="pu-h1">대시보드</h1>
          <p class="pu-lead">실시간 운영 현황입니다.</p>
          <AdminStats :ov="ov" />
          <h2 class="pu-sec-title">승인 대기 신청</h2>
          <AdminAppTable
            :key="'d' + seq"
            :rows="pendingTop"
            @approve="askApprove"
            @reject="askReject"
            @confirm="saveConfirm"
          />
        </div>
        <div v-if="tab === 'products'">
          <div class="pu-sec-head">
            <h1 class="pu-h1">제품 관리</h1>
            <span style="flex: 1"></span>
            <button class="pu-btn pu-btn-primary small" @click="openForm(null)">제품 등록</button>
          </div>
          <div class="toolbar">
            <input v-model="pt.q.value" class="search" placeholder="제품명·스토어로 검색" @input="pt.reset()" />
          </div>
          <AdminProductTable
            :rows="pt.rows.value"
            :sort-key="pt.sortKey.value"
            :sort-dir="pt.sortDir.value"
            @sort="pt.toggleSort"
            @edit="openForm"
            @status="askRecruit"
          />
          <AdminPager :page="pt.page.value" :pages="pt.pages.value" :total="pt.total.value" @goto="pt.setPage" />
          <AdminProductForm v-if="formOpen" :key="formKey" :initial="formInitial" @saved="onSaved" @cancel="formOpen = false" />
        </div>
        <div v-if="tab === 'apps'">
          <div class="pu-sec-head">
            <h1 class="pu-h1">신청 관리</h1>
            <span style="flex: 1"></span>
            <label class="pu-small">상태
              <select v-model="appFilter" class="search" @change="reload">
                <option value="">전체</option>
                <option value="pending">대기</option>
                <option value="approved">승인</option>
                <option value="rejected">반려</option>
              </select>
            </label>
          </div>
          <div class="toolbar">
            <input v-model="at.q.value" class="search" placeholder="제품·신청자로 검색" @input="at.reset()" />
          </div>
          <AdminAppTable
            :key="'a' + seq"
            :rows="at.rows.value"
            :sort-key="at.sortKey.value"
            :sort-dir="at.sortDir.value"
            @sort="at.toggleSort"
            @approve="askApprove"
            @reject="askReject"
            @confirm="saveConfirm"
          />
          <AdminPager :page="at.page.value" :pages="at.pages.value" :total="at.total.value" @goto="at.setPage" />
        </div>
        <div v-if="tab === 'invites'">
          <div class="pu-sec-head">
            <h1 class="pu-h1">초대 코드</h1>
            <span style="flex: 1"></span>
            <label class="pu-small">개수 <input v-model.number="mintCount" type="number" min="1" max="100" class="search narrow" /></label>
            <label class="pu-small">회당 <input v-model.number="mintUses" type="number" min="1" max="1000" class="search narrow" /></label>
            <button class="pu-btn pu-btn-primary small" :disabled="minting" @click="mint">{{ minting ? '발급 중…' : '발급' }}</button>
          </div>
          <p class="pu-small pu-muted">베타 기간 가입에 필요합니다. 중단된 코드는 즉시 막힙니다.</p>
          <AdminInviteTable :rows="invites" @revoke="askRevokeInvite" />
        </div>
      </div>
    </div>
    <ConfirmModal
      v-if="modal"
      :title="modal.title"
      :message="modal.message"
      :confirm-label="modal.ok"
      :danger="modal.danger"
      @confirm="doModal"
      @cancel="modal = null"
    />
  </PublicShell>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import PublicShell from '../components/PublicShell.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import AdminStats from '../components/promo/AdminStats.vue'
import AdminPager from '../components/promo/AdminPager.vue'
import AdminProductTable from '../components/promo/AdminProductTable.vue'
import AdminAppTable from '../components/promo/AdminAppTable.vue'
import AdminProductForm from '../components/promo/AdminProductForm.vue'
import AdminInviteTable from '../components/promo/AdminInviteTable.vue'
import { useTable } from '../lib/adminTable.js'
import { apiFetch } from '../stores/auth.js'
import { toast } from '../stores/toast.js'

const tab = ref('dash')
const ov = ref({})
const products = ref([])
const apps = ref([])
const appFilter = ref('')
const seq = ref(0)
const formOpen = ref(false)
const formKey = ref(0)
const formInitial = ref({})
const modal = ref(null)
const invites = ref([])
const mintCount = ref(5)
const mintUses = ref(1)
const minting = ref(false)

const pt = useTable(() => products.value, ['name', 'store', 'category'])
const at = useTable(() => apps.value, ['product', 'memberNickname', 'memberEmail'])
const pendingTop = computed(() => apps.value.filter((a) => a.status === 'pending').slice(0, 5))

async function reload() {
  const [o, ps, as, iv] = await Promise.all([
    apiFetch('/api/promo/admin/overview'),
    apiFetch('/api/promo/admin/products'),
    apiFetch('/api/promo/admin/applications' + (appFilter.value ? `?status=${appFilter.value}` : '')),
    apiFetch('/api/promo/admin/invites'),
  ])
  ov.value = o
  products.value = ps.products || []
  apps.value = as.applications || []
  invites.value = iv.invites || []
  seq.value++
}

async function mint() {
  minting.value = true
  try {
    await apiFetch('/api/promo/admin/invites', { method: 'POST', body: JSON.stringify({ count: mintCount.value, maxUses: mintUses.value }) })
    toast('초대 코드를 발급했습니다')
    await reload()
  } catch (e) {
    toast(e.message || '실패했습니다', 'error')
  } finally {
    minting.value = false
  }
}

function askRevokeInvite(r) {
  modal.value = { kind: 'revokeInvite', id: r.id, title: '초대 중단', message: `"${r.code}" 코드를 중단할까요?`, ok: '중단', danger: true }
}

onMounted(async () => {
  try {
    await reload()
  } catch (e) {
    toast(e.message || '실패했습니다', 'error')
  }
})

function openForm(p) {
  formInitial.value = p ? { ...p } : {}
  formOpen.value = true
  formKey.value++
}

async function onSaved() {
  formOpen.value = false
  await reload()
}

function askApprove(a) {
  modal.value = { kind: 'approve', id: a.id, title: '신청 승인', message: `"${a.product}" 신청을 승인할까요? 비공개 링크가 열립니다.`, ok: '승인' }
}

function askReject(a, reason) {
  if (!reason.trim()) {
    toast('반려 사유를 입력하세요', 'error')
    return
  }
  modal.value = { kind: 'reject', id: a.id, reason, title: '신청 반려', message: `"${a.product}" 신청을 반려할까요? 사유: ${reason}`, ok: '반려', danger: true }
}

function askRecruit(p, status) {
  if (status === 'recruiting') {
    saveRecruit(p.id, status)
    return
  }
  modal.value = { kind: 'recruit', id: p.id, status, title: '모집 마감', message: `"${p.name}" 모집을 마감할까요? 더 이상 신청을 받지 않습니다.`, ok: '마감', danger: true }
}

async function doModal() {
  const m = modal.value
  modal.value = null
  if (!m) return
  try {
    if (m.kind === 'approve' || m.kind === 'reject') {
      await apiFetch(`/api/promo/admin/applications/${m.id}/decision`, {
        method: 'POST',
        body: JSON.stringify({ approve: m.kind === 'approve', reason: m.reason || '' }),
      })
      toast(m.kind === 'approve' ? '승인했습니다' : '반려했습니다')
    } else if (m.kind === 'recruit') {
      await saveRecruit(m.id, m.status, true)
      return
    } else if (m.kind === 'revokeInvite') {
      await apiFetch(`/api/promo/admin/invites/${m.id}/revoke`, { method: 'POST' })
      toast('초대 코드를 중단했습니다')
    }
    await reload()
  } catch (e) {
    toast(e.message || '실패했습니다', 'error')
  }
}

async function saveRecruit(id, status, skipReload) {
  try {
    await apiFetch(`/api/promo/admin/products/${id}/recruit`, { method: 'POST', body: JSON.stringify({ status }) })
    toast('저장했습니다')
    if (!skipReload) await reload()
    else await reload()
  } catch (e) {
    toast(e.message || '실패했습니다', 'error')
  }
}

async function saveConfirm(a, orders) {
  try {
    await apiFetch(`/api/promo/admin/applications/${a.id}/confirm`, { method: 'POST', body: JSON.stringify({ orders }) })
    toast('저장했습니다')
    await reload()
  } catch (e) {
    toast(e.message || '실패했습니다', 'error')
  }
}
</script>

<style scoped>
.toolbar { margin-bottom: 12px; }
.search {
  border: 1px solid var(--pu-line);
  border-radius: var(--pu-r-sm);
  padding: 9px 12px;
  font-size: 14px;
  font-family: inherit;
  color: var(--pu-ink);
  background: var(--pu-card);
  width: 100%;
  max-width: 340px;
}
.pu-sec-head .search { width: auto; }
.search.narrow { width: 76px; max-width: none; }
.pu-sec-title { margin: 20px 0 12px; }
@media (max-width: 900px) {
  .search { max-width: none; }
  .pu-sec-head { flex-wrap: wrap; }
}
</style>
