<template>
  <PublicShell>
    <div class="pu-card pu-authsplit">
      <div class="pu-authside">
        <h2>홍보하고<br />수수료를 받으세요</h2>
        <ul>
          <li>마음에 드는 제품에 신청</li>
          <li>승인되면 비공개 링크 열림</li>
          <li>주문 1건당 고정 수수료 적립</li>
        </ul>
      </div>
      <form class="pu-authform pu-form" novalidate @submit.prevent="submit">
        <label>이메일 <input v-model="email" type="email" autocomplete="email" autofocus @input="dirty.email = true" /></label>
        <p v-if="(dirty.email || tried) && emailMsg" class="pu-error">{{ emailMsg }}</p>
        <label>비밀번호 <span class="pwrow"><input v-model="password" :type="show ? 'text' : 'password'" autocomplete="current-password" @input="dirty.pw = true" /><button type="button" class="pu-btn small" @click="show = !show">{{ show ? '숨기기' : '보기' }}</button></span></label>
        <p v-if="(dirty.pw || tried) && pwMsg" class="pu-error">{{ pwMsg }}</p>
        <p v-if="msg" class="pu-error" role="alert">{{ msg }}</p>
        <p style="margin-top: 8px; margin-bottom: 0">
          <button class="pu-btn pu-btn-primary block" type="submit" :disabled="busy">{{ busy ? '로그인 중…' : '로그인' }}</button>
        </p>
        <p class="pu-small pu-muted go-signup">아직 회원이 아니세요? <RouterLink to="/promo/signup">가입하기</RouterLink></p>
      </form>
    </div>
  </PublicShell>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PublicShell from '../components/PublicShell.vue'
import { memberLogin } from '../stores/member.js'
import { emailError } from '../lib/signupValidate.js'

const route = useRoute()
const router = useRouter()
const email = ref('')
const password = ref('')
const show = ref(false)
const busy = ref(false)
const tried = ref(false)
const msg = ref('')
const dirty = ref({ email: false, pw: false })

const emailMsg = computed(() => emailError(email.value))
const pwMsg = computed(() => (!password.value ? '비밀번호를 입력하세요' : ''))

function backTo() {
  const next = route.query.next
  return typeof next === 'string' && next.startsWith('/promo') ? next : '/promo'
}

async function submit() {
  tried.value = true
  if (busy.value || emailMsg.value || pwMsg.value) return
  busy.value = true
  msg.value = ''
  try {
    await memberLogin(email.value.trim(), password.value)
    router.replace(backTo())
  } catch {
    msg.value = '이메일 또는 비밀번호가 틀렸습니다'
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.pwrow { display: flex; gap: 8px; }
.pwrow input { flex: 1; min-width: 0; }
.pwrow button { flex: none; }
.pu-error { margin: 0; font-size: 13px; }
.go-signup { margin: 0; text-align: center; }
</style>
