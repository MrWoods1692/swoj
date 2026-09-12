# 架构总览

## 组件

```
浏览器 (Vue3 SPA)
    │
    │ HTTPS
    ▼
HTTP Server (Go 标准库 net/http)
    │
    ├─ recoveryMiddleware        兜底 panic
    ├─ corsMiddleware            跨域
    ├─ ipBlockMiddleware         IP 黑名单
    ├─ csrfMiddleware            X-CSRF-Token 校验（写方法）
    ├─ accessLogMiddleware       所有请求写 operation_logs
    ▼
router (priv/pub 分层)
    │
    ├─ JWT 中间件（priv 路由）
    └─ Handler
          │
          ├─ sqlite DB (modernc.org/sqlite)
          ├─ judge 队列 (go-judge subprocess)
          ├─ AI client (yunzhiapi.cn / OpenAI)
          └─ OAuth client (校园墙)
```

## 中间件顺序

`router.go` 定义：`recovery → cors → ipBlock → csrf → accessLog → handler`。

- `csrf` 在 `auth` 之前：未登录的写方法先被 CSRF 检查，因此返回 403 而非 401
- `accessLog` 在最内层：所有请求（含匿名、含被 CSRF 拦截的）都落日志
- 中间件日志失败静默，不影响请求

## 分层

| 层 | 文件 | 职责 |
|---|---|---|
| 入口 | `cmd/swoj/main.go` | 解析参数、启动 |
| 引导 | `bootstrap.go` | 迁移、队列、AI 配置、清理循环 |
| 配置 | `config.go` | 环境变量读取、admin_configs 覆盖 |
| 路由 | `router.go` | `pri`/`pub` 注册、中间件串联 |
| 模型 | `model.go` | 数据行 struct |
| Schema | `schema_*.go` | CREATE TABLE |
| 通用 | `handler_common.go`、`http_util.go` | decode/encode/Fail/OK/logOp |
| 认证 | `handler_auth.go`、`handler_oauth.go`、`jwt.go` | JWT、Cookie、OAuth |
| 业务 | `handler_*.go` | 按域拆分（problem/submission/ai/...） |
| 队列 | `queue.go` | 测评并发调度 |
| Judge | `judge.go` | go-judge 子进程封装 |
| AI | `ai_client.go` | Provider 分派、token 估算 |
| 积分 | `points.go`、`achievements.go` | 积分发放、成就计算 |

## 权限模型

- **`super`**：首位完成校园墙授权的用户；拥有全部能力
- **`admin`**：`super` 在后台指派；权限等价，但不可指派他人
- **`teacher`**：可创建/修改作业与资料；不可管理用户
- **`user`**：普通用户；可读、可提交、可使用 AI
- 未登录：仅可访问 `pub` 路由

后端统一通过：
- `requireClaims` — 登录即可
- `requireTeacherClaims` — teacher/admin/super
- `requireAdminClaims` — admin/super

前端 `auth.isAdmin` / `auth.isTeacher` 计算属性决定按钮显隐；权限边界由后端强制。

## 数据一致性口径

- **排名**：去重 AC 题数严格更多的人数 + 1（`countACDistinct`）
- **状态**：`accepted`/`wrong_answer`/`time_limit_exceeded`/`runtime_error`/`memory_limit_exceeded`/`output_limit_exceeded`/`compile_error`/`pending`/`running` 等
- **积分**：提交成功 +1，AC +10，评论点赞 +2，管理员可手动调整
- **日志**：中间件日志 `api:<METHOD> <PATH>`；业务日志 `logOp(action, target, ...)`

## 错误处理

- 所有 handler 用 `Fail(w, code, msg)` 返回 JSON `{code, msg}`
- 所有成功用 `OK(w, data)` 返回 `{code: 0, data: ...}`
- `recoveryMiddleware` 兜底 panic，返回 500

## 部署形态

单二进制 + 静态目录：

```
/opt/swoj/
├── swoj                     后端二进制
├── web/dist/                前端构建产物
├── data/swoj.db             SQLite 数据库
└── logs/swoj.log            标准输出日志
```

Go 服务直接 serve 静态目录（`SWOJ_STATIC`）；无需独立 web server。生产环境可在前面加 nginx 做 TLS 与反向代理，见 [`DEPLOY.md`](DEPLOY.md)。
