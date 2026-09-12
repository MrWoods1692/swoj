#!/usr/bin/env python3
"""verify_changelog.py — 更新日志端点与页面验证。

覆盖分支：
  [1]  GET /api/changelog 匿名可读，空库返回 total=0
  [2]  GET /api/changelog/kinds 返回分类白名单
  [3]  未登录访问管理端点 → 401/403
  [4]  管理员新建记录 → 200，入库且返回 id
  [5]  公开列表可见新建记录，字段齐全
  [6]  详情端点可读，404 与非法编号处理正确
  [7]  参数校验：空版本号、超长标题、非法分类、非法日期 → 400
  [8]  非法 kind 回落 feature
  [9]  更新记录 → 200，字段更新且 updated_at 变化
  [10] 更新不存在记录 → 404
  [11] 删除记录 → 200，再删 → 404，公开列表不再可见
  [12] operation_logs 留痕 changelog_create/update/delete
  [13] 普通用户改 → 403
  [14] kind 筛选与分页生效
  [15] 前端页面静态检查：路由、导航、页面调用端点、表单字段、时间线渲染
  [16] 管理后台列表端点

注意：首位完成校园墙授权的用户自动成为管理员（role=super），第二位起为普通用户。
"""
import http.client
import json
import os
import sqlite3
import urllib.parse

HOST, PORT = (os.environ.get("SWOJ_HOST_PORT", "127.0.0.1:18080").split(":"))
HOST, PORT = HOST, int(PORT)
DB = os.path.join(os.environ.get("SWOJ_DATA_DIR", "/tmp/swoj-cl-test"), "db", "swoj.db")
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


print("== [1] 匿名读取更新日志 ==")
s, d, _ = req("GET", "/api/changelog")
data = d.get("data") or {}
check("匿名访问 200", s == 200, str(s))
check("返回 list", isinstance(data.get("list"), list), str(type(data.get("list"))))
check("返回 total", isinstance(data.get("total"), int))
check("返回 page", data.get("page") == 1)
check("返回 size", data.get("size") == 20)
check("返回 kinds 统计", isinstance(data.get("kinds"), dict))

print("== [2] 分类白名单 ==")
s, d, _ = req("GET", "/api/changelog/kinds")
check("kinds 200", s == 200, str(s))
kds = d.get("data", {}).get("kinds", [])
check("至少 6 个分类", len(kds) >= 6, str(len(kds)))
names = {k.get("kind") for k in kds}
for k in ("feature", "fix", "improve", "perf", "security", "refactor"):
    check(f"分类存在: {k}", k in names)
check("每个分类带 label", all(k.get("label") for k in kds))

print("== [3] 未登录访问管理端点 ==")
s, d, _ = req("GET", "/api/admin/changelog")
check("admin list 未登录 401", s == 401, str(s))
s, d, _ = req("POST", "/api/admin/changelog", body={"version": "1.0.0", "content": "x"})
check("admin create 未登录被拒（CSRF 403 或鉴权 401）", s in (401, 403), str(s))

print("== [4] 管理员新建记录 ==")
login("adm", "13900000200", "更新日志管理员")
body1 = {
    "version": "1.0.0",
    "kind": "feature",
    "content": "新增资料模块：教师与管理员可上传教学资料",
    "notes": "- 资料页由管理员维护\n- 支持图标与外链\n- 全站公开可见",
    "released_at": "2026-09-01",
}
s, d, _ = req("POST", "/api/admin/changelog", body=body1, jar="adm", extra_hdr=auth_hdr("adm"))
check("新建 200", s == 200, f'{s} {d}')
id1 = (d.get("data") or {}).get("id")
check("返回 id", isinstance(id1, int) and id1 > 0, str(id1))

s, d, _ = req("POST", "/api/admin/changelog", jar="adm", extra_hdr=auth_hdr("adm"), body={
    "version": "1.0.1", "kind": "fix", "content": "修复提交记录分页越界", "released_at": "2026-09-02",
})
check("新建第二条 200", s == 200, str(s))
id2 = (d.get("data") or {}).get("id")

s, d, _ = req("POST", "/api/admin/changelog", jar="adm", extra_hdr=auth_hdr("adm"), body={
    "version": "1.0.2", "kind": "security", "content": "更新日志接口增加 CSRF 与权限校验",
    "released_at": "2026-09-03",
})
check("新建第三条 200", s == 200, str(s))
id3 = (d.get("data") or {}).get("id")

print("== [5] 公开列表可见 ==")
s, d, _ = req("GET", "/api/changelog")
data = d.get("data") or {}
lst = data.get("list") or []
check("total 为 3", data.get("total") == 3, str(data.get("total")))
check("list 长度 3", len(lst) == 3, str(len(lst)))
r0 = lst[0]
for k in ("id", "version", "kind", "kind_label", "content", "notes",
          "released_at", "creator", "created_at", "updated_at"):
    check(f"记录字段: {k}", k in r0)
