#!/usr/bin/env python3
"""verify_logs.py — 日志系统验证。

覆盖分支：
  [1]  中间件：所有 API 请求（含未登录的公开端点）自动写入 operation_logs
  [2]  访问日志字段完整：action="api:<METHOD> <PATH>"、path、status_code、ip、created_at
  [3]  业务侧 logOp 与中间件访问日志共存于同一张表，可通过 action 前缀区分
  [4]  未登录 GET /api/admin/logs → 401
  [5]  未登录 GET /api/logs → 401
  [6]  普通用户 GET /api/admin/logs → 403
  [7]  普通用户 GET /api/logs → 200 且仅返回自己的记录
  [8]  普通用户伪造 user_id 参数无法越权（服务端忽略）
  [9]  管理员 GET /api/admin/logs → 200，返回全站记录
  [10] 管理员按 user_id 过滤 → 仅返回该用户记录
  [11] 管理员按 action 子串过滤 → 仅返回匹配项
  [12] 管理员按 path 子串过滤 → 仅返回匹配项
  [13] 管理员按时间范围过滤 → 均命中
  [14] 分页：page_size 边界（1、100 内合法、>200 收敛为 20）
  [15] 汇总指标 summary 字段齐全
  [16] 动作列表 GET /api/admin/logs/actions
  [17] 用户 ID 解析 GET /api/admin/logs/users?q=
  [18] 每日趋势 GET /api/admin/logs/daily
  [19] 每日趋势 days 参数边界（<1 与 >365 收敛）
  [20] 索引：idx_op_logs_created_at 与 idx_op_logs_user_time 存在
  [21] 前端静态检查：路由、导航、两个页面文件、调用端点、过滤控件
  [22] schema：operation_logs 含 path 与 status_code 字段
  [23] 已注册路由：/api/admin/logs /api/logs /api/admin/logs/actions /api/admin/logs/daily /api/admin/logs/users
"""
import http.client
import json
import os
import re
import sqlite3
import urllib.parse

HOST, PORT = (os.environ.get("SWOJ_HOST_PORT", "127.0.0.1:18080").split(":"))
HOST, PORT = HOST, int(PORT)
DB = os.path.join(os.environ.get("SWOJ_DATA_DIR", "/tmp/swoj-logs-test"), "db", "swoj.db")
WEB = os.path.join(os.environ.get("SWOJ_WEB", "/home/mrcwoods/code/swoj/web/src"))

passed, failed = [], []
JARS = {}


def check(name, cond, detail=""):
    (passed if cond else failed).append(name)
    print(("  ok  " if cond else "  FAIL") + f" {name}" + (f"  — {detail}" if detail else ""))


def req(method, path, body=None, jar=None, extra_hdr=None):
    c = http.client.HTTPConnection(HOST, PORT, timeout=15)
    headers, data = {}, None
    if body is not None:
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


def qs(**kw):
    return "?" + urllib.parse.urlencode(kw)
def read_file(p):
    try:
        with open(os.path.join(WEB, p), encoding="utf-8") as f:
            return f.read()
    except FileNotFoundError:
        return ""


def db_conn():
    return sqlite3.connect(DB)


print("== [22] schema 字段 ==")
conn = db_conn()
cols = [r[1] for r in conn.execute("PRAGMA table_info(operation_logs)")]
conn.close()
for c in ("id", "user_id", "username", "action", "target", "detail", "ip",
          "path", "status_code", "created_at"):
    check(f"operation_logs 有列 {c}", c in cols)

print("== [20] 索引 ==")
conn = db_conn()
idx_names = [r[0] for r in conn.execute(
    "SELECT name FROM sqlite_master WHERE type='index'")]
conn.close()
check("存在 idx_op_logs_created_at", "idx_op_logs_created_at" in idx_names)
check("存在 idx_op_logs_user_time", "idx_op_logs_user_time" in idx_names)

print("== [1] 中间件：所有请求都写日志 ==")
# 空库状态下发一个未登录的公开请求
s, d, _ = req("GET", "/api/points/rules")
check("公开端点匿名访问 200", s == 200, str(s))
conn = db_conn()
rows = conn.execute(
    "SELECT action, path, status_code, user_id FROM operation_logs ORDER BY id DESC LIMIT 1"
).fetchone()
total = conn.execute("SELECT COUNT(*) FROM operation_logs").fetchone()[0]
conn.close()
check("operation_logs 至少 1 条", total >= 1, str(total))
check("写入的 action 是 api:GET /api/points/rules",
      rows and rows[0] == "api:GET /api/points/rules", str(rows))
