<template>
  <div class="twrap">
    <table class="tbl">
      <thead><tr>
        <th><button @click="$emit('sort', 'name')">제품명{{ arrow('name') }}</button></th>
        <th><button @click="$emit('sort', 'store')">스토어{{ arrow('store') }}</button></th>
        <th class="r"><button @click="$emit('sort', 'price')">판매가{{ arrow('price') }}</button></th>
        <th class="r">수수료</th>
        <th><button @click="$emit('sort', 'status')">상태{{ arrow('status') }}</button></th>
        <th>마감일</th>
        <th>관리</th>
      </tr></thead>
      <tbody>
        <tr v-for="p in rows" :key="p.id">
          <td><strong>{{ p.name }}</strong></td>
          <td class="pu-muted">{{ p.store }}</td>
          <td class="r pu-num">{{ won(p.price) }}</td>
          <td class="r pu-num">{{ won(p.feePerOrder) }}</td>
          <td><span class="pu-chip" :class="p.status === 'recruiting' ? 'ok' : 'neutral'">{{ p.status === 'recruiting' ? '모집 중' : '마감' }}</span></td>
          <td class="pu-muted pu-small pu-num">{{ p.recruitUntil || '상시' }}</td>
          <td class="acts">
            <button class="pu-btn small" @click="$emit('edit', p)">수정</button>
            <button v-if="p.status !== 'recruiting'" class="pu-btn small" @click="$emit('status', p, 'recruiting')">모집 시작</button>
            <button v-else class="pu-btn small" @click="$emit('status', p, 'closed')">모집 마감</button>
          </td>
        </tr>
        <tr v-if="!rows.length"><td colspan="7" class="pu-muted empty">등록된 제품이 없습니다.</td></tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { formatWon } from '../../stores/member.js'

const props = defineProps({
  rows: { type: Array, default: () => [] },
  sortKey: { type: String, default: '' },
  sortDir: { type: Number, default: 1 },
})
defineEmits(['sort', 'edit', 'status'])

function arrow(k) {
  return props.sortKey === k ? (props.sortDir === 1 ? ' ▲' : ' ▼') : ''
}
function won(v) {
  return formatWon(v || 0)
}
</script>

<style scoped>
.twrap { overflow-x: auto; border: 1px solid var(--pu-line); border-radius: var(--pu-r); background: var(--pu-card); }
.tbl { width: 100%; border-collapse: collapse; font-size: 14px; min-width: 760px; }
.tbl th, .tbl td { padding: 10px 12px; text-align: left; border-bottom: 1px solid var(--pu-line-soft); white-space: nowrap; }
.tbl tr:last-child td { border-bottom: none; }
.tbl th button { border: none; background: none; font: inherit; font-weight: 800; color: var(--pu-soft); cursor: pointer; padding: 0; }
.tbl th.r, .tbl td.r { text-align: right; }
.tbl .acts { display: flex; gap: 6px; }
.tbl .empty { text-align: center; padding: 28px 12px; }
</style>
