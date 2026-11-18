<template>
  <div class="pu-form">
    <label class="pu-check su-all"><input type="checkbox" :checked="all" @change="setAll($event.target.checked)" /> 전체 동의합니다</label>
    <div v-for="t in TERMS" :key="t.key" class="su-term">
      <label class="pu-check"><input type="checkbox" v-model="terms[t.key]" /> <span><em :class="t.required ? 'req' : 'opt'">{{ t.required ? '필수' : '선택' }}</em> {{ t.title }}</span></label>
      <button class="pu-btn small" @click="open = t.key">보기</button>
    </div>
    <TermsModal v-if="open" :title="current.title" :lines="current.lines" @close="open = ''" />
    <p class="pu-small pu-muted full">전문 보기: <RouterLink to="/promo/terms">이용약관</RouterLink> · <RouterLink to="/promo/privacy">개인정보처리방침</RouterLink></p>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import TermsModal from './TermsModal.vue'

const props = defineProps({ terms: { type: Object, required: true } })
const open = ref('')
const TERMS = [
  { key: 'service', required: true, title: '서비스 이용약관', lines: ['승인된 본인의 비공개 링크로만 홍보합니다.', '제품별 금지 표현과 광고 표기 규칙을 지킵니다.', '허위·과장 홍보 시 승인이 취소될 수 있습니다.'] },
  { key: 'privacy', required: true, title: '개인정보 처리방침', lines: ['수집 항목: 이메일, 닉네임, 신청·정산 기록.', '목적: 회원 식별, 신청 심사, 수수료 정산.', '보관: 탈퇴 후 30일 뒤 파기(정산 기록은 법령 기간 보관).'] },
  { key: 'marketing', required: false, title: '마케팅 수신 동의', lines: ['모집 알림과 혜택 소식을 이메일로 받습니다.', '내 홍보함에서 언제든 변경할 수 있습니다.'] },
]
const current = computed(() => TERMS.find((t) => t.key === open.value) || TERMS[0])
const all = computed(() => props.terms.service && props.terms.privacy && props.terms.marketing)
function setAll(v) { props.terms.service = v; props.terms.privacy = v; props.terms.marketing = v }
</script>

<style scoped>
.su-all { font-weight: 700 !important; border-bottom: 1px solid var(--pu-line); padding-bottom: 10px; }
.su-term { display: flex; align-items: center; gap: 8px; }
.su-term label { flex: 1; }
.su-term em { font-style: normal; font-size: 12px; font-weight: 800; margin-right: 4px; }
.su-term .req { color: var(--pu-accent-dark); }
.su-term .opt { color: var(--pu-muted); }
</style>
