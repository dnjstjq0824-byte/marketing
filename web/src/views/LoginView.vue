<template>
  <div class="login">
    <p class="brand"><LogoMark :size="40" /> SOMUN</p>
    <p class="muted">AI가 만들고 올리는 마케팅</p>
    <form class="card" novalidate @submit.prevent="submit">
      <div class="field">
        <label for="login-id">아이디</label>
        <input id="login-id" ref="idInput" v-model="username" autocomplete="username" autofocus @input="dirty.id = true" />
        <FieldError :message="dirty.id || tried ? userErr : ''" />
      </div>
      <div class="field">
        <label for="login-pw">비밀번호</label>
        <span class="pwrow">
          <input id="login-pw" v-model="password" :type="show ? 'text' : 'password'" autocomplete="current-password" @input="dirty.pw = true" />
          <button type="button" class="ghost small" @click="show = !show">{{ show ? '숨기기' : '보기' }}</button>
        </span>
        <FieldError :message="dirty.pw || tried ? pwErr : ''" />
      </div>
      <label class="remember"><input type="checkbox" v-model="remember" /> 아이디 저장</label>
      <p v-if="msg" class="error" role="alert">{{ msg }}</p>
      <button type="submit" :disabled="busy || locked">{{ busy ? '로그인 중…' : '로그인' }}</button>
      <p class="links">홍보 회원이세요? <RouterLink to="/promo/signup">가입하기</RouterLink> · <RouterLink to="/promo">홍보관 가기</RouterLink> · <RouterLink to="/brand">SOMUN 소개</RouterLink></p>
    </form>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiFetch, setSession } from '../stores/auth.js'
import FieldError from '../components/console/FieldError.vue'
import LogoMark from '../components/LogoMark.vue'

const REMEMBER_KEY = 'somun_operator_remember'

const route = useRoute()
const router = useRouter()
const username = ref(localStorage.getItem(REMEMBER_KEY) || '')
const password = ref('')
const remember = ref(!!localStorage.getItem(REMEMBER_KEY))
const show = ref(false)
const busy = ref(false)
const locked = ref(false)
const tried = ref(false)
const msg = ref('')
const dirty = ref({ id: false, pw: false })
const idInput = ref(null)

const userErr = computed(() => (!username.value.trim() ? '아이디를 입력하세요' : ''))
const pwErr = computed(() => (!password.value ? '비밀번호를 입력하세요' : ''))

onMounted(() => {
  if (idInput.value) idInput.value.focus()
})

async function submit() {
  tried.value = true
  if (busy.value || locked.value || userErr.value || pwErr.value) return
  busy.value = true
  msg.value = ''
  try {
    const data = await apiFetch('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username: username.value.trim(), password: password.value }),
    })
    if (remember.value) localStorage.setItem(REMEMBER_KEY, username.value.trim())
    else localStorage.removeItem(REMEMBER_KEY)
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
.brand { font-size: 28px; font-weight: 800; display: flex; align-items: center; gap: var(--s3); }
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
.field { display: flex; flex-direction: column; gap: var(--s2); }
.field > label { font-weight: 700; }
input[type="text"], input[type="password"] {
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: var(--s3);
  background: var(--panel);
  font-size: 16px;
  width: 100%;
}
input:focus { outline: 2px solid var(--accent-soft); border-color: var(--accent); }
.pwrow { display: flex; gap: var(--s2); }
.pwrow input { flex: 1; }
.remember { display: flex; align-items: center; gap: var(--s2); font-size: 14px; }
.remember input { width: 18px; height: 18px; }
button[type="submit"] {
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: var(--r-md);
  padding: var(--s3);
  font-weight: 700;
  font-size: 16px;
  cursor: pointer;
}
button[type="submit"]:hover { background: var(--accent-dark); }
button[type="submit"]:disabled { opacity: 0.6; cursor: not-allowed; }
.error { color: var(--warn); margin: 0; }
.links { margin: 0; text-align: center; font-size: 14px; color: var(--muted); }
.ghost { border: 1px solid var(--line); background: var(--panel); color: var(--ink); border-radius: var(--r-md); }
.small { font-size: 13px; padding: var(--s1) var(--s2); white-space: nowrap; }
</style>
