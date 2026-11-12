<template>
  <section v-if="view === 'metrics'" class="page">
    <h2>지표</h2>
    <p class="muted">0이거나 누락인 건은 해석 없이 '원인 미분류'로 표기합니다.</p>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="card">
      <h3>지표 기록</h3>
      <div class="formrow">
        <label>콘텐츠 ID <input v-model="metric.contentId" placeholder="(선택)" /></label>
        <label>채널 <input v-model="metric.channel" placeholder="예: instagram" /></label>
        <label>노출 <input v-model="metric.exposures" type="number" min="0" /></label>
        <label>유입 <input v-model="metric.inflows" type="number" min="0" /></label>
        <label>주문 <input v-model="metric.orders" type="number" min="0" /></label>
        <label>문의 <input v-model="metric.inquiries" type="number" min="0" /></label>
        <button @click="saveMetric">기록</button>
      </div>
    </div>
    <div class="card">
      <h3>수집 기록</h3>
      <table v-if="metrics.length">
        <thead><tr><th>ID</th><th>콘텐츠</th><th>채널</th><th>노출</th><th>유입</th><th>주문</th><th>문의</th><th>판정</th><th>판정 흐름</th></tr></thead>
        <tbody>
          <tr v-for="m in metrics" :key="m.id">
            <td>{{ m.id }}</td>
            <td>{{ m.contentId || '—' }}</td>
            <td>{{ m.channel || '—' }}</td>
            <td>{{ num(m.exposures) }}</td>
            <td>{{ num(m.inflows) }}</td>
            <td>{{ num(m.orders) }}</td>
            <td>{{ num(m.inquiries) }}</td>
            <td>{{ m.causeClass || '—' }}</td>
            <td>
              <label class="check"><input type="checkbox" :checked="m.checkUrl" @change="setCheck(m, 'checkUrl', $event)" /> ①URL실재</label>
              <label class="check"><input type="checkbox" :checked="m.checkSession" @change="setCheck(m, 'checkSession', $event)" /> ②수집경로·세션</label>
              <label class="check"><input type="checkbox" :checked="m.checkDisplay" @change="setCheck(m, 'checkDisplay', $event)" /> ③채널표시</label>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-else class="muted">기록이 없습니다.</p>
      <p class="muted small">판정 흐름: ① 발행 URL 실재 → ② 수집 경로·세션 상태 → ③ 채널 표시 여부 순서로 확인합니다.</p>
    </div>
  </section>

  <section v-else-if="view === 'criteria'" class="page">
    <h2>판정·기준</h2>
    <p class="muted">아래는 결정 대기 항목입니다. 확정값으로 채우지 않고 대기 표시로 둡니다.</p>
    <div class="card">
      <h3>모순 6건 (대기)</h3>
      <ol>
        <li>블로그 채널 — 종전 결정(로그인 세션 직접 포스팅) vs 약관·API 종료</li>
        <li>Instagram 게시 한도 100 vs 50</li>
        <li>TikTok Intended Use vs SOMUN 1차 사용 형태</li>
        <li>피클의 마케팅 활용 전제 (공식 근거 없음)</li>
        <li>스마트스토어 판독 — 화면 열람 vs 자동 수집의 경계</li>
        <li>Instagram 해시태그 30 vs 5</li>
      </ol>
    </div>
    <div class="card">
      <h3>미확인 15건 (대기)</h3>
      <ol>
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
        <li>상주 Muse 연결의 구체 기술 방식</li>
        <li>SOMUN 상표 정식 검색·도메인 확정</li>
      </ol>
    </div>
    <div class="card">
      <h3>PO 결정 필요 3건 (대기)</h3>
      <ol>
        <li>모순-1 처리 방향</li>
        <li>모순-3 확인 선행 여부</li>
        <li>파일럿 발행에 쓸 SNS 계정</li>
      </ol>
    </div>
  </section>

  <section v-else-if="view === 'resident'" class="page">
    <h2>Muse 상주 관리</h2>
    <p class="muted">실제 Muse 연동은 트랜치 2입니다. 이 화면에서는 작업 큐(최근 100건)의 처리·실패 상태를 봅니다.</p>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="card">
      <div class="toolbar">
        <span class="muted">상주 연결: 대기 (트랜치 2 범위)</span>
        <button class="ghost" @click="loadJobs">새로고침</button>
      </div>
      <table v-if="jobs.length">
        <thead><tr><th>ID</th><th>종류</th><th>상품</th><th>상태</th><th>작업자</th><th>사유</th><th>생성</th></tr></thead>
        <tbody>
          <tr v-for="j in jobs" :key="j.id">
            <td>{{ j.id }}</td>
            <td>{{ j.type }}</td>
            <td>{{ j.refId }}</td>
            <td>{{ j.status }}</td>
            <td>{{ j.workerName || '—' }}</td>
            <td>{{ j.error || '—' }}</td>
            <td>{{ shortDate(j.createdAt) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-else class="muted">작업이 없습니다.</p>
      <p class="muted small">실패한 작업은 자동 재등록되지 않습니다. 원인을 확인한 뒤 상품을 다시 투입하세요.</p>
    </div>
  </section>

  <section v-else-if="view === 'system'" class="page">
    <h2>시스템 구성</h2>
    <p class="muted">DB·업로드 백업과 실행 자리 안내입니다. 앞단은 Cloudflare 전제, 실행 자리는 로컬입니다.</p>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="card">
      <h3>정기 백업</h3>
      <p>대상: DB 파일 + 업로드 폴더. 서버 기동 시 BACKUP_INTERVAL_MIN 간격으로 자동 실행됩니다.</p>
      <button @click="backupNow">지금 백업 실행</button>
      <p v-if="backupMsg" class="ok">{{ backupMsg }}</p>
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
    <h2>모바일 앱 안내·연결</h2>
    <p class="muted">Flutter 앱 자체는 트랜치 2입니다. 이 화면은 안내와 연결 상태만 제공합니다.</p>
    <div class="card">
      <h3>안내</h3>
      <ol>
        <li>트랜치 2에서 Flutter 앱이 나오면 이 화면에서 연결 코드를 확인합니다.</li>
        <li>하단 탭 5개(홈·대화·검수·발행·더보기)는 앱 설계에 반영돼 있습니다.</li>
      </ol>
    </div>
    <div class="card">
      <h3>연결 상태</h3>
      <p>대기 중 — 연결된 모바일 기기가 없습니다.</p>
    </div>
  </section>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import { apiFetch } from '../stores/auth.js'

const props = defineProps({ view: { type: String, default: 'metrics' } })

const error = ref('')
const metrics = ref([])
const metric = ref({ contentId: '', channel: '', exposures: '', inflows: '', orders: '', inquiries: '' })
const jobs = ref([])
const backupMsg = ref('')

function num(v) {
  return v === null || v === undefined ? '—' : v
}

function shortDate(iso) {
  return iso ? iso.slice(0, 16).replace('T', ' ') : '—'
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
    }
  } catch (e) {
    error.value = e.message
  }
}

async function loadJobs() {
  const data = await apiFetch('/api/jobs')
  jobs.value = data.jobs || []
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
  } catch (e) {
    error.value = e.message
  }
}

watch(() => props.view, load)
onMounted(load)
</script>
