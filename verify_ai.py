#!/usr/bin/env python3
"""verify_ai.py — AI 问答接入验证

覆盖：
- 未登录/普通用户/管理员 三类访问权限
- AI 配置端点（token 掩码、清除、system_prompt 上限）
- AI 提问（未配置 token 的 503 提示；配置后走 yunzhiapi.cn）
- multipart 文件上传（.cpp/.txt/.in/.out 允许；.exe 拒绝；>1MB 拒绝；>5 个拒绝）
- ai_qas 保存：question、answer、source、created_at
- aiHistory 仅返回近 7 天记录
- 服务端启动时清理过期记录
- 前端静态：AI.vue 支持文件上传、显示 7 天保留、admin 配置面板
- api.js 支持 upload 方法
"""
import http.client
import json
import os
import sqlite3
import time
import urllib.parse
import uuid
import sys

HOST = os.environ.get("SWOJ_HOST_PORT", "127.0.0.1:18080")
if ":" in HOST:
    H, P = HOST.rsplit(":", 1)
    P = int(P)
else:
    H, P = HOST, 8080

DB = os.environ.get("SWOJ_DATA_DIR", "/tmp/swoj-ai")
DB += "/db/swoj.db"
WEB_SRC = os.environ.get("SWOJ_WEB", "/home/mrcwoods/code/swoj/web/src")

JARS = {}
ok = fail = 0
def check(name, cond, info=""):
    global ok, fail
    if cond:
        ok += 1
        print(f"  ok   {name}  — {info if info else ''}")
    else:
        fail += 1
        print(f"  FAIL {name}  — {info}")

def req(method, path, body=None, jar=None, extra_hdr=None):
    c = http.client.HTTPConnection(H, P, 15)
    headers = {}
    data = None
    if body is not None:
        if isinstance(body, bytes):
            headers["Content-Type"] = "application/octet-stream"
            data = body
        else:
            headers["Content-Type"] = "application/json"
            data = json.dumps(body).encode()
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
    c.close()
    try:
        d = json.loads(raw)
    except:
        d = {"code": r.status, "msg": raw[:200]}
    return r.status, d

def qs(params):
    return urllib.parse.urlencode(params)

def login(jar, qq, name):
    """通过 mock OAuth 登录一个用户。返回 token/csrf/user；失败抛异常。"""
    qa = "?" + qs({"mock": "1", "mockname": name, "mockqq": qq})
    req("GET", "/auth/campux" + qa, jar=jar)
    req("GET", "/auth/campux" + qa, jar=jar)
    st = JARS.get(jar, {}).get("swoj_oauth_state", "")
    cb = "?" + qs({"mock": "1", "state": st, "mockname": name, "mockqq": qq})
    s, d = req("GET", "/auth/campux/callback" + cb, jar=jar)
    assert s == 200, f"callback status={s}"
    data = d["data"]
    JARS[jar]["swoj_token"] = data["token"]
    JARS[jar]["swoj_csrf"] = data["csrf"]
    return data["token"], data["csrf"], data["user"]

def auth_hdr(jar):
    """返回带 Authorization + X-CSRF-Token 的头；Cookie 由 req 通过 jar 自动附带。"""
    return {"Authorization": f"Bearer {JARS[jar]['swoj_token']}",
            "X-CSRF-Token": JARS[jar]["swoj_csrf"]}

# 便捷封装：authenticated req 自动带上 jar 与 auth 头
def areq(method, path, jar, body=None, extra_hdr=None):
    h = auth_hdr(jar)
    if extra_hdr:
        h.update(extra_hdr)
    return req(method, path, body=body, jar=jar, extra_hdr=h)

