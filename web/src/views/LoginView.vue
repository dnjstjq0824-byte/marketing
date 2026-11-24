<template>
  <div class="login-wrap">
    <div class="login-shell">
      <div class="login-brand">
        <div class="lb-top">
          <LogoMark :size="38" />
          <span class="lb-name">SOMUN</span>
        </div>
        <h1>AI 에이전트 팀이<br />처음부터 끝까지<br />마케팅을 실행합니다</h1>
        <p class="lb-sub">상품 링크·사진·주제만 투입하면 분석·전략·제작·검수·발행·성과 수집까지 자동으로 완주하는 마케팅 코어 엔진.</p>
        <ul class="lb-points">
          <li>브랜드 전담 AI 팀 · 작성자/검수자 자동 분리</li>
          <li>발행 전 유사문서·금칙어·대가 표시 자동 가드레일</li>
          <li>멀티 플랫폼 자동 배정·예약 발행 및 24h 성과 수집</li>
        </ul>
      </div>

      <form class="login-card" novalidate @submit.prevent="submit">
        <div class="lc-head">
          <h2>운영자 콘솔 로그인</h2>
          <p class="muted small">등록된 운영자 계정으로 로그인하세요.</p>
        </div>

        <div class="field">
          <label for="login-id">아이디</label>
          <input id="login-id" ref="idInput" v-model="username" autocomplete="username" placeholder="admin" autofocus @input="dirty.id = true" />
          <FieldError :message="dirty.id || tried ? userErr : ''" />
        </div>

        <div class="field">
          <label for="login-pw">비밀번호</label>
          <div class="pwrow">
            <input id="login-pw" v-model="password" :type="show ? 'text' : 'password'" autocomplete="current-password" placeholder="비밀번호 입력" @input="dirty.pw = true" />
            <button type="button" class="ghost small" @click="show = !show">{{ show ? '숨기기' : '보기' }}</button>
          </div>
          <FieldError :message="dirty.pw || tried ? pwErr : ''" />
        </div>

        <label class="remember"><input type="checkbox" v-model="remember" /> 아이디 저장</label>

        <p v-if="msg" class="banner err" role="alert">{{ msg }}</p>

        <button type="submit" class="login-btn" :disabled="busy || locked">
          {{ busy ? '로그인 중…' : '콘솔 들어가기 →' }}
        </button>

        <p class="links">
          홍보 회원이세요?
          <RouterLink to="/promo/signup">가입하기</RouterLink> ·
          <RouterLink to="/promo">홍보관</RouterLink> ·
          <RouterLink to="/brand">SOMUN 소개</RouterLink>
        </p>
      </form>
    </div>
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
      msg.value = '아이디 또는 비밀번호가 틀렸습니다.'
    }
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.login-wrap {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--bg);
}
.login-shell {
  width: 100%;
  max-width: 920px;
  display: grid;
  grid-template-columns: 1.05fr 0.95fr;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--r-xl);
  overflow: hidden;
  box-shadow: var(--shadow-lg);
}
.login-brand {
  background: linear-gradient(155deg, #0f172a 0%, #0b3b49 60%, #0e7490 100%);
  color: #f8fafc;
  padding: 44px 40px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  justify-content: center;
}
.lb-top { display: flex; align-items: center; gap: 10px; }
.lb-name { font-size: 20px; font-weight: 800; letter-spacing: 0.1em; color: #fff; }
.login-brand h1 { font-size: 28px; font-weight: 800; line-height: 1.28; letter-spacing: -0.03em; color: #fff; }
.lb-sub { font-size: 14.5px; line-height: 1.65; color: rgba(248, 250, 252, 0.8); }
.lb-points { list-style: none; margin: 8px 0 0; padding: 18px 0 0; border-top: 1px solid rgba(255, 255, 255, 0.14); display: flex; flex-direction: column; gap: 10px; font-size: 13.5px; color: rgba(248, 250, 252, 0.92); }
.lb-points li::before { content: '✓ '; color: #67e8f9; font-weight: 800; margin-right: 4px; }

.login-card {
  padding: 44px 40px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 18px;
}
.lc-head h2 { font-size: 22px; font-weight: 800; margin-bottom: 4px; }
.field { display: flex; flex-direction: column; gap: 6px; }
.pwrow { display: flex; gap: 8px; }
.pwrow input { flex: 1; }
.remember { display: flex; align-items: center; gap: 6px; font-size: 13.5px; font-weight: 500; color: var(--ink-soft); cursor: pointer; }
.login-btn {
  width: 100%;
  padding: 13px 18px;
  font-size: 15px;
  font-weight: 700;
  border-radius: var(--r-md);
  background: linear-gradient(135deg, #06b6d4 0%, #0e7490 100%);
  color: #fff;
}
.login-btn:hover:not(:disabled) { filter: brightness(1.06); }
.links { text-align: center; font-size: 13px; color: var(--muted); padding-top: 6px; }

@media (max-width: 780px) {
  .login-shell { grid-template-columns: 1fr; max-width: 440px; }
  .login-brand { padding: 28px 24px; gap: 12px; }
  .login-brand h1 { font-size: 22px; }
  .lb-points { display: none; }
  .login-card { padding: 28px 24px; }
}
</style>
