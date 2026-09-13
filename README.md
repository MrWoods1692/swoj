# SWOJ — School Wall Online Judge

从 0 开始实现的中学算法竞赛 Online Judge：Go 后端 + SQLite + Vue3/Vite 前端 + go-judge 测评（仅 C++）。

- 语言：Go 1.22、Vue 3 + Vite 5、现代 ES2022
- 数据库：SQLite（`modernc.org/sqlite` 纯 Go 驱动，无需 CGO）
- 部署：单二进制 + 静态前端目录，`go build -o swoj ./cmd/swoj` 即可
- 权限模型：首个完成校园墙 OAuth 的用户自动成为 `super`（最高权限），其余为普通用户
- 存储：所有数据写入 `$SWOJ_DATA_DIR/swoj.db`（默认 `./data/swoj.db`）

## 功能一览

| 模块 | 说明 |
|---|---|
| 题库 | 题目 CRUD、难度/标签、多语言题干（`content` 字段） |
| 提交/测评 | 并发队列（默认 4 worker），仅 C++17，接入 go-judge |
| AI 问答 | 接入 `yunzhiapi.cn/API/deepseek.php`，支持文件上传、7 天保留、Token 用量统计 |
| 训练/比赛/作业 | 三种组织形式，训练支持多题组与截止 |
| 讨论区 | 题目下讨论、点赞、教师置顶 |
| 排行榜 | 个人主页排名口径统一（去重 AC 题数严格更多 +1） |
| 错题本 | 失败提交自动归档 |
| 积分/商城/等级 | 提交/AC/讨论行为发放积分，可兑换装扮 |
| 在线编辑器 | CodeMirror 6 + Ace 主题，支持题目模板 |
| 服务状态/测评队列 | 实时展示 judge 状态、当前队列 |
| 推荐服务 | 用户做题历史 → 相似度打分 |
| 日志系统 | 所有 API 请求写 `operation_logs`（含匿名）、业务操作写带 `action` 语义日志 |
| 多语言 | i18n（zh-CN / en）；`navigator.language` 自动识别 |
| 关于/开源/联系/更新日志/服务条款 | 静态内容页 |
| 管理后台 | 题目、用户、日志、AI 配置、资料、公告、积分、商城 |

## 快速开始

```bash
# 1) 构建后端
go build -o swoj ./cmd/swoj

# 2) 构建前端
cd web && npm install && npm run build
cd ..

# 3) 配置 judge（可选，未配置时提交会返回 503）
export SWOJ_JUDGE_BIN=/usr/local/bin/go-judge

# 4) 启动
SWOJ_LISTEN=:8080 SWOJ_STATIC=./web/dist ./swoj
```

浏览器打开 http://localhost:8080 ，走一次「校园墙登录」→ 首个用户成为 `super`，进入「管理后台 → AI 配置」填入 `yunzhiapi.cn` 的 Token 即可开始使用 AI 问答。

## 文档

- [`docs/README.md`](docs/README.md) — 文档索引
- [`docs/API.md`](docs/API.md) — 全部对外 API 一览（112 条路由）
- [`docs/AI.md`](docs/AI.md) — AI 问答模块：接口规格、Token 估算、文件上传、7 天保留、用量统计
- [`docs/AI_STATS.md`](docs/AI_STATS.md) — AI Token 用量统计方案（字段设计、估算规则、查询端点、前端展示）

部署、systemd、nginx 配置见本文档「部署」章节。

## 环境配置

所有配置通过环境变量注入，无配置文件。

| 变量 | 默认 | 说明 |
|---|---|---|
| `SWOJ_LISTEN` | `:8080` | 监听地址 |
| `SWOJ_STATIC` | `./web/dist` | 前端静态目录 |
| `SWOJ_HOST` | `http://localhost:8080` | 对外地址，用于回调与分享链接 |
| `SWOJ_DATA_DIR` | `./data` | SQLite 数据库目录 |
| `SWOJ_JUDGE_BIN` | 空 | go-judge 可执行路径 |
| `SWOJ_WORKERS` | `4` | 测评并发 worker 数 |
| `SWOJ_QUEUE_SIZE` | `64` | 测评队列上限 |
| `SWOJ_MEM_LIMIT_MB` | `256` | 单题内存上限 |
| `SWOJ_TIME_LIMIT_MS` | `2000` | 默认单题时间上限 |
| `SWOJ_OAUTH_MOCK` | `0` | `1` 启用 mock OAuth（开发用） |
| `SWOJ_OAUTH_SECRET` | 随机 | OAuth 签名密钥 |
| `SWOJ_AI_ENABLED` | `1` | AI 服务总开关 |
| `SWOJ_AI_PROVIDER` | `yunzhi` | AI 提供商：`yunzhi` 或 `openai` |
| `SWOJ_AI_URL` | `https://yunzhiapi.cn/API/deepseek.php` | yunzhi 接口地址 |
| `SWOJ_AI_TOKEN` | 空 | yunzhi Token（推荐由管理后台保存） |
| `SWOJ_AI_SYSTEM_PROMPT` | 内置 | 模型系统提示词 |