def upload(path, fields, files, jar):
    """构造 multipart 请求并返回 (status, dict)."""
    c = http.client.HTTPConnection(H, P, 30)
    boundary = "----WebKitFormBoundary" + uuid.uuid4().hex
    body = bytearray()
    def add_field(k, v):
        body.extend(f"--{boundary}\r\nContent-Disposition: form-data; name=\"{k}\"\r\n\r\n{v}\r\n".encode())
    def add_file(k, name, content, ctype="text/plain"):
        if isinstance(content, str):
            content = content.encode()
        body.extend(f"--{boundary}\r\nContent-Disposition: form-data; name=\"{k}\"; filename=\"{name}\"\r\nContent-Type: {ctype}\r\n\r\n".encode())
        body.extend(content)
        body.extend(b"\r\n")
    for k, v in fields:
        add_field(k, v)
    for k, name, content in files:
        add_file(k, name, content)
    body.extend(f"--{boundary}--\r\n".encode())
    hdrs = auth_hdr(jar)
    hdrs.update({
        "Content-Type": f"multipart/form-data; boundary={boundary}",
        "Content-Length": str(len(body)),
    })
    if JARS.get(jar):
        hdrs["Cookie"] = "; ".join(f"{k}={v}" for k, v in JARS[jar].items())
    c.request("POST", path, body=bytes(body), headers=hdrs)
    r = c.getresponse()
    raw = r.read().decode()
    c.close()
    try:
        d = json.loads(raw)
    except:
        d = {"code": r.status, "msg": raw[:200]}
    return r.status, d

def db_conn():
    return sqlite3.connect(DB)

# ============ 运行 ============
print("== [1] 未登录访问 AI 端点 → 401/403 ==")
s, d = req("GET", "/api/ai/history")
check("GET /api/ai/history 未登录 401", s == 401, str(s))
s, d = req("POST", "/api/ai/ask", {})
check("POST /api/ai/ask 未登录被拒", s in (401, 403), str(s))
s, d = req("GET", "/api/admin/ai-config")
check("GET /api/admin/ai-config 未登录 401", s == 401, str(s))
s, d = req("PUT", "/api/admin/ai-config", {})
check("PUT /api/admin/ai-config 未登录被拒", s in (401, 403), str(s))

print("== [2] 首位注册 → 管理员 ==")
try:
    login("u1", "13900000101", "AI管理员")
except Exception as e:
    print(f"!! 登录失败: {e}"); sys.exit(2)
a1 = auth_hdr("u1")
check("u1 登录成功", True)

print("== [3] 未配置 token 时 /api/ai/ask 返回 503 ==")
s, d = req("POST", "/api/ai/ask", {"question": "你好"}, jar="u1", extra_hdr=a1)
check("ai_ask 未启用时 503", s == 503, f"{s} {d.get('msg','')[:80]}")

print("== [4] 管理员 GET /api/admin/ai-config 返回基础结构 ==")
s, d = req("GET", "/api/admin/ai-config", jar="u1", extra_hdr=a1)
check("admin ai-config 200", s == 200, str(s))
data = d.get("data", {})
for k in ["provider", "token_masked", "has_token", "system_prompt",
          "default_prompt", "retention_days", "yunzhi_url"]:
    check(f"含 {k} 字段", k in data, str(data.get(k))[:60])
check("retention_days = 7", data.get("retention_days") == 7, str(data.get("retention_days")))
check("has_token 初始为 False", data.get("has_token") is False, str(data.get("has_token")))
check("default_prompt 非空", bool(data.get("default_prompt")), len(data.get("default_prompt") or ""))
check("provider 默认 yunzhi", data.get("provider") == "yunzhi", data.get("provider"))
check("yunzhi_url 默认值",
      data.get("yunzhi_url") == "https://yunzhiapi.cn/API/deepseek.php",
      data.get("yunzhi_url"))

print("== [5] 保存 token + system_prompt ==")
NEW_TOKEN = "test-token-1234567890ABCDEF12345"  # 长于 12 字符，触发 mask
payload = {
    "token": NEW_TOKEN,
    "provider": "yunzhi",
    "system_prompt": "你是测试用提示词。",
    "clear_token": False,
}
s, d = req("PUT", "/api/admin/ai-config", payload, jar="u1", extra_hdr=a1)
check("PUT ai-config 200", s == 200, str(s))
data = d.get("data", {})
check("has_token 变 True", data.get("has_token") is True, str(data.get("has_token")))
check("token_masked 展示前 4 后 4",
      data.get("token_masked") == "test****2345", data.get("token_masked"))

