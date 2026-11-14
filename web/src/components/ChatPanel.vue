<template>
  <div class="panel" role="dialog" :aria-label="`${name} 대화`">
    <div class="phead">
      <strong>{{ name }}</strong>
      <span class="sp"></span>
      <button class="ghost small" @click="newChat" :disabled="busy">새 대화</button>
      <button class="ghost small" @click="$emit('close')" aria-label="닫기">×</button>
    </div>
    <div ref="scroll" class="msgs">
      <div v-if="!items.length && !loading" class="muted small center">첫 메시지를 보내보세요.</div>
      <div v-for="m in items" :key="m.id" class="msg" :class="m.role">
        <p>{{ m.text }}</p>
      </div>
      <div v-if="busy" class="msg assistant typing"><p aria-label="답변 중">···</p></div>
    </div>
    <form class="composer" @submit.prevent="send">
      <textarea ref="box" v-model="text" rows="2" placeholder="메시지 (Shift+Enter 줄바꿈)" aria-label="메시지"
        @keydown.enter.exact.prevent="send" :disabled="busy || !canChat" />
      <button type="submit" :disabled="busy || !canChat || !text.trim()">보내기</button>
    </form>
    <p v-if="!canChat" class="muted small center">연결된 계정에서만 대화할 수 있습니다.</p>
    <p v-if="msg" :class="ok ? 'muted small center' : 'error center'">{{ msg }}</p>
  </div>
</template>

<script setup>
import { nextTick, onMounted, ref, watch } from 'vue'
import { apiFetch } from '../stores/auth.js'

const props = defineProps({
  accountId: { type: Number, required: true },
  name: { type: String, default: '' },
  canChat: { type: Boolean, default: true },
})
defineEmits(['close'])

const items = ref([])
const text = ref('')
const busy = ref(false)
const loading = ref(true)
const msg = ref('')
const ok = ref(false)
const scroll = ref(null)
const box = ref(null)

function scrollDown() {
  nextTick(() => {
    const el = scroll.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

async function load() {
  try {
    const data = await apiFetch(`/api/chat/history?accountId=${props.accountId}&limit=50`)
    items.value = data.items || []
    scrollDown()
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
}

async function poll(afterId) {
  for (let i = 0; i < 24; i++) {
    await new Promise((r) => setTimeout(r, 5000))
    try {
      const st = await apiFetch(`/api/chat/pending?afterId=${afterId}`)
      if (!st.done) continue
      return st
    } catch (e) {
      return { done: true, failed: true, message: e.message }
    }
  }
  return { done: true, failed: true, message: '시간 초과' }
}

async function send() {
  const message = text.value.trim()
  if (!message || busy.value || !props.canChat) return
  busy.value = true
  msg.value = ''
  text.value = ''
  items.value.push({ id: 't-' + Date.now(), role: 'user', text: message })
  scrollDown()
  try {
    const data = await apiFetch('/api/chat/send', {
      method: 'POST',
      body: JSON.stringify({ accountId: props.accountId, message }),
    })
    if (data.reply !== undefined) {
      items.value.push({ id: 'r-' + Date.now(), role: 'assistant', text: data.reply })
    } else if (data.pending) {
      const st = await poll(data.afterId)
      if (st.failed) {
        msg.value = st.message || '실패'
        ok.value = false
      } else {
        items.value.push({ id: 'r-' + Date.now(), role: 'assistant', text: st.reply })
      }
    }
    scrollDown()
  } catch (e) {
    msg.value = e.message || '전송 실패'
    ok.value = false
  } finally {
    busy.value = false
    box.value?.focus()
  }
}

async function newChat() {
  if (busy.value) return
  try {
    await apiFetch('/api/chat/threads/reset', { method: 'POST', body: JSON.stringify({ accountId: props.accountId }) })
    items.value = []
    msg.value = '새 대화를 시작합니다.'
    ok.value = true
  } catch (e) {
    msg.value = e.message || '실패'
    ok.value = false
  }
}

watch(() => props.accountId, () => {
  items.value = []
  loading.value = true
  load()
})

onMounted(() => {
  load()
  box.value?.focus()
})
</script>

<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--panel);
}
.phead { display: flex; align-items: center; gap: var(--s2); padding: var(--s3) var(--s4); border-bottom: 1px solid var(--line-soft); }
.phead .sp { flex: 1; }
.msgs { flex: 1; overflow-y: auto; padding: var(--s4); display: flex; flex-direction: column; gap: var(--s2); }
.msg { max-width: 85%; }
.msg p { margin: 0; padding: var(--s2) var(--s3); border-radius: var(--r-md); white-space: pre-wrap; word-break: break-word; }
.msg.user { align-self: flex-end; }
.msg.user p { background: var(--accent); color: #fff; }
.msg.assistant { align-self: flex-start; }
.msg.assistant p { background: var(--soft); }
.msg.typing p { color: var(--muted); }
.composer { display: flex; gap: var(--s2); padding: var(--s3) var(--s4); border-top: 1px solid var(--line-soft); }
.composer textarea {
  flex: 1;
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: var(--s2) var(--s3);
  background: var(--panel);
  color: var(--ink);
  resize: none;
}
.composer button {
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: var(--r-md);
  padding: var(--s2) var(--s4);
  font-weight: 700;
}
.composer button:disabled { opacity: 0.6; }
.center { text-align: center; }
.error { color: var(--warn); text-align: center; }
.ghost { border: 1px solid var(--line); background: var(--panel); color: var(--ink); border-radius: var(--r-md); }
.small { font-size: 13px; padding: var(--s1) var(--s2); }
</style>
