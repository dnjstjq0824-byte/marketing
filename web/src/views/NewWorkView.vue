<template>
  <div class="new">
    <div class="page-head">
      <div>
        <h1 class="page-title">새 작업</h1>
        <p class="muted small">상품 링크·사진·주제 중 하나만 넣으면 AI 팀이 분석부터 발행까지 자동으로 진행합니다.</p>
      </div>
      <span class="ctx-pill small">채널 <b class="num">{{ pickedCount }}</b>개 · {{ activeTeamLabel }}</span>
    </div>

    <div class="layout">
      <div class="col-main">
        <!-- Input Source Card -->
        <section class="card" aria-label="입력 대상">
          <h2 class="section-title">1. 무엇을 마케팅할까요?</h2>
          <div class="seg" role="tablist" aria-label="입력 종류">
            <button v-for="k in kinds" :key="k" type="button" role="tab" :aria-selected="kind === k" :class="{ on: kind === k }" @click="kind = k">
              {{ kindLabel(k) }}
            </button>
          </div>

          <div v-if="kind === 'url'" class="in-block">
            <input v-model.trim="url" type="url" placeholder="https://smartstore.naver.com/..." aria-label="상품 링크" autofocus />
            <p v-if="urlHint" class="detect ok small">✓ {{ urlHint }}</p>
            <p v-else class="muted micro">스마트스토어·쿠팡·자사몰 등 상품 상세 페이지 주소를 붙여넣으세요.</p>
          </div>

          <div v-else-if="kind === 'topic'" class="in-block">
            <textarea v-model="topic" rows="3" placeholder="예: 1인 가구 무선 핸디 청소기 고르는 법" aria-label="주제" maxlength="200"></textarea>
            <div class="topic-foot">
              <div class="examples">
                <button v-for="ex in TOPIC_EXAMPLES" :key="ex" type="button" class="ex-chip" @click="topic = ex">{{ ex }}</button>
              </div>
              <span class="muted micro num">{{ topic.length }}/200</span>
            </div>
          </div>

          <div v-else class="in-block">
            <Dropzone />
          </div>

          <FieldError :message="errors.input" :hint="hints.input" />

          <!-- Optional Media Attachment on URL / Topic tabs too (UI_DESIGN S2) -->
          <div v-if="kind !== 'image'" class="opt-media">
            <button type="button" class="ghost small" @click="showExtraMedia = !showExtraMedia">
              {{ showExtraMedia ? '▾ 참고 이미지·자료 닫기' : '＋ 참고 이미지·파일 첨부 (선택)' }}
              <span v-if="doneMedia.length" class="num">({{ doneMedia.length }})</span>
            </button>
            <div v-if="showExtraMedia" class="opt-drop">
              <Dropzone />
            </div>
          </div>
        </section>

        <!-- Target Channels Card -->
        <section class="card" aria-label="발행 채널과 버전">
          <div class="sect-head">
            <h2 class="section-title">2. 어디에 올릴까요?</h2>
            <span class="muted micro">연결된 채널이 자동 선택됩니다</span>
          </div>
          <div class="chan-grid">
            <div v-for="c in CHANNELS" :key="c.key" class="chan-card" :class="{ on: !!picked[c.key] }">
              <button type="button" class="chan-toggle" :aria-pressed="!!picked[c.key]" @click="toggleChannel(c.key)">
                <span class="chkbox" aria-hidden="true">{{ picked[c.key] ? '✓' : '' }}</span>
                <span class="chan-info">
                  <strong>{{ c.label }}</strong>
                  <small class="muted">{{ c.desc }}</small>
                </span>
                <span v-if="connectedCount(c.key) > 0" class="conn-badge num">연결 {{ connectedCount(c.key) }}</span>
              </button>
              <div v-if="picked[c.key]" class="chcfg">
                <label class="micro">플랫폼
                  <select v-model="picked[c.key].platform">
                    <option v-for="p in platformsFor(c.key)" :key="p" :value="p">{{ platformDisplay(p) }}</option>
                  </select>
                </label>
                <div class="ver-box">
                  <span class="micro muted">버전 수</span>
                  <div class="stepper" role="group" aria-label="버전 수">
                    <button type="button" class="ghost" @click="bump(c.key, -1)" :disabled="picked[c.key].versions <= 1" aria-label="버전 줄이기">−</button>
                    <span class="num">{{ picked[c.key].versions }}</span>
                    <button type="button" class="ghost" @click="bump(c.key, 1)" :disabled="picked[c.key].versions >= 10" aria-label="버전 늘리기">＋</button>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <FieldError :message="errors.channels" :hint="hints.channels" />
          <p v-if="totalConnectedChannels === 0" class="banner warn">
            연결된 발행 채널이 아직 없습니다. 콘텐츠는 먼저 생성되며, <RouterLink to="/connect">연결 화면</RouterLink>에서 채널을 붙이면 자동 발행됩니다.
          </p>
        </section>
      </div>

      <div class="col-side">
        <!-- Team & Options Card -->
        <section class="card" aria-label="팀과 옵션">
          <h2 class="section-title">3. 담당 팀 · 옵션</h2>
          <div class="field-stack">
            <label>사업 · 브랜드
              <input v-model.trim="business" placeholder="예: LUNO (비우면 공용)" maxlength="80" list="biz-presets" />
              <datalist id="biz-presets">
                <option v-for="b in businessOptions" :key="b" :value="b" />
              </datalist>
            </label>

            <label>맡길 팀
              <select v-model="team" :disabled="teamsLoading">
                <option value="">자동 (사업 전담팀 우선 배정)</option>
                <option v-for="t in teams" :key="t.team.id" :value="String(t.team.id)">{{ t.team.name }}</option>
              </select>
            </label>

            <div v-if="resolvedTeam" class="team-preview">
              <div class="tp-row">
                <strong>{{ resolvedTeam.team.name }}</strong>
                <StatusChip :kind="resolvedTeam.team.enabled && resolvedTeam.memberCount > 0 ? 'ok' : 'hold'">
                  {{ resolvedTeam.team.enabled ? (resolvedTeam.memberCount > 0 ? '가동 가능' : '에이전트 없음') : '멈춤' }}
                </StatusChip>
              </div>
              <p class="muted micro">
                {{ (resolvedTeam.businesses || []).length ? resolvedTeam.businesses.join(' · ') + ' 전담' : '전담 없음' }}
                · 에이전트 <b class="num">{{ resolvedTeam.memberCount }}</b>기
                · 진행 <b class="num">{{ resolvedTeam.stats?.runningWorks || 0 }}</b>건
              </p>
            </div>

            <div class="divider"></div>

            <label class="check-row"><input v-model="useStart" type="checkbox" /> 발행 시작 시각 예약</label>
            <input v-if="useStart" v-model="startAt" type="datetime-local" aria-label="발행 시작" />

            <label class="check-row"><input v-model="sponsored" type="checkbox" /> 제휴·협찬 상품입니다 (대가 표시 자동 삽입)</label>
            <div v-if="sponsored" class="in-block">
              <label>대가 표시 문구
                <textarea v-model="disclosure" rows="2" placeholder="예: 이 포스팅은 제휴 마케팅 활동의 일환으로 일정액의 수수료를 제공받습니다."></textarea>
              </label>
              <span class="muted micro">모든 게시물 첫 부분에 자동 삽입되며 검수·발행 가드가 누락을 차단합니다.</span>
            </div>
            <FieldError :message="errors.affiliate" :hint="hints.affiliate" />
          </div>
        </section>

        <!-- Submit Card -->
        <div class="submit-box">
          <button type="button" class="primary-launch" :disabled="busy || uploading" @click="submit">
            {{ busy ? '작업 시작 중…' : uploading ? '파일 업로드 중…' : 'AI에게 맡기기 →' }}
          </button>
          <p class="muted micro center-t">AI가 분석·전략·제작·검수·발행까지 알아서 진행합니다.</p>
          <button type="button" class="ghost full-btn" @click="router.back()">취소</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiFetch } from '../stores/auth.js'
