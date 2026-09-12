#!/usr/bin/env python3
"""verify_rest_reminder.py — 在线满 1 小时休息提醒验证。

覆盖分支：
  [1]  POST /api/points/online 响应含 online_seconds 累计秒数
  [2]  online_seconds 与库内 online_stats.online_seconds 一致
  [3]  累积上报跨过后，累计秒数达到 1 小时阈值（前端 fire() 触发条件）
  [4]  单次上报超 600 秒被截断（既有后端行为回归）
  [5]  GET /api/points/online 仍返回 online_seconds（组件初始读取路径）
  [6]  未登录调用两个端点 → 401
  [7]  普通用户可正常上报，多用户累计互不干扰
  [8]  App.vue 已挂载 RestReminder
  [9]  组件静态检查：两端点调用、1 小时阈值、10 分钟自动消失、
       5 分钟顺延、卸载清理定时器、登录态 watch、护眼文案

阈值 1 小时是硬编码护眼建议，与后端「专注六小时」成就（online_6h）互不干扰。
"""
import http.client
import json
import os
import re
import sqlite3
import urllib.parse

HOST, PORT = (os.environ.get("SWOJ_HOST_PORT", "127.0.0.1:18080").split(":"))
HOST, PORT = HOST, int(PORT)
DB = os.path.join(os.environ.get("SWOJ_DATA_DIR", "/tmp/swoj-rest-test"), "db", "swoj.db")
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


def read_file(p):
    try:
        with open(os.path.join(WEB, p), encoding="utf-8") as f:
            return f.read()
    except FileNotFoundError:
        return ""


print("== [6] 未登录访问 ==")
s, d, _ = req("GET", "/api/points/online")
check("GET online 未登录 401", s == 401, str(s))
s, d, _ = req("POST", "/api/points/online", body={"seconds": 60})
check("POST online 未登录被拒（CSRF 403 或鉴权 401）", s in (401, 403), str(s))

print("== [1] 心跳响应含 online_seconds ==")
login("u1", "13900000300", "休息测试用户")
s, d, _ = req("POST", "/api/points/online", body={"seconds": 300},
              jar="u1", extra_hdr=auth_hdr("u1"))
data = d.get("data") or {}
check("心跳 200", s == 200, str(s))
check("返回 seconds", data.get("seconds") == 300, str(data.get("seconds")))
check("返回 points", isinstance(data.get("points"), int), str(data.get("points")))
check("返回 balance", isinstance(data.get("balance"), int), str(data.get("balance")))
check("返回 online_seconds", isinstance(data.get("online_seconds"), int),
      str(data.get("online_seconds")))
check("online_seconds 等于累计 300", data.get("online_seconds") == 300,
      str(data.get("online_seconds")))

print("== [2] 与库内累计一致 ==")
conn = sqlite3.connect(DB)
uid = conn.execute("SELECT id FROM users WHERE username='休息测试用户'").fetchone()[0]
row = conn.execute("SELECT online_seconds FROM online_stats WHERE user_id=?",
                   (uid,)).fetchone()
conn.close()
check("库内 online_seconds 为 300", row and row[0] == 300, str(row))

print("== [4] 单次超 600 秒被截断 ==")
s, d, _ = req("POST", "/api/points/online", body={"seconds": 99999},
              jar="u1", extra_hdr=auth_hdr("u1"))
data = d.get("data") or {}
check("返回 seconds 被截断为 600", data.get("seconds") == 600, str(data.get("seconds")))
check("累计等于 300+600=900", data.get("online_seconds") == 900,
      str(data.get("online_seconds")))

print("== [5] GET 端点仍含 online_seconds ==")
s, d, _ = req("GET", "/api/points/online", jar="u1", extra_hdr=auth_hdr("u1"))
data = d.get("data") or {}
check("GET 200", s == 200, str(s))
for k in ("online_seconds", "hours", "points_awarded", "next_point_at", "balance"):
    check(f"GET 字段: {k}", k in data)
