# AI Token 用量统计

本文档描述 SWOJ 的 AI Token 用量统计方案：字段设计、估算规则、查询端点、前端展示位置。

## 1. 设计目标

- **可见**：用户能看到自己的总消耗与今日消耗；管理员能看到全局与按用户/按日趋势
- **轻量**：不依赖上游计费回调；估算基于本地字符统计，无网络开销
- **可加总**：字段可 `SUM`；聚合查询走索引，单条 O(1)
- **可追溯**：每条问答记录都带 token 数，方便定位异常高峰

## 2. 数据字段

`ai_qas` 表新增三列（`internal/app/schema_domain.go`）：

| 字段 | 类型 | 含义 |
|---|---|---|
| `prompt_tokens` | INTEGER | 本次提问发给模型的 prompt 估算 token 数 |
| `answer_tokens` | INTEGER | 模型返回内容估算 token 数 |
| `total_tokens` | INTEGER | `prompt_tokens + answer_tokens` |

写入时机：`handler_leaderboard.aiAsk` 在 `INSERT` 时同步落库；失败调用（502/503）不落库，因此失败不计入统计。

## 3. Token 估算规则

`estimateTokens(s string) int`，在 `handler_leaderboard.go`：

| 字符范围 | 每字 token |
|---|---|
| U+4E00–U+9FFF（CJK 统一汉字） | 1 |
| U+3000–U+30FF（CJK 符号和标点、假名） | 1 |
| U+AC00–U+D7AF（谚文音节） | 1 |
| U+3400–U+4DBF（CJK 扩展 A） | 1 |
| 空白字符（空格/Tab/换行） | 0 |
| 其他（ASCII、拉丁、其他非 CJK） | 每 4 字符 1 |

**为什么不用 BPE 分词器？** 部署成本高（需下载词表）；对配额展示场景足够近似。要精确对齐上游计费，需接入 usage 回调（当前未实现）。

**误差量级**：中英文混合的编程问答，估算与 DeepSeek 官方 token 计数的偏差通常在 ±15% 以内，量级一致。

## 4. 查询端点

### 4.1 `GET /api/ai/stats`（登录即可）

```json
{
  "total_calls": 12,
  "total_tokens": 4567,
  "prompt_tokens": 3000,
  "answer_tokens": 1567,
  "today_calls": 3,
  "today_tokens": 812,
  "retention_days": 7
}
```

SQL：

```sql
SELECT COALESCE(SUM(prompt_tokens),0),
       COALESCE(SUM(answer_tokens),0),
       COUNT(*)
FROM ai_qas WHERE user_id = ?;

SELECT COALESCE(SUM(total_tokens),0), COUNT(*)
FROM ai_qas WHERE user_id = ? AND date(created_at) = date('now');
```

两次聚合查询；`ai_qas` 表有 `user_id` 与 `created_at` 索引。

### 4.2 `GET /api/admin/ai-stats`（管理员）

```json
{
  "total_calls": 431,
  "total_tokens": 152340,
  "total_users": 23,
  "top_users": [
    { "user_id": 1, "username": "alice", "calls": 89, "tokens": 32456 }
  ],
  "daily_7d": [
    { "day": "2026-09-07", "calls": 12, "tokens": 4521 }
  ]
}
```

- `top_users`：`ORDER BY tokens DESC LIMIT 20`，JOIN `users` 拿用户名
- `daily_7d`：`WHERE created_at >= datetime('now', '-7 days') GROUP BY date(created_at)`

### 4.3 `GET /api/admin/ai-config`（管理员，附带全局量）

在原有 `aiConfigGet` 响应里追加 `global_tokens` 与 `global_calls`，让配置面板不用额外请求就能展示全局用量。

## 5. 前端展示

`web/src/views/AI.vue`：

| 位置 | 展示内容 |
|---|---|
| 提问面板顶部（登录用户） | chip 一行：总提问 / 总 Token / 今日次数·token |
| 回答面板底部 | `本次消耗 N token（prompt X + answer Y）` |
| 历史记录每条右下角 | chip：`N token` |
| 管理员配置面板顶部 | chip 一行：全局提问 / 全局 Token / 活跃用户 |
| 管理员配置面板中部 | 表格：Token 消耗 Top 用户（近 7 天） |
| 管理员配置面板底部 | 表格：近 7 天趋势 |

提交 AI 提问成功后自动刷新 `myStats`；页面加载时也各刷一次。

## 6. 保留期与统计的关系

- 记录保留 7 天，超时被 `purgeExpiredAIQAs` 清理
- 因此 `total_calls` / `total_tokens` 实际反映的是「近 7 天累计」，不是历史全量
- 若需要长期配额展示，需在 `purge` 前把聚合结果同步到独立的 `user_stats` 表（当前未实现）
- 前端文案「保留 7 天」已明确告知用户

## 7. 边界与已知问题

- **未配置 Token 时**：`aiAsk` 直接返回 503，不写库，`stats` 不受影响
- **AI 服务调用失败（502）**：不写库
- **估算偏乐观**：`total_tokens` 通常略低于上游实际消耗，用于配额展示足够；用于计费请接入 usage 回调
- **无并发锁**：`SUM` 在 SQLite 事务级别可见，多写多读无竞争

## 8. 验证

`verify_ai.py` 第 [17] 节 24 条断言：

- 未提问前 `total_calls=0` / `total_tokens=0`
- 手动插入一条 `(100, 50, 150)` 后，`total_calls >= 1`、`total_tokens >= 150`
- 管理员端点 `top_users` / `daily_7d` 是数组
- 普通用户访问 admin 端点返回 403
- `ai-config` 响应包含 `global_tokens` / `global_calls`
- 静态检查：`AI.vue` 包含 `/api/ai/stats`、`/api/admin/ai-stats`、`total_tokens`、`today_tokens`；后端包含 `estimateTokens` / `aiStats` / `aiAdminStats`

运行：

```bash
python3 verify_ai.py  # 期望 116/0
```