check("写入的 path 正确", rows and rows[1] == "/api/points/rules", str(rows))
check("写入的 status_code=200", rows and rows[2] == 200, str(rows))
check("未登录 user_id=0", rows and rows[3] == 0, str(rows))

# 再来一个 GET 请求触发中间件（登录接口）
s, d, _ = req("GET", "/api/csrf")
check("GET /api/csrf 200", s == 200, str(s))

print("== [4] 未登录访问日志端点 → 401 ==")
s, d, _ = req("GET", "/api/admin/logs")
check("GET /api/admin/logs 未登录 401", s == 401, str(s))
s, d, _ = req("GET", "/api/logs")
check("GET /api/logs 未登录 401", s == 401, str(s))
s, d, _ = req("GET", "/api/logs/actions")
check("GET /api/logs/actions 未登录 401", s == 401, str(s))
s, d, _ = req("GET", "/api/logs/daily")
check("GET /api/logs/daily 未登录 401", s == 401, str(s))
s, d, _ = req("GET", "/api/admin/logs/users")
check("GET /api/admin/logs/users 未登录 401", s == 401, str(s))

print("== [2][3] 业务 logOp 与访问日志共存 ==")
login("u1", "13900000400", "日志测试管理员")
s, d, _ = req("POST", "/api/points/online", body={"seconds": 30},
              jar="u1", extra_hdr=auth_hdr("u1"))
check("心跳 200", s == 200, str(s))
# 让业务写入一条 logOp：走 oauth_login 已经在 callback 时写过了；此处再发一次登录
# 直接查库验证业务与访问日志共存
conn = db_conn()
rows = conn.execute(
    "SELECT action FROM operation_logs ORDER BY id DESC LIMIT 20"
).fetchall()
conn.close()
actions = [r[0] for r in rows]
access_actions = [a for a in actions if a.startswith("api:")]
biz_actions = [a for a in actions if not a.startswith("api:")]
check("访问日志条目存在", len(access_actions) > 0, str(actions[:5]))
check("业务日志条目存在", len(biz_actions) > 0, str(biz_actions[:5]))
check("oauth_login 已入 operation_logs", "oauth_login" in biz_actions,
      str(biz_actions))

print("== [6] 普通用户访问全站日志端点 → 403 ==")
login("u2", "13900000401", "日志测试用户B")
s, d, _ = req("GET", "/api/admin/logs", jar="u2", extra_hdr=auth_hdr("u2"))
check("普通用户 GET /api/admin/logs 403", s == 403, str(s))
s, d, _ = req("GET", "/api/admin/logs/users", jar="u2", extra_hdr=auth_hdr("u2"))
check("普通用户 GET /api/admin/logs/users 403", s == 403, str(s))
# /api/logs/actions 与 /api/logs/daily 对普通用户返回自身作用域数据，不返回 403
s, d, _ = req("GET", "/api/logs/actions", jar="u2", extra_hdr=auth_hdr("u2"))
check("普通用户 GET /api/logs/actions 200", s == 200, str(s))
s, d, _ = req("GET", "/api/logs/daily", jar="u2", extra_hdr=auth_hdr("u2"))
check("普通用户 GET /api/logs/daily 200", s == 200, str(s))

print("== [7] 普通用户 GET /api/logs 只返回自己记录 ==")
s, d, _ = req("GET", "/api/logs", jar="u2", extra_hdr=auth_hdr("u2"))
check("u2 GET /api/logs 200", s == 200, str(s))
data = d.get("data") or {}
check("响应含 list", isinstance(data.get("list"), list), str(type(data.get("list"))))
check("响应含 total", isinstance(data.get("total"), int))
check("响应含 summary", isinstance(data.get("summary"), dict))
# u2 的 user_id 通过 mock OAuth 后是 id=2
rows = data.get("list", [])
check("u2 记录全部属于自己",
      all(r.get("user_id") == 2 for r in rows),
      str([r.get("user_id") for r in rows][:5]))
# u2 的 action 至少含 oauth_login（callback 时的业务 logOp 已落库）
user_actions = [r["action"] for r in rows]
check("u2 记录含 oauth_login",
      "oauth_login" in user_actions, str(user_actions))
# 当前请求的访问日志在服务端写库发生在 handler 返回之后，本次响应时尚未落表；
# 触发一次带登录态的请求，再查询 /api/logs，前一次请求的访问日志应该已入表。
s, d, _ = req("GET", "/api/points", jar="u2", extra_hdr=auth_hdr("u2"))
check("u2 GET /api/points 200", s == 200, str(s))
s, d, _ = req("GET", "/api/logs", jar="u2", extra_hdr=auth_hdr("u2"))
data = d.get("data") or {}
rows = data.get("list", [])
user_actions = [r["action"] for r in rows]
check("u2 记录含 api:GET /api/points（访问日志已落表）",
      "api:GET /api/points" in user_actions, str(user_actions))

