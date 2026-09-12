# 讨论模块

## 数据表

```sql
CREATE TABLE discussions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  problem_id INTEGER DEFAULT 0,      -- 0 表示全站讨论
  user_id INTEGER NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  likes INTEGER DEFAULT 0,
  pinned INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE discussion_replies (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  discussion_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  content TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE discussion_likes (
  user_id INTEGER NOT NULL,
  discussion_id INTEGER NOT NULL,
  PRIMARY KEY(user_id, discussion_id)
);
```

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/discussions` | pub | 列表（可按 problem_id 过滤） |
| GET | `/api/discussions/{id}` | pub | 详情（含回复） |
| POST | `/api/discussions` | pri | 创建讨论 |
| POST | `/api/discussions/{id}/reply` | pri | 回复 |
| POST | `/api/discussions/{id}/like` | pri | 点赞/取消（幂等） |

## 请求示例

### 创建讨论

```json
POST /api/discussions
{
  "problem_id": 12,
  "title": "这题会不会有负数？",
  "content": "输入范围有没有负数边界？"
}
```

### 回复

```json
POST /api/discussions/12/reply
{ "content": "没有负数，都是非负整数。" }
```

### 点赞

```
POST /api/discussions/12/like     # 幂等：已点赞则取消，未点赞则加上
```

## 响应示例

```json
{
  "code": 0,
  "data": {
    "id": 12,
    "problem_id": 1,
    "user": { "id": 5, "username": "alice" },
    "title": "这题会不会有负数？",
    "content": "输入范围有没有负数边界？",
    "likes": 3,
    "pinned": 0,
    "liked": true,
    "replies": [
      {
        "id": 30,
        "user": { "id": 2, "username": "bob" },
        "content": "没有负数",
        "created_at": "2026-09-13 03:20:00"
      }
    ],
    "created_at": "2026-09-13 03:00:00"
  }
}
```

## 业务规则

- 讨论创建时 `likes=0`，`pinned=0`
- 回复数通过 JOIN 实时计算
- 点赞：`INSERT OR IGNORE` 到 `discussion_likes`，同时 `UPDATE discussions SET likes = likes ± 1`
- 教师/管理员可置顶（当前由后台接口控制，前端未暴露）
- 作者本人可删除自己的讨论（当前未实现独立 DELETE 端点；后台可删）

## 权限

- 公开：读列表、读详情
- 登录用户：创建、回复、点赞
- 教师/管理员：置顶、删除（当前通过 `DELETE /api/discussions/{id}` 实现，由 `requireTeacherClaims` 校验）

## 前端

`web/src/views/DiscussionDetail.vue`、`Discussions.vue`；每个题目详情页内嵌讨论区。

## 验证

暂无独立 `verify_discussions.py`；`verify_logs.py` 覆盖 `discussion_create` 语义日志。
