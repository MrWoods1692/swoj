#!/usr/bin/env python3
"""verify_notes.py — 个人笔记端点验证。

覆盖：
  [1]  GET /api/notes 匿名 → 401
  [2]  未登录写方法 → 403（CSRF 在 auth 之前）
  [3]  用户创建笔记 → 200 且入库
  [4]  列表返回字段齐全（title/snippet/tags/pinned/created_at/updated_at）
  [5]  详情返回完整 content
  [6]  字段边界：标题为空/仅空白/超长、正文超长、非法 JSON
  [7]  标签规范化：多分隔符、去重、去空、上限 8、长度 12
  [8]  更新笔记（含 pinned 切换）
  [9]  404 与跨用户越权（读/改/删）
  [10] 删除笔记，重复删除 404
  [11] operation_logs 留痕 note_create/update/delete
  [12] 前端静态检查：路由/导航/页面调用端点/字段
  [13] 后端静态检查：handler/schema/router 关键字段
"""
import http.client
import json
import os
import re
import sqlite3
import urllib.parse

HOST, PORT = (os.environ.get("SWOJ_HOST_PORT", "127.0.0.1:18080").split(":"))
HOST, PORT = HOST, int(PORT)
DB = os.path.join(os.environ.get("SWOJ_DATA_DIR", "/tmp/swoj-notes"), "db", "swoj.db")
WEB = os.path.join(os.environ.get("SWOJ_WEB", "/home/mrcwoods/code/swoj/web/src"))

passed, failed = [], []
JARS = {}


def check(name, cond, detail=""):
    (passed if cond else failed).append(name)
    print(("  ok  " if cond else "  FAIL") + f" {name}" + (f"  — {detail}" if detail else ""))


def req(method, path, body=None, jar=None, extra_hdr=None, raw_body=None):
    c = http.client.HTTPConnection(HOST, PORT, timeout=15)
    headers, data = {}, None
    if raw_body is not None:
        data = raw_body
    elif body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    if extra_hdr:
        headers.update(extra_hdr)
    if jar and JARS.get(jar):
        headers["Cookie"] = "; ".join(f"{k}={v}" for k, v in JARS[jar].items())
    c.request(method, path, body=data, headers=headers)
    r = c.getresponse()
    raw = r.read().decode()
    if jar:
        JARS.setdefault(jar, {})
        for h, v in r.getheaders():
            if h.lower() == "set-cookie":
                for part in v.split(";"):
                    if "=" in part:
                        k, _, val = part.partition("=")
                        k, val = k.strip(), val.strip()
                        if k == "swoj_oauth_verifier" and not val:
                            continue
                        JARS[jar][k] = val
    try:
        data_json = json.loads(raw)
    except Exception:
        data_json = {}
    c.close()
    return r.status, data_json, raw


def login(jar, qq, name):
    qa = "?" + urllib.parse.urlencode({"mock": "1", "mockname": name, "mockqq": qq})
    req("GET", "/auth/campux" + qa, jar=jar)
    req("GET", "/auth/campux" + qa, jar=jar)
    st = JARS.get(jar, {}).get("swoj_oauth_state", "")
    cb = "?" + urllib.parse.urlencode({"mock": "1", "state": st, "mockname": name, "mockqq": qq})
    s, d, _ = req("GET", "/auth/campux/callback" + cb, jar=jar)
    assert s == 200, f"callback status={s}"
    data = d["data"]
    JARS.setdefault(jar, {})["swoj_token"] = data["token"]
    JARS[jar]["swoj_csrf"] = data["csrf"]
    return data["token"], data["csrf"], data["user"]


def auth_hdr(jar):
    return {"Authorization": f"Bearer {JARS[jar]['swoj_token']}",
            "X-CSRF-Token": JARS[jar]["swoj_csrf"]}


def read_file(p):
    try:
        with open(os.path.join(WEB, p), encoding="utf-8") as f:
            return f.read()
    except FileNotFoundError:
        return ""


def read_repo_file(p):
    try:
        with open(os.path.join("/home/mrcwoods/code/swoj", p), encoding="utf-8") as f:
            return f.read()
    except FileNotFoundError:
        return ""


