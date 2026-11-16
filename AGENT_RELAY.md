# SOMUN 에이전트 릴레이 — 계정별 역할 분담 설계 (D37)

- 상태: **폐기**(의장 결정 2026-11-14). 고정 체인(Meta→GPT→Claude→Muse) 대신 [작업지시서](WORK_ORDER.md) W16의 자유 팀 편성으로 간다. 이 문서는 참고용으로만 둔다.
- 현행 계정 분리 규칙(W6·W16, 2026-11-16 W10 개정): 생성한 계정과 검수하는 계정은
  달라야 하며, 팀 안 배정 규칙이 이를 강제한다. 한 계정은 한 번에 한 팀에만 속하고,
  팀원이 1명뿐인 팀은 "검수를 위해 1명 더 필요" 상태로 멈추고 알림한다.
  팀 실행기가 서버 배정대로 "작업 + 배정 에이전트"를 실행하며, 고정 레인·D레인·
  실행안(plan) 단계·`SOMUN_PLAN_STAGE` 스위치는 도입하지 않는다.
- 셀렉터 확보 순서(현행): ① Meta AI 채팅 확정(개통됨) ② 네이버·티스토리 세션 포스팅
  (W5-1, 테스트 계정 개통 후) ③ ChatGPT·Claude·Gemini 중 2개 이상 채팅 확보
  (W6-2, 생성≠검수 보장) ④ AI 이미지·영상 생성 hook (W14-2·3, 플랫폼별 실세션 확인)
  ⑤ 성과 수집 셀렉터 (W7, 실게시 후). 아래 §7 표는 확보 전 초안으로만 본다.
- 지시 원문 요지: Meta AI가 콘텐츠 전략을 세우면, GPT가 그 전략으로 실행 방안을 정하고,
  Claude가 콘텐츠를 생성하고, Muse AI가 발행한다.
- 문서 관계: [설계서](DESIGN.md) §4 파이프라인·§12-3 Resident Adapter의 확장이다.
  관련 결정 D9·D18·D19·D21·D24·D25·D27. 컬럼·API 상세는 확정 후 [부록](APPENDIX.md)에 옮긴다.
- 표기: (현행) = 2026-11-13 작업 트리 기준 코드·DB에서 확인한 사실. (신설)·(변경) = 본 설계.

## 1. 핵심 원칙

1. **에이전트끼리 직접 대화하지 않는다.** 서버가 바통을 넘긴다. 앞 단계 결과가 DB에 저장되면
   서버가 다음 단계 작업을 자동으로 만들고, 다음 레인 에이전트가 그 작업을 가져간다(현행 작업 큐 그대로).
   - 이유: 단계 결과가 구조화된 기록으로 남는다(감사·재현). 리스·재대기·반려 상한(D21·D25)을
     그대로 재사용한다. 한 AI가 멈춰도 앞 단계 결과는 남아 재개 시 이어진다.
2. **에이전트 1기 = 계정 1개 = 레인 1개.** 어느 AI가 어느 단계를 맡는지는
   에이전트↔계정 연결(`agents.account_id`)로만 정한다. 계정 교체는 연결만 바꾸면 된다.
3. **생성한 계정과 검수하는 계정은 달라야 한다.** 자기 글을 자기가 검수하면 C 게이트(D19)가 무의미하다.
4. **실제 업로드는 AI 채팅이 아니라 발행 프로그램이 한다.** AI는 판단하고, 글을 올리는 일은
   어댑터 P레인(직접군: 네이버·티스토리) 또는 서버 실행기(API군)가 한다.

범위 밖: 에이전트 다수가 한 방에서 자유롭게 주고받는 그룹방(자유 대화)은 본 설계에 넣지 않는다.
필요하면 별도 설계로 다룬다(§12).

## 2. 현행과의 차이 (코드 대조 2026-11-13)

