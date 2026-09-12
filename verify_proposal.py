"""学生出题 verify: 学生提交 → 老师/管理员审核 → 入题库 + 发积分。
覆盖：匿名 401 / 权限隔离 403 / 字段边界 400 / 创建 / 详情隔离 / 审核通过 / 审核拒绝 /
      撤回 / 删除 / 重复审核幂等 / 操作日志 / 老师列表
"""
import os, sys, time, json, sqlite3, requests

HOST, PORT = (os.environ.get("SWOJ_HOST_PORT", "127.0.0.1:18099").split(":"))
BASE = os.environ.get("SWOJ_HOST", f"http://{HOST}:{PORT}")
DATA_DIR = os.environ.get("SWOJ_DATA_DIR", "/tmp/swoj-verify-proposal")
DB_PATH = os.path.join(DATA_DIR, "db", "swoj.db")

PASS = 0
FAIL = 0

def check(name, cond, extra=""):
    global PASS, FAIL
    if cond:
        PASS += 1
        print(f"  [ok]   {name}")
    else:
        FAIL += 1
        print(f"  [FAIL] {name}  {extra}")

def data_of(r):
    try:
        return r.json().get("data", {}) or {}
    except Exception:
        return {}

# 登录：SWOJ_OAUTH_MOCK=1 时 /auth/campux?mock=1&mockname=X&mockqq=Y 直接 302 到 callback
def login(mockqq, name):
    s = requests.Session()
    params = {"mock": "1", "mockname": name, "mockqq": mockqq}
    r = s.get(f"{BASE}/auth/campux", params=params, allow_redirects=True, timeout=10)
    d = data_of(r)
    # 存入 headers 供后续请求用
    s.headers["Authorization"] = f"Bearer {d.get('token','')}"
    s.headers["X-CSRF-Token"] = d.get("csrf","")
    return s

# 3 个用户：第一个 = superadmin，第二个改为 teacher，第三个 user
# 校园墙用户名需 >= 3 字（handler_oauth.go 校验），因此用带后缀的名字
s_admin = login("qq_admin", "超级管理员")
s_teacher = login("qq_teacher", "老师乙")
s_student = login("qq_student", "学生甲")

conn = sqlite3.connect(DB_PATH)
conn.execute("UPDATE users SET role='teacher' WHERE qq='qq_teacher'")
conn.execute("UPDATE users SET role='user' WHERE qq='qq_student'")
conn.commit()
admin_id = conn.execute("SELECT id FROM users WHERE qq='qq_admin'").fetchone()[0]
teacher_id = conn.execute("SELECT id FROM users WHERE qq='qq_teacher'").fetchone()[0]
student_id = conn.execute("SELECT id FROM users WHERE qq='qq_student'").fetchone()[0]
conn.close()

# UPDATE role 后重新登录老师，让 JWT 拿到正确角色（JWT 签发时 role 从 users 表读）
s_teacher = login("qq_teacher", "老师乙")