import { toast } from '../stores/toast.js'
import { uploads } from '../stores/upload.js'
import Dropzone from '../components/Dropzone.vue'
import StatusChip from '../components/StatusChip.vue'
import FieldError from '../components/console/FieldError.vue'

const router = useRouter()
const route = useRoute()
const kinds = ['url', 'image', 'topic']
const TOPIC_EXAMPLES = [
  '겨울철 캠핑 난로 고르는 법',
  '1인 가구 무선 청소기 비교 추천',
  '건성 피부 수분크림 성분 체크리스트',
]
const CHANNELS = [
  { key: 'blog', label: '블로그 장문', desc: '네이버 블로그 · 티스토리' },
  { key: 'instagram', label: '인스타그램 피드', desc: '이미지 + 캡션 + 해시태그' },
  { key: 'shortform', label: '숏폼 영상', desc: '유튜브 쇼츠 · 릴스 · 틱톡' },
  { key: 'cardnews', label: '카드뉴스', desc: '슬라이드 요약 이미지' },
]
const PLATFORMS = {
  blog: ['naver', 'tistory'],
  instagram: ['instagram'],
  shortform: ['youtube', 'tiktok', 'instagram'],
  cardnews: ['instagram'],
}
const PLATFORM_KO = {
  naver: '네이버 블로그',
  tistory: '티스토리',
  instagram: '인스타그램',
  youtube: '유튜브',
  tiktok: '틱톡',
}