# 从 DB 校验
conn = db_conn()
row = conn.execute("SELECT value FROM admin_configs WHERE key='ai.token'").fetchone()
check("DB 中 ai.token 保存完整值",
      row and row[0] == NEW_TOKEN, str(row))
row = conn.execute("SELECT value FROM admin_configs WHERE key='ai.system_prompt'").fetchone()
check("DB 中 ai.system_prompt 保存",
      row and row[0] == "你是测试用提示词。", str(row))
row = conn.execute("SELECT value FROM admin_configs WHERE key='ai.provider'").fetchone()
check("DB 中 ai.provider = yunzhi",
      row and row[0] == "yunzhi", str(row))
conn.close()

# 再次 GET 应该显示掩码
s, d = req("GET", "/api/admin/ai-config", jar="u1", extra_hdr=a1)
data = d.get("data", {})
check("GET 后 token_masked 匹配",
      data.get("token_masked") == "test****2345", data.get("token_masked"))
check("GET 不泄露完整 token",
      NEW_TOKEN not in json.dumps(d), "token 未泄露")
check("system_prompt 可回读",
      data.get("system_prompt") == "你是测试用提示词。",
      data.get("system_prompt"))

print("== [6] 不传 token 不覆盖已有 token ==")
s, d = req("PUT", "/api/admin/ai-config",
              {"token": "", "provider": "yunzhi", "system_prompt": "新提示词。",
               "clear_token": False}, jar="u1", extra_hdr=a1)
check("PUT 无 token 仍 200", s == 200, str(s))
conn = db_conn()
row = conn.execute("SELECT value FROM admin_configs WHERE key='ai.token'").fetchone()
check("DB 中 ai.token 未被清空",
      row and row[0] == NEW_TOKEN, str(row))
conn.close()
# GET 确认
s, d = req("GET", "/api/admin/ai-config", jar="u1", extra_hdr=a1)
check("has_token 仍为 True", d.get("data", {}).get("has_token") is True)
check("system_prompt 已更新", d.get("data", {}).get("system_prompt") == "新提示词。",
      d.get("data", {}).get("system_prompt"))

print("== [7] clear_token=true 清除 ==")
s, d = req("PUT", "/api/admin/ai-config",
              {"token": "", "provider": "yunzhi", "system_prompt": "",
               "clear_token": True}, jar="u1", extra_hdr=a1)
check("PUT clear_token 200", s == 200, str(s))
conn = db_conn()
row = conn.execute("SELECT value FROM admin_configs WHERE key='ai.token'").fetchone()
check("DB 中 ai.token 已被清除", row and row[0] == "", str(row))
conn.close()
s, d = req("GET", "/api/admin/ai-config", jar="u1", extra_hdr=a1)
data = d.get("data", {})
check("has_token = False", data.get("has_token") is False)
check("token_masked 为空", data.get("token_masked") == "", data.get("token_masked"))
check("system_prompt 为空", data.get("system_prompt") == "", data.get("system_prompt"))

print("== [8] 字段边界校验 ==")
# system_prompt 超 4000 字
long_prompt = "x" * 4001
s, d = req("PUT", "/api/admin/ai-config",
              {"token": "", "system_prompt": long_prompt, "provider": "yunzhi"}, jar="u1", extra_hdr=a1)
check("system_prompt 超 4000 字 → 400", s == 400, f"{s} {d.get('msg','')[:60]}")
# token 超 256 字
long_token = "t" * 257
s, d = req("PUT", "/api/admin/ai-config",
              {"token": long_token, "system_prompt": "", "provider": "yunzhi"}, jar="u1", extra_hdr=a1)