def main():
    global PASS, FAIL
    print(f"\n=== 学生出题 verify  ({BASE})  ===")
    print(f"    admin={admin_id}  teacher={teacher_id}  student={student_id}\n")

    # 登录校验
    print("\n[0] 登录校验")
    d = data_of(s_student.get(f"{BASE}/api/auth/me"))
    u = d.get("data") or d
    check("学生登录", u and u.get("id") == student_id and u.get("role") == "user", f"got {u}")
    d = data_of(s_teacher.get(f"{BASE}/api/auth/me"))
    u = d.get("data") or d
    check("老师登录", u and u.get("id") == teacher_id and u.get("role") == "teacher", f"got {u}")
    d = data_of(s_admin.get(f"{BASE}/api/auth/me"))
    u = d.get("data") or d
    check("管理员登录", u and u.get("id") == admin_id and u.get("role") == "super", f"got {u}")

    # 匿名
    print("\n[1] 匿名访问")
    r = requests.get(f"{BASE}/api/proposals")
    check("匿名 GET /api/proposals → 401", r.status_code == 401)
    # 匿名 POST（写操作走 CSRF 先于 auth，返回 403 而不是 401；与项目约定一致）
    r = requests.post(f"{BASE}/api/proposals", json={})
    check("匿名 POST → 401 或 403", r.status_code in (401, 403))
    r = requests.get(f"{BASE}/api/admin/proposals")
    check("匿名 GET /api/admin/proposals → 401", r.status_code == 401)

    # 权限隔离
    print("\n[2] 权限隔离")
    r = s_student.get(f"{BASE}/api/admin/proposals")
    check("学生 GET /api/admin/proposals → 403", r.status_code == 403)
    r = s_student.post(f"{BASE}/api/admin/proposals/1/review", json={"accepted":True})
    check("学生 POST review → 403", r.status_code == 403)
    r = s_teacher.get(f"{BASE}/api/admin/proposals")
    check("老师 GET /api/admin/proposals → 200", r.status_code == 200)

    # 字段边界
    print("\n[3] 字段边界")
    body_ok = {
        "name":"测试题A",
        "background":"这是一道有趣的背景",
        "description":"描述内容",
        "input_format":"输入若干整数",
        "output_format":"输出一个整数",
        "hint":"提示",
        "cases":[{"input":"1","output":"1"}],
    }
    bad_cases = [
        ("空 body", {}),
        ("空 name", {**body_ok, "name":""}),
        ("name 仅空格", {**body_ok, "name":"   "}),
        ("空 description", {**body_ok, "description":""}),
        ("空 cases", {**body_ok, "cases":[]}),
        ("所有 cases 空", {**body_ok, "cases":[{"input":"","output":""}]}),
        ("name 超长 100", {**body_ok, "name":"a"*100}),
        ("单例输入 20000", {**body_ok, "cases":[{"input":"x"*20000,"output":"y"}]}),
        ("单例输出 20000", {**body_ok, "cases":[{"input":"y","output":"z"*20000}]}),
        ("background 超长", {**body_ok, "background":"b"*15000}),
        ("描述超长", {**body_ok, "description":"d"*60000}),
        ("样例 60 组", {**body_ok, "cases":[{"input":f"i{j}","output":f"o{j}"} for j in range(60)]}),
    ]
    for label, body in bad_cases:
        r = s_student.post(f"{BASE}/api/proposals", json=body)
        check(f"拒绝 → 400：{label}", r.status_code == 400, f"got {r.status_code} {r.text[:120]}")

    # 创建
    print("\n[4] 创建提议")
    r = s_student.post(f"{BASE}/api/proposals", json=body_ok)
    check("创建成功 200", r.status_code == 200, r.text[:200])
    d = data_of(r)
    pid = d.get("id")
    check("返回 id > 0", pid and pid > 0)
    check("初始状态 pending", d.get("status") == "pending")
    check("默认奖励 10", d.get("reward_default") == 10)

    # 列表
    print("\n[5] 我的提议列表")
    d = data_of(s_student.get(f"{BASE}/api/proposals"))
    check("学生列表非空", d.get("total",0) == 1)
    check("列表包含 pid", any(x["id"] == pid for x in (d.get("list") or [])))
    d = data_of(s_teacher.get(f"{BASE}/api/proposals"))
    check("老师列表为空", d.get("total",0) == 0)

    # 详情
    print("\n[6] 详情")
    r = s_student.get(f"{BASE}/api/proposals/{pid}")
    check("作者看详情 200", r.status_code == 200)
    d = data_of(r)
    check("cases 数量正确", len(d.get("cases") or []) == 1)
    check("author_name 已写", d.get("author_name") == "学生甲", f"got {d.get('author_name')}")

    r = s_teacher.get(f"{BASE}/api/proposals/{pid}")
    check("老师看详情 200", r.status_code == 200)
    r = s_admin.get(f"{BASE}/api/proposals/{pid}")
    check("管理员看详情 200", r.status_code == 200)
    r = s_student.get(f"{BASE}/api/proposals/99999")
    check("不存在 → 404", r.status_code == 404)

    # 学生不能审核
    r = s_student.post(f"{BASE}/api/admin/proposals/{pid}/review", json={"accepted":True})
    check("学生审核 → 403", r.status_code == 403)

    # 审核通过
    print("\n[7] 审核通过 → 题库 + 积分")
    r = s_teacher.post(f"{BASE}/api/admin/proposals/{pid}/review",
        json={"accepted":True, "reward_points":15, "comment":"优质题"})
    check("审核通过 200", r.status_code == 200, r.text[:200])
    d = data_of(r)
    check("状态 approved", d.get("status") == "approved")
    check("reward_points=15", d.get("reward_points") == 15)
    problem_id = d.get("problem_id")
    check("problem_id > 0", problem_id and problem_id > 0)

    conn = sqlite3.connect(DB_PATH)
    row = conn.execute("SELECT name, content, hint FROM problems WHERE id=?", (problem_id,)).fetchone()
    check("problems 表已写入", row is not None)
    if row:
        check("题目名一致", row[0] == "测试题A")
        check("content 含背景段", "题目背景" in row[1])
        check("content 含描述段", "题目描述" in row[1])
        check("content 含输入段", "输入格式" in row[1])
        check("content 含输出段", "输出格式" in row[1])
        check("hint 一致", row[2] == "提示")
    cases = conn.execute("SELECT input, output FROM cases WHERE problem_id=? ORDER BY index_no", (problem_id,)).fetchall()
    check("cases 表 1 组", len(cases) == 1)
    if cases:
        check("case 内容一致", cases[0] == ("1","1"))

    user_points = conn.execute("SELECT points FROM users WHERE id=?", (student_id,)).fetchone()[0]
    check("作者积分入账 15", user_points == 15, f"got {user_points}")

    log = conn.execute("SELECT delta, category, ref_type, ref_id, remark FROM points_log WHERE user_id=? ORDER BY id DESC LIMIT 1", (student_id,)).fetchone()
    check("积分流水已写", log is not None)
    if log:
        check("delta=15", log[0] == 15)
        check("category=admin", log[1] == "admin")
        check("ref_type=problem", log[2] == "problem")
        check("ref_id=problem_id", log[3] == problem_id)
        check("remark 含题目名", "测试题A" in log[4])

    r = s_student.get(f"{BASE}/api/proposals/{pid}")
    d = data_of(r)
    check("提议状态 approved", d.get("status") == "approved")
    check("提议 reward_points=15", d.get("reward_points") == 15)
    check("提议 approved_problem_id 已写", d.get("approved_problem_id") == problem_id)
    check("review_comment 已存", d.get("review_comment") == "优质题")
    check("reviewer_id 已存", d.get("reviewer_id") == teacher_id)
    conn.close()

    # 幂等
    print("\n[8] 幂等")
    r = s_teacher.post(f"{BASE}/api/admin/proposals/{pid}/review", json={"accepted":True})
    check("重复审核 → 400", r.status_code == 400)

    # 拒绝
    print("\n[9] 审核拒绝")
    body2 = {"name":"拒绝的题","description":"拒绝描述","cases":[{"input":"a","output":"a"}]}
    r = s_student.post(f"{BASE}/api/proposals", json=body2)
    pid2 = data_of(r).get("id")
    r = s_teacher.post(f"{BASE}/api/admin/proposals/{pid2}/review",
        json={"accepted":False, "comment":"质量不高"})
    check("拒绝 200", r.status_code == 200)
    d = data_of(r)
    check("状态 rejected", d.get("status") == "rejected")
    check("拒绝无奖励", d.get("reward_points") == 0)
    r = s_student.get(f"{BASE}/api/proposals/{pid2}")
    d = data_of(r)
    check("review_comment 已存", d.get("review_comment") == "质量不高")

    # 管理员审核
    print("\n[10] 管理员审核")
    body3 = {"name":"管理员审核题","description":"d","cases":[{"input":"a","output":"a"}]}
    r = s_student.post(f"{BASE}/api/proposals", json=body3)
    pid3 = data_of(r).get("id")
    r = s_admin.post(f"{BASE}/api/admin/proposals/{pid3}/review",
        json={"accepted":True, "reward_points":20, "comment":"ok"})
    check("管理员审核 200", r.status_code == 200, r.text[:200])
    d = data_of(r)
    check("reward=20", d.get("reward_points") == 20)
    check("problem_id > 0", d.get("problem_id",0) > 0)

    # 撤回
    print("\n[11] 撤回")
    body4 = {"name":"撤回题","description":"d","cases":[{"input":"a","output":"a"}]}
    r = s_student.post(f"{BASE}/api/proposals", json=body4)
    pid4 = data_of(r).get("id")
    r = s_student.put(f"{BASE}/api/proposals/{pid4}/withdraw")
    check("撤回 200", r.status_code == 200)
    check("状态 withdrawn", data_of(r).get("status") == "withdrawn")

    r = s_student.put(f"{BASE}/api/proposals/{pid}/withdraw")
    check("已通过不能撤回 → 400", r.status_code == 400)
    r = s_teacher.put(f"{BASE}/api/proposals/{pid4}/withdraw")
    check("他人撤回 → 404", r.status_code == 404)
    r = s_student.put(f"{BASE}/api/proposals/99999/withdraw")
    check("不存在撤回 → 404", r.status_code == 404)

    # 删除
    print("\n[12] 删除")
    r = s_student.delete(f"{BASE}/api/proposals/{pid4}")
    check("删除撤回的 200", r.status_code == 200)
    r = s_student.get(f"{BASE}/api/proposals/{pid4}")
    check("删除后 404", r.status_code == 404)
    r = s_student.delete(f"{BASE}/api/proposals/{pid}")
    check("已通过不能删除 → 400", r.status_code == 400)
    r = s_teacher.delete(f"{BASE}/api/proposals/{pid2}")
    check("他人删除 → 404", r.status_code == 404)

    # 操作日志
    print("\n[13] 操作日志")
    conn = sqlite3.connect(DB_PATH)
    acts = {r[0] for r in conn.execute("SELECT DISTINCT action FROM operation_logs WHERE action LIKE 'proposal_%'").fetchall()}
    for a in ("proposal_create", "proposal_withdraw", "proposal_delete", "proposal_approve", "proposal_reject"):
        check(f"日志 {a}", a in acts)
    author = conn.execute("SELECT DISTINCT user_id FROM operation_logs WHERE action='proposal_create'").fetchall()
    check("proposal_create 作者是学生", any(r[0] == student_id for r in author))
    reviewer = conn.execute("SELECT DISTINCT user_id FROM operation_logs WHERE action='proposal_approve'").fetchall()
    check("proposal_approve 是老师或管理员", any(r[0] in (teacher_id, admin_id) for r in reviewer))
    conn.close()

    # 老师审核列表
    print("\n[14] 老师审核列表")
    r = s_teacher.get(f"{BASE}/api/admin/proposals")
    check("老师列表 200", r.status_code == 200)
    d = data_of(r)
    check("列表 >= 3", d.get("total",0) >= 3, d.get("total"))
    r = s_student.get(f"{BASE}/api/admin/proposals")
    check("学生 admin 列表 → 403", r.status_code == 403)

    print(f"\n{'='*40}")
    print(f"通过 {PASS} / 失败 {FAIL}")
    print(f"{'='*40}")
    return FAIL == 0

if __name__ == "__main__":
    ok = main()
    sys.exit(0 if ok else 1)
