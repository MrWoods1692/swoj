# 认证与用户模块

## OAuth 流程

```
用户点击「校园墙登录」
    ↓
浏览器 GET /auth/campux
    ↓
后端 302 跳转 https://pass.campux.cn/oauth/authorize?...
    ↓
用户授权
    ↓
校园墙回调 https://swoj.com/auth/campux/callback?code=...
    ↓
后端换 token → 拉取用户信息 → upsert 到 users
    ↓
下发 swoj_token / swoj_csrf Cookie → 302 回原页
```

## 数据表

```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  oauth_id TEXT UNIQUE,          -- 校园墙用户 ID
  username TEXT NOT NULL,
  avatar TEXT,
  role TEXT DEFAULT 'user',      -- user / teacher / admin / super
  enabled INTEGER DEFAULT 1,
  points INTEGER DEFAULT 0,
  terms_accepted_at DATETIME,    -- 首次接受服务条款时间
  last_online_at DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## 首位用户 → super

`handler_oauth.go` 在 upsert 前检查：

```go
n, _ := db.QueryRow("SELECT COUNT(*) FROM users").Int64()
role := "user"
if n == 0 {
    role = "super"   // 首位授权用户成为超级管理员
}
```

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/auth/campux` | pub | 发起 OAuth 跳转 |
| GET | `/auth/campux/callback` | pub | OAuth 回调 |
| POST | `/api/auth/logout` | pub | 退出登录（清 Cookie） |
| GET | `/api/csrf` | pub | 匿名 CSRF Token |
| GET | `/api/auth/me` | pri | 当前用户信息 |
| PUT | `/api/auth/me` | pri | 修改本人资料 |
| GET | `/api/auth/me/stats` | pri | 个人统计（提交/AC/排名） |
| GET | `/api/users/{id}/homepage` | pub | 用户主页聚合数据（公开） |

## 请求示例

### 修改资料

```json
PUT /api/auth/me
{ "username": "new_name", "avatar": "https://..." }
```

### 个人统计

```
GET /api/auth/me/stats
```

响应：

```json
{
  "submissions": 42,
  "accepted": 30,
  "distinct_ac": 25,
  "rank": 12,
  "level": { "id": 5, "name": "进阶", "points": 120 },
  "points": 380
}
```

排名算法：去重 AC 题数严格更多的人数 + 1（`countACDistinct`）。全站口径一致（`leaderboard` / `myStats` / `userHomepage`）。

## 用户主页聚合

`/api/users/{id}/homepage` 返回：

```json
{
  "user": { "id": 5, "username": "alice", "role": "user" },
  "stats": { "submissions": 42, "accepted": 30, "distinct_ac": 25, "rank": 12 },
  "recent_submissions": [ ... ],
  "recent_discussions": [ ... ]
}
```

公开接口；任何用户可访问任何人的主页。

## 业务规则

- JWT：`HS256`，payload 含 `user_id` / `role` / `exp`；有效期 7 天
- Cookie：`swoj_token`（HttpOnly + SameSite=Lax + Secure）；`swoj_csrf`（SameSite=Lax，可 JS 读取）
- 退出：前端清 cookie 缓存；服务端 token 无法真正撤销（无 blacklist）
- 禁用用户：`enabled=0` → 下次请求返回 401
- 角色修改：仅 `super` / `admin` 可操作；不能改首位 super

## 前端

`web/src/views/Login.vue`、`Profile.vue`、`Homepage.vue`；`AuthState` 存于 Pinia，页面加载时通过 `GET /api/auth/me` 同步。

## 验证

`verify_changelog.py` 覆盖 OAuth mock 流程与首位 super 分配。
