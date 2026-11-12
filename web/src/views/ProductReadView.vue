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
        <p v-if="!products.length" class="muted">등록된 상품이 없습니다.</p>
      </div>

      <div class="card grow2">
        <template v-if="detail">
          <h3>#{{ detail.product.id }} {{ detail.product.name || '(판독 전)' }}</h3>
          <p class="muted small">출구 {{ detail.product.exitType }} · <a :href="detail.product.sourceUrl" target="_blank" rel="noreferrer">원본 링크</a></p>
          <div v-if="detail.images && detail.images.length" class="thumbs">
            <img v-for="im in detail.images" :key="im.id" :src="'/uploads/' + im.path" :alt="'상품 이미지 ' + im.id" />
          </div>
          <template v-if="detail.reading">
            <dl class="reading">
              <div><dt>상품명</dt><dd>{{ detail.reading.productName || '—' }}</dd></div>
              <div><dt>가격 요약</dt><dd>{{ detail.reading.priceSummary || '—' }}</dd></div>
              <div><dt>옵션</dt><dd>{{ detail.reading.optionsText || '—' }}</dd></div>
              <div><dt>배송</dt><dd>{{ detail.reading.shippingText || '—' }}</dd></div>
              <div><dt>상세 핵심</dt><dd>{{ detail.reading.detailCore || '—' }}</dd></div>
              <div><dt>리뷰 요약</dt><dd>{{ detail.reading.reviewSummary || '—' }}</dd></div>
              <div><dt>이미지 설명</dt><dd>{{ detail.reading.imageDesc || '—' }}</dd></div>
              <div class="unknowns"><dt>미확인(unknowns)</dt><dd>{{ detail.reading.unknowns || '—' }}</dd></div>
            </dl>
          </template>
          <p v-else class="muted">아직 판독 결과가 없습니다. 작업 큐에서 analyze 작업 상태를 확인하세요.</p>
          <h4>작업 이력</h4>
          <ul v-if="detail.jobs && detail.jobs.length" class="jobs">
            <li v-for="j in detail.jobs" :key="j.id">
              #{{ j.id }} {{ j.type }} · {{ j.status }} · {{ j.workerName || '—' }}
              <span v-if="j.error" class="error">· {{ j.error }}</span>
            </li>
          </ul>
          <p v-else class="muted">작업 이력이 없습니다.</p>
        </template>
        <p v-else class="muted">왼쪽에서 상품을 선택하세요.</p>
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

async function load() {
  const data = await apiFetch('/api/products')
  products.value = data.products || []
}

async function select(id) {
  selected.value = id
  detail.value = await apiFetch(`/api/products/${id}`)
}

onMounted(load)
</script>