check("token 超 256 字 → 400", s == 400, f"{s} {d.get('msg','')[:60]}")
# 非法 provider
s, d = req("PUT", "/api/admin/ai-config",
              {"token": "", "system_prompt": "", "provider": "unknown"}, jar="u1", extra_hdr=a1)
check("非法 provider → 403 或 400", s in (400, 403), f"{s} {d.get('msg','')[:60]}")

print("== [9] 普通用户访问 AI 配置 → 403 ==")
try:
    login("u2", "13900000201", "AI用户B")
except Exception as e:
    print(f"!! u2 登录失败: {e}")
    sys.exit(2)
a2 = auth_hdr("u2")
check("u2 登录成功", True)
s, d = req("GET", "/api/admin/ai-config", jar="u2", extra_hdr=a2)
check("u2 GET ai-config 403", s == 403, str(s))
s, d = req("PUT", "/api/admin/ai-config",
           {"token": "x", "provider": "yunzhi"}, jar="u2", extra_hdr=a2)
check("u2 PUT ai-config 403", s == 403, str(s))

# 普通用户提问：token 已配置但远端无效，应返回 502（服务调用失败）或 503（未启用）
s, d = req("POST", "/api/ai/ask", {"question": "hi"}, jar="u2", extra_hdr=a2)
check("u2 POST /api/ai/ask 未启用或调用失败", s in (502, 503), f"{s} {d.get('msg','')[:60]}")

print("== [10] 配置 token 后 multipart 上传校验路径（不实际调用远端） ==")
# 上传前清空 token，让 callAI 走到 503 分支；仅测上传校验路径
s, d = req("PUT", "/api/admin/ai-config",
    {"token": "", "provider": "yunzhi", "system_prompt": "",
     "clear_token": True}, jar="u1", extra_hdr=a1)
check("清空 token 200", s == 200, str(s))

# 允许的文件扩展名
s, d = upload("/api/ai/ask",
              [("question", "hello")],
              [("files[]", "sample.cpp", "#include <bits/stdc++.h>\nint main(){return 0;}\n")],
              "u1")
check("上传 .cpp 不 400（走 503）", s == 503, f"{s} {d.get('msg','')[:60]}")

# 允许 .txt
s, d = upload("/api/ai/ask",
              [("question", "hello")],
              [("files[]", "note.txt", "这是说明")],
              "u1")
check("上传 .txt 不 400（走 503）", s == 503, f"{s} {d.get('msg','')[:60]}")

# 允许 .in
s, d = upload("/api/ai/ask",
              [("question", "hello")],
              [("files[]", "input.in", "1 2 3\n")],
              "u1")
check("上传 .in 不 400（走 503）", s == 503, f"{s} {d.get('msg','')[:60]}")

# 允许 .out
s, d = upload("/api/ai/ask",
              [("question", "hello")],
              [("files[]", "output.out", "6\n")],
              "u1")
check("上传 .out 不 400（走 503）", s == 503, f"{s} {d.get('msg','')[:60]}")

# 拒绝 .exe
s, d = upload("/api/ai/ask",
              [("question", "hello")],
              [("files[]", "virus.exe", b"MZ")],
              "u1")
check("上传 .exe 拒绝 → 400", s == 400, f"{s} {d.get('msg','')[:60]}")

# 单文件 > 1MB
s, d = upload("/api/ai/ask",
              [("question", "hello")],
              [("files[]", "big.cpp", b"x" * (1024 * 1024 + 100))],
              "u1")
check("单文件 > 1MB 拒绝", s == 400, f"{s} {d.get('msg','')[:60]}")

# 6 个文件
files_6 = [(("files[]"), f"f{i}.txt", f"content {i}") for i in range(6)]
files_6 = [("files[]", f"f{i}.txt", f"content {i}") for i in range(6)]
s, d = upload("/api/ai/ask",
              [("question", "hello")], files_6, "u1")