check("GET online_seconds 为 900", data.get("online_seconds") == 900,
      str(data.get("online_seconds")))
check("GET hours 为 0", data.get("hours") == 0, str(data.get("hours")))

print("== [3] 跨过 1 小时阈值 ==")
final = {}
for i in range(5):
    s, d, _ = req("POST", "/api/points/online", body={"seconds": 600},
                  jar="u1", extra_hdr=auth_hdr("u1"))
    final = d.get("data") or {}
    check(f"第 {i+6} 次心跳 200", s == 200, str(s))
check("累计 3900 秒", final.get("online_seconds") == 3900,
      str(final.get("online_seconds")))
check("已跨过 3600 阈值（前端 fire() 触发条件成立）",
      final.get("online_seconds") >= 3600, str(final.get("online_seconds")))

print("== [7] 普通用户可上报 ==")
login("u2", "13900000301", "休息测试用户B")
s, d, _ = req("POST", "/api/points/online", body={"seconds": 120},
              jar="u2", extra_hdr=auth_hdr("u2"))
check("普通用户心跳 200", s == 200, str(s))
check("普通用户 online_seconds 为 120",
      (d.get("data") or {}).get("online_seconds") == 120,
      str((d.get("data") or {}).get("online_seconds")))
check("普通用户与 u1 累计互不干扰",
      (d.get("data") or {}).get("online_seconds") != 3900)

print("== [8] App.vue 挂载组件 ==")
app_vue = read_file("App.vue")
check("App.vue 引入 RestReminder", "RestReminder" in app_vue)
check("App.vue 模板挂载组件", "<RestReminder" in app_vue)

print("== [9] 组件静态检查 ==")
comp = read_file("components/RestReminder.vue")
check("组件文件存在", len(comp) > 500, str(len(comp)))
check("组件调用心跳端点", "api.post('/api/points/online'" in comp)
check("组件调用在线摘要端点", "api.get('/api/points/online'" in comp)
check("阈值常量 HOUR = 60 * 60",
      re.search(r"HOUR\s*=\s*60\s*\*\s*60", comp) is not None)
check("心跳间隔 TICK = 120",
      re.search(r"TICK\s*=\s*120", comp) is not None)
check("顺延间隔 SNOOZE = 5 分钟",
      re.search(r"SNOOZE\s*=\s*5\s*\*\s*60", comp) is not None)
check("自动消失 AUTO = 10 分钟",
      re.search(r"AUTO\s*=\s*10\s*\*\s*60", comp) is not None)
check("达到阈值时 fire()",
      "lastSeen.value >= due" in comp and "fire()" in comp)
check("fire 起 1 秒倒计时", "countdown.value -= 1" in comp)
check("倒计时到 0 自动 dismiss", "countdown.value <= 0) dismiss()" in comp)
check("dismiss 后顺延 due",
      "Math.max(due, lastSeen.value) + SNOOZE" in comp)
check("卸载清理 tick 定时器", "clearInterval(tickTimer)" in comp)
check("卸载清理倒计时定时器", "clearInterval(countTimer)" in comp)
check("登出时停止计时", "tickTimer = null" in comp)
check("未登录不打扰", "auth.user" in comp)
check("登录态 watch immediate",
      "watch(() => auth.user" in comp and "immediate: true" in comp)
check("文案含护眼建议", "让眼睛放松" in comp)
check("文案含 1 小时提示", "已连续在线满 1 小时" in comp)
check("文案含 10-20 分钟建议", "10–20 分钟" in comp)
check("文案含 6 米视距建议", "6 米外" in comp)
check("有操作按钮", '@click="dismiss"' in comp)
check("组件为 fixed 定位浮层", "position: fixed" in comp)
check("未含调试预览入口", "rest-preview" not in comp)

print(f"\n通过 {len(passed)} / 失败 {len(failed)}")
if failed:
    for f in failed:
        print("  - " + f)
    raise SystemExit(1)
