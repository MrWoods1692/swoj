#!/usr/bin/env python3
"""verify_judge_info.py — 测评机状态与配置参数端点验证。

覆盖分支：
  [1]  GET /api/judge/info 匿名可读，返回 status/version/runtime/judge/queue/nodes
  [2]  runtime 字段齐全且与运行环境一致（num_cpu>0、go_version 以 "go" 开头）
  [3]  judge 参数与启动环境变量一致（workers/queue_size/mem_limit）
  [4]  judge 视图不泄露密钥类字段
  [5]  nodes 数组含种子 builtin 节点，status/age_seconds 字段存在
  [6]  nodes_total/nodes_online/nodes_busy 计数与 nodes 数组一致
  [7]  未登录访问配置读/写 → 401
  [8]  管理员读配置 → 200，返回 overridden 与 restart_required
  [9]  管理员改参数 → 200 且立即生效；operation_logs 留痕 judge_config_set
  [10] 参数越界 → 400（workers=0 / mem_limit=1 / timeout=0 / queue=超大）
  [11] 只改单个参数不影响其他参数
  [12] 路径类参数（judge_bin、cgroup_mount）不在可写白名单内
  [13] 普通用户改参数 → 403
  [14] 前端页面静态检查：状态页引用 /api/judge/info、展示参数区、节点列表
  [15] 旧端点 /api/status 仍可用（回归）

注意：authorize 的 mock 分支会 302 直接跳回回调并消费 state，
因此连发两次 authorize，用最后一次签发的 state cookie 手动回调取 JSON 令牌。
首位完成校园墙授权的用户自动成为管理员，第二位起为普通用户。
"""
import http.client
import json
import os
import sqlite3
import urllib.parse

HOST, PORT = (os.environ.get("SWOJ_HOST_PORT", "127.0.0.1:18080").split(":"))
HOST, PORT = HOST, int(PORT)
DB = os.path.join(os.environ.get("SWOJ_DATA_DIR", "/tmp/swoj-judge-test"), "db", "swoj.db")
WEB = os.path.join(os.environ.get("SWOJ_WEB", "/home/mrcwoods/code/swoj/web/src"))

passed, failed = [], []
JARS = {}


def check(name, cond, detail=""):
    (passed if cond else failed).append(name)
    print(("  ok  " if cond else "  FAIL") + f" {name}" + (f"  — {detail}" if detail else ""))


def req(method, path, body=None, jar=None, extra_hdr=None):
    """发一次请求，返回 (status, json_dict, raw_body)。不跟随 302，手动管理 cookie jar。"""
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
    """mock OAuth 登录，令牌写回 jar，返回 (token, csrf, user_dict)。"""
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


print("== [1] 匿名读取测评机信息 ==")
s, d, _ = req("GET", "/api/judge/info")
info = d.get("data") or {}
check("匿名访问 200", s == 200, str(s))
for k in ("status", "version", "uptime_seconds", "started_at",
          "runtime", "judge", "queue", "nodes", "nodes_total", "nodes_online", "nodes_busy"):
    check(f"字段存在: {k}", k in info)

print("== [2] 运行环境 ==")
rt = info.get("runtime") or {}
for k in ("go_version", "goos", "goarch", "num_cpu", "gomaxprocs",
          "num_goroutine", "mem_alloc_mb", "num_gc", "data_dir"):
    check(f"runtime 字段: {k}", k in rt)
check("num_cpu > 0", rt.get("num_cpu", 0) > 0, str(rt.get("num_cpu")))
check("go_version 以 go 开头", str(rt.get("go_version", "")).startswith("go"), rt.get("go_version"))
check("num_goroutine > 0", rt.get("num_goroutine", 0) > 0)

print("== [3] judge 参数与启动环境变量一致 ==")
j = info.get("judge") or {}
for k in ("workers", "pool_size", "queue_size", "queue_capacity", "restart_pending",
          "user_timeout_ms", "mem_limit_mb", "mem_extra_mb", "cgroup_mount", "judge_bin",
          "builtin_judge", "languages", "compiler", "compiler_path", "compiler_version"):
    check(f"judge 字段: {k}", k in j)
