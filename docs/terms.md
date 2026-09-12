# 服务条款模块

## 数据表

```sql
CREATE TABLE terms (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  content TEXT NOT NULL,          -- HTML 格式
  version TEXT,                   -- 版本号
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE users (
  terms_accepted_at DATETIME      -- 用户接受时间
  -- 其余字段见 auth.md
);
```

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/terms` | pub | 服务条款详情 |
| POST | `/api/terms/accept` | pri | 用户接受条款 |

## 请求示例

```
POST /api/terms/accept     # 无 body
```

响应：`{ "accepted_at": "2026-09-13 03:20:00" }`

## 业务规则

- 首次登录检测到 `terms_accepted_at IS NULL` → 前端弹条款弹窗，用户点击「同意」→ POST 记录时间
- 条款更新后（管理员改 `updated_at`）→ 前端检测版本变化，弹窗让用户重新接受
- 条款内容通过 `admin_configs.terms_content` 或 `terms` 表存储；管理员在「管理后台 → 服务条款」编辑

## 权限

- 公开：读条款详情
- 登录用户：接受条款

## 前端

`web/src/views/Terms.vue` — 只读展示；`web/src/components/TermsModal.vue` — 首次登录弹窗。

## 验证

`verify_terms.py` — 34 条断言，覆盖：

- GET 条款详情 200 与结构
- 未登录 POST → 403（CSRF）
- 登录 POST → 200 并写入 `terms_accepted_at`
- 重复 POST 幂等（时间戳更新）
- 条款内容长度上限校验
