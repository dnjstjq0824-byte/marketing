<template>
  <div class="detail">
    <RouterLink to="/works" class="back">← 작업 목록</RouterLink>
    <Skeleton v-if="loading" :rows="5" />
    <div v-else-if="loadError" class="card center">
      <p>불러오지 못했습니다.</p>
      <button @click="load">다시 시도</button>
    </div>
    <template v-else-if="data">
      <div class="head">
        <div>
          <h1 class="page-title">{{ data.product.name || data.product.sourceUrl }}</h1>
          <p class="muted small">{{ data.product.business || '공용 사업' }} · {{ data.product.sourceUrl }} · 맡은 팀: {{ teamName }}</p>
          <p class="muted small indrow">업종: {{ industryLabel }}
            <select v-model="industryDraft" class="indsel" aria-label="업종">
              <option value="general">일반(공통 금칙어만)</option>
              <option value="cosmetics">화장품</option>
              <option value="health">건강기능</option>
              <option value="food">식품</option>
            </select>
            <button v-if="industryDirty" class="ghost small" @click="saveIndustry">저장</button>
          </p>
        </div>
        <div class="btnrow">
          <button v-if="!allHeld" class="ghost" @click="setHeld(true)">멈추기</button>
          <button v-else class="ghost" @click="setHeld(false)">다시 진행</button>
          <button class="ghost danger" @click="confirmDelete = true">삭제</button>
        </div>
      </div>
      <StageBar :current="data.stage" />
      <section class="card" aria-label="게시물">
        <h2 class="section-title">게시물 {{ data.contents.length }}건</h2>
        <div v-if="!data.contents.length" class="muted">아직 만든 게시물이 없습니다. AI가 만드는 중입니다.</div>
        <div v-for="p in data.contents" :key="p.id" class="post">
          <div class="prow">
            <strong>{{ p.title || '(제목 없음)' }}</strong>
            <StatusChip :kind="stateKind(p)">{{ POST_STATE[p.status] || p.status }}</StatusChip>
            <StatusChip v-if="p.held" kind="hold">멈춤</StatusChip>
          </div>
          <p class="muted small">{{ p.channel }} · {{ p.targetPlatform || '플랫폼 미정' }} · {{ p.targetAccount || '계정 미정' }} · 조회 {{ p.views }}<span v-if="p.likes"> · 공감 {{ p.likes }}</span><span v-if="p.comments"> · 댓글 {{ p.comments }}</span></p>
          <p v-if="p.lastNote" class="muted small note">↩ {{ p.lastNote }}</p>
          <div class="prow btns">
            <button v-if="!p.held" class="ghost small" @click="holdPost(p.id, true)">멈추기</button>
            <button v-else class="ghost small" @click="holdPost(p.id, false)">다시 진행</button>
            <button class="ghost small" @click="regenPost(p.id)">다시 만들기</button>
            <button v-if="p.status === 'queued'" class="ghost small" @click="publishTarget = p">올라가 있음</button>
            <a v-if="p.url" class="ghost small link" :href="p.url" target="_blank" rel="noopener">열기</a>
          </div>
        </div>
      </section>
      <section v-if="data.images.length" class="card" aria-label="첨부">
        <h2 class="section-title">첨부 {{ data.images.length }}건</h2>
        <div class="tiles">
          <a v-for="m in data.images" :key="m.id" :href="m.url" target="_blank" rel="noopener" class="tile"><img :src="m.url" alt="" loading="lazy" /></a>
        </div>
      </section>
      <section class="card" aria-label="AI 대화 기록">
        <h2 class="section-title">AI와 주고받은 원문 보기</h2>
        <div v-if="!log.length" class="muted">기록이 없습니다.</div>
        <ol v-else class="log">
          <li v-for="(e, i) in log" :key="i">
            <StatusChip kind="info">{{ WORK_STAGE[e.stage] || AI_LOG_TYPE[e.type] || e.type }}</StatusChip>
            <span class="muted small">{{ e.account || e.worker }} · {{ shortTime(e.at) }}</span>
            <details v-if="e.prompt || e.reply" class="orig">
              <summary class="small">원문 펼치기</summary>
              <div v-if="e.prompt" class="small"><strong>보낸 내용</strong><pre class="pre">{{ e.prompt }}</pre></div>
              <div v-if="e.reply" class="small"><strong>받은 답장</strong><pre class="pre">{{ e.reply }}</pre></div>
            </details>
            <p v-else class="small">{{ e.summary || e.error }}</p>
          </li>
        </ol>
      </section>
      <section class="card" aria-label="사람 조작 기록">
        <h2 class="section-title">사람 조작 기록</h2>
        <div v-if="!audit.length" class="muted">조작 기록이 없습니다.</div>
        <ol v-else class="log">
          <li v-for="a in audit" :key="a.id">
            <span class="small"><strong>{{ AUDIT_ACTION[a.action] || a.action }}</strong> · {{ a.actor }} · {{ shortTime(a.createdAt) }}</span>
            <p v-if="a.detail" class="muted small">{{ a.detail }}</p>
          </li>
        </ol>
      </section>
    </template>
    <ConfirmModal v-if="confirmDelete" title="작업을 삭제할까요?" message="게시물·기록·첨부가 함께 지워집니다." confirmLabel="삭제" danger @confirm="remove" @cancel="confirmDelete = false" />
    <div v-if="publishTarget" class="modal-back" @click.self="publishTarget = null">
      <div class="modal" role="dialog" aria-label="발행 확인">
        <h3>올라가 있나요?</h3>
        <p class="muted small">실제 게시물 주소를 넣으세요.</p>
        <input v-model="publishUrl" placeholder="https://" aria-label="게시물 주소" />
        <p v-if="publishMsg" :class="publishOk ? 'banner okk' : 'error'">{{ publishMsg }}</p>
        <div class="btnrow">
          <button class="ghost" @click="publishTarget = null; publishMsg = ''">닫기</button>
          <button class="ghost" @click="denyPublish">안 올라감</button>
          <button @click="confirmPublish">올라가 있음</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiFetch } from '../stores/auth.js'
