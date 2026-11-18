<template>
  <PublicShell v-if="isPromo">
    <div class="pu-card pu-empty">
      <p class="nf-code pu-num">404</p>
      <h1 class="pu-sec-title">페이지를 찾을 수 없습니다</h1>
      <p class="pu-muted">주소가 바뀌었거나 삭제된 페이지입니다.</p>
      <div class="pu-rowline nf-btns">
        <RouterLink to="/promo" class="pu-btn pu-btn-primary">홍보 메인으로</RouterLink>
        <RouterLink to="/promo/products" class="pu-btn">제품 보기</RouterLink>
      </div>
    </div>
  </PublicShell>
  <div v-else class="nf">
    <p class="nf-code num">404</p>
    <h1>페이지를 찾을 수 없습니다</h1>
    <p class="muted">주소가 바뀌었거나 삭제된 페이지입니다.</p>
    <div class="nf-btns">
      <RouterLink to="/" class="btn-primary">홈으로</RouterLink>
      <button class="ghost" @click="router.back()">뒤로 가기</button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PublicShell from '../components/PublicShell.vue'

const route = useRoute()
const router = useRouter()
const isPromo = computed(() => route.path.startsWith('/promo'))
</script>

<style scoped>
.nf-code { font-size: 44px; font-weight: 800; margin: 0; color: var(--pu-accent-dark, var(--accent-dark)); }
.nf-btns { justify-content: center; margin-top: 18px; }
.nf { min-height: 100vh; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px; padding: 24px; text-align: center; }
.nf h1 { font-size: 22px; margin: 0; }
.nf .nf-btns { display: flex; gap: 8px; }
.nf .btn-primary { background: var(--accent); color: #fff; border-radius: var(--r-md); padding: 10px 20px; text-decoration: none; font-weight: 700; }
.nf .ghost { border: 1px solid var(--line); background: var(--panel); color: var(--ink); border-radius: var(--r-md); padding: 10px 20px; font-weight: 700; }
</style>
