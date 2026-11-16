"""W20 ops observability probe: transcripts, audit, sessions, health, phrases.

Usage (fresh DATA_DIR, default dev secrets):
    SOMUN_URL=http://127.0.0.1:<port> SOMUN_DATADIR=<isolated dir> \\
        python tools/probes/w20.py

Standard library only. Everything writable goes under the isolated
SOMUN_DATADIR. Refuses to run on a non-fresh DB. Exits nonzero on the
first failure.

Execution plan (printed before running):
  1. assert fresh DB (no products)
  2. create work, seed one AI turn, read the full originals back
  3. pause/resume the work, read the audit history back
  4. fail the analyze job, stored reason is the screen phrase
  5. settings save + team batch + account unregister leave audit rows
  6. seeded queued post: hold + "not published" confirm leave audit rows
  7. health endpoint carries db/disk/executors/pcLoad/teamQueue/errors
  8. backup generation bundles the encrypted sessions file
  9. delete the work, the delete audit row survives
"""
import json
import os
import sqlite3
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from probe_lib import Client, ProbeError, check, summary

BASE_NEEDED = os.environ.get("SOMUN_URL", "").strip()
DATADIR = os.environ.get("SOMUN_DATADIR", "").strip()

PLAN = [
    "1. assert fresh DB (no products)",
    "2. create work, seed one AI turn, read the full originals back",
    "3. pause/resume the work, read the audit history back",
    "4. fail the analyze job, stored reason is the screen phrase",
    "5. settings save + team batch + account unregister leave audit rows",
    "6. seeded queued post: hold + 'not published' confirm leave audit rows",
    "7. health endpoint carries db/disk/executors/pcLoad/teamQueue/errors",
    "8. backup generation bundles the encrypted sessions file",
    "9. delete the work, the delete audit row survives",
]


def db():
    con = sqlite3.connect(os.path.join(DATADIR, "somun.db"))
    con.row_factory = sqlite3.Row
    return con


