# 题库模块

## 数据表

```sql
CREATE TABLE problems (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT UNIQUE,             -- 题目编号，如 P0001
  name TEXT NOT NULL,
  content TEXT,                 -- 题目描述（支持 HTML）
  input_desc TEXT,
  output_desc TEXT,
  input_example TEXT,
  output_example TEXT,
  hint TEXT,
  difficulty INTEGER DEFAULT 0, -- 1-5
  tags TEXT,                    -- 逗号分隔
  language INTEGER DEFAULT 3,   -- 3 = C++17
  time_limit_ms INTEGER DEFAULT 1000,
  memory_limit_mb INTEGER DEFAULT 256,
  test_count INTEGER DEFAULT 0,
  author_id INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

测试点通过 `judge.go` 生成的独立文件存储；不写入数据库。

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/problems` | pub | 列表（分页/搜索/难度/标签/作者） |
| GET | `/api/problems/{id}` | pub | 详情（含讨论数、最近提交） |
| POST | `/api/admin/problems` | admin | 创建题目 |
| POST | `/api/admin/problems/{id}` | admin | 更新题目 |
| DELETE | `/api/admin/problems/{id}` | admin | 删除题目 |

## 请求示例

### 创建题目

```json
POST /api/admin/problems
{
  "name": "A + B",
  "content": "<p>输入两个整数，输出它们的和。</p>",
  "input_desc": "一行两个整数 A B",
  "output_desc": "一行一个整数",
  "input_example": "1 2",
  "output_example": "3",
  "hint": "注意溢出",
  "difficulty": 1,
  "tags": "基础,数学",
  "time_limit_ms": 1000,
  "memory_limit_mb": 256
}
```

### 列表查询

```
GET /api/problems?page=1&page_size=20&keyword=dp&difficulty=3&tag=排序&sort=asc&order_by=created_at
```

## 响应示例

```json
{
  "code": 0,
  "data": {
    "total": 128,
    "items": [
      {
        "id": 1,
        "code": "P0001",
        "name": "A + B",
        "difficulty": 1,
        "tags": ["基础", "数学"],
        "language": "C++17",
        "time_limit_ms": 1000,
        "memory_limit_mb": 256,
        "test_count": 5,
        "submissions": 142,
        "accepted": 120,
        "created_at": "2026-09-10 03:20:00",
        "author": "admin"
      }
    ]
  }
}
```

## 业务规则

- **唯一性**：`code` 字段唯一；创建时若为空则自动生成（`P` + 4 位序号）
- **难度**：1-5 级；`difficulty=0` 表示未分级
- **标签**：自由文本，逗号分隔；前端展示为 chip
- **删除**：软删除（`deleted=1`）；关联的提交与讨论保留
- **作者**：`author_id` 记录创建者，可在「管理后台」修改

## 权限

- 公开：读列表、读详情
- 管理员：创建、更新、删除

## 前端

`web/src/views/Problems.vue` — 列表 + 筛选；`ProblemDetail.vue` — 详情 + 在线编辑器 + 提交记录 + 讨论。

## 验证

暂无独立 `verify_problems.py`；题库 CRUD 的边界由 `verify_logs.py` 中 `problem_create` 等语义日志动作间接覆盖。
