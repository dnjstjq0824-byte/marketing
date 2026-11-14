<template>
  <div class="new">
    <h1 class="page-title">새 작업</h1>
    <section class="card" aria-label="대상">
      <h2 class="section-title">무엇으로 만드나요?</h2>
      <div class="seg" role="tablist" aria-label="입력 종류">
        <button v-for="k in kinds" :key="k" role="tab" :aria-selected="kind === k" :class="{ on: kind === k }" @click="kind = k">{{ kindLabel(k) }}</button>
      </div>
      <input v-if="kind === 'url'" v-model="url" type="url" placeholder="https://상품-페이지-주소" aria-label="링크" />
      <input v-else-if="kind === 'topic'" v-model="topic" placeholder="주제 (예: 여름 신상 패딩)" aria-label="주제" maxlength="200" />
      <Dropzone v-else />
      <p v-if="errors.input" class="error" role="alert">{{ errors.input }}</p>
    </section>
    <section class="card" aria-label="채널과 버전">
      <h2 class="section-title">어디에 올리나요?</h2>
      <div class="chips">
        <button v-for="c in CHANNELS" :key="c.key" class="chip" :class="{ on: picked[c.key] }" :aria-pressed="!!picked[c.key]" @click="toggleChannel(c.key)">
          {{ c.label }}
        </button>
      </div>
      <div v-for="c in CHANNELS" :key="'cfg-' + c.key">
        <div v-if="picked[c.key]" class="chcfg">
          <strong>{{ c.label }}</strong>
          <label>플랫폼
            <select v-model="picked[c.key].platform">
              <option v-for="p in platformsFor(c.key)" :key="p" :value="p">{{ p }}</option>
            </select>
          </label>
          <div class="stepper" role="group" aria-label="버전 수">
            <button @click="bump(c.key, -1)" :disabled="picked[c.key].versions <= 1" aria-label="버전 줄이기">−</button>
            <span class="num">{{ picked[c.key].versions }}</span>
            <button @click="bump(c.key, 1)" :disabled="picked[c.key].versions >= 4" aria-label="버전 늘리기">＋</button>
          </div>
        </div>
      </div>
      <p v-if="errors.channels" class="error" role="alert">{{ errors.channels }}</p>
    </section>
    <section class="card" aria-label="사업과 옵션">
      <h2 class="section-title">사업과 옵션</h2>
      <label>사업 <input v-model="business" placeholder="비우면 공용" maxlength="80" /></label>
      <label class="check"><input v-model="useStart" type="checkbox" /> 발행 시작 시간 정하기</label>
      <input v-if="useStart" v-model="startAt" type="datetime-local" aria-label="발행 시작" />
      <label class="check"><input v-model="sponsored" type="checkbox" /> 제휴·광고입니다</label>
      <label v-if="sponsored">대가 표시 문구 <textarea v-model="disclosure" rows="2" placeholder="예: 이 게시물은 대가를 받고 작성됐습니다"></textarea></label>
      <p v-if="errors.affiliate" class="error" role="alert">{{ errors.affiliate }}</p>
    </section>
    <div class="submitbar">
      <button class="ghost" @click="router.back()">취소</button>
      <button class="primary" :disabled="busy" @click="askConfirm">{{ busy ? '만드는 중…' : '만들기' }}</button>
    </div>
    <ConfirmModal v-if="confirming" title="작업을 만들까요?" :message="confirmText" confirmLabel="만들기" @confirm="submit" @cancel="confirming = false" />
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch } from '../stores/auth.js'
import { toast } from '../stores/toast.js'
import { uploads } from '../stores/upload.js'
import ConfirmModal from '../components/ConfirmModal.vue'
import Dropzone from '../components/Dropzone.vue'

const router = useRouter()
const kinds = ['url', 'image', 'topic']
const CHANNELS = [
  { key: 'blog', label: '네이버 블로그' },
  { key: 'instagram', label: '인스타그램' },
  { key: 'shortform', label: '숏폼' },
  { key: 'cardnews', label: '카드뉴스' },
]
const PLATFORMS = {
  blog: ['naver', 'tistory'],
  instagram: ['instagram'],
  shortform: ['youtube', 'tiktok', 'instagram'],
  cardnews: ['instagram'],
}

const kind = ref('url')
const url = ref('')
const topic = ref('')
const picked = ref({})
const business = ref('')
const useStart = ref(false)
const startAt = ref('')
const sponsored = ref(false)
const disclosure = ref('')
const busy = ref(false)
const confirming = ref(false)
const errors = ref({})

function kindLabel(k) {
  return k === 'url' ? '링크' : k === 'image' ? '사진·파일' : '주제'
}

function platformsFor(key) {
  return PLATFORMS[key] || ['naver']
}