env_workers = int(os.environ.get("SWOJ_EXPECT_WORKERS", "3"))
env_queue = int(os.environ.get("SWOJ_EXPECT_QUEUE", "200"))
env_mem = int(os.environ.get("SWOJ_EXPECT_MEM", "512"))
check("workers 等于环境变量", j.get("workers") == env_workers, f'{j.get("workers")}!={env_workers}')
check("queue_size 等于环境变量", j.get("queue_size") == env_queue, f'{j.get("queue_size")}!={env_queue}')
check("mem_limit_mb 等于环境变量", j.get("mem_limit_mb") == env_mem, f'{j.get("mem_limit_mb")}!={env_mem}')
check("user_timeout_ms 为正整数", isinstance(j.get("user_timeout_ms"), int) and j.get("user_timeout_ms") > 0)
check("languages 仅 cpp", j.get("languages") == ["cpp"], str(j.get("languages")))
check("compiler 为 g++", j.get("compiler") == "g++")
check("compiler_path 为字符串", isinstance(j.get("compiler_path"), str))
check("builtin_judge 为布尔", isinstance(j.get("builtin_judge"), bool))
check("queue 结构齐全", all(k in (info.get("queue") or {})
      for k in ("running", "pending", "capacity", "total", "done", "workers")))
check("queue.capacity 与 judge.queue_size 一致",
      (info.get("queue") or {}).get("capacity") == j.get("queue_size"))

print("== [4] 不泄露密钥 ==")
flat = json.dumps(info, ensure_ascii=False).lower()
check("响应不含 secret", "secret" not in flat)
check("响应不含 jwt", "jwt" not in flat)
check("响应不含 token", "token" not in flat)
check("响应不含 apikey", "apikey" not in flat)

print("== [5][6] 节点 ==")
nodes = info.get("nodes") or []
check("至少一个节点", len(nodes) >= 1, str(len(nodes)))
n0 = nodes[0]
for k in ("id", "name", "status", "judge_type", "total_count",
          "accept_count", "running", "created_at", "last_seen", "age_seconds"):
    check(f"节点字段: {k}", k in n0)
check("种子节点名 builtin", n0.get("name") == "builtin", n0.get("name"))
check("nodes_total 与数组长度一致", info.get("nodes_total") == len(nodes))
online = sum(1 for n in nodes if n.get("status") == 0)
busy = sum(1 for n in nodes if n.get("status") == 2)
check("nodes_online 与数组一致", info.get("nodes_online") == online,
      f'{info.get("nodes_online")}!={online}')
check("nodes_busy 与数组一致", info.get("nodes_busy") == busy, f'{info.get("nodes_busy")}!={busy}')
check("status 字段存在", info.get("status") in ("running", "busy", "offline"))

print("== [7] 未登录访问配置 ==")
s, d, _ = req("GET", "/api/admin/judge/config")
check("GET config 未登录 401", s == 401, str(s))
s, d, _ = req("PUT", "/api/admin/judge/config", body={"workers": 2})
check("PUT config 未登录被拒（CSRF 403 或鉴权 401）", s in (401, 403), str(s))

print("== [8] 管理员读配置 ==")
login("adm", "13900000100", "测评管理员")
s, d, _ = req("GET", "/api/admin/judge/config", jar="adm", extra_hdr=auth_hdr("adm"))
check("管理员读配置 200", s == 200, str(s))
got = d.get("data") or {}
check("返回 judge 视图", "judge" in got)
check("返回 overridden", "overridden" in got, str(got.keys()))
check("返回 restart_required", "restart_required" in got)
check("restart_required 含 judge_bin", "judge_bin" in (got.get("restart_required") or []))
check("restart_required 含 cgroup_mount",
      "cgroup_mount" in (got.get("restart_required") or []))

print("== [9] 管理员改参数 ==")
new_workers = max(1, env_workers - 1)
s, d, _ = req("PUT", "/api/admin/judge/config",
              jar="adm", extra_hdr=auth_hdr("adm"),
              body={"user_timeout_ms": 2500, "mem_limit_mb": 1024})
