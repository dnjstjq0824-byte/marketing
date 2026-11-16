# SOMUN 마케팅 코어 — API·스키마 부록

- 상태: 부록. 본문([기획서](PLANNING.md)·[설계서](DESIGN.md)·[상용화 계획](PLAN.md)·[README](README.md))의 상세 근거로 둔다.
- 문서 지위: 초기 개발 문서(의장 지정 2026-11-12). 환경·상황에 따라 개정한다.
- 기준: 커밋 2161e46, 스키마 v5, API 58종(공개 2·작업자 4·운영자 52), 화면 15보기.
  A~E는 현재 코드, A-15·B-4는 계획(P0·P1), F·G는 검증 절차다.
- 작성 원칙: 모든 컬럼·enum·에러 코드는 코드에서 확인했다. 코드에 없는 값은 만들지 않고
  `미확인 — 코드 확인 필요`로 남긴다. 예시에는 실계정명·실토큰·개인정보를 쓰지 않으며
  계정은 피클-1 같은 슬롯 라벨 수준으로만 쓴다.
- 본문과 부록이 다르면 코드를 기준으로 삼고 본문을 고친다.

## A. DB 스키마 부록 (스키마 v5)

SQLite. 시각은 전부 RFC3339 UTC 문자열(TEXT), 참·거짓은 INTEGER(0/1),
금액·수수료 같은 미확인 수치는 어떤 테이블에도 없다. 아래 제약은 DDL 선언 기준이다.

### A-1. users — 운영자

| 컬럼 | 타입 | NULL | 기본값 | 제약·설명 |
|---|---|---|---|---|
| id | INTEGER | NO | 자동 | PK AUTOINCREMENT |
| username | TEXT | NO | — | UNIQUE |
| password_hash | TEXT | NO | — | bcrypt 해시 |
| role | TEXT | NO | 'operator' | 최초 시드 운영자는 'admin', API 추가는 'operator'. 값 집행 없음(1인 전제) |
| created_at | TEXT | NO | — | RFC3339 |
| updated_at | TEXT | NO | — | RFC3339 |

### A-2. products — 상품

| 컬럼 | 타입 | NULL | 기본값 | 제약·설명 |
|---|---|---|---|---|
| id | INTEGER | NO | 자동 | PK AUTOINCREMENT |
| name | TEXT | NO | '' | 판독 전에는 빈값 |
| source_url | TEXT | NO | '' | http(s) URL(검증은 서비스층) |
| exit_type | TEXT | NO | 'A' | CHECK IN ('A','B') |
| priority | INTEGER | NO | 0 | 0 이상(검증은 서비스층) |
| business | TEXT | NO | '' | v2 추가. 비교·집계는 NOCASE |
| created_at | TEXT | NO | — | RFC3339 |
| updated_at | TEXT | NO | — | RFC3339 |

### A-3. product_images — 상품 이미지

| 컬럼 | 타입 | NULL | 기본값 | 제약·설명 |
|---|---|---|---|---|
| id | INTEGER | NO | 자동 | PK AUTOINCREMENT |
| product_id | INTEGER | NO | — | FK → products(id) ON DELETE CASCADE, 인덱스 |
| path | TEXT | NO | — | 업로드 파일명(확장자 jpg·jpeg·png·gif·webp만) |
| sort | INTEGER | NO | 0 | 정렬 순서 |
| created_at | TEXT | NO | — | RFC3339 |

### A-4. readings — 판독 결과 (상품당 1건)

| 컬럼 | 타입 | NULL | 기본값 | 제약·설명 |
|---|---|---|---|---|
| id | INTEGER | NO | 자동 | PK AUTOINCREMENT |
| product_id | INTEGER | NO | — | UNIQUE, FK → products(id) ON DELETE CASCADE |
| product_name | TEXT | NO | '' | 8문자열 중 1 |
| price_summary | TEXT | NO | '' | 8문자열 중 2 |
| options_text | TEXT | NO | '' | 8문자열 중 3 |
| shipping_text | TEXT | NO | '' | 8문자열 중 4 |
| detail_core | TEXT | NO | '' | 8문자열 중 5 |
| review_summary | TEXT | NO | '' | 8문자열 중 6 |
| image_desc | TEXT | NO | '' | 8문자열 중 7 |
| unknowns | TEXT | NO | '' | 미확인 분리 기재. 지어 채우기 금지 |
| created_at | TEXT | NO | — | RFC3339 |
| updated_at | TEXT | NO | — | RFC3339 |

### A-5. strategies — 마케팅 전략 (상품당 1건, A 산출물)

| 컬럼 | 타입 | NULL | 기본값 | 제약·설명 |
|---|---|---|---|---|
| id | INTEGER | NO | 자동 | PK AUTOINCREMENT |
| product_id | INTEGER | NO | — | UNIQUE, FK → products(id) ON DELETE CASCADE |
| angle | TEXT | NO | '' | 앵글. 필수(빈값 400) |
| channels | TEXT | NO | '' | 선택 |
| key_messages | TEXT | NO | '' | 선택 |
| schedule_note | TEXT | NO | '' | 선택 |
| created_at | TEXT | NO | — | RFC3339 |
| updated_at | TEXT | NO | — | RFC3339 |

### A-6. contents — 콘텐츠 (핵심 상태 기계)

| 컬럼 | 타입 | NULL | 기본값 | 제약·설명 |
|---|---|---|---|---|
| id | INTEGER | NO | 자동 | PK AUTOINCREMENT |
| product_id | INTEGER | NO | — | FK → products(id) ON DELETE CASCADE, 인덱스 |
| channel | TEXT | NO | '' | CHECK IN ('','instagram','shortform','cardnews','blog') |
| target_platform | TEXT | NO | '' | 예약 시 확정. 공식 6종·직접 2종(표 A-13) |
| channel_group | TEXT | NO | '' | CHECK IN ('','official_api','manual') |
| title | TEXT | NO | '' | — |
| body | TEXT | NO | '' | — |
| disclosure | TEXT | NO | '' | 대가 표시. 출구 B 초안에 자동 삽입, 수정 API로 변경 불가 |
| is_sponsored | INTEGER | NO | 0 | 출구 B 여부 |
| is_virtual_persona | INTEGER | NO | 0 | AI 가상인물 등장 여부 |
| ai_label | TEXT | NO | '' | 가상인물일 때만 부착 |
| status | TEXT | NO | 'draft' | CHECK 5종. 전이표 A-14, 인덱스 |
| target_account | TEXT | NO | '' | 슬롯 라벨(내부 키). 인덱스 |
| scheduled_at | TEXT | NO | '' | RFC3339 UTC. 빈값 = 미예약 큐 항목 |
| reserve_seq | INTEGER | NO | 0 | 예약 적재 순번(동시각 직렬용) |
| held | INTEGER | NO | 0 | 보류 여부 |
| hold_reason | TEXT | NO | '' | 보류 사유 |
| created_at | TEXT | NO | — | RFC3339 |
| updated_at | TEXT | NO | — | RFC3339 |

### A-7. publish_logs — 발행 기록 (중복 차단의 물리적 토대)

| 컬럼 | 타입 | NULL | 기본값 | 제약·설명 |
|---|---|---|---|---|
| id | INTEGER | NO | 자동 | PK AUTOINCREMENT |
| content_id | INTEGER | NO | — | FK → contents(id) ON DELETE CASCADE |
| channel | TEXT | NO | '' | unique 삼중항 중 1 |
| account | TEXT | NO | '' | unique 삼중항 중 2(슬롯 라벨) |
| status | TEXT | NO | 'scheduled' | CHECK IN ('scheduled','held','done','failed') |
| executed_at | TEXT | NO | '' | 실행 시각(RFC3339). 트랜치 1 수동 기록에서만 채움 |
| note | TEXT | NO | '' | 메모·URL 증거·auto-advanced·chief-approved |
| created_at | TEXT | NO | — | RFC3339 |

UNIQUE(content_id, channel, account). 트랜치 1의 API 생성·수정은
scheduled·held만 허용하고 done·failed는 거부한다.

### A-8. metrics — 지표 (NULL = 미수집)