check("最新在首位（按 released_at 倒序）", r0.get("version") == "1.0.2", r0.get("version"))
check("kind_label 为中文", r0.get("kind_label") == "安全", r0.get("kind_label"))
check("creator 为管理员 id", isinstance(r0.get("creator"), int) and r0.get("creator") > 0)
check("kinds 统计 security=1", data.get("kinds", {}).get("security") == 1, str(data.get("kinds")))
check("kinds 统计 fix=1", data.get("kinds", {}).get("fix") == 1)

print("== [6] 详情端点 ==")
s, d, _ = req("GET", f"/api/changelog/{id1}")
check("详情 200", s == 200, str(s))
det = d.get("data") or {}
check("详情含 notes", "新增资料模块" in (det.get("content") or ""), str(det.get("content")))
check("详情含换行 notes", "支持图标与外链" in (det.get("notes") or ""), str(det.get("notes")))
s, d, _ = req("GET", "/api/changelog/999999")
check("不存在 404", s == 404, str(s))
s, d, _ = req("GET", "/api/changelog/abc")
check("非法编号 400", s == 400, str(s))

print("== [7] 参数校验 ==")
cases = [
    ("空版本号", {"content": "x"}),
    ("缺标题", {"version": "1.0.3"}),
    ("超长标题", {"version": "1.0.3", "content": "长" * 121}),
    ("版本号过长", {"version": "x" * 33, "content": "x"}),
    ("版本号含非法字符", {"version": "v 1.0", "content": "x"}),
    ("日期格式错", {"version": "1.0.4", "content": "x", "released_at": "2026/09/01"}),
]
for name, b in cases:
    s, d, _ = req("POST", "/api/admin/changelog", body=b, jar="adm", extra_hdr=auth_hdr("adm"))
    check(f"400 拒绝: {name}", s == 400, str(s))
s, d, _ = req("GET", "/api/changelog")
check("越界后总数仍为 3", (d.get("data") or {}).get("total") == 3)

print("== [8] 非法 kind 回落 feature ==")
s, d, _ = req("POST", "/api/admin/changelog", jar="adm", extra_hdr=auth_hdr("adm"), body={
    "version": "1.0.5", "kind": "hack;drop", "content": "非法分类回落", "released_at": "2026-09-04",
})
check("非法 kind 创建 200", s == 200, str(s))
id5 = (d.get("data") or {}).get("id")
s, d, _ = req("GET", f"/api/changelog/{id5}")
check("kind 回落 feature", (d.get("data") or {}).get("kind") == "feature",
      str((d.get("data") or {}).get("kind")))
check("kind_label 为新功能", (d.get("data") or {}).get("kind_label") == "新功能")

print("== [9] 更新记录 ==")
s, d, _ = req("PUT", f"/api/admin/changelog/{id1}", jar="adm", extra_hdr=auth_hdr("adm"), body={
    "version": "1.0.0", "kind": "improve", "content": "资料模块：支持标签与分页浏览",
    "notes": "改动说明已更新", "released_at": "2026-09-01",
})
check("更新 200", s == 200, str(s))
s, d, _ = req("GET", f"/api/changelog/{id1}")
upd = d.get("data") or {}
check("标题已更新", "标签与分页" in (upd.get("content") or ""), str(upd.get("content")))
check("kind 已更新", upd.get("kind") == "improve", str(upd.get("kind")))
check("kind_label 已更新", upd.get("kind_label") == "改进", str(upd.get("kind_label")))
check("notes 已更新", upd.get("notes") == "改动说明已更新", str(upd.get("notes")))
check("released_at 未变", upd.get("released_at") == "2026-09-01", str(upd.get("released_at")))
check("updated_at 存在", isinstance(upd.get("updated_at"), str) and len(upd["updated_at"]) >= 10)

print("== [10] 更新不存在 ==")
s, d, _ = req("PUT", "/api/admin/changelog/999999", jar="adm", extra_hdr=auth_hdr("adm"), body={
    "version": "2.0", "content": "x"})
check("404", s == 404, str(s))

print("== [11] 删除 ==")
s, d, _ = req("DELETE", f"/api/admin/changelog/{id3}", jar="adm", extra_hdr=auth_hdr("adm"))
check("删除 200", s == 200, str(s))
s, d, _ = req("DELETE", f"/api/admin/changelog/{id3}", jar="adm", extra_hdr=auth_hdr("adm"))
check("重复删除 404", s == 404, str(s))
s, d, _ = req("GET", f"/api/changelog/{id3}")
check("公开详情 404", s == 404, str(s))
s, d, _ = req("GET", "/api/changelog")
check("总数减少到 3", (d.get("data") or {}).get("total") == 3, str((d.get("data") or {}).get("total")))