check("改参数 200", s == 200, str(s) + " " + json.dumps(d, ensure_ascii=False)[:200])
check("返回 applied", (d.get("data") or {}).get("applied") is True)
s2, d2, _ = req("GET", "/api/judge/info")
j2 = (d2.get("data") or {}).get("judge") or {}
check("超时立即生效", j2.get("user_timeout_ms") == 2500,
      f'{j2.get("user_timeout_ms")}!=2500')
check("内存限制立即生效", j2.get("mem_limit_mb") == 1024,
      f'{j2.get("mem_limit_mb")}!=1024')
check("workers 未被改写", j2.get("workers") == env_workers,
      f'{j2.get("workers")}!={env_workers}')
check("queue_capacity 与队列实际容量一致",
      j2.get("queue_capacity") == ((d2.get("data") or {}).get("queue") or {}).get("capacity"))
s3, d3, _ = req("GET", "/api/admin/judge/config", jar="adm", extra_hdr=auth_hdr("adm"))
ov = (d3.get("data") or {}).get("overridden") or {}
check("overridden 记录 judge_timeout_ms",
      ov.get("judge_timeout_ms") == 2500, str(ov))
check("overridden 记录 judge_mem_limit",
      ov.get("judge_mem_limit") == 1024, str(ov))
check("restart_pending 为 false（未改队列/worker）",
      j2.get("restart_pending") is False, str(j2.get("restart_pending")))
conn = sqlite3.connect(DB)
row = conn.execute("SELECT action, target, detail FROM operation_logs WHERE action='judge_config_set' ORDER BY id DESC LIMIT 1").fetchone()
conn.close()
check("operation_logs 留痕 judge_config_set", row is not None, str(row))
check("留痕含参数数量", row is not None and "params=" in (row[2] or ""))

print("== [10] 参数越界 ==")
bad_cases = [
    ("workers 为 0", {"workers": 0}),
    ("workers 过大", {"workers": 1000}),
    ("pool_size 为 0", {"pool_size": 0}),
    ("queue_size 为 0", {"queue_size": 0}),
    ("queue_size 超大", {"queue_size": 999999}),
    ("timeout 过小", {"user_timeout_ms": 10}),
    ("timeout 为 0", {"user_timeout_ms": 0}),
    ("timeout 过大", {"user_timeout_ms": 999999}),
    ("mem_limit 过小", {"mem_limit_mb": 1}),
    ("mem_limit 负数", {"mem_limit_mb": -512}),
    ("mem_extra 负数", {"mem_extra_mb": -1}),
    ("mem_extra 过大", {"mem_extra_mb": 8192}),
]
for name, body in bad_cases:
    s, d, _ = req("PUT", "/api/admin/judge/config",
                  jar="adm", extra_hdr=auth_hdr("adm"), body=body)
    check(f"越界拒绝: {name}", s == 400, str(s))
check("越界后超时未被改动",
      ((req("GET", "/api/judge/info")[1].get("data") or {}).get("judge") or {}).get("user_timeout_ms") == 2500)

print("== [11] 单参数修改不影响其他 ==")
before = ((req("GET", "/api/judge/info")[1].get("data") or {}).get("judge") or {}).copy()
req("PUT", "/api/admin/judge/config", jar="adm", extra_hdr=auth_hdr("adm"),
    body={"mem_extra_mb": 96})
after = ((req("GET", "/api/judge/info")[1].get("data") or {}).get("judge") or {})
check("mem_extra_mb 已更新", after.get("mem_extra_mb") == 96, str(after.get("mem_extra_mb")))
for k in ("workers", "pool_size", "queue_size", "user_timeout_ms", "mem_limit_mb"):
    check(f"未改字段保持: {k}", after.get(k) == before.get(k),
          f'{after.get(k)}!={before.get(k)}')

print("== [12] 路径类参数不可写 ==")
s, d, _ = req("PUT", "/api/admin/judge/config", jar="adm", extra_hdr=auth_hdr("adm"),
              body={"judge_bin": "/etc/passwd", "cgroup_mount": "/tmp/evil", "workers": new_workers})
