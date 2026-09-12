#!/usr/bin/env python3
"""verify_materials.py — 学习资料端点验证。

覆盖分支：
  [1]  materials 表与列落地
  [2]  GET /api/materials/meta 匿名可用，返回分类与图标白名单
  [3]  GET /api/materials 匿名可读，空库返回空列表与分类计数
  [4]  GET /api/materials/{id} 不存在 → 404，编号非法 → 400
  [5]  POST 未登录 → 401；管理员之外的普通用户 → 403
  [6]  管理员建资料：正常 200；空标题 400；非法链接 400
  [7]  分类与图标越界回落默认值
  [8]  更新成功与不存在更新 → 404
  [9]  分类筛选与置顶排序
  [10] 详情点击量 +1
  [11] 删除成功与重复删除 → 404
  [12] operation_logs 留痕
  [13] 前端路由/导航/词条静态检查

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
DB = os.path.join(os.environ.get("SWOJ_DATA_DIR", "/tmp/swoj-terms-test"), "db", "swoj.db")
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


def login(name, qq, jar):
    """mock 校园墙授权建号，返回 (token, csrf, user)。"""
    qa = "?" + urllib.parse.urlencode({"mock": "1", "mockname": name, "mockqq": qq})
    req("GET", "/auth/campux" + qa, jar=jar)
    req("GET", "/auth/campux" + qa, jar=jar)
    state = JARS[jar].get("swoj_oauth_state", "")
    cb = "?" + urllib.parse.urlencode({"mock": "1", "state": state, "mockname": name, "mockqq": qq})
    st, d, _ = req("GET", "/auth/campux/callback" + cb, jar=jar)
    assert st == 200, f"callback status={st}"
    data = d["data"]
    JARS[jar]["swoj_token"] = data["token"]
    JARS[jar]["swoj_csrf"] = data["csrf"]
    return data["token"], data["csrf"], data["user"]


print(f"[*] target http://{HOST}:{PORT}")

# [1] schema
print("[1] schema")
conn = sqlite3.connect(DB)
cols = {r[1]: r[2].upper() for r in conn.execute("PRAGMA table_info(materials)")}
conn.close()
for c in ("id", "title", "category", "url", "icon", "desc", "pinned", "clicks",
          "creator_id", "created_at", "updated_at"):
    check(f"materials.{c} 列存在", c in cols, f"type={cols.get(c)}")

# [2] meta
print("[2] GET /api/materials/meta")
st, d, _ = req("GET", "/api/materials/meta")
d = d["data"]
check("匿名 200", st == 200, f"status={st}")
check("返回分类白名单", d["categories"][0] == "" and "算法" in d["categories"],
      f"cats={d['categories']}")
check("返回图标白名单", "book" in d["icons"] and "video" in d["icons"], f"icons={d['icons']}")
cats = d["categories"]

# [3] 空列表
print("[3] GET /api/materials (空库)")
st, d, _ = req("GET", "/api/materials")
d = d["data"]
check("匿名 200", st == 200, f"status={st}")
check("空库列表为空", d["list"] == [], f"total={d['total']}")
check("返回分类计数结构", isinstance(d["categories"], dict), f"cats={d['categories']}")

# [4] 详情边界
print("[4] 详情边界")
st, _, raw = req("GET", "/api/materials/99999")
check("不存在 404", st == 404, f"status={st} body={raw[:80]}")
st, _, _ = req("GET", "/api/materials/abc")
check("编号非法 400", st == 400, f"status={st}")

# [5] 权限
print("[5] 权限")
st, d0, _ = req("GET", "/api/csrf", jar="anon")
csrf_anon = d0["data"]["csrf"]
JARS["anon"]["swoj_csrf"] = csrf_anon
st, _, raw = req("POST", "/api/admin/materials",
                 {"title": "x", "url": "https://a.com"}, jar="anon",
                 extra_hdr={"X-CSRF-Token": csrf_anon})
check("未登录 401", st == 401, f"status={st} body={raw[:80]}")

tok_a, csrf_a, u_a = login("资料管理员", "13900000001", "admin")
tok_u, csrf_u, u_u = login("资料普通用户", "13900000002", "user")
check("首位授权用户为管理员", u_a["role"] in ("super", "admin", "superadmin"), f"role={u_a['role']}")
check("第二位为普通用户", u_u["role"] == "user", f"role={u_u['role']}")

st, _, raw = req("POST", "/api/admin/materials",
                 {"title": "越权资料", "url": "https://a.com"}, jar="user",
                 extra_hdr={"X-CSRF-Token": csrf_u})
check("普通用户 403", st == 403, f"status={st} body={raw[:80]}")
st, _, _ = req("GET", "/api/admin/materials", jar="user")
check("普通用户读列表也 403", st == 403, f"status={st}")
HDR = {"X-CSRF-Token": csrf_a}

# [6] 建资料
print("[6] 创建资料")
st, _, raw = req("POST", "/api/admin/materials", {"title": "  ", "url": "https://a.com"},
                 jar="admin", extra_hdr=HDR)
check("空标题 400", st == 400, f"status={st} body={raw[:80]}")
for bad in ("ftp://a.com/x", "http://nodot", "//a.com/x", "https://a.com/x y", ""):
    st, _, raw = req("POST", "/api/admin/materials",
                     {"title": "非法链接", "url": bad}, jar="admin", extra_hdr=HDR)
    check(f"非法链接 400  — {bad!r}", st == 400, f"status={st} body={raw[:80]}")

st, d, _ = req("POST", "/api/admin/materials", {
    "title": "洛谷入门算法讲义", "category": "入门", "icon": "book",
    "url": "https://loj.ac/blog/view/10001", "desc": "适合零基础的同学", "pinned": True},
    jar="admin", extra_hdr=HDR)
check("正常创建 200", st == 200, f"status={st}")
mid = d["data"]["id"]
check("返回新 id", mid > 0, f"id={mid}")

# [7] 白名单回落
print("[7] 白名单回落")
st, d, _ = req("POST", "/api/admin/materials", {
    "title": "越界字段回落", "category": "不存在的分类", "icon": "<script>",
    "url": "https://codeforces.com"}, jar="admin", extra_hdr=HDR)
check("200", st == 200, f"status={st}")
fid = d["data"]["id"]
st, d, _ = req("GET", f"/api/materials/{fid}")
m = d["data"]
check("分类回落空串", m["category"] == "", f"category={m['category']!r}")
check("图标回落 book", m["icon"] == "book", f"icon={m['icon']!r}")
check("非法图标未原样入库", m["icon"] != "<script>", "")

# [8] 更新
print("[8] 更新")
st, d, _ = req("PUT", f"/api/admin/materials/{mid}", {
    "title": "洛谷入门算法讲义（修订版）", "category": "算法", "icon": "doc",
    "url": "https://loj.ac/blog/view/10002", "desc": "已更新", "pinned": True},
    jar="admin", extra_hdr=HDR)
check("更新 200", st == 200, f"status={st}")
st, d, _ = req("GET", f"/api/materials/{mid}")
m = d["data"]
check("标题已更新", m["title"] == "洛谷入门算法讲义（修订版）", f"title={m['title']}")
check("分类已更新", m["category"] == "算法", f"cat={m['category']}")
check("图标已更新", m["icon"] == "doc", f"icon={m['icon']}")
check("发布人昵称回填", bool(m["creator"]), f"creator={m['creator']!r}")
st, _, raw = req("PUT", "/api/admin/materials/99999",
                 {"title": "x", "url": "https://a.com"}, jar="admin", extra_hdr=HDR)
check("更新不存在 404", st == 404, f"status={st} body={raw[:80]}")
st, _, raw = req("PUT", f"/api/admin/materials/{mid}", {"title": "x", "url": "不合法"},
                 jar="admin", extra_hdr=HDR)
check("更新非法链接 400", st == 400, f"status={st}")

# [9] 分类筛选与排序
print("[9] 筛选与排序")
st, d, _ = req("GET", "/api/materials?category=" + urllib.parse.quote("算法"))
check("分类筛选 200", st == 200, f"status={st}")
ids = [x["id"] for x in d["data"]["list"]]
check("筛选命中修订版", mid in ids, f"ids={ids}")
check("筛选排除未分类", fid not in ids, f"ids={ids}")
st, d, _ = req("GET", "/api/materials?category=" + urllib.parse.quote("不存在"))
check("非法分类视为全部", st == 200 and len(d["data"]["list"]) == 2,
      f"n={len(d['data']['list'])}")

# [10] 详情点击量
print("[10] 点击量")
st, d, _ = req("GET", f"/api/materials/{mid}")
c0 = d["data"]["clicks"]
st, d, _ = req("GET", f"/api/materials/{mid}")
c1 = d["data"]["clicks"]
check("详情 200", st == 200, f"status={st}")
check("点击量递增", c1 == c0 + 1, f"{c0} -> {c1}")

# [11] 删除
print("[11] 删除")
st, d, _ = req("DELETE", f"/api/admin/materials/{fid}", jar="admin", extra_hdr=HDR)
check("删除 200", st == 200, f"status={st}")
st, _, _ = req("GET", f"/api/materials/{fid}")
check("删除后 404", st == 404, f"status={st}")
st, _, raw = req("DELETE", f"/api/admin/materials/{fid}", jar="admin", extra_hdr=HDR)
check("重复删除 404", st == 404, f"status={st} body={raw[:80]}")

# [12] 日志
print("[12] operation_logs")
conn = sqlite3.connect(DB)
rows = conn.execute(
    "SELECT action, target, detail FROM operation_logs WHERE action LIKE 'material_%'").fetchall()
conn.close()
actions = [r[0] for r in rows]
check("material_create 留痕", actions.count("material_create") >= 2, f"rows={rows}")
check("material_update 留痕", "material_update" in actions, "")
check("material_delete 留痕", "material_delete" in actions, "")

# [13] 前端静态检查
print("[13] 前端静态检查")
def rd(p):
    with open(os.path.join(WEB, p), encoding="utf-8") as f:
        return f.read()
router, app, i18n, view = rd("router.js"), rd("App.vue"), rd("i18n.js"), rd("views/Materials.vue")
check("router 含 /materials", "/materials" in router, "")
check("App 导航含资料入口", "/materials" in app, "")
check("i18n 含 materials 词条", "materials:" in i18n, "")
check("api 暴露 isEditor", "isEditor" in rd("api.js"), "")
for frag in ("api/materials/meta", "/api/admin/materials", "openAdd", "editOne",
             "auth.isEditor", "grid--2 mat-layout", "ICON_EMOJI", "open(m)"):
    check(f"页面含 {frag}", frag in view, "")

print()
print(f"=== {len(passed)} passed, {len(failed)} failed ===")
if failed:
    print("失败项：")
    for f in failed:
        print("  -", f)
    raise SystemExit(1)
