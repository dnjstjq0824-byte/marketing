<template>
  <div class="arow">
    <div class="ava" :class="ai ? 'ai' : 'ch'" aria-hidden="true">{{ initial }}</div>
    <div class="info">
      <div class="who">
        <strong>{{ account.name }}</strong>
        <StatusChip :kind="stateKind">{{ stateText }}</StatusChip>
        <RouterLink v-if="team" :to="'/teams/' + team.id" class="teamchip">{{ team.name }}</RouterLink>
        <span v-else class="muted micro">무소속</span>
        <span v-if="team && team.leader" class="leader">팀장</span>
        <span v-if="held > 0" class="held-pill">보류 {{ held }}건</span>
      </div>
      <p v-if="metaLine" class="muted small">{{ metaLine }}</p>
      <p v-if="ai && aiMetaLine" class="muted micro">{{ aiMetaLine }}</p>
    </div>
    <div class="btns">
      <button v-if="ai" class="ghost small" :disabled="account.sessionState !== 'connected'" @click="$emit('chat')">대화</button>
      <button v-if="account.sessionState !== 'connected'" class="small" @click="$emit('login')">로그인</button>
      <button v-if="!ai" class="ghost small" @click="$emit('mode')">{{ account.postingMode === 'auto' ? '자동' : '수동' }}</button>
      <div class="menuwrap">
        <button class="ghost small" @click="menu = !menu" aria-label="더보기">⋯</button>
        <div v-if="menu" class="menu" role="menu">
          <button v-if="account.sessionState === 'connected'" role="menuitem" @click="act('login')">다시 로그인</button>
          <button v-if="ai && account.sessionState === 'connected'" role="menuitem" @click="act('probe')">대화 연결 테스트</button>
          <button v-if="ai" role="menuitem" @click="act('plan')">요금제·기능</button>
          <button v-if="!ai" role="menuitem" @click="act('backend')">실행 방식</button>
          <button role="menuitem" class="danger-item" @click="act('unregister')">연결 해제</button>
        </div>
      </div>
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

const initial = computed(() => (props.account.platform || props.account.name || '?').slice(0, 1).toUpperCase())

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
  if (props.account.lastJobAt) parts.push(`최근 ${props.account.lastJobAt.slice(5, 16).replace('T', ' ')}`)
  return parts.join(' · ')
})

const CAP_LABEL = { text: '텍스트', image: '이미지', video: '영상', file: '파일' }
const capsText = computed(() => {
  try {
    const arr = JSON.parse(props.account.capabilities || '[]')
    if (!Array.isArray(arr) || !arr.length) return ''
    return arr.map((c) => CAP_LABEL[c] || c).join(' · ')
  } catch {
    return ''
  }
})

const aiMetaLine = computed(() => {
  const parts = []
  if (props.account.planTier) parts.push(props.account.planTier)
  if (capsText.value) parts.push(capsText.value)
  if (props.account.chatStatus === 'ok') parts.push('대화 가능')
  else if (props.account.chatStatus === 'unlinked') parts.push('대화 미연결')
  return parts.join(' · ')
})

function act(which) {
  menu.value = false
  emit(which)
}
</script>

<style scoped>
.arow {
  border-top: 1px solid var(--line-soft);
  padding: 12px 0;
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.ava {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 800;
  font-size: 14px;
  flex-shrink: 0;
}
.ava.ai { background: var(--accent-soft); color: var(--accent-dark); }
.ava.ch { background: var(--info-soft); color: var(--info); }
.info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.who { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; min-width: 0; }
.who strong { font-size: 14px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.teamchip {
  font-size: 11.5px;
  font-weight: 700;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--soft);
  color: var(--ink-soft);
  text-decoration: none;
}
.teamchip:hover { background: var(--accent-soft); color: var(--accent-dark); }
.leader {
  font-size: 11px;
  font-weight: 700;
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--gold-soft);
  color: var(--gold);
}
.held-pill {
  font-size: 11.5px;
  font-weight: 700;
  color: var(--hold);
}
.btns { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
.menuwrap { position: relative; }
.menu {
  position: absolute;
  right: 0;
  top: calc(100% + 4px);
  min-width: 156px;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  box-shadow: var(--shadow-lg);
  padding: 6px;
  z-index: 40;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.menu button {
  background: transparent;
  border: none;
  color: var(--ink);
  justify-content: flex-start;
  text-align: left;
  padding: 8px 10px;
  font-size: 13px;
  font-weight: 500;
  border-radius: 7px;
}
.menu button:hover { background: var(--soft); }
.menu .danger-item { color: var(--warn); }
@media (max-width: 680px) {
  .arow { flex-wrap: wrap; align-items: flex-start; }
  .btns { width: 100%; justify-content: flex-end; margin-top: 4px; }
}
</style>
