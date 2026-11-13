<template>
  <section v-if="view === 'metrics'" class="page">
    <h2>지표</h2>
    <p class="muted">0이거나 누락인 건은 해석 없이 '원인 미분류'로 표기합니다.</p>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="card">
      <h3>지표 기록</h3>
      <div class="form-grid">
        <label>콘텐츠 ID <input v-model="metric.contentId" placeholder="(선택)" /></label>
        <label>채널 <input v-model="metric.channel" placeholder="예: instagram" /></label>
        <label>노출 <input v-model="metric.exposures" type="number" min="0" /></label>
        <label>유입 <input v-model="metric.inflows" type="number" min="0" /></label>
        <label>주문 <input v-model="metric.orders" type="number" min="0" /></label>
        <label>문의 <input v-model="metric.inquiries" type="number" min="0" /></label>
        <div class="full"><button @click="saveMetric">기록</button></div>
      </div>
    </div>
    <div class="card">
      <div class="sect-head"><h3>수집 기록</h3><span class="count">{{ metrics.length }}건</span></div>
      <div v-if="metrics.length" class="table-wrap">
      <table>
        <thead><tr><th class="id">ID</th><th class="num">콘텐츠</th><th>채널</th><th class="num">노출</th><th class="num">유입</th><th class="num">주문</th><th class="num">문의</th><th>판정</th><th>판정 흐름</th><th></th></tr></thead>
        <tbody>
          <tr v-for="m in metrics" :key="m.id">
            <td class="id">{{ m.id }}</td>
            <td class="num">{{ m.contentId || '—' }}</td>
            <td>{{ m.channel || '—' }}</td>
            <td class="num">{{ num(m.exposures) }}</td>
            <td class="num">{{ num(m.inflows) }}</td>
            <td class="num">{{ num(m.orders) }}</td>
            <td class="num">{{ num(m.inquiries) }}</td>
            <td>{{ m.causeClass || '—' }}</td>
            <td>
              <label class="check"><input type="checkbox" :checked="m.checkUrl" @change="setCheck(m, 'checkUrl', $event)" /> ①URL실재</label>
              <label class="check"><input type="checkbox" :checked="m.checkSession" @change="setCheck(m, 'checkSession', $event)" /> ②수집경로·세션</label>
              <label class="check"><input type="checkbox" :checked="m.checkDisplay" @change="setCheck(m, 'checkDisplay', $event)" /> ③채널표시</label>
            </td>
            <td><button class="ghost small" @click="confirmDelMetric = m.id">삭제</button></td>
          </tr>
        </tbody>
      </table>
      </div>
      <EmptyArt v-else art="lens" title="기록이 없습니다" hint="위 폼에서 첫 지표를 기록하세요." />
      <p class="muted small">판정 흐름: ① 발행 URL 실재 → ② 수집 경로·세션 상태 → ③ 채널 표시 여부 순서로 확인합니다.</p>
    </div>
  </section>

  <section v-else-if="view === 'criteria'" class="page">
    <h2>판정·기준</h2>
    <p class="muted">해소된 항목은 <span class="pill p-approved">해소</span> 표시로 구분합니다. 대기 항목은 확정값으로 채우지 않습니다.</p>
    <div class="card">
      <div class="sect-head"><h3>모순 6건</h3><span class="count">해소 3 · 대기 3</span></div>
      <ol class="declist">
        <li>블로그 채널 — 종전 결정(로그인 세션 직접 포스팅) vs 약관·API 종료</li>
        <li>Instagram 게시 한도 100 vs 50 <span class="pill p-approved">해소</span></li>
        <li>TikTok Intended Use vs SOMUN 1차 사용 형태 <span class="pill p-approved">해소</span></li>
        <li>피클의 마케팅 활용 전제 (공식 근거 없음)</li>
        <li>스마트스토어 판독 — 화면 열람 vs 자동 수집의 경계</li>
        <li>Instagram 해시태그 30 vs 5 <span class="pill p-approved">해소</span></li>
      </ol>
    </div>
    <div class="card">
      <div class="sect-head"><h3>미확인 15건</h3><span class="count">해소 1 · 대기 14</span></div>
      <ol class="declist">
        <li>쿠팡 파트너스 약관 원문</li>
        <li>Instagram 한도·해시태그 확정값</li>
        <li>Threads 단독 약관 문구</li>
        <li>Blogger 쿼터</li>
        <li>네이버 블로그 공식 콘텐츠 규격</li>
        <li>티스토리 공식 콘텐츠 규격</li>
        <li>스마트스토어 제3자 공개 정보 범위 공식 안내</li>
        <li>경쟁 도구 세션 유지 상세</li>
        <li>Metricool 공식 요금표</li>
        <li>피클 마케팅 활용 방식</li>
        <li>APR 매출과 발행 방식의 인과</li>
        <li>TikTok 감사 소요 기간</li>
        <li>플랫폼 자체 AI 라벨 정책</li>
        <li>상주 Muse 연결의 구체 기술 방식 <span class="pill p-approved">해소</span></li>
        <li>SOMUN 상표 정식 검색·도메인 확정</li>
      </ol>
    </div>
    <div class="card">
      <div class="sect-head"><h3>PO 결정 3건</h3><span class="count">해소 1 · 대기 2</span></div>
      <ol class="declist">
        <li>모순-1 처리 방향 <span class="pill p-approved">해소</span></li>
        <li>모순-3 확인 선행 여부</li>
        <li>파일럿 슬롯 등록 + 로그인 수행</li>
      </ol>
    </div>
  </section>

  <section v-else-if="view === 'resident'" class="page">
    <h2>Muse 상주 관리</h2>
    <p class="muted">상주 작업자(A/B/C 레인)의 연결 상태와 작업 큐(최근 100건)의 처리·실패 상태를 봅니다.</p>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="card">
      <div class="toolbar">
        <span class="muted">상주 연결: {{ residentOnline }}기 온라인 / {{ residentTotal }}기 등록 (1시간 완료 {{ residentDone }} · 실패 {{ residentFailed }})</span>
        <button class="ghost" @click="loadJobs">새로고침</button>
      </div>
      <div v-if="jobs.length" class="table-wrap">
        <table>
          <thead><tr><th>ID</th><th>종류</th><th>상품</th><th>상태</th><th>작업자</th><th class="num">시도</th><th>리스</th><th>사유</th><th>생성</th><th></th></tr></thead>
          <tbody>
            <tr v-for="j in jobs" :key="j.id">
              <td class="id">{{ j.id }}</td>
              <td>{{ jobTypeLabel(j.type) }}</td>
              <td class="num">{{ j.refId }}</td>
              <td><span :class="'pill p-' + j.status">{{ jobStatusLabel(j.status) }}</span></td>
              <td>{{ j.workerName || '—' }}</td>
              <td class="num">{{ j.attempts || 0 }}</td>
              <td>{{ j.leaseUntil ? shortTime(j.leaseUntil) : '—' }}</td>
              <td>{{ j.error || '—' }}</td>
              <td>{{ shortDate(j.createdAt) }}</td>
              <td>
                <button v-if="j.status === 'pending' || j.status === 'claimed'" class="ghost small" @click="cancelJob(j.id)">취소</button>
                <button v-if="j.status === 'failed'" class="ghost small" @click="requeueJob(j.id)">재등록</button>
                <button class="ghost small" @click="confirmDelJob = j.id">삭제</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyArt v-else art="tray" title="작업이 없습니다" />
      <p class="muted small">실패한 작업은 자동 재등록되지 않습니다. 원인을 확인한 뒤 상품을 다시 투입하세요.</p>
    </div>
  </section>

  <section v-else-if="view === 'system'" class="page">
    <h2>시스템 구성</h2>
    <p class="muted">DB·업로드 백업과 실행 자리 안내입니다. 앞단은 Cloudflare 전제, 실행 자리는 로컬입니다.</p>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="card">
      <div class="sect-head"><h3>정기 백업</h3><span class="count">{{ backups.length }}세대 보관</span></div>
      <p>대상: DB 파일 + 업로드 폴더. 서버 기동 시 BACKUP_INTERVAL_MIN 간격으로 자동 실행됩니다.</p>
      <button @click="backupNow">지금 백업 실행</button>
      <p v-if="backupMsg" class="ok">{{ backupMsg }}</p>
      <ul v-if="backups.length" class="speclist">
        <li v-for="b in backups" :key="b"><span class="k">백업</span><span>{{ b }}</span></li>
      </ul>
      <p v-else class="empty">아직 백업이 없습니다.</p>
    </div>
    <div class="card">
      <div class="sect-head"><h3>보관 정리</h3></div>
      <p>완료된 작업 90일, 지표 180일까지 보관합니다. 진행 중·대기 작업은 손대지 않습니다. RETENTION_CHECK_HOURS 간격으로 자동 실행됩니다.</p>
      <button @click="purgeNow">지금 정리 실행</button>
      <p v-if="purgeMsg" class="ok">{{ purgeMsg }}</p>
    </div>
    <div class="card">
      <div class="sect-head"><h3>발행 실행기</h3><span :class="execForm.heartbeatMin > 0 ? 'pill p-connected' : 'pill p-pending'">{{ execForm.heartbeatMin > 0 ? '가동' : '정지' }}</span></div>
      <p class="muted small">공식 API군 판정 통과 건만 실행합니다. 심박 0이면 판정만 하고 실행은 쉽니다.</p>
      <div class="form-grid">
        <label>심박(분, 0=정지) <input v-model.number="execForm.heartbeatMin" type="number" min="0" /></label>
        <label>계정당 최소 간격(분) <input v-model.number="execForm.minIntervalMin" type="number" min="0" max="1440" /></label>
        <label>틱당 최대 처리 <input v-model.number="execForm.maxActive" type="number" min="1" /></label>
        <div class="full"><button @click="saveExecutor">저장</button></div>
      </div>
      <p v-if="execMsg" class="banner okk">{{ execMsg }}</p>
    </div>
    <div class="card">
      <h3>구성 메모</h3>
      <ul>
        <li>DB는 SQLite로 시작하며, 접근 코드는 store 한 층에 모여 있습니다.</li>
        <li>주문·결제는 파트너 몰에서 일어나며 이 시스템은 다루지 않습니다.</li>
        <li>GCP는 이 플랫폼에 쓰지 않습니다(LUNO 전용).</li>
      </ul>
    </div>
  </section>

  <section v-else class="page">
    <h2>모바일 앱 (PWA)</h2>
    <p class="muted">별도 네이티브 앱 없이 설치형 웹(PWA)으로 씁니다. 푸시·카메라 같은
      네이티브 기능이 필요해지면 그때 재검토합니다.</p>
    <div class="card">
      <h3>설치 방법</h3>
      <ol class="declist">
        <li>모바일 Chrome·Edge에서 콘솔 주소로 접속합니다.</li>
        <li>브라우저 메뉴에서 '앱 설치'·'홈 화면에 추가'를 누릅니다.</li>
        <li>홈 아이콘으로 독립 창 실행. 반응형 레이아웃으로 그대로 씁니다.</li>
      </ol>
    </div>
    <div class="card">
      <h3>알아둘 점</h3>
      <ul class="speclist">
        <li><span class="k">오프라인</span><span>앱 껍데기는 캐시되지만 데이터는 서버 연결이 필요합니다.</span></li>
        <li><span class="k">외부 접속</span><span>같은 PC가 아니면 Cloudflare Tunnel 주소를 씁니다.</span></li>
      </ul>
    </div>
  </section>

  <div v-if="confirmDelMetric !== null" class="modal-back" @click.self="confirmDelMetric = null">
    <div class="modal">
      <h3>지표 #{{ confirmDelMetric }} 삭제</h3>
      <p>이 기록만 지워집니다. 되돌릴 수 없습니다.</p>
      <div class="btnrow">
        <button class="danger" @click="doDeleteMetric">삭제</button>
        <button class="ghost" @click="confirmDelMetric = null">취소</button>
      </div>
    </div>
  </div>
  <div v-if="confirmDelJob !== null" class="modal-back" @click.self="confirmDelJob = null">
    <div class="modal">
      <h3>작업 #{{ confirmDelJob }} 삭제</h3>
      <p>작업 이력에서 지워집니다. 되돌릴 수 없습니다.</p>
      <div class="btnrow">
        <button class="danger" @click="doDeleteJob">삭제</button>
        <button class="ghost" @click="confirmDelJob = null">취소</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import { apiFetch } from '../stores/auth.js'