## 目录结构

```
swoj/
├── cmd/swoj/          # main
├── internal/app/      # 全部业务代码
│   ├── bootstrap.go   # 启动初始化：迁移、队列、AI 配置、清理循环
│   ├── config.go      # 环境变量读取
│   ├── router.go      # 路由注册（pri / pub）
│   ├── middleware*.go # recovery / cors / ip-block / csrf / accessLog
│   ├── schema_*.go    # 数据表（core / domain / ops / points）
│   ├── handler_*.go   # 按业务域拆分的 handler
│   ├── ai_client.go   # AI 提供商分派与 token 估算
│   ├── judge_*.go     # go-judge 调用与队列
│   └── oauth.go       # 校园墙 OAuth
├── web/               # Vue3 + Vite
│   ├── src/views/     # 每个页面一个 .vue
│   ├── src/api.js     # fetch 封装（登录、CSRF、FormData）
│   └── src/router.js  # vue-router
└── verify_*.py        # 各模块验证脚本（pytest 风格，无依赖）
```

## 权限模型

- **`super`**：首位完成校园墙授权的用户；拥有管理员所有能力
- **`admin`**：由 `super` 在后台指派；与 `super` 权限等价但不可指派他人
- **`user`**：普通用户；可读题目、提交、讨论、使用 AI
- 未登录：只能访问 `/api/health`、`/api/announcements`、`/api/notices` 等公开端点

后端统一走 `requireClaims` / `requireAdminClaims` / `requireTeacherClaims`；前端 `auth.isAdmin` / `auth.isTeacher` 计算属性决定按钮显隐。

## 接口一览（摘要）

完整 112 条路由定义见 `internal/app/router.go`。核心：

```
POST   /api/problems                       创建题目（admin）
GET    /api/problems                       列表（公开）
GET    /api/problems/:id                   详情（公开）
POST   /api/submissions                    提交代码
GET    /api/submissions/:id                提交详情（含 judge 输出）
GET    /api/judge/queue                    当前测评队列
POST   /api/ai/ask                         AI 问答（JSON 或 multipart 上传）
GET    /api/ai/history                     个人 AI 历史（保留 7 天）
GET    /api/ai/stats                       个人 AI 用量统计
GET    /api/admin/ai-config                读取 AI 配置（admin）
PUT    /api/admin/ai-config                保存 AI 配置（admin）
GET    /api/admin/ai-stats                 全局 AI 用量统计（admin）
GET    /api/logs                           个人操作日志
GET    /api/admin/logs                     全局操作日志（admin）
```

## AI 问答

**接入**：默认使用 `yunzhiapi.cn` 的 DeepSeek 代理，管理员在「管理后台 → AI 配置」填入 Token 即可启用；也可通过环境变量 `SWOJ_AI_TOKEN` 直接指定。

**接口规格**：

```
GET https://yunzhiapi.cn/API/deepseek.php
  ?token=...
  &question=...
  &system=...     （可选，模型提示词）
  &type=text     （可选，默认 text；json 返回结构化；true 流式）
```

状态码：`200` 成功、`500` 请求超时。

**Token 用量统计**：

- 表：`ai_qas(prompt_tokens, answer_tokens, total_tokens)`
- 估算规则（`estimateTokens`）：CJK 字符每字 1 token、其他非空白每 4 字符 1 token
- 个人统计：`GET /api/ai/stats`（总次数、总 token、prompt/answer 拆分、今日用量）
- 全局统计：`GET /api/admin/ai-stats`（总量、活跃用户、Top 20、近 7 天趋势）

**文件上传**：

- 允许扩展名：`.cpp .cxx .cc .c .h .hpp .txt .in .out .md .py .java .js .ts .go .rs .sh .sql .json .xml .yml .yaml .csv .log`
- 单文件 ≤ 1MB、最多 5 个、总大小 ≤ 3MB
- 文件内容直接拼接进 prompt，无 question 时作为提问主体

