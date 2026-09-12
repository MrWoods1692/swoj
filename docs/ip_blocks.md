# IP 黑名单模块

## 数据表

```sql
CREATE TABLE ip_blocks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cidr TEXT NOT NULL,
  reason TEXT,
  created_by INTEGER,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/admin/ip-blocks` | admin | 黑名单列表 |
| POST | `/api/admin/ip-blocks` | admin | 加入（支持 CIDR） |
| DELETE | `/api/admin/ip-blocks/{id}` | admin | 移除 |

## 请求示例

```json
POST /api/admin/ip-blocks
{ "cidr": "10.0.0.0/8", "reason": "测试内网段" }
```

响应：`{ "id": 1, "cidr": "10.0.0.0/8", "created_at": "2026-09-13 03:20:00" }`

## 业务规则

- CIDR 匹配：`netip.ParsePrefix`；支持单个 IP（无 `/`）与网段
- 中间件顺序：`recovery → cors → ipBlock → csrf → accessLog`
- 命中黑名单：返回 403「访问受限」；同时写 access_log（`status_code=403`）
- 加载策略：`ipBlockMiddleware` 启动时把全部 CIDR 载入内存，变更需手动重启或走 `admin_configs` 热刷新（当前实现每次请求查库）

## 权限

- 管理员

## 前端

`web/src/views/Admin.vue` 内嵌「IP 黑名单」面板；支持 CIDR 输入与 reason 字段。

## 验证

暂无独立 verify；通过 `verify_logs.py` 中间件日志覆盖。