| # | 현행 | 근거 | 본 설계 |
|---|---|---|---|
| 1 | A·B·C 레인 3기가 전부 같은 계정(3번 Meta AI)으로 채팅한다 | `scripts/workers/resident-{a,b,c}.cmd`의 `SOMUN_CHAT_ACCOUNT_ID=3` | 레인마다 다른 계정(§4) |
| 2 | 에이전트의 `account_id`가 전부 비어 있고, 워커는 이 값을 읽지 않는다 | DB `agents`, [worker.py](worker/worker.py) `main()` | 서버가 준 연결 계정으로만 채팅(§5) |
| 3 | 워커 토큰만 있으면 어느 계정으로든 채팅할 수 있다 | [chat/handler.go](backend/internal/chat/handler.go) `Send` | 자기 연결 계정만 허용(§5) |
| 4 | 같은 계정에 동시에 여러 채팅이 돌 수 있다(동시 브라우저 상한이 프로세스 안에서만 작동) | [browser.py](adapter/browser.py) 세마포어, [chat/service.go](backend/internal/chat/service.go) | 계정 단위 직렬화 + 서버 전역 상한(§6) |
| 5 | 실제 답장이 오는 AI는 Meta뿐. GPT·Claude·Gemini 채팅 셀렉터는 TBD | [platforms.py](adapter/platforms.py), DB `chat_messages`(계정 4·5·6 답장 0건) | 셀렉터 확정 + 답장 판정 플랫폼별 분리(§7) |
| 6 | Muse AI가 플랫폼 목록·계정에 없다 | [platforms.py](adapter/platforms.py), DB `channel_accounts` | 플랫폼 등록(§7, U-1) |
| 7 | 전략 다음 단계가 바로 생성이다. 실행 방안 단계가 없다 | [queue/service.go](backend/internal/queue/service.go) `SubmitStrategy` | 실행안(plan) 단계 신설(§3) |

## 3. 최종 구조

### 3-1. 역할표

| 순서 | 작업 type | 레인 | 담당 계정(기본안) | 하는 일 |
|---|---|---|---|---|
| 1 | analyze | A | Meta AI | 상품 판독(사실 정리). 현행 유지 |
| 2 | strategy | A | Meta AI | 전략: 앵글·채널·핵심 메시지·일정 메모. 현행 유지 |
| 3 | **plan** (신설) | **D** (신설) | GPT | 실행안: 채널별로 형식·훅·전개·CTA·톤을 정한다 |
| 4 | generate | B | Claude | 실행안대로 채널별 초안 작성 |
| 5 | review | C | Muse AI | 검수 + 발행 결정(승인·대상 계정·플랫폼·예약 시각) |
| 6 | publish | P | (AI 아님) 어댑터 | 직접군 실제 업로드. API군은 서버 실행기 |

- 지시의 "Muse AI가 발행한다"는 **Muse가 발행을 결정하고, 발행 프로그램이 실제로 올린다**로 해석한다.
  설계서 §4의 "C(muse ai 검수·배포)"와 같은 해석이다. 근거는 §3-3(Muse 확인 결과).
- 레인 문자 D = Direction(실행안). 레인은 벤더 이름이 아니라 역할로 부른다(계정은 바뀔 수 있다).

### 3-2. 흐름

```
상품 투입
  │
  ▼ analyze ─[A: Meta]──▶ readings 저장 ──서버가 strategy 작업 생성
  ▼ strategy ─[A: Meta]─▶ strategies 저장 ─서버가 plan 작업 생성        (변경: 기존엔 generate)
  ▼ plan ─────[D: GPT]──▶ plans 저장 ─────서버가 generate 작업 생성     (신설)
  ▼ generate ─[B: Claude]▶ contents(draft) ─서버가 review 작업 생성
  ▼ review ───[C: Muse]─▶ 승인+대상 → queued·예약 ─▶ publish(P: 어댑터) / 실행기(API군)
                    │
                    └ 반려 → 서버가 generate 재작업 생성(피드백 첨부, 사슬당 2회, D21)
                             3회째 → 사람에게 알림(review_human)
```

