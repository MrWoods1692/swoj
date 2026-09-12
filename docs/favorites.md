# 收藏与错题本模块

## 数据表

```sql
CREATE TABLE favorites (
  user_id INTEGER NOT NULL,
  type TEXT NOT NULL,       -- 'problem' / 'discussion' / 'material'
  item_id INTEGER NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(user_id, type, item_id)
);

CREATE TABLE wrong_questions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  problem_id INTEGER NOT NULL,
  last_submitted_at DATETIME,
  attempts INTEGER DEFAULT 1,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(user_id, problem_id)
);
```

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/api/favorites/{type}/{id}` | pri | 切换收藏（幂等） |
| GET | `/api/favorites/{type}` | pri | 收藏列表 |
| GET | `/api/wrong-questions` | pri | 错题本 |
| DELETE | `/api/wrong-questions/{id}` | pri | 从错题本移除 |

## 请求示例

```
POST /api/favorites/problem/12        # 已收藏则取消，否则加上
GET  /api/favorites/problem           # 全部收藏的题目
GET  /api/wrong-questions             # 全部错题
DELETE /api/wrong-questions/1         # 移除单条
```

## 响应示例（错题本）

```json
{
  "code": 0,
  "data": [
    {
      "id": 1,
      "problem_id": 12,
      "problem_name": "动态规划入门",
      "attempts": 5,
      "last_submitted_at": "2026-09-13 03:20:00"
    }
  ]
}
```

## 业务规则

- 收藏：`INSERT OR IGNORE` + 存在则 `DELETE`，实现 toggle
- 错题本自动归档：提交非 AC 结果时，`INSERT OR REPLACE`（attempts 递增、`last_submitted_at` 更新）
- 用户 AC 后不会自动从错题本移除；需手动删除或「一键清空」（当前未实现批量清除端点）
- `type` 允许值：`problem` / `discussion` / `material`；其他返回 400

## 权限

- 登录用户：所有端点

## 前端

`web/src/views/Favorites.vue`、`WrongQuestions.vue`；每个题目详情页右上角有收藏图标。

## 验证

暂无独立 verify；`verify_logs.py` 覆盖 `favorite_toggle` 语义日志。
