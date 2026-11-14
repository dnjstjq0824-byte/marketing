<template>
  <div class="modal-back" @click.self="onBackdrop">
    <div class="modal" role="dialog" :aria-label="`${account.name} 로그인`">
      <div class="sect-head">
        <h3>{{ account.name }} 로그인</h3>
        <span class="pill">{{ platformLabel(account.platform) }}</span>
      </div>
      <p class="muted small">{{ loginHint(account.platform) }}</p>
      <ol class="steps">
        <li><span class="n" :class="{ on: phase === 'ask' }">1</span> 브라우저 열기</li>
        <li><span class="n" :class="{ on: phase === 'waiting' }">2</span> 뜬 창에서 직접 로그인</li>
        <li><span class="n" :class="{ on: phase === 'busy' || phase === 'done' }">3</span> 완료 확인</li>
      </ol>
      <label v-if="phase === 'ask' && needsGoogleLogin(account.platform)" class="check">
        <input v-model="realProfile" type="checkbox" /> 내 크롬으로 열기 (Google 로그인이 막힐 때만, 크롬 완전 종료 필요)
      </label>
      <p v-if="phase === 'ask'" class="muted small">브라우저 열기를 누르면 새 시크릿 창으로 열립니다. 브라우저가 안 뜨면: 파워셸에서 <code>adapter\.venv\Scripts\python -m adapter.login {{ account.name }} {{ account.platform }} --incognito</code> 실행 후 직접 로그인하고, 아래 바로 확인을 누르세요.</p>
      <p v-if="phase === 'waiting'" class="banner warn">브라우저가 열렸습니다. 로그인을 끝낸 뒤 아래 완료 확인을 누르세요.</p>
      <p v-if="phase === 'busy'" class="banner warn">확인 중입니다. 잠시 기다리세요.</p>
      <p v-if="msg" :class="ok ? 'banner okk' : 'error'">{{ msg }}</p>
      <div class="btnrow">
        <button v-if="phase === 'ask'" @click="start">브라우저 열기</button>
        <button v-if="phase === 'ask'" class="ghost" @click="finish">터미널 로그인 후 바로 확인</button>
        <button v-if="phase === 'waiting'" @click="finish">로그인 완료 확인</button>
        <button v-if="phase === 'done' && next" @click="$emit('next', next)">다음 계정 로그인 ({{ next.name }})</button>
        <button class="ghost" @click="close">{{ phase === 'waiting' || phase === 'busy' ? '취소' : '닫기' }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { apiFetch } from '../stores/auth.js'
import { GOOGLE_SSO_PLATFORMS, LOGIN_HINT, label, PLATFORM } from '../stores/labels.js'

const props = defineProps({
  account: { type: Object, required: true },
  next: { type: Object, default: null },
})
const emit = defineEmits(['done', 'close', 'next'])

const phase = ref('ask')
const msg = ref('')
const ok = ref(false)
const realProfile = ref(false)

function platformLabel(p) {
  return label(PLATFORM, p, p)
}

function loginHint(p) {
  return label(LOGIN_HINT, p, '열린 창에서 로그인하세요.')
}

function needsGoogleLogin(p) {
  return GOOGLE_SSO_PLATFORMS.includes(p)
}

async function start() {
  msg.value = ''
  try {
    await apiFetch(`/api/session/accounts/${props.account.id}/login-start`, {
      method: 'POST',
      body: JSON.stringify({ realProfile: !!realProfile.value, incognito: !realProfile.value }),
    })
    phase.value = 'waiting'
  } catch (e) {
    msg.value = e.message
    ok.value = false
  }
}

async function finish() {
  phase.value = 'busy'
  msg.value = ''
  try {
    const a = await apiFetch(`/api/session/accounts/${props.account.id}/login-finish`, { method: 'POST' })
    phase.value = 'done'
    msg.value = `${a.name} 연결됐습니다.`
    ok.value = true
    emit('done', a)
  } catch (e) {
    phase.value = 'waiting'
    msg.value = e.message
    ok.value = false
  }
}

async function close() {
  if (phase.value === 'waiting' || phase.value === 'busy') {
    try {
      await apiFetch(`/api/session/accounts/${props.account.id}/login-cancel`, { method: 'POST' })
    } catch { /* already gone */ }
  }
  emit('close')
}

function onBackdrop() {
  if (phase.value !== 'busy') close()
}
</script>

<style scoped>
.modal-back {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 150;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--s4);
}
.modal {
  background: var(--panel);
  border-radius: var(--r-lg);
  box-shadow: var(--shadow-lg);
  padding: var(--s5);
  max-width: 520px;
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: var(--s3);
}
.modal h3 { margin: 0; }
.sect-head { display: flex; align-items: center; gap: var(--s2); }
.pill { background: var(--soft); border-radius: 999px; padding: 0 10px; font-size: 12px; }
.steps { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--s1); }
.steps li { display: flex; align-items: center; gap: var(--s2); }
.n {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  border: 1px solid var(--line);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  color: var(--muted);
}
.n.on { background: var(--accent); border-color: var(--accent); color: #fff; }
.check { display: flex; align-items: center; gap: var(--s2); }
.check input { width: auto; }
code { background: var(--soft); padding: 0 4px; border-radius: 4px; font-size: 12px; }
.banner { border-radius: var(--r-md); padding: var(--s3); }
.banner.warn { background: var(--hold-soft); }
.banner.okk { background: var(--ok-soft); color: var(--ok); }
.error { color: var(--warn); }
.btnrow { display: flex; gap: var(--s2); flex-wrap: wrap; }
.btnrow button { border-radius: var(--r-md); padding: var(--s2) var(--s4); border: 1px solid var(--accent); background: var(--accent); color: #fff; }
.btnrow button.ghost { border: 1px solid var(--line); background: var(--panel); color: var(--ink); }
</style>