import { JOB_STATUS, JOB_TYPE, label } from '../stores/labels.js'
import EmptyArt from '../components/EmptyArt.vue'

function jobTypeLabel(t) {
  return label(JOB_TYPE, t)
}

function jobStatusLabel(s) {
  return label(JOB_STATUS, s)
}

const props = defineProps({ view: { type: String, default: 'metrics' } })

const error = ref('')
const metrics = ref([])
const metric = ref({ contentId: '', channel: '', exposures: '', inflows: '', orders: '', inquiries: '' })
const jobs = ref([])
const backupMsg = ref('')
const backups = ref([])
const purgeMsg = ref('')
const execForm = ref({ heartbeatMin: 0, minIntervalMin: 30, maxActive: 2 })
const execMsg = ref('')
const confirmDelMetric = ref(null)
const confirmDelJob = ref(null)
const residentOnline = ref(0)
const residentTotal = ref(0)
const residentDone = ref(0)
const residentFailed = ref(0)

function num(v) {
  return v === null || v === undefined ? '—' : v
}

function shortDate(iso) {
  return iso ? iso.slice(0, 16).replace('T', ' ') : '—'
}

function shortTime(iso) {
  return iso ? iso.slice(11, 16) : '—'
}

function optInt(v) {
  if (v === '' || v === null || v === undefined) return null
  return Number(v)
}

