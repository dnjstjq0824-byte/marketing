<template>
  <div class="pu-tabs" role="tablist" aria-label="가이드 분류">
    <button
      v-for="c in cats"
      :key="c.id"
      class="pu-tab"
      :class="{ on: c.id === active }"
      role="tab"
      :aria-selected="c.id === active"
      @click="$emit('select', c.id)"
    >
      {{ c.label }}
    </button>
  </div>
</template>

<script setup>
defineProps({
  cats: { type: Array, default: () => [] },
  active: { type: String, default: '' },
})
defineEmits(['select'])
</script>
