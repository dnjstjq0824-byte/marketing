<template>
  <div class="sk" aria-hidden="true">
    <div v-for="n in rows" :key="n" class="row"><span class="bar w1"></span><span class="bar w2"></span></div>
  </div>
</template>

<script setup>
defineProps({ rows: { type: Number, default: 3 } })
</script>

<style scoped>
.sk { display: flex; flex-direction: column; gap: var(--s3); }
.row {
  background: var(--panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-lg);
  padding: var(--s4);
  display: flex;
  flex-direction: column;
  gap: var(--s2);
}
.bar { height: 14px; border-radius: 7px; background: linear-gradient(90deg, var(--line-soft), var(--soft), var(--line-soft)); }
.w1 { width: 40%; }
.w2 { width: 75%; }
@media (prefers-reduced-motion: no-preference) {
  .bar { animation: shimmer 1.4s infinite; }
  @keyframes shimmer {
    0% { opacity: 0.6; }
    50% { opacity: 1; }
    100% { opacity: 0.6; }
  }
}
</style>
