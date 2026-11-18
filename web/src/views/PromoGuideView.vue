<template>
  <PublicShell>
    <GuideHero v-model="q" />
    <GuideTabs :cats="GUIDE_CATS" :active="cat" @select="selectCat" />
    <div class="glayout">
      <div class="gmain">
        <p v-if="q.trim()" class="pu-muted pu-small gq">“{{ q.trim() }}” 검색 결과입니다.</p>
        <p v-if="!shown.length" class="pu-card">일치하는 가이드가 없습니다. 다른 키워드로 검색해 보세요.</p>
        <GuideArticle
          v-for="a in shown"
          :key="a.id"
          :article="a"
          :cat-label="catLabel(a.cat)"
          :show-cat="!!q.trim()"
        />
      </div>
      <GuideToc v-if="shown.length" :items="shown" :active-id="activeId" @update:activeId="activeId = $event" @jump="jump" />
    </div>
  </PublicShell>
</template>

<script setup>
import { computed, ref } from 'vue'
import PublicShell from '../components/PublicShell.vue'
import GuideHero from '../components/promo/GuideHero.vue'
import GuideTabs from '../components/promo/GuideTabs.vue'
import GuideArticle from '../components/promo/GuideArticle.vue'
import GuideToc from '../components/promo/GuideToc.vue'
import { GUIDE_ARTICLES, GUIDE_CATS } from '../lib/guideContent.js'

const q = ref('')
const cat = ref('start')
const activeId = ref('')

function hay(a) {
  return [a.title, a.desc, ...a.steps.flat(), ...a.tips, ...a.warns].join(' ')
}

const shown = computed(() => {
  const needle = q.value.trim()
  if (needle) return GUIDE_ARTICLES.filter((a) => hay(a).includes(needle))
  return GUIDE_ARTICLES.filter((a) => a.cat === cat.value)
})

function catLabel(id) {
  return (GUIDE_CATS.find((c) => c.id === id) || {}).label || ''
}

function selectCat(id) {
  cat.value = id
  q.value = ''
  activeId.value = ''
}

function jump(id) {
  activeId.value = id
  document.getElementById('g-' + id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
</script>

<style scoped>
.glayout { display: grid; grid-template-columns: 1fr 220px; gap: 20px; align-items: start; }
.gmain { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
.gq { margin: 0; }
@media (max-width: 900px) { .glayout { grid-template-columns: 1fr; } }
</style>