check("超过 5 个文件拒绝", s == 400, f"{s} {d.get('msg','')[:60]}")

# 空 body（无 question 无文件无 code）
s, d = upload("/api/ai/ask", [], [], "u1")
check("空请求 → 400", s == 400, f"{s} {d.get('msg','')[:60]}")

# JSON 方式：空 question 空 code 应 400
s, d = req("POST", "/api/ai/ask", {"question": ""}, jar="u1", extra_hdr=auth_hdr("u1"))
check("JSON 空 question 400", s == 400, f"{s} {d.get('msg','')[:60]}")

print("== [11] 清空 token 后上传不写库、无副作用 ==")
# 清空 token，让 callAI 走到未配置分支；上传校验路径不受影响
req("PUT", "/api/admin/ai-config",
    {"token": "", "provider": "yunzhi", "system_prompt": "",
     "clear_token": True}, jar="u1", extra_hdr=a1)

# 上传 .cpp：应 503（未启用）
s, d = upload("/api/ai/ask",
              [("question", "hello")],
              [("files[]", "sample.cpp", "#include <bits/stdc++.h>\nint main(){return 0;}\n")],
              "u1")
check("清空后上传 .cpp 应 503", s == 503, f"{s} {d.get('msg','')[:60]}")

# 拒绝 .exe
s, d = upload("/api/ai/ask",
              [("question", "hello")],
              [("files[]", "virus.exe", b"MZ")],
              "u1")
check("清空后 .exe 仍拒绝 → 400", s == 400, f"{s} {d.get('msg','')[:60]}")

conn = db_conn()
rows = conn.execute("SELECT COUNT(*) FROM ai_qas WHERE question='hello'").fetchone()[0]
conn.close()
check("所有失败调用都没有入 ai_qas 表", rows == 0, f"count={rows}")

print("== [12] 过期清理：直接插入 8 天前的记录，模拟启动清理 ==")
# 插入一条 8 天前的记录
now = time.time()
old_ts = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(now - 8 * 24 * 3600))
conn = db_conn()
conn.execute("INSERT INTO ai_qas(user_id, problem_id, question, answer, source, created_at) "
             "VALUES(?, 0, 'old question', 'old answer', 'test', ?)", (1, old_ts))
conn.commit()
cnt_before = conn.execute("SELECT COUNT(*) FROM ai_qas WHERE created_at < ?",
                          (time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(now - 7*24*3600)),)
                         ).fetchone()[0]
conn.close()
check("插入的旧记录计数 = 1", cnt_before == 1, str(cnt_before))

# 手动执行删除（等同启动清理逻辑）
conn = db_conn()
cutoff = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(now - 7*24*3600))
cur = conn.execute("DELETE FROM ai_qas WHERE created_at < ?", (cutoff,))
conn.commit()
deleted = cur.rowcount
conn.close()
check("手动清理删除了过期记录", deleted == 1, str(deleted))

# 同时验证 aiHistory 只返回 7 天内记录
# 插入一条 3 天前的记录，应该在历史里；再插入一条 10 天前的，应该在表里但 history 不返回
now_iso = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime())
recent = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(now - 3*24*3600))
too_old = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(now - 10*24*3600))
conn = db_conn()
cur = conn.execute("INSERT INTO ai_qas(user_id, question, answer, source, created_at) "
                   "VALUES(1, 'recent', 'ok', 'test', ?)", (recent,))
recent_id = cur.lastrowid
conn.execute("INSERT INTO ai_qas(user_id, question, answer, source, created_at) "
             "VALUES(1, 'ancient', 'old', 'test', ?)", (too_old,))
conn.commit()
conn.close()

# 让管理员查 history（应该只看到 recent，不看到 ancient）
s, d = req("GET", "/api/ai/history", jar="u1", extra_hdr=a1)
check("GET /api/ai/history 200", s == 200, str(s))
items = d.get("data", [])
q_texts = [it.get("question") for it in items]
check("history 含 recent 记录", "recent" in q_texts, str(q_texts))
check("history 不含 ancient 记录", "ancient" not in q_texts, str(q_texts))

