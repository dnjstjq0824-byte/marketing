# SOMUN 마케팅 코어 엔진 — AI 자동화 마케팅 툴

마케팅만 하는 시스템. URL·이미지·주제 중 하나를 던지면 AI 팀이 분석→전략→
제작(글·이미지·영상)→검수→발행→성과 수집까지 사람 조작 없이 끝낸다.
사람은 입력·최초 로그인·(선택) 멈춤·삭제·다운로드만 한다.
주문·결제는 파트너 몰에서 일어나며 이 시스템은 주문을 받지 않는다.

문서: [작업지시서](WORK_ORDER.md)(우선) · [기획서](PLANNING.md) · [설계서](DESIGN.md) ·
[상용화 계획](PLAN.md) · [화면 설계서](UI_DESIGN.md) · 본 README(실행·운영).
문서 지위: 초기 개발 문서(의장 지정 2026-11-12). 환경·상황에 따라 개정한다.

현재 범위: 7화면 웹 콘솔(홈·새 작업·작업·미디어·연결·팀·설정) + Go 서버(스키마 v27·
112 라우트) + 팀 실행기 + 세션 어댑터. 입력 3종·자동 배정·세션 발행·성과 수집·
미디어 생성·AI 팀·안전망이 구현돼 있고, 사람 조작 0회 완주를 E2E로 검증한다.
실채널 셀렉터는 테스트 계정 개통 순서대로 확정한다. 모바일은 PWA 확정(Flutter 폐기).

## 스택

- 웹 콘솔: Vue 3.5 + vue-router 4.6 + Vite 6.4 + PWA(매니페스트·서비스워커) (`web/`)
- 백엔드: Go (`backend/`, x/crypto v0.59.0, modernc.org/sqlite v1.62.0)
- 작업자 층: Python 3 표준라이브러리만 (`worker/`, 설치 불필요)
- 어댑터: Python + Playwright (`adapter/`, 별도 venv. 실화면 로그인·채팅·발행·수집)
- 영상 합성: ffmpeg 서브프로세스 호출(Playwright 번들 WebM·시스템 설치 시 MP4, NOTICE 고지)
- DB: SQLite 시작. SQL은 `backend/internal/store` 한 패키지에만 있다.

## 실행

세 창을 연다(프로젝트 루트 기준).

```powershell
# 1. 백엔드 (기본 :8080, dist가 있으면 콘솔까지 서빙). 반드시 루트에서 실행한다.
cd backend
go build -o somun.exe ./cmd/server
cd ..
.\backend\somun.exe
```

```powershell
# 2. 웹 콘솔 빌드 (최초 1회 + 변경 시)
cd web
npm install
npm run build
# 개발 모드: npm run dev → http://localhost:5173/ (API는 :8080으로 프록시)
```

```powershell
# 3. 테스트 작업자 (큐 흐름 검증용. 상주는 아래 install-workers)
cd worker
python worker.py --fake
# --fake 없이 실AI 환경변수 없으면 기동 거부(종료 코드 2). 운영에서는 가짜 결과 거부.

```powershell
# 3b. 팀 실행기 등록 (PC당 1기, 실AI 실행, 로그온 자동 시작)
powershell -File scripts\install-workers.ps1 -ServerUrl http://127.0.0.1:18080 -ChatAccountId 3
# 관리자 권한 불필요(시작 폴더 방식). 토큰은 scripts/workers/*.cmd에만 저장(git 제외).
# 먼저 -DryRun으로 확인: powershell -File scripts\install-workers.ps1 -DryRun
```
- 구형 A/B/C 레인 상주 작업자(`resident-a/b/c.cmd`)는 폐지됐다. 3b 스크립트가
  기존 등록·시작 폴더 바로가기·러너 파일을 함께 정리하고 팀 실행기(레인 T)
  1기를 등록한다. 스키마 v25부터 서버 기동 시 A/B/C 에이전트는 자동 중지되며,
  실행기를 등록하기 전까지 새 작업은 대기한다.

```powershell
# 4. P레인 발행 어댑터 (실화면 세션 포스팅. 최초 1회 설치 후)
adapter\.venv\Scripts\python -m playwright install chromium  # 최초 1회
adapter\.venv\Scripts\python -m adapter.login <계정명> naver_blog  # 최초 1회 직접 로그인
$env:AGENT_TOKEN = "<P레인 에이전트 토큰>"
adapter\.venv\Scripts\python -m adapter.worker_adapter
# 상세: adapter/README.md
```

브라우저에서 `http://localhost:8080/` 을 연다. 운영자 로그인은 설정 화면에서
한다(기본 admin / changeme123).