| 컬럼 | 타입 | NULL | 기본값 | 제약·설명 |
|---|---|---|---|---|
| id | INTEGER | NO | 자동 | PK AUTOINCREMENT |
| content_id | INTEGER | YES | NULL | FK → contents(id) ON DELETE CASCADE |
| channel | TEXT | NO | '' | — |
| exposures | INTEGER | YES | NULL | NULL 허용 |
| inflows | INTEGER | YES | NULL | NULL 허용 |
| orders | INTEGER | YES | NULL | NULL 허용 |
| inquiries | INTEGER | YES | NULL | NULL 허용 |
| cause_class | TEXT | NO | '' | 4종 전부가 0·NULL이면 원인 미분류 자동 기재 |
| check_url | INTEGER | NO | 0 | 3단계 중 ① 발행 URL 실재 |
| check_session | INTEGER | NO | 0 | 3단계 중 ② 수집 경로·세션 상태 |
| check_display | INTEGER | NO | 0 | 3단계 중 ③ 채널 표시 여부 |
| note | TEXT | NO | '' | — |
| created_at | TEXT | NO | — | RFC3339 |
| updated_at | TEXT | NO | — | RFC3339 |

### A-9. jobs — 작업 큐 (ref_id는 비-FK)

| 컬럼 | 타입 | NULL | 기본값 | 제약·설명 |
|---|---|---|---|---|
| id | INTEGER | NO | 자동 | PK AUTOINCREMENT |
| type | TEXT | NO | — | CHECK IN ('analyze','strategy','generate','review') |
| ref_id | INTEGER | NO | — | 상품 ID. 비-FK(상품 삭제 시 명시 삭제), 인덱스 |
| status | TEXT | NO | 'pending' | CHECK IN ('pending','claimed','done','failed'), 인덱스 |
| worker_name | TEXT | NO | '' | claim 시 기록. 미지정 시 resident-claude |
| payload | TEXT | NO | '' | JSON 문자열(입력 재료) |
| result | TEXT | NO | '' | JSON 문자열(등록 결과) |
| error | TEXT | NO | '' | 실패 사유 |
| created_at | TEXT | NO | — | RFC3339 |
| claimed_at | TEXT | NO | '' | RFC3339. 지연 감시 기준(claim 후 30분) |
| finished_at | TEXT | NO | '' | RFC3339. 보관 기산점(완료·실패 90일) |

인덱스: (status, type, id), (type, ref_id).

### A-10. affiliate_links — 제휴 링크 (출구 B)

| 컬럼 | 타입 | NULL | 기본값 | 제약·설명 |
|---|---|---|---|---|
| id | INTEGER | NO | 자동 | PK AUTOINCREMENT |
| product_id | INTEGER | NO | — | FK → products(id) ON DELETE CASCADE, 인덱스 |
| partner_name | TEXT | NO | '' | 제휴처명 |
| url | TEXT | NO | — | http(s) URL(검증은 서비스층) |
| created_at | TEXT | NO | — | RFC3339 |

수수료율 컬럼은 없다(미확인 수치 금지).

### A-11. channel_accounts — 채널·에이전트 계정 (자격증명은 암호문만)

| 컬럼 | 타입 | NULL | 기본값 | 제약·설명 |
|---|---|---|---|---|
| id | INTEGER | NO | 자동 | PK AUTOINCREMENT |
| name | TEXT | NO | — | UNIQUE. 슬롯 라벨(큐 연결 키, 불변) |
| platform | TEXT | NO | '' | 플랫폼명(자유 입력) |
| owner | TEXT | NO | '' | 계정 주인 |
| role | TEXT | NO | 'worker' | chief·worker |
| business | TEXT | NO | '' | 담당 사업 |
| session_state | TEXT | NO | 'pending' | pending·connected·expired·limited·inactive_hold |
| posting_mode | TEXT | NO | 'manual' | auto·manual(기본 수동) |
| enc_id | TEXT | NO | '' | 로그인 ID 암호문(AES-256-GCM). 평문 없음 |
| enc_pw | TEXT | NO | '' | 비밀번호 암호문(AES-256-GCM). 평문 없음 |
| last_checked_at | TEXT | NO | '' | 경량 점검 시각 |
| last_wake_at | TEXT | NO | '' | 깨우기 시각 |
| usage | TEXT | NO | '' | 사용량 메모 |
| last_job_at | TEXT | NO | '' | 마지막 작업 시각 |
| created_at | TEXT | NO | — | RFC3339 |
| updated_at | TEXT | NO | — | RFC3339 |

### A-12. review_decisions — C 판정 기록 (감사 추적)

| 컬럼 | 타입 | NULL | 기본값 | 제약·설명 |
|---|---|---|---|---|
| id | INTEGER | NO | 자동 | PK AUTOINCREMENT |
| content_id | INTEGER | NO | — | FK → contents(id) ON DELETE CASCADE, 인덱스 |
| decider | TEXT | NO | '' | 판정 작업자명(claim 주체) |
| decision | TEXT | NO | '' | CHECK IN ('','approved','rejected') |
| reason | TEXT | NO | '' | 판정 사유(반려 시 재생성 피드백으로 재사용) |
| created_at | TEXT | NO | — | RFC3339 |

### A-13. 플랫폼 → 채널군 매핑 (코드 확정값)

- official_api: instagram·facebook·threads·tiktok·youtube·blogger.
- manual: naver_blog·tistory.
- 그 외 플랫폼은 미확인 취급하고 예약·판정에서 거부한다.

### A-14. 콘텐츠 상태 전이표 (코드 확정값)

- 전진(한 단계씩만): draft → review → approved → queued → published.
- 역행(한 단계씩만): review → draft, approved → review, queued → approved.
- 건너뛰기·두 단계 역행은 거부(`transition A -> B is not allowed`, 400).
- queued → published 직접 전이는 API에서 409로 차단한다. published 도달은
  수동 발행 완료 기록(queued+예약+URL 증거)·실행기(P1-1)·publish 작업 결과(P1-2)뿐이다.
- 플랫폼 경로 예외(구현됨): 자동 모드 예약 시 queued까지 자동 전진, C 승인+목적지 시 queued 직행.
- 작업자 경로는 상태를 바꾸지 않는다(초안 생성만). 상태 전이는 플랫폼(사람 검수·C 판정)만 한다.

마이그레이션 메모: app_meta 테이블에 schema_version을 기록한다(현재 "5").
기동 시 멱등 실행한다. v2는 사업 컬럼 추가, v4·v5는 jobs 타입 CHECK 확장을 위한
테이블 재구축이다(jobs를 가리키는 FK가 없어 재구축이 안전하다).

## B. API 스키마 부록 (58종)

### B-0. 공통 규칙 (코드 확정값)

- 기준 경로: `/api`. REST+JSON. 시각은 RFC3339 문자열.
- 인증 2종. 운영자: `Authorization: Bearer <로그인 토큰>`(12시간, 미들웨어 검사).
  작업자: `Bearer <WORKER_TOKEN>` + `X-Worker-Name`(미지정 시 resident-claude로 기록).
- 오류 형식: `{ "error": "메시지" }` + HTTP 상태 코드. 목록이 비면 `[]`(`null` 금지).
- ID·비밀번호 평문은 어떤 응답에도 나가지 않는다(저장 여부만).
- 목록 상한: 상품·콘텐츠·발행 기록·지표 200건, 작업 최근 100건.

### B-1. 공개 2종

| 메서드·경로 | 응답 | 설명 |
|---|---|---|
| GET /api/health | 200 `{"ok":true,"time":"..."}` | 생존 확인 |
| POST /api/auth/login | 아래 | 운영자 로그인 → 토큰 |

요청: `{"username":"admin","password":"..."}`.
응답 200: `{"token":"...","user":{"id":1,"username":"admin","role":"admin"}}`
(최초 시드 운영자는 role `admin`, POST /api/users로 추가한 운영자는 `operator`).
오류: 400 (본문 파손·필드 누락), 401 (자격증명 불일치).

### B-2. 작업자 4종 (Bearer WORKER_TOKEN)

인증 실패: 401 `worker token required`·`invalid worker token`.

