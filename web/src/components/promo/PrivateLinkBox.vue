<template>
  <div class="plinkbox">
    <p v-if="url" class="pu-small plink">{{ url }}</p>
    <p class="pu-rowline plinkbtns">
      <button v-if="!url" class="pu-btn small" :disabled="busy" @click="open">링크 보기</button>
      <button v-else class="pu-btn small" @click="copy">복사</button>
    </p>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { promoFetch } from '../../stores/member.js'
import { toast } from '../../stores/toast.js'

const props = defineProps({ appId: { type: [String, Number], required: true } })
const url = ref('')
const busy = ref(false)

async function open() {
  busy.value = true
  try {
    const data = await promoFetch(`/api/promo/applications/${props.appId}/private-link`)
    url.value = data.url || ''
  } catch (e) {
    toast(e.message || '실패했습니다', 'error')
  } finally {
    busy.value = false
  }
}

async function copy() {
  try {
    await navigator.clipboard.writeText(url.value)
    toast('복사했습니다')
  } catch {
    toast('복사에 실패했습니다', 'error')
  }
}
</script>

<style scoped>
.plink { word-break: break-all; margin: 0 0 8px; }
.plinkbtns { margin: 0; }
</style>
