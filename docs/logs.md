# 日志系统模块

## 设计目标

- **所有操作都留痕**：包括匿名请求、被 CSRF 拦截的写方法、失败请求
- **两级日志**：中间件级 API 访问日志 + 业务级语义日志
- **可过滤**：按用户、动作、路径、日期范围筛选
- **可聚合**：按日汇总、Top 用户、状态码分布

## 数据表

```sql
CREATE TABLE operation_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER DEFAULT 0,          -- 0 = 匿名
  username TEXT DEFAULT '',
  action TEXT NOT NULL,               -- 见下方动作字典
  method TEXT,                        -- HTTP 方法（业务日志为空）
  path TEXT,                          -- 请求路径（业务日志为空）
  status_code INTEGER DEFAULT 0,      -- HTTP 状态码（业务日志为 0）
  ip TEXT,
  user_agent TEXT,
  detail TEXT,                        -- JSON 结构体，业务字段
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_logs_user ON operation_logs(user_id);
CREATE INDEX idx_logs_action ON operation_logs(action);
CREATE INDEX idx_logs_created ON operation_logs(created_at);
```

## 日志两种记录

| 来源 | action 特征 | path | status_code |
|---|---|---|---|
| `accessLogMiddleware` | `api:<METHOD> <PATH>` | 请求路径 | HTTP 状态码 |
| `logOp(action, target, ...)` | 语义如 `discussion_create` | 空 | 0 |

**示例**：

```
id=12345  user_id=0  action="api:GET /api/health"  path="/api/health"  status_code=200
id=12346  user_id=5  action="submission_create"     path=""  status_code=0  detail={"problem_id":12}
```

## 中间件

`handler_accesslog.go`：

```go
func (s *Server) accessLogMiddleware(jwtSecret string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        wrapped := &statusRecorder{ResponseWriter: w, status: 200}
        next.ServeHTTP(wrapped, r)
        // 写库，失败静默
    })
}
```

- 用 `statusRecorder` 包装 ResponseWriter 拿到实际状态码
- 解码 JWT 拿 `user_id`（匿名 user_id=0）
- 失败静默：日志写不进去不能影响业务请求

## 语义动作字典

由 `handler_logs.go` 的 `logActionOptions` 提供：

```
submission_create   提交代码
discussion_create   创建讨论
discussion_reply    回复讨论
discussion_like     点赞讨论
problem_create      创建题目
problem_update      更新题目
ai_ask              AI 提问
admin_points_adjust 管理员调整积分
admin_shop_create   管理员创建商品
...
```

新增业务操作时，在 `logOp(...)` 调用处使用统一的动词_noun 命名，`logActionOptions` 会自动从数据库 distinct 出来。

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/admin/logs` | admin | 全局日志（可按 user_id/action/date 过滤） |
| GET | `/api/admin/logs/users` | admin | 日志中的 user_id → 用户映射（下拉筛选用） |
| GET | `/api/logs` | pri | 个人日志 |
| GET | `/api/logs/actions` | pri | 语义动作字典 |
| GET | `/api/logs/daily` | pri | 按日汇总 |

## 请求示例

### 全局日志（管理员）

```
GET /api/admin/logs?page=1&page_size=50&user_id=5&action=ai_ask&from=2026-09-13&to=2026-09-14
```

### 个人日志

```
GET /api/logs?page=1&page_size=50&action=api:POST /api/ai/ask
```

### 按日汇总

```
GET /api/logs/daily?from=2026-09-01&to=2026-09-13
```

响应：

```json
[
  { "day": "2026-09-13", "count": 42, "error_count": 3 }
]
```

## 响应示例

```json
{
  "code": 0,
  "data": {
    "total": 1245,
    "items": [
      {
        "id": 12345,
        "user_id": 5,
        "username": "alice",
        "action": "ai_ask",
        "method": "",
        "path": "",
        "status_code": 0,
        "ip": "127.0.0.1",
        "detail": { "total_tokens": 456 },
        "created_at": "2026-09-13 03:20:00"
      }
    ]
  }
}
```

## 业务规则

- 中间件日志先写；业务日志通过 `logOp` 单独写
- 匿名请求：`user_id=0`、`username=""`
- 4xx / 5xx 请求会同时写一条中间件日志（`status_code` 非 200）；业务 handler 若在失败前未调用 `logOp`，则不会有语义日志
- 时间过滤：`created_at` 按日切分（`date(created_at) >= from`）
- 保留期：日志表不设自动清理（保留全量历史，可通过 `DELETE FROM operation_logs WHERE created_at < ?` 手动清理）

## 权限

- 公开：无（所有日志端点均需登录）
- 登录用户：仅查看自己的日志
- 管理员：全局日志、按用户过滤、动作字典

## 前端

`web/src/views/AdminLogs.vue`（管理后台）、`MyLogs.vue`（个人中心）；均支持按日期、动作、状态码筛选。

## 验证

`verify_logs.py` — 116 条断言，覆盖：

- 匿名请求写日志（user_id=0）
- 未登录 POST 被 CSRF 拦截仍写日志
- 登录请求写 user_id
- 业务日志通过 `logOp` 落库
- 按 user_id / action / date 过滤正确
- 个人日志仅返回本人记录
- 管理员端点 vs 普通用户端点权限分离
