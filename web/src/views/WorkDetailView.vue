<template>
  <div class="detail">
    <DetailCrumbs :title="data?.product?.name || data?.product?.sourceUrl || '작업'" />
    <Skeleton v-if="loading" :rows="5" />
    <div v-else-if="loadError" class="card center">
      <p>작업 정보를 불러오지 못했습니다.</p>
      <button @click="load">다시 시도</button>
    </div>

    <template v-else-if="data">
      <!-- Header Summary Card -->
      <section class="card head-card">
        <div class="head">
          <div class="head-info">
            <h1 class="page-title">{{ data.product.name || data.product.sourceUrl }}</h1>
            <p class="muted small meta-line">
              <span>{{ data.product.business || '공용 사업' }}</span>
              <span v-if="data.product.sourceUrl"> · <a :href="data.product.sourceUrl" target="_blank" rel="noopener">{{ shortUrl(data.product.sourceUrl) }} ↗</a></span>
              <span> · 담당 팀: <b>{{ teamName }}</b></span>
            </p>
            <div class="indrow">
              <span class="muted micro">업종 안전망</span>
              <select v-model="industryDraft" class="indsel" aria-label="업종">
                <option value="general">일반 (공통 금칙어)</option>
                <option value="cosmetics">화장품</option>
                <option value="health">건강기능</option>
                <option value="food">식품</option>
              </select>
              <button v-if="industryDirty" class="small" @click="saveIndustry">적용</button>
            </div>
          </div>
          <div class="btnrow">
            <button v-if="!allHeld" class="ghost" @click="setHeld(true)">전체 멈춤</button>
            <button v-else class="ghost" @click="setHeld(false)">다시 진행</button>
            <button class="ghost danger" @click="confirmDelete = true">삭제</button>
          </div>
        </div>
        <div class="stage-wrap">
          <StageBar :current="data.stage" />
        </div>
      </section>

      <!-- Main 2-Column Workspace -->
      <div class="workspace">
        <div class="col-left">
          <!-- Posts Section -->
          <section class="card" aria-label="게시물">
            <div class="sect-head">
              <h2 class="section-title">생성된 게시물 <span class="muted small num">({{ data.contents.length }})</span></h2>
              <span class="muted micro">읽기 전용 · 전문 보기에서 본문 확인</span>
            </div>
            <div v-if="!data.contents.length" class="empty">
              <strong>아직 생성된 게시물이 없습니다</strong>
              <span class="small">AI 팀이 분석·전략을 마치고 초안을 작성 중입니다.</span>
            </div>
            <div v-for="p in data.contents" :key="p.id" class="post">
              <div class="prow">
                <strong class="ptitle">{{ p.title || '(제목 생성 중)' }}</strong>
                <StatusChip :kind="stateKind(p)">{{ POST_STATE[p.status] || p.status }}</StatusChip>
                <StatusChip v-if="p.held" kind="hold">멈춤</StatusChip>
              </div>
              <p class="muted small pmeta">
                <span>{{ platformLabel(p.targetPlatform || p.channel) }}</span>
                <span v-if="p.targetAccount"> · {{ p.targetAccount }}</span>
                <span v-if="p.scheduledAt && p.status === 'queued'"> · 예약 {{ shortTime(p.scheduledAt) }}</span>
                <span class="num"> · 조회 {{ p.views }}<template v-if="p.likes"> · 공감 {{ p.likes }}</template><template v-if="p.comments"> · 댓글 {{ p.comments }}</template></span>
              </p>
              <p v-if="p.lastNote" class="note-box small">↩ 검수 피드백: {{ p.lastNote }}</p>
              <div class="prow btns">
                <button class="ghost small" @click="openPreview(p)">전문 보기</button>
                <a v-if="p.url" class="ext-btn small" :href="p.url" target="_blank" rel="noopener">게시물 열기 ↗</a>
                <button v-if="!p.held && p.status !== 'published'" class="ghost small" @click="holdPost(p.id, true)">멈추기</button>
                <button v-else-if="p.held" class="ghost small" @click="holdPost(p.id, false)">다시 진행</button>
                <button v-if="p.status !== 'published'" class="ghost small" @click="regenPost(p.id)">다시 만들기</button>
                <button v-if="p.status === 'queued'" class="ghost small" @click="publishTarget = p">발행 여부 확인</button>
              </div>
            </div>
          </section>

          <!-- AI Reading & Strategy Insights -->
          <section v-if="data.reading || data.strategy" class="card" aria-label="AI 분석 및 전략">
            <h2 class="section-title">AI 분석 · 전략 요약</h2>
            <details v-if="data.reading" class="insight" open>
              <summary>상품 판독 결과</summary>
              <dl class="specs">
                <template v-if="data.reading.productName"><dt>상품명</dt><dd>{{ data.reading.productName }}</dd></template>
                <template v-if="data.reading.price"><dt>가격</dt><dd>{{ data.reading.price }}</dd></template>
                <template v-if="data.reading.target"><dt>타깃</dt><dd>{{ data.reading.target }}</dd></template>
                <template v-if="data.reading.usp"><dt>핵심 강점</dt><dd>{{ data.reading.usp }}</dd></template>
                <template v-if="data.reading.summary"><dt>요약</dt><dd>{{ data.reading.summary }}</dd></template>
                <template v-if="data.reading.unknowns"><dt>미확인 항목</dt><dd class="muted">{{ data.reading.unknowns }}</dd></template>
              </dl>
            </details>
            <details v-if="data.strategy" class="insight" open>
              <summary>마케팅 전략</summary>
              <dl class="specs">
                <template v-if="data.strategy.angle"><dt>핵심 앵글</dt><dd>{{ data.strategy.angle }}</dd></template>
                <template v-if="data.strategy.keyMessages"><dt>핵심 메시지</dt><dd>{{ data.strategy.keyMessages }}</dd></template>
                <template v-if="data.strategy.channels"><dt>채널 전략</dt><dd>{{ data.strategy.channels }}</dd></template>
              </dl>
            </details>
          </section>

          <!-- Attached / Generated Media -->
          <section v-if="data.images.length" class="card" aria-label="미디어">
            <h2 class="section-title">미디어 <span class="muted small num">({{ data.images.length }})</span></h2>
            <div class="tiles">
              <button v-for="m in data.images" :key="m.id" class="tile" @click="openMedia(m)" aria-label="미디어 크게 보기">
                <AuthImg :src="m.url" alt="" />
              </button>
            </div>
          </section>
        </div>

        <!-- Right Column: Timeline -->
        <section class="card col-right" aria-label="진행 기록">
          <h2 class="section-title">AI 진행 기록</h2>
          <WorkTimeline :items="timeline" />
        </section>
      </div>
    </template>

    <!-- Full Content Preview Slide-over Drawer (UI_DESIGN S3-D) -->
    <div v-if="previewOpen" class="drawer-back" @click.self="previewOpen = false">
      <div class="preview-drawer" role="dialog" aria-label="게시물 전문 보기">
        <div class="pd-head">
          <div>
            <h3 class="pd-title">{{ previewDetail?.title || previewPost?.title || '(제목 없음)' }}</h3>
            <p class="muted micro">{{ platformLabel(previewPost?.targetPlatform || previewPost?.channel) }} · {{ previewPost?.targetAccount || '계정 미정' }}</p>
          </div>
          <div class="btnrow">
            <button class="ghost small" :disabled="!previewDetail?.body" @click="copyBody">본문 복사</button>
            <button class="ghost small" @click="previewOpen = false" aria-label="닫기">✕</button>
          </div>
        </div>
        <div class="pd-body">
          <Skeleton v-if="previewLoading" :rows="4" />
          <template v-else-if="previewDetail">
            <div v-if="previewDetail.disclosure" class="banner warn">{{ previewDetail.disclosure }}</div>
            <div v-if="previewDetail.aiLabel" class="banner info">AI 표시: {{ previewDetail.aiLabel }}</div>
            <article class="article-body">{{ previewDetail.body || '(본문이 아직 비어 있습니다)' }}</article>
            <div v-if="previewReviews.length" class="rev-sec">
              <h4 class="section-title">AI 검수 이력</h4>
              <ul class="rev-list">
                <li v-for="r in previewReviews" :key="r.id">
                  <div class="prow">
                    <StatusChip :kind="r.decision === 'approved' ? 'ok' : 'warn'">{{ r.decision === 'approved' ? '승인' : '반려' }}</StatusChip>
                    <strong class="small">{{ r.decider }}</strong>
                    <span class="muted micro num">{{ shortTime(r.createdAt) }}</span>
                  </div>
                  <p v-if="r.reason" class="small muted">{{ r.reason }}</p>
                </li>
              </ul>
            </div>
          </template>
          <p v-else class="muted small">게시물 상세를 불러오지 못했습니다.</p>
        </div>
      </div>
    </div>

    <ConfirmModal v-if="confirmDelete" title="작업을 삭제할까요?" message="게시된 글은 외부 채널에서 지워지지 않습니다. 콘솔 기록과 미디어만 삭제됩니다." confirmLabel="삭제" danger @confirm="remove" @cancel="confirmDelete = false" />

    <div v-if="publishTarget" class="modal-back" @click.self="publishTarget = null">
      <div class="modal" role="dialog" aria-label="발행 확인">
        <h3>실제로 올라갔나요?</h3>
        <p class="muted small">중복 게시를 막기 위해 실제 게시물 주소를 입력해 확정하거나, 안 올라갔으면 다음 주기에 다시 시도합니다.</p>
        <input v-model="publishUrl" placeholder="https://..." aria-label="게시물 주소" />
        <p v-if="publishMsg" :class="publishOk ? 'banner okk' : 'error'">{{ publishMsg }}</p>
        <div class="btnrow">
          <button class="ghost" @click="publishTarget = null; publishMsg = ''">닫기</button>
          <button class="ghost" @click="denyPublish">안 올라감</button>
          <button @click="confirmPublish">올라가 있음</button>
        </div>
      </div>
    </div>

    <Lightbox v-if="boxItem" :item="boxItem" :src="boxSrc" :hasPrev="false" :hasNext="false" @close="closeBox" />
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiFetch, auth } from '../stores/auth.js'
import { usePoll } from '../stores/poll.js'
import { toast } from '../stores/toast.js'
import AuthImg from '../components/AuthImg.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import Lightbox from '../components/Lightbox.vue'
import Skeleton from '../components/Skeleton.vue'
import StageBar from '../components/StageBar.vue'
import StatusChip from '../components/StatusChip.vue'
import DetailCrumbs from '../components/console/DetailCrumbs.vue'
import WorkTimeline from '../components/console/WorkTimeline.vue'
import { AI_LOG_TYPE, AUDIT_ACTION, INDUSTRY, label, PLATFORM, POST_STATE, WORK_STAGE } from '../stores/labels.js'

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

