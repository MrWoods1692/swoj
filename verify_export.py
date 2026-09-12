#!/usr/bin/env python3
"""
verify_export.py  比赛成绩表多格式导出验证

覆盖：
- 匿名 / 无 format / 不支持的 format → 400
- 不存在的 contest → 404
- 6 种格式均可下载，Content-Type / Content-Disposition / X-Rows / X-Format 均正确
- csv/tsv：UTF-8 BOM + 表头 + 数据行；tsv 使用制表符
- json：数组，含 rank/username/accepted/first_at 字段
- markdown：管道表格 + 表头 + 数据行
- html：包含 <table> + 转义
- xlsx：可解压为 OOXML，含 sheet1.xml 和 workbook.xml，数据正确
- 排名规则：与 /api/contests/{id}/rank 一致（按去重 AC 题数降序、首次通过时间升序）
"""
import os, sys, zipfile, io, re, json, sqlite3, time

import requests

BASE = os.environ.get("SWOJ_HOST", "http://127.0.0.1:18098")
DATA_DIR = os.environ.get("SWOJ_DATA_DIR", "/tmp/swoj-export-test")
DB = os.path.join(DATA_DIR, "db", "swoj.db")

PASS = 0
FAIL = 0

def check(name, cond, detail=""):
    global PASS, FAIL
    if cond:
        PASS += 1
        print(f"  [ok]   {name}")
    else:
        FAIL += 1
        print(f"  [FAIL] {name}  {detail}")


def login(name, qq):
    """mock OAuth：单次 GET + 跟随重定向，回调返回 JSON（token+csrf+user）。"""
    r = requests.get(f"{BASE}/auth/campux",
                     params={"mock": "1", "mockname": name, "mockqq": qq},
                     allow_redirects=True)
    if r.status_code != 200:
        raise RuntimeError(f"login failed: {r.status_code} {r.text[:200]}")
    j = r.json()
    token = j.get("data", {}).get("token")
    csrf = j.get("data", {}).get("csrf")
    if not token or not csrf:
        raise RuntimeError(f"login no token/csrf: {r.text[:200]}")
    sess = requests.Session()
    sess.cookies.set("swoj_token", token)
    sess.cookies.set("swoj_csrf", csrf)
    sess.headers["X-CSRF-Token"] = csrf
    return sess


def db_conn():
    return sqlite3.connect(DB)


def seed(contest_id, problems, users):
    """
    在指定 contest 下为若干用户造 AC 提交。
    problems: 已存在的 problem 列表
    users: [(username, [(problem_index, offset_seconds), ...])]
    """
    conn = db_conn()
    base = time.time()
    for u, items in users:
        for pi, off in items:
            pid = problems[pi]
            conn.execute(
                "INSERT INTO submissions(user_id, username, problem_id, code, mode, contest_id, status, created_at) "
                "VALUES (?,?,?,'int main(){return 0;}', 'cpp', ?, ?, datetime(?, 'unixepoch'))",
                (0, u, pid, contest_id, 1, off + base))
    conn.commit()
    conn.close()