**GET /api/worker/jobs/next?type=**
가장 오래된 대기 작업 1건을 원자적으로 claim한다. type 필수(소문자화 후 검사).
허용값: analyze·strategy·generate·review. 그 외·누락은 400.
대기 없음: 200 `{"job":null,"message":"대기 중인 작업이 없습니다"}`.
성공: 200 `{"job": JobView}`. JobView =
`{id,type,refId,status,workerName,payload,createdAt,claimedAt,finishedAt,error}`
(payload는 JSON 객체, 빈값이면 `{}`).

**GET /api/worker/jobs** → 200 `{"jobs":[JobView, ...]}` (최신순 100건).

**POST /api/worker/jobs/{id}/result** (비멱등)
타입별 요청 본문:

- analyze (8문자열, 전부 string이어야 함):
  `{"product_name":"","price_summary":"","options_text":"","shipping_text":"",`
  `"detail_core":"","review_summary":"","image_desc":"","unknowns":""}`
  효과: 판독 저장(upsert) + strategy 작업 자동 생성.
- strategy: `{"angle":"(필수)","channels":"","key_messages":"","schedule_note":""}`
  효과: 전략 저장(upsert) + generate 작업 자동 생성. 빈 angle은 400.
- generate: `{"contents":[{"channel":"","title":"","body":"","virtual_persona":false}]}`
  허용 채널 4종(instagram·shortform·cardnews·blog)만 채택하고 나머지는 버린다.
  본문 빈 항목도 버린다. 빈 contents·전부 불가 채널은 400.
  효과: 전부 draft로 저장(출구 B면 대가 표시 자동 삽입, 가상인물이면 AI 라벨 부착)
  + review 작업 1건 자동 생성(페이로드 `{"product_id":N,"content_ids":[...]}`).
  응답 200: `{"job":JobView,"drafts":채택수}`.
- review: `{"decisions":[{"content_id":N,"approve":true,"reason":"",`
  `"account":"","platform":"","scheduledAt":""}]}`
  규칙: decisions 비어 있으면 400. content_id는 양수. account·platform은 함께
  있거나 함께 없어야 한다(한쪽만 있으면 400). 작업 페이로드의 content_ids
  범위를 벗어나면 400(레거시 빈 페이로드는 전부 허용).
  효과(판정 전수 검증 후 일괄 적용, 판정자·사유 전부 기록):
  승인+목적지 → queued·예약·발행 기록(chief-approved).
  승인만 → draft·review는 approved까지 전진.
  반려 → draft·approved는 review로, queued는 approved로 되돌리고,
  반려 누적 2회까지 generate 재생성(피드백 동봉), 3회째는 사람이 맡는다.
  published 건은 기록만 하고 손대지 않는다.
  저장 verdict enum: `approved`·`rejected` (DDL CHECK 확정값).
  예약 시각은 RFC3339 미래 시각만 허용하고 UTC로 정규화한다.

공통 오류: 400 (완료건 재등록 `result already registered...`·검증 실패·JSON 파손),
404 (작업 없음), 409 (미claim 작업 `job must be claimed before result registration`).

**POST /api/worker/jobs/{id}/fail** — 요청 `{"error":"실패 사유"}`(필수).
실패 작업은 자동 재등록되지 않는다. 예외(W9-5): 발행 작업의 `PRE_PUBLISH`
사유(로그인 진입·셀렉터 미발견)는 1회에 한해 pending으로 돌아가 다음 폴에
자동 재시도되고(`attempts=1`, 사유는 error에 보관), 2회째는 실패로 닫힌다.
`UNCERTAIN_POST`와 코드 없는 발행 실패는 실패로 닫히고 게시물을 보류한다
(중복 게시 방지, confirm-published로 사람 확인). 보류 사유는
`hold_reason`에 기록된다. 400 (사유 누락·완료건), 404 (작업 없음).
응답 200: `{"job":JobView}`.

체인 요약: 상품 등록 → analyze → 판독 저장 → strategy → generate → draft →
review → queued·예약. 사람은 검수·승인·예외·감사 담당이다.

### B-3. 운영자 52종 (Bearer 로그인 토큰)

공통 인증 오류: 401 `operator login required`·`invalid or expired session`.
`{id}` 자리 숫자가 아니면 400. 아래 오류는 인증 통과 후의 것이다.

**인증·사용자 6종**

| 메서드·경로 | 요청 | 응답·오류 |
|---|---|---|
| GET /api/auth/me | — | 200 `{"id":N,"username":""}` |
| POST /api/auth/logout | — | 200 `{"ok":true}`(토큰 파기) |
| PATCH /api/auth/password | `{"current":"","next":""}` | 200 `{"ok":true}`. 401(현 비번 불일치), 400(8자 미만) |
| GET /api/users | — | 200 `{"users":[{"id":N,"username":"","role":""}]}` |
| POST /api/users | `{"username":"","password":""}` | 201 `{"user":{...}}`. 400(빈 이름·8자 미만), 409(중복) |
| DELETE /api/users/{id} | — | 200 `{"ok":true,"deleted":N}`. 409(본인·마지막), 404 |

**상품 11종**

- POST /api/products → 201 `{"product":Product,"analyzeJob":Job}`.
  JSON: `{"sourceUrl":"","exitType":"A","priority":0,"business":""}`.
  멀티파트도 허용(sourceUrl·exitType·business·priority 필드 + images 파일).
  제한: 이미지 최대 10장·장당 10MB, 확장자 jpg·jpeg·png·gif·webp.
  오류 400: `exitType must be A or B`·`sourceUrl must be an http(s) URL`·
  `priority must be >= 0`·이미지 제한·지원 외 확장자.
- GET /api/products?business= → 200 `{"products":[Product,...]}` (최신순 200건,
  사업 필터 NOCASE).
- GET /api/products/{id} → 200 Detail =
  `{"product":Product,"images":[],"reading":Reading|null,"strategy":Strategy|null,`
  `"jobs":[Job,...],"affiliateLinks":[]}`. 404.
- PATCH /api/products/{id} →
  `{"name":"","sourceUrl":"","exitType":"","priority":0,"business":"","industry":""}` → 200 Product.
  industry는 `cosmetics|health|food`(금칙어 사전 선택), `general`(일반으로 해제), `""`(유지).
  sourceUrl은 비울 수 있다(주제·이미지 입력 건). 400(출구·URL·우선순위·업종), 404.
- PATCH /api/products/{id}/reading → 판독 8필드(camelCase) → 200 Reading. 404.
- PATCH /api/products/{id}/strategy →
  `{"angle":"","channels":"","keyMessages":"","scheduleNote":""}` → 200 Strategy. 404.
- DELETE /api/products/{id} → 200 `{"ok":true,"deleted":N,"imagesRemoved":N}`.
  하드 삭제(하위 전체 + 업로드 파일). 404.
- DELETE /api/products/{id}/images/{imageId} → 200 `{"ok":true,"deleted":N}`. 404.
- POST /api/products/{id}/affiliate-links →
  `{"partnerName":"","url":""}` → 201 `{"affiliateLink":{...}}`. 400(URL), 404.
- PATCH /api/products/{id}/affiliate-links/{linkId} → 200 `{"affiliateLink":{...}}`.
  400(URL), 404(상품 불일치 포함).
- DELETE /api/products/{id}/affiliate-links/{linkId} → 200 `{"ok":true,"deleted":N}`. 404.

**콘텐츠 7종**

- GET /api/contents?status=&business= → 200 `{"contents":[Content,...]}` (최신순 200건).
- GET /api/contents/{id} → 200 Content. 404.
- PATCH /api/contents/{id} →
  `{"title":"","body":"","virtualPersona":false,"status":""}` → 200 Content.
  제목·본문·가상인물 플래그 수정 + 상태 전이(A-14). 대가 표시(disclosure)는
  이 API로 변경 불가(구조적). status=published는 409
  (`actual publishing is tranche 2; status stays queued`).
  오류: 400(허용 외 전이), 404.
- DELETE /api/contents/{id} → 200 `{"ok":true,"deleted":N}`(기록·지표 포함 삭제). 404.
- POST /api/contents/{id}/schedule →
  `{"account":"슬롯 라벨","platform":"","scheduledAt":"RFC3339 또는 빈값"}` →
  200 `{"content":Content,"queuePosition":N}`.
  queued 건은 직접 예약. 그 전 단계는 자동 모드 계정에 한해 queued까지 자동 전진,
  수동 모드는 거부. 과거 시각·미지정 계정·미확인 플랫폼은 400. 404.
