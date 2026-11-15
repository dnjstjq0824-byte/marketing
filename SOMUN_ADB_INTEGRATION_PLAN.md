# SOMUN ADB 통합 설계서 (MuMu12 + ADB Runner)

- 상태: 설계안. 프로덕션 코드 변경 없음 (본 문서만 신규).
- 일자: 2026-11-15. 전제: 아키텍처 검토(R1~R5, B1~B4, 상태머신) 승인안 기준.
- 업데이트: PoC 1~7 판정 기준표 + 24시간 운용 규약 반영. 코드 변경 없음.
- 원칙: 현행 Playwright/큐/세션 코드는 손대지 않고, 신규 코드는 추가(additive-only)로만 붙인다.

## 0. 용어·범위

- Runner: 큐에서 작업을 물어다 실행하는 실행기. 현행=Playwright(파이썬 워커), 신규=MuMu ADB(Go 내장).
- 대상 앱 3종: 피클(`com.pickle.client`), Vooy, 키링(`com.ilevit.pmf.pepper`). 타겟 앱 조작 주체는 각 앱 자체이며, 우리 백엔드는 지시 전달 + 상태 관측만 한다(비침투).
- 본 문서는 PoC 1~3(포트 탐색, 1계정 1포스트 왕복, 72시간 유지)까지의 최소 구조만 다룬다.

## ① 현행 저장소 재사용 모듈 정밀 매핑

| 기능 | 현행 위치 | 핵심 API | ADB 재사용 |
|---|---|---|---|
| 작업 큐 영속 | `internal/store/store.go` | `CreateJob`, `GetJob`, `Job` | 그대로 사용, 타입만 추가 |
| 클레임·리스 | `internal/store/claim.go` | `ClaimRoutedJob`, `ExtendJobLease`, `ReapExpiredLeases` | 그대로 사용 |
| 잡 타입·상태 | `internal/queue/model.go` | `JobPublish`, `StatusPending`~`StatusFailed` | `adb_post` 타입 추가 |
| 워커 HTTP | `internal/queue/handler.go` | `Next`, `Result`, `Fail` | ADB 워커도 동일 호출 |
| 상한 설정값 | `internal/store/settings.go` | `AutomationSettings.DailyCap` | 값 읽기, 집행은 신규 |
| 간격·스태거 | `internal/executor/executor.go` | `MinIntervalMin`, `NextEligibleAt` | 게이트 로직 이식 |
| 세션 상태 | `internal/session/service.go` | `StatePending`~`StateInactiveHold` | 상수 2개 추가 확장 |
| 재로그인 큐 | `internal/session/service.go` | `ReloginQueue`, `ReloginItem` | 사유 필드 1개 추가 |
| 알림 outbox | `internal/notify/service.go` | `Notify`, `JobFailed`, `SessionState` | kind 2개 추가(마이그레이션) |
| 자격 금고 | `internal/session/service.go` | `SetCredentials`, `CredentialsFor` | 그대로 사용(AES-GCM) |
| 발행 진입점 | `adapter/worker_adapter.py` | `publish_handler` | PlaywrightRunner로 승격 |
| 워커 루프 | `worker/worker.py` | `run_loop`, `make_real_handlers` | 핸들러 맵 패턴 차용 |

세부 근거:

- 큐: `Job{ID, Type, RefID, Status, WorkerName, Payload, Result, Error, ...}`(`internal/store/store.go:537`).
  상태 전이는 `pending → claimed → done/failed`(`internal/queue/model.go:21`).
  ADB 작업은 신규 타입 문자열(`adb_post`) 추가로 수용, 기존 5종(`analyze`~`publish`) 불변.
- 클레임: `ClaimRoutedJob(agent, types, leaseSec)`이 원자적 할당 + `ExtendJobLease` 연장 +
  `ReapExpiredLeases` 회수를 담당. ADB Runner는 워커 토큰으로 동일 HTTP를 호출하므로
  데드락·이중실행 방지가 그대로 상속된다. 유일 추가점은 ADB 인스턴스 affinity(어느 기기에서
  실행할지)를 `Payload` JSON에 싣는 규칙뿐이다.