def main():
    global PASS, FAIL

    # 用管理员身份建题、建比赛
    admin = login("超级管理员", "qq_admin_export")
    # 建 3 道题
    problems = []
    for i in range(3):
        r = admin.post(f"{BASE}/api/admin/problems",
                       json={"name": f"导出题{i}", "content": "题面"})
        assert r.status_code == 200, r.text[:200]
        problems.append(r.json()["data"]["id"])
    check("建 3 道种子题", len(problems) == 3, problems)

    # 建 1 个 rank 比赛（含 rank 开关，用于展示排行榜）
    now = time.time()
    start_str = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime(now - 3600))
    end_str = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime(now + 86400))
    r = admin.post(f"{BASE}/api/contests",
                   json={"name": "导出测试赛", "info": "多格式导出验证", "rank": True, "visible": True,
                         "start_time": start_str, "end_time": end_str})
    assert r.status_code == 200, r.text[:200]
    cid = r.json()["data"]["id"]
    check("建比赛", cid > 0, cid)

    # 造数据：给 3 个独立用户造 AC 提交
    # 导出Alice 通过 3 题（去重 3）；导出Bob 提交 3 次但只有 2 道去重；导出Carol 通过 2 题且时间更早
    # 排名口径：去重题数降序 → 导出Alice(3) > 导出Carol(2) & 导出Bob(2)；同题数按首次通过时间升序 → 导出Carol 早于 导出Bob
    users = {}
    for name, qq in [("导出Alice", "qq_alice_export"), ("导出Bob", "qq_bob_export"), ("导出Carol", "qq_carol_export")]:
        conn = db_conn()
        conn.execute("INSERT OR IGNORE INTO users(username, oauth_provider, oauth_id, oauth_name, role, qq, can_submit) VALUES (?,?,?,?,?,?,1)",
                     (name, "mock", qq, name, "user", qq))
        conn.commit()
        users[name] = conn.execute("SELECT id FROM users WHERE qq=?", (qq,)).fetchone()[0]
        conn.close()
    now = time.time()
    conn = db_conn()
    def add(name, pid, off):
        # 用 datetime(?, 'unixepoch') 写入 TEXT 时间戳，与 handler_submission.go 的
        # SQLite 默认 CURRENT_TIMESTAMP 一致（modernc.org/sqlite 不隐式转 time.Time，
        # handler_contest.go 已改为 Scan 成 string）。
        conn.execute("INSERT INTO submissions(user_id, username, problem_id, code, mode, contest_id, status, created_at) VALUES (?,?,?,?, 'cpp', ?, 1, datetime(?, 'unixepoch'))",
                     (users[name], name, problems[pid], 'int main(){return 0;}', cid, off + now))
    # 导出Alice: 3 题
    add("导出Alice", 0, 100); add("导出Alice", 1, 200); add("导出Alice", 2, 300)
    # 导出Bob: 同题 2 次 + 另一题 1 次 = 2 去重
    add("导出Bob", 0, 500); add("导出Bob", 0, 501); add("导出Bob", 1, 600)
    # 导出Carol: 2 题，时间早于 导出Bob
    add("导出Carol", 0, 50); add("导出Carol", 1, 150)
    conn.commit()
    conn.close()

    # 等 SQLite 落盘（WAL）
    time.sleep(0.5)

    # 先跑一次 rank 端点确认口径
    r = requests.get(f"{BASE}/api/contests/{cid}/rank")
    check("rank 200", r.status_code == 200, r.status_code)
    raw = r.json()
    # /api/contests/{id}/rank 走 OK(w, list) 包装，data 才是列表
    rank_data = raw.get("data", raw) if isinstance(raw, dict) else raw
    check("rank 3 行", len(rank_data) == 3, len(rank_data))
    check("rank[0] 是 导出Alice", rank_data[0]["username"] == "导出Alice", rank_data[0])
    check("rank[0] accepted=3", rank_data[0]["accepted"] == 3, rank_data[0])
    check("rank[1] 是 导出Carol", rank_data[1]["username"] == "导出Carol", rank_data[1])
    check("rank[2] 是 导出Bob", rank_data[2]["username"] == "导出Bob", rank_data[2])

    # ---------- 参数校验 ----------
    r = requests.get(f"{BASE}/api/contests/{cid}/rank/export")
    # 默认 csv，应 200
    check("默认 format=csv 返回 200", r.status_code == 200, r.status_code)

    r = requests.get(f"{BASE}/api/contests/{cid}/rank/export", params={"format": "txt"})
    check("不支持的 format → 400", r.status_code == 400, r.status_code)

    r = requests.get(f"{BASE}/api/contests/99999/rank/export", params={"format": "csv"})
    check("不存在的 contest → 404", r.status_code == 404, r.status_code)

    r = requests.get(f"{BASE}/api/contests/notanum/rank/export", params={"format": "csv"})
    check("非数字 id → 400", r.status_code == 400, r.status_code)

    # ---------- CSV ----------
    r = requests.get(f"{BASE}/api/contests/{cid}/rank/export", params={"format": "csv"})
    check("csv 200", r.status_code == 200, r.status_code)
    ct = r.headers.get("Content-Type", "")
    check("csv Content-Type", "text/csv" in ct, ct)
    cd = r.headers.get("Content-Disposition", "")
    check("csv Content-Disposition 含 csv", ".csv" in cd, cd)
    check("csv X-Rows=3", r.headers.get("X-Rows") == "3", r.headers.get("X-Rows"))
    check("csv X-Format=csv", r.headers.get("X-Format") == "csv", r.headers.get("X-Format"))
    body = r.content
    check("csv BOM", body[:3] == b"\xef\xbb\xbf", body[:3].hex())
    text = body.decode("utf-8-sig")
    lines = text.strip().splitlines()
    check("csv 行数 = 4", len(lines) == 4, lines)
    check("csv 表头", lines[0] == "名次,用户名,通过题数,首次通过时间", lines[0])
    check("csv 第 1 行名次=1", lines[1].startswith("1,导出Alice,"), lines[1])
    check("csv 第 2 行名次=2", lines[2].startswith("2,导出Carol,"), lines[2])
    check("csv 第 3 行名次=3", lines[3].startswith("3,导出Bob,"), lines[3])

    # ---------- TSV ----------
    r = requests.get(f"{BASE}/api/contests/{cid}/rank/export", params={"format": "tsv"})
    check("tsv 200", r.status_code == 200, r.status_code)
    check("tsv Content-Type", "tab-separated" in r.headers.get("Content-Type", ""), r.headers.get("Content-Type"))
    body = r.content
    check("tsv BOM", body[:3] == b"\xef\xbb\xbf", body[:3].hex())
    text = body.decode("utf-8-sig")
    lines = text.strip().splitlines()
    check("tsv 表头 tab", lines[0] == "名次\t用户名\t通过题数\t首次通过时间", lines[0])
    check("tsv 数据 tab", lines[1].startswith("1\t导出Alice\t3\t"), lines[1])

    # ---------- JSON ----------
    r = requests.get(f"{BASE}/api/contests/{cid}/rank/export", params={"format": "json"})
    check("json 200", r.status_code == 200, r.status_code)
    check("json Content-Type", "application/json" in r.headers.get("Content-Type", ""), r.headers.get("Content-Type"))
    j = r.json()
    check("json 是数组", isinstance(j, list), type(j))
    check("json 长度 3", len(j) == 3, len(j))
    check("json rank=1", j[0]["rank"] == 1, j[0])
    check("json username", j[0]["username"] == "导出Alice", j[0])
    check("json accepted", j[0]["accepted"] == 3, j[0])
    check("json first_at 格式", re.match(r"\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$", j[0]["first_at"]), j[0])

    # ---------- MARKDOWN ----------
    r = requests.get(f"{BASE}/api/contests/{cid}/rank/export", params={"format": "markdown"})
    check("md 200", r.status_code == 200, r.status_code)
    check("md Content-Type", "markdown" in r.headers.get("Content-Type", ""), r.headers.get("Content-Type"))
    text = r.text
    check("md 含 H1", text.startswith("# "), text[:50])
    check("md 含比赛名", "导出测试赛" in text, "")
    check("md 含分隔行", "| --- | --- | --- | --- |" in text, text[:500])
    check("md 含 导出Alice", "导出Alice" in text, "")
    # 计算数据行（以 | 开头但不是分隔行的行）
    md_rows = [l for l in text.splitlines() if l.startswith("|") and "---" not in l]
    check("md 表头 + 3 行数据 = 4", len(md_rows) == 4, len(md_rows))

    # ---------- HTML ----------
    r = requests.get(f"{BASE}/api/contests/{cid}/rank/export", params={"format": "html"})
    check("html 200", r.status_code == 200, r.status_code)
    check("html Content-Type", "text/html" in r.headers.get("Content-Type", ""), r.headers.get("Content-Type"))
    text = r.text
    check("html 含 doctype", "<!doctype html>" in text.lower(), "")
    check("html 含 table", "<table>" in text, "")
    check("html 含 th", "<th>名次</th>" in text, text[:400])
    check("html 含 导出Alice", "导出Alice" in text, "")
    check("html CSP 头", r.headers.get("Content-Security-Policy") == "default-src 'none'", r.headers.get("Content-Security-Policy"))
    # 数据行数量：<tr> 除表头外的行数
    data_tr = text.count("<tr>")
    check("html 4 个 tr (表头 + 3 数据)", data_tr == 4, data_tr)

    # ---------- XLSX ----------
    r = requests.get(f"{BASE}/api/contests/{cid}/rank/export", params={"format": "xlsx"})
    check("xlsx 200", r.status_code == 200, r.status_code)
    check("xlsx Content-Type", "spreadsheetml" in r.headers.get("Content-Type", ""), r.headers.get("Content-Type"))
    check("xlsx Content-Disposition 含 xlsx", ".xlsx" in r.headers.get("Content-Disposition", ""), "")
    body = r.content
    check("xlsx 是 zip (PK)", body[:2] == b"PK", body[:4].hex())
    try:
        with zipfile.ZipFile(io.BytesIO(body)) as z:
            names = z.namelist()
        check("xlsx 含 [Content_Types].xml", "[Content_Types].xml" in names, names)
        check("xlsx 含 _rels/.rels", "_rels/.rels" in names, names)
        check("xlsx 含 xl/workbook.xml", "xl/workbook.xml" in names, names)
        check("xlsx 含 xl/worksheets/sheet1.xml", "xl/worksheets/sheet1.xml" in names, names)
        with zipfile.ZipFile(io.BytesIO(body)) as z:
            ct = z.read("[Content_Types].xml").decode()
            check("xlsx Content_Types 声明 worksheet", "worksheet+xml" in ct, ct[:200])
            wb = z.read("xl/workbook.xml").decode()
            check("xlsx workbook 含比赛名", "导出测试赛" in wb, wb[:300])
            sheet = z.read("xl/worksheets/sheet1.xml").decode()
            check("xlsx sheet 含名次表头", "名次" in sheet, sheet[:200])
            check("xlsx sheet 含 导出Alice", "导出Alice" in sheet, "")
            check("xlsx sheet 含 导出Bob", "导出Bob" in sheet, "")
            check("xlsx sheet 含 导出Carol", "导出Carol" in sheet, "")
            # 检查 A1 表头：inlineStr
            check("xlsx A1 表头引用", 'r="A1"' in sheet, sheet[:400])
            check("xlsx A1 是 inlineStr", 't="inlineStr"' in sheet[:500], sheet[:500])
    except Exception as e:
        check("xlsx 解压", False, str(e))

    # ---------- XLSX 空结果 ----------
    r = admin.post(f"{BASE}/api/contests",
                   json={"name": "空比赛", "info": "", "rank": True, "visible": True,
                         "start_time": "2026-01-01 00:00:00", "end_time": "2026-12-31 23:59:59"})
    assert r.status_code == 200, r.text[:200]
    empty_cid = r.json()["data"]["id"]
    r = requests.get(f"{BASE}/api/contests/{empty_cid}/rank/export", params={"format": "csv"})
    check("空比赛 csv 200", r.status_code == 200, r.status_code)
    check("空比赛 X-Rows=0", r.headers.get("X-Rows") == "0", r.headers.get("X-Rows"))
    text = r.content.decode("utf-8-sig")
    lines = text.strip().splitlines()
    check("空比赛只有表头", len(lines) == 1, lines)
    r = requests.get(f"{BASE}/api/contests/{empty_cid}/rank/export", params={"format": "xlsx"})
    check("空比赛 xlsx 200", r.status_code == 200, r.status_code)
    check("空比赛 xlsx 是 zip", r.content[:2] == b"PK", r.content[:4].hex())

    # ---------- 汇总 ----------
    print("=" * 40)
    print(f"通过 {PASS} / 失败 {FAIL}")
    print("=" * 40)
    sys.exit(0 if FAIL == 0 else 1)


if __name__ == "__main__":
    main()