def main():
    for line in PLAN:
        print(line, flush=True)
    if not BASE_NEEDED or not DATADIR:
        raise ProbeError("SOMUN_URL and SOMUN_DATADIR are required")
    c = Client()
    tok = c.login()

    code, body = c.op("GET", "/api/products")
    items = body.get("products") or body.get("items") or []
    check("1-fresh", code == 200 and items == [], str((code, body)))

    # 2. work + one AI turn on its product thread.
    code, body = c.op("POST", "/api/products",
                      {"sourceUrl": "https://example.com/w20", "exitType": "A"})
    if code not in (200, 201):
        raise ProbeError("2-create: %s %s" % (code, body))
    pid = (body.get("product") or {}).get("id")
    aid = (body.get("analyzeJob") or {}).get("id")
    check("2-create", bool(pid and aid), str(body))
    con = db()
    acc = con.execute("INSERT INTO channel_accounts(name,platform,role,session_state,created_at,updated_at)"
                      " VALUES('w20-ai','meta_ai','worker','connected',datetime('now'),datetime('now'))")
    acc_id = acc.lastrowid
    con.execute("INSERT INTO chat_messages(account_id,role,text,thread_key,created_at)"
                " VALUES(?,'user','PROMPT-W20-PROBE','p%d',datetime('now'))" % pid, (acc_id,))
    con.execute("INSERT INTO chat_messages(account_id,role,text,thread_key,created_at)"
                " VALUES(?,'assistant','REPLY-W20-PROBE','p%d',datetime('now'))" % pid, (acc_id,))
    con.commit()
    con.close()
    code, body = c.op("GET", "/api/works/%d/ai-log" % pid)
    turns = body.get("items") or []
    check("2-originals", code == 200 and len(turns) == 1
          and turns[0].get("prompt") == "PROMPT-W20-PROBE"
          and turns[0].get("reply") == "REPLY-W20-PROBE"
          and turns[0].get("account") == "w20-ai", str(body))

    # 3. pause/resume audit.
    code, _ = c.op("POST", "/api/works/%d/pause" % pid)
    check("3-pause", code == 200, str(code))
    code, _ = c.op("POST", "/api/works/%d/resume" % pid)
    check("3-resume", code == 200, str(code))
    code, body = c.op("GET", "/api/works/%d/audit" % pid)
    rows = body.get("items") or []
    kinds = [r.get("action") for r in rows]
    check("3-audit", code == 200 and kinds == ["work_pause", "work_resume"]
          and all(r.get("actor") == "admin" and r.get("createdAt") for r in rows), str(body))

    # 4. failure translation.
    code, _ = c.wk("GET", "/api/worker/jobs/next?type=analyze")
    check("4-claim", code == 200, str(code))
    code, _ = c.wk("POST", "/api/worker/jobs/%d/fail" % aid, {"error": "Post https://x: connection refused"})
    check("4-fail", code == 200, str(code))
    code, body = c.op("GET", "/api/jobs")
    jobs = body.get("jobs") or body.get("items") or []
    mine = [j for j in jobs if j.get("id") == aid]
    check("4-phrase", code == 200 and len(mine) == 1
          and mine[0].get("error") == "연결이 끊겼습니다", str(mine))

    # 5. settings + team + unregister audit.
    code, settings = c.op("GET", "/api/settings/automation")
    check("5-settings-get", code == 200 and settings.get("backupIncludeSessions") is True, str(settings))
    settings["dailyCap"] = 5
    code, _ = c.op("PUT", "/api/settings/automation", settings)
    check("5-settings-put", code == 200, str(code))
    code, body = c.op("POST", "/api/teams", {"name": "w20팀", "businesses": []})
    if code not in (200, 201):
        raise ProbeError("5-team: %s %s" % (code, body))
    team_id = (body.get("team") or body).get("id")
    code, body = c.op("POST", "/api/session/accounts",
                      {"name": "w20-member", "platform": "meta_ai", "role": "worker",
                       "kind": "agent_service"})
    if code not in (200, 201):
        raise ProbeError("5-account: %s %s" % (code, body))
    member_id = (body.get("account") or body).get("id")
    code, _ = c.op("PUT", "/api/teams/%d/members" % team_id,
                   {"members": [{"accountId": member_id, "roles": []}]})
    check("5-members", code == 200, str(code))
    code, _ = c.op("DELETE", "/api/session/accounts/%d" % member_id)
    check("5-unregister", code == 200, str(code))
    con = db()
    recent = [dict(r) for r in con.execute(
        "SELECT action,actor FROM audit_log ORDER BY id DESC LIMIT 3")]
    con.close()
    check("5-audit", [r["action"] for r in recent] ==
          ["account_unregister", "team_members", "settings_change"]
          and all(r["actor"] == "admin" for r in recent), str(recent))

    # 6. seeded queued post: hold + confirm-missing audit.
    con = db()
    cur = con.execute("INSERT INTO contents(product_id,channel,target_platform,target_account,"
                      "title,body,status,created_at,updated_at)"
                      " VALUES(?,'blog','naver_blog','w20-ai','t','b','queued',datetime('now'),datetime('now'))",
                      (pid,))
    post_id = cur.lastrowid
    con.commit()
    con.close()
    code, _ = c.op("POST", "/api/works/%d/posts/%d/hold" % (pid, post_id), {"held": True})
    check("6-hold", code == 200, str(code))
    code, body = c.op("POST", "/api/works/%d/posts/%d/confirm-published" % (pid, post_id),
                      {"published": False})
    check("6-confirm", code == 200, str((code, body)))
    code, body = c.op("GET", "/api/works/%d/audit" % pid)
    kinds = [r.get("action") for r in (body.get("items") or [])]
    check("6-audit", code == 200 and kinds[-2:] == ["post_hold", "confirm_missing"], str(kinds))

    # 7. health shape.
    code, body = c.op("GET", "/api/system/health")
    check("7-shape", code == 200 and body.get("db", {}).get("ok") is True
          and (body.get("disk") or {}).get("totalBytes", 0) > 0
          and "agents" in (body.get("executors") or {})
          and isinstance(body.get("pcLoad"), list)
          and isinstance(body.get("teamQueue"), list)
          and isinstance(body.get("recentErrors"), list)
          and len(body.get("recentErrors")) >= 1, str(code))

    # 8. backup bundles encrypted sessions.
    prof = os.path.join(DATADIR, "profiles", "w20-ai")
    os.makedirs(prof, exist_ok=True)
    with open(os.path.join(prof, "storage_state.json"), "w", encoding="utf-8") as fh:
        fh.write('{"w20":true}')
    code, body = c.op("POST", "/api/system/backup")
    check("8-backup", code == 200, str((code, body)))
    code, body = c.op("GET", "/api/system/backups")
    gens = body.get("backups") or []
    check("8-list", code == 200 and len(gens) >= 1, str(body))
    bundle = os.path.join(DATADIR, "backups", gens[0], "profiles.enc")
    with open(bundle, "rb") as fh:
        magic = fh.read(8)
    check("8-bundle", magic == b"SOMUNP1\x00", magic.hex())

    # 9. delete keeps its audit row.
    code, _ = c.op("DELETE", "/api/works/%d" % pid)
    check("9-delete", code == 200, str(code))
    con = db()
    row = con.execute("SELECT action,actor,work_id FROM audit_log WHERE action='work_delete'"
                      " ORDER BY id DESC LIMIT 1").fetchone()
    con.close()
    check("9-audit", row is not None and row["actor"] == "admin" and row["work_id"] == pid,
          str(dict(row) if row else None))

    summary("W20")


if __name__ == "__main__":
    try:
        main()
    except ProbeError as exc:
        print("W20 FAIL %s" % exc, flush=True)
        raise SystemExit(1)