- POST /api/contents/{id}/publish-record → `{"url":"발행된 URL"}` → 200 Content(published).
  조건: queued + 예약됨 + URL 증거. 404, 409(이미 기록됨),
  400(미queued·미예약·URL 불량).
- GET /api/contents/{id}/reviews → 200 `{"reviews":[ReviewDecision,...]}`. 404.

**발행 계획·큐 2종**

- GET /api/publish/plan → 200 `{"items":[PlanItem,...]}` (official_api 다음 manual 순).
  PlanItem =
  `{contentId,title,account,platform,group,scheduledAt,session,mode,verdict,reason}`.
  판정 순서(코드 확정값): 보류 → held. 미등록 계정 → unknown_account.
  세션 미연결 → blocked_session. 수동 모드 → manual. 미예약 → ready.
  미래 시각 → scheduled_future. 도래 → ready. 읽기 전용(상태 불변).
- GET /api/publish-queue?group= → 200 `{"group":"","items":[Content,...]}`.
  group 필수(official_api·manual). 혼합 조회 없음. 그 외는 400.

**발행 기록 4종**

- GET /api/publish-logs → 200 `{"logs":[PublishLog,...]}` (200건).
- POST /api/publish-logs →
  `{"contentId":N,"channel":"","account":"","status":"","note":""}` → 201 PublishLog.
  status는 scheduled·held만(그 외 409). 삼중항 중복 409. 콘텐츠 없음 404.
- PATCH /api/publish-logs/{id} → `{"status":"","note":""}` → 200 PublishLog.
  상태 범위 동일(scheduled·held 외 409). 404.
- DELETE /api/publish-logs/{id} → 200 `{"ok":true,"deleted":N}`. 404.

**지표 4종**

- GET /api/metrics → 200 `{"metrics":[Metric,...]}` (200건).
- POST /api/metrics →
  `{"contentId":N|null,"channel":"","exposures":N|null,"inflows":N|null,`
  `"orders":N|null,"inquiries":N|null,"note":""}` → 201 Metric.
  4종 전부가 0·NULL이면 causeClass에 원인 미분류 자동 기재.
- PATCH /api/metrics/{id} →
  `{"checkUrl":false,"checkSession":false,"checkDisplay":false,"note":""}` →
  200 Metric. 3단계 전체 교체(부분 아님). 404.
- DELETE /api/metrics/{id} → 200 `{"ok":true,"deleted":N}`. 404.

**작업(운영자용) 3종**

- GET /api/jobs → 200 `{"jobs":[JobView,...]}` (작업자 목록과 동일, 최신순 100건).
- POST /api/jobs/{id}/cancel → `{"reason":""}`(생략 시 `cancelled by operator`) →
  200 `{"job":JobView}`(실패 처리, 재등록 없음). 완료건 409, 없음 404.
- DELETE /api/jobs/{id} → 200 `{"ok":true,"deleted":N}`. 404.

**세션·슬롯 11종**

Account 응답형 =
`{id,name,platform,owner,role,business,sessionState,postingMode,`
`hasCredentials,lastCheckedAt,lastWakeAt,usage,lastJobAt}`.
평문 자격증명은 어떤 응답에도 없다.

- GET /api/session/accounts → 200 `{"accounts":[Account,...]}`.
- POST /api/session/accounts →
  `{"name":"슬롯 라벨","platform":"","owner":"","role":"","business":"",`
  `"loginId":"","password":""}` → 201 Account.
  오류: 400(이름 누락·역할 오기·자격증명 한쪽 누락), 409(이름 중복).
  역할은 chief·worker만. 자격증명은 둘 다 있을 때만 저장된다.
- PATCH /api/session/accounts/{id} →
  `{"sessionState":"","usage":"","owner":"","role":"","business":"",`
  `"postingMode":"","loginId":"","password":""}` → 200 Account.
  세션 상태는 5종만(그 외 400). expired·limited·inactive_hold로 바꾸면 해당 계정 queued건 보류.
  expired·limited에서 connected로 돌리면 재개(1일 이상 경과건은 사유 갱신 후 보류 유지).
  inactive_hold에서 connected로 돌리면 자동 재개하지 않는다. 발행 모드 auto·manual.
  자격증명은 둘 다 줄 때만 교체(한쪽만이면 400). 이름 변경 필드 없음(D6).
- DELETE /api/session/accounts/{id} → 200 `{"ok":true,"deleted":N}`.
  queued 참조가 하나라도 있으면 409. 404.
- POST /api/session/check → 200 `{"checked":N}` (경량 점검, 타임스탬프만).
- POST /api/session/accounts/{id}/wake → 200 Account. 404.
- GET /api/session/relogin-queue → 200 `{"items":[{accountId,accountName,owner,`
  `sessionState,heldCount}]}` (만료·한도 계정만).
- GET /api/session/slots → 200 `{"slots":[{id,label,agentName,accountId}]}`.
  기동 시 A·B·C 초기화(인메모리).
- POST /api/session/slots → `{"label":"","agentName":""}` → 201 Slot.
  400(라벨 누락·라벨 중복).
- PATCH /api/session/slots/{id} → `{"label":"","agentName":""}` → 200 Slot.
  400(라벨 중복), 404.
- DELETE /api/session/slots/{id} → 200 `{"ok":true,"deleted":N}`. 404.

**대시보드·시스템 4종**

- GET /api/dashboard → 200
  `{"counts":{reviewWaiting,jobsPending,jobsClaimed,jobsFailed,jobsStuck,`
  `queuedTotal,heldTotal,scheduledToday},"today":[Content,...],`
  `"byBusiness":[{business,products,queued,review}],"reloginNeeded":[...]}`.
  오늘 일정은 로컬일 자정~자정을 UTC 반개구간으로 환산해 조회한다.
  지연(jobsStuck)은 claim 후 30분 경과건이다.
- POST /api/system/backup → 200 `{"ok":true,"dest":"..."}`.
  대상: DB(+WAL) + 금고 키 + 업로드 폴더. 세대 보관(기본 7).
- GET /api/system/backups → 200 `{"backups":["...",...]}` (최신순).
- POST /api/system/purge → 200 `{"ok":true,"jobs":N,"metrics":N}`.
  완료·실패 작업 90일, 지표 180일 경과분 제거. 진행 중·대기는 손대지 않는다.

**대표 예시 3종**

1. 로그인 → 상품 등록:
   `POST /api/auth/login {"username":"admin","password":"..."}` →
   `{"token":"...","user":{...}}`.
   `POST /api/products {"sourceUrl":"https://...","exitType":"A","business":"글로우사인"}` →
   201 `{"product":{"id":1,...},"analyzeJob":{"id":1,"status":"pending",...}}`.
2. 작업자 처리:
   `GET /api/worker/jobs/next?type=analyze` → `{"job":{...,"status":"claimed"}}`.
   `POST /api/worker/jobs/1/result` + analyze 8문자열 → 200 (strategy 자동 생성).
3. C 판정:
   `POST /api/worker/jobs/9/result`
   `{"decisions":[{"content_id":3,"approve":true,"reason":"톤 적합",`
   `"account":"피클-1","platform":"instagram","scheduledAt":""}]}` →
   200 (해당 건 queued + 예약 + 판정 기록).

## C. 화면 명세 복원 (15보기 + 셸)

원 지시서 §8(디자인 v1.8)과 코드 구현을 대조해 풀어쓴다. 코드에 없는 요구는
`미구현`으로 표기한다. 호출 API는 B장의 경로를 따른다.

### C-0. 셸 (App.vue)

- 밝은 사이드바(15메뉴·6그룹) + 상단 바 + 본문. 로고(SOMUN) 클릭 시 `/`(대시보드)로 이동한다.
- 하단에 로그인 표시(사용자명 + 로그인 중, 미로그인 시 설정 안내). 라우트 가드는 없다.
  미로그인에도 화면은 열리고 API 호출이 401로 막힌다. 운영자 로그인은 설정 화면에서만 진입한다.
- 반응형(900px 이하) + PWA(매니페스트·서비스워커·SVG 아이콘).

