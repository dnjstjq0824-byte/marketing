<template>
  <div class="pu-form">
    <fieldset class="su-group">
      <legend>계정</legend>
      <label>이메일(아이디) <input v-model="form.email" type="email" autocomplete="email" autofocus @input="touch('email')" /></label>
      <p v-if="touched.email && emailMsg" class="pu-error">{{ emailMsg }}</p>
      <p v-else-if="touched.email" class="su-ok">사용 가능한 형식입니다</p>
      <label>닉네임 <input v-model="form.nickname" maxlength="30" autocomplete="nickname" @input="touch('nickname')" /></label>
      <p v-if="touched.nickname && nickMsg" class="pu-error">{{ nickMsg }}</p>
      <label>초대 코드 <input v-model="form.invite" placeholder="예: SOMUN-AB12-CD34" autocomplete="off" @input="touch('invite')" /></label>
      <p v-if="touched.invite && invMsg" class="pu-error">{{ invMsg }}</p>
      <p v-else class="pu-small pu-muted">베타 기간에는 초대 코드가 필요합니다. 운영자에게 받으세요.</p>
    </fieldset>
    <fieldset class="su-group">
      <legend>비밀번호</legend>
      <label>비밀번호(8자 이상) <input v-model="form.password" :type="show ? 'text' : 'password'" autocomplete="new-password" @input="touch('password')" /></label>
      <PasswordMeter :password="form.password" />
      <p v-if="touched.password && pwMsg" class="pu-error">{{ pwMsg }}</p>
      <label>비밀번호 확인 <input v-model="form.confirm" :type="show ? 'text' : 'password'" autocomplete="new-password" @input="touch('confirm')" /></label>
      <p v-if="touched.confirm && cfMsg" class="pu-error">{{ cfMsg }}</p>
      <label class="pu-check"><input type="checkbox" v-model="show" /> 비밀번호 보기</label>
    </fieldset>
  </div>
</template>

<script setup>
import { reactive, ref, computed } from 'vue'
import PasswordMeter from './PasswordMeter.vue'
import { emailError, nicknameError, passwordError, confirmError, inviteError } from '../../lib/signupValidate.js'

const props = defineProps({ form: { type: Object, required: true } })
const show = ref(false)
const touched = reactive({})
function touch(k) { touched[k] = true }
const emailMsg = computed(() => emailError(props.form.email))
const nickMsg = computed(() => nicknameError(props.form.nickname))
const invMsg = computed(() => inviteError(props.form.invite))
const pwMsg = computed(() => passwordError(props.form.password))
const cfMsg = computed(() => confirmError(props.form.password, props.form.confirm))
</script>

<style scoped>
.su-group { border: 1px solid var(--pu-line); border-radius: var(--pu-r-sm); padding: 14px; display: flex; flex-direction: column; gap: 10px; margin: 0; }
.su-group legend { font-size: 14px; font-weight: 800; padding: 0 6px; }
.su-ok { color: var(--pu-ok); font-size: 13px; margin: 0; }
.pu-error { margin: 0; font-size: 13px; }
</style>
