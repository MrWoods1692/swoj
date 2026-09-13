#!/usr/bin/env python3
"""verify_terms.py — 用户协议与隐私政策端点验证。

覆盖分支：
  [1]  users.terms_accepted_at 列落地（新字段）
  [2]  GET /api/terms 匿名可访问：返回版本、accepted_at 为空
  [3]  POST /api/terms/accept 未登录 → 401
  [4]  mock OAuth 走完整 authorize→callback 链路建号
  [5]  登录后 GET /api/terms → need_accept 为 true
  [6]  /me 在同意前返回空 terms_accepted_at
  [7]  空版本 → 400
  [8]  错误版本 → 400
  [9]  缺 CSRF 令牌 → 403
  [10] 正确版本 → 200 并写入版本
  [11] /me 同步返回 terms_accepted_at
  [12] GET /api/terms → need_accept 变为 false
  [13] 重复确认幂等
  [14] operation_logs 留痕 terms_accept
  [15] 前端文案与路由静态检查

注意：authorize 的 mock 分支会 302 直接跳回回调并消费 state，
因此连发两次 authorize，用最后一次签发的 state/verifier cookie 手动回调取 JSON 令牌。
"""
import http.client
import json
import os
import re
import sqlite3
import sys
import urllib.parse

BASE_HOST = os.environ.get("SWOJ_HOST_PORT", "127.0.0.1:18080").split(":")
HOST, PORT = BASE_HOST[0], int(BASE_HOST[1])
DB = os.path.join(os.environ.get("SWOJ_DATA_DIR", "/tmp/swoj-terms-test"), "db", "swoj.db")
WEB = os.path.join(os.environ.get("SWOJ_WEB", "/home/mrcwoods/code/swoj/web/src"))
VERSION = "2026-09-3"

passed, failed = [], []
JARS = {}   # name -> {cookie_name: value}


def check(name, cond, detail=""):
    (passed if cond else failed).append(name)
    print(("  ok  " if cond else "  FAIL") + f" {name}" + (f"  — {detail}" if detail else ""))


def cookie_str(jar):
    return "; ".join(f"{k}={v}" for k, v in JARS.get(jar, {}).items())


def req(method, path, body=None, jar=None, extra_hdr=None, send_cookies=True):
    """发一次请求，返回 (status, json_dict, raw_body, response_headers)。
    不跟随 302，手动管理 cookie jar，保证 authorize 与 callback 的 state 传递可测。"""
    c = http.client.HTTPConnection(HOST, PORT, timeout=15)
    headers = {}
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    if extra_hdr:
        headers.update(extra_hdr)
    if send_cookies and jar and JARS.get(jar):
        headers["Cookie"] = cookie_str(jar)
    c.request(method, path, body=data, headers=headers)
    r = c.getresponse()
    raw = r.read().decode()
    # 收集 Set-Cookie
    if jar:
        JARS.setdefault(jar, {})
        for h, v in r.getheaders():
            if h.lower() == "set-cookie":
                for part in v.split(";"):
                    if "=" in part:
                        k, _, val = part.partition("=")
                        if k.strip() == "swoj_oauth_verifier" and not val.strip():
                            continue
                        JARS[jar][k.strip()] = val.strip()
    try:
        data_json = json.loads(raw)
    except Exception:
        data_json = {}
    c.close()
    return r.status, data_json, raw, dict(r.getheaders())


def qs(p):
    return p


print(f"[*] target http://{HOST}:{PORT}")

# [1] schema
print("[1] schema")
conn = sqlite3.connect(DB)
cols = {r[1] for r in conn.execute("PRAGMA table_info(users)")}
conn.close()
check("users.terms_accepted_at 列存在", "terms_accepted_at" in cols, f"cols={len(cols)} 个")