### C-1. 대시보드 `/` (HomeView)

상태 요약 5종 + 사업별 현황. 호출: GET /api/dashboard.

- 지표 타일: 검수 대기·대기 작업·처리 중 작업·발행 큐·오늘 일정.
- 사업별 현황: 사업·상품·검수 대기·발행 큐 표(미지정은 `(미지정)`).
- 이상 알림: 실패 작업·지연 작업(30분+)·보류 발행·계정별 재로그인 필요를 문장으로 나열.
  없으면 `이상이 없습니다. 평온합니다.`
- 오늘 일정: 시각·ID·제목·계정 목록. 보류 pill 표시.
- 큐 요약: 실패 작업 N건·지연 N건·보류 발행 N건. 재로그인 필요: 계정·보류 건수.

### C-2. 대화 `/chat` (ChatView)

- 상단 고정 문구: `에이전트 슬롯이 늘어도 의장님의 대화 상대는 총괄 1자리로 고정됩니다.`
- 탭 2개. 총괄 탭: 판정·기준·지표 대화창(고정 1창). 작업자 탭: 에이전트별 스레드 목록.
- 로그인 동선(카드 내 단계 전환, 목업): 버튼 → 안내(공식 로그인 화면·의장님 계정·완료 후 활성화)
  → 시연 화면 → 완료 시 대화 활성화. 만료 상태 미리보기 버튼 포함.
- 만료 시 대화보다 재로그인 안내가 먼저 보인다. 시연 응답은 `[총괄 시연 응답]`으로 표식한다.
- 스레드: 실계정 목록(GET /api/session/accounts) 위에 목업 메시지.
  만료·한도 계정에 `새 소식` 배지. 연결 상태가 아니면 읽기 전용 +
  `로그아웃된 에이전트입니다. 스레드는 읽기 전용으로 남습니다.`
- 실화면 로그인 창으로 교체는 P2. 실전송 없음(미구현이 아니라 범위 외).

### C-3. 상품 투입 `/products` (ProductInputView)

- 등록 폼: 상품 링크(http/https 필수)·사업·출구 유형(A 파트너·B 제휴 라디오, 기본 A)·
  우선순위(0 이상)·이미지(최대 10장·장당 10MB). 멀티파트 전송.
- 성공 시 `상품 #N 등록됨 · 판독 작업 #M 자동 생성` 표시.
- 등록 표: ID·사업·이름(판독 전 표시)·링크(새 창)·출구 pill·등록일·삭제.
  사업 필터(전체 사업 + 목록). 삭제는 확인 모달(하위 전체 삭제 경고).

### C-4. 상품 판독 `/readings` (ProductReadView)

- 좌: 상품 선택 목록(`#ID 이름·링크`). 우: 상세. 호출: GET /api/products/{id}.
- 상품 수정: 이름·사업·링크·출구·우선순위 저장(PATCH).
- 이미지 썸네일 + 장당 삭제.
- 판독 결과: 8필드 정의형 표시(미확인 unknowns 분리 강조). 판독 수정 버튼으로
  8필드 폼 전환·저장·취소. 판독 전이면 큐 확인 안내.
- 마케팅 전략(A): 앵글·채널·핵심 메시지·일정 메모 표시 + 수정 폼. 없으면 큐 확인 안내.
- 제휴 링크(출구 B): 목록(제휴처·URL·수정·삭제) + 등록 폼. 삭제 확인 모달.
- 작업 이력: `#ID 종류` + 상태 pill + 작업자 + 오류 사유.

### C-5. 콘텐츠 검수 `/review` (ContentReviewView)

- 필터: 상태(전체·5종)·사업 + 새로고침. 호출: GET /api/contents.
- 좌: 목록(상태 pill·ID·채널·제목·보류 배지). 우: 상세.
- 상세: 대가 표시 박스(자동 삽입·삭제 불가 문구와 함께 읽기 전용 표시),
  AI 가상인물 표시 박스(있을 때만), 제목·본문 수정, 가상인물 체크박스, 본문 저장.
- 상태 전이: 허용된 다음 상태만 버튼으로 노출(역행 1개 + 전진 1개, published 제외).
  queued에서는 `발행(published)은 트랜치 2에서 열립니다` 안내.
- 삭제(확인 모달, 기록·지표 포함 삭제 경고) + 판정 이력(판정자·approved·rejected pill·사유).
  호출: GET /api/contents/{id}/reviews.

### C-6. 발행 큐 `/queue` (PublishQueueView)

- 예약 등록: 콘텐츠 ID·계정(슬롯 라벨)·플랫폼(8종 선택지, API군·직접군 표식)·예정 시각.
  성공 시 직렬 순서 N번 표시. 과거 시각 거부·동시각 직렬 안내 문구 포함.
- 실행 계획(읽기 전용): ID·제목·계정·판정(6종 verdict pill)·사유 표. 새로고침.
- 발행 완료 기록(수동): 콘텐츠 ID + 발행된 URL. queued+예약 조건 안내.
- 발행 기록(plan): 등록 폼(콘텐츠 ID·채널·계정·상태 scheduled·held·메모) +
  표(행 수정·삭제). 삼중항 중복 거부·done·failed 트랜치 2 안내.
- 2군 분리 표: 공식 발행 API군 / 직접 posting군(API 없음) 각각 ID·사업·제목·계정·예정·보류.
  직접군 부제목은 코드에 `네이버 블로그 · 티스토리 — 처리 방향은 PO 결정 대기`로
  표시된다. PO-1이 세션 포스팅 진행으로 해소됐으므로 이 문구는 화면 문구 오류다.
  P0-4에서 `네이버 블로그 · 티스토리 — 세션 포스팅(P1-2)`으로 고친다.

### C-7. 지표 `/metrics` (OpsViews)

- 기록 폼: 콘텐츠 ID(선택)·채널·노출·유입·주문·문의. 0·누락은 원인 미분류 안내.
- 수집 표: ID·콘텐츠·채널·4종 수치(NULL은 —)·판정(causeClass)·판정 흐름 체크 3개
  (①URL실재·②수집경로·세션·③채널표시, 즉시 저장)·삭제.
- 판정 흐름 순서 안내 문구 포함.

### C-8. 에이전트 연결 `/agents` (ConnectViews)

원 지시서 요구(슬롯 + 카드 보기 + 목록 보기 + 필터 + 집계)를 전부 갖춘 화면이다.

- 슬롯 카드: 칸 라벨·에이전트명 타일 + 행별 에이전트명 수정·삭제 + 추가 폼(칸 이름·에이전트명).
  A·B·C는 예시이며 중립 구조다.
- 연결된 계정 요약(카드 보기): 전체 계정·총괄 수·작업자 수·이상 계정 수 4타일.
  이상 = 세션 expired·limited·inactive_hold.
- 계정 목록(목록 보기): 계정·역할 pill·담당 사업·세션 상태 pill·ID·비번(저장됨·—)·
  사용량·마지막 작업 표.
- 필터: 역할(전체·총괄·작업자) + 사업(전체 + 목록) + `이상 계정만` 체크 + 새로고침.
  상단 집계(4타일)는 필터와 무관하게 전체 기준이다.
- 계정 등록: 계정명(슬롯 라벨)·플랫폼·주인·역할·담당 사업·로그인 ID·비밀번호.
  `AES-256으로 암호화 저장되며 화면에 다시 보이지 않습니다` 안내.
- 계정 정보 수정: 계정 선택 + 주인·역할·사업·자격증명 변경.
  `계정명은 큐 연결 키라 바꿀 수 없습니다` 안내(이름 변경 필드 없음, D6).

### C-9. 채널 계정 `/channels` (ConnectViews)

- 표: 계정·플랫폼·세션 상태 pill·발행 모드 pill(자동·수동)·자동 발행 가능 여부·
  마지막 확인 시각·변경 버튼(연결·만료·한도·모드 전환·해제).
  자동 발행 가능 여부는 `가능 (트랜치 2)`·`불가 · 발행 정지`로 표시된다.
