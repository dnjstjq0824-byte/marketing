<template>
  <section class="page">
    <h2>설정</h2>
    <p class="muted">운영자 로그인은 이 화면에서만 진입합니다. 계정 화면에 몰아넣지 않습니다.</p>

    <div v-if="!auth.user" class="card narrow">
      <h3>운영자 로그인</h3>
      <form @submit.prevent="doLogin">
        <label>아이디
          <input v-model="username" autocomplete="username" required />
        </label>
        <label>비밀번호
          <input v-model="password" type="password" autocomplete="current-password" required />
        </label>
        <p v-if="error" class="error">{{ error }}</p>
        <button type="submit" :disabled="busy">{{ busy ? '로그인 중…' : '로그인' }}</button>
      </form>
    </div>

    <div v-else class="card narrow">
      <h3>로그인됨</h3>
      <p><strong>{{ auth.user.username }}</strong> (id {{ auth.user.id }})</p>
      <button @click="doLogout">로그아웃</button>
    </div>

    <div class="card narrow">
      <h3>안내</h3>
      <ul>
        <li>기본 계정은 서버 환경변수 ADMIN_USER / ADMIN_PASS 로 정합니다.</li>
        <li>로그인 세션은 12시간 뒤 만료됩니다.</li>
      </ul>
    </div>
  </section>
</template>

<script setup>
import { ref } from 'vue'
import { auth, login, logout } from '../stores/auth.js'

const username = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function doLogin() {
  busy.value = true
  error.value = ''
  try {
    await login(username.value.trim(), password.value)
    password.value = ''
  } catch (e) {
    error.value = e.message || '로그인 실패'
  } finally {
    busy.value = false
  }
}

async function doLogout() {
  await logout()
}
</script>