- 상한: `DailyCap`(기본 3)은 값만 저장되고 집행처가 없다(전수 검색 확인). 따라서
  ADB 설계에서 클레임 시점에 직접 집행한다: `GetAutomationSettings` 읽기 → 당일 done 카운트 →
  초과 시 `NextEligibleAt=내일`로 연기. 집행 로직은 나중에 Playwright와 공용 함수로 승격.
- 간격: executor가 `MinIntervalMin`(기본 30) + `publish_logs.next_eligible_at`으로
  동일 계정 간격을 강제한다(D29). ADB Runner는 발행 직전에 동일 게이트를 호출한다.
- 세션: 5상태(`pending/connected/expired/limited/inactive_hold`) + `ReloginQueue() []*ReloginItem`.
  ADB용 `challenge`(캡차·2FA·팝업 감지됨)·`parked`(인간 대기) 2상태를 상수로 추가한다.
  상태머신 표기 매핑: ACTIVE=`connected`, CHALLENGE=`challenge`, PARKED=`parked`,
  RECOVERING=복구 시도 중 일시 상태(DB 미저장), COOLDOWN=서킷브레이커 차단(DB 미저장).
  `ReloginItem`에 `reason`(무엇이 막혔는지) 1필드 추가. 기존 5상태 전이 로직 불변.
- 알림: `Notify(kind, refType, refID, message)` + 미읽음 dedup. kind는 DB CHECK 제약이므로
  `challenge_parked`, `challenge_recovered` 추가 시 마이그레이션이 1회 필요하다(본 설계의 유일한 스키마 변경).
  UI 종(🔔)+`/api/notifications`는 수정 없이 수신한다.
- 스크린샷 전달: 알림에 바이너리 첨부 수단이 없으므로, 캡처 PNG는 미디어 업로드
  (`POST /api/media/uploads` 계열) 후 알림 message에 조회 경로를 싣는다. 신규 API 없음.
- 금고: `session.Service`가 머신로컬 키(`.vault.key`)로 AES-256-GCM 암복호화.
  `SetCredentials(id, loginID, password)` / `CredentialsFor(id)`.
  ADB 계정의 앱 로그인 자격도 동일 금고에 저장하고, 평문은 로그·API에 절대 노출 금지(현행 주석 규칙 유지).
- Playwright 이음매: `publish_handler(job_payload)`가 현행 발행 진입점,
  `run_loop(client, handlers)` + `make_real_handlers(client, account_id)`가
  타입→함수 디스패치 + 리스 연장 스레드 패턴이다. Go Runner는 이 계약을 그대로 본뜬다.

## ② Pluggable Runner 아키텍처 (Playwright + ADB 공존)

목표: 상위 큐·세션 매니저는 하나로 두고, 실행 방식만 갈아끼운다.
신규 Go 패키지 `internal/runner/` 1개만 추가하고, 기존 패키지는 import 방향이
`runner → store/session/notify/queue`가 되게 한다(역방향 의존 금지).

```text
                        ┌──────────────────────────────────┐
  somun UI (WS) ──▶     │ Ingress: 기존 queue.Handler.Next │
  /api/worker/*         │ (클레임·리스·결과수신, 수정 없음)  │
                        └───────────────┬──────────────────┘
                                        │ Job{Type, Payload}
                        ┌───────────────▼──────────────────┐
                        │ Dispatcher: runner.Router        │
                        │  Payload.runner: "playwright"    │
                        │              vs "adb:<instance>"  │
                        └──┬────────────────────────────┬──┘
              ┌────────────▼──────────┐   ┌─────────────▼─────────────┐
              │ PlaywrightRunner      │   │ MumuAdbRunner × N         │
              │ (기존 파이썬 워커를   │   │ InstanceQueue(전담 큐)     │
              │  HTTP로 그대로 호출)  │   │ CircuitBreaker(인스턴스별) │
              └────────────┬──────────┘   │ InputMutex(전역 1개 공유)  │
                           └──────────────┬─────────────┘
                              ┌────────────▼──────────────┐
                              │ Collector: 결과 집계      │
                              │ Submit*/Fail + Notify     │
                              │ + 세션 상태 전이          │
                              └───────────────────────────┘
```

핵심 인터페이스(신규 파일 `internal/runner/runner.go`):

