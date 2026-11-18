<template>
  <div class="pu-tablewrap">
    <table class="pu-table">
      <thead><tr><th>코드</th><th>사용</th><th>발급자</th><th>상태</th><th></th></tr></thead>
      <tbody>
        <tr v-for="r in rows" :key="r.id">
          <td class="mono">{{ r.code }}</td>
          <td class="num">{{ r.used }}/{{ r.maxUses }}</td>
          <td>{{ r.createdBy || '-' }}</td>
          <td>{{ r.revoked ? '중단' : (r.used >= r.maxUses ? '소진' : '사용중') }}</td>
          <td><button v-if="!r.revoked && r.used < r.maxUses" class="pu-btn small" @click="$emit('revoke', r)">중단</button></td>
        </tr>
        <tr v-if="!rows.length"><td colspan="5" class="pu-muted">발급된 코드가 없습니다.</td></tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
defineProps({ rows: { type: Array, default: () => [] } })
defineEmits(['revoke'])
</script>

<style scoped>
.mono { font-family: ui-monospace, monospace; font-size: 13px; }
.num { font-variant-numeric: tabular-nums; }
</style>
