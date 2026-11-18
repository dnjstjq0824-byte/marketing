<template>
  <PublicShell>
    <h1 class="pu-h1">서비스 이용약관</h1>
    <p class="pu-lead">SOMUN 홍보 위탁 이용 조건입니다.</p>
    <LegalDoc :article="TERMS_ARTICLE" :version="LEGAL_VERSION" :notice="LEGAL_NOTICE" />
  </PublicShell>
</template>

<script setup>
import PublicShell from '../components/PublicShell.vue'
import LegalDoc from '../components/promo/LegalDoc.vue'
import { TERMS_ARTICLE, LEGAL_VERSION, LEGAL_NOTICE } from '../lib/legalContent.js'
</script>
