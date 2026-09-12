# 公告模块

## 数据表

```sql
CREATE TABLE notices (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  pinned INTEGER DEFAULT 0,
  top INTEGER DEFAULT 0,        -- 置顶
  created_by INTEGER,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/notices` | pub | 公告列表（首页顶部） |
| GET | `/api/notices/{id}` | pub | 公告详情 |
| GET | `/api/admin/notices` | admin | 全部公告（含已下架） |
| POST | `/api/admin/notices` | admin | 创建公告 |
| PUT | `/api/admin/notices/{id}` | admin | 更新公告 |
| DELETE | `/api/admin/notices/{id}` | admin | 删除公告 |

## 请求示例

### 创建

```json
POST /api/admin/notices
{
  "title": "本周六维护公告",
  "content": "19:00 - 21:00 系统维护，期间无法提交。",
  "top": 1,
  "pinned": 1
}
```

## 业务规则

- 公开列表按 `top DESC, pinned DESC, created_at DESC` 排序
- 详情接口返回完整 HTML 内容
- 删除为软删除（`deleted=1`），公开列表不再显示

## 权限

- 公开：读列表、读详情
- 管理员：创建、更新、删除

## 前端

首页顶部通知条、`web/src/views/Admin.vue` 内嵌公告管理面板。

## 验证

暂无独立 verify；通过 `verify_logs.py` 的中间件日志覆盖 `api:POST /api/admin/notices`。