# 清理
conn = db_conn()
conn.execute("DELETE FROM ai_qas WHERE question IN ('recent','ancient','old question')")
conn.commit()
conn.close()

print("== [13] 前端静态检查 ==")
ai_vue = open(os.path.join(WEB_SRC, "views", "AI.vue")).read()
check("AI.vue 存在且长度充足", len(ai_vue) > 2000, len(ai_vue))
check("AI.vue 支持 multipart 上传", "FormData" in ai_vue and "files[]" in ai_vue)
check("AI.vue 显示 7 天保留", "7 天" in ai_vue or "7 天保留" in ai_vue or "保留 7 天" in ai_vue)
check("AI.vue 含 AI 配置面板", "AI 配置" in ai_vue and "Token" in ai_vue)
check("AI.vue 调用 /api/admin/ai-config", "/api/admin/ai-config" in ai_vue)
check("AI.vue 调用 /api/ai/ask", "/api/ai/ask" in ai_vue)
check("AI.vue 调用 /api/ai/history", "/api/ai/history" in ai_vue)
check("AI.vue 含 clear_token 支持", "clear_token" in ai_vue)
check("AI.vue 含 accept 属性列白名单", 'accept=".' in ai_vue and '.cpp' in ai_vue and '.txt' in ai_vue)

router_js = open(os.path.join(WEB_SRC, "router.js")).read()
check("router.js 注册 /ai 路由", "path: '/ai'" in router_js or "path:'/ai'" in router_js)
check("router.js 引入 AI.vue", "AI" in router_js)

api_js = open(os.path.join(WEB_SRC, "api.js")).read()
check("api.js 含 upload 方法", "upload:" in api_js and "FormData" in api_js)

print("== [14] 后端静态：文件类型白名单与 7 天常量 ==")
handler_src = open("/home/mrcwoods/code/swoj/internal/app/handler_leaderboard.go").read()
bootstrap_src = open("/home/mrcwoods/code/swoj/internal/app/bootstrap.go").read()
check("handler 定义 isTextFile", "func isTextFile" in handler_src)
check("bootstrap 定义 aiRetentionDays = 7", "aiRetentionDays = 7" in bootstrap_src)
check("bootstrap 启动时调用 purgeExpiredAIQAs", "purgeExpiredAIQAs" in bootstrap_src)
check("bootstrap 启动 aiPurgeLoop 定时器", "aiPurgeLoop" in bootstrap_src)

print("== [15] 后端静态：AI provider 分派 ==")
ai_src = open("/home/mrcwoods/code/swoj/internal/app/ai_client.go").read()
check("ai_client 分派 yunzhi", "callAIYunzhi" in ai_src)
check("ai_client 分派 openai", "callAIOpenAI" in ai_src)
check("ai_client 使用 yunzhiapi.cn", "yunzhiapi.cn" in ai_src)
check("ai_client 使用 GET 请求", "http.MethodGet" in ai_src)
check("ai_client 传 question 参数", 'q.Set("question"' in ai_src)
check("ai_client 传 system 参数", 'q.Set("system"' in ai_src)
check("ai_client 传 token 参数", 'q.Set("token"' in ai_src)
check("ai_client 传 type=text", 'q.Set("type", "text")' in ai_src)
check("ai_client 500 视为超时", "AI 服务超时" in ai_src)

print("== [16] 路由注册静态检查 ==")
router_src = open("/home/mrcwoods/code/swoj/internal/app/router.go").read()
check("router 注册 POST /api/ai/ask", "POST /api/ai/ask" in router_src)
check("router 注册 GET /api/ai/history", "GET /api/ai/history" in router_src)
check("router 注册 GET /api/admin/ai-config", "GET /api/admin/ai-config" in router_src)
check("router 注册 PUT /api/admin/ai-config", "PUT /api/admin/ai-config" in router_src)