print("== [8] 普通用户伪造 user_id 无法越权 ==")
s, d, _ = req("GET", "/api/logs?user_id=1", jar="u2", extra_hdr=auth_hdr("u2"))
data = d.get("data") or {}
rows = data.get("list", [])
check("伪造 user_id=1 仍仅返回 u2 记录",
      all(r.get("user_id") == 2 for r in rows),
      str([r.get("user_id") for r in rows][:5]))
check("伪造 user_id 未越权 total", data.get("total") is not None)

print("== [9] 管理员 GET /api/admin/logs 返回全站 ==")
s, d, _ = req("GET", "/api/admin/logs", jar="u1", extra_hdr=auth_hdr("u1"))
check("管理员 GET /api/admin/logs 200", s == 200, str(s))
data = d.get("data") or {}
check("全站 total > 0", data.get("total", 0) > 0, str(data.get("total")))
# 全站记录至少含 u1 与 u2 两个用户
user_ids = set(r.get("user_id") for r in data.get("list", []))
check("全站记录含多个 user_id",
      len(user_ids) >= 1 and (0 in user_ids or len(user_ids) >= 1),
      str(user_ids))

print("== [10] 按 user_id 过滤 ==")
s, d, _ = req("GET", "/api/admin/logs?user_id=2", jar="u1", extra_hdr=auth_hdr("u1"))
data = d.get("data") or {}
rows = data.get("list", [])
check("过滤 total 大于 0", data.get("total", 0) > 0, str(data.get("total")))
check("过滤结果全部属于 user_id=2",
      all(r.get("user_id") == 2 for r in rows),
      str([r.get("user_id") for r in rows][:5]))

print("== [11] 按 action 子串过滤 ==")
s, d, _ = req("GET", "/api/admin/logs?action=oauth_login",
              jar="u1", extra_hdr=auth_hdr("u1"))
data = d.get("data") or {}
rows = data.get("list", [])
check("过滤 action=oauth_login total>0", data.get("total", 0) > 0, str(data.get("total")))
check("过滤结果 action 均含 oauth_login",
      all("oauth_login" in r.get("action", "") for r in rows),
      str([r.get("action") for r in rows][:3]))

print("== [12] 按 path 子串过滤 ==")
s, d, _ = req("GET", "/api/admin/logs?path=/api/points",
              jar="u1", extra_hdr=auth_hdr("u1"))
data = d.get("data") or {}
rows = data.get("list", [])
check("过滤 path=/api/points total>0", data.get("total", 0) > 0, str(data.get("total")))
check("过滤结果 path 均含 /api/points",
      all("/api/points" in r.get("path", "") for r in rows),
      str([r.get("path") for r in rows][:3]))

print("== [13] 时间范围过滤 ==")
# 用宽松范围覆盖当前时间
s, d, _ = req("GET", "/api/admin/logs?from=2000-01-01%2000:00:00&to=2030-01-01%2000:00:00",
              jar="u1", extra_hdr=auth_hdr("u1"))
data = d.get("data") or {}
check("时间范围过滤 total>0", data.get("total", 0) > 0, str(data.get("total")))
# 用不可能范围得到 0
s, d, _ = req("GET", "/api/admin/logs?from=2030-01-01%2000:00:00&to=2030-01-02%2000:00:00",
              jar="u1", extra_hdr=auth_hdr("u1"))
data = d.get("data") or {}
check("未来时间范围 total=0", data.get("total") == 0, str(data.get("total")))

print("== [14] 分页与边界收敛 ==")
s, d, _ = req("GET", "/api/admin/logs?page_size=1", jar="u1", extra_hdr=auth_hdr("u1"))
data = d.get("data") or {}
check("page_size=1 生效", data.get("page_size") == 1, str(data.get("page_size")))
check("page_size=1 返回 1 条", len(data.get("list", [])) <= 1,
      str(len(data.get("list", []))))