```go
// AgentRunner는 1개 작업을 끝까지 실행한다. 호출자는 재시도 정책을 모른다.
type AgentRunner interface {
  // Run은 ctx 취소·데드라인을 준수해야 한다. 멱등이 아닐 수 있으므로
  // Router가 at-most-once로 호출하고, 재시도는 새 Job으로만 발행한다.
  Run(ctx context.Context, job JobRef) (Result, error)
  // Health는 H3 soak 하네스가 폴링한다. nil이면 정상.
  Health(ctx context.Context) error
  // Name은 로그·메트릭용 ("playwright" | "adb:pickle" ...).
  Name() string
}

type JobRef struct {
  ID        int64             // jobs.id (리스 연장·결과 보고 키)
  Type      string            // "publish" | "adb_post"
  AccountID int64             // channel_accounts.id
  Payload   json.RawMessage   // 지시문·미디어·타겟 식별자
}

type Result struct {
  Ok       bool
  Output   string            // 성공 시 증거(게시 URL 등)
  Verdict  Verdict            // done | retryable | challenge | fatal
  Evidence string            // 실패·챌린지 시 스크린샷 media 경로(없으면 "")
}

type Verdict string
const (
  VerdictDone      Verdict = "done"
  VerdictRetryable Verdict = "retryable" // transport 오류 등, 쿨다운 후 새 Job
  VerdictChallenge Verdict = "challenge" // 캡차·2FA·팝업 → 세션 challenge 전이
  VerdictFatal     Verdict = "fatal"     // 설정 오류 등, 인간 조치 없이 불가
)
```

인스턴스 스코프 구조체(신규 파일 `internal/runner/adb.go`):

```go
// AdbInstance는 MuMu 인스턴스 1기의 전담 실행 컨텍스트다(R1·R4 해결).
type AdbInstance struct {
  Name     string            // "pickle" | "vooy" | "keyring"
  MumuIdx  int               // MuMuManager -v 인덱스(표시용, 신뢰 금지)
  Serial   string            // 재확인된 adb serial (매 명령 전 검증)
  Queue    chan AdbCommand   // 전담 명령 큐(버퍼 16, race-free)
  Breaker  *CircuitBreaker   // 연속 실패 N회 → 쿨다운(기본 15분)
  client   *AdbClient        // MuMuManager+adb 래퍼(아래 ④ 하네스와 공용)
}

// InputMutex는 덤프 중 오입력을 막는 전역 락(R3 해결).
// screencap 판독·dumpsys 실행 중에는 tap/swipe 발행을 전면 금지한다.
var InputMutex sync.Mutex

// CircuitBreaker는 임계값 기반 단순 차단기다.
type CircuitBreaker struct {
  Failures    int
  Threshold   int           // 기본 3
  Cooldown    time.Duration // 기본 15m
  TrippedUntil time.Time
}
func (b *CircuitBreaker) Allow() bool
func (b *CircuitBreaker) Record(err error)
```

3단 분리 배치:

- Ingress(수신): 기존 `queue.Handler` 그대로. ADB 워커는 워커 토큰으로
  `GET /api/worker/jobs/next?type=adb_post` 롱폴 → `ClaimRoutedJob`이 원자 할당.
  변경점 없음. 단, Payload에 `runner:"adb:<instance>"` + `accountId` 필수(validate.go에
  `adb_post` 타입만 추가 — 기존 5종 검증식 뒤에 1조건 OR).
- Dispatcher(실행): `runner.Router{Playwright, Adb map[string]*AdbInstance}`.
  `JobRef`를 보고 Runner 선택 → `Run(ctx)` 호출. ctx 데드라인은 작업당 10분(PoC),
  내부는 명령별 5s/15s(아래 ③). 라우터는 Runner 내부를 모른다(인터페이스만 의존).
- Collector(집계): `Result.Verdict` 매핑 — done→`Submit*`+done, retryable→쿨다운 후
  새 Job 발행(원본은 failed가 아니라 done+note, 재시도 폭풍 방지),
  challenge→세션 `challenge` 전이 + `challenge_parked` 알림(스크린샷 경로 포함),
  fatal→`JobFailed` 알림 + 세션 `parked` 전이.

