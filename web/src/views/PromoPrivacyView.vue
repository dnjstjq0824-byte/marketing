<template>
  <PublicShell>
    <h1 class="pu-h1">개인정보 처리방침</h1>
    <p class="pu-lead">수집·이용·보관 기준입니다.</p>
    <LegalDoc :article="PRIVACY_ARTICLE" :version="LEGAL_VERSION" :notice="LEGAL_NOTICE" />
  </PublicShell>
</template>

<script setup>
import PublicShell from '../components/PublicShell.vue'
import LegalDoc from '../components/promo/LegalDoc.vue'
import { PRIVACY_ARTICLE, LEGAL_VERSION, LEGAL_NOTICE } from '../lib/legalContent.js'
</script>