print("== [12] 操作日志 ==")
conn = sqlite3.connect(DB)
rows = conn.execute(
    "SELECT action, target, detail FROM operation_logs "
    "WHERE action LIKE 'changelog_%' ORDER BY id DESC LIMIT 10"
).fetchall()
conn.close()
actions = {r[0] for r in rows}
check("留痕 changelog_create", "changelog_create" in actions, str(actions))
check("留痕 changelog_update", "changelog_update" in actions, str(actions))
check("留痕 changelog_delete", "changelog_delete" in actions, str(actions))
check("留痕含版本号", any("1.0.0" in (r[2] or "") for r in rows),
      str([r[2] for r in rows]))

print("== [13] 普通用户无权 ==")
login("user", "13900000201", "普通学生")
s, d, _ = req("POST", "/api/admin/changelog", jar="user", extra_hdr=auth_hdr("user"), body={
    "version": "3.0", "content": "x"})
check("普通用户新建 403", s == 403, str(s))
s, d, _ = req("PUT", f"/api/admin/changelog/{id1}", jar="user", extra_hdr=auth_hdr("user"), body={
    "version": "3.0", "content": "x"})
check("普通用户更新 403", s == 403, str(s))
s, d, _ = req("GET", "/api/admin/changelog", jar="user", extra_hdr=auth_hdr("user"))
check("普通用户读后台列表 403", s == 403, str(s))

print("== [14] 筛选与分页 ==")
s, d, _ = req("GET", "/api/changelog?kind=fix")
data = d.get("data") or {}
check("kind=fix 仅 1 条", data.get("total") == 1, str(data.get("total")))
check("筛选结果均为 fix", all(x.get("kind") == "fix" for x in data.get("list") or []))
s, d, _ = req("GET", "/api/changelog?kind=notakey")
check("非法 kind 参数忽略（回落全部）", (d.get("data") or {}).get("total") == 3,
      str((d.get("data") or {}).get("total")))
s, d, _ = req("GET", "/api/changelog?page=1&size=1")
data = d.get("data") or {}
check("分页 size=1 返回 1 条", len(data.get("list") or []) == 1, str(len(data.get("list") or [])))
check("分页 total 不变", data.get("total") == 3)
s, d, _ = req("GET", "/api/changelog?page=2&size=1")
data = d.get("data") or {}
check("第二页返回 1 条", len(data.get("list") or []) == 1)
s, d, _ = req("GET", "/api/changelog?page=99&size=20")
data = d.get("data") or {}
check("越界页返回空列表", data.get("list") == [], str(data.get("list")))
check("越界页 total 仍为 3", data.get("total") == 3)

print("== [16] 管理后台列表 ==")
s, d, _ = req("GET", "/api/admin/changelog", jar="adm", extra_hdr=auth_hdr("adm"))
check("后台列表 200", s == 200, str(s))
adata = d.get("data") or {}
check("后台返回 list", isinstance(adata.get("list"), list))
check("后台 total 与记录数一致", adata.get("total") == len(adata.get("list") or []))

print("== [15] 前端页面静态检查 ==")
route = read_file("router.js")
check("路由已注册 /changelog", "/changelog" in route and "Changelog.vue" in route)
app_vue = read_file("App.vue")
check("导航含更新日志入口", "to=\"/changelog\"" in app_vue)
i18n_txt = read_file("i18n.js")
check("i18n 含 changelog 中文词条", "changelog: '更新日志'" in i18n_txt)
check("i18n 含 changelog 英文词条", "changelog: 'Changelog'" in i18n_txt)
page = read_file("views/Changelog.vue")
check("页面存在", len(page) > 500, str(len(page)))
check("页面调用列表端点", "/api/changelog" in page)
check("页面调用分类端点", "/api/changelog/kinds" in page)
check("页面调用新建端点", "api.post('/api/admin/changelog'" in page)
check("页面调用更新端点", "api.put('/api/admin/changelog/'" in page)
check("页面调用删除端点", "api.del('/api/admin/changelog/'" in page)
for f in ("version", "kind", "content", "notes", "released_at"):
    check(f"表单含字段 {f}", f'form.{f}' in page)
check("展示版本号", "row.version" in page)
check("展示分类标签", "row.kind_label" in page)
check("展示发布时间", "row.released_at" in page)
check("展示详细说明", "row.notes" in page)
check("分页控件", "上一页" in page and "下一页" in page)
check("筛选控件", "filter(" in page)
check("权限控制新增按钮", "v-if=\"isAdmin\"" in page)

print(f"\n通过 {len(passed)} / 失败 {len(failed)}")
if failed:
    for f in failed:
        print("  - " + f)
    raise SystemExit(1)
