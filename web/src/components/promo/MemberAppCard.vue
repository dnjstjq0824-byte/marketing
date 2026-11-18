<template>
  <div class="pu-card">
    <div class="pu-rowline">
      <strong>{{ app.product }}</strong>
      <span class="pu-chip" :class="chip">{{ statusText }}</span>
    </div>
    <p class="pu-muted pu-small pu-num">{{ formatFee(app.feeAgreed) }} · {{ (app.createdAt || '').slice(0, 10) }}</p>
    <p v-if="app.status === 'rejected' && app.reason" class="pu-small">반려 사유: {{ app.reason }}</p>
    <PrivateLinkBox v-if="app.status === 'approved'" :app-id="app.id" />
    <p v-else class="lockline"><span class="pu-chip neutral">승인 후 열림</span></p>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import PrivateLinkBox from './PrivateLinkBox.vue'
import { formatFee } from '../../stores/member.js'

const props = defineProps({ app: { type: Object, required: true } })
const statusText = computed(() =>
  props.app.status === 'approved' ? '승인' : props.app.status === 'rejected' ? '반려' : '승인 대기')
const chip = computed(() =>
  props.app.status === 'approved' ? 'ok' : props.app.status === 'rejected' ? 'warn' : 'hold')
</script>

<style scoped>
.lockline { margin-bottom: 0; }
</style>