**保留期**：AI 问答记录保留 7 天；`bootstrap.go` 启动时清理 + 每 6 小时循环清理过期记录。

## 日志系统

所有 API 请求与业务操作写入 `operation_logs` 表，两种记录共存：

| 来源 | action 特征 | path | status_code |
|---|---|---|---|
| `accessLogMiddleware` | `api:<METHOD> <PATH>` | 请求路径 | HTTP 状态码 |
| `logOp`（业务） | 语义如 `discussion_create` | 空 | 0 |

- 匿名请求：`user_id=0, username=""`
- 中间件日志失败静默，不影响请求
- 用户可在 `/logs` 查看个人日志；管理员在 `/admin/logs` 查看全局
- 消费端按 `action` 前缀区分两种记录

## 数据表

按业务域拆分：

- `schema_core.go`：`users` / `problems` / `submissions` / `judge_logs`
- `schema_domain.go`：`operation_logs` / `admin_configs` / `ai_qas` / `recommendation` / `discussion_likes` / `materials` 等
- `schema_ops.go`：`contests` / `assignments` / `training` / `wrong` / `announcements` / `share_links` 等
- `schema_points.go`：`point_records` / `achievements` / `shop_items` / `shop_orders` 等

**变更口径**：项目未部署，无历史库；schema 变更直接改 `CREATE TABLE`，不做 `ALTER` 兼容（见 `schema_core.go:9` 注释）。

## 开发

```bash
# 后端
go build -o /tmp/swoj-bin ./cmd/swoj
SWOJ_LISTEN=127.0.0.1:18080 SWOJ_DATA_DIR=/tmp/swoj-dev \
SWOJ_STATIC=./web/dist SWOJ_OAUTH_MOCK=1 SWOJ_OAUTH_SECRET=dev \
nohup /tmp/swoj-bin > /tmp/swoj-dev.log 2>&1 &

# 前端（开发模式）
cd web && npm run dev

# 提交前跑一次全部 verify
for f in verify_*.py; do
  python3 "$f" || exit 1
done
```

## 验证脚本

每模块一份 `verify_*.py`，无第三方依赖（只用 stdlib 的 `urllib` / `sqlite3` / `json`），单脚本可独立运行：

| 脚本 | 检查数 | 覆盖 |
|---|---|---|
| `verify_ai.py` | 116 | AI 权限、Token 掩码、上传白名单/大小/数量边界、7 天清理、用量统计 |
| `verify_changelog.py` | 105 | 更新日志 CRUD 与权限 |
| `verify_materials.py` | 67 | 资料模块、链接白名单、URL 校验 |
| `verify_rest_reminder.py` | 54 | 每小时休息提醒 |
| `verify_terms.py` | 34 | 服务条款 |
| `verify_logs.py` | 116 | 中间件日志、业务日志、按角色/时间/用户过滤 |

跑单脚本：

```bash
rm -rf /tmp/swoj-verify && SWOJ_HOST_PORT=127.0.0.1:18080 \
SWOJ_DATA_DIR=/tmp/swoj-verify python3 verify_ai.py
```

## 部署

推荐 systemd 服务：

```ini
[Unit]
Description=SWOJ Online Judge
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/swoj
Environment=SWOJ_LISTEN=0.0.0.0:8080
Environment=SWOJ_STATIC=/opt/swoj/web/dist
Environment=SWOJ_DATA_DIR=/opt/swoj/data
Environment=SWOJ_HOST=https://oj.example.com
Environment=SWOJ_JUDGE_BIN=/usr/local/bin/go-judge
ExecStart=/opt/swoj/swoj
Restart=always
User=swoj
ProtectSystem=strict
ReadWritePaths=/opt/swoj/data

[Install]
WantedBy=multi-user.target
```

反向代理（nginx）配置示例：

```nginx
server {
    listen 443 ssl http2;
    server_name oj.example.com;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 120s;   # AI 问答可能较慢
    }
}
```

## 贡献

提交规范（Conventional Commits）：

```
feat(ai): 新增 Token 用量统计
fix(judge): 修复超时判断边界
docs: 补充部署指南
refactor(router): 拆分路由注册
```

**开发节奏**：每完成一个可交付步骤 → 跑通 verify 脚本 → 回归既有 verify_*.py → 规范提交 → push 远端。跳过任何一步都会破坏既有约定。

## License

MIT License，见 `LICENSE`。
