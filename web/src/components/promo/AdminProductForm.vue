<template>
  <div class="pu-card formcard">
    <h2 class="pu-sec-title">{{ initial.id ? '제품 수정' : '제품 등록' }}</h2>
    <div class="pu-form pu-formgrid">
      <label>제품명 <input v-model="f.name" /></label>
      <label>스토어 <input v-model="f.store" /></label>
      <label>판매가(원) <input v-model.number="f.price" type="number" min="0" /></label>
      <label>사진 주소(URL) <input v-model="f.photo" /></label>
      <label>카테고리 <select v-model="f.category"><option value="">없음</option><option v-for="c in CATS" :key="c" :value="c">{{ c }}</option></select></label>
      <label>모집 마감일 <input v-model="f.recruitUntil" type="date" /></label>
      <label>주문 건당 수수료(원) <input v-model.number="f.feePerOrder" type="number" min="0" /></label>
      <label>수수료·정산 조건 <input v-model="f.settleTerms" /></label>
      <label class="full">스마트스토어 비공개 링크 <input v-model="f.privateUrl" placeholder="수정 시 비워두면 기존 유지" /></label>
      <label class="full">사실 정보 <textarea v-model="f.facts"></textarea></label>
      <label class="full">홍보 포인트 <textarea v-model="f.points"></textarea></label>
      <label class="full">하면 안 되는 표현 <textarea v-model="f.bannedWords"></textarea></label>
      <label class="full">제공 자료 안내 <textarea v-model="f.materials"></textarea></label>
    </div>
    <p v-if="msg" class="pu-error">{{ msg }}</p>
    <p class="pu-rowline"><button class="pu-btn pu-btn-primary small" @click="submit">저장</button><button class="pu-btn small" @click="$emit('cancel')">취소</button></p>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { apiFetch } from '../../stores/auth.js'
import { CATEGORIES as CATS } from '../../stores/member.js'
import { toast } from '../../stores/toast.js'

const props = defineProps({ initial: { type: Object, default: () => ({}) } })
const emit = defineEmits(['saved', 'cancel'])

const f = ref({ name: '', store: '', price: 0, photo: '', category: '', recruitUntil: '', feePerOrder: 0, settleTerms: '', privateUrl: '', facts: '', points: '', bannedWords: '', materials: '', ...props.initial })
const msg = ref('')

async function submit() {
  msg.value = ''
  try {
    if (props.initial.id) await apiFetch(`/api/promo/admin/products/${props.initial.id}`, { method: 'PATCH', body: JSON.stringify(f.value) })
    else await apiFetch('/api/promo/admin/products', { method: 'POST', body: JSON.stringify(f.value) })
    toast('저장했습니다')
    emit('saved')
  } catch (e) {
    msg.value = e.message || '실패했습니다'
  }
}
</script>

<style scoped>
.formcard { margin-top: 16px; display: flex; flex-direction: column; gap: 14px; }
.formcard textarea { min-height: 64px; }
</style>
