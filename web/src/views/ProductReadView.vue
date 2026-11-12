<template>
  <section class="page">
    <h2>상품 판독</h2>
    <p class="muted">작업자가 저장한 판독 결과(8개 문자열 필드)를 상품별로 확인합니다. 확인되지 않은 내용은 unknowns에 분리 표기됩니다.</p>

    <div class="row">
      <div class="card grow">
        <h3>상품 선택</h3>
        <ul class="picklist">
          <li v-for="p in products" :key="p.id">
            <button :class="{ active: selected === p.id }" @click="select(p.id)">
              #{{ p.id }} {{ p.name || p.sourceUrl }}
            </button>
          </li>
        </ul>
        <p v-if="!products.length" class="empty">등록된 상품이 없습니다.</p>
      </div>

      <div class="card grow2">
        <template v-if="detail">
          <h3>#{{ detail.product.id }} {{ detail.product.name || '(판독 전)' }}</h3>
          <p class="muted small">사업 {{ detail.product.business || '(미지정)' }} · 출구 {{ detail.product.exitType }} · <a :href="detail.product.sourceUrl" target="_blank" rel="noreferrer">원본 링크</a></p>
          <h4>상품 수정</h4>
          <div class="form-grid">
            <label>이름 <input v-model="prodForm.name" /></label>
            <label>사업 <input v-model="prodForm.business" /></label>
            <label>링크 <input v-model="prodForm.sourceUrl" /></label>
            <label>출구 유형
              <select v-model="prodForm.exitType">
                <option value="A">A · 파트너</option>
                <option value="B">B · 제휴</option>
              </select>
            </label>
            <label>우선순위 <input v-model.number="prodForm.priority" type="number" min="0" /></label>
            <div class="full"><button @click="saveProduct">상품 저장</button></div>
          </div>
          <p v-if="prodError" class="error">{{ prodError }}</p>
          <div v-if="detail.images && detail.images.length" class="thumbs">
            <figure v-for="im in detail.images" :key="im.id">
              <img :src="'/uploads/' + im.path" :alt="'상품 이미지 ' + im.id" />
              <figcaption><button class="ghost" @click="deleteImage(im.id)">삭제</button></figcaption>
            </figure>
          </div>
          <template v-if="detail.reading">
            <div class="sect-head"><h4>판독 결과</h4><button v-if="!editingReading" class="ghost" @click="startReadingEdit">판독 수정</button></div>
            <dl v-if="!editingReading" class="reading">
              <div><dt>상품명</dt><dd>{{ detail.reading.productName || '—' }}</dd></div>
              <div><dt>가격 요약</dt><dd>{{ detail.reading.priceSummary || '—' }}</dd></div>
              <div><dt>옵션</dt><dd>{{ detail.reading.optionsText || '—' }}</dd></div>
              <div><dt>배송</dt><dd>{{ detail.reading.shippingText || '—' }}</dd></div>
              <div><dt>상세 핵심</dt><dd>{{ detail.reading.detailCore || '—' }}</dd></div>
              <div><dt>리뷰 요약</dt><dd>{{ detail.reading.reviewSummary || '—' }}</dd></div>
              <div><dt>이미지 설명</dt><dd>{{ detail.reading.imageDesc || '—' }}</dd></div>
              <div class="unknowns"><dt>미확인(unknowns)</dt><dd>{{ detail.reading.unknowns || '—' }}</dd></div>
            </dl>
            <div v-else class="form-grid">
              <label>상품명 <input v-model="readForm.productName" /></label>
              <label>가격 요약 <input v-model="readForm.priceSummary" /></label>
              <label>옵션 <input v-model="readForm.optionsText" /></label>
              <label>배송 <input v-model="readForm.shippingText" /></label>
              <label class="full">상세 핵심 <input v-model="readForm.detailCore" /></label>
              <label class="full">리뷰 요약 <input v-model="readForm.reviewSummary" /></label>
              <label class="full">이미지 설명 <input v-model="readForm.imageDesc" /></label>
              <label class="full">미확인(unknowns) <input v-model="readForm.unknowns" /></label>
              <div class="full btnrow">
                <button @click="saveReading">판독 저장</button>
                <button class="ghost" @click="editingReading = false">취소</button>
              </div>
            </div>
            <p v-if="readError" class="error">{{ readError }}</p>
          </template>
          <p v-else class="muted">아직 판독 결과가 없습니다. 작업 큐에서 analyze 작업 상태를 확인하세요.</p>
          <h4>제휴 링크 (출구 B)</h4>
          <ul v-if="detail.affiliateLinks && detail.affiliateLinks.length" class="speclist">
            <li v-for="l in detail.affiliateLinks" :key="l.id">
              <template v-if="editingLink === l.id">
                <input v-model="linkEdit.partnerName" placeholder="제휴처명" />
                <input v-model="linkEdit.url" placeholder="https://…" />
                <button @click="saveLink(l.id)">저장</button>
                <button class="ghost" @click="editingLink = null">취소</button>
              </template>
              <template v-else>
                <span class="k">{{ l.partnerName || '제휴처' }}</span>
                <span class="ellipsis"><a :href="l.url" target="_blank" rel="noreferrer">{{ l.url }}</a></span>
                <button class="ghost" @click="startLinkEdit(l)">수정</button>
                <button class="ghost" @click="confirmDelLink = l.id">삭제</button>
              </template>
            </li>
          </ul>
          <p v-else class="empty">등록된 제휴 링크가 없습니다.</p>
          <div class="form-grid">
            <label>제휴처명 <input v-model="affForm.partnerName" placeholder="예: 쿠팡 파트너스" /></label>
            <label>링크 URL <input v-model="affForm.url" placeholder="https://…" /></label>
            <div class="full"><button @click="addAffiliate">링크 등록</button></div>
          </div>
          <p v-if="affError" class="error">{{ affError }}</p>
          <h4>작업 이력</h4>
          <ul v-if="detail.jobs && detail.jobs.length" class="speclist">
            <li v-for="j in detail.jobs" :key="j.id">
              <span class="k">#{{ j.id }} {{ j.type }}</span>
              <span><span :class="'pill p-' + j.status">{{ j.status }}</span> {{ j.workerName || '—' }}
                <span v-if="j.error" class="error">· {{ j.error }}</span></span>
            </li>
          </ul>
          <p v-else class="empty">작업 이력이 없습니다.</p>
        </template>
        <p v-else class="muted">왼쪽에서 상품을 선택하세요.</p>
      </div>
    </div>

    <div v-if="confirmDelLink !== null" class="modal-back" @click.self="confirmDelLink = null">
      <div class="modal">
        <h3>제휴 링크 삭제</h3>
        <p>이 링크만 지워집니다. 되돌릴 수 없습니다.</p>
        <div class="btnrow">
          <button @click="doDeleteLink">삭제</button>
          <button class="ghost" @click="confirmDelLink = null">취소</button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiFetch } from '../stores/auth.js'

