<template>
  <PublicShell>
    <h1 class="pu-h1">회원가입</h1>
    <p class="pu-lead">이메일로 가입하고 바로 홍보를 시작하세요.</p>
    <ol class="su-progress">
      <li v-for="(s, i) in steps" :key="s" :class="{ on: step === i + 1, done: step > i + 1 }">{{ s }}</li>
    </ol>
    <div class="pu-card su-card">
      <SignupAccount v-if="step === 1" :form="form" />
      <SignupTerms v-else-if="step === 2" :terms="terms" />
      <div v-else class="su-welcome">
        <p class="su-hello">{{ form.nickname }}님, 환영합니다!</p>
        <p class="pu-muted">가입이 완료됐습니다. 어디로 갈까요?</p>
        <div class="su-btns">
          <RouterLink to="/promo/me" class="pu-btn pu-btn-primary">내 홍보함 가기</RouterLink>
          <RouterLink to="/promo/guide" class="pu-btn">이용 가이드</RouterLink>
        </div>
      </div>
      <p v-if="msg" class="pu-error" role="alert">{{ msg }}</p>
      <div v-if="step < 3" class="su-nav">
        <button v-if="step === 2" class="pu-btn" @click="step = 1">이전</button>
        <button v-if="step === 1" class="pu-btn pu-btn-primary block" :disabled="!step1ok" @click="step = 2">다음</button>
        <button v-else class="pu-btn pu-btn-primary block" :disabled="!termsOk || busy" @click="submit">{{ busy ? '가입 중…' : '가입하기' }}</button>
      </div>
      <p v-if="step < 3" class="pu-small pu-muted su-login">이미 회원이세요? <RouterLink to="/promo/login">로그인</RouterLink></p>
    </div>
  </PublicShell>
</template>

<script setup>
import { reactive, ref, computed } from 'vue'
import PublicShell from '../components/PublicShell.vue'
import SignupAccount from '../components/promo/SignupAccount.vue'
import SignupTerms from '../components/promo/SignupTerms.vue'
import { memberSignup } from '../stores/member.js'
import { emailError, nicknameError, passwordError, confirmError, inviteError, signupServerError } from '../lib/signupValidate.js'

const steps = ['계정 정보', '약관 동의', '가입 완료']
const step = ref(1)
const busy = ref(false)
const msg = ref('')
const form = reactive({ email: '', nickname: '', password: '', confirm: '', invite: '' })
const terms = reactive({ service: false, privacy: false, marketing: false })
const step1ok = computed(() => !emailError(form.email) && !nicknameError(form.nickname) && !passwordError(form.password) && !confirmError(form.password, form.confirm) && !inviteError(form.invite))
const termsOk = computed(() => terms.service && terms.privacy)

async function submit() {
  if (busy.value || !step1ok.value || !termsOk.value) return
  busy.value = true
  msg.value = ''
  try {
    await memberSignup(form.email.trim(), form.password, form.nickname.trim(), form.invite.trim())
    step.value = 3
  } catch (e) {
    msg.value = signupServerError(e)
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.su-progress { list-style: none; margin: 0 0 14px; padding: 0; display: flex; gap: 8px; }
.su-progress li { flex: 1; text-align: center; font-size: 13px; font-weight: 700; color: var(--pu-muted); border-bottom: 3px solid var(--pu-line); padding-bottom: 8px; }
.su-progress li.on { color: var(--pu-accent-dark); border-color: var(--pu-accent); }
.su-progress li.done { color: var(--pu-ok); border-color: var(--pu-ok); }
.su-card { max-width: 560px; display: flex; flex-direction: column; gap: 16px; }
.su-nav { display: flex; gap: 8px; }
.su-nav .block { flex: 1; }
.su-welcome { display: flex; flex-direction: column; gap: 8px; text-align: center; padding: 12px 0; }
.su-hello { font-size: 20px; font-weight: 800; margin: 0; }
.su-btns { display: flex; gap: 8px; justify-content: center; margin-top: 8px; flex-wrap: wrap; }
.su-login { margin: 0; text-align: center; }
</style>