const previewOpen = ref(false)
const previewLoading = ref(false)
const previewPost = ref(null)
const previewDetail = ref(null)
const previewReviews = ref([])

const boxItem = ref(null)
const boxSrc = ref('')

const industryDirty = computed(() => {
  const draft = industryDraft.value === 'general' ? '' : industryDraft.value
  return draft !== (data.value?.product?.industry || '')
})
const allHeld = computed(() => (data.value?.contents || []).length > 0 && (data.value.contents.every((p) => p.held)))
const teamName = computed(() => {
  const t = data.value?.brief?.team
  if (!t) return '자동 배정'
  const found = teams.value.find((x) => String(x.team.id) === String(t))
  return found ? found.team.name : '기본 팀'
})

function platformLabel(p) { return label(PLATFORM, p, p || '미정') }
function shortTime(iso) { return (iso || '').slice(5, 16).replace('T', ' ') }
function shortUrl(u) { return u.length > 48 ? u.slice(0, 48) + '…' : u }

function stateKind(p) {
  if (p.held) return 'hold'
  if (p.status === 'published') return 'ok'
  if (p.status === 'failed') return 'warn'
  if (p.status === 'queued' || p.status === 'approved') return 'info'
  return 'neutral'
}

const timeline = computed(() => {
  const items = []
  for (const e of log.value) {
    const detail = [e.prompt && `보낸 내용\n${e.prompt}`, e.reply && `받은 답장\n${e.reply}`].filter(Boolean).join('\n\n')
    items.push({
      at: shortTime(e.at), raw: e.at || '',
      title: WORK_STAGE[e.stage] || AI_LOG_TYPE[e.type] || e.type || 'AI',
      note: [e.account || e.worker, e.summary || e.error].filter(Boolean).join(' · '),
      detail: detail || '',
      kind: e.error ? 'warn' : 'ai',
    })
  }
  for (const a of audit.value) {
    items.push({
      at: shortTime(a.createdAt), raw: a.createdAt || '',
      title: AUDIT_ACTION[a.action] || a.action,
      note: [a.actor, a.detail].filter(Boolean).join(' · '),
      detail: '', kind: 'human',
    })
  }
  return items.sort((a, b) => (b.raw < a.raw ? -1 : 1)).slice(0, 40)
})

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
    if (!industryDirty.value) industryDraft.value = d?.product?.industry || 'general'
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

