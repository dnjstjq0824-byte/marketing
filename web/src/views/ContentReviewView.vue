<template>
  <section class="page">
    <h2>콘텐츠 검수</h2>
    <p class="muted">사람의 검수 동작만 상태를 바꿉니다. 작업자는 초안을 만들 뿐입니다.</p>

    <div class="toolbar">
      <label>상태
        <select v-model="filter" @change="load">
          <option value="">전체</option>
          <option value="draft">draft</option>
          <option value="review">review</option>
          <option value="approved">approved</option>
          <option value="queued">queued</option>
          <option value="published">published</option>
        </select>
      </label>
      <button @click="load">새로고침</button>
    </div>
    <p v-if="error" class="error">{{ error }}</p>

    <div class="row">
      <div class="card grow">
        <h3>목록</h3>
        <ul class="picklist">
          <li v-for="c in items" :key="c.id">
            <button :class="{ active: selected === c.id }" @click="select(c.id)">
              #{{ c.id }} [{{ c.channel }}] {{ c.title || '(제목 없음)' }} · {{ c.status }}
              <span v-if="c.held" class="badge">보류</span>
            </button>
          </li>
        </ul>
        <p v-if="!items.length" class="muted">해당 상태의 콘텐츠가 없습니다.</p>
      </div>

      <div v-if="detail" class="card grow2">
        <h3>#{{ detail.id }} {{ detail.title || '(제목 없음)' }}</h3>
        <p class="muted small">채널 {{ detail.channel }} · 상태 {{ detail.status }} · 상품 #{{ detail.productId }}</p>

        <div v-if="detail.disclosure" class="disclosure">
          <strong>대가 표시 (자동 삽입 · 삭제 불가)</strong>
          <p>{{ detail.disclosure }}</p>
        </div>
        <div v-if="detail.aiLabel" class="ailabel">
          <strong>AI 가상인물 표시</strong>
          <p>{{ detail.aiLabel }}</p>
        </div>

        <label>제목
          <input v-model="edit.title" />
        </label>
        <label>본문
          <textarea v-model="edit.body" rows="10"></textarea>
        </label>
        <label class="check">
          <input v-model="edit.persona" type="checkbox" />
          AI 가상인물이 추천·보증 주체로 등장 (체크 시에만 AI 표시 부착)
        </label>
        <div class="btnrow">
          <button @click="save">본문 저장</button>
        </div>

        <h4>상태 전이</h4>
        <div class="btnrow">
          <button v-for="to in nextStates(detail.status)" :key="to" @click="move(to)">
            → {{ to }}
          </button>
          <span v-if="detail.status === 'queued'" class="muted small">발행(published)은 트랜치 2에서 열립니다.</span>
        </div>
        <p v-if="msg" class="ok">{{ msg }}</p>
      </div>
      <div v-else class="card grow2">
        <p class="muted">왼쪽에서 콘텐츠를 선택하세요.</p>
      </div>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiFetch } from '../stores/auth.js'

const filter = ref('')
const items = ref([])
const selected = ref(null)
const detail = ref(null)
const edit = ref({ title: '', body: '', persona: false })
const error = ref('')
const msg = ref('')

const forward = { draft: 'review', review: 'approved', approved: 'queued', queued: 'published' }
const backward = { review: 'draft', approved: 'review', queued: 'approved' }

function nextStates(status) {
  const out = []
  if (backward[status]) out.push(backward[status])
  if (forward[status] && forward[status] !== 'published') out.push(forward[status])
  return out
}

async function load() {
  error.value = ''
  const qs = filter.value ? `?status=${filter.value}` : ''
  const data = await apiFetch(`/api/contents${qs}`)
  items.value = data.contents || []
}

async function select(id) {
  selected.value = id
  msg.value = ''
  detail.value = await apiFetch(`/api/contents/${id}`)
  edit.value = { title: detail.value.title || '', body: detail.value.body || '', persona: !!detail.value.isVirtualPersona }
}

async function save() {
  error.value = ''
  msg.value = ''
  try {
    detail.value = await apiFetch(`/api/contents/${selected.value}`, {
      method: 'PATCH',
      body: JSON.stringify({ title: edit.value.title, body: edit.value.body, virtualPersona: edit.value.persona }),
    })
    msg.value = '저장했습니다. 대가 표시는 그대로 유지됩니다.'
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function move(to) {
  error.value = ''
  msg.value = ''
  try {
    detail.value = await apiFetch(`/api/contents/${selected.value}`, {
      method: 'PATCH',
      body: JSON.stringify({ status: to }),
    })
    msg.value = `상태 → ${to}`
    await load()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)
</script>