- 실행안 단계 스위치: `SOMUN_PLAN_STAGE`(기본 `0`). `0`이면 현행대로 strategy에서 바로 generate로 간다.
  GPT 셀렉터가 확정되기 전에도 나머지 체인이 돌게 하기 위함이다. `1`이면 plan을 거친다.
- 위 흐름도의 "3회째 사람에게 알림"은 폐기 전 초안이다. 현행(W6)은 3회째 사슬을
  `archived`로 자동 폐기하고 사람에게 넘기지 않는다(머리말 현행 규칙 참조).
  스위치는 **작업 생성 시점**에만 읽는다. 이미 만들어진 작업은 바뀌지 않는다.

### 3-3. Muse 확인 결과 (https://muse.ai/ 공개 페이지, 2026-11-13 열람)

- 정체: Meta가 만든 개인 AI 에이전트("Your Personal AI Agent from Meta"). 웹·모바일·데스크톱 앱과
  WhatsApp에서 채팅으로 쓴다. 질문에 답하는 데 그치지 않고 클라우드 환경에서 일을 대신 수행하는 에이전트형이다.
  → **채팅형이므로 C레인(검수·발행 결정)에 넣을 수 있다.** 다른 AI처럼 어댑터 채팅 경로를 쓴다.
- 외부 발행: 이메일·캘린더·Instagram·Facebook·Meta 광고와 업무 도구(Canva·Notion·Shopify·Slack 등)를
  연결할 수 있다. 다만 "승인 없이는 아무것도 게시·발송·결제하지 않는다"고 명시돼 있다.
  네이버 블로그·티스토리 연결은 언급이 없다.
  → **무인 자동 발행 주체로 쓸 수 없다.** 건마다 사람 승인이 끼고, 직접군(네이버·티스토리)은 지원하지 않는다.
  따라서 실제 업로드는 계획대로 P레인 어댑터·서버 실행기가 맡는다(§1 원칙 4 유지).
  Instagram·Facebook을 Muse 연결 기능으로 올리는 경로는 "사람 승인 1클릭" 수동 모드 후보로만 남긴다(U-8).
- 공개 API: 공개 페이지에 없음(`/api`·`/docs`는 비로그인 401). 연동은 화면 자동화(어댑터)뿐이다.
- 요금: 대부분 무료, 상위 구독제 있음. 한도 수치는 공개 페이지에 없다(U-6).
- **이용 지역: 미국·캐나다, 만 18세 이상만 이용 가능하다고 적혀 있다.** 국내에서 가입·로그인이
  막히면 C레인 배정안 자체가 성립하지 않는다(U-7, 최우선 확인).
- 같은 Meta 계열이지만 Meta AI(A레인)와 Muse(C레인)는 별개 서비스·별개 세션으로 등록한다.
  생성(B, Claude)과 검수(C, Muse)가 다른 회사 AI이므로 계정 배타 원칙(§1-3)도 충족한다.
- 한계: 위 내용은 공개 마케팅 페이지 기준이다. 로그인 후 화면 구조·실제 한도·지역 차단 여부는
  R-3에서 실세션으로 확인한다.

## 4. 단계별 입출력 계약

모든 AI 응답은 JSON 1개다(현행 `_JSON_RULE` 유지). 파싱·검증 실패 = 작업 실패(사람 확인). 추측 금지.

### 4-1. strategy (현행 유지)

- 결과: `{angle, channels, key_messages, schedule_note}`. angle 필수.

### 4-2. plan (신설)

- 작업 페이로드(서버 생성): `product_id, product_name, detail_core, review_summary,
  angle, channels, key_messages, schedule_note`.
- 결과:

```json
{
  "items": [
    {"channel": "instagram", "format": "캐러셀 5장", "hook": "첫 장 문구",
     "structure": "문제→해결→근거→혜택→CTA", "cta": "프로필 링크 클릭", "tone": "친근한 반말"}
  ],
  "notes": "전 채널 공통 주의사항"
}
```

