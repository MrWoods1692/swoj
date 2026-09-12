# 开发指南

## 环境

- Go 1.22+
- Node.js 18+
- 可选：go-judge（`/usr/local/bin/go-judge`）

## 1. 克隆与构建

```bash
git clone https://github.com/MrWoods1692/swoj.git
cd swoj

# 后端
go mod tidy
go build -o swoj ./cmd/swoj

# 前端
cd web
npm install
npm run dev        # 开发模式，热重载
npm run build     # 生产构建
cd ..
```

## 2. 本地启动

```bash
export SWOJ_LISTEN=127.0.0.1:18080
export SWOJ_DATA_DIR=/tmp/swoj-dev
export SWOJ_STATIC=$PWD/web/dist
export SWOJ_HOST=http://127.0.0.1:18080
export SWOJ_OAUTH_MOCK=1              # 启用 mock OAuth，跳过真实校园墙
export SWOJ_OAUTH_SECRET=dev-secret
export SWOJ_JUDGE_BIN=$(which go-judge 2>/dev/null || true)

nohup ./swoj > /tmp/swoj-dev.log 2>&1 &
```

首次访问会走 mock OAuth → 首位用户成为 `super`。

## 3. 目录结构

```
swoj/
├── cmd/swoj/          main
├── internal/app/      全部业务代码（~9400 行）
│   ├── bootstrap.go   启动初始化
│   ├── config.go      环境变量
│   ├── router.go      路由注册（112 条）
│   ├── middleware*.go recovery/cors/ipBlock/csrf/accessLog
│   ├── schema_*.go    数据表（core/domain/ops/points）
│   ├── handler_*.go   按业务域拆分的 handler（25+ 文件）
│   ├── ai_client.go   AI 提供商分派
│   ├── judge_*.go     go-judge 封装
│   └── oauth.go       校园墙 OAuth
├── web/               Vue3 + Vite
│   ├── src/views/     每个页面一个 .vue
│   ├── src/api.js     fetch 封装
│   └── src/router.js  vue-router
└── verify_*.py        各模块验证脚本
```

## 4. 修改后端

```bash
go build ./...          # 检查编译
go vet ./...            # 静态检查
```

跑本地服务验证：

```bash
./swoj                  # 前台运行便于看输出
```

## 5. 修改前端

```bash
cd web
npm run dev             # 热重载开发模式
npm run build           # 生产构建 → web/dist
```

前端通过 Go 服务的 `SWOJ_STATIC` 静态托管，开发时可用 `npm run dev` 的 dev server + 代理。

## 6. 数据库变更

项目未部署，无历史库；schema 变更**直接改 `CREATE TABLE`**，不做 `ALTER` 兼容（见 `internal/app/schema_core.go:9` 注释）。

```go
// schema_core.go
var schemaCore = []string{
    `CREATE TABLE IF NOT EXISTS users (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      ...
    )`,
}
```

修改后重启服务即生效（`migrate` 只在启动时跑一次）。**如果本地有测试库**：`rm -rf /tmp/swoj-dev` 后重启。

## 7. 添加新路由

`internal/app/router.go`：

```go
// pri 需要登录；pub 公开
pri("POST /api/your-feature", s.yourHandler)
pub("GET /api/your-feature", s.yourHandler)
```

权限选择：
- 只读公开数据 → `pub`
- 用户自己的数据 → `pri`
- 教师/管理员 → 用 `pri` 但 handler 内部调 `requireTeacherClaims` / `requireAdminClaims`

## 8. 添加新 handler

参考 `internal/app/handler_*.go` 命名：

```go
// handler_feature.go
package app

import "net/http"

type FeatureReq struct {
    Name string `json:"name"`
}

func (s *Server) createFeature(w http.ResponseWriter, r *http.Request) {
    claims, ok := requireAdminClaims(w, r)
    if !ok { return }
    var req FeatureReq
    if err := decode(r, &req); err != nil {
        Fail(w, http.StatusBadRequest, err.Error())
        return
    }
    // ... 业务逻辑
    OK(w, map[string]any{"id": 1})
    logOp(s.db, "feature_create", claims.UserID, "1")  // 写业务日志
}
```

## 9. 添加新页面

```bash
cd web/src/views
$EDITOR Feature.vue
```

在 `web/src/router.js` 注册路由：

```js
{ path: '/features', component: () => import('./views/Feature.vue'), meta: { auth: true } }
```

## 10. 添加验证脚本

每新增 API 模块必须写一份 `verify_*.py`，无第三方依赖：

```python
#!/usr/bin/env python3
import urllib.request, json, sqlite3, os, sys

H, P = os.environ.get("SWOJ_HOST_PORT", "127.0.0.1:18080").split(":")
DB = os.environ.get("SWOJ_DATA_DIR", "/tmp/swoj-dev") + "/swoj.db"

ok = fail = 0
def check(name, cond, info=""):
    global ok, fail
    if cond:
        ok += 1
        print(f"  ok   {name}  — {info}")
    else:
        fail += 1
        print(f"  FAIL {name}  — {info}")

# 测试逻辑
print(f"通过 {ok} / 失败 {fail}")
if fail: sys.exit(1)
```

跑：

```bash
rm -rf /tmp/swoj-verify && SWOJ_HOST_PORT=127.0.0.1:18080 \
SWOJ_DATA_DIR=/tmp/swoj-verify python3 verify_feature.py
```

**规范**：
- 覆盖匿名访问、401/403、404 不存在、400 边界、字段校验
- 独立可跑（自带 `req`/`login`/`db_conn` helpers）
- 提交前必须全绿

## 11. 提交与推送

**开发节奏**：改代码 → 写/更新 verify_*.py → 跑通新脚本 + 回归既有 → 规范提交 → push

```bash
git add -A
git commit -m "feat(module): 简短描述

- 变更点 1
- 变更点 2
- verify: verify_xxx.py N/0 通过"
git push origin main
```

Commit message 前缀：`feat` / `fix` / `docs` / `refactor` / `test` / `chore`。

## 12. 调试技巧

**看后端日志**：

```bash
tail -f /tmp/swoj-dev.log
```

**直接查数据库**：

```bash
sqlite3 /tmp/swoj-dev/swoj.db
> .schema ai_qas
> SELECT COUNT(*) FROM users;
```

**看前端网络请求**：浏览器 DevTools → Network；注意 Cookie `swoj_token` 与 `swoj_csrf`。

**看操作日志**：

```bash
sqlite3 /tmp/swoj-dev/swoj.db "SELECT action, path, status_code FROM operation_logs ORDER BY id DESC LIMIT 20"
```

## 13. 常见问题

**Q: 前端页面 404？**
A: Go 服务需要 `SWOJ_STATIC` 指向 `web/dist`。

**Q: 提交代码返回 503？**
A: 未配置 `SWOJ_JUDGE_BIN`。

**Q: AI 问答返回 503？**
A: 管理员未配置 Token。管理后台 → AI 配置。

**Q: 本地启动后访问不到？**
A: 检查 `SWOJ_LISTEN` 端口是否被占用；`lsof -i :18080`。

**Q: 中文乱码？**
A: 浏览器与服务器都需 UTF-8。数据库字段默认 UTF-8。
