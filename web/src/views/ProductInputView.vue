<template>
  <section class="page">
    <h2>상품 투입</h2>
    <p class="muted">링크+이미지로 상품을 등록하면 상품 판독(analyze) 작업이 자동 생성됩니다.</p>

    <form class="card" @submit.prevent="submit">
      <label>상품 링크 (http/https)
        <input v-model="form.sourceUrl" placeholder="https://…" required />
      </label>
      <fieldset>
        <legend>출구 유형</legend>
        <label><input v-model="form.exitType" type="radio" value="A" /> A · 파트너 몰 링크</label>
        <label><input v-model="form.exitType" type="radio" value="B" /> B · 제휴 링크</label>
        <p class="muted small">기본값은 A입니다. 자체·파트너 상품이 우선입니다.</p>
      </fieldset>
      <label>우선순위 (숫자, 클수록 먼저)
        <input v-model.number="form.priority" type="number" min="0" />
      </label>
      <label>이미지 (최대 10장, 장당 10MB)
        <input ref="fileInput" type="file" accept="image/*" multiple @change="onFiles" />
      </label>
      <p v-if="error" class="error">{{ error }}</p>
      <p v-if="done" class="ok">{{ done }}</p>
      <button type="submit" :disabled="busy">{{ busy ? '등록 중…' : '상품 등록' }}</button>
    </form>

    <div class="card">
      <h3>등록된 상품</h3>
      <p v-if="loading" class="muted">불러오는 중…</p>
      <table v-else-if="products.length">
        <thead><tr><th>ID</th><th>이름</th><th>링크</th><th>출구</th><th>등록</th></tr></thead>
        <tbody>
          <tr v-for="p in products" :key="p.id">
            <td>{{ p.id }}</td>
            <td>{{ p.name || '(판독 전)' }}</td>
            <td class="ellipsis"><a :href="p.sourceUrl" target="_blank" rel="noreferrer">{{ p.sourceUrl }}</a></td>
            <td>{{ p.exitType === 'B' ? 'B · 제휴' : 'A · 파트너' }}</td>
            <td>{{ shortDate(p.createdAt) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-else class="muted">아직 등록된 상품이 없습니다.</p>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiFetch } from '../stores/auth.js'

const form = ref({ sourceUrl: '', exitType: 'A', priority: 0 })
const files = ref([])
const fileInput = ref(null)
const error = ref('')
const done = ref('')
const busy = ref(false)
const loading = ref(true)
const products = ref([])

function onFiles(e) {
  files.value = Array.from(e.target.files || [])
}

function shortDate(iso) {
  return (iso || '').slice(0, 16).replace('T', ' ')
}

async function load() {
  loading.value = true
  try {
    const data = await apiFetch('/api/products')
    products.value = data.products || []
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
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
    for (const f of files.value) body.append('images', f)
    const data = await apiFetch('/api/products', { method: 'POST', body })
    done.value = `상품 #${data.product.id} 등록됨 · 판독 작업 #${data.analyzeJob.id} 자동 생성`
    form.value = { sourceUrl: '', exitType: 'A', priority: 0 }
    files.value = []
    if (fileInput.value) fileInput.value.value = ''
    await load()
  } catch (e) {
    error.value = e.message || '등록 실패'
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>
