# SOMUN 마케팅 코어 엔진 — 트랜치 1 (코어)

마케팅만 하는 시스템. 상품 투입 → 판독 → 콘텐츠 생성 → 검수 → 발행 큐까지를
로컬에서 비용 0원으로 돌린다. 주문·결제는 파트너 몰에서 일어나며 이 시스템은
주문을 받지 않는다.

문서: [기획서](PLANNING.md) · [설계서](DESIGN.md) · [상용화 계획](PLAN.md) ·
본 README(실행·운영).

현재 범위: 트랜치 1(플랫폼 뼈대·상품·판독·콘텐츠·검수·작업 큐·
웹 콘솔 15보기·세션/슬롯 골격·모바일 앱 안내 화면) + 디자인·CRUD 전수·
다사업 차원·발행 모드·PWA 보강. SNS 실등록·실발행, Muse 실연동은
트랜치 2이며 이 리포에 그 코드는 없다. 모바일은 PWA 확정(Flutter 폐기).

## 스택

- 웹 콘솔: Vue 3.5 + vue-router 4.6 + Vite 6.4 + PWA(매니페스트·서비스워커) (`web/`)
- 백엔드: Go (`backend/`, x/crypto v0.59.0, modernc.org/sqlite v1.62.0)
- 작업자 층: Python 3 표준라이브러리만 (`worker/`, 설치 불필요)
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
# 3. 테스트 작업자 (큐 흐름 검증용. Muse 실연동이 아니다)
cd worker
python worker.py
```

브라우저에서 `http://localhost:8080/` 을 연다. 운영자 로그인은 설정 화면에서
한다(기본 admin / changeme123, 환경변수로 변경).

## 테스트·게이트

```powershell
cd backend
go vet ./...
go test ./...
```

```powershell
cd worker
python test_worker.py            # 서버 없이 도는 순수 테스트
python test_worker.py --run-once # 실서버 대상 1건 처리
SOMUN_LIVE=1 python -m unittest test_worker  # 실서버 연동 포함
```

## 환경변수

| 변수 | 기본값 | 설명 |
|---|---|---|
| PORT | 8080 | 서버 포트 |
| DATA_DIR | data | DB·업로드·백업 루트 |
| WORKER_TOKEN | local-worker-token | 작업자 API 토큰(기본값은 로컬 전용) |
| ADMIN_USER / ADMIN_PASS | admin / changeme123 | 최초 운영자 시드 |
| BACKUP_INTERVAL_MIN | 60 | 정기 백업 간격(분), 0이면 스케줄러 끔 |
| BACKUP_KEEP | 7 | 백업 보관 세대 |
| CHECK_INTERVAL_MIN | 15 | 세션 경량 점검 간격(분), 0이면 타이머 끔 |
| WEB_DIST_DIR | web/dist | 서빙할 콘솔 빌드 경로 |
| SERVER_URL | http://localhost:8080 | 작업자 접속 주소 |
| WORKER_NAME | resident-claude | X-Worker-Name 미지정 시 기록명 |
| POLL_INTERVAL_SEC | 5 | 작업자 폴링 간격 |
| WORKER_TIMEOUT_SEC | 30 | 작업자 요청 타임아웃(초) |

## API

