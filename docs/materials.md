# 资料模块

## 数据表

```sql
CREATE TABLE materials (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  description TEXT,
  category TEXT NOT NULL,        -- 见 /api/materials/meta
  url TEXT NOT NULL,
  file_type TEXT,                -- pdf / doc / xlsx / ppt / other
  size_bytes INTEGER DEFAULT 0,
  downloads INTEGER DEFAULT 0,
  created_by INTEGER,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/materials/meta` | pub | 分类字典 |
| GET | `/api/materials` | pub | 资料列表（可按分类过滤） |
| GET | `/api/materials/{id}` | pub | 资料详情 |
| GET | `/api/admin/materials` | teacher | 全部资料（含下架） |
| POST | `/api/admin/materials` | teacher | 创建资料 |
| PUT | `/api/admin/materials/{id}` | teacher | 更新资料 |
| DELETE | `/api/admin/materials/{id}` | teacher | 删除资料 |

## 请求示例

### 创建资料

```json
POST /api/admin/materials
{
  "title": "动态规划讲义",
  "description": "入门到进阶",
  "category": "讲义",
  "url": "https://files.example.com/dp.pdf",
  "file_type": "pdf",
  "size_bytes": 524288
}
```

## 业务规则

- URL 白名单：仅允许 `https://` 开头且域名在允许列表中的链接；拒绝 `http://`、`file://`、`javascript:` 等
- 下载计数：前端点击下载按钮时 POST 一次（当前通过用户点击 +1 实现）
- 分类字典由 `/api/materials/meta` 提供；管理员可动态维护

## 权限

- 公开：读全部资料
- 教师/管理员：创建、更新、删除

## 前端

`web/src/views/Materials.vue` — 网格 + 分类筛选；`web/src/views/Admin.vue` 内嵌资料管理面板。

## 验证

`verify_materials.py` — 67 条断言，覆盖：

- URL 白名单校验（http / file / javascript 拒绝）
- 分类字典返回
- 首位 super 用户可创建
- 普通用户 403
- 已下架资料不可见