- `가벼운 점검 실행` 버튼(POST /api/session/check) + 점검 원칙 안내 문구.
- 재로그인 필요 큐: 계정·주인·보류 건수·세션 pill. 없으면 `없음`.
- 해제 확인 모달. 보류 중이면 해제 거부(실제로는 queued 참조 전체가 가드 대상).
- 화면 상단 부제는 코드에 `아이디·비밀번호는 저장하지 않습니다`로 표시된다.
  D15로 암호화 저장이 확정됐으므로 이 문구는 화면 문구 오류다. P0-4에서
  `로그인 ID·비밀번호는 AES-256으로 암호화 저장되며 다시 표시되지 않습니다`로 고친다.

### C-10. 자체 채널 `/owned` (ConnectViews)

- 체크인데일리 카드(운영 중 자리 표시) + 주식·코인 뉴스 확장 슬롯 카드(확장 예정).
  상세 지표는 지표 화면에서 확인한다는 안내. API 호출 없음.

### C-11. 판정·기준 `/criteria` (OpsViews)

- 모순 6건·미확인 15건·PO 결정 3건(원 지시서 §10 목록)을 대기 표시로 나열.
  확정값 입력란 없음. API 호출 없음(정적 표시).
- PO-3 항목은 `파일럿 발행에 쓸 SNS 계정`으로 표시된다. PO-3 재정의(슬롯 등록+
  로그인 수행)에 따라 문구 오류다. 또한 해소된 항목(PO-1·모순-2·3·6·미확인-14)도 전부 `대기`로
  표시된다. P0-4에서 해소 항목을 `해소` pill로 구분하고 PO-3 문구를 고친다.

### C-12. Muse 상주 관리 `/resident` (OpsViews)

- `실제 Muse 연동은 트랜치 2` + `상주 연결: 대기 (트랜치 2 범위)` 안내.
- 작업 표(최근 100건): ID·종류·상품·상태 pill·작업자·사유·생성·행 버튼.
  pending·claimed에만 취소 버튼, 삭제는 전 상태. 호출: GET /api/jobs,
  POST /api/jobs/{id}/cancel, DELETE /api/jobs/{id}.
- `실패한 작업은 자동 재등록되지 않습니다` 안내.

### C-13. 시스템 구성 `/system` (OpsViews)

- 정기 백업: 보관 세대 수 + 대상·주기 안내 + `지금 백업 실행` + 결과 메시지 + 세대 목록.
- 보관 정리: 90일·180일·진행 중 손대지 않음 안내 + `지금 정리 실행` + 결과 메시지.
  호출: POST /api/system/purge.
- 구성 메모 3줄(SQLite 단일 층·주문 제외·GCP 미사용).

### C-14. 모바일 앱 `/mobile` (OpsViews, PWA 안내)

- 설치 방법 3단계(접속 → 앱 설치·홈 추가 → 독립 창 실행) + 알아둘 점
  (오프라인 껍데기 캐시·외부 접속 Tunnel). API 호출 없음.
- 메뉴명은 `모바일 앱`이나 내용은 PWA 안내다. 네이티브 하단 5탭 설계는 폐기됨.

### C-15. 설정 `/settings` (SettingsView)

- 운영자 로그인 폼(미로그인 시) / 로그인됨 표시 + 로그아웃.
- 운영자 목록(아이디·역할·삭제, 본인 제외) + 추가 폼(8자+ 안내).
- 비밀번호 변경(현재·새 8자+) + 결과 메시지.
- 안내 3줄(환경변수 기본값·12시간 만료·본인·마지막 삭제 금지).

## D. 인수 기준 10개 복원 (원 지시서 §13)

원 지시서 §13의 항목을 인수 프로브·코드로 확인해 복원한다. done의 정의는
이 10개로만 판단하며, `프로브 회귀` 한 줄로 축약하지 않는다. 1~9번은 공개
HTTP API로 확인하고, 10번은 사람이 브라우저를 눌러 확인한다.

1. **자동 연쇄**: 상품 등록 시 analyze 작업이 pending으로 자동 생성되고,
   판독 저장 후 strategy → generate가 순서대로 자동 생성되는가.
   확인: POST /api/products → analyzeJob 확인 → analyze 결과 등록 →
   strategy 조회·결과 등록 → generate pending 확인.
   실패 시 판정: 연쇄 누락. 해당 Submit 함수의 자동 생성 분기가 깨진 것이다.
2. **claim 단일 승자**: 병렬 4건 이상의 동시 claim에서 같은 작업이
   중복 배정되지 않는가.
   확인: pending 1건에 4개 스레드 동시 GET next → 승자 1건, 나머지는 null.
   실패 시 판정: 중복 배정. claim 원자성이 깨진 것이다.
3. **완료건 재등록 400**: done인 작업에 결과를 다시 등록하면 400인가.
   확인: 결과 등록된 작업에 POST result → 400 `result already registered...`.
   실패 시 판정: 비멱등 위반.
4. **실패 미재등록**: 실패 등록한 작업이 자동으로 다시 큐에 오르지 않는가.
   확인: POST fail 후 next를 반복 조회해도 해당 ID가 나오지 않는다.
   실패 시 판정: 재등록 버그.
5. **미확인 분리·지어 채우기 금지**: analyze 결과에서 확인되지 않은 항목이
   unknowns로 분리되고, 빈 필드를 지어서 채우지 않는가.
   확인: price_summary 빈값 + unknowns 기재로 등록 → 판독 조회에서 그대로 확인.
   실패 시 판정: 분리 위반.
6. **상태 전이 규율**: 허용 전이와 한 단계 역행만 되고, 건너뛰기·두 단계
   역행이 거부되는가. 작업자 경로로는 상태가 바뀌지 않는가.
   확인: draft→review 200, review→draft 200, draft→approved 400,
   review→queued 400, approved→draft 400, queued→approved 200,
   queued→published 409, 작업자 토큰으로 PATCH 시 401.
   실패 시 판정: 전이표 위반·차단 누락.
7. **대가 표시 자동 삽입·삭제 불가**: 제휴(출구 B) 콘텐츠에 대가 표시가
   제목 또는 첫 부분 상당 위치에 자동 삽입되고, 검수 화면에서 삭제할 수 없는가.
   확인: 출구 B 상품 → generate → 초안의 disclosure 비어 있지 않음 →
   제목·본문 수정 후에도 disclosure 유지.
   실패 시 판정: 표시 누락·삭제 가능 버그.
8. **계정별 보류 격리**: 슬롯을 세션 만료 상태로 두면 해당 계정의 발행 큐건만
   보류로 바뀌고 다른 계정은 영향이 없는가.
   확인: accA·accB에 예약 → accA 만료 → accA건만 held, accB건 정상 →
   재로그인 큐에 accA만 → 재연결 시 보류 해제.
   실패 시 판정: 격리 위반·재개 누락.
9. **예약 규율**: 과거 시각 예약이 입력 단계에서 거부되고, 같은 계정·같은
   시각 예약 2건이 적재 순서대로 직렬로 큐에 서는가.
   확인: 과거 시각 POST schedule → 400. 같은 계정·시각 2건 →
   queuePosition 1·2 순서대로.
   실패 시 판정: 과거 허용·순번 위반.
10. **화면 15개·대화 동선**: §8의 메뉴 15개가 전부 열리고, 대화 화면의
    로그인 동선(카드 내 단계 전환: 버튼→안내→시연 화면→활성화, C-2)과 재로그인 우선 안내가
    상태대로 보이는가. 원 지시서의 모달·팝업 표현은 트랜치 1 구현에서 카드 내 단계로 대체됐다.
    확인: 사람이 브라우저에서 15경로를 눌러 열고, 대화 화면에서 로그인 단계
    전환·만료 미리보기·재로그인 우선 안내를 눈으로 확인한다. 자동화 불가 항목.
    실패 시 판정: 화면 미달. 열리지 않는 경로·빠진 동선을 기록한다.

## E. 용어·매핑 표