- ADMIN_USER·ADMIN_PASS는 DB에 운영자가 하나도 없을 때 **최초 1회 시드에만** 쓰인다.
  이미 기동한 적이 있으면 환경변수를 바꿔도 비밀번호는 안 바뀐다. 설정 화면에서 바꾼다.
- 서버 바인드: dev 기본은 모든 인터페이스(`:PORT`)라 공용망에서 띄우지 않는다.
  `SOMUN_ENV=production`에서는 127.0.0.1에만 바인드하고 기본 비밀값이면 기동을
  거부한다(상용 기동 가드). 외부 접속은 Cloudflare 터널로 붙인다(아래 외부 공개 절).

## 개발 착수

작업 순서와 티켓별 변경 위치·인수 기준은 [작업지시서](WORK_ORDER.md) §4·§5를 따른다
(W0~W21, 1→1'→2→3→4단계). 설계 근거는 [설계서](DESIGN.md) D1~D41,
스키마·API 상세는 [부록](APPENDIX.md), 검증은 부록 F(E2E)·G(부하 시험).

## 테스트·게이트

```powershell
cd backend
go vet ./...
gofmt -l .        # 출력이 없어야 한다
go test ./...
```

```powershell
cd worker
python test_worker.py            # 서버 없이 도는 순수 테스트
python test_worker.py --run-once # 실서버 대상 1건 처리
SOMUN_LIVE=1 python -m unittest test_worker  # 실서버 연동 포함
```

```powershell
# 어댑터 테스트 (adapter/.venv 사용)
adapter\.venv\Scripts\python adapter/test_adapter.py
$env:PLAYWRIGHT_E2E='1'; adapter\.venv\Scripts\python adapter/test_adapter.py  # 실브라우저 포함
```

```powershell
# 무인 완주 E2E (격리 서버 + 격리 포트, fresh DB에서만 동작)
powershell -File scripts\start.ps1 -Port 18083 -DataDir "$env:TEMP\somun_e2e"
$env:SOMUN_URL='http://127.0.0.1:18083'; $env:STUB_PORT='18401'
python tools/probes/e2e_auto.py  # 입력 3종 3/3 published
```

```powershell
# 소크 (가상 에이전트 N기·T분, 판정까지)
$env:SOMUN_URL='http://127.0.0.1:18080'
python tools/soak.py --agents 10 --minutes 5
```

## 환경변수

| 변수 | 기본값 | 설명 |
|---|---|---|
| PORT | 8080 | 서버 포트 |
| SOMUN_ENV | dev | dev·production. production은 기본 비밀값이면 기동 거부 |
| BIND_ADDR | dev 빈값(=전체)·production 127.0.0.1 | 리슨 주소 |
| DATA_DIR | data | DB·업로드·백업 루트 |
| WORKER_TOKEN | local-worker-token | 작업자 API 토큰(기본값은 로컬 전용) |
| ADMIN_USER / ADMIN_PASS | admin / changeme123 | 최초 운영자 시드(운영자 0명일 때만 적용) |
| BACKUP_INTERVAL_MIN | 60 | 정기 백업 간격(분), 0이면 스케줄러 끔 |
| BACKUP_KEEP | 7 | 백업 보관 세대 |
| BACKUP_MIRROR_DIR | start.ps1 기본 `$HOME/somun-backup-mirror` | 최신 세대를 이 폴더에도 복사(외부 사본) |
| LEASE_SEC | 600 | claim 리스(초). 1차 만료 1회 복귀, 2차 실패 |
| LONGPOLL_MAX_SEC | 25 | claim 롱폴 상한(초). WriteTimeout보다 작게 유지 |
| DB_READ_CONNS | 4 | 읽기 풀 연결 수(쓰기는 1 고정) |
| EXECUTOR_HEARTBEAT_MIN | 0 | 실행기 틱(분). 0이면 판정만 하고 실행 정지 |
| EXECUTOR_MIN_INTERVAL_MIN | 30 | 같은 계정 연속 발행 최소 간격(분, 0~1440) |
| EXECUTOR_MAX_ACTIVE | 2 | 틱당 최대 처리 건수 |
| CHECK_INTERVAL_MIN | 15 | 세션 경량 점검 간격(분), 0이면 타이머 끔 |
| RETENTION_CHECK_HOURS | 24 | 보관 정리 주기(시간), 0이면 타이머 끔 |
| WEB_DIST_DIR | web/dist | 서빙할 콘솔 빌드 경로 |
| SERVER_URL | http://localhost:8080 | 작업자 접속 주소 |
| WORKER_NAME | resident-claude | X-Worker-Name 미지정 시 기록명 |
| POLL_INTERVAL_SEC | 5 | 작업자 폴링 간격 |
| WORKER_TIMEOUT_SEC | 30 | 작업자 요청 타임아웃(초) |
| AGENT_TOKEN | (비어 있음) | 설정 시 에이전트 모드(레인 라우팅·롱폴·하트비트) |
| LONGPOLL_WAIT_SEC | 20 | claim 롱폴 대기(초) |
| HEARTBEAT_SEC | 60 | 에이전트 생존 신호 간격(초) |
| JOB_HEARTBEAT_SEC | 120 | claim 리스 연장 간격(초) |
| SOMUN_REAL_AI | 0 | 1이면 작업자가 연결된 AI로 실실행(가짜 핸들러 대체) |
| SOMUN_CHAT_ACCOUNT_ID | 0 | 실실행에 쓰는 연결된 AI 계정 id |
| CHAT_TIMEOUT_SEC | 120 | 채팅 요청 타임아웃(초) |

## API

인증 두 갈래: 운영자 화면은 `Authorization: Bearer <로그인 토큰>`,
작업자는 `Bearer <AGENT_TOKEN 또는 WORKER_TOKEN>` + `X-Worker-Name`.
에이전트 토큰은 신원을 토큰으로 확정하고 X-Worker-Name은 무시한다.
production에서는 공유 토큰 경로가 닫힌다(에이전트 토큰만).
오류 형식은 `{ "error": "메시지" }` + HTTP 상태 코드다.
요청·응답 스키마와 에러 코드별 의미는 [부록 B](APPENDIX.md)를 따른다.

| 메서드·경로 | 인증 | 설명 |
|---|---|---|
| GET /api/health | 없음 | 생존 확인 |
| POST /api/auth/login | 없음 | 운영자 로그인 → 토큰 |
| GET /api/auth/me | 운영자 | 세션 확인 |
| POST /api/auth/logout | 운영자 | 세션 파기 |
| PATCH /api/auth/password | 운영자 | 본인 비밀번호 변경 |
| GET/POST /api/users | 운영자 | 운영자 목록·추가 |
| DELETE /api/users/{id} | 운영자 | 운영자 삭제(본인·마지막 제외) |
| GET/POST /api/products | 운영자 | 상품 목록(?business=)·등록(등록 시 analyze 자동 생성) |
| GET /api/products/{id} | 운영자 | 상품+이미지+판독+작업+제휴링크 |
| PATCH /api/products/{id} | 운영자 | 상품 수정 |
| DELETE /api/products/{id} | 운영자 | 상품 하드 삭제(하위 전체+업로드 파일) |
| DELETE /api/products/{id}/images/{imageId} | 운영자 | 이미지 1장 삭제 |
| DELETE /api/contents/{id} | 운영자 | 콘텐츠 하드 삭제(기록·지표 포함) |
| GET /api/contents/{id}/reviews | 운영자 | 판정 이력 조회 |
| GET /api/contents?status=&business= | 운영자 | 콘텐츠 목록 |
| GET /api/contents/{id} | 운영자 | 조회 |
| POST /api/works/{id}/posts/{postId}/confirm-published | 운영자 | 올라가 있음/안 올라감 확인 |
| GET /api/publish-logs | 운영자 | 발행 기록 목록 |
| GET /api/metrics | 운영자 | 지표 목록(자동 수집분은 origin=collect:24h/72h/7d) |
| GET /api/jobs | 운영자 | 작업 최근 100건(실패 확인용) |
| GET /api/dashboard | 운영자 | 집계+오늘 일정+사업별 현황+재로그인 큐 |
| GET /api/notifications?unread=&limit=&offset= | 운영자 | 알림 발신함 + 미읽음 수 |
| POST /api/notifications/{id}/read | 운영자 | 알림 1건 읽음(없으면 404) |
| POST /api/notifications/read-all | 운영자 | 미읽음 전체 읽음 |
| POST /api/system/backup | 운영자 | 수동 백업(DB+금고 키+업로드+암호화 세션) |
| GET /api/system/backups | 운영자 | 백업 세대 목록(최신순) |
| GET /api/system/health | 운영자 | 상태 진단(DB·디스크·실행기·PC별 실행·팀별 대기·최근 오류 10건) |
| POST /api/system/purge | 운영자 | 보관 정리 수동 실행(완료 작업 90일·지표 180일·AI 원문 90일) |
| GET/POST /api/session/accounts | 운영자 | 세션 계정 목록·등록(ID·비번 암호화 저장) |
| PATCH /api/session/accounts/{id} | 운영자 | 세션 상태(보류·재개)·발행 모드·일일상한·주인·역할·사업·자격증명·종류·플랫폼ID 변경 |
| GET/PUT /api/settings/automation | 운영자 | 자동화 설정 조회·저장(간격·상한·시간대·분량·기본값·보관) |
| GET/PUT /api/automation | 운영자 | 자동화 켜기/끄기(셸 칩) |
| GET /api/system/version | 운영자 | 앱·DB 스키마 버전 |
| POST /api/session/accounts/{id}/login-start | 운영자 | 로그인 브라우저 열기(이미 진행 중이면 409) |
| POST /api/session/accounts/{id}/login-finish | 운영자 | 로그인 확정·세션 검증·연결 전환 |
| POST /api/session/accounts/{id}/login-cancel | 운영자 | 진행 중 로그인 취소(멱등) |
| POST /api/chat/send | 운영자 | AI 계정으로 1건 전송 → 실답장(미연결 409·미지원 501, 느리면 202+폴링) |
| GET /api/chat/pending?afterId= | 운영자 | 백그라운드 전송 결과 폴링 |
| GET /api/chat/history?accountId=&limit= | 운영자 | 대화 기록(오래된 순) |
| POST /api/worker/chat/send | 작업자 | 작업자용 전송(같은 본문, 202 시 폴링) |
| GET /api/worker/chat/pending?afterId= | 작업자 | 작업자용 결과 폴링 |
| GET /api/session/relogin-queue | 운영자 | 재로그인 필요 목록 |
| DELETE /api/session/accounts/{id} | 운영자 | 계정 해제(큐 참조 있으면 거부) |
| GET/POST /api/agents | 운영자 | 에이전트 목록(필터·페이지)·등록(토큰 1회 표시) |
| GET/PATCH/DELETE /api/agents/{id} | 운영자 | 상세(최근 작업)·수정·삭제(처리 중 있으면 409) |
| POST /api/agents/{id}/rotate-token | 운영자 | 토큰 순환(새 토큰 1회 표시) |
| POST /api/agents/bulk | 운영자 | 일괄 일시중지·재개·레인 변경 |
| GET /api/agents/stats | 운영자 | 온라인·오프라인·중지·시간당 완료·실패·실패 상위 |
| GET /api/worker/jobs/next?type=&wait= | 작업자 | 레인 라우팅 claim(에이전트는 타입 생략·롱폴 가능) |
| GET /api/worker/jobs | 작업자 | 작업 최근 100건 |
| POST /api/worker/jobs/{id}/result | 작업자 | 결과 등록(비멱등, 완료건 재등록 400, 타인 claim 409) |
| POST /api/worker/jobs/{id}/fail | 작업자 | 실패 등록(발행 PRE_PUBLISH만 1회 자동 재시도·UNCERTAIN은 보류, 타인 claim 409) |
| POST /api/worker/heartbeat | 에이전트 | 생존 신호(중지면 enabled=false) |
| POST /api/worker/jobs/{id}/heartbeat | 에이전트 | 리스 연장(타인 claim 409) |
| POST /api/media/uploads | 운영자 | 청크 업로드 시작(이미지·영상·문서, 5MB 청크) |
| PUT /api/media/uploads/{id}/parts/{n} | 운영자 | 조각 업로드(순차) |
| POST /api/media/uploads/{id}/complete | 운영자 | 조립+매직바이트 검증+중복 제거(해시) |
| GET /api/media?kind=&product=&from=&to= | 운영자 | 보관함 목록(업로드·AI 이미지·AI 영상·카드) |
| GET /api/media/{id} | 운영자 | 미리보기 정보(프롬프트·AI계정·게시물·다운로드명) |
| GET /api/media/{id}/file | 운영자 | 원본(첨부명 규칙, X-SHA256 해시 헤더) |
| GET /api/media/{id}/download | 운영자 | /file 별칭 |
| GET /api/media/{id}/thumb | 운영자 | 썸네일(없으면 404) |
| POST /api/media/zip | 운영자 | 일괄 ZIP(최대 50개, 스트리밍) |
| DELETE /api/media/{id} | 운영자 | 삭제(게시물 연결본은 409) |
| GET /api/product-images/{id} | 운영자 | 구방식 상품 이미지(공개 /uploads/ 대체) |
| POST /api/worker/media/uploads 등 3종 | 작업자 | 작업자용 업로드(같은 경로, 로컬 렌더 제출용) |
| POST /api/worker/media/gen | 작업자 | AI 세션 실생성(미확인 플랫폼 409→로컬 폴백) |
| GET /api/media/{id}/signed | 운영자 | 발행 어댑터용 서명 URL(24시간) |
| GET/POST /api/teams | 운영자 | 팀 목록·생성(브랜드 전담팀) |
| GET/PATCH/DELETE /api/teams/{id} | 운영자 | 팀 상세·수정·삭제(팀원·진행 작업은 기본 팀 이관) |
| PUT /api/teams/{id}/brand | 운영자 | 팀 브랜드 정보(소개·말투·금칙어·구매 링크·업종) |
| PUT /api/teams/{id}/members | 운영자 | 팀원 교체(한 번에 한 팀만) |
| POST /api/teams/{id}/members | 운영자 | 팀원 추가 |
| DELETE /api/teams/{id}/members/{accountId} | 운영자 | 팀원 제거 |
| POST /api/teams/{id}/members/{accountId}/roles | 운영자 | 팀원 역할 지정(manual 모드) |
| PUT /api/teams/{id}/channels | 운영자 | 팀 채널 묶음 교체 |
| POST /api/teams/{id}/channels | 운영자 | 팀 채널 추가 |
| DELETE /api/teams/{id}/channels/{accountId} | 운영자 | 팀 채널 제거 |
| POST /api/teams/{id}/leader | 운영자 | 팀장 지정 |
| GET/POST /api/works | 운영자 | 작업 목록·등록(입력 3종: url·image·topic) |
| GET/DELETE /api/works/{id} | 운영자 | 작업 상세(읽기 전용 카드)·삭제 |
| POST /api/works/{id}/pause·/resume | 운영자 | 발행 중지·재개 |
| POST /api/works/{id}/posts/{postId}/hold | 운영자 | 게시물 보류 |
| POST /api/works/{id}/posts:generate | 운영자 | 재생성 요청 |
| GET /api/works/{id}/ai-log | 운영자 | AI와 주고받은 원문(프롬프트·답장 전문, 최근 100건) |
| GET /api/works/{id}/audit | 운영자 | 사람 조작 기록(멈춤·삭제·팀 변경·계정 해제·설정 변경·발행 확인) |
| GET /api/overview | 운영자 | 홈 현황(단계별 건수·최근 발행·조치 필요) |
| GET/POST /api/ai-platforms | 운영자 | AI 플랫폼 목록(기본 5종+사용자 정의)·추가 |
| PATCH/DELETE /api/ai-platforms/{key} | 운영자 | 사용자 정의 AI 수정·삭제 |
| GET /api/channel-platforms | 운영자 | 채널 플랫폼 목록(데이터 관리, 준비 중 포함) |
| PATCH /api/channel-platforms/{key} | 운영자 | 채널 플랫폼 설정 변경 |
| POST /api/chat/threads/reset | 운영자 | AI 계정 대화 새 대화로 초기화(console 스레드) |
| POST /api/session/accounts/bulk | 운영자 | 계정 여러 개 한 번에 등록 |
| POST /api/session/login-queue/next | 운영자 | 로그인 대기열 다음 계정 꺼내기 |
| POST /api/session/accounts/{id}/chat-probe | 운영자 | AI 계정 대화 가능 여부 점검 |
| GET /api/worker/boot-check | 작업자 | 실행기 기동 점검(가짜 결과 차단) |

제거된 수동 경로(W8, §3-3): 발행 기록 수기 CRUD·발행 완료 수기 기록·예약 등록 폼·
콘텐츠 상태 전이·본문 편집·지표 수기 입력·판독·전략 수동 편집·제휴 링크 CRUD·
발행 계획표·작업 취소·재등록·삭제 API는 없다. 사람은 발행 중지·삭제·다운로드만 한다.

## 구조

```
backend/cmd/server      서버 기동·백업/점검/보관 스케줄러
backend/internal/config 환경변수 로드·검증
backend/internal/router 전체 라우트
backend/internal/store  DB 접근층(유일 SQL)·마이그레이션·백업
backend/internal/auth   운영자 로그인·토큰
backend/internal/product 상품 투입·출구유형·제휴링크
backend/internal/queue  작업 큐(analyze·strategy·research·generate·review·publish·media·collect)
backend/internal/scheduler 배정기+수집 스윕(발행 24시간·72시간·7일 collect 자동 생성)
backend/internal/guard  발행 전 검사(유사문서·금칙어·대가 표시·이미지 권리·지시문 차단)
adapter/collect         발행 URL 공개 지표 수집(미확인 플랫폼은 빈 라운드, 비용 0)
backend/internal/media  미디어 보관함·청크업로드·썸네일·AI실생성 브리지
adapter/genmedia        AI 이미지·영상 세션 생성(실세션 확인 전에는 거부)
adapter/card_render     로컬 카드뉴스·썸네일 렌더(Playwright, 비용 0)
adapter/video_compose   로컬 숏폼 합성(ffmpeg, 비용 0)
backend/internal/teams  브랜드 전담팀(팀원·채널·브랜드 정보)
backend/internal/works  작업(입력 3종·읽기 전용 카드·중지·AI 원문)
backend/internal/content 콘텐츠 전이·검수·발행큐·지표
backend/internal/session 세션·발행모드·보류·재로그인큐·일괄 등록
backend/internal/agent 에이전트 등록·팀 배정 claim·리스·롱폴
worker/                 팀 실행기 런타임 + 흐름 검증용 테스트 작업자
web/src                 콘솔 7화면(홈·새 작업·작업·미디어·연결·팀·설정, 구 경로 13종 리다이렉트)
```

## 외부 공개 (Cloudflare Tunnel + Access, 무료 범위)

서버는 production에서 127.0.0.1에만 바인드한다. 외부 접속은
cloudflared 터널이 로컬 포트로 붙는 구조다. 개통 절차(최초 1회):

```powershell
# 1. cloudflared 설치 후 로그인(브라우저 승인 1회)
cloudflared tunnel login
# 2. 터널 생성 + DNS 연결(예: somun.example.com)
cloudflared tunnel create somun-core
cloudflared tunnel route dns somun-core somun.example.com
# 3. 인그레스 설정 파일(~/.cloudflared/config.yml)
#    - hostname: somun.example.com
#      service: http://127.0.0.1:8080
# 4. 실행(포그라운드 확인 후 자동 시작으로 전환)
cloudflared tunnel run somun-core
```

- 콘솔 경로(`/`, `/settings` 등)에는 Cloudflare Access(무료 범위)를 한 겹 더
  걸고, 개통 시점에 과금 0원임을 확인한다.
- 작업자 API(`/api/worker/*`)는 에이전트별 토큰으로 인증한다.
  공유 WORKER_TOKEN 경로는 dev 전용이며 production에서 닫힌다.
- 자동 시작: `powershell -File scripts\install-task.ps1 -DryRun`으로
  명령을 확인한 뒤 `-DryRun` 없이 실행하면 로그온 트리거가 등록된다.
- 업데이트·롤백: `scripts\update.ps1` → `scripts\rollback.ps1`
  (스냅샷→빌드→보존→교체→재기동→헬스 확인).

## 운영 메모

- 백업 대상은 DB 일관 스냅샷(VACUUM INTO)+금고 키+업로드 폴더+
  암호화 세션(`profiles.enc`, 금고 키 AES-256-GCM)이며, 스케줄러가 자동
  수행한다. 세션 포함은 설정 고급에서 끄기 가능(기본 켬). 세션 번들은
  `storage_state.json`·`thread*.json`만 담고 브라우저 캐시는 제외한다.
  수동 실행: 설정 고급의 버튼, 또는 `POST /api/system/backup`.
- 복원: 서버 정지 상태에서 `.\backend\somun.exe restore <세대명>` 실행.
  현 data는 형제 `data-pre-restore-<시각>/`로 옮겨지고 세대가 복사된 뒤
  무결성 검사를 통과해야 끝난다. 세대 목록은 `GET /api/system/backups`.
  다음 기동 때 세션 번들이 `data/profiles`로 돌아오고, 세션 파일이 없는
  계정은 "로그인 필요"로 바뀌어 재로그인 대기열에 오른다.
- 보관 정리(완료 작업 90일·지표 180일·AI 원문 90일)도 스케줄러+수동 버튼으로 돈다.
  진행 중·대기 작업은 지우지 않는다.
- 상태 진단은 설정 고급 한 줄 요약+`GET /api/system/health`(DB·디스크
  여유·실행기 접속·PC별 동시 실행·팀별 대기·최근 오류 10건). 디스크 여유
  10% 미만이면 `disk_low` 알림 1건이 뜬다(읽을 때까지 중복 없음).
- 작업 실패 사유는 내부 오류 대신 화면 문구(UI 문구 사전)로 저장·표시된다.
  예: "connection refused" → "연결이 끊겼습니다".
- 발행 큐는 공식 API군(official_api)과 직접군(manual)으로 분리돼 있고
  섞어서 조회할 수 없다. 직접군은 P레인 어댑터가 세션 포스팅하고,
  API군은 서버 실행기가 발행한다. 실채널 셀렉터는 테스트 계정 개통 후 확정한다.
- 게시 한도 상수(Instagram 100/50 등)는 코드에 두지 않는다.
- 제휴 수수료율 같은 미확인 수치는 모델·화면에 넣지 않는다.
- data/, web/dist/, node_modules/는 커밋하지 않는다.
- 세션 상태를 expired·limited·inactive_hold로 바꾸면 그 계정의 queued 건이 보류된다.
  expired·limited → connected는 자동 재개(1일 이상 경과 건 제외), inactive_hold → connected는
  자동 재개하지 않으므로 보류 건을 사람이 확인한다.
- 서비스워커는 `/api/` 응답을 캐시하지 않는다(네트워크 전용). 공용 PC에서는
  사용 후 사이트 데이터를 지운다.
- 공개 `/uploads/` 경로는 폐지됐다(404). 미디어는 전부 운영자 인증
  `/api/media/*`로만 열고, 구방식 상품 이미지는 `/api/product-images/{id}`다.
- 이미지·영상 생성 2단: 1순위 AI 세션 실생성(기능 있는 계정에만 배정,
  유료 우선은 설정에서 끄기 가능), 2순위 로컬 합성(카드 렌더·숏폼 WebM,
  비용 0). 팀에 생성 기능이 없으면 처음부터 로컬로 간다.
- AI 실생성 개통 절차(플랫폼별, 사람 1회): ① 연결 화면에서 계정 로그인
  ② `adapter/.venv/Scripts/python -m adapter.genmedia <계정> <플랫폼>
  --kind gen_image|gen_video --out <빈폴더> --probe "테스트 프롬프트"` 실행
  ③ 출력 후보 중 원본 다운로드를 확인 ④ 확인된 hook·download 값을
  `adapter/platforms.py`에 고정(image_gen="verified" 등) 후 회귀 실행.
  확인 전에는 서버가 409로 거부하고 글 생성은 멈추지 않는다.
- 로컬 합성은 Playwright 번들 ffmpeg(WebM/VP9)을 쓰고, 시스템 ffmpeg이
  있으면 MP4(H.264)로 만든다. 라이선스는 NOTICE 참조.
- 성과 자동 수집(W7): 발행 후 24시간·72시간·7일에 P레인 collect 작업이
  자동 생성되고, P 어댑터가 발행 URL의 공개 지표(조회·공감·댓글)를 읽어
  metrics에 저장한다(origin=collect:창). 값이 없으면 원인 미분류로 남고,
  같은 사업장의 다음 전략 작업 페이로드에 최근 성과 상위 3건이 실린다.
- 수집 셀렉터 개통 절차(플랫폼별, 사람 1회): ① 실게시 1건 발행
  ② `adapter/.venv/Scripts/python -m adapter.collect <계정> <플랫폼>
  --url <발행URL> --probe` 실행 ③ 후보 셀렉터 중 조회·공감·댓글 값을
  확인 ④ `ADAPTER_SELECTORS_DIR/<플랫폼>.json`에 collect_views·
  collect_likes·collect_comments+collect_verified="verified" 고정 후
  회귀 실행. 확인 전에는 빈 라운드(원인 미분류)로 기록된다.