# [2] 匿名 GET /api/terms
print("[2] GET /api/terms (匿名)")
st, d, _, _ = req("GET", "/api/terms")
d = d["data"]
check("匿名返回 200", st == 200, f"status={st}")
check("返回协议版本", d.get("version") == VERSION, f"version={d.get('version')}")
check("匿名 accepted_at 为空", d.get("accepted_at") == "", f"accepted_at={d.get('accepted_at')!r}")
check("匿名 need_accept 为 true", d.get("need_accept") is True, f"data={d}")

# [3] 未登录 accept → 401
print("[3] accept 未登录")
st, d0, _, _ = req("GET", "/api/csrf", jar="anon")
csrf0 = d0["data"]["csrf"]
JARS["anon"]["swoj_csrf"] = csrf0
st, d, raw, _ = req("POST", "/api/terms/accept", {"version": VERSION}, jar="anon",
                    extra_hdr={"X-CSRF-Token": csrf0})
check("未登录 401", st == 401, f"status={st} body={raw[:90]}")

# [4] mock OAuth 建号
print("[4] mock OAuth 建号")
qa = "?" + urllib.parse.urlencode({"mock": "1", "mockname": "协议测试", "mockqq": "13800000001"})
st, _, _, _ = req("GET", "/auth/campux" + qa, jar="u")
check("authorize 首次跳转 302", st == 302, f"status={st}")
st, _, _, _ = req("GET", "/auth/campux" + qa, jar="u")
check("authorize 第二次跳转 302", st == 302, f"status={st}")
state_val = JARS["u"].get("swoj_oauth_state", "")
check("authorize 签发 state cookie", bool(state_val), f"state={state_val[:12]}")
cb = "?" + urllib.parse.urlencode({
    "mock": "1", "state": state_val, "mockname": "协议测试", "mockqq": "13800000001"})
st, d, _, _ = req("GET", "/auth/campux/callback" + cb, jar="u")
check("回调 200", st == 200, f"status={st}")
tok, csrf = d["data"]["token"], d["data"]["csrf"]
check("返回 token 与 csrf", bool(tok and csrf), f"tok={'y' if tok else 'n'} csrf={'y' if csrf else 'n'}")
u = d["data"].get("user", {})
check("新账号 terms_accepted_at 为空", not u.get("terms_accepted_at"), f"value={u.get('terms_accepted_at')!r}")
JARS["u"]["swoj_token"] = tok
JARS["u"]["swoj_csrf"] = csrf
AUTH = dict(extra_hdr={"X-CSRF-Token": csrf})

# [5] 登录后 GET /api/terms → need_accept true
print("[5] GET /api/terms (已登录未同意)")
st, d, _, _ = req("GET", "/api/terms", jar="u")
d = d["data"]
check("need_accept 为 true", d.get("need_accept") is True, f"data={d}")
check("accepted_at 仍为空", d.get("accepted_at") == "", "")

# [6] /me 同意前
print("[6] /me 同意前")
st, d, _, _ = req("GET", "/api/auth/me", jar="u")
check("200 且 terms_accepted_at 为空", st == 200 and not d["data"].get("terms_accepted_at"),
      f"terms={d['data'].get('terms_accepted_at')!r}")

# [7] 空版本 → 400
print("[7] accept 空版本")
st, _, raw, _ = req("POST", "/api/terms/accept", {"version": ""}, jar="u", **AUTH)
check("空版本 400", st == 400, f"status={st}")

# [8] 错误版本 → 400
print("[8] accept 错误版本")
st, _, raw, _ = req("POST", "/api/terms/accept", {"version": "1999-01"}, jar="u", **AUTH)
check("错误版本 400", st == 400, f"status={st} body={raw[:90]}")

# [9] 缺 CSRF → 403
print("[9] accept 缺 CSRF")
st, _, raw, _ = req("POST", "/api/terms/accept", {"version": VERSION}, jar="u")
check("缺 CSRF 403", st == 403, f"status={st} body={raw[:90]}")