PlaywrightRunner는 Go에서 파이썬을 직접 호출하지 않는다. 기존 워커 HTTP 프로토콜을
그대로 두되, Go 측에 `PlaywrightRunner{BaseURL, Token}` 어댑터를 두어 동일
`AgentRunner`로 취급한다. 파이썬 코드 수정 0줄이 목표이며, 불가피하면
`adapter/worker_adapter.py`에 `runner` 에코 1줄만 허용한다.

## ③ 비침투 관측 파이프라인 상세 명세 (R2·R3 해결)

원칙: 게스트에 에이전트를 심지 않는다. 접근성 서비스 추가 금지, 앱 개조 금지.
관측은 호스트에서만 하고, 순서는 가벼운 것부터 무거운 것으로(1순위→3순위 폴백).

### L1 — logcat 태그·키워드 필터 (평시 유일 상시 관측)

- 호출 조건: 작업 시작 시 1회 구독 개시, 작업 종료 시 구독 해제. 폴링 아님(스트림).
- 명령: `adb -s <serial> logcat --pid=<agent_pid> -T 1 -s <Tag>:I` +
  키워드 매처(성공 토큰·에러 토큰·챌린지 토큰 3종, 앱별 사전 정의).
- 타임아웃: 토큰 대기 5s. 초과하면 L2로 1회 폴백(스트림은 유지).
- 성공 판정: 성공 토큰 수신 즉시 done(후속 관측 중단).
- 주의: pid는 작업 시작 시 `pidof <package>`로 고정. pid 변경(앱 재시작) 감지 시
  구독 재수립 + `retryable` 1회. logcat 버퍼는 `-G 1M`으로 축소.

### L2 — 온디맨드 screencap + 호스트 판정 (L1 침묵 시)

- 호출 조건: L1 5s 침묵, 또는 챌린지 의심 신호(에러 토큰·비정상 종료 코드).
- 명령: `adb -s <serial> exec-out screencap -p` 1장. 연속 촬영 금지(호출 간 최소 3s).
- 판정(호스트 CPU): 템플릿 매칭(성공 화면·캡차·2FA·팝업 닫기 버튼 순) → 미판정 시
  OCR 1회(관심영역 크롭 후). 판정 전체 데드라인 15s.
- 분기: 성공 화면→done, 닫기 가능 팝업→InputMutex 하에 tap 1회 후 L1 복귀,
  캡차·2FA→`challenge` 전이 + 스크린샷을 미디어 업로드 후 `challenge_parked` 알림,
  미판정 2회 연속→L3으로 1회 폴백.
- 타임아웃: 캡처+판정 합 15s. 초과하면 L3로 폴백(재촬영 금지).

### L3 — 뮤텍스 하 dumpsys·제한 덤프 (최후 수단)

- 호출 조건: L2 미판정 2회 연속, 또는 전경 앱 확인이 필요할 때.
- 명령(순서대로, 먼저 성공하면 중단):
  1. `adb -s <serial> shell dumpsys activity activities | grep mFocusedApp` (전경 패키지 확인)
  2. 필요시에만 `uiautomator dump --compressed /dev/stdout` 1회 (XML 파싱은 호스트)
- 규칙: 실행 전 `InputMutex` 획득, 해제 후 500ms 쿨다운(에이전트 제스처와 경합 방지).
  덤프 중에는 해당 인스턴스의 모든 tap·swipe 발행을 큐에서 보류한다.
- 타임아웃: 단계당 15s. 실패 시 `retryable`(transport 의심) 또는 증거 확보 후 `challenge`.
- 금지: L3 연속 호출(회당 최소 30s 간격), L3 결과에 기반한 좌표 탭(신선도 만료).

폴백 흐름도:

```text
작업 시작 ─▶ L1 구독 ──토큰──▶ done
                │ 5s 침묵
                ▼
               L2 캡처+판정 ──성공──▶ done
                │ ├─팝업→tap 1회→L1 복귀
                │ └─미판정 2회
                ▼
               L3 dumpsys/덤프 ──판정──▶ done|challenge
                │ 15s 초과·실패
                ▼
            retryable(쿨다운) — 재시도는 새 Job으로만
```

앱별 토큰 사전(PoC에서 실측 후 확정, 파일: 신규 `internal/runner/tokens_<app>.go`):