function toggleChannel(key) {
  if (picked.value[key]) {
    delete picked.value[key]
    return
  }
  picked.value[key] = { platform: platformsFor(key)[0], versions: 1 }
}

function bump(key, d) {
  const c = picked.value[key]
  if (!c) return
  c.versions = Math.min(4, Math.max(1, c.versions + d))
}

const doneMedia = computed(() => uploads.filter((r) => r.state === 'done' && r.mediaId).map((r) => r.mediaId))
const uploading = computed(() => uploads.some((r) => r.state === 'queued' || r.state === 'active'))

const confirmText = computed(() => {
  const n = Object.keys(picked.value).length
  return `채널 ${n}개로 작업을 만듭니다.`
})

function validate() {
  const e = {}
  if (kind.value === 'url' && !url.value.trim()) e.input = '링크를 입력하세요.'
  if (kind.value === 'topic' && !topic.value.trim()) e.input = '주제를 입력하세요.'
  if (kind.value === 'image') {
    if (uploading.value) e.input = '파일이 올라가는 중입니다. 잠시 기다리세요.'
    else if (!doneMedia.value.length) e.input = '파일을 올리세요.'
  }
  if (!Object.keys(picked.value).length) e.channels = '채널을 하나 이상 고르세요.'
  if (sponsored.value && !disclosure.value.trim()) e.affiliate = '대가 표시 문구를 입력하세요.'
  errors.value = e
  return !Object.keys(e).length
}

function askConfirm() {
  if (!validate()) return
  confirming.value = true
}

async function submit() {
  confirming.value = false
  if (busy.value) return
  busy.value = true
  try {
    const inputs = []
    if (kind.value === 'url') inputs.push({ kind: 'url', url: url.value.trim() })
    else if (kind.value === 'topic') inputs.push({ kind: 'topic', text: topic.value.trim() })
    else inputs.push({ kind: 'image', mediaIds: doneMedia.value })
    const body = {
      inputs,
      channels: Object.entries(picked.value).map(([channel, c]) => ({ channel, platform: c.platform, versions: c.versions })),
      business: business.value.trim(),
      startAt: useStart.value ? startAt.value : '',
      affiliate: { isSponsored: sponsored.value, disclosure: disclosure.value.trim() },
      team: '',
    }
    const data = await apiFetch('/api/works', { method: 'POST', body: JSON.stringify(body) })
    toast(`작업 #${data.id}을 만들었습니다`)
    router.push(`/works/${data.id}`)
  } catch (e) {
    toast(e.message || '만들기 실패', 'error')
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.new { display: flex; flex-direction: column; gap: var(--s4); max-width: 720px; }
.card {
  background: var(--panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-lg);
  padding: var(--s5);
  box-shadow: var(--shadow);
  display: flex;
  flex-direction: column;
  gap: var(--s3);
}
.seg { display: flex; gap: var(--s2); }
.seg button {
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
  border-radius: 999px;
  padding: var(--s2) var(--s4);
}
.seg button.on { background: var(--accent); border-color: var(--accent); color: #fff; font-weight: 700; }
input, select, textarea {
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: var(--s3);
  background: var(--panel);
  width: 100%;
}
label { display: flex; flex-direction: column; gap: var(--s2); font-weight: 700; }
label.check { flex-direction: row; align-items: center; font-weight: 400; }
label.check input { width: auto; }
.chips { display: flex; flex-wrap: wrap; gap: var(--s2); }
.chip { border: 1px solid var(--line); background: var(--panel); color: var(--ink); border-radius: 999px; padding: var(--s2) var(--s4); }
.chip.on { background: var(--accent-soft); border-color: var(--accent); font-weight: 700; }
.chcfg { display: flex; align-items: center; gap: var(--s4); padding: var(--s3); background: var(--soft); border-radius: var(--r-md); }
.chcfg strong { flex: 1; }
.chcfg label { flex: 1; }
.stepper { display: flex; align-items: center; gap: var(--s2); }
.stepper button {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  border: 1px solid var(--line);
  background: var(--panel);
  color: var(--ink);
  font-size: 18px;
  line-height: 1;
}
.stepper button:disabled { opacity: 0.4; }
.error { color: var(--warn); }
.submitbar { display: flex; gap: var(--s3); justify-content: flex-end; position: sticky; bottom: 0; padding: var(--s3) 0; background: var(--bg); }
.submitbar button { border-radius: var(--r-md); padding: var(--s3) var(--s5); font-weight: 700; }
.submitbar .primary { background: var(--accent); color: #fff; border: none; }
.submitbar .primary:disabled { opacity: 0.6; }
.submitbar .ghost { border: 1px solid var(--line); background: var(--panel); }
@media (max-width: 900px) {
  .chcfg { flex-wrap: wrap; }
}
</style>
