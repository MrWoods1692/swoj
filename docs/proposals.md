# 学生出题（问题提议）

学生出题是题库的众包补充通道：普通用户提交题目草稿，老师或管理员审核通过后进入正式题库，并按默认分值（10 分，可覆盖到 1–100）给作者发放积分。

## 数据模型

### `problem_proposals`
| 字段 | 类型 | 说明 |
|---|---|---|
| id | INTEGER PK | 提议 ID |
| author_id | INTEGER | 提交者（不可变） |
| author_name | TEXT | 提交时的真实名快照（便于展示） |
| name | TEXT NOT NULL | 题目名称（1–80 字） |
| background | TEXT | 题目背景（可选，≤ 1 万字） |
| description | TEXT NOT NULL | 题目描述（必填，≤ 5 万字） |
| input_format | TEXT | 输入格式（可选，≤ 5 千字） |
| output_format | TEXT | 输出格式（可选，≤ 5 千字） |
| hint | TEXT | 提示说明（可选，≤ 5 千字） |
| status | TEXT | `pending` / `approved` / `rejected` / `withdrawn` |
| review_comment | TEXT | 审核意见 |
| reviewer_id | INTEGER | 审核人 |
| reviewed_at | TEXT | 审核时间（RFC3339） |
| reward_points | INTEGER | 通过时入账的积分 |
| approved_problem_id | INTEGER | 通过后写入 problems 表返回的 id |
| created_at | DATETIME | 提交时间 |

### `proposal_cases`
提议附带的样例。`UNIQUE(proposal_id, index_no)`；审核通过后按 `index_no` 顺序迁移到 `cases` 表，然后清空 `proposal_cases`。

## 状态机

```
        ┌─────────────┐  作者撤回     ┌───────────┐
        │   pending   │ ────────────▶ │ withdrawn │ ── DELETE ──▶ (删除)
        └──────┬──────┘               └──────┬────┘
               │ 老师/管理员审核              │ 再撤回（保留状态）
               ▼                              │
        ┌────────────────────┐  ┌────────────┘
        │  approved /        │
        │  rejected          │  ←─── 撤回后仍可再次审核
        └────────────────────┘
```

- `pending`：等待审核；审核通过 → `approved`（一次性发分入库），审核拒绝 → `rejected`。
- `withdrawn`：作者主动撤回；只有 `pending` / `rejected` 可撤回，其他状态 400。
- `rejected` 保留 `review_comment`，便于作者修改思路后再次提交。
- 已审核的提议（`approved` / `rejected`）不可重复审核，重复请求 → 400。

## API

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/api/proposals` | pri | 新建提议 |
| GET | `/api/proposals` | pri | 我的提议列表 |
| GET | `/api/proposals/{id}` | pri | 提议详情（仅作者或审核员） |
| PUT | `/api/proposals/{id}/withdraw` | pri | 作者撤回（仅 pending / rejected） |
| DELETE | `/api/proposals/{id}` | pri | 作者删除（仅 withdrawn / rejected） |
| GET | `/api/admin/proposals` | teacher+ | 待审核列表（老师或管理员） |
| POST | `/api/admin/proposals/{id}/review` | teacher+ | 审核（通过/拒绝 + 奖励积分） |

审核请求体：
```json
{
  "accepted": true,
  "reward_points": 15,
  "comment": "优质题"
}
```
- `accepted`：未传默认 `true`；显式 `false` 走拒绝分支。
- `reward_points`：仅 `accepted=true` 时生效，范围 1–100，未传用默认值 10。
- `comment`：审核意见，可选。

## 审核通过时的入库行为

- 写入 `problems`：
  - `name` = 提议 name
  - `difficulty` = `Easy`（默认；老师可在管理后台修改）
  - `problem_type` = `OJ`
  - `time_limit` / `mem_limit` = 1000 / 256
  - `content` = `## 题目背景` / `## 题目描述` / `## 输入格式` / `## 输出格式` 拼接（空段落跳过）
  - `hint` = 提议 hint
- 写入 `cases`：按 `index_no` 顺序迁移提议样例。
- 提议状态改为 `approved`，写入 `approved_problem_id`、`reward_points`、`reviewed_at`。
- 调用 `awardPoints(authorID, reward_points, "admin", "problem", problemID, "学生出题审核通过：<题目名>")` 发放积分。

## 操作日志

所有提议动作都写入 `operation_logs`：
- `proposal_create`（作者）
- `proposal_withdraw`（作者）
- `proposal_delete`（作者）
- `proposal_approve`（审核员）
- `proposal_reject`（审核员）

## 校验规则

| 字段 | 边界 |
|---|---|
| name | 必填；≤ 80 字 |
| description | 必填；≤ 50000 字 |
| background | ≤ 10000 字 |
| input_format / output_format / hint | ≤ 5000 字 |
| cases | 1–50 组；每组 input 或 output 至少一个非空；单例字段 ≤ 10000 字 |

## 前端

- 页面：`web/src/views/Proposals.vue`（含作者端 + 审核端）
- 路由：`/proposals`（`web/src/router.js`）
- 顶栏：登录后显示「出题」chip（`web/src/App.vue`）