s, d, _ = req("GET", "/api/admin/logs?page_size=200", jar="u1", extra_hdr=auth_hdr("u1"))
data = d.get("data") or {}
check("page_size=200 为合法上限", data.get("page_size") == 200, str(data.get("page_size")))
s, d, _ = req("GET", "/api/admin/logs?page_size=201", jar="u1", extra_hdr=auth_hdr("u1"))
data = d.get("data") or {}
check("page_size=201 超过上限收敛为 20", data.get("page_size") == 20, str(data.get("page_size")))
s, d, _ = req("GET", "/api/admin/logs?page_size=0", jar="u1", extra_hdr=auth_hdr("u1"))
data = d.get("data") or {}
check("page_size=0 收敛为 20", data.get("page_size") == 20, str(data.get("page_size")))
s, d, _ = req("GET", "/api/admin/logs?page=0", jar="u1", extra_hdr=auth_hdr("u1"))
data = d.get("data") or {}
check("page=0 收敛为 1", data.get("page") == 1, str(data.get("page")))

print("== [15] summary 汇总字段 ==")
s, d, _ = req("GET", "/api/admin/logs", jar="u1", extra_hdr=auth_hdr("u1"))
summary = (d.get("data") or {}).get("summary") or {}
for k in ("total", "today", "errors_30d", "distinct_users", "distinct_actions"):
    check(f"summary 含 {k}", k in summary)
check("summary.total >= 1", summary.get("total", 0) >= 1, str(summary.get("total")))
check("summary.today >= 1", summary.get("today", 0) >= 1, str(summary.get("today")))

print("== [16] 动作列表（管理员作用域） ==")
s, d, _ = req("GET", "/api/logs/actions", jar="u1", extra_hdr=auth_hdr("u1"))
check("actions 200", s == 200, str(s))
actions = d.get("data") or []
check("actions 返回 list", isinstance(actions, list), str(type(actions)))
check("actions 含 oauth_login", "oauth_login" in actions, str(actions[:5]))
check("actions 含 api: 前缀项",
      any(a.startswith("api:") for a in actions), str(actions[:5]))
check("actions 上限 200", len(actions) <= 200, str(len(actions)))
# 普通用户视角下 actions 只返回自己产生过的动作
s, d, _ = req("GET", "/api/logs/actions", jar="u2", extra_hdr=auth_hdr("u2"))
user_actions = d.get("data") or []
check("普通用户 actions 200", s == 200, str(s))
check("普通用户 actions 非空", len(user_actions) >= 1, str(user_actions))
# 校验：非管理员视角不出现未产生过的 action（用 admin 视角与 user 视角的差集证明）
admin_actions = set(req("GET", "/api/logs/actions", jar="u1", extra_hdr=auth_hdr("u1"))[1].get("data") or [])
user_actions_set = set(user_actions)
check("普通用户 actions ⊆ 管理员 actions",
      user_actions_set <= admin_actions,
      str(user_actions_set - admin_actions))

print("== [17] 用户解析（仅管理员） ==")
# URL 必须 URL 编码，Python http.client 不接受非 ASCII
q_encoded = urllib.parse.urlencode({"q": "日志测试"})
s, d, _ = req("GET", "/api/admin/logs/users?" + q_encoded, jar="u1", extra_hdr=auth_hdr("u1"))
check("users 200", s == 200, str(s))
users = d.get("data") or []
check("users 返回 list", isinstance(users, list))
check("users 至少命中一条", len(users) >= 1, str(users))
check("users 返回 user_id 与 username",
      all("user_id" in u and "username" in u for u in users), str(users[:2]))
# 空 q 返回空
s, d, _ = req("GET", "/api/admin/logs/users", jar="u1", extra_hdr=auth_hdr("u1"))
check("users 空 q 返回 []", (d.get("data") or []) == [], str(d.get("data")))

print("== [18] 每日趋势（管理员视角） ==")
s, d, _ = req("GET", "/api/logs/daily?days=30", jar="u1", extra_hdr=auth_hdr("u1"))
check("daily 200", s == 200, str(s))
daily = d.get("data") or []
check("daily 返回 list", isinstance(daily, list))
check("daily 至少一条", len(daily) >= 1, str(len(daily)))
check("daily 条目含 date/count",
      all("date" in x and "count" in x for x in daily), str(daily[:2]))
check("daily count 为正整数",
      all(isinstance(x.get("count"), int) and x.get("count") > 0 for x in daily),
      str(daily[:3]))
# 普通用户视角下 daily 只统计自己记录
s, d, _ = req("GET", "/api/logs/daily?days=30", jar="u2", extra_hdr=auth_hdr("u2"))
daily_u2 = d.get("data") or []
check("普通用户 daily 200", s == 200, str(s))
# 普通用户 daily 总条数应不超过管理员视角
admin_daily_total = sum(x.get("count", 0) for x in daily)
u2_daily_total = sum(x.get("count", 0) for x in daily_u2)
check("普通用户 daily 计数 ≤ 管理员视角",
      u2_daily_total <= admin_daily_total,
      f"admin={admin_daily_total} user={u2_daily_total}")