check("携带路径参数被忽略而非报错", s == 200, str(s))
after = ((req("GET", "/api/judge/info")[1].get("data") or {}).get("judge") or {})
check("judge_bin 未被改写", after.get("judge_bin") == before.get("judge_bin"),
      str(after.get("judge_bin")))
check("cgroup_mount 未被改写", after.get("cgroup_mount") == before.get("cgroup_mount"))
conn = sqlite3.connect(DB)
rows = conn.execute("SELECT key FROM admin_configs WHERE key IN ('judge_bin','cgroup_mount')").fetchall()
conn.close()
check("admin_configs 未写入路径类 key", rows == [], str(rows))

print("== [12b] 需重启参数标记 ==")
req("PUT", "/api/admin/judge/config", jar="adm", extra_hdr=auth_hdr("adm"),
    body={"queue_size": 300})
after = ((req("GET", "/api/judge/info")[1].get("data") or {}).get("judge") or {})
check("queue_size 覆盖后 restart_pending 为 true",
      after.get("restart_pending") is True, str(after.get("restart_pending")))
check("展示值仍为队列实际容量", after.get("queue_capacity") == env_queue,
      f'{after.get("queue_capacity")}!={env_queue}')
check("展示 queue_size 仍为启动值", after.get("queue_size") == env_queue,
      f'{after.get("queue_size")}!={env_queue}')
s, d, _ = req("GET", "/api/admin/judge/config", jar="adm", extra_hdr=auth_hdr("adm"))
ov = (d.get("data") or {}).get("overridden") or {}
check("overridden 记录 queue_size=300", ov.get("judge_queue") == 300, str(ov))

print("== [13] 普通用户无权修改 ==")
login("usr", "13900000200", "测评普通用户")
s, d, _ = req("PUT", "/api/admin/judge/config",
              jar="usr", extra_hdr=auth_hdr("usr"), body={"workers": 1})
check("普通用户改参数 403", s == 403, str(s))
s, d, _ = req("GET", "/api/admin/judge/config", jar="usr", extra_hdr=auth_hdr("usr"))
check("普通用户读配置 403", s == 403, str(s))

print("== [14] 前端静态检查 ==")
st_vue = read_file("views/Status.vue")
check("状态页存在", len(st_vue) > 500, str(len(st_vue)))
check("状态页调用 /api/judge/info", "/api/judge/info" in st_vue)
check("状态页调用配置写接口", "/api/admin/judge/config" in st_vue)
check("展示运行环境块", "运行环境" in st_vue)
check("展示测评配置参数块", "测评配置参数" in st_vue)
check("展示节点列表", "测评节点" in st_vue)
for k in ("workers", "pool_size", "queue_size", "user_timeout_ms",
          "mem_limit_mb", "mem_extra_mb"):
    check(f"表单含参数 {k}", k in st_vue)
check("显示内存限制", "内存限制" in st_vue)
check("显示单用例超时", "超时" in st_vue)
check("显示心跳", "心跳" in st_vue)
check("显示 CPU 核数", "CPU" in st_vue)
check("显示数据目录", "数据目录" in st_vue)
check("显示编译器版本", "编译器版本" in st_vue)
check("节点状态含在线/忙碌", "在线" in st_vue and "忙碌" in st_vue)
check("保存按钮受权限控制", "canEdit" in st_vue)
check("队列水位展示", "gauge" in st_vue)

router = read_file("router.js")
check("路由已注册 /status", "/status" in router)

print("== [15] 旧端点回归 ==")
s, d, _ = req("GET", "/api/status")
check("/api/status 仍可用", s == 200, str(s))
old = d.get("data") or {}
check("/api/status 仍有 queue", "queue" in old)
check("/api/status 仍有 node", "node" in old)

print()
print(f"通过 {len(passed)} / 失败 {len(failed)}")
if failed:
    print("失败项:")
    for f in failed:
        print("  -", f)
    raise SystemExit(1)