import { toast } from '../stores/toast.js'
import ConfirmModal from '../components/ConfirmModal.vue'
import Skeleton from '../components/Skeleton.vue'
import StageBar from '../components/StageBar.vue'
import StatusChip from '../components/StatusChip.vue'
import { AI_LOG_TYPE, AUDIT_ACTION, INDUSTRY, POST_STATE, WORK_STAGE } from '../stores/labels.js'

const route = useRoute()
const router = useRouter()
const id = route.params.id
const data = ref(null)
const log = ref([])
const audit = ref([])
const teams = ref([])
const loading = ref(true)
const loadError = ref(false)
const confirmDelete = ref(false)
const publishTarget = ref(null)
const publishUrl = ref('')
const publishMsg = ref('')
const publishOk = ref(false)
const industryDraft = ref('general')

const industryLabel = computed(() => INDUSTRY[data.value?.product?.industry || ''] || '일반')
const industryDirty = computed(() => {
  const draft = industryDraft.value === 'general' ? '' : industryDraft.value
  return draft !== (data.value?.product?.industry || '')
})

const allHeld = computed(() => (data.value?.contents || []).length > 0 && (data.value.contents.every((p) => p.held)))

const teamName = computed(() => {
  const t = data.value?.brief?.team
  if (!t) return '자동'
  const found = teams.value.find((x) => String(x.team.id) === String(t))
  return found ? found.team.name : '알 수 없는 팀'
})

function shortTime(iso) {
  return (iso || '').slice(5, 16).replace('T', ' ')
}

function stateKind(p) {
  if (p.held) return 'hold'
  if (p.status === 'published') return 'ok'
  if (p.status === 'queued' || p.status === 'approved') return 'info'
  return 'neutral'
}

