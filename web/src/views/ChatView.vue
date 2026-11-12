<template>
  <section class="page">
    <h2>대화</h2>
    <p class="muted">에이전트 슬롯이 늘어도 의장님의 대화 상대는 총괄 1자리로 고정됩니다.</p>

    <div class="tabs">
      <button :class="{ active: peer === 'chief' }" @click="peer = 'chief'">총괄 (판정·기준·지표)</button>
      <button :class="{ active: peer === 'worker' }" @click="peer = 'worker'">작업자 (생산 지시)</button>
    </div>

    <!-- Re-login notice takes precedence over the conversation when expired. -->
    <div v-if="session === 'expired'" class="card">
      <div class="sect-head"><h3>세션이 만료됐습니다</h3><span class="pill p-expired">만료</span></div>
      <p>대화보다 재로그인이 먼저입니다. 아래 버튼으로 다시 로그인하면 대화가 활성화됩니다.</p>
      <button @click="openModal">Claude 로그인</button>
    </div>

    <div v-else class="card">
      <div v-if="session !== 'connected'">
        <p class="muted">아직 로그인 전입니다. 로그인하면 {{ peerName }} 대화가 활성화됩니다. (트랜치 1: 화면·상태까지만, 실제 세션 연결 없음)</p>
        <button @click="openModal">Claude 로그인</button>
        <button class="ghost" @click="session = 'expired'">만료 상태 미리보기</button>
      </div>
      <div v-else>
        <div class="chatlog">
          <div v-for="(m, i) in messages" :key="i" :class="m.me ? 'me' : 'them'"><p>{{ m.text }}</p></div>
          <p v-if="!messages.length" class="muted">{{ peerName }}와 연결됐습니다. 메시지를 입력하세요. (트랜치 1 화면 시연용)</p>
        </div>
        <div class="formrow">
          <label class="grow">메시지 <input v-model="draft" @keyup.enter="send" /></label>
          <button @click="send">보내기</button>
          <button class="ghost" @click="session = 'expired'">세션 만료 시연</button>
        </div>
      </div>
    </div>

    <!-- Step 1: guidance modal. -->
    <div v-if="modal" class="modal-back" @click.self="modal = false">
      <div class="modal">
        <h3>Claude 로그인 안내</h3>
        <ol>
          <li>아래 버튼을 누르면 공식 로그인 창이 열립니다.</li>
          <li>의장님 계정으로 로그인합니다.</li>
          <li>완료되면 이 화면으로 돌아와 대화가 활성화됩니다.</li>
        </ol>
        <div class="btnrow">
          <button @click="openPopup">공식 로그인 창 열기</button>
          <button class="ghost" @click="modal = false">닫기</button>
        </div>
      </div>
    </div>

    <!-- Step 2: official login popup (simulated window for tranche 1). -->
    <div v-if="popup" class="modal-back">
      <div class="modal">
        <h3>공식 로그인 (시연)</h3>
        <p class="muted">트랜치 1에서는 실제 세션을 연결하지 않습니다. 아래 완료 버튼으로 동선만 확인합니다.</p>
        <button @click="finishLogin">로그인 완료</button>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue'

const peer = ref('chief')
const session = ref('idle') // idle | connected | expired
const modal = ref(false)
const popup = ref(false)
const draft = ref('')
const messages = ref([])

const peerName = computed(() => (peer.value === 'chief' ? '총괄' : '작업자'))

function openModal() {
  modal.value = true
}

function openPopup() {
  modal.value = false
  popup.value = true
}

function finishLogin() {
  popup.value = false
  session.value = 'connected'
  messages.value = []
}

function send() {
  const text = draft.value.trim()
  if (!text) return
  messages.value.push({ me: true, text })
  messages.value.push({ me: false, text: `[${peerName.value} 시연 응답] "${text}" 확인했습니다. (트랜치 1: 실제 에이전트 미연결)` })
  draft.value = ''
}
</script>
