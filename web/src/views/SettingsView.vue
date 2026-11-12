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

    <div v-if="auth.user" class="card narrow">
      <div class="sect-head"><h3>운영자 목록</h3><span class="count">{{ users.length }}명</span></div>
      <ul v-if="users.length" class="speclist">
        <li v-for="u in users" :key="u.id">
          <span class="k">{{ u.username }}</span>
          <span>{{ u.role }} <button v-if="u.id !== auth.user.id" class="ghost" @click="removeUser(u.id)">삭제</button></span>
        </li>
      </ul>
      <div class="form-grid">
        <label>새 아이디 <input v-model="newUser.username" /></label>
        <label>비밀번호(8자+) <input v-model="newUser.password" type="password" /></label>
        <div class="full"><button @click="addUser">운영자 추가</button></div>
      </div>
      <p v-if="userError" class="error">{{ userError }}</p>
    </div>

    <div v-if="auth.user" class="card narrow">
      <h3>비밀번호 변경</h3>
      <label>현재 비밀번호 <input v-model="pwForm.current" type="password" /></label>
      <label>새 비밀번호(8자+) <input v-model="pwForm.next" type="password" /></label>
      <button @click="changePassword">변경</button>
      <p v-if="pwMsg" class="ok">{{ pwMsg }}</p>
      <p v-if="pwError" class="error">{{ pwError }}</p>
    </div>

    <div class="card narrow">
      <h3>안내</h3>
      <ul>
        <li>기본 계정은 서버 환경변수 ADMIN_USER / ADMIN_PASS 로 정합니다.</li>
        <li>로그인 세션은 12시간 뒤 만료됩니다.</li>
        <li>자기 계정과 마지막 운영자는 삭제할 수 없습니다.</li>
      </ul>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import { apiFetch, auth, login, logout } from '../stores/auth.js'

const username = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)
const users = ref([])
const newUser = ref({ username: '', password: '' })
const userError = ref('')
const pwForm = ref({ current: '', next: '' })
const pwMsg = ref('')
const pwError = ref('')

async function doLogin() {
  busy.value = true
  error.value = ''
  try {
    await login(username.value.trim(), password.value)
    password.value = ''
    await loadUsers()
  } catch (e) {
    error.value = e.message || '로그인 실패'
  } finally {
    busy.value = false
  }
}

async function doLogout() {
  await logout()
  users.value = []
}

async function loadUsers() {
  if (!auth.user) return
  userError.value = ''
  try {
    const data = await apiFetch('/api/users')
    users.value = data.users || []
  } catch (e) {
    userError.value = e.message
  }
}

async function addUser() {
  userError.value = ''
  try {
    await apiFetch('/api/users', { method: 'POST', body: JSON.stringify(newUser.value) })
    newUser.value = { username: '', password: '' }
    await loadUsers()
  } catch (e) {
    userError.value = e.message
  }
}

async function removeUser(id) {
  userError.value = ''
  try {
    await apiFetch(`/api/users/${id}`, { method: 'DELETE' })
    await loadUsers()
  } catch (e) {
    userError.value = e.message
  }
}

async function changePassword() {
  pwMsg.value = ''
  pwError.value = ''
  try {
    await apiFetch('/api/auth/password', { method: 'PATCH', body: JSON.stringify(pwForm.value) })
    pwForm.value = { current: '', next: '' }
    pwMsg.value = '비밀번호를 변경했습니다.'
  } catch (e) {
    pwError.value = e.message
  }
}

watch(() => auth.user, loadUsers)
onMounted(loadUsers)
</script>