- 검증(서버 `ValidatePlanResult`):
  - items 1개 이상. 모든 값은 문자열.
  - channel은 허용 4종(instagram·shortform·cardnews·blog) 안이어야 한다.
    strategy.channels가 비어 있지 않으면 그 목록 안이어야 한다. 밖이면 버린다.
  - hook 필수(빈 문자열 금지). 같은 채널이 여러 번 나오면 첫 항목만 남긴다.
  - 남은 항목이 0개면 400.
- 프롬프트 초안(`worker/ai_tasks.py`):
  `"실행안 수립. 아래 전략을 읽고 채널별 실행 방안을 JSON으로 답한다. <JSON 규칙>
  형식: {"items":[{"channel","format","hook","structure","cta","tone"}],"notes"}.
  채널은 <전략 채널 또는 허용 4종> 중에서만. 전략에 없는 사실을 만들지 않는다.
  product_name·detail_core·review_summary·angle·key_messages·schedule_note: ..."`

### 4-3. generate (변경)

- 페이로드에 `plan`(items 배열·notes)을 추가한다. 재생성 페이로드에도 추가한다.
  현행 재생성 페이로드는 `SubmitReview`가 DB에서 전략을 읽어 만들므로 같은 자리에서 실행안도 읽는다.
- 프롬프트: 채널별 실행안을 그대로 넣고 "실행안의 형식·훅·전개·CTA·톤을 따른다"를 추가한다.
- 검증: plan이 있으면 실행안에 없는 채널의 초안은 불가 채널로 취급해 버린다(현행 "전부 불가 채널 400" 규칙과 같다).

### 4-4. review (변경)

- 페이로드에 추가: `plan`(실행안 준수 검사용), `targets`(발행 가능한 대상 목록:
  연결된 발행용 계정의 `name·platform·minIntervalMin`).
- 결과는 현행 `ReviewDecisionItem` 그대로: `content_id, approve, reason(필수), account, platform, scheduledAt`.
  account·platform은 targets 안에서만 고를 수 있다. 밖이면 400(결정 전체 거부, 부분 반영 없음).
- 승인 기준에 "실행안 준수"를 추가한다. 반려 사유는 그대로 재생성 피드백이 된다.
- 프롬프트가 지금은 대상 필드를 요구하지 않는다. targets가 비어 있으면 승인만 하고 대상은 비워 둔다
  (현행대로 approved 상태에서 사람이 예약).
- 위 "AI가 발행 대상을 고른다"는 폐기 전 초안이다. 현행(W4)은 AI 판정에 계정을 싣지 않고
  승인 즉시 서버 배정기가 계정·예약을 정한다.

## 5. 계정 배정과 권한

- 진실의 원천은 `agents.account_id`다. 환경변수 `SOMUN_CHAT_ACCOUNT_ID`는 공유 토큰(dev) 폴백으로만 남긴다.
- (변경) `POST /api/worker/heartbeat` 응답에 `accountId`를 추가한다. 워커는 기동 시와 매 하트비트마다
  이 값을 읽어 채팅 계정으로 쓴다. 비어 있으면 작업을 가져가지 않고 "계정 미연결"로 대기한다.
- (변경) `POST /api/worker/chat/send`: 에이전트 토큰 호출이면 요청 accountId가 그 에이전트의
  account_id와 같아야 한다. 다르면 403. 공유 토큰(dev)은 현행 유지.
- (신설) 계정 배타: 한 계정은 에이전트 1기에만 연결된다(`agents.account_id` 부분 UNIQUE, NULL 제외).
  위반 시 등록·수정 409. 결과적으로 B(생성)와 C(검수)는 반드시 다른 계정이 된다.
  - 규모 확장은 계정을 늘려서 한다. 계정 1개로는 동시 대화 1개가 실질 한계이기 때문이다(§6).
- (변경) `scripts/install-workers.ps1`: `-ChatAccountId` 한 값 대신 레인별 매핑
  `-LaneAccounts "A=3,D=5,B=4,C=<muse>"`을 받아 에이전트 등록 시 accountId로 넘긴다.
  러너 파일에는 계정 ID를 쓰지 않는다.