async function load() {
  error.value = ''
  try {
    if (props.view === 'metrics') {
      const data = await apiFetch('/api/metrics')
      metrics.value = data.metrics || []
    } else if (props.view === 'resident') {
      await loadJobs()
    } else if (props.view === 'system') {
      const data = await apiFetch('/api/system/backups')
      backups.value = data.backups || []
      const cfg = await apiFetch('/api/executor/config')
      execForm.value = { heartbeatMin: cfg.heartbeatMin, minIntervalMin: cfg.minIntervalMin, maxActive: cfg.maxActive }
    }
  } catch (e) {
    error.value = e.message
  }
}

async function loadJobs() {
  const [data, stats, agents] = await Promise.all([
    apiFetch('/api/jobs'),
    apiFetch('/api/agents/stats').catch(() => null),
    apiFetch('/api/agents?limit=1').catch(() => null),
  ])
  jobs.value = data.jobs || []
  if (stats) {
    residentOnline.value = stats.online || 0
    residentDone.value = stats.done1h || 0
    residentFailed.value = stats.failed1h || 0
  }
  if (agents) residentTotal.value = agents.total || 0
}

async function saveExecutor() {
  error.value = ''
  execMsg.value = ''
  try {
    const data = await apiFetch('/api/executor/config', { method: 'POST', body: JSON.stringify(execForm.value) })
    execForm.value = { heartbeatMin: data.heartbeatMin, minIntervalMin: data.minIntervalMin, maxActive: data.maxActive }
    execMsg.value = '실행기 설정을 저장했습니다.'
  } catch (e) {
    error.value = e.message
  }
}