const kind = ref('url')
const url = ref('')
const topic = ref('')
const picked = ref({})
const business = ref('')
const team = ref('')
const teams = ref([])
const accounts = ref([])
const useStart = ref(false)
const startAt = ref('')
const sponsored = ref(false)
const disclosure = ref('이 게시물은 제휴 마케팅 활동의 일환으로 일정액의 수수료를 제공받습니다.')
const showExtraMedia = ref(false)
const busy = ref(false)
const teamsLoading = ref(true)
const errors = ref({})
const hints = ref({})

function kindLabel(k) {
  return k === 'url' ? '상품 링크' : k === 'image' ? '사진·파일' : '주제'
}
function platformsFor(key) { return PLATFORMS[key] || ['naver'] }
function platformDisplay(p) { return PLATFORM_KO[p] || p }

const urlHint = computed(() => {
  const u = url.value.trim().toLowerCase()
  if (!u || !/^https?:\/\//.test(u)) return ''
  if (u.includes('smartstore.naver.com') || u.includes('brand.naver.com')) return '네이버 스마트스토어 상품으로 보입니다'
  if (u.includes('coupang.com')) return '쿠팡 상품으로 보입니다'
  if (u.includes('11st.co.kr')) return '11번가 상품으로 보입니다'
  if (u.includes('musinsa.com')) return '무신사 상품으로 보입니다'
  if (u.includes('oliveyoung.co.kr')) return '올리브영 상품으로 보입니다'
  return '웹 페이지 링크로 인식됐습니다'
})

function toggleChannel(key) {
  if (picked.value[key]) {
    delete picked.value[key]
    return
  }
  picked.value[key] = { platform: platformsFor(key)[0], versions: 1 }
}

const PLATFORM_CHANNEL = {
  naver_blog: ['blog', 'naver'], tistory: ['blog', 'tistory'],
  instagram: ['instagram', 'instagram'],
  youtube: ['shortform', 'youtube'], tiktok: ['shortform', 'tiktok'],
}

function connectedCount(channelKey) {
  const plats = platformsFor(channelKey).map((p) => (p === 'naver' ? 'naver_blog' : p))
  return accounts.value.filter((a) => a.kind !== 'agent_service' && a.sessionState === 'connected' && plats.includes(a.platform)).length
}

const totalConnectedChannels = computed(() =>
  accounts.value.filter((a) => a.kind !== 'agent_service' && a.sessionState === 'connected').length,
)

const businessOptions = computed(() => {
  const set = new Set()
  for (const t of teams.value) for (const b of t.businesses || []) set.add(b)
  return [...set]
})

const resolvedTeam = computed(() => {
  if (team.value) return teams.value.find((t) => String(t.team.id) === team.value) || null
  const biz = business.value.trim().toLowerCase()
  if (biz) {
    const match = teams.value.find((t) => (t.businesses || []).some((b) => b.toLowerCase() === biz))
    if (match) return match
  }
  return teams.value.find((t) => t.team.isDefault) || teams.value[0] || null
})

const activeTeamLabel = computed(() => (resolvedTeam.value ? resolvedTeam.value.team.name : '자동 배정'))

function bump(key, d) {
  const c = picked.value[key]
  if (!c) return
  c.versions = Math.min(10, Math.max(1, c.versions + d))
}

const doneMedia = computed(() => uploads.filter((r) => r.state === 'done' && r.mediaId).map((r) => r.mediaId))
const uploading = computed(() => uploads.some((r) => r.state === 'queued' || r.state === 'active'))
const pickedCount = computed(() => Object.keys(picked.value).length)

onMounted(async () => {
  if (typeof route.query.kind === 'string' && kinds.includes(route.query.kind)) kind.value = route.query.kind
  if (typeof route.query.url === 'string' && route.query.url) url.value = route.query.url
  if (typeof route.query.topic === 'string' && route.query.topic) topic.value = route.query.topic
  if (typeof route.query.team === 'string' && route.query.team) team.value = route.query.team
  if (typeof route.query.business === 'string' && route.query.business) business.value = route.query.business

  const [tm, acc] = await Promise.all([
    apiFetch('/api/teams').catch(() => ({ items: [] })),
    apiFetch('/api/session/accounts').catch(() => ({ accounts: [] })),
  ])
  teams.value = tm.items || []
  accounts.value = acc.accounts || []
  teamsLoading.value = false

  const seen = new Set()
  for (const a of accounts.value) {
    if (a.kind === 'agent_service' || a.sessionState !== 'connected') continue
    const map = PLATFORM_CHANNEL[a.platform]
    if (!map || seen.has(map[0] + map[1])) continue
    seen.add(map[0] + map[1])
    if (!picked.value[map[0]]) picked.value[map[0]] = { platform: map[1], versions: 1 }
  }
  if (!Object.keys(picked.value).length) {
    picked.value.blog = { platform: 'naver', versions: 1 }
  }
})

function validate() {
  const e = {}
  const h = {}
  if (kind.value === 'url' && !url.value.trim()) {
    e.input = '상품 링크를 입력하세요.'
    h.input = 'https:// 로 시작하는 상품 페이지 주소를 붙여넣으세요'
  }
  if (kind.value === 'topic' && topic.value.trim().length < 2) {
    e.input = '주제를 입력하세요.'
    h.input = '예: 겨울철 캠핑 난로 고르는 법'
  }
  if (kind.value === 'image') {
    if (uploading.value) {
      e.input = '파일이 올라가는 중입니다. 잠시 기다리세요.'
      h.input = '업로드가 끝나면 바로 시작할 수 있습니다'
    } else if (!doneMedia.value.length) {
      e.input = '사진 또는 파일을 1개 이상 올리세요.'
      h.input = '클릭하거나 끌어다 놓으세요'
    }
  }
  if (!Object.keys(picked.value).length) {
    e.channels = '발행할 채널을 하나 이상 선택하세요.'
    h.channels = '블로그·인스타그램·숏폼·카드뉴스 중 선택할 수 있습니다'
  }
  if (sponsored.value && !disclosure.value.trim()) {
    e.affiliate = '대가 표시 문구를 입력하세요.'
    h.affiliate = '제휴·협찬 상품에는 대가 표시 문구가 필수입니다'
  }
  errors.value = e
  hints.value = h
  return !Object.keys(e).length
}

async function submit() {
  if (!validate() || busy.value) return
  busy.value = true
  try {
    const inputs = []
    if (kind.value === 'url') inputs.push({ kind: 'url', url: url.value.trim() })
    else if (kind.value === 'topic') inputs.push({ kind: 'topic', text: topic.value.trim() })
    if (kind.value === 'image' || doneMedia.value.length > 0) {
      if (doneMedia.value.length > 0) inputs.push({ kind: 'image', mediaIds: doneMedia.value })
    }
    const body = {
      inputs,
      channels: Object.entries(picked.value).map(([channel, c]) => ({ channel, platform: c.platform, versions: c.versions })),
      business: business.value.trim(),
      startAt: useStart.value ? startAt.value : '',
      affiliate: { isSponsored: sponsored.value, disclosure: sponsored.value ? disclosure.value.trim() : '' },
      team: team.value,
    }
    const data = await apiFetch('/api/works', { method: 'POST', body: JSON.stringify(body) })
    toast('AI가 작업을 시작했습니다')
    router.push(`/works/${data.id}`)
  } catch (e) {
    toast(e.message || '작업 시작 실패', 'error')
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.new { display: flex; flex-direction: column; gap: 20px; }
.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.ctx-pill {
  background: var(--accent-soft);
  color: var(--accent-dark);
  padding: 5px 12px;
  border-radius: 999px;
  font-weight: 700;
}

.layout { display: grid; grid-template-columns: 1.45fr 1fr; gap: 20px; align-items: start; }
.col-main, .col-side { display: flex; flex-direction: column; gap: 18px; min-width: 0; }
.card { display: flex; flex-direction: column; gap: 14px; }

.seg {
  display: inline-flex;
  background: var(--soft);
  padding: 4px;
  border-radius: 12px;
  gap: 4px;
  align-self: flex-start;
}
.seg button {
  background: transparent;
  color: var(--ink-soft);
  border: none;
  border-radius: 9px;
  padding: 8px 18px;
  font-weight: 600;
}
.seg button.on { background: var(--panel); color: var(--ink); font-weight: 700; box-shadow: var(--shadow-sm); }

.in-block { display: flex; flex-direction: column; gap: 8px; }
.detect { font-weight: 600; }
.topic-foot { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; }
.examples { display: flex; flex-wrap: wrap; gap: 6px; }
.ex-chip {
  background: var(--soft);
  color: var(--ink-soft);
  border: 1px solid var(--line);
  border-radius: 999px;
  padding: 4px 11px;
  font-size: 12px;
  font-weight: 500;
}
.ex-chip:hover { background: var(--accent-soft); border-color: var(--accent); color: var(--accent-dark); }
.opt-media { padding-top: 6px; border-top: 1px solid var(--line-soft); display: flex; flex-direction: column; gap: 10px; align-items: flex-start; }
.opt-drop { width: 100%; }

.chan-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.chan-card {
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  background: var(--panel);
  overflow: hidden;
  transition: border-color var(--t), box-shadow var(--t);
}
.chan-card.on { border-color: var(--accent); box-shadow: 0 0 0 2px var(--accent-soft); }
.chan-toggle {
  width: 100%;
  background: transparent;
  color: var(--ink);
  border: none;
  padding: 12px 14px;
  justify-content: flex-start;
  text-align: left;
  gap: 10px;
}
.chan-toggle:hover { background: var(--soft); }
.chkbox {
  width: 20px;
  height: 20px;
  border-radius: 6px;
  border: 1.5px solid var(--line);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 800;
  flex: none;
}
.chan-card.on .chkbox { background: var(--accent); border-color: var(--accent); color: #fff; }
.chan-info { display: flex; flex-direction: column; flex: 1; min-width: 0; }
.chan-info strong { font-size: 14px; }
.chan-info small { font-size: 11.5px; font-weight: 400; }
.conn-badge { font-size: 11px; font-weight: 700; color: var(--ok); background: var(--ok-soft); padding: 2px 7px; border-radius: 999px; }

.chcfg {
  display: flex;
  align-items: flex-end;
  gap: 10px;
  padding: 10px 14px 12px;
  background: var(--soft);
  border-top: 1px solid var(--line-soft);
}
.chcfg label { flex: 1; display: flex; flex-direction: column; gap: 4px; }
.chcfg select { padding: 6px 8px; font-size: 13px; }
.ver-box { display: flex; flex-direction: column; gap: 4px; align-items: center; }
.stepper { display: flex; align-items: center; gap: 6px; }
.stepper button { width: 28px; height: 28px; padding: 0; border-radius: 8px; font-size: 15px; }
.stepper span { min-width: 18px; text-align: center; font-weight: 700; }

.field-stack { display: flex; flex-direction: column; gap: 12px; }
.field-stack label { display: flex; flex-direction: column; gap: 5px; }
.check-row { flex-direction: row !important; align-items: center; gap: 6px !important; font-weight: 500 !important; color: var(--ink) !important; cursor: pointer; }
.divider { height: 1px; background: var(--line-soft); margin: 4px 0; }

.team-preview {
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.tp-row { display: flex; align-items: center; justify-content: space-between; gap: 8px; }

.submit-box { display: flex; flex-direction: column; gap: 8px; }
.primary-launch {
  width: 100%;
  padding: 14px 20px;
  font-size: 15.5px;
  font-weight: 800;
  border-radius: var(--r-md);
  background: linear-gradient(135deg, #06b6d4 0%, #0e7490 100%);
  color: #fff;
  box-shadow: 0 8px 22px -8px rgba(14, 116, 144, 0.55);
}
.primary-launch:hover:not(:disabled) { filter: brightness(1.06); }
.full-btn { width: 100%; }
.center-t { text-align: center; }

@media (max-width: 980px) {
  .layout { grid-template-columns: 1fr; }
}
@media (max-width: 640px) {
  .chan-grid { grid-template-columns: 1fr; }
}
</style>
