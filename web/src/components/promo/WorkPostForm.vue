<template>
  <div>
    <form class="pu-rowline" @submit.prevent="submit">
      <input v-model="url" placeholder="https:// 게시물 주소" aria-label="게시물 주소" class="pinput" />
      <button class="pu-btn small" :disabled="busy">등록</button>
    </form>
    <p v-if="msg" class="pu-error pu-small pmsg">{{ msg }}</p>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { promoFetch } from '../../stores/member.js'
import { toast } from '../../stores/toast.js'

const props = defineProps({ appId: { type: [String, Number], required: true } })
const emit = defineEmits(['posted'])
const url = ref('')
const msg = ref('')
const busy = ref(false)

async function submit() {
  msg.value = ''
  busy.value = true
  try {
    await promoFetch(`/api/promo/applications/${props.appId}/posts`, {
      method: 'POST',
      body: JSON.stringify({ url: url.value }),
    })
    url.value = ''
    toast('등록했습니다')
    emit('posted')
  } catch (e) {
    msg.value = e.message || '실패했습니다'
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.pinput {
  flex: 1;
  min-width: 200px;
  border: 1px solid var(--pu-line);
  border-radius: var(--pu-r-sm);
  padding: 8px 10px;
  font-size: 14px;
  font-family: inherit;
  color: var(--pu-ink);
  background: var(--pu-card);
}
.pmsg { margin-bottom: 0; }
</style>