- 피클: 패키지 `com.pickle.client`, 성공·에러·챌린지 토큰 미확정(PoC 2에서 채집).
- Vooy: 패키지명 미확정(PoC 1에서 `pm list packages`로 확정).
- 키링: 패키지 `com.ilevit.pmf.pepper`, 토큰 미확정(PoC 2에서 채집).
- 공통 챌린지 힌트: "captcha", "verify", "보안", "차단", "다시 시도" (대소문자 무시 부분일치).

### MuMu12 24시간 운용 규약 (헬스체크 + 롤링 리스타트)

- 렌더 유지: 백그라운드 렌더 유지 옵션 on, ECO 모드 off, 창 최소화 상태에서도
  프레임 갱신 확인(정확한 설정명은 버전별 상이 → PoC 1에서 확정).
- Doze·절전: 에이전트 3종 + MuMu 본체를 배터리 최적화 화이트리스트에 등록.
  WiFi 슬립 방지, 시계 NTP 동기(세션·OTP 어긋남 방지).
- 3중 헬스체크(60s 주기, 인스턴스별): ①ADB 살아있음(`get-state` 응답),
  ②에이전트 앱 전경(`dumpsys` 패키지 일치), ③최근 하트비트 신선도(5분 이내).
  1개라도 실패 → 자동 복구(재접속 → 인스턴스 재시작 순) → 복구율 집계.
- 롤링 리스타트: 24시간마다 또는 메모리 임계 초과 시. 조건은 유휴 + 큐 비었을 때만,
  1기씩 순차(전체 동시 재시작 금지). 절차: drain(신규 배정 중지) → 진행 작업 완료
  대기(최대 10분, 초과 시 안전 중단 후 재시작) → 스냅샷 → 재시작 → 헬스 3종 통과
  확인 → 큐 복귀. 재시작 중 해당 인스턴스 작업은 타 인스턴스로 우회하지 않는다
  (앱-인스턴스 1:1 고정 원칙, 대기열 적체로 처리).
- 디스크: 주기 compact + 스냅샷 세대 정리. `MuMuManager` 출력 파싱 실패율 별도 집계.

## ④ PoC 1~3 검증용 CLI 하네스 명세

백엔드 통합 전에 단독 실행하는 검증 도구. 신규 경로 `tools/adbprobe/`(Go, 기존
`backend/` 모듈 import) 또는 파이썬 단일 파일 중 택일 — 권장안은 Go(아래 ②의
`AdbClient`를 그대로 공용하므로 이중 구현 방지). 운영 큐·DB를 건드리지 않는다.

### H1 — `adbprobe resolve` (PoC 1: 포트 동적 탐색 + 생존성 게이트)

- 동작: `MuMuManager.exe` 인스턴스 목록 조회 → 각 인스턴스 기동 확인 →
  `adb connect` → `shell getprop ro.build.version.sdk` + `pm list packages`로
  3종 앱 설치·패키지명 확정(Vooy 패키지명 여기서 채집).
- 추가 게이트: Play Integrity·에뮬 차단 여부 1차 판별(대상 앱 실행 후 크래시·경고
  문구 템플릿 매칭). 실패 시 전체 PoC 중단 판정 출력 후 exit 3.
- 통과 기준: 3/3 인스턴스 `serial` 확정 + SDK≥31 + 패키지 3종 확인. 출력은 JSON.

### H2 — `adbprobe post --instance <name> --dry-run|--live` (PoC 2: 1계정 1포스트 왕복)

- 동작: L1 구독 개시 → 지시 전달(클립보드 경유 텍스트 주입이 1순위, 키 이벤트가
  후순위) → 성공 토큰 대기(5s) → L2/L3 폴백 체인(위 ③과 동일 상수) → 결과 리포트.
- `--dry-run`: 입력 없이 관측 체인만 검증(토큰 사전 채집 모드).
- `--live`: 테스트 계정 1개로 하단 초안 1건 발행(운영 계정 사용 금지).
- 통과 기준: 지시→성공 토큰→리포트 왕복 3회 연속 성공 + 앱별 토큰 사전 초안 출력.

### H3 — `adbprobe soak --hours 72` (PoC 3: 72시간 유지 + 헬스체크)

