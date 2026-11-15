// Shared Korean labels for enums. API values stay English; only display text lives here.
export const CONTENT_STATUS = { draft: '초안', review: '검수', approved: '승인', queued: '대기', published: '발행됨' }
export const JOB_STATUS = { pending: '대기', claimed: '처리 중', done: '완료', failed: '실패' }
export const JOB_TYPE = { analyze: '판독', strategy: '전략', generate: '생성', review: '검수', research: '조사' }
export const SESSION_STATE = { pending: '대기', connected: '연결', expired: '만료', limited: '한도', inactive_hold: '비활성 보류' }
export const VERDICT = {
  ready: '실행 가능',
  manual: '수동 모드',
  held: '보류',
  blocked_session: '세션 미연결',
  unknown_account: '미등록 계정',
  scheduled_future: '예약됨',
}
export const ROLE = { chief: '총괄', worker: '작업자' }
export const POSTING_MODE = { auto: '자동', manual: '수동' }
export const EXIT_TYPE = { A: 'A · 파트너', B: 'B · 제휴' }
export const CHANNEL = { instagram: '인스타그램', shortform: '숏폼', cardnews: '카드뉴스', blog: '블로그' }
export const PLATFORM = {
  meta_ai: 'Meta AI', claude_app: 'Claude', chatgpt: 'ChatGPT', gemini: 'Gemini', muse: 'Muse',
  naver_blog: '네이버 블로그', tistory: '티스토리',
  instagram: '인스타그램', threads: '스레드', facebook: '페이스북', x: 'X',
  tiktok: '틱톡', youtube: '유튜브', blogger: '블로거', wordpress: '워드프레스',
  stubboard: '스텁(시험)',
}
export const KIND = { agent_service: 'AI 서비스', channel: '채널' }
// AI service platforms (kind=agent_service). Everything else is a channel.
export const AI_PLATFORMS = ['meta_ai', 'claude_app', 'chatgpt', 'gemini', 'muse']
// Platforms where Google SSO is a primary login path: the real-profile
// login option is shown only for these (fresh driven-browser Google SSO
// stalls at the post-identifier risk check).
export const GOOGLE_SSO_PLATFORMS = ['gemini', 'chatgpt', 'claude_app']
// Login order: AI services first, channels last (blogs, then SNS-first).
export const PLATFORM_GROUPS = [
  { label: 'AI 서비스', options: AI_PLATFORMS },
  { label: '채널', options: ['naver_blog', 'tistory', 'instagram', 'threads', 'facebook', 'x', 'tiktok', 'youtube', 'blogger', 'wordpress'] },
]
export const LOGIN_HINT = {
  meta_ai: 'Meta AI 페이지에서 로그인을 누른 뒤 Facebook/Instagram 계정으로 로그인하세요.',
  claude_app: 'Claude 로그인 화면에서 계정으로 로그인하세요.',
  chatgpt: 'ChatGPT 로그인 화면에서 계정으로 로그인하세요.',
  gemini: 'Gemini에서 Google 계정으로 로그인하세요.',
  muse: 'Muse 페이지에서 Log in을 누른 뒤 로그인하세요. 지역 제한(미국·캐나다)으로 막히면 그 화면 그대로 알려주세요.',
  naver_blog: '네이버 로그인 화면에서 계정으로 로그인하세요.',
  tistory: '티스토리 로그인 화면에서 계정으로 로그인하세요.',
  instagram: '인스타그램 로그인 화면에서 계정으로 로그인하세요.',
  threads: '스레드 로그인 화면에서 계정으로 로그인하세요.',
  facebook: '페이스북 로그인 화면에서 계정으로 로그인하세요.',
  x: 'X 로그인 화면에서 계정으로 로그인하세요.',
  tiktok: '틱톡 로그인 화면에서 계정으로 로그인하세요.',
  youtube: 'YouTube용 Google 계정으로 로그인하세요.',
  blogger: '블로거용 Google 계정으로 로그인하세요.',
  wordpress: '워드프레스 로그인 화면에서 계정으로 로그인하세요.',
}

export function label(map, value, fallback) {
  return map[value] || fallback || value || '—'
}

// UI_DESIGN §6 screen vocabulary. API values stay English; display text only.
export const WORK_STAGE = { intake: '접수', analyze: '분석', research: '조사', strategy: '전략', generate: '제작', review: '검수', publish: '발행', collect: '성과' }
export const POST_STATE = {
  draft: '만드는 중', review: '검수 중', approved: '승인됨',
  queued: '발행 대기', unassigned: '연결 계정 없음', published: '발행됨',
  held: '멈춤', failed: '발행 실패', archived: '폐기',
  drafting: '만드는 중', reviewing: '검수 중', rewriting: '다시 쓰는 중',
}
export const ACCOUNT_STATE = { connected: '연결됨', login_required: '로그인 필요', limited: '사용 한도', checking: '확인 중', off: '꺼둠' }
export const MEDIA_KIND = { upload: '업로드', gen_image: 'AI 이미지', gen_video: 'AI 영상', card: '카드뉴스' }
export const TEAM_ROLE = { analyze: '분석', strategy: '전략', generate: '제작', review: '검수' }
export const TEAM_ROLE_MODE = { auto: '자동', manual: '직접 지정' }
export const INPUT_KIND = { url: '링크', image: '사진·파일', topic: '주제' }