const products = ref([])
const selected = ref(null)
const detail = ref(null)
const affForm = ref({ partnerName: '', url: '' })
const affError = ref('')
const confirmDelLink = ref(null)
const prodForm = ref({ name: '', business: '', sourceUrl: '', exitType: 'A', priority: 0 })
const prodError = ref('')
const editingReading = ref(false)
const readForm = ref({ productName: '', priceSummary: '', optionsText: '', shippingText: '', detailCore: '', reviewSummary: '', imageDesc: '', unknowns: '' })
const readError = ref('')
const editingLink = ref(null)
const linkEdit = ref({ partnerName: '', url: '' })

function syncProdForm() {
  const p = detail.value && detail.value.product
  prodForm.value = p
    ? { name: p.name || '', business: p.business || '', sourceUrl: p.sourceUrl || '', exitType: p.exitType || 'A', priority: p.priority || 0 }
    : { name: '', business: '', sourceUrl: '', exitType: 'A', priority: 0 }
}

async function saveProduct() {
  prodError.value = ''
  try {
    await apiFetch(`/api/products/${selected.value}`, { method: 'PATCH', body: JSON.stringify(prodForm.value) })
    detail.value = await apiFetch(`/api/products/${selected.value}`)
    syncProdForm()
    await load()
  } catch (e) {
    prodError.value = e.message
  }
}

function startReadingEdit() {
  const r = detail.value.reading || {}
  readForm.value = {
    productName: r.productName || '', priceSummary: r.priceSummary || '', optionsText: r.optionsText || '',
    shippingText: r.shippingText || '', detailCore: r.detailCore || '', reviewSummary: r.reviewSummary || '',
    imageDesc: r.imageDesc || '', unknowns: r.unknowns || '',
  }
  readError.value = ''
  editingReading.value = true
}

async function saveReading() {
  readError.value = ''
  try {
    await apiFetch(`/api/products/${selected.value}/reading`, { method: 'PATCH', body: JSON.stringify(readForm.value) })
    detail.value = await apiFetch(`/api/products/${selected.value}`)
    editingReading.value = false
  } catch (e) {
    readError.value = e.message
  }
}

async function deleteImage(imageId) {
  try {
    await apiFetch(`/api/products/${selected.value}/images/${imageId}`, { method: 'DELETE' })
    detail.value = await apiFetch(`/api/products/${selected.value}`)
  } catch (e) {
    prodError.value = e.message
  }
}

function startLinkEdit(l) {
  linkEdit.value = { partnerName: l.partnerName || '', url: l.url || '' }
  editingLink.value = l.id
}

async function saveLink(linkId) {
  affError.value = ''
  try {
    await apiFetch(`/api/products/${selected.value}/affiliate-links/${linkId}`, {
      method: 'PATCH',
      body: JSON.stringify({ partnerName: linkEdit.value.partnerName.trim(), url: linkEdit.value.url.trim() }),
    })
    editingLink.value = null
    detail.value = await apiFetch(`/api/products/${selected.value}`)
  } catch (e) {
    affError.value = e.message
  }
}

async function doDeleteLink() {
  const linkId = confirmDelLink.value
  confirmDelLink.value = null
  affError.value = ''
  try {
    await apiFetch(`/api/products/${selected.value}/affiliate-links/${linkId}`, { method: 'DELETE' })
    detail.value = await apiFetch(`/api/products/${selected.value}`)
  } catch (e) {
    affError.value = e.message
  }
}

async function load() {
  const data = await apiFetch('/api/products')
  products.value = data.products || []
}

async function select(id) {
  selected.value = id
  affError.value = ''
  prodError.value = ''
  readError.value = ''
  editingReading.value = false
  editingLink.value = null
  detail.value = await apiFetch(`/api/products/${id}`)
  syncProdForm()
}

async function addAffiliate() {
  affError.value = ''
  try {
    await apiFetch(`/api/products/${selected.value}/affiliate-links`, {
      method: 'POST',
      body: JSON.stringify({ partnerName: affForm.value.partnerName.trim(), url: affForm.value.url.trim() }),
    })
    affForm.value = { partnerName: '', url: '' }
    detail.value = await apiFetch(`/api/products/${selected.value}`)
  } catch (e) {
    affError.value = e.message
  }
}

onMounted(load)
</script>
