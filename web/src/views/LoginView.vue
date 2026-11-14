<template>
  <div class="login">
    <p class="brand"><span class="mark" aria-hidden="true">●</span> SOMUN</p>
    <p class="muted">AI가 만들고 올리는 마케팅</p>
    <form class="card" @submit.prevent="submit">
      <label>아이디
        <input v-model="username" autocomplete="username" required />
      </label>
      <label>비밀번호
        <span class="pwrow">
          <input v-model="password" :type="show ? 'text' : 'password'" autocomplete="current-password" required />
          <button type="button" class="ghost small" @click="show = !show" :aria-label="show ? '비밀번호 숨기기' : '비밀번호 보기'">👁</button>
        </span>
      </label>
      <p v-if="msg" class="error" role="alert">{{ msg }}</p>
      <button type="submit" :disabled="busy || locked">{{ busy ? '로그인 중…' : '로그인' }}</button>
    </form>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiFetch, setSession } from '../stores/auth.js'

const route = useRoute()
const router = useRouter()
const username = ref('')
const password = ref('')
const show = ref(false)
const busy = ref(false)
const locked = ref(false)
const msg = ref('')

async function submit() {
  if (busy.value || locked.value) return
  busy.value = true
  msg.value = ''
  try {
    const data = await apiFetch('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username: username.value, password: password.value }),
    })
    setSession(data.token, data.user)
    const next = typeof route.query.next === 'string' && route.query.next.startsWith('/') ? route.query.next : '/'
    router.push(next)
  } catch (e) {
    if (e.status === 429) {
      locked.value = true
      msg.value = '로그인 시도가 너무 많습니다. 15분 뒤 다시 시도하세요.'
    } else {
      msg.value = '아이디 또는 비밀번호가 틀렸습니다'
    }
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.login {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--s2);
  padding: var(--s4);
}
.brand { font-size: 28px; font-weight: 800; }
.brand .mark { color: var(--accent); }
.card {
  background: var(--panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-lg);
  box-shadow: var(--shadow);
  padding: var(--s6);
  width: 100%;
  max-width: 400px;
  display: flex;
  flex-direction: column;
  gap: var(--s4);
}
label { display: flex; flex-direction: column; gap: var(--s2); font-weight: 700; }
input {
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: var(--s3);
  background: var(--panel);
  font-weight: 400;
}
.pwrow { display: flex; gap: var(--s2); }
.pwrow input { flex: 1; }
button[type="submit"] {
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: var(--r-md);
  padding: var(--s3);
  font-weight: 700;
}
button[type="submit"]:disabled { opacity: 0.6; }
.error { color: var(--warn); }
.ghost { border: 1px solid var(--line); background: var(--panel); color: var(--ink); border-radius: var(--r-md); }
.small { font-size: 13px; padding: var(--s1) var(--s2); }
</style>