| 문서 용어 | 코드 용어 | 의미·규칙 |
|---|---|---|
| 슬롯 라벨 | RegisterAccount name · contents.target_account · publish_logs.account | 등록 시 사용자가 붙이는 이름(예: 피클-1). 큐 연결 키, 불변(D6), 이름 변경 API 없음 |
| 플랫폼 아이디 | (세션에서 발견·기록, P2 어댑터) | 로그인 후 확인되는 실제 플랫폼 계정. 사전 확정 금지, 사후 기록. 엉뚱한 계정 로그인 검증용 |
| 자격증명 | enc_id·enc_pw · hasCredentials | AES-256-GCM 암호문으로만 저장. API·화면은 저장 여부만 |
| 채널군 | channel_group (official_api·manual) | A-13 매핑. 혼합 조회 없음 |
| 작업 종류 | jobs.type | analyze·strategy·generate·review 4종 고정 |
| 판정 | review_decisions.decision | approved·rejected. 입력은 approve 불리언 + 사유 + 목적지 |
| 실행 판정 | PlanItem.verdict | ready·manual·held·blocked_session·unknown_account·scheduled_future. 읽기 전용 |
| 슬롯 | slots.label (인메모리) | 에이전트 칸. A·B·C는 예시, 중립 구조. P1-0에서 agents로 대체 |
| 에이전트 | agents.label (P1-0) | 상주 AI 작업자 1기. 라벨 불변, 토큰으로 인증. 최대 100기 운용 |
| 레인 | agents.lane (A·B·C·P) | A=analyze·strategy, B=generate, C=review, P=publish(직접군) |
| 리스 | jobs.lease_until (P1-0) | claim 유효 기한. 하트비트로 연장, 만료 시 D25 규칙 |
| 어댑터 호스트 | agents.host (P1-0) | 에이전트를 실제로 구동하는 PC. 동시 활성 상한을 가진다 |
| 역할 | role (chief·worker) | 계정 역할. 운영자 role 컬럼은 미집행(1인 전제) |
| 발행 모드 | posting_mode (auto·manual) | 계정별, 기본 수동. 실행기는 자동만 |

PO-3 재정의 반영: PO-3는 `파일럿 슬롯 등록 + 로그인 수행`이다. 실계정명을
사전에 확정하지 않는다. 슬롯 라벨은 등록 시 정하는 내부 키이고, 플랫폼
아이디는 로그인 후에 기록하는 검증값이다.

## PO 운영 중 확정 항목 (부록 제외 목록)

의도된 제외다. 미확인 수치를 확정값으로 만들지 않는다.

- 채널별 콘텐츠 규격(이미지 크기·자수 제한 등 공식 규격)
- 채널별 발행 주기·시간대 정책
- 사업별·채널별 KPI 목표치
- 플랫폼 권장 문구의 확정 문구 채택 여부
- PO-3 파일럿 슬롯의 실제 등록·로그인 수행(사용자가 직접)
- 첫 API 채널의 공식 요구사항(계정 유형·앱 설정·미디어 조건) — 개통 시 확인·기록(기본안 instagram)
- AI 서비스별 사용 한도 — 상수 금지, limited 감지로 처리
- 계정별 발행 최소 간격 값 — 운영자가 계정마다 입력(D29)

## A-15. 스키마 v6~v8 계획 (P0·P1, 미구현)

규칙은 A장과 같다(시각 RFC3339 UTC TEXT, 불리언 INTEGER, 미확인 수치 컬럼 금지).
각 버전 상승 전 자동 백업 1세대(D32).

**v6 (P0-3·P0-4)**

- 신설 notifications:
  `id INTEGER PK AUTOINCREMENT`, `kind TEXT NOT NULL CHECK IN ('job_failed','job_stuck','lease_expired',
  'session_expired','session_limited','publish_failed','backup_failed','overdue_hold','review_human')`,
  `severity TEXT NOT NULL DEFAULT 'warn' CHECK IN ('info','warn','error')`,
  `ref_type TEXT NOT NULL DEFAULT ''`(job·content·account·agent·system), `ref_id INTEGER NOT NULL DEFAULT 0`,
  `dedup_key TEXT NOT NULL`(kind:ref_type:ref_id), `message TEXT NOT NULL`,
  `created_at TEXT NOT NULL`, `read_at TEXT NOT NULL DEFAULT ''`.
  인덱스: UNIQUE(dedup_key) WHERE read_at='' (부분 인덱스), (read_at, id).
- contents 추가: `revision_of INTEGER NOT NULL DEFAULT 0`(직전 원본, 0=최초),
  `revision_root_id INTEGER NOT NULL DEFAULT 0`(사슬 최초 ID). 기존 행은 마이그레이션에서
  revision_root_id=id로 채운다. 인덱스 (revision_root_id).

**v7 (P1-0)**

- 신설 agents:
  `id PK`, `label TEXT NOT NULL UNIQUE`(불변), `lane TEXT NOT NULL CHECK IN ('A','B','C','P')`,
  `host TEXT NOT NULL DEFAULT ''`, `businesses TEXT NOT NULL DEFAULT ''`(쉼표 목록, 빈값=전체, NOCASE 비교),
  `platforms TEXT NOT NULL DEFAULT ''`, `capacity INTEGER NOT NULL DEFAULT 1`(1 이상),
  `enabled INTEGER NOT NULL DEFAULT 1`, `token_hash TEXT NOT NULL UNIQUE`(SHA-256 hex),
  `token_prefix TEXT NOT NULL`(앞 8자), `account_id INTEGER NULL REFERENCES channel_accounts(id) ON DELETE SET NULL`,
  `last_seen_at TEXT NOT NULL DEFAULT ''`, `created_at`, `updated_at`.
- jobs 재구축: type CHECK에 'publish' 추가. 추가 컬럼 `agent_id INTEGER NOT NULL DEFAULT 0`(비-FK, 이력 보존),
  `business TEXT NOT NULL DEFAULT ''`, `priority INTEGER NOT NULL DEFAULT 0`, `attempts INTEGER NOT NULL DEFAULT 0`,
  `lease_until TEXT NOT NULL DEFAULT ''`. 기존 행은 상품에서 business·priority를 채운다.
  인덱스: (status, type, priority DESC, id), (type, ref_id), (agent_id, status), (status, lease_until).
- channel_accounts 추가: `kind TEXT NOT NULL DEFAULT 'channel' CHECK IN ('channel','agent_service')`,
  `platform_user_id TEXT NOT NULL DEFAULT ''`(로그인 후 사후 기록, PO-3).

**v8 (P1-1)**

- publish_logs 재구축: status CHECK에 'publishing' 추가. 추가 컬럼 `external_url TEXT NOT NULL DEFAULT ''`,
  `external_id TEXT NOT NULL DEFAULT ''`, `error TEXT NOT NULL DEFAULT ''`. UNIQUE 삼중항 유지.
  API 생성·수정은 계속 scheduled·held만 허용한다(publishing·done·failed는 실행 경로 전용).
- channel_accounts 추가: `enc_api_token TEXT NOT NULL DEFAULT ''`(AES-256-GCM),
  `min_interval_min INTEGER NULL`(운영자 입력, NULL이면 auto 전환 거부).

## B-4. API 추가 계획 (P0·P1, 미구현)

| 단계 | 메서드·경로 | 인증 | 요청 → 응답 |
|---|---|---|---|
| P0-3 | GET /api/notifications?unread=1&limit=&offset= | 운영자 | → `{"items":[Notification],"unread":N}` |
| P0-3 | POST /api/notifications/{id}/read | 운영자 | → `{"ok":true}`. 404 |
| P0-3 | POST /api/notifications/read-all | 운영자 | → `{"ok":true,"updated":N}` |
| P1-0 | GET /api/agents?lane=&enabled=&business=&host=&q=&limit=&offset= | 운영자 | → `{"items":[Agent],"total":N}` |
| P1-0 | POST /api/agents | 운영자 | `{label,lane,host,businesses,platforms,capacity,accountId}` → 201 `{"agent":Agent,"token":"(1회)"}`. 400·409(라벨 중복) |
| P1-0 | GET /api/agents/{id} | 운영자 | → Agent + 최근 작업 20건 |
| P1-0 | PATCH /api/agents/{id} | 운영자 | lane·host·businesses·platforms·capacity·enabled·accountId (label 불가) → Agent |
| P1-0 | DELETE /api/agents/{id} | 운영자 | claimed 작업 있으면 409 |
| P1-0 | POST /api/agents/{id}/rotate-token | 운영자 | → `{"token":"(1회)"}`. 이전 토큰 즉시 무효 |
| P1-0 | POST /api/agents/bulk | 운영자 | `{ids:[...], action:"pause"|"resume"|"lane", lane?}` → `{"updated":N}` |
| P1-0 | GET /api/agents/stats | 운영자 | → `{online,offline,paused,backlogByType:{...},done1h,failed1h,topFailing:[...]}` |
| P1-0 | POST /api/jobs/{id}/requeue | 운영자 | failed → pending, attempts 0. 그 외 상태 409 |
| P1-0 | POST /api/worker/heartbeat | 작업자 | `{sessionState?, activeJobs, version}` → `{"ok":true,"enabled":bool}` |
| P1-0 | POST /api/worker/jobs/{id}/heartbeat | 작업자 | → `{"leaseUntil":"..."}`. 본인 claim 아니면 409 |
| P1-1 | GET /api/executor/status | 운영자 | → `{enabled,intervalSec,lastTickAt,running:[...],done24h,failed24h}` |

