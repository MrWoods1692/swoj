# 个人笔记（notes）

`notes` 模块提供仅本人可见的私密备忘，用于记录题目思路、复盘、错题笔记等。与「错题本」的定位互补：错题本由提交自动归档，笔记完全由用户手动维护。

## 1. 数据表

```sql
CREATE TABLE IF NOT EXISTS notes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  tags TEXT DEFAULT '',
  pinned INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_notes_user_time ON notes(user_id, created_at);
```

- **user_id**：笔记归属人；所有查询、更新、删除都以 `user_id` 为谓词，天然实现跨用户隔离，无需额外权限判断。
- **title**：1–80 字，去空白后非空。
- **content**：正文，上限 100 000 字；不做 Markdown 渲染，仅按纯文本保存（前端展示时按原文换行）。
- **tags**：逗号分隔的标签字符串；写入前由 `normalizeNote` 规范化（见 §2）。
- **pinned**：置顶标记，影响列表排序（置顶优先）。
- 无附件字段：笔记是纯文本，不承载外部链接或文件。

## 2. 字段规范化规则

| 字段 | 规则 | 违规行为 |
|---|---|---|
| title | 去首尾空白，1–80 字 | 空 / 仅空白 / 超长 → 400 |
| content | 去首尾空白，≤ 100 000 字 | 超长 → 400 |
| tags | 按 `,` `，` ` ` `、` `;` `；` 切分，每段去空白，去空串、去重、每段 ≤ 12 字、最多 8 个，再用逗号拼回 | 非法段静默丢弃，不报错 |
| pinned | 布尔，落库为 0/1 | 缺省 false |

标签上限是「去重去空之后」的 8 个，即 `dp, dp, dp` 归一后只有 1 个标签。

## 3. 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/notes` | pri | 列出本人笔记，置顶优先，其次按 `updated_at` 倒序，上限 200 条。列表页正文截断为 `snippet`（120 字），完整内容走详情接口。 |
| POST | `/api/notes` | pri | 新建笔记。请求体 `{title, content, tags, pinned}`。 |
| GET | `/api/notes/{id}` | pri | 单条详情，返回完整 `content`。跨用户访问 → 404。 |
| PUT | `/api/notes/{id}` | pri | 全字段更新（含 `pinned` 切换）。跨用户访问 → 404。 |
| DELETE | `/api/notes/{id}` | pri | 硬删除（笔记是纯用户数据，保留 0 天）。跨用户访问 → 404。 |

请求示例：

```bash
curl -X POST -H 'Content-Type: application/json' \
     -H 'Authorization: Bearer $TOKEN' \
     -H 'X-CSRF-Token: $CSRF' \
     -d '{"title":"动态规划入门","content":"从背包问题开始…","tags":"dp,背包","pinned":false}' \
     http://host:port/api/notes
```

响应：`{"code":200,"msg":"ok","data":{"id":1}}`

## 4. 错误码

| 场景 | 状态 | 消息 |
|---|---|---|
| 未登录 GET | 401 | 请先登录 |
| 未登录写 | 403 | CSRF 校验失败 |
| 标题为空 / 仅空白 / 超长 | 400 | 标题不能为空或超长 |
| 正文超长 | 400 | 标题不能为空或超长（同一 handler 报错文案） |
| 非法 JSON | 400 | 请求体格式错误 |
| pathID 非法（如 `/api/notes/abc`） | 400 | 笔记编号无效 |
| 不存在 / 跨用户访问 | 404 | 笔记不存在 |

## 5. 前端

- 路由：`/notes`（`web/src/views/Notes.vue`），登录用户在顶栏「笔记」chip 进入。
- 页面结构：编辑器（新建/编辑） + 搜索筛选栏 + 卡片列表 + 删除确认弹窗。
- 编辑保存调用 `POST /api/notes` 或 `PUT /api/notes/{id}`；切换置顶先 GET 详情再 PUT，避免列表页的 `snippet` 覆盖 `content`。
- 列表搜索走前端 `filter`（列表上限 200 条已足够本地筛选），不走后端参数。

## 6. 业务日志

写入 `operation_logs` 表，action 分别为 `note_create` / `note_update` / `note_delete`，target 为 `id`，detail 为标题。用户在「我的日志」和管理端「日志」都能看到笔记操作轨迹。

## 7. 与「错题本」的差异

| 维度 | 错题本 | 笔记 |
|---|---|---|
| 来源 | 提交未 AC 时自动写入 | 用户手动创建 |
| 归属 | 提交所属用户 | 创建者 |
| 关联题目 | `problem_id` 外键 | 无外键，纯文本 |
| 更新方式 | 提交通过时 `accepted=1` 自动置位 | 用户编辑 |
| 删除 | 软删除（`removed=1`） | 硬删除 |
| 前端页 | `/wrong` | `/notes` |

## 8. 保留策略

笔记无自动清理：删除由用户主动触发；账号注销时由 OAuth 层负责级联（当前未实现）。
