<template>
  <div class="bd-page">
    <BrandHeader />
    <main id="bd-main" class="bd-wrap">
      <BrandHero />
      <BrandSteps />
      <BrandFeatures />
      <BrandGuard />
      <BrandCta />
    </main>
    <BrandFooter />
  </div>
</template>

<script setup>
import BrandHeader from '../components/brand/BrandHeader.vue'
import BrandHero from '../components/brand/BrandHero.vue'
import BrandSteps from '../components/brand/BrandSteps.vue'
import BrandFeatures from '../components/brand/BrandFeatures.vue'
import BrandGuard from '../components/brand/BrandGuard.vue'
import BrandCta from '../components/brand/BrandCta.vue'
import BrandFooter from '../components/brand/BrandFooter.vue'
</script>

<style scoped>
.bd-page { background: var(--bg); color: var(--ink); min-height: 100vh; }
.bd-wrap { max-width: 1120px; margin: 0 auto; padding: 0 20px 8px; }
</style>

<style>
html:has(.bd-page) { scroll-behavior: smooth; }
@media (prefers-reduced-motion: reduce) {
  html:has(.bd-page) { scroll-behavior: auto; }
}
</style>