- 삭제(P1-0): GET/POST /api/session/slots, PATCH·DELETE /api/session/slots/{id} (4종, agents로 대체).
- 변경(P1-0): GET /api/worker/jobs/next에 `wait`(0~LONGPOLL_MAX_SEC) 추가, `type` 생략 가능(레인 타입 전체).
  에이전트 토큰 사용 시 X-Worker-Name은 무시하고 agents.label을 기록한다.
  PATCH /api/session/accounts/{id}에 `kind`·`platformUserId`(P1-0), `apiToken`·`minIntervalMin`(P1-1) 추가.
  `postingMode:"auto"`는 min_interval_min이 없으면 400(P1-1).
- 라우트 수: 58 → 61(P0) → 68(P1-0) → 69(P1-1).
- 신규 환경변수: SOMUN_ENV, BIND_ADDR, BACKUP_MIRROR_DIR(P0) · LEASE_SEC(기본 600), LONGPOLL_MAX_SEC(기본 25),
  DB_READ_CONNS(기본 4)(P1-0) · EXECUTOR_INTERVAL_SEC(기본 0), EXECUTOR_PARALLEL(기본 2), PUBLIC_BASE_URL(P1-1) ·
  ADAPTER_MAX_ACTIVE(기본 2), ADAPTER_IDLE_CLOSE_MIN(기본 10)(P1-2, 어댑터 호스트).

## F. 브라우저 E2E 체크리스트 (인수 10, 총괄 수동 서명)

준비: 새 DATA_DIR로 서버 기동, 운영자 로그인, 테스트 작업자 실행. 각 항목은 통과·실패·메모를 남긴다.

1. 사이드바 15메뉴가 6그룹으로 보이고 각 메뉴 클릭 시 해당 경로가 열린다(15건).
2. 로고 클릭 시 대시보드. 새로고침해도 같은 경로가 유지된다(SPA 폴백).
3. 미로그인 상태 화면 진입 시 401 오류가 안내로 보이고 설정 화면에서 로그인 가능.
4. 상품 투입: 이미지 2장 등록 → `판독 작업 #M 자동 생성` 표시 → 표에 행 추가.
5. 상품 판독: 작업자 처리 후 판독 8필드·unknowns 강조·전략 표시. 판독 수정 저장·취소.
6. 출구 B 상품: 제휴 링크 등록·수정·삭제(확인 모달).
7. 콘텐츠 검수: 상태 필터·사업 필터 동작. 대가 표시 박스 읽기 전용, 본문 수정 후에도 유지.
8. 허용 전이 버튼만 노출, queued에서 published 버튼 없음·트랜치 2 안내.
9. 판정 이력에 판정자·approved/rejected pill·사유 표시.
10. 발행 큐: 과거 시각 예약 거부 메시지, 같은 계정·시각 2건 직렬 순서 1·2.
11. 실행 계획 표에 verdict pill 6종 중 해당 값 표시, 새로고침 동작.
12. 수동 발행 완료 기록: URL 없이 거부, queued+예약 건은 published로 바뀜.
13. 2군 분리 표가 따로 보이고 직접군 부제 문구가 C-6 수정 문구와 같다(P0-4 이후).
14. 지표: 0·빈값 기록 시 `원인 미분류`, 체크 3개 즉시 저장.
15. 에이전트 연결: 계정 등록(자격증명 저장됨 표시, 평문 재표시 없음), 필터·`이상 계정만`.
16. 채널 계정: 만료 전환 → 재로그인 큐 등재·보류 건수, 연결 전환 → 큐에서 사라짐. 부제 문구 C-9 수정본.
17. queued 참조가 있는 계정 해제 시 거부 메시지.
18. 판정·기준: 해소 항목 `해소` 표시, PO-3 문구 수정본(P0-4 이후).
19. Muse 상주 관리: 작업 취소(pending·claimed만)·삭제 동작.
20. 시스템 구성: 지금 백업 → 세대 목록 맨 위에 추가. 지금 정리 → 결과 메시지.
21. 대화: 로그인 단계 전환(버튼→안내→시연→활성화), 만료 미리보기에서 재로그인 안내가 대화보다 먼저.
22. 연결 상태가 아닌 계정 스레드는 읽기 전용 문구 표시.
23. 설정: 운영자 추가(8자 미만 거부)·삭제(본인 삭제 버튼 없음)·비밀번호 변경·로그아웃.
24. 900px 이하 폭에서 레이아웃 깨짐 없음, 가로 스크롤 없음.
25. PWA: 설치 가능, 오프라인에서 셸이 뜨고 데이터 영역은 오류 안내(낡은 API 데이터 표시 없음, P0-4 이후).
26. 알림 뱃지: 작업 실패 유발 → 뱃지 증가 → 읽음 처리 → 감소(P0-3 이후).

서명: 일자·커밋·통과 수/전체·실패 항목 메모. 결과는 PLAN 검증 기록에 남긴다.

## G. 부하 시험 명세 (P1-0, 가상 에이전트 100기)

- 도구: `tools/loadtest.py`(표준라이브러리, 스레드). 서버는 새 DATA_DIR, dev 모드.
- 준비: agents API로 가상 에이전트 100기 등록(레인 A 30·B 40·C 20·P 10, 사업 3종 분산),
  상품 300건 투입(사업 3종, 우선순위 혼합).
- 동작: 각 가상 에이전트가 롱폴 claim(wait=25) → 0.5~3초 무작위 처리 → 결과 등록(B-2 스키마) →
  60초마다 하트비트. 1%는 결과 없이 종료(리스 만료 유도), 2%는 fail 등록.
- 지속: 1시간. 동시에 운영자 대시보드·목록 API를 초당 2회 호출.
- 목표(초기, 실측으로 조정):
  - 중복 claim 0건(작업별 claim 에이전트 1기, 결과 중복 등록 0건).
  - 빈 큐가 아닐 때 claim p95 ≤ 200ms, 작업 생성 → 대기 중 롱폴 깨어남 p95 ≤ 1초.
  - 서버 5xx 0건, 로그의 SQLITE_BUSY 0건.
  - 리스 만료 작업은 1회 재대기 후 처리되고 2회째는 failed + 알림(dedup 적용).
  - 레인 밖 타입·사업 밖 작업 claim 0건.
  - 대시보드·목록 p95 ≤ 1초, 서버 메모리 증가가 시험 후반 30분 동안 평탄.
- 미달 시: 원인 지표(잠금 대기·쿼리 시간)를 기록하고 D26 조정 → 재시험. 그래도 미달이면
  PostgreSQL 이전 판단 자료로 쓴다(설계서 §2-1).

## 변경 이력

- 2026-11-12: 초판. DB 12테이블·API 58종·화면 15보기·인수 10개·용어표.
  화면 문구 오류 3건(C-6·C-9·C-11)을 기록만 하고 코드는 손대지 않음.
- 2026-11-12: 초기 개발 문서로 지정(의장).
- 2026-11-13: 코드 대조 정정(users.role 시드값 admin, 세션 inactive_hold 규칙, 인수 10 동선 표현).
  화면 문구 오류를 P0-4 수정 대상으로 전환. A-15 스키마 v6~v8·B-4 API 계획·F E2E 체크리스트·
  G 부하 시험(에이전트 100기)·용어(에이전트·레인·리스·호스트) 추가.
