<template>
  <footer class="bd-foot">
    <div class="bd-foot-in">
      <div class="bd-fbrand">
        <strong>SOMUN</strong>
        <span>AI가 만들고 올리는 마케팅</span>
      </div>
      <nav class="bd-fcol" aria-label="바로가기">
        <RouterLink to="/login">콘솔 열기</RouterLink>
        <RouterLink to="/promo">위탁 마켓</RouterLink>
        <RouterLink to="/promo/guide">이용 가이드</RouterLink>
        <RouterLink to="/promo/signup">회원가입</RouterLink>
      </nav>
    </div>
    <div class="bd-base"><span>© SOMUN</span></div>
  </footer>
</template>

<style scoped>
.bd-foot { border-top: 1px solid var(--line-soft); margin-top: 24px; }
.bd-foot-in { max-width: 1120px; margin: 0 auto; padding: 32px 20px; display: flex; gap: 32px; justify-content: space-between; flex-wrap: wrap; }
.bd-fbrand { display: flex; flex-direction: column; gap: 4px; }
.bd-fbrand strong { font-size: 16px; letter-spacing: 0.5px; }
.bd-fbrand span { color: var(--muted); font-size: 13px; }
.bd-fcol { display: flex; gap: 20px; flex-wrap: wrap; align-items: center; }
.bd-fcol a { color: var(--ink-soft); text-decoration: none; font-size: 14px; font-weight: 700; }
.bd-fcol a:hover { color: var(--accent-dark); }
.bd-base { max-width: 1120px; margin: 0 auto; padding: 0 20px 28px; color: var(--muted); font-size: 13px; }
</style>
