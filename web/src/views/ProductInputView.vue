<template>
  <section class="page">
    <h2>상품 투입</h2>
    <p class="muted">링크+이미지로 상품을 등록하면 상품 판독(analyze) 작업이 자동 생성됩니다.</p>

    <form class="card" @submit.prevent="submit">
      <h3>새 상품 등록</h3>
      <div class="form-grid">
        <label class="full">상품 링크 <span class="req">*</span>
          <input v-model="form.sourceUrl" placeholder="https://…" required />
          <span class="hint">http/https로 시작해야 합니다.</span>
        </label>
        <label>사업 <input v-model="form.business" placeholder="예: 글로우사인" /></label>
        <fieldset>
          <legend>출구 유형</legend>
          <label><input v-model="form.exitType" type="radio" value="A" /> A · 파트너 몰 링크</label>
          <label><input v-model="form.exitType" type="radio" value="B" /> B · 제휴 링크</label>
          <p class="muted small">기본값은 A입니다. 자체·파트너 상품이 우선입니다.</p>
        </fieldset>
        <div>
          <label>우선순위
            <input v-model.number="form.priority" type="number" min="0" />
            <span class="hint">숫자가 클수록 먼저 처리됩니다.</span>
          </label>
          <label>이미지
            <input ref="fileInput" type="file" accept="image/*" multiple @change="onFiles" />
            <span class="hint">최대 10장 · 장당 10MB · jpg/png/gif/webp</span>
          </label>
        </div>
      </div>
      <p v-if="error" class="banner err">{{ error }}</p>
      <p v-if="done" class="banner okk">{{ done }}</p>
      <button type="submit" :disabled="busy">{{ busy ? '등록 중…' : '상품 등록' }}</button>
    </form>

    <div class="card">
      <div class="sect-head"><h3>등록된 상품</h3><span class="count">{{ products.length }}건</span></div>
      <div class="toolbar">
        <label>사업 필터
          <select v-model="bizFilter" @change="load">
            <option value="">전체 사업</option>
            <option v-for="b in bizOptions" :key="b" :value="b">{{ b || '(미지정)' }}</option>
          </select>
        </label>
      </div>
      <p v-if="loading" class="muted"><span class="skel">불러오는 중입니다</span></p>
      <div v-else-if="products.length" class="table-wrap">
        <table>
          <thead><tr><th class="id">ID</th><th>사업</th><th>이름</th><th>링크</th><th>출구</th><th>등록</th><th></th></tr></thead>
          <tbody>
            <tr v-for="p in products" :key="p.id">
              <td class="id">{{ p.id }}</td>
              <td>{{ p.business || '(미지정)' }}</td>
              <td>{{ p.name || '(판독 전)' }}</td>
              <td class="ellipsis"><a :href="p.sourceUrl" target="_blank" rel="noreferrer">{{ p.sourceUrl }}</a></td>
              <td><span :class="p.exitType === 'B' ? 'pill p-review' : 'pill p-approved'">{{ exitLabel(p.exitType) }}</span></td>
              <td>{{ shortDate(p.createdAt) }}</td>
              <td><button class="ghost small" @click="confirmDel = p.id">삭제</button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyArt v-else art="plant" title="아직 등록된 상품이 없습니다" hint="위 폼에서 첫 상품을 등록하세요." />
    </div>

    <div v-if="confirmDel !== null" class="modal-back" @click.self="confirmDel = null">
      <div class="modal">
        <h3>상품 #{{ confirmDel }} 삭제</h3>
        <p>판독 결과·이미지·콘텐츠·작업·제휴 링크가 함께 지워집니다. 되돌릴 수 없습니다.</p>
        <div class="btnrow">
          <button class="danger" @click="doDelete">삭제</button>
          <button class="ghost" @click="confirmDel = null">취소</button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiFetch } from '../stores/auth.js'
import { EXIT_TYPE, label } from '../stores/labels.js'
import EmptyArt from '../components/EmptyArt.vue'

function exitLabel(e) {
  return label(EXIT_TYPE, e)
}

const form = ref({ sourceUrl: '', exitType: 'A', priority: 0, business: '' })
const files = ref([])
const fileInput = ref(null)
const error = ref('')
const done = ref('')
const busy = ref(false)
const loading = ref(true)
const products = ref([])
const confirmDel = ref(null)
const bizFilter = ref('')
const bizOptions = ref([])

function onFiles(e) {
  files.value = Array.from(e.target.files || [])
}

function shortDate(iso) {
  return (iso || '').slice(0, 16).replace('T', ' ')
}

async function load() {
  loading.value = true
  try {
    const qs = bizFilter.value ? `?business=${encodeURIComponent(bizFilter.value)}` : ''
    const data = await apiFetch(`/api/products${qs}`)
    products.value = data.products || []
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function loadBizOptions() {
  try {
    const data = await apiFetch('/api/products')
    const set = new Set((data.products || []).map((p) => p.business || ''))
    bizOptions.value = [...set].sort()
  } catch {
    /* options stay as-is */
  }
}

async function doDelete() {
  const id = confirmDel.value
  confirmDel.value = null
  error.value = ''
  try {
    await apiFetch(`/api/products/${id}`, { method: 'DELETE' })
    await load()
    await loadBizOptions()
  } catch (e) {
    error.value = e.message
  }
}

async function submit() {
  busy.value = true
  error.value = ''
  done.value = ''
  try {
    const body = new FormData()
    body.append('sourceUrl', form.value.sourceUrl.trim())
    body.append('exitType', form.value.exitType)
    body.append('priority', String(form.value.priority || 0))
    body.append('business', (form.value.business || '').trim())
    for (const f of files.value) body.append('images', f)
    const data = await apiFetch('/api/products', { method: 'POST', body })
    done.value = `상품 #${data.product.id} 등록됨 · 판독 작업 #${data.analyzeJob.id} 자동 생성`
    form.value = { sourceUrl: '', exitType: 'A', priority: 0, business: '' }
    files.value = []
    if (fileInput.value) fileInput.value.value = ''
    await load()
    await loadBizOptions()
  } catch (e) {
    error.value = e.message || '등록 실패'
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
  await load()
  await loadBizOptions()
})
</script>
