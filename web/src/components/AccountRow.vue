<template>
  <div class="row">
    <div class="who">
      <strong>{{ account.name }}</strong>
      <StatusChip :kind="stateKind">{{ stateText }}</StatusChip>
      <RouterLink v-if="team" :to="'/teams/' + team.id" class="teamchip">{{ team.name }}</RouterLink>
      <span v-else class="muted small">무소속</span>
      <span v-if="team && team.leader" class="leader">팀장</span>
      <span v-if="held > 0" class="muted small">대기 {{ held }}건</span>
    </div>
    <p class="muted small">{{ metaLine }}</p>
    <p v-if="ai && (account.planTier || capsText || account.chatStatus)" class="muted small">{{ aiMetaLine }}</p>
    <div class="btns">
      <button v-if="ai" class="ghost small" @click="$emit('chat')">대화</button>
      <button v-if="account.sessionState !== 'connected'" class="small primary" @click="$emit('login')">로그인</button>
      <button v-if="!ai" class="ghost small" @click="$emit('mode')">{{ account.postingMode === 'auto' ? '수동으로' : '자동으로' }}</button>
      <button class="ghost small" @click="menu = !menu" aria-label="더보기">⋯</button>
    </div>
    <div v-if="menu" class="menu" role="menu">
      <button v-if="account.sessionState === 'connected'" role="menuitem" @click="act('login')">로그인 다시</button>
      <button v-if="ai && account.sessionState === 'connected'" role="menuitem" @click="act('probe')">대화 연결 테스트</button>
      <button v-if="ai" role="menuitem" @click="act('plan')">요금제·기능</button>
      <button v-if="!ai" role="menuitem" @click="act('backend')">실행 방식</button>
      <button role="menuitem" class="danger" @click="act('unregister')">해제</button>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import StatusChip from './StatusChip.vue'
import { ACCOUNT_STATE } from '../stores/labels.js'

const props = defineProps({
  account: { type: Object, required: true },
  ai: { type: Boolean, default: false },
  held: { type: Number, default: 0 },
  team: { type: Object, default: null },
})
const emit = defineEmits(['chat', 'login', 'unregister', 'mode', 'probe', 'plan', 'backend'])
const menu = ref(false)

const stateKey = computed(() => {
  switch (props.account.sessionState) {
    case 'connected': return 'connected'
    case 'limited': return 'limited'
    case 'inactive_hold': return 'off'
    default: return 'login_required'
  }
})

const stateKind = computed(() => {
  switch (stateKey.value) {
    case 'connected': return 'ok'
    case 'limited': return 'hold'
    case 'off': return 'neutral'
    default: return 'warn'
  }
})

const stateText = computed(() => ACCOUNT_STATE[stateKey.value] || props.account.sessionState)

const backendText = computed(() => {
  const be = props.account.executorBackend || 'direct'
  if (be === 'direct') return '직접 발행'
  const relay = props.account.relayAccount ? `(${props.account.relayAccount})` : ''
  return `${be} 경유${relay}`
})

const metaLine = computed(() => {
  const parts = []
  if (props.account.owner) parts.push(props.account.owner)
  if (props.account.role && props.ai) parts.push(props.account.role === 'chief' ? '총괄' : '작업자')
  if (!props.ai) parts.push(backendText.value)
  if (props.account.lastJobAt) parts.push(`마지막 작업 ${props.account.lastJobAt.slice(5, 16).replace('T', ' ')}`)
  return parts.join(' · ')
})

const CAP_LABEL = { text: '텍스트', image: '이미지', video: '영상', file: '파일' }

const capsText = computed(() => {
  try {
    const arr = JSON.parse(props.account.capabilities || '[]')
    if (!Array.isArray(arr) || !arr.length) return ''
    return arr.map((c) => CAP_LABEL[c] || c).join('·')
  } catch {
    return ''
  }
})

const aiMetaLine = computed(() => {
  const parts = []
  if (props.account.planTier) parts.push(props.account.planTier)
  if (capsText.value) parts.push(capsText.value)
  if (props.account.chatStatus === 'ok') parts.push('대화 연결됨')
  else if (props.account.chatStatus === 'unlinked') parts.push('대화 미연결')
  return parts.join(' · ')
})

function act(which) {
  menu.value = false
  emit(which)
}
</script>

<style scoped>
.row {
  border-top: 1px solid var(--line-soft);
  padding: var(--s3) 0;
  display: flex;
  flex-direction: column;
  gap: var(--s1);
  position: relative;
}
.who { display: flex; align-items: center; gap: var(--s2); flex-wrap: wrap; }
.who strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.teamchip {
  font-size: 12px;
  font-weight: 700;
  background: var(--accent-soft);
  color: var(--accent-dark);
  border-radius: 999px;
  padding: 1px 10px;
  text-decoration: none;
  white-space: nowrap;
}
.teamchip:hover { text-decoration: underline; }
.leader { font-size: 12px; font-weight: 800; color: var(--hold); white-space: nowrap; }
.btns { display: flex; gap: var(--s2); }
.btns button, .ghost {
  border-radius: var(--r-md);
  padding: var(--s1) var(--s3);
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
  font-size: 13px;
}
.btns button.primary { background: var(--accent); border-color: var(--accent); color: #fff; font-weight: 700; }
.menu {
  position: absolute;
  right: 0;
  top: 100%;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  box-shadow: var(--shadow-lg);
  display: flex;
  flex-direction: column;
  z-index: 20;
  min-width: 140px;
}
.menu button { background: none; border: none; color: var(--ink); text-align: left; padding: var(--s2) var(--s3); }
.menu button.danger { color: var(--warn); }
</style>
