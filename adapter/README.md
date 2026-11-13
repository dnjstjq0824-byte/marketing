# Adapter (P-lane session posting, D9/D27/D36)

`worker/`는 표준라이브러리만 쓴다. 브라우저 구동이 필요한 세션 포스팅은
이 패키지가 맡으며, 유일한 서드파티 예외(Playwright)도 여기만 해당한다.

## 설치 (최초 1회, 프로젝트 루트에서)

```powershell
python -m venv adapter\.venv
adapter\.venv\Scripts\python -m pip install -r adapter\requirements.txt
adapter\.venv\Scripts\python -m playwright install chromium
```

브라우저는 기본 캐시(`~/.cache/ms-playwright`)에 깔린다.
다른 경로를 쓰려면 `PLAYWRIGHT_BROWSERS_PATH`를 지정한다.

## 파일럿 로그인 (실계정 1회, 의장 직접)

순서: AI 서비스 먼저(Meta AI → Claude), 채널은 맨 나중(네이버·티스토리).
방법 1(권장): 콘솔 연결 화면의 계정별 로그인 버튼. 브라우저가 뜨면 직접
로그인하고 완료 확인을 누른다.
방법 2: 이 PC의 PowerShell(프로젝트 루트)에서. 브라우저 창이 뜨면 직접
로그인하고 터미널에서 Enter를 누른다. 자격을 누구에게도 보내지 않는다.
서버가 띄운 브라우저가 화면에 안 보이면 이 방법 2를 쓴다(자기 셸에서
띄운 브라우저는 반드시 보인다). 저장 후 콘솔 로그인 모달의
"터미널 로그인 후 바로 확인"으로 검증·연결한다.

Google 로그인이 "안전하지 않을 수 있음"으로 막히면: 어댑터는 기본으로
설치된 진짜 크롬으로 창을 띄운다(묶음 크로뮴이 아님). 묶음 크로뮴으로
되돌리려면 `ADAPTER_BROWSER_CHANNEL=bundled`.
실화면 로그인은 계정별 고정 프로필(`profiles/<계정>/profile`)을 써서
게스트처럼 안 보이고, 자동화 표시도 숨긴다.
Google 로그인이 그래도 막히면 본인 크롬으로 직접 연다:

```powershell
# 1. 크롬을 완전히 종료 (작업관리자에서 chrome.exe 확인)
# 2. 본인 프로필로 로그인 창 열기
adapter\.venv\Scripts\python -m adapter.login <계정명> <플랫폼> --real-profile
# 3. Google로 로그인 후 터미널에서 Enter (또는 콘솔에서 바로 확인)
```

콘솔 로그인 모달의 "내 Chrome 프로필로 열기" 체크도 같은 동작이다.
크롬이 켜져 있으면 시작 단계에서 409로 바로 알려준다.
게스트처럼 보이는 게 싫으면 `--incognito`로 시크릿 창을 띄울 수 있다.
(단, 시크릿은 흔적이 더 없어서 Google 심사는 그대로다.)

## 세션 검증

```powershell
adapter\.venv\Scripts\python -m adapter.verify <계정명> <플랫폼>
```

종료 0=로그인됨, 1=미로그인, 2=판정 불가. 콘솔 완료 확인이 매번 이
검증을 돌리고, 미로그인 세션은 자동 삭제한다.

## 실채팅

```powershell
adapter\.venv\Scripts\python -m adapter.chat <계정명> meta_ai "메시지"
```

새 대화로 1건 보내고 답장이 굳을 때까지 기다린 뒤 JSON으로 출력한다.
종료 0=답장, 1=실패, 2=불가(미로그인·셀렉터 미확정). 콘솔·작업자는
POST /api/chat/send·/api/worker/chat/send로 같은 경로를 쓴다.

```powershell
# 0. 콘솔 연결 화면에서 세션 계정 등록 (이름 예: pilot-naver)
#    http://127.0.0.1:18080/ → 연결 → 세션 계정 → 추가
# 1. 실화면 로그인 (네이버)
adapter\.venv\Scripts\python -m adapter.login pilot-naver naver_blog
# 2. 실화면 로그인 (티스토리, 쓸 때만)
adapter\.venv\Scripts\python -m adapter.login pilot-tistory tistory
```

브라우저가 열리면 직접 로그인(2단계·캡차 포함)한 뒤 터미널에서 Enter.
세션은 `data/profiles/<계정>/storage_state.json`에 저장되고 이후 발행은
헤드리스로 돈다. 로그인되면 총괄에게 알린다 → compose 셀렉터 확정 →
P레인 에이전트 연결 → 실발행 파일럿 순으로 진행한다.

## 워커 기동 (P레인 에이전트 토큰 필요)

```powershell
$env:AGENT_TOKEN = "<콘솔 에이전트 화면에서 발급>"
adapter\.venv\Scripts\python -m adapter.worker_adapter
```

## 드릴 (실계정 없이 기구 검증)

```powershell
python tools\stubboard.py 18399   # 가짜 플랫폼
$env:STUB_BASE_URL = "http://127.0.0.1:18399"
$env:ADAPTER_PLATFORM_MAP = "naver_blog=stubboard"
adapter\.venv\Scripts\python -m adapter.login stub-1 stubboard --auto u p
$env:AGENT_TOKEN = "<P레인 토큰>"
adapter\.venv\Scripts\python -m adapter.worker_adapter
```

## 테스트

```powershell
python adapter\test_adapter.py   # 순수 단위 (플레이ライト 불필요)
$env:PLAYWRIGHT_E2E = "1"
adapter\.venv\Scripts\python adapter\test_adapter.py   # 스텁 E2E 포함
```

## 주의

- 세션 파일은 자격과 동급이다. 백업에 포함하지 않고(금고 키·DB와 분리),
  타인과 공유하지 않는다.
- `ADAPTER_PLATFORM_MAP`은 드릴 전용이다. 상용에서는 플랫폼명을 그대로 쓴다.
- 증거 스크린샷은 `data/uploads/evidence/`에 남고, 발행 기록 note에 경로가
  함께 기록된다. 다중 호스트 증거 동기화는 P2 과제다.
