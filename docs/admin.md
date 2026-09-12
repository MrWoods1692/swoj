# 管理后台模块

覆盖全部 `pri` 且 handler 内部收敛到教师或管理员的端点，按功能域分组。

## 管理员判定

```go
// handler_admin.go
func requireAdminClaims(w http.ResponseWriter, r *http.Request) (*Claims, bool) {
    claims, ok := parseClaims(r)
    if !ok {
        Fail(w, http.StatusUnauthorized, "未登录")
        return nil, false
    }
    if claims.Role != "admin" && claims.Role != "super" {
        Fail(w, http.StatusForbidden, "权限不足")
        return nil, false
    }
    return claims, true
}
```

- `super`：首位完成校园墙授权的用户，拥有全部能力
- `admin`：`super` 在后台指派；权限等价，但不可指派他人
- `teacher`：可创建/修改作业与资料；不可管理用户

## 路由分组

### 用户管理

```
pri  GET    /api/admin/users               用户列表（含搜索/分页）
pri  PUT    /api/admin/users/{id}          修改角色 / 禁用
```

禁用：`users.enabled=0`；被禁用用户下次登录时 401。

### 题目管理

```
pri  POST   /api/admin/problems            创建题目
pri  POST   /api/admin/problems/{id}       更新题目
pri  DELETE /api/admin/problems/{id}       删除题目
```

### 测评机配置

```
pri  GET    /api/admin/judge/config        读取 judge 配置
pri  PUT    /api/admin/judge/config        保存 judge 配置
pri  GET    /api/admin/queue               当前测评队列
pri  GET    /api/admin/stats               后台统计（含按日趋势）
```

### 节点管理

```
pri  GET    /api/admin/nodes               节点列表（多 judge 节点）
pri  POST   /api/admin/nodes               创建节点
pri  PUT    /api/admin/nodes/{id}          更新节点
pri  DELETE /api/admin/nodes/{id}          删除节点
```

节点用于分布测评负载；每个节点对应一台 judge 机器。

### IP 黑名单

```
pri  GET    /api/admin/ip-blocks           黑名单列表
pri  POST   /api/admin/ip-blocks           加入（支持 CIDR）
pri  DELETE /api/admin/ip-blocks/{id}      移除
```

`ipBlockMiddleware` 在 CORS 之后检查；命中返回 403。

### 全局配置

```
pri  GET    /api/admin/config              全局配置（键值）
pri  POST   /api/admin/config              保存配置
```

存储于 `admin_configs(key, value)`；覆盖环境变量。

### AI 配置

详见 [`AI.md`](AI.md)。

### 公告

```
pri  GET    /api/admin/notices             全部公告
pri  POST   /api/admin/notices             创建
pri  PUT    /api/admin/notices/{id}        更新
pri  DELETE /api/admin/notices/{id}        删除
```

### 更新日志

```
pri  GET    /api/admin/changelog           全部
pri  POST   /api/admin/changelog           创建
pri  PUT    /api/admin/changelog/{id}      更新
pri  DELETE /api/admin/changelog/{id}      删除
```

### 资料

```
pri  GET    /api/admin/materials           全部（teacher/admin）
pri  POST   /api/admin/materials           创建（teacher/admin）
pri  PUT    /api/admin/materials/{id}      更新
pri  DELETE /api/admin/materials/{id}      删除
```

### 日志

详见 [`logs.md`](logs.md)。

### 积分 / 商城

详见 [`points.md`](points.md)。

## 请求示例

### 修改用户角色

```json
PUT /api/admin/users/5
{ "role": "teacher", "enabled": true }
```

### 保存全局配置

```json
POST /api/admin/config
{ "key": "max_daily_checkin_points", "value": "2" }
```

### 加入 IP 黑名单

```json
POST /api/admin/ip-blocks
{ "cidr": "10.0.0.0/8", "reason": "测试内网段" }
```

## 业务规则

- 首位 `super` 不可被降级
- 用户禁用后所有未过期 JWT 立即失效（下次请求被拒）
- IP 黑名单：CIDR 匹配；未命中直接放行
- 配置变更立即生效；不需要重启服务
- 所有写操作写业务日志（`logOp`）

## 前端

`web/src/views/Admin.vue` 聚合所有管理面板；子组件按域拆分（`AdminUsers.vue`、`AdminProblems.vue`、`AdminLogs.vue` 等）。

## 验证

暂无独立 `verify_admin.py`；各子模块的 verify 脚本覆盖相应端点。