- 동작: 60s 주기로 인스턴스당 헬스 3종(ADB 살아있음·에이전트 전경·하트비트 신선도)
  기록. 이상 시 자동 복구(재접속→인스턴스 재시작 순) 후 복구 성공률 집계.
- 메모리·핸들 추이 로그, MuMuManager 출력 파싱 실패율 집계.
- 통과 기준: 72시간 무인 완주 + 복구율 100% + 메모리 증가율 1차 회귀 기울기 기준 이하.

### 실행 시나리오 (순서 고정)

1. `adbprobe resolve` → Go/No-Go 판정(실패 시 중단, LD 회귀 금지).
2. `adbprobe post --dry-run` × 앱 3종 → 토큰 사전 채집.
3. `adbprobe post --live` → 3회 연속 왕복.
4. `adbprobe soak --hours 72` → 백그라운드 실행 후 리포트.
5. 전 단계 로그를 `tools/adbprobe/report/`에 보관(판정 근거).

## 5. 변경 금지 구역 + 승인 필요 지점

변경 금지(본 설계에서 손대지 않음):

- `adapter/*.py`, `worker/*.py` 전체 (Playwright 경로).
- `internal/store/*` 기존 함수·스키마 전부.
- `internal/queue/*`, `internal/session/*`, `internal/notify/*` 기존 함수 전부.
  단, 아래 승인 항목 4건(신규 파일·조건 1개·마이그레이션 1회·상수/필드)은 예외.

승인 필요(구현 착수 시 의장 승인):

- 신규 패키지 `internal/runner/` + 신규 CLI `tools/adbprobe/` (순수 추가).
- `queue/validate.go`에 `adb_post` 1조건 추가.
- `notify` kind 2종 추가용 마이그레이션 1회(CHECK 제약).
- 세션 상태 상수 2종(`challenge`, `parked`) + `ReloginItem.reason` 1필드.

## 6. 미확정 사항 (PoC에서 확정)

- Vooy 패키지명, 앱별 logcat 태그·토큰 사전, MuMu ADB 포트 규칙 실측값.
- MuMu 백그라운드 렌더 유지 옵션의 정확한 설정명(버전별 상이).
- 일일 안전 동작량/계정(가정 IP 실측 전까지 가정치 사용 금지).

## 7. PoC 1~7 검증 체크리스트 및 Go/No-Go 판정 기준

순서 고정. 앞 단계 No-Go 시 뒷 단계 진행 금지. PoC 1 No-Go는 전체 설계 폐기.

| # | 검증 내용 | 통과(Go) | 폐기(No-Go) |
|---|---|---|---|
| 1 | MuMu12 + 앱 3종 + 타겟 SNS 차단 여부 | 3종 실행·민감동작 가능 | 1종이라도 차단 |
| 2 | 1계정 1포스트 왕복 | 3회 연속 성공 + 토큰 사전 | 원인 불명 실패 1회 |
| 3 | ADB 72시간 유지·헬스 | 무인 완주 + 복구율 100% | 복구 실패 1건 |
| 4 | 일일 안전 동작량/계정 | 수치 확정 + 재현 | 일별 2배 이상 출렁임 |
| 5 | 챌린지→복귀 루프 | park→복귀 3회 연속 | 인간 없이 막힘 발생 |
| 6 | PC 1대 안정 상한 | 3기 안정 + 여유 30% | 3기 불가 |
| 7 | 세션 수명·재인증 주기 | 수명·주기 확정 | 24h 미만 만료 반복 |

체크리스트:

- [ ] PoC 1: `adbprobe resolve` JSON (serial 3/3, SDK≥31, 패키지 3종, 차단 판정).
- [ ] PoC 2: dry-run 토큰 사전 → live 3회 연속 리포트.
- [ ] PoC 3: 72시간 로그 + 복구율 + 메모리 회귀 기울기.
- [ ] PoC 4: 계정당 상한 수치 + 3일 재현 기록.
- [ ] PoC 5: park→인간→복귀 3회 기록 + 오분류율.
- [ ] PoC 6: 인스턴스 수별 CPU·RAM·성공률 표.
- [ ] PoC 7: 세션 수명 분포 + 재인증 주기 확정값.
- [ ] 전 단계 로그를 `tools/adbprobe/report/`에 보관.