- 대체 계정: 같은 레인에 다른 계정으로 에이전트를 1기 더 등록하면 레인 라우팅(D24)이
  자동으로 나눠 가진다. 한 계정의 세션이 만료돼도 그 레인은 계속 돈다. 단, C레인 예비 계정이
  B레인 계정과 같으면 안 된다(계정 배타가 막는다).

## 6. 동시성

- (신설) 계정 단위 직렬화: `chat.Service`에 계정 ID별 잠금을 둔다. 같은 계정의 채팅은
  한 번에 1건만 어댑터를 띄우고, 나머지는 줄을 선다. 콘솔 대화와 워커 채팅이 같은 계정에서
  부딪히는 경우도 같은 잠금으로 막는다. 세션 파일 동시 저장 경합도 함께 사라진다.
- (신설) 서버 전역 브라우저 상한: 서버가 띄우는 채팅 프로세스 수를 `ADAPTER_MAX_ACTIVE`로 제한한다
  (서버 쪽 세마포어). 현행 세마포어는 어댑터 프로세스 안에서만 작동해 채팅에는 효과가 없다.
- 대기 시간이 리스를 넘지 않도록 워커는 대기 중에도 작업 하트비트를 보낸다(현행 `_run_with_lease` 유지).

## 7. 플랫폼 준비 (셀렉터)

| 플랫폼 | 용도 | 현행 | 필요 작업 |
|---|---|---|---|
| meta_ai | A 전략 | 채팅 확정 | 없음 |
| chatgpt | D 실행안 | 로그인됨·채팅 TBD | 채팅 셀렉터 4종 확정 |
| claude_app | B 생성 | 로그인됨·채팅 TBD | 채팅 셀렉터 4종 확정 |
| muse (신설) | C 검수·발행 결정 | 없음 | 플랫폼 등록(`https://muse.ai/`)·국내 로그인 가능 확인(U-7)·채팅 셀렉터 |
| gemini | 예비 | 로그인됨·채팅 TBD | 선택(U-5) |
| naver_blog·tistory | P 발행 | 발행 셀렉터 TBD | 기존 P0-1·P1-2 범위 |

- 채팅 셀렉터 4종: `chat_path·composer_selector·send_selector·reply_selector`.
  실세션 화면에서만 확정할 수 있다. 의장이 로그인한 상태에서 총괄이 화면 구조를 확인한다.
- (변경) 답장 완료 판정의 플랫폼별 분리: 현행 [chat.py](adapter/chat.py)의 본문 추출(`.markdown-content`),
  완료 표시(`assistant-message-actions`), 생성 중 표시(`aria-label=Stop`)는 Meta 화면에 맞춰져 있다.
  `Platform`에 `reply_body_selector·done_selector·busy_selector`를 추가하고 플랫폼마다 값을 둔다.
  비어 있으면 현행 동작(안정 판정 라운드)으로 떨어진다.

## 8. 데이터 모델 (스키마 v11)

- (신설) `plans`: `id, product_id UNIQUE FK(products) ON DELETE CASCADE, items TEXT(JSON),
  notes TEXT, created_at, updated_at`. 상품당 1행 upsert(strategies와 같은 방식).
- (변경) `jobs.type` CHECK에 `plan` 추가, `agents.lane` CHECK에 `D` 추가.
  SQLite는 CHECK를 바꿀 수 없으므로 v7과 같은 테이블 재구축 방식으로 올린다.
  올리기 전 자동 백업 1세대(D32).
- (변경) 코드 정책: `LaneTypes["D"] = {"plan"}`. `ValidLane`·화면 레인 선택지·Bulk relane 검증에 D 추가.
- (변경) `agents.account_id` 부분 UNIQUE 인덱스(`WHERE account_id IS NOT NULL`).
  기존 데이터에 중복 연결이 있으면 마이그레이션이 실패하지 말고 중복분을 NULL로 풀고 알림 1건을 남긴다.

## 9. 실패·예외

- 단계 실패: 현행 규칙 유지. failed 기록, 자동 재등록 없음, 사람이 콘솔에서 재등록.
  무응답 리스 만료만 1회 자동 재대기(D25).
