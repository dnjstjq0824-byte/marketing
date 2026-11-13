"""Real-AI task prompts and reply parsers (stdlib only).

Workers with SOMUN_REAL_AI=1 send these prompts through /api/worker/chat/send
to a connected AI account and parse the JSON replies into job results.
Anything unparseable raises ValueError, which the worker loop records as a
job failure for human review. Nothing here guesses: missing facts stay blank.
"""
import json
import re
import urllib.request

ANALYZE_FIELDS = ("product_name", "price_summary", "options_text",
                  "shipping_text", "detail_core", "review_summary",
                  "image_desc", "unknowns")
ANALYZE_FACTS = tuple(f for f in ANALYZE_FIELDS if f != "unknowns")


def fetch_page_text(url, timeout=20, limit=8000):
    """Best-effort visible text from a URL. "" on any failure."""
    if not url:
        return ""
    try:
        req = urllib.request.Request(
            url, headers={"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"})
        with urllib.request.urlopen(req, timeout=timeout) as res:
            raw = res.read(200000).decode("utf-8", errors="ignore")
    except Exception:
        return ""
    text = re.sub(r"(?is)<(script|style|noscript)[^>]*>.*?</\1>", " ", raw)
    text = re.sub(r"(?s)<[^>]*>", " ", text)
    text = re.sub(r"\s+", " ", text).strip()
    return text[:limit]
STRATEGY_FIELDS = ("angle", "channels", "key_messages", "schedule_note")
GENERATE_CHANNELS = ("instagram", "shortform", "cardnews", "blog")

_JSON_RULE = ("반드시 JSON 한 개만 출력한다. 설명·인사·마크다운 금지. "
              "모르는 값은 빈 문자열로 둔다.")


def build_prompt(job_type, payload):
    """Build the AI prompt for one job. payload is the claim payload dict."""
    if job_type == "analyze":
        page = (payload.get("page_text", "") or "")[:6000]
        return ("상품 판독. 아래 정보를 읽고 JSON으로 답한다. %s\n"
                "키: %s\n"
                "source_url: %s\nimages: %s\npage_text: %s"
                % (_JSON_RULE, ",".join(ANALYZE_FIELDS),
                   payload.get("source_url", ""),
                   ",".join(payload.get("images", []) or []),
                   page if page else "(페이지 본문 없음)"))
    if job_type == "strategy":
        return ("판독 기반 전략 수립. 아래 판독을 읽고 JSON으로 답한다. %s\n"
                "키: %s. 모든 값은 문자열. channels는 쉼표 문자열"
                "(예: instagram,blog).\nproduct_name: %s\ndetail_core: %s"
                % (_JSON_RULE, ",".join(STRATEGY_FIELDS),
                   payload.get("product_name", ""),
                   payload.get("detail_core", "")))
    if job_type == "generate":
        revision = ""
        if payload.get("revision_of"):
            revision = ("\n재생성: 초안 %s번이 반려됐다. 피드백을 반영해 새로 쓴다.\n"
                        "feedback: %s"
                        % (payload.get("revision_of"),
                           payload.get("feedback", "")))
        return ("전략 기반 초안 생성. 아래 전략을 읽고 JSON으로 답한다. %s\n"
                "형식: {\"contents\": [{\"channel\": 채널명, \"title\": 제목, "
                "\"body\": 본문}]}. 채널은 %s 중에서만 쓴다.\nangle: %s\n"
                "key_messages: %s%s"
                % (_JSON_RULE, ",".join(GENERATE_CHANNELS),
                   payload.get("angle", ""),
                   payload.get("key_messages", ""), revision))
    if job_type == "review":
        ids = payload.get("content_ids", []) or []
        drafts = []
        for item in payload.get("contents", []) or []:
            if not isinstance(item, dict):
                continue
            drafts.append("[%s/%s] %s\n%s" % (
                item.get("content_id", "?"), item.get("channel", "?"),
                item.get("title", ""), item.get("body", "")))
        facts = payload.get("facts", {}) or {}
        fact_lines = []
        for key in ("product_name", "price_summary", "detail_core",
                    "review_summary"):
            if facts.get(key):
                fact_lines.append("%s: %s" % (key, facts[key]))
        return ("초안 검수. 아래 초안을 읽고 판정하고 JSON으로 답한다. %s\n"
                "형식: {\"decisions\": [{\"content_id\": ID, \"approve\": "
                "true/false, \"reason\": 판정 사유 한 줄(필수, 빈 문자열 금지)}]}.\n"
                "승인 기준: 아래 사실 기준과 다름없고, 채널에 맞고, 금칙어·"
                "허위가 없음. 사실 기준에 있는 내용은 참으로 인정한다.\n"
                "사실 기준:\n%s\n초안:\n%s"
                % (_JSON_RULE,
                   "\n".join(fact_lines) if fact_lines else "(사실 기준 없음)",
                   "\n\n".join(drafts) if drafts else
                   "(초안 본문 없음, content_ids: %s)"
                   % ",".join(str(i) for i in ids)))
    raise ValueError("unknown job type: %s" % job_type)


