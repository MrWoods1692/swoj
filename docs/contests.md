# 训练 / 比赛 / 作业模块

三种组织形式共享 `problems` 与 `submissions` 表，通过 `source` 字段区分来源。

## 数据表

```sql
CREATE TABLE training_plans (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  description TEXT,
  difficulty_min INTEGER DEFAULT 1,
  difficulty_max INTEGER DEFAULT 5,
  created_by INTEGER,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE training_plan_problems (
  plan_id INTEGER NOT NULL,
  problem_id INTEGER NOT NULL,
  sort INTEGER DEFAULT 0,
  PRIMARY KEY(plan_id, problem_id)
);

CREATE TABLE contests (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  description TEXT,
  start_at DATETIME,
  end_at DATETIME,
  scoring INTEGER DEFAULT 0,       -- 积分规则引用
  created_by INTEGER,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE contest_enrolls (
  contest_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  enrolled_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(contest_id, user_id)
);

CREATE TABLE assignments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  description TEXT,
  due_at DATETIME,
  created_by INTEGER,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## 接口

```
pub  GET  /api/training-plans              训练计划列表
pub  GET  /api/training-plans/{id}         训练计划详情（含题目组）
pri  POST /api/training-plans              创建训练计划（admin）

pub  GET  /api/contests                    比赛列表
pub  GET  /api/contests/{id}               比赛详情
pub  GET  /api/contests/{id}/rank          比赛排行榜（参赛者积分）
pri  POST /api/contests                    创建比赛（admin）
pri  POST /api/contests/{id}/enroll        报名（登录用户）

pub  GET  /api/assignments                 作业列表
pub  GET  /api/assignments/{id}            作业详情
pri  POST /api/assignments                 创建作业（teacher/admin）
```

## 请求示例

### 创建训练计划

```json
POST /api/training-plans
{
  "name": "动态规划入门",
  "description": "推荐先做这些题",
  "difficulty_min": 1,
  "difficulty_max": 2,
  "problems": [1, 2, 3, 4, 5]
}
```

### 创建比赛

```json
POST /api/contests
{
  "name": "校际邀请赛",
  "description": "...",
  "start_at": "2026-09-20 19:00:00",
  "end_at": "2026-09-20 22:00:00",
  "problems": [10, 11, 12]
}
```

## 响应示例（比赛详情）

```json
{
  "code": 0,
  "data": {
    "id": 1,
    "name": "校际邀请赛",
    "start_at": "2026-09-20 19:00:00",
    "end_at": "2026-09-20 22:00:00",
    "state": "ongoing",           // upcoming / ongoing / ended
    "enrolled": 42,
    "problems": [
      { "id": 10, "name": "A + B", "difficulty": 1 }
    ]
  }
}
```

## 业务规则

- 训练计划：可重复加入/移除题目；`difficulty_min/max` 用于筛选
- 比赛：`start_at` 前 → upcoming；`end_at` 前 → ongoing；否则 ended
- 报名：报名后才能在比赛内提交；提交时 `submissions.source='contest'`、`source_id=contest_id`
- 作业：`due_at` 后不能提交；提交时 `submissions.source='assignment'`
- 积分规则：比赛可通过 `POST /api/admin/points/contest/{id}/rule` 自定义计分

## 权限

- 公开：读列表、读详情、比赛排行榜
- 登录用户：报名比赛、提交（在时间窗内）
- 教师/管理员：创建作业
- 管理员：创建训练计划与比赛

## 前端

`web/src/views/Training.vue`、`TrainingDetail.vue`、`Contests.vue`、`ContestDetail.vue`、`Assignments.vue`、`AssignmentDetail.vue`。

## 验证

暂无独立 verify 脚本；`verify_logs.py` 覆盖 `contest_enroll` 等语义日志动作。