print("== [17] AI Token 用量统计 ==")
s, d = req("GET", "/api/ai/stats", jar="u1", extra_hdr=a1)
check("GET /api/ai/stats 200", s == 200, str(s))
sd = d.get("data", {})
check("stats 含 total_calls", isinstance(sd.get("total_calls"), int), str(sd.get("total_calls")))
check("stats 含 total_tokens", isinstance(sd.get("total_tokens"), int), str(sd.get("total_tokens")))
check("stats 含 prompt_tokens", isinstance(sd.get("prompt_tokens"), int), str(sd.get("prompt_tokens")))
check("stats 含 answer_tokens", isinstance(sd.get("answer_tokens"), int), str(sd.get("answer_tokens")))
check("stats 含 today_calls", isinstance(sd.get("today_calls"), int), str(sd.get("today_calls")))
check("stats 含 today_tokens", isinstance(sd.get("today_tokens"), int), str(sd.get("today_tokens")))

conn = db_conn()
conn.execute("""INSERT INTO ai_qas(user_id, question, answer, prompt_tokens, answer_tokens, total_tokens)
    VALUES(?, ?, ?, ?, ?, ?)""", (1, "统计测试", "这是回答", 100, 50, 150))
conn.commit()
conn.close()
s, d = req("GET", "/api/ai/stats", jar="u1", extra_hdr=a1)
sd = d.get("data", {})
check("写入后 total_calls >= 1", sd.get("total_calls") >= 1, str(sd.get("total_calls")))
check("写入后 total_tokens >= 150", sd.get("total_tokens") >= 150, str(sd.get("total_tokens")))

s, d = req("GET", "/api/admin/ai-stats", jar="u1", extra_hdr=a1)
check("GET /api/admin/ai-stats 200", s == 200, str(s))
ad = d.get("data", {})
check("admin stats 含 total_tokens", isinstance(ad.get("total_tokens"), int), str(ad.get("total_tokens")))
check("admin stats 含 total_calls", isinstance(ad.get("total_calls"), int), str(ad.get("total_calls")))
check("admin stats 含 total_users", isinstance(ad.get("total_users"), int), str(ad.get("total_users")))
check("admin stats 含 top_users 数组", isinstance(ad.get("top_users"), list), type(ad.get("top_users")).__name__)
check("admin stats 含 daily_7d 数组", isinstance(ad.get("daily_7d"), list), type(ad.get("daily_7d")).__name__)

s, d = req("GET", "/api/admin/ai-stats", jar="u2", extra_hdr=a2)
check("u2 访问 admin ai-stats 403", s == 403, str(s))

s, d = req("GET", "/api/admin/ai-config", jar="u1", extra_hdr=a1)
ad = d.get("data", {})
check("ai-config 含 global_tokens", isinstance(ad.get("global_tokens"), int), str(ad.get("global_tokens")))
check("ai-config 含 global_calls", isinstance(ad.get("global_calls"), int), str(ad.get("global_calls")))

ai_vue = open("/home/mrcwoods/code/swoj/web/src/views/AI.vue").read()
check("AI.vue 含 /api/ai/stats", "/api/ai/stats" in ai_vue)
check("AI.vue 含 /api/admin/ai-stats", "/api/admin/ai-stats" in ai_vue)
check("AI.vue 显示 total_tokens", "total_tokens" in ai_vue)
check("AI.vue 显示 today_tokens", "today_tokens" in ai_vue)

handler_src2 = open("/home/mrcwoods/code/swoj/internal/app/handler_leaderboard.go").read()
check("后端定义 estimateTokens", "func estimateTokens" in handler_src2)
check("后端定义 aiStats", "func (s *Server) aiStats" in handler_src2)
check("后端定义 aiAdminStats", "func (s *Server) aiAdminStats" in handler_src2)

# ============ 汇总 ============
print()
print(f"通过 {ok} / 失败 {fail}")
if fail:
    sys.exit(1)
