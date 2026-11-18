<template>
  <div class="twrap">
    <table class="tbl">
      <thead><tr>
        <th><button @click="$emit('sort', 'product')">제품{{ arrow('product') }}</button></th>
        <th><button @click="$emit('sort', 'memberNickname')">신청자{{ arrow('memberNickname') }}</button></th>
        <th><button @click="$emit('sort', 'status')">상태{{ arrow('status') }}</button></th>
        <th class="r">합의 수수료</th>
        <th class="r">확인 주문</th>
        <th>처리</th>
      </tr></thead>
      <tbody>
        <tr v-for="a in rows" :key="a.id">
          <td><strong>{{ a.product }}</strong><br /><span class="pu-muted pu-small">{{ (a.channels || '').slice(0, 40) }}</span></td>
          <td>{{ a.memberNickname }}<br /><span class="pu-muted pu-small">{{ a.memberEmail }}</span></td>
          <td><span class="pu-chip" :class="chip(a.status)">{{ statusText(a.status) }}</span></td>
          <td class="r pu-num">{{ won(a.feeAgreed) }}</td>
          <td class="r pu-num">{{ a.confirmedOrders || 0 }}</td>
          <td class="acts">
            <template v-if="a.status === 'pending'">
              <button class="pu-btn small pu-btn-primary" @click="$emit('approve', a)">승인</button>
              <input v-model="reasons[a.id]" class="mini" placeholder="반려 사유" />
              <button class="pu-btn small" @click="$emit('reject', a, reasons[a.id] || '')">반려</button>
            </template>
            <template v-else-if="a.status === 'approved'">
              <input v-model.number="counts[a.id]" class="mini num" type="number" min="0" />
              <button class="pu-btn small" @click="$emit('confirm', a, counts[a.id] || 0)">확인 저장</button>
            </template>
            <span v-else class="pu-muted pu-small">{{ a.reason || '반려됨' }}</span>
          </td>
        </tr>
        <tr v-if="!rows.length"><td colspan="6" class="pu-muted empty">신청 내역이 없습니다.</td></tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { formatWon } from '../../stores/member.js'

const props = defineProps({
  rows: { type: Array, default: () => [] },
  sortKey: { type: String, default: '' },
  sortDir: { type: Number, default: 1 },
})
defineEmits(['sort', 'approve', 'reject', 'confirm'])

const reasons = ref({})
const counts = ref(Object.fromEntries(props.rows.map((a) => [a.id, a.confirmedOrders || 0])))

function arrow(k) {
  return props.sortKey === k ? (props.sortDir === 1 ? ' ▲' : ' ▼') : ''
}
function chip(s) {
  return s === 'approved' ? 'ok' : s === 'rejected' ? 'warn' : 'hold'
}
function statusText(s) {
  return s === 'approved' ? '승인' : s === 'rejected' ? '반려' : '승인 대기'
}
function won(v) {
  return formatWon(v || 0)
}
</script>

<style scoped>
.twrap { overflow-x: auto; border: 1px solid var(--pu-line); border-radius: var(--pu-r); background: var(--pu-card); }
.tbl { width: 100%; border-collapse: collapse; font-size: 14px; min-width: 820px; }
.tbl th, .tbl td { padding: 10px 12px; text-align: left; border-bottom: 1px solid var(--pu-line-soft); vertical-align: top; }
.tbl tr:last-child td { border-bottom: none; }
.tbl th button { border: none; background: none; font: inherit; font-weight: 800; color: var(--pu-soft); cursor: pointer; padding: 0; }
.tbl th.r, .tbl td.r { text-align: right; }
.tbl .acts { min-width: 220px; }
.tbl .acts > * { margin: 0 6px 6px 0; vertical-align: middle; }
.mini { border: 1px solid var(--pu-line); border-radius: 10px; padding: 6px 8px; font-size: 13px; font-family: inherit; color: var(--pu-ink); background: var(--pu-card); width: 130px; }
.mini.num { width: 76px; }
.tbl .empty { text-align: center; padding: 28px 12px; }
</style>