def run():
    print("\n[1] 匿名访问 → 401/403")
    s, d, _ = req("GET", "/api/notes")
    check("匿名 GET /api/notes → 401", s == 401, f"status={s}")
    s, d, _ = req("GET", "/api/notes/1")
    check("匿名 GET /api/notes/1 → 401", s == 401, f"status={s}")
    s, d, _ = req("POST", "/api/notes", body={"title": "x", "content": "y"})
    check("匿名 POST /api/notes → 403（CSRF）", s == 403, f"status={s}")
    s, d, _ = req("PUT", "/api/notes/1", body={"title": "x", "content": "y"})
    check("匿名 PUT /api/notes/1 → 403", s == 403, f"status={s}")
    s, d, _ = req("DELETE", "/api/notes/1")
    check("匿名 DELETE /api/notes/1 → 403", s == 403, f"status={s}")

    print("\n[2] 登录两位用户")
    login("alice", "13900000300", "笔记用户甲")
    login("bob", "13900000301", "笔记用户乙")
    check("alice 拿到 token", bool(JARS.get("alice", {}).get("swoj_token")))
    check("bob 拿到 token", bool(JARS.get("bob", {}).get("swoj_token")))

    print("\n[3] alice 创建笔记")
    s, d, _ = req("POST", "/api/notes", jar="alice", extra_hdr=auth_hdr("alice"),
                  body={"title": "动态规划入门", "content": "从背包问题开始，先理解状态转移…",
                        "tags": "dp,背包,基础", "pinned": False})
    check("POST /api/notes → 200", s == 200, f"status={s} data={d}")
    nid = (d.get("data") or {}).get("id") if s == 200 else None
    check("返回 id 为正整数", isinstance(nid, int) and nid > 0, f"id={nid}")

    print("\n[4] 列表字段齐全")
    s, d, _ = req("GET", "/api/notes", jar="alice", extra_hdr=auth_hdr("alice"))
    check("GET /api/notes → 200", s == 200, f"status={s}")
    lst = (d.get("data") or {}).get("list") or []
    check("列表包含新建笔记", any(n.get("id") == nid for n in lst), f"list_len={len(lst)}")
    if lst:
        n0 = next(n for n in lst if n["id"] == nid)
        check("title 正确", n0.get("title") == "动态规划入门", f"title={n0.get('title')!r}")
        check("tags 是数组", isinstance(n0.get("tags"), list), f"tags={n0.get('tags')}")
        check("tags 包含 dp", "dp" in (n0.get("tags") or []), f"tags={n0.get('tags')}")
        check("tags 包含 背包", "背包" in (n0.get("tags") or []), f"tags={n0.get('tags')}")
        check("pinned 是布尔", isinstance(n0.get("pinned"), bool), f"pinned={n0.get('pinned')}")
        check("pinned=false", n0.get("pinned") is False)
        check("snippet 存在", "snippet" in n0, f"keys={list(n0.keys())}")
        check("snippet 包含『背包』", "背包" in (n0.get("snippet") or ""), f"snippet={n0.get('snippet')!r}")
        check("created_at 存在", "created_at" in n0 and n0["created_at"])
        check("updated_at 存在", "updated_at" in n0 and n0["updated_at"])

    print("\n[5] 详情完整内容")
    if nid:
        s, d, _ = req("GET", f"/api/notes/{nid}", jar="alice", extra_hdr=auth_hdr("alice"))
        check("GET 详情 → 200", s == 200, f"status={s}")
        check("content 完整", "状态转移" in (d.get("data") or {}).get("content", ""),
              f"content={(d.get('data') or {}).get('content','')[:60]!r}")
        check("detail 无 snippet 字段", "snippet" not in (d.get("data") or {}))

    print("\n[6] 字段边界")
    s, d, _ = req("POST", "/api/notes", jar="alice", extra_hdr=auth_hdr("alice"),
                  body={"title": "", "content": "x"})
    check("标题为空 → 400", s == 400, f"status={s}")
    s, d, _ = req("POST", "/api/notes", jar="alice", extra_hdr=auth_hdr("alice"),
                  body={"title": "   ", "content": "x"})
    check("标题仅空白 → 400", s == 400, f"status={s}")
    s, d, _ = req("POST", "/api/notes", jar="alice", extra_hdr=auth_hdr("alice"),
                  body={"title": "a" * 81, "content": "x"})
    check("标题 81 字 → 400", s == 400, f"status={s}")
    s, d, _ = req("POST", "/api/notes", jar="alice", extra_hdr=auth_hdr("alice"),
                  body={"title": "a" * 80, "content": "x"})
    check("标题 80 字 → 200", s == 200, f"status={s}")
    if s == 200:
        req("DELETE", f"/api/notes/{d['data']['id']}", jar="alice", extra_hdr=auth_hdr("alice"))
    s, d, _ = req("POST", "/api/notes", jar="alice", extra_hdr=auth_hdr("alice"),
                  body={"title": "边界", "content": "x" * 100_001})
    check("正文 100001 字 → 400", s == 400, f"status={s}")
    s, d, _ = req("POST", "/api/notes", jar="alice", extra_hdr=auth_hdr("alice"),
                  raw_body=b"{invalid")
    check("非法 JSON → 400", s == 400, f"status={s}")

    print("\n[7] 标签规范化")
    s, d, _ = req("POST", "/api/notes", jar="alice", extra_hdr=auth_hdr("alice"),
                  body={"title": "标签测试", "content": "x",
                        "tags": "dp, dp ，dp  ；贪心， 动态规划、搜索;搜索,暴力"})
    if s == 200:
        nid2 = d["data"]["id"]
        s, d, _ = req("GET", f"/api/notes/{nid2}", jar="alice", extra_hdr=auth_hdr("alice"))
        tags = (d.get("data") or {}).get("tags") or []
        check("去重", len(tags) == len(set(tags)), f"tags={tags}")
        check("去空", all(t != "" for t in tags), f"tags={tags}")
        check("包含 dp", "dp" in tags, f"tags={tags}")
        check("包含 贪心", "贪心" in tags, f"tags={tags}")
        check("包含 搜索", "搜索" in tags, f"tags={tags}")
        check("包含 暴力", "暴力" in tags, f"tags={tags}")
        req("DELETE", f"/api/notes/{nid2}", jar="alice", extra_hdr=auth_hdr("alice"))
    s, d, _ = req("POST", "/api/notes", jar="alice", extra_hdr=auth_hdr("alice"),
                  body={"title": "标签上限", "content": "x",
                        "tags": "a1,a2,a3,a4,a5,a6,a7,a8,a9,a10"})
    if s == 200:
        nid3 = d["data"]["id"]
        s, d, _ = req("GET", f"/api/notes/{nid3}", jar="alice", extra_hdr=auth_hdr("alice"))
        tags = (d.get("data") or {}).get("tags") or []
        check("标签上限 8 个", len(tags) == 8, f"tags={tags}")
        req("DELETE", f"/api/notes/{nid3}", jar="alice", extra_hdr=auth_hdr("alice"))
    s, d, _ = req("POST", "/api/notes", jar="alice", extra_hdr=auth_hdr("alice"),
                  body={"title": "标签长度", "content": "x",
                        "tags": "短,超长标签超过十二个字的上限,中"})
    if s == 200:
        nid4 = d["data"]["id"]
        s, d, _ = req("GET", f"/api/notes/{nid4}", jar="alice", extra_hdr=auth_hdr("alice"))
        tags = (d.get("data") or {}).get("tags") or []
        check("超长标签被丢弃", "超长标签超过十二个字的上限" not in tags, f"tags={tags}")
        check("短标签保留", "短" in tags, f"tags={tags}")
        req("DELETE", f"/api/notes/{nid4}", jar="alice", extra_hdr=auth_hdr("alice"))

    print("\n[8] 更新笔记（含置顶切换）")
    if nid:
        s, d, _ = req("PUT", f"/api/notes/{nid}", jar="alice", extra_hdr=auth_hdr("alice"),
                      body={"title": "动态规划进阶", "content": "新的正文内容",
                            "tags": "dp,进阶", "pinned": True})
        check("PUT 更新 → 200", s == 200, f"status={s}")
        s, d, _ = req("GET", f"/api/notes/{nid}", jar="alice", extra_hdr=auth_hdr("alice"))
        data = d.get("data") or {}
        check("title 已更新", data.get("title") == "动态规划进阶", f"title={data.get('title')!r}")
        check("content 已更新", data.get("content") == "新的正文内容", f"content={data.get('content')!r}")
        check("tags 已更新", data.get("tags") == ["dp", "进阶"], f"tags={data.get('tags')}")
        check("pinned=true", data.get("pinned") is True, f"pinned={data.get('pinned')}")

    print("\n[9] 404 与跨用户越权")
    s, d, _ = req("GET", "/api/notes/999999", jar="alice", extra_hdr=auth_hdr("alice"))
    check("不存在的笔记 → 404", s == 404, f"status={s}")
    if nid:
        s, d, _ = req("GET", f"/api/notes/{nid}", jar="bob", extra_hdr=auth_hdr("bob"))
        check("bob 读 alice 的笔记 → 404", s == 404, f"status={s}")
        s, d, _ = req("PUT", f"/api/notes/{nid}", jar="bob", extra_hdr=auth_hdr("bob"),
                      body={"title": "hacked", "content": "x"})
        check("bob 改 alice 的笔记 → 404", s == 404, f"status={s}")
        s, d, _ = req("DELETE", f"/api/notes/{nid}", jar="bob", extra_hdr=auth_hdr("bob"))
        check("bob 删 alice 的笔记 → 404", s == 404, f"status={s}")
    s, d, _ = req("GET", "/api/notes/abc", jar="alice", extra_hdr=auth_hdr("alice"))
    check("pathID 非法 → 400", s == 400, f"status={s}")

    print("\n[10] 删除")
    if nid:
        s, d, _ = req("DELETE", f"/api/notes/{nid}", jar="alice", extra_hdr=auth_hdr("alice"))
        check("DELETE → 200", s == 200, f"status={s}")
        check("removed=true", (d.get("data") or {}).get("removed") is True, f"data={d}")
        s, d, _ = req("GET", f"/api/notes/{nid}", jar="alice", extra_hdr=auth_hdr("alice"))
        check("删除后读 → 404", s == 404, f"status={s}")
        s, d, _ = req("DELETE", f"/api/notes/{nid}", jar="alice", extra_hdr=auth_hdr("alice"))
        check("重复删除 → 404", s == 404, f"status={s}")

    print("\n[11] operation_logs 留痕")
    s, d, _ = req("POST", "/api/notes", jar="alice", extra_hdr=auth_hdr("alice"),
                  body={"title": "日志测试笔记", "content": "x"})
    if s == 200:
        log_nid = d["data"]["id"]
        con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
        cur = con.cursor()
        cur.execute("SELECT action FROM operation_logs WHERE detail=? ORDER BY id DESC LIMIT 1",
                    ("日志测试笔记",))
        row = cur.fetchone()
        check("note_create 落库", row is not None and row[0] == "note_create", f"row={row}")
        s, d, _ = req("PUT", f"/api/notes/{log_nid}", jar="alice", extra_hdr=auth_hdr("alice"),
                      body={"title": "日志测试笔记", "content": "改内容"})
        cur.execute("SELECT action FROM operation_logs WHERE detail=? ORDER BY id DESC LIMIT 1",
                    ("日志测试笔记",))
        row = cur.fetchone()
        check("note_update 落库", row is not None and row[0] == "note_update", f"row={row}")
        req("DELETE", f"/api/notes/{log_nid}", jar="alice", extra_hdr=auth_hdr("alice"))
        cur.execute("SELECT action FROM operation_logs WHERE detail=? ORDER BY id DESC LIMIT 1",
                    ("日志测试笔记",))
        row = cur.fetchone()
        check("note_delete 落库", row is not None and row[0] == "note_delete", f"row={row}")
        con.close()

    print("\n[12] 前端静态检查")
    notes_vue = read_file("views/Notes.vue")
    for kw in ["/api/notes", "api.post", "api.put", "api.del", "pinned", "tags", "snippet",
               "笔记", "删除"]:
        check(f"Notes.vue 含 {kw}", kw in notes_vue)
    router_js = read_file("router.js")
    check("router.js 注册 /notes", "/notes" in router_js and "Notes.vue" in router_js)
    app_vue = read_file("App.vue")
    check("App.vue 有笔记导航", "/notes" in app_vue)
    i18n_js = read_file("i18n.js")
    check("i18n.js 有中文 notes", "notes: '笔记'" in i18n_js)
    check("i18n.js 有英文 notes", "notes: 'Notes'" in i18n_js)

    print("\n[13] 后端静态检查")
    handler_src = read_repo_file("internal/app/handler_note.go")
    for kw in ["noteList", "noteDetail", "noteCreate", "noteUpdate", "noteDelete",
               "normalizeNote", "splitTags", "noteByID", "requireClaims",
               "note_create", "note_update", "note_delete"]:
        check(f"handler_note.go 含 {kw}", kw in handler_src)
    schema_src = read_repo_file("internal/app/schema_domain.go")
    check("schema 有 notes 表", "CREATE TABLE IF NOT EXISTS notes" in schema_src)
    check("schema 有 user_id 字段", "user_id INTEGER NOT NULL" in schema_src)
    check("schema 有 tags 字段", "tags TEXT DEFAULT" in schema_src)
    check("schema 有 pinned 字段", "pinned INTEGER DEFAULT" in schema_src)
    check("schema 有 idx_notes_user_time 索引", "idx_notes_user_time" in schema_src)
    router_src = read_repo_file("internal/app/router.go")
    for kw in ["GET /api/notes", "POST /api/notes", "GET /api/notes/{id}",
               "PUT /api/notes/{id}", "DELETE /api/notes/{id}"]:
        check(f"router.go 注册 {kw}", kw in router_src)


if __name__ == "__main__":
    run()
    print()
    print(f"通过 {len(passed)} / 失败 {len(failed)}")
    if failed:
        print(f"\n失败用例：")
        for f in failed:
            print(f"  - {f}")
    import sys
    if failed:
        sys.exit(1)
