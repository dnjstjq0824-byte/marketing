<template>
  <article :id="'g-' + article.id" :data-guide-article="article.id" class="pu-card gart">
    <p v-if="showCat" class="gcat"><span class="pu-chip neutral">{{ catLabel }}</span></p>
    <h3 class="pu-sec-title">{{ article.title }}</h3>
    <p class="pu-muted pu-small gdesc">{{ article.desc }}</p>
    <ol class="gsteps">
      <li v-for="(s, i) in article.steps" :key="i"><strong>{{ s[0] }}</strong><span>{{ s[1] }}</span></li>
    </ol>
    <p v-for="(t, i) in article.tips" :key="'t' + i" class="callout tip">팁 · {{ t }}</p>
    <p v-for="(w, i) in article.warns" :key="'w' + i" class="callout warn">주의 · {{ w }}</p>
    <GuideFeedback :article-id="article.id" />
  </article>
</template>

<script setup>
import GuideFeedback from './GuideFeedback.vue'

defineProps({
  article: { type: Object, required: true },
  catLabel: { type: String, default: '' },
  showCat: { type: Boolean, default: false },
})
</script>

<style scoped>
.gart { display: flex; flex-direction: column; gap: 6px; scroll-margin-top: 84px; }
.gcat, .gdesc { margin: 0; }
.gsteps { list-style: none; margin: 6px 0 0; padding: 0; display: flex; flex-direction: column; gap: 8px; counter-reset: gstep; }
.gsteps li {
  counter-increment: gstep;
  border: 1px solid var(--pu-line-soft);
  border-radius: var(--pu-r-sm);
  padding: 10px 12px 10px 48px;
  position: relative;
  background: var(--pu-panel);
}
.gsteps li::before {
  content: counter(gstep);
  position: absolute;
  left: 12px;
  top: 10px;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--pu-accent);
  color: #fff;
  font-size: 13px;
  font-weight: 800;
  display: flex;
  align-items: center;
  justify-content: center;
}
.gsteps strong { display: block; font-size: 14px; }
.gsteps span { font-size: 13px; color: var(--pu-muted); }
.callout { margin: 4px 0 0; font-size: 13px; border-radius: var(--pu-r-sm); padding: 10px 12px; }
.callout.tip { background: var(--pu-ok-soft); color: var(--pu-ok); }
.callout.warn { background: var(--pu-warn-soft); color: var(--pu-warn); }
</style>