async function doDeleteMetric() {
  const id = confirmDelMetric.value
  confirmDelMetric.value = null
  error.value = ''
  try {
    await apiFetch(`/api/metrics/${id}`, { method: 'DELETE' })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function cancelJob(id) {
  error.value = ''
  try {
    await apiFetch(`/api/jobs/${id}/cancel`, { method: 'POST', body: JSON.stringify({}) })
    await loadJobs()
  } catch (e) {
    error.value = e.message
  }
}

async function doDeleteJob() {
  const id = confirmDelJob.value
  confirmDelJob.value = null
  error.value = ''
  try {
    await apiFetch(`/api/jobs/${id}`, { method: 'DELETE' })
    await loadJobs()
  } catch (e) {
    error.value = e.message
  }
}

async function requeueJob(id) {
  error.value = ''
  try {
    await apiFetch(`/api/jobs/${id}/requeue`, { method: 'POST' })
    await loadJobs()
  } catch (e) {
    error.value = e.message
  }
}

async function saveMetric() {
  error.value = ''
  try {
    const body = {
      channel: metric.value.channel.trim(),
      exposures: optInt(metric.value.exposures),
      inflows: optInt(metric.value.inflows),
      orders: optInt(metric.value.orders),
      inquiries: optInt(metric.value.inquiries),
    }
    if (metric.value.contentId !== '') body.contentId = Number(metric.value.contentId)
    await apiFetch('/api/metrics', { method: 'POST', body: JSON.stringify(body) })
    metric.value = { contentId: '', channel: '', exposures: '', inflows: '', orders: '', inquiries: '' }
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function setCheck(m, key, ev) {
  error.value = ''
  try {
    const body = { checkUrl: m.checkUrl, checkSession: m.checkSession, checkDisplay: m.checkDisplay, note: m.note || '' }
    body[key] = ev.target.checked
    const updated = await apiFetch(`/api/metrics/${m.id}`, { method: 'PATCH', body: JSON.stringify(body) })
    Object.assign(m, updated)
  } catch (e) {
    error.value = e.message
    ev.target.checked = m[key]
  }
}

async function backupNow() {
  error.value = ''
  backupMsg.value = ''
  try {
    const res = await apiFetch('/api/system/backup', { method: 'POST' })
    backupMsg.value = `백업 완료: ${res.dest}`
    const data = await apiFetch('/api/system/backups')
    backups.value = data.backups || []
  } catch (e) {
    error.value = e.message
  }
}

async function purgeNow() {
  error.value = ''
  purgeMsg.value = ''
  try {
    const res = await apiFetch('/api/system/purge', { method: 'POST' })
    purgeMsg.value = `정리 완료: 작업 ${res.jobs}건 · 지표 ${res.metrics}건`
  } catch (e) {
    error.value = e.message
  }
}

watch(() => props.view, load)
onMounted(load)
</script>
