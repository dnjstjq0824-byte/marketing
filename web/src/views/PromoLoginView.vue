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
      <div class="pu-authform">
        <div class="pu-modebtns">
          <button class="pu-btn small" :class="{ 'pu-btn-primary': mode === 'login' }" @click="mode = 'login'">로그인</button>
          <button class="pu-btn small" :class="{ 'pu-btn-primary': mode === 'signup' }" @click="mode = 'signup'">회원가입</button>
        </div>
        <div class="pu-form">
          <label>이메일 <input v-model="form.email" type="email" autocomplete="email" /></label>
          <label>비밀번호 (8자 이상) <input v-model="form.password" type="password" autocomplete="current-password" @keyup.enter="submit" /></label>
          <label v-if="mode === 'signup'">닉네임 <input v-model="form.nickname" @keyup.enter="submit" /></label>
        </div>
        <p v-if="msg" class="pu-error">{{ msg }}</p>
        <p style="margin-top: 16px; margin-bottom: 0">
          <button class="pu-btn pu-btn-primary block" @click="submit">{{ mode === 'login' ? '로그인' : '가입하기' }}</button>
        </p>
      </div>
    </div>
  </PublicShell>
</template>

<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PublicShell from '../components/PublicShell.vue'
import { memberLogin, memberSignup } from '../stores/member.js'

const route = useRoute()
const router = useRouter()
const mode = ref('login')
const form = ref({ email: '', password: '', nickname: '' })
const msg = ref('')

function backTo() {
  const next = route.query.next
  return typeof next === 'string' && next.startsWith('/promo') ? next : '/promo'
}

async function submit() {
  msg.value = ''
  try {
    if (mode.value === 'login') {
      await memberLogin(form.value.email, form.value.password)
    } else {
      await memberSignup(form.value.email, form.value.password, form.value.nickname)
    }
    router.replace(backTo())
  } catch (e) {
    msg.value = e.message || '실패했습니다'
  }
}
</script>
