<template>
  <ol v-if="items.length" class="tl">
    <li v-for="(it, i) in items" :key="i">
      <i class="dot" :class="it.kind" aria-hidden="true"></i>
      <div class="entry">
        <div class="head">
          <strong>{{ it.title }}</strong>
          <span class="muted small">{{ it.at }}</span>
        </div>
        <p v-if="it.note" class="muted small note">{{ it.note }}</p>
        <details v-if="it.detail" class="orig">
          <summary class="small">원문 펼치기</summary>
          <pre class="pre">{{ it.detail }}</pre>
        </details>
      </div>
    </li>
  </ol>
  <p v-else class="muted">기록이 없습니다.</p>
</template>

<script setup>
defineProps({ items: { type: Array, default: () => [] } })
</script>

<style scoped>
.tl { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
.tl li { display: flex; gap: var(--s3); padding: var(--s2) 0; border-top: 1px solid var(--line-soft); }
.tl li:first-child { border-top: none; padding-top: 0; }
.dot { width: 10px; height: 10px; border-radius: 50%; margin-top: 5px; flex-shrink: 0; background: var(--muted); }
.dot.ai { background: var(--accent); }
.dot.human { background: var(--ok); }
.dot.warn { background: var(--warn); }
.entry { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: var(--s1); }
.head { display: flex; gap: var(--s2); align-items: baseline; flex-wrap: wrap; }
.note { margin: 0; overflow: hidden; text-overflow: ellipsis; }
.orig summary { cursor: pointer; color: var(--accent-dark); }
.pre {
  white-space: pre-wrap; word-break: break-word; background: var(--soft);
  border: 1px solid var(--line-soft); border-radius: var(--r-md);
  padding: var(--s2); margin: var(--s1) 0 0; max-height: 300px; overflow: auto;
  font-size: 12px; line-height: 1.6;
}
</style>