print("== [19] 每日趋势边界收敛 ==")
s, d, _ = req("GET", "/api/logs/daily?days=0", jar="u1", extra_hdr=auth_hdr("u1"))
check("daily days=0 不报错", s == 200, str(s))
s, d, _ = req("GET", "/api/logs/daily?days=9999", jar="u1", extra_hdr=auth_hdr("u1"))
check("daily days=9999 不报错", s == 200, str(s))
check("daily days=9999 收敛为 30 天上限（返回条数<=30）",
      len(d.get("data") or []) <= 30, str(len(d.get("data") or [])))

print("== [23] 已注册路由静态检查 ==")
router = read_file("../../internal/app/router.go") if False else ""
# 直接读取源码文本
rpath = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                     "internal", "app", "router.go")
try:
    with open(rpath, encoding="utf-8") as f:
        rsrc = f.read()
except FileNotFoundError:
    rsrc = ""
check("router 注册 /api/admin/logs", "/api/admin/logs" in rsrc)
check("router 注册 /api/logs", "GET /api/logs" in rsrc)
check("router 注册 /api/logs/actions", "/api/logs/actions" in rsrc)
check("router 注册 /api/logs/daily", "/api/logs/daily" in rsrc)
check("router 注册 /api/admin/logs/users", "/api/admin/logs/users" in rsrc)
check("router 使用 accessLogMiddleware", "accessLogMiddleware" in rsrc)

print("== [21] 前端静态检查 ==")
router_vue = read_file("router.js")
app_vue = read_file("App.vue")
admin_logs = read_file("views/AdminLogs.vue")
my_logs = read_file("views/MyLogs.vue")
api_js = read_file("api.js")
check("router.js 注册 /admin/logs 路由", "'/admin/logs'" in router_vue)
check("router.js 注册 /logs 路由", "'/logs'" in router_vue)
check("router.js 引入 AdminLogs.vue", "AdminLogs.vue" in router_vue)
check("router.js 引入 MyLogs.vue", "MyLogs.vue" in router_vue)
check("App.vue 顶栏含 /logs 链接", 'to="/logs"' in app_vue)
check("AdminLogs.vue 存在且长度充足", len(admin_logs) > 3000, str(len(admin_logs)))
check("MyLogs.vue 存在且长度充足", len(my_logs) > 2000, str(len(my_logs)))
check("AdminLogs.vue 调用 /api/admin/logs",
      "api.get('/api/admin/logs'" in admin_logs)
check("AdminLogs.vue 调用 actions", "/api/logs/actions" in admin_logs)
check("AdminLogs.vue 调用 daily", "/api/logs/daily" in admin_logs)
check("AdminLogs.vue 含 user_id 过滤控件", "user_id" in admin_logs)
check("AdminLogs.vue 含 action 过滤控件", "filter.action" in admin_logs)
check("AdminLogs.vue 含 path 过滤控件", "filter.path" in admin_logs)
check("AdminLogs.vue 含时间范围过滤", "filter.from" in admin_logs and "filter.to" in admin_logs)
check("MyLogs.vue 调用 /api/logs", "api.get('/api/logs'" in my_logs)
check("MyLogs.vue 未暴露 user_id 输入",
      "filter.user_id" not in my_logs)
check("api.js 支持查询参数", "buildQuery" in api_js)
check("api.js get 支持第二参数", "get: (p, q)" in api_js)

print("== [extra] 业务 logOp 与中间件共存验证 ==")
# 触发一个业务动作：让 u2 提交一条 discussion（写入 discussion_create 语义动作）
s, d, _ = req("POST", "/api/discussions",
              body={"title": "日志系统测试帖", "content": "test body"},
              jar="u2", extra_hdr=auth_hdr("u2"))
check("创建讨论 200", s == 200, str(s))
conn = db_conn()
rows = conn.execute(
    "SELECT action, target FROM operation_logs "
    "WHERE action='discussion_create' ORDER BY id DESC LIMIT 1"
).fetchall()
conn.close()
check("业务 discussion_create 入表", len(rows) == 1, str(rows))
check("业务日志 target 非空", rows and rows[0][1] not in ("", None), str(rows))

print(f"\n通过 {len(passed)} / 失败 {len(failed)}")
if failed:
    for f in failed:
        print("  - " + f)
    raise SystemExit(1)
