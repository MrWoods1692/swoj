# 提交与测评模块

## 数据表

```sql
CREATE TABLE submissions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  problem_id INTEGER NOT NULL,
  code TEXT NOT NULL,
  language INTEGER DEFAULT 3,        -- 3 = C++17
  status INTEGER DEFAULT 0,          -- 见下方状态码
  score INTEGER DEFAULT 0,
  accepted INTEGER DEFAULT 0,
  time_ms INTEGER DEFAULT 0,
  memory_mb INTEGER DEFAULT 0,
  message TEXT,                      -- 错误信息
  stdout TEXT,
  stderr TEXT,
  judge_time_ms INTEGER DEFAULT 0,   -- 单个测试点耗时
  test_point_status TEXT,            -- 每测试点结果，如 "110011"
  source TEXT DEFAULT 'web',         -- 来源：web / contest / assignment / training
  source_id INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE judge_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  submission_id INTEGER NOT NULL,
  test_point INTEGER NOT NULL,       -- 1-based
  status INTEGER,
  time_ms INTEGER,
  memory_mb INTEGER,
  output TEXT,
  stderr TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## 状态码

| 值 | 含义 | 显示 |
|---|---|---|
| 0 | 等待测评 | pending |
| 1 | 测评中 | running |
| 2 | 编译错误 | CE |
| 3 | 通过 | AC |
| 4 | 答案错误 | WA |
| 5 | 时间超限 | TLE |
| 6 | 运行时错误 | RE |
| 7 | 内存超限 | MLE |
| 8 | 输出超限 | OLE |
| 9 | 系统错误 | SE |

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/api/submissions` | pri | 提交代码 |
| GET | `/api/submissions` | pri | 个人提交列表 |
| GET | `/api/submissions/{id}` | pri | 提交详情（含 judge 输出） |

## 请求示例

### 提交

```json
POST /api/submissions
{
  "problem_id": 1,
  "code": "#include <bits/stdc++.h>\nusing namespace std;\nint main(){...}",
  "language": 3,
  "source": "web"
}
```

### 列表查询

```
GET /api/submissions?page=1&page_size=20&problem_id=1&status=3
```

## 响应示例

```json
{
  "code": 0,
  "data": {
    "total": 142,
    "items": [
      {
        "id": 1,
        "problem_id": 1,
        "problem_name": "A + B",
        "language": "C++17",
        "status": 3,
        "status_text": "AC",
        "score": 100,
        "accepted": 1,
        "time_ms": 12,
        "memory_mb": 2.1,
        "created_at": "2026-09-13 03:20:00"
      }
    ]
  }
}
```

## 测评流程

```
提交 → 入队列（queue.go）
       ↓
   4 worker 并发
       ↓
   judge.go → go-judge 子进程
       ↓
   解析 go-judge 输出 → 更新 submissions + judge_logs
       ↓
   前端轮询 /api/submissions/{id}
```

- 并发 worker 数：`SWOJ_WORKERS`（默认 4）
- 队列上限：`SWOJ_QUEUE_SIZE`（默认 64）
- 内存上限：`SWOJ_MEM_LIMIT_MB`（默认 256）
- 时间上限：题目字段 `time_limit_ms`，未设则用 `SWOJ_TIME_LIMIT_MS`（默认 2000）

## 业务规则

- 仅 C++17；其他语言返回 400
- 单次提交最长 20000 字符；超出返回 400
- 同一用户对同一题目 5 秒内重复提交 → 409（防刷）
- 未配置 `SWOJ_JUDGE_BIN` → 503
- 提交失败（SE/编译错误）不计入 `wrong_questions`
- AC 提交 → 自动发 10 积分（见 `points.go`）

## 权限

- 登录用户：提交自己的代码、查看自己的提交
- 管理员：可通过 `/api/submissions` 查看所有人（当前未实现跨用户查询接口）

## 前端

`web/src/views/ProblemDetail.vue` 内嵌「提交」面板；`Submissions.vue` 展示提交列表；`SubmissionDetail.vue` 展示详情与 judge 输出。

## 验证

`verify_logs.py` 覆盖 `submission_create` 语义日志与状态码写入；暂无独立 `verify_submissions.py`。