# [10] 正确版本 → 200
print("[10] accept 正确版本")
st, d, _, _ = req("POST", "/api/terms/accept", {"version": VERSION}, jar="u", **AUTH)
check("200", st == 200, f"status={st}")
check("写入版本号", d["data"].get("terms_accepted_at") == VERSION, f"data={d['data']}")
check("返回 version", d["data"].get("version") == VERSION, "")

# [11] /me 同步
print("[11] /me 同步")
st, d, _, _ = req("GET", "/api/auth/me", jar="u")
check("/me terms_accepted_at 已写入", d["data"].get("terms_accepted_at") == VERSION,
      f"value={d['data'].get('terms_accepted_at')!r}")

# [12] GET /api/terms → need_accept false
print("[12] GET /api/terms (已同意)")
st, d, _, _ = req("GET", "/api/terms", jar="u")
d = d["data"]
check("need_accept 变为 false", d.get("need_accept") is False, f"data={d}")
check("accepted_at 已填充", d.get("accepted_at") == VERSION, f"accepted_at={d.get('accepted_at')!r}")

# [13] 幂等
print("[13] 幂等")
st, _, _, _ = req("POST", "/api/terms/accept", {"version": VERSION}, jar="u", **AUTH)
check("重复确认仍 200", st == 200, f"status={st}")

# [14] 操作日志
print("[14] 操作日志")
conn = sqlite3.connect(DB)
try:
    (n,) = conn.execute("SELECT COUNT(*) FROM operation_logs WHERE action='terms_accept'").fetchone()
except sqlite3.Error as e:
    print(f"  (表缺失: {e})")
    n = 0
conn.close()
check("terms_accept 已留痕", n >= 1, f"count={n}")

# [15] 前端静态检查
print("[15] 前端资源")
legals = open(os.path.join(WEB, "legals.js"), encoding="utf-8").read()
gate = open(os.path.join(WEB, "components", "TermsGate.vue"), encoding="utf-8").read()
check("legals 导出当前版本", f"TERMS_VERSION = '{VERSION}'" in legals, "")
check("协议不再把 QQ 号列为自填项", "个人简介。QQ 号由校园墙授权自动写入" in legals, "")
check("协议声明真实姓名仅可填写一次", "真实姓名仅可在首次登录时填写一次" in legals, "")
check("门禁提示不再说真实姓名可自清", "真实姓名仅可填写一次" in gate and "可自行清空。" not in gate, "")
check("设置页无 QQ 输入项", 'v-model="form.qq"' not in open(os.path.join(WEB, "views", "Settings.vue"), encoding="utf-8").read(), "")
# 防回归：后端只注册了 /api/admin/stats，路径写反会命中接口不存在的 404。
admin_vue = open(os.path.join(WEB, "views", "Admin.vue"), encoding="utf-8").read()
check("后台统计路径为 /api/admin/stats", "/api/admin/stats" in admin_vue and "/api/stats/admin" not in admin_vue, "")
check("协议与隐私章节齐全", legals.count("'title'") + legals.count("title:") >= 20, "")
check("导出 TERMS_DOC 与 PRIVACY_DOC", "TERMS_DOC" in legals and "PRIVACY_DOC" in legals, "")
router = open(os.path.join(WEB, "router.js"), encoding="utf-8").read()
check("router 含 /terms 与 /privacy", "/terms" in router and "/privacy" in router, "")
app = open(os.path.join(WEB, "App.vue"), encoding="utf-8").read()
check("App 挂载 TermsGate", "TermsGate" in app, "")
check("页脚含协议与隐私入口", "/terms" in app and "/privacy" in app, "")
check("门禁对比 TERMS_VERSION", "TERMS_VERSION" in gate, "")
i18n_txt = open(os.path.join(WEB, "i18n.js"), encoding="utf-8").read()
check("i18n 含协议与隐私词条", "terms:" in i18n_txt and "privacy:" in i18n_txt, "")

print(f"\n=== {len(passed)} passed, {len(failed)} failed ===")
if failed:
    print("FAILED: " + ", ".join(failed))
    sys.exit(1)
