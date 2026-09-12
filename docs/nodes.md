# Judge 节点与 IP 黑名单

## 节点（nodes）

节点是分布测评的扩展点。当前实现：节点表已建，实际调度未实现（单 judge 二进制 + 内存队列）。

```sql
CREATE TABLE nodes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  host TEXT NOT NULL,
  port INTEGER DEFAULT 9801,
  enabled INTEGER DEFAULT 1,
  load INTEGER DEFAULT 0,
  last_heartbeat DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/admin/nodes` | admin | 节点列表 |
| POST | `/api/admin/nodes` | admin | 创建节点 |
| PUT | `/api/admin/nodes/{id}` | admin | 更新节点 |
| DELETE | `/api/admin/nodes/{id}` | admin | 删除节点 |

### 业务规则

- `last_heartbeat` 距当前 < 60s 视为在线
- `enabled=0` 的节点不参与调度
- 删除节点不影响历史提交

## IP 黑名单

```sql
CREATE TABLE ip_blocks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cidr TEXT NOT NULL,
  reason TEXT,
  created_by INTEGER,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/admin/ip-blocks` | admin | 黑名单列表 |
| POST | `/api/admin/ip-blocks` | admin | 加入（支持 CIDR） |
| DELETE | `/api/admin/ip-blocks/{id}` | admin | 移除 |

### 请求示例

```json
POST /api/admin/ip-blocks
{ "cidr": "10.0.0.0/8", "reason": "测试内网段" }
```

### 业务规则

- CIDR 匹配；命中返回 403
- 中间件顺序：`recovery → cors → ipBlock → csrf → accessLog`
- 被 IP 黑名单拦截的请求仍写 access_log（`status_code=403`）
- 支持 CIDR 与单个 IP；CIDR 用 `netip` 库解析

## 权限

- 全部端点：管理员

## 前端

`web/src/views/Admin.vue` 内嵌「节点管理」与「IP 黑名单」面板。

## 验证

暂无独立 verify；通过 `verify_logs.py` 的中间件日志覆盖。