async function openPreview(p) {
  previewPost.value = p
  previewDetail.value = null
  previewReviews.value = []
  previewOpen.value = true
  previewLoading.value = true
  try {
    const [det, rev] = await Promise.all([
      apiFetch(`/api/contents/${p.id}`),
      apiFetch(`/api/contents/${p.id}/reviews`).catch(() => ({ reviews: [] })),
    ])
    previewDetail.value = det.content || det
    previewReviews.value = rev.reviews || rev.items || []
  } catch {
    previewDetail.value = null
  } finally {
    previewLoading.value = false
  }
}

async function copyBody() {
  const txt = [previewDetail.value?.disclosure, previewDetail.value?.body, previewDetail.value?.aiLabel].filter(Boolean).join('\n\n')
  try {
    await navigator.clipboard.writeText(txt)
    toast('본문을 복사했습니다')
  } catch {
    toast('복사에 실패했습니다', 'error')
  }
}

async function openMedia(m) {
  boxItem.value = m
  boxSrc.value = ''
  try {
    const headers = {}
    if (auth.token) headers['Authorization'] = `Bearer ${auth.token}`
    const res = await fetch(m.url, { headers })
    if (res.ok) boxSrc.value = URL.createObjectURL(await res.blob())
  } catch { /* ignore */ }
}