def _extract_json(text):
    start = text.find("{")
    end = text.rfind("}")
    if start < 0 or end <= start:
        raise ValueError("no JSON object in reply")
    try:
        return json.loads(text[start:end + 1])
    except ValueError as exc:
        raise ValueError("bad JSON in reply: %s" % exc)


def _string_fields(data, fields):
    out = {}
    for key in fields:
        value = data.get(key, "")
        if not isinstance(value, str):
            raise ValueError("field %s must be a string" % key)
        out[key] = value
    return out


def parse_reply(job_type, text):
    """Parse one AI reply into a job result dict. Raises ValueError."""
    data = _extract_json(text or "")
    if not isinstance(data, dict):
        raise ValueError("reply JSON must be an object")
    if job_type == "analyze":
        result = _string_fields(data, ANALYZE_FIELDS)
        if not any(result[f].strip() for f in ANALYZE_FACTS):
            raise ValueError("empty reading: no facts confirmed")
        return result
    if job_type == "strategy":
        channels = data.get("channels", "")
        if isinstance(channels, list):
            channels = ",".join(str(c) for c in channels)
            data = dict(data, channels=channels)
        result = _string_fields(data, STRATEGY_FIELDS)
        if not result["angle"].strip():
            raise ValueError("angle must not be empty")
        return result
    if job_type == "generate":
        items = data.get("contents")
        if not isinstance(items, list) or not items:
            raise ValueError("contents must be a non-empty list")
        kept = []
        for item in items:
            if not isinstance(item, dict):
                continue
            channel = item.get("channel", "")
            title = item.get("title", "")
            body = item.get("body", "")
            if channel not in GENERATE_CHANNELS:
                continue
            if not isinstance(body, str) or not body.strip():
                continue
            if not isinstance(title, str):
                continue
            kept.append({"channel": channel, "title": title, "body": body})
        if not kept:
            raise ValueError("no valid draft kept")
        return {"contents": kept}
    if job_type == "review":
        items = data.get("decisions")
        if not isinstance(items, list) or not items:
            raise ValueError("decisions must be a non-empty list")
        out = []
        for item in items:
            if not isinstance(item, dict):
                raise ValueError("bad decision item")
            cid = item.get("content_id", 0)
            if isinstance(cid, str) and cid.strip().isdigit():
                cid = int(cid.strip())
            if not isinstance(cid, int) or isinstance(cid, bool) or cid <= 0:
                raise ValueError("bad content_id in decision")
            approve = item.get("approve")
            if not isinstance(approve, bool):
                raise ValueError("approve must be true/false")
            reason = item.get("reason", "")
            if not isinstance(reason, str):
                raise ValueError("reason must be a string")
            out.append({"content_id": cid, "approve": approve,
                        "reason": reason})
        return {"decisions": out}
    raise ValueError("unknown job type: %s" % job_type)