- 세션 만료: 채팅이 "저장된 세션 없음"으로 끝나면 계정이 expired로 바뀌고(현행) 재로그인 알림이 뜬다.
  그 레인만 멈추고, 앞 단계 결과는 DB에 남아 재로그인 후 이어진다.
- 실행안 단계에는 반려·재생성 루프가 없다. 실패하면 사람에게 간다.
- 스위치 전환: `SOMUN_PLAN_STAGE`를 1에서 0으로 내려도, 이미 생성된 plan 작업은 그대로 처리된다.
  plan이 없는 generate 작업은 현행 프롬프트로 돈다.

## 10. 구현 티켓

순서: R-1·R-2·R-3 병행 → R-4 → R-5 → R-6 → R-7. 실발행은 P0-1·P1-2(네이버·티스토리 셀렉터)에 의존한다.

### R-1. 레인별 계정 분리·권한 (§5)
- 내용: 하트비트 응답 accountId, 워커가 연결 계정 사용, 워커 채팅 계정 검사(403),
  계정 배타 UNIQUE, install-workers.ps1 레인별 매핑.
- 변경 위치: `internal/agent`(handler·service), `internal/chat/handler.go`, `internal/store`(agents·migrate),
  `worker/worker.py`·`config.py`, `scripts/install-workers.ps1`.
- 인수: A·B·C 에이전트가 각자 다른 계정으로 채팅한다(chat_messages의 account_id가 레인별로 갈린다).
  다른 에이전트 계정으로 보내면 403. 한 계정을 두 에이전트에 연결하면 409.

### R-2. 계정 직렬화·전역 상한 (§6)
- 변경 위치: `internal/chat/service.go`.
- 인수: 같은 계정에 동시 3건 → 어댑터 실행이 겹치지 않는다(단위 테스트, sendRunner 대역).
  상한 2에서 서로 다른 계정 3건 → 동시 실행 최대 2.

### R-3. 채팅 셀렉터 확정 (§7, 사람 작업 포함)
- 내용: chatgpt·claude_app 셀렉터 확정, 답장 판정 필드 플랫폼별 분리, Muse 플랫폼 등록.
  Muse는 국내 로그인 가능 여부(U-7)를 먼저 확인하고, 막히면 C레인 대체안을 정한 뒤 진행한다.
- 변경 위치: `adapter/platforms.py`, `adapter/chat.py`, `adapter/test_adapter.py`.
- 인수: 각 계정으로 `python -m adapter.chat <계정> <플랫폼> "1+1은?"`이 종료 0과 답장 JSON을 낸다.
  콘솔 대화 화면에서 계정별 답장이 기록된다.

### R-4. 실행안 단계 신설 (§3·§4-2·§8)
- 내용: 스키마 v11(plans·CHECK 재구축), plan 작업 type·레인 D, `SubmitPlan`·`ValidatePlanResult`,
  strategy→plan→generate 연결(스위치), 결과 등록 라우트, 워커 프롬프트·파서.
- 변경 위치: `internal/store`(migrate·store), `internal/queue`(model·service·validate·handler),
  `internal/config`, `router`, `worker/ai_tasks.py`·`worker.py`·`test_worker.py`.
- 인수: 스위치 1에서 strategy 결과 등록 → plan 작업 1건 생성. plan 결과 등록 → plans 저장 + generate 1건.
  전략 채널 밖 항목은 버려진다. hook 빈 항목만 있으면 400. 스위치 0에서는 현행과 같다(회귀).
  v10 DB가 v11로 올라가며 기존 jobs·agents 행이 보존된다.

### R-5. 실행안·발행 대상 전달 (§4-3·§4-4)
- 내용: generate·재생성·review 페이로드에 plan 추가, review 페이로드에 targets 추가,
  실행안 밖 채널 초안 거부, 결정 대상 검사.
- 인수: 실행안이 instagram·blog뿐이면 shortform 초안은 저장되지 않는다. 반려 재생성 페이로드에 plan이 있다.
  targets 밖 계정으로 승인하면 400. targets 안 계정이면 queued·예약까지 전진한다.