function closeBox() {
  if (boxSrc.value) URL.revokeObjectURL(boxSrc.value)
  boxSrc.value = ''
  boxItem.value = null
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
        business: p.business || '', industry: industryDraft.value === 'general' ? '' : industryDraft.value,
      }),
    })
    toast('업종 안전망을 저장했습니다')
    await load()
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

async function setHeld(held) {
  try {
    await apiFetch(`/api/works/${id}/${held ? 'pause' : 'resume'}`, { method: 'POST' })
    toast(held ? '작업을 멈췄습니다' : '작업을 다시 진행합니다')
    await load()
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

async function remove() {
  confirmDelete.value = false
  try {
    await apiFetch(`/api/works/${id}`, { method: 'DELETE' })
    toast('작업을 삭제했습니다')
    router.push('/works')
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

async function holdPost(postId, held) {
  try {
    await apiFetch(`/api/works/${id}/posts/${postId}/hold`, { method: 'POST', body: JSON.stringify({ held }) })
    toast(held ? '게시물을 멈췄습니다' : '게시물을 다시 진행합니다')
    await load()
  } catch (e) {
    toast(e.message || '실패', 'error')
  }
}

async function regenPost(postId) {
  try {
    await apiFetch(`/api/works/${id}/posts:generate`, { method: 'POST', body: JSON.stringify({ postId }) })
    toast('재생성 작업을 등록했습니다')
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
    publishMsg.value = '발행 완료로 기록했습니다.'
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
    toast('다음 발행 주기에 다시 시도합니다')
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
    previewOpen.value = false
  }
}

const isActiveStage = () => {
  const st = data.value?.stage || ''
  return ['intake', 'analyze', 'research', 'strategy', 'generate', 'review', 'publish'].includes(st)
}
const poll = usePoll(load, { isBusy: isActiveStage })

onMounted(() => {
  poll.start()
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => {
  window.removeEventListener('keydown', onKey)
  closeBox()
})
</script>

<style scoped>
.detail { display: flex; flex-direction: column; gap: 18px; }
.head-card { display: flex; flex-direction: column; gap: 16px; }
.head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; flex-wrap: wrap; }
.head-info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 6px; }
.meta-line { overflow-wrap: anywhere; }
.indrow { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-top: 2px; }
.indsel { width: auto; padding: 4px 10px; font-size: 12.5px; }
.stage-wrap { padding-top: 12px; border-top: 1px solid var(--line-soft); }

.workspace { display: grid; grid-template-columns: 1.35fr 1fr; gap: 18px; align-items: start; }
.col-left { display: flex; flex-direction: column; gap: 18px; min-width: 0; }
.col-right { min-width: 0; display: flex; flex-direction: column; gap: 12px; }
.card { display: flex; flex-direction: column; gap: 12px; }

.post { border-top: 1px solid var(--line-soft); padding-top: 14px; display: flex; flex-direction: column; gap: 8px; }
.prow { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.ptitle { font-size: 15px; }
.pmeta { line-height: 1.5; }
.note-box { background: var(--hold-soft); color: var(--hold); padding: 8px 12px; border-radius: 8px; font-weight: 500; }
.ext-btn {
  display: inline-flex;
  align-items: center;
  padding: 5px 11px;
  border-radius: 7px;
  font-size: 12.5px;
  font-weight: 700;
  text-decoration: none;
  background: var(--accent-soft);
  color: var(--accent-dark);
}
.ext-btn:hover { background: var(--accent); color: var(--accent-ink); }

.insight { border: 1px solid var(--line-soft); border-radius: var(--r-md); padding: 12px 14px; background: var(--soft); }
.insight summary { font-weight: 700; font-size: 13.5px; cursor: pointer; color: var(--ink); }
.specs { display: grid; grid-template-columns: 92px 1fr; gap: 6px 12px; margin: 10px 0 0; font-size: 13px; }
.specs dt { color: var(--muted); font-weight: 600; }
.specs dd { margin: 0; color: var(--ink); overflow-wrap: anywhere; }

.tiles { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; }
.tile {
  padding: 0;
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  aspect-ratio: 1;
  overflow: hidden;
  background: var(--soft);
}
.tile img { width: 100%; height: 100%; object-fit: cover; }

.drawer-back { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.55); backdrop-filter: blur(2px); z-index: 150; display: flex; justify-content: flex-end; }
.preview-drawer {
  width: 540px;
  max-width: 94vw;
  height: 100vh;
  background: var(--panel);
  box-shadow: var(--shadow-lg);
  display: flex;
  flex-direction: column;
}
.pd-head {
  padding: 18px 20px;
  border-bottom: 1px solid var(--line);
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.pd-title { font-size: 16px; margin-bottom: 2px; }
.pd-body { padding: 20px; overflow-y: auto; flex: 1; display: flex; flex-direction: column; gap: 14px; }
.article-body {
  white-space: pre-wrap;
  line-height: 1.75;
  font-size: 14.5px;
  color: var(--ink);
  background: var(--soft);
  padding: 16px 18px;
  border-radius: var(--r-md);
  border: 1px solid var(--line-soft);
}
.rev-sec { display: flex; flex-direction: column; gap: 10px; padding-top: 12px; border-top: 1px solid var(--line-soft); }
.rev-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 10px; }
.rev-list li { display: flex; flex-direction: column; gap: 4px; padding: 10px 12px; background: var(--soft); border-radius: var(--r-sm); }
.center { text-align: center; display: flex; flex-direction: column; align-items: center; gap: 10px; }

@media (max-width: 1020px) {
  .workspace { grid-template-columns: 1fr; }
}
@media (max-width: 600px) {
  .tiles { grid-template-columns: repeat(2, 1fr); }
}
</style>