async function load() {
  loadError.value = false
  try {
    const [d, l, t, a] = await Promise.all([
      apiFetch(`/api/works/${id}`),
      apiFetch(`/api/works/${id}/ai-log`),
      apiFetch('/api/teams').catch(() => ({ items: [] })),
      apiFetch(`/api/works/${id}/audit`).catch(() => ({ items: [] })),
    ])
    data.value = d
    log.value = l.items || []
    teams.value = t.items || []
    audit.value = a.items || []
    industryDraft.value = d?.product?.industry || 'general'
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

async function saveIndustry() {
  const p = data.value?.product
  if (!p) return
  try {
    await apiFetch(`/api/products/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({
        name: p.name || '', sourceUrl: p.sourceUrl || '',
        exitType: p.exitType || '', priority: p.priority || 0,
        business: p.business || '', industry: industryDraft.value,
      }),
    })
    toast('업종을 저장했습니다')
    await load()
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

async function setHeld(held) {
  try {
    await apiFetch(`/api/works/${id}/${held ? 'pause' : 'resume'}`, { method: 'POST' })
    toast(held ? '멈췄습니다' : '다시 진행합니다')
    await load()
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

async function remove() {
  confirmDelete.value = false
  try {
    await apiFetch(`/api/works/${id}`, { method: 'DELETE' })
    toast('삭제했습니다')
    router.push('/works')
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

async function holdPost(postId, held) {
  try {
    await apiFetch(`/api/works/${id}/posts/${postId}/hold`, { method: 'POST', body: JSON.stringify({ held }) })
    await load()
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

async function regenPost(postId) {
  try {
    await apiFetch(`/api/works/${id}/posts:generate`, { method: 'POST', body: JSON.stringify({ postId }) })
    toast('다시 만듭니다')
    await load()
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

async function confirmPublish() {
  publishMsg.value = ''
  try {
    await apiFetch(`/api/works/${id}/posts/${publishTarget.value.id}/confirm-published`, { method: 'POST', body: JSON.stringify({ published: true, url: publishUrl.value }) })
    publishOk.value = true
    publishMsg.value = '기록했습니다.'
    publishUrl.value = ''
    await load()
  } catch (e) {
    publishOk.value = false
    publishMsg.value = e.message
  }
}

async function denyPublish() {
  publishMsg.value = ''
  try {
    await apiFetch(`/api/works/${id}/posts/${publishTarget.value.id}/confirm-published`, { method: 'POST', body: JSON.stringify({ published: false }) })
    publishTarget.value = null
    publishUrl.value = ''
    toast('다음 발행에서 다시 시도합니다')
    await load()
  } catch (e) {
    publishOk.value = false
    publishMsg.value = e.message
  }
}

function onKey(e) {
  if (e.key === 'Escape') {
    publishTarget.value = null
    publishMsg.value = ''
  }
}

watch(publishTarget, (open) => {
  if (open) window.addEventListener('keydown', onKey)
  else window.removeEventListener('keydown', onKey)
})
onUnmounted(() => window.removeEventListener('keydown', onKey))

onMounted(load)
</script>

<style scoped>
.detail { display: flex; flex-direction: column; gap: var(--s4); }
.back { font-size: 13px; }
.indrow { display: flex; align-items: center; gap: var(--s2); flex-wrap: wrap; }
.indsel { border: 1px solid var(--line); border-radius: var(--r-md); padding: var(--s1) var(--s2); background: var(--panel); }
.head { display: flex; align-items: flex-start; gap: var(--s4); }
.head > div:first-child { flex: 1; min-width: 0; }
.card {
  background: var(--panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-lg);
  padding: var(--s4);
  box-shadow: var(--shadow);
  display: flex;
  flex-direction: column;
  gap: var(--s3);
}
.center { text-align: center; }
.center button {
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: var(--r-md);
  padding: var(--s2) var(--s4);
}
.btnrow { display: flex; gap: var(--s2); }
.btnrow button { border-radius: var(--r-md); padding: var(--s2) var(--s4); border: 1px solid var(--line); background: var(--panel); color: var(--ink); }
.btnrow button.danger { background: var(--warn); border-color: var(--warn); color: #fff; }
.ghost { border: 1px solid var(--line); background: var(--panel); color: var(--ink); border-radius: var(--r-md); }
.small { font-size: 13px; padding: var(--s1) var(--s2); }
.post { border-top: 1px solid var(--line-soft); padding-top: var(--s3); display: flex; flex-direction: column; gap: var(--s2); }
.prow { display: flex; align-items: center; gap: var(--s2); flex-wrap: wrap; }
.btns { gap: var(--s2); }
.link { text-decoration: none; display: inline-block; }
.tiles { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--s2); }
.tile { display: block; aspect-ratio: 1; border-radius: var(--r-md); overflow: hidden; background: var(--soft); border: 1px solid var(--line-soft); }
.tile img { width: 100%; height: 100%; object-fit: cover; }
.log { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--s3); }
.log li { display: flex; flex-direction: column; gap: var(--s1); border-top: 1px solid var(--line-soft); padding-top: var(--s2); }
.log p { margin: 0; overflow: hidden; text-overflow: ellipsis; }
.orig summary { cursor: pointer; color: var(--accent-dark); }
.pre { white-space: pre-wrap; word-break: break-word; background: var(--soft); border: 1px solid var(--line-soft); border-radius: var(--r-md); padding: var(--s2); margin: var(--s1) 0 0; max-height: 320px; overflow: auto; font-size: 12px; line-height: 1.6; }
.modal-back { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.4); z-index: 150; display: flex; align-items: center; justify-content: center; padding: var(--s4); }
.modal { background: var(--panel); border-radius: var(--r-lg); box-shadow: var(--shadow-lg); padding: var(--s5); max-width: 440px; width: 100%; display: flex; flex-direction: column; gap: var(--s3); }
.modal input { border: 1px solid var(--line); border-radius: var(--r-md); padding: var(--s3); background: var(--panel); width: 100%; }
.modal h3 { margin: 0; }
.error { color: var(--warn); }
@media (min-width: 1400px) {
  .detail { display: grid; grid-template-columns: 7fr 5fr; align-items: start; }
  .detail > .back, .detail > .head, .detail > :deep(.stages) { grid-column: 1 / -1; }
  .detail > section[aria-label="게시물"] { grid-column: 1; }
  .detail > section[aria-label="첨부"] { grid-column: 1; }
  .detail > section[aria-label="AI 대화 기록"] { grid-column: 2; grid-row: span 2; }
}
</style>
