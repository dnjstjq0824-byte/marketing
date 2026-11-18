<template>
  <section class="pu-band ghero" aria-label="가이드 검색">
    <p class="pu-chip">사용 가이드</p>
    <h1 class="pu-h1 gtitle">무엇을 도와드릴까요?</h1>
    <p class="pu-lead glead">키워드를 입력하면 관련 가이드가 바로 걸러집니다.</p>
    <div class="gsearch">
      <input
        :value="modelValue"
        type="search"
        placeholder="예: 수수료, 링크, 반려"
        aria-label="가이드 검색"
        @input="$emit('update:modelValue', $event.target.value)"
      />
      <button v-if="modelValue" class="pu-btn small" @click="$emit('update:modelValue', '')">지우기</button>
    </div>
  </section>
</template>

<script setup>
defineProps({ modelValue: { type: String, default: '' } })
defineEmits(['update:modelValue'])
</script>

<style scoped>
.ghero { margin-bottom: 18px; background: linear-gradient(135deg, var(--pu-accent-soft), var(--pu-card)); }
.gtitle { font-size: 28px; margin: 10px 0 4px; }
.glead { margin-bottom: 14px; }
.gsearch { display: flex; gap: 8px; max-width: 520px; }
.gsearch input {
  flex: 1;
  border: 1px solid var(--pu-line);
  border-radius: 999px;
  padding: 11px 18px;
  font-size: 15px;
  font-family: inherit;
  color: var(--pu-ink);
  background: var(--pu-card);
}
.gsearch input:focus { outline: 2px solid var(--pu-accent-soft); border-color: var(--pu-accent); }
</style>