인증 두 갈래: 운영자 화면은 `Authorization: Bearer <로그인 토큰>`,
작업자는 `Bearer <WORKER_TOKEN>` + `X-Worker-Name`. 오류 형식은
`{ "error": "메시지" }` + HTTP 상태 코드다.

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
| PATCH /api/products/{id}/reading | 운영자 | 판독 결과 수정 |
| DELETE /api/products/{id} | 운영자 | 상품 하드 삭제(하위 전체+업로드 파일) |
| DELETE /api/products/{id}/images/{imageId} | 운영자 | 이미지 1장 삭제 |
| POST /api/products/{id}/affiliate-links | 운영자 | 제휴 링크 등록 |
| PATCH /api/products/{id}/affiliate-links/{linkId} | 운영자 | 제휴 링크 수정 |
| DELETE /api/products/{id}/affiliate-links/{linkId} | 운영자 | 제휴 링크 1건 삭제 |
| DELETE /api/contents/{id} | 운영자 | 콘텐츠 하드 삭제(기록·지표 포함) |
| GET /api/contents?status=&business= | 운영자 | 콘텐츠 목록 |
| GET/PATCH /api/contents/{id} | 운영자 | 조회·본문수정·상태전이·가상인물 표시 |
| POST /api/contents/{id}/schedule | 운영자 | 발행 예약(과거 시각 거부) |
| GET /api/publish-queue?group= | 운영자 | 군별 발행 큐(official_api·manual 분리) |
| GET/POST /api/publish-logs | 운영자 | 발행 기록(중복 삼중항 거부) |
| PATCH /api/publish-logs/{id} | 운영자 | 기록 상태·메모 수정 |
| DELETE /api/publish-logs/{id} | 운영자 | 기록 삭제 |
| GET/POST /api/metrics | 운영자 | 지표 기록·목록(0·누락은 원인 미분류) |
| PATCH /api/metrics/{id} | 운영자 | 3단계 판정 흐름 체크 |
| DELETE /api/metrics/{id} | 운영자 | 지표 삭제 |
| GET /api/jobs | 운영자 | 작업 최근 100건(실패 확인용) |
| POST /api/jobs/{id}/cancel | 운영자 | 작업 취소(실패 처리, 재등록 없음) |
| DELETE /api/jobs/{id} | 운영자 | 작업 삭제 |
| GET /api/dashboard | 운영자 | 집계+오늘 일정+사업별 현황+재로그인 큐 |
| POST /api/system/backup | 운영자 | 수동 백업 |
| GET /api/system/backups | 운영자 | 백업 세대 목록(최신순) |
| GET/POST /api/session/accounts | 운영자 | 세션 계정 목록·등록 |
| PATCH /api/session/accounts/{id} | 운영자 | 세션 상태(보류·재개)·발행 모드·주인·역할·사업 변경 |
| POST /api/session/accounts/{id}/wake | 운영자 | 배정 세션 깨우기 |
| POST /api/session/check | 운영자 | 경량 점검 수동 실행 |
| GET /api/session/relogin-queue | 운영자 | 재로그인 필요 목록 |
| DELETE /api/session/accounts/{id} | 운영자 | 계정 해제(보류건 있으면 거부) |
| GET/POST /api/session/slots | 운영자 | 에이전트 슬롯 목록·추가 |
| PATCH /api/session/slots/{id} | 운영자 | 슬롯 수정 |
| DELETE /api/session/slots/{id} | 운영자 | 슬롯 삭제 |
| GET /api/worker/jobs/next?type= | 작업자 | 대기 작업 원자적 claim |
| GET /api/worker/jobs | 작업자 | 작업 최근 100건 |
| POST /api/worker/jobs/{id}/result | 작업자 | 결과 등록(비멱등, 완료건 재등록 400) |
| POST /api/worker/jobs/{id}/fail | 작업자 | 실패 등록(자동 재등록 없음) |

## 구조

```
backend/cmd/server      서버 기동·백업/점검 스케줄러
backend/internal/config 환경변수 로드·검증
backend/internal/router 전체 라우트
backend/internal/store  DB 접근층(유일 SQL)·마이그레이션·백업
backend/internal/auth   운영자 로그인·토큰
backend/internal/product 상품 투입·출구유형·제휴링크
backend/internal/queue  작업 큐 (§5 규격)
backend/internal/content 콘텐츠 전이·검수·발행큐·지표
backend/internal/session 슬롯·세션·발행모드·보류·재로그인큐
worker/                 작업자 런타임 + 흐름 검증용 테스트 작업자
web/src                 콘솔 (라우터 15보기·6그룹)
```

## 운영 메모

- 백업 대상은 DB 파일과 업로드 폴더이며, 스케줄러가 자동 수행한다.
  수동 실행: 설정이 아닌 시스템 구성 화면의 버튼, 또는
  `POST /api/system/backup`.
- 발행 큐는 공식 API군(official_api)과 직접군(manual)으로 분리돼 있고
  섞어서 조회할 수 없다. 실제 발행 호출은 없다.
- 게시 한도 상수(Instagram 100/50 등)는 코드에 두지 않는다.
- 제휴 수수료율 같은 미확인 수치는 모델·화면에 넣지 않는다.
- data/, web/dist/, node_modules/는 커밋하지 않는다.