### R-6. 화면
- 내용: 에이전트 연결 화면에 레인 D·계정 연결 필수 표시, 상품 상세에 실행안 섹션(전략 아래),
  대화 화면 오피스 스트립에 계정별 담당 단계 표시.
- 변경 위치: `web/src/views/ConnectViews.vue`·`ProductReadView.vue`·`ChatView.vue`, `web/src/stores/labels.js`.

### R-7. 종단 인수
- 실제 상품 1종이 Meta(전략) → GPT(실행안) → Claude(생성) → Muse(검수·발행 결정) → 발행 큐까지
  사람 개입 없이 흐른다. 각 단계 작업의 worker_name이 서로 다른 에이전트이고,
  각 에이전트의 연결 계정이 서로 다르다. review_decisions의 판정자가 Muse 에이전트다.
- 반려 1회 재현: Muse 반려 사유가 Claude 재생성 페이로드의 feedback으로 들어간다.
- 실발행까지는 P0-1·P1-2 완료 후 이어서 확인한다.

## 11. 미확인·결정 필요

| ID | 항목 | 기본안 | 결정자 |
|---|---|---|---|
| U-1 | ~~Muse AI의 정체~~ **해소(2026-11-13):** Meta의 채팅형 AI 에이전트, 발행은 건별 사람 승인, 직접군 미지원, 공개 API 없음(§3-3). C레인 배정, 실제 업로드는 P레인 유지 | — | — |
| U-2 | 판독(analyze)도 Meta가 맡는가 | 맡는다(레인 A 유지) | 의장 |
| U-3 | 실행안 항목(format·hook·structure·cta·tone) 확정 | 본 문서 §4-2안 | 총괄 |
| U-4 | 레인 문자 D 명명 | D(Direction) | 총괄 |
| U-5 | Gemini의 역할 | 체인 밖 예비 계정(대체 투입용) | 의장 |
| U-6 | 각 AI 서비스의 일일 사용 한도·자동화 차단 수준 | 미확인. R-7 중 관측해 기록 | 총괄 |
| U-7 | **Muse 국내 이용 가능 여부**(공개 페이지상 미국·캐나다 만 18세 이상 한정) | 의장이 실화면 로그인으로 확인. 막히면 C레인을 GPT 또는 Gemini로 대체(B=Claude와 다른 계정이면 됨) | 의장 |
| U-8 | Instagram·Facebook을 Muse 연결 기능(건별 승인)으로 올리는 수동 경로를 둘지 | 두지 않는다. 발행은 P레인·실행기로 일원화(기록·증거 규칙 D7 유지) | 의장 |

## 12. 그룹방(자유 대화)과의 관계

- 본 설계는 정해진 순서로 바통을 넘기는 릴레이다. 에이전트가 서로의 발언을 보고 즉석에서 받아치는
  그룹방은 아니다.
- 그룹방이 필요하면 별도 설계로 다룬다: `rooms·room_members·room_messages` 테이블,
  발언 순서·종료 조건을 정하는 서버 측 진행자, 대화창 재사용(기억 유지), 화면 실시간 푸시.
  본 설계의 R-1(계정 분리)·R-2(계정 직렬화)·R-3(셀렉터)은 그룹방에도 그대로 필요한 선행 작업이다.

## 13. 변경 이력

- 2026-11-13: 초안 작성(의장 지시). 코드 대조로 현행 차이 7건 확인, 실행안 단계·레인 D·계정 배타·
  계정 직렬화 설계, 티켓 R-1~R-7, 미확인 U-1~U-6.
- 2026-11-13: Muse 공개 페이지 확인(§3-3). U-1 해소, U-7(지역 제한)·U-8(Muse 연결 발행 경로) 추가.
- 2026-11-16: 작업지시서 §8 개정(W10). 머리말에 현행 W6·W16 계정 분리 규칙과
  셀렉터 확보 순서 명시. 본문은 폐기 전 초안 그대로 둔다.
