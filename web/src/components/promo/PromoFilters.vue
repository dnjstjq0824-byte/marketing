<template>
  <div class="filters">
    <div class="pu-form fsearch">
      <input :value="filters.q" type="search" placeholder="제품·상점 검색" aria-label="키워드 검색" @input="set('q', $event.target.value)" />
    </div>
    <div class="pu-cats" role="list" aria-label="카테고리">
      <button class="pu-cat" :class="{ on: !filters.cat }" @click="set('cat', '')"><i>전</i><span>전체</span></button>
      <button v-for="c in CATEGORIES" :key="c" class="pu-cat" :class="{ on: filters.cat === c }" @click="set('cat', c)"><i>{{ c.slice(0, 1) }}</i><span>{{ c }}</span></button>
    </div>
    <div class="pu-rowline frow">
      <label class="pu-check fopen"><input :checked="filters.openOnly" type="checkbox" @change="set('openOnly', $event.target.checked)" /> 모집 중만</label>
      <select v-model="sortModel" class="fsort" aria-label="정렬">
        <option value="popular">인기순</option>
        <option value="deadline">마감임박순</option>
        <option value="fee">수수료높은순</option>
        <option value="new">최신순</option>
      </select>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { CATEGORIES } from '../../stores/member.js'

const props = defineProps({ filters: { type: Object, required: true } })
const emit = defineEmits(['change'])

function set(key, value) {
  emit('change', { ...props.filters, [key]: value })
}

const sortModel = computed({
  get: () => props.filters.sort,
  set: (v) => set('sort', v),
})
</script>

<style scoped>
.filters { margin-bottom: 18px; }
.fsearch { margin-bottom: 6px; }
.frow { justify-content: space-between; margin-top: 4px; }
.fopen { font-size: 14px; font-weight: 700; }
.fsort { border: 1px solid var(--pu-line); border-radius: 999px; padding: 8px 14px; font-size: 14px; font-family: inherit; color: var(--pu-ink); background: var(--pu-card); }
</style>
