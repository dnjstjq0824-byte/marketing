# SOMUN 마케팅 코어 엔진 — 트랜치 1 (코어)

마케팅만 하는 시스템. 상품 투입 → 판독 → 콘텐츠 생성 → 검수 → 발행 큐까지를
로컬에서 비용 0원으로 돌린다. 주문·결제는 파트너 몰에서 일어나며 이 시스템은
주문을 받지 않는다.

트랜치 1 범위: 플랫폼 뼈대·상품·판독·콘텐츠·검수·작업 큐·웹 콘솔 15보기·
세션/슬롯 골격·모바일 앱 안내 화면. SNS 실등록·실발행, Muse 실연동,
Flutter 앱은 트랜치 2이며 이 리포에 그 코드는 없다.

## 스택

- 웹 콘솔: Vue 3.5 + vue-router 4.6 + Vite 6.4 (`web/`)
- 백엔드: Go (`backend/`, x/crypto v0.59.0, modernc.org/sqlite v1.62.0)
- 작업자 층: Python 3 표준라이브러리만 (`worker/`, 설치 불필요)
- DB: SQLite 시작. SQL은 `backend/internal/store` 한 패키지에만 있다.

## 실행

세 창을 연다(프로젝트 루트 기준).

```powershell
# 1. 백엔드 (기본 :8080, dist가 있으면 콘솔까지 서빙)
cd backend
go build -o somun.exe ./cmd/server
.\somun.exe
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
| SERVER_URL | http://localhost:8080 | 작업자 접속 주소 |
| WORKER_NAME | resident-claude | X-Worker-Name 미지정 시 기록명 |
| POLL_INTERVAL_SEC | 5 | 작업자 폴링 간격 |

## 구조

```
backend/cmd/server      서버 기동·백업 스케줄러
backend/internal/config 환경변수 로드·검증
backend/internal/router 전체 라우트
backend/internal/store  DB 접근층(유일 SQL)·마이그레이션·백업
backend/internal/auth   운영자 로그인·토큰
backend/internal/product 상품 투입·출구유형·제휴링크
backend/internal/queue  작업 큐 (§5 규격)
backend/internal/content 콘텐츠 전이·검수·발행큐·지표
backend/internal/session 슬롯·세션 골격·보류·재로그인큐
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
