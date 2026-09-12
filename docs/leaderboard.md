# 排行榜模块

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/leaderboard` | pub | 全站排行榜 |
| GET | `/api/contests/{id}/rank` | pub | 比赛排行榜 |
| GET | `/api/points/rank` | pub | 积分排行榜 |

## 请求示例

```
GET /api/leaderboard?page=1&page_size=50
GET /api/leaderboard?problem_id=12     # 单题排行榜
```

## 响应示例

```json
{
  "code": 0,
  "data": {
    "total": 128,
    "items": [
      {
        "rank": 1,
        "user": { "id": 5, "username": "alice", "avatar": "..." },
        "distinct_ac": 45,
        "submissions": 180,
        "accepted": 60
      }
    ]
  }
}
```

## 排名算法

**口径统一**（`countACDistinct`）：

1. 统计用户**去重**的 AC 题数（同题多次 AC 只算 1 次）
2. 该用户的 rank = 「去重 AC 题数严格更多的人数」+ 1

SQL：

```sql
SELECT COUNT(DISTINCT problem_id) FROM submissions
WHERE user_id = ? AND accepted = 1;
```

单题排行榜则改为：

```sql
SELECT COUNT(DISTINCT user_id) FROM submissions
WHERE problem_id = ? AND accepted = 1;
```

- 全站口径一致：`/api/leaderboard` / `/api/auth/me/stats` / `/api/users/{id}/homepage` 都用同一算法
- 并列不占位：3 人并列第 5 → 下一个是第 6（不是第 8）

## 比赛排行榜

只统计「参赛用户」+ 「比赛中提交的 AC 题数」；未报名用户不出现在比赛排行榜。

```sql
SELECT e.user_id, COUNT(DISTINCT s.problem_id) AS ac
FROM contest_enrolls e
LEFT JOIN submissions s ON s.user_id = e.user_id
  AND s.source = 'contest' AND s.source_id = e.contest_id
  AND s.accepted = 1
GROUP BY e.user_id ORDER BY ac DESC;
```

## 积分排行榜

按 `users.points` 降序；并列按 `created_at` 早的靠前。

## 业务规则

- 已禁用用户不出现
- 匿名访问允许；无需登录
- 分页默认 50，最大 200

## 前端

`web/src/views/Leaderboard.vue` — 支持按题目筛选；`web/src/views/ContestDetail.vue` 内嵌比赛排行榜；`Points.vue` 显示积分排行榜。

## 验证

暂无独立 `verify_leaderboard.py`；排名一致性由 `verify_changelog.py` 中的 userHomepage 用例覆盖。
