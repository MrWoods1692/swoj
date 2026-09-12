# 更新日志模块

## 数据表

```sql
CREATE TABLE changelogs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  kind TEXT NOT NULL,             -- 见 /api/changelog/kinds
  created_by INTEGER,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/changelog` | pub | 更新日志列表（可按 kind 过滤） |
| GET | `/api/changelog/kinds` | pub | 分类字典 |
| GET | `/api/changelog/{id}` | pub | 更新日志详情 |
| GET | `/api/admin/changelog` | admin | 全部更新日志 |
| POST | `/api/admin/changelog` | admin | 创建 |
| PUT | `/api/admin/changelog/{id}` | admin | 更新 |
| DELETE | `/api/admin/changelog/{id}` | admin | 删除 |

## 分类字典

`/api/changelog/kinds` 返回：

```json
[
  { "id": 1, "code": "feature", "name": "新功能" },
  { "id": 2, "code": "fix",     "name": "修复" },
  { "id": 3, "code": "improve", "name": "优化" },
  { "id": 4, "code": "hotfix",  "name": "紧急修复" }
]
```

## 请求示例

```json
POST /api/admin/changelog
{
  "title": "AI Token 用量统计",
  "content": "新增个人与全局 Token 用量统计...",
  "kind": "feature"
}
```

## 业务规则

- 公开列表按 `created_at DESC` 排序
- 详情返回完整 HTML 内容
- 删除为软删除（`deleted=1`），公开列表不再显示

## 权限

- 公开：读列表、读详情、读分类
- 管理员：创建、更新、删除

## 前端

`web/src/views/Changelog.vue` — 时间线布局；`web/src/views/Admin.vue` 内嵌更新日志管理面板。

## 验证

`verify_changelog.py` — 105 条断言，覆盖：

- 匿名 GET 列表返回 200 与结构
- 未登录 POST → 403（CSRF）
- 普通用户 POST → 403
- super 用户 POST → 200
- 详情 404（不存在 ID）
- kind 白名单校验（非法 kind → 400）
- 编辑权限、软删除
