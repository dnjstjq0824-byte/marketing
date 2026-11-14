<template>
  <div class="row">
    <div class="who">
      <strong>{{ account.name }}</strong>
      <StatusChip :kind="stateKind">{{ stateText }}</StatusChip>
      <span v-if="held > 0" class="muted small">대기 {{ held }}건</span>
    </div>
    <p class="muted small">{{ metaLine }}</p>
    <div class="btns">
      <button v-if="ai" class="ghost small" @click="$emit('chat')">대화</button>
      <button v-if="account.sessionState !== 'connected'" class="small primary" @click="$emit('login')">로그인</button>
      <button v-if="!ai" class="ghost small" @click="$emit('mode')">{{ account.postingMode === 'auto' ? '수동으로' : '자동으로' }}</button>
      <button class="ghost small" @click="menu = !menu" aria-label="더보기">⋯</button>
    </div>
    <div v-if="menu" class="menu" role="menu">
      <button v-if="account.sessionState === 'connected'" role="menuitem" @click="act('login')">로그인 다시</button>
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
})
const emit = defineEmits(['chat', 'login', 'unregister', 'mode'])
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

const metaLine = computed(() => {
  const parts = []
  if (props.account.owner) parts.push(props.account.owner)
  if (props.account.role && props.ai) parts.push(props.account.role === 'chief' ? '총괄' : '작업자')
  if (props.account.lastJobAt) parts.push(`마지막 작업 ${props.account.lastJobAt.slice(5, 16).replace('T', ' ')}`)
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
.who { display: flex; align-items: center; gap: var(--s2); }
.who strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
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
