# AI 问答模块文档

本文档描述 SWOJ 的 AI 编程问答模块：接口规格、Token 用量统计、文件上传、权限模型、7 天保留策略。

## 1. 总体设计

```
浏览器 ── /api/ai/ask (multipart) ──┐
                                    │
                                    ▼
                             handler_leaderboard.aiAsk
                                    │
                     ┌──────────────┼──────────────┐
                     │              │              │
                     ▼              ▼              ▼
             权限校验          请求校验          Token 校验
         (requireClaims)    (文件白名单/大小)  (aiConfigGet)
                                    │
                                    ▼
                        ai_client.callAI (Provider 分派)
                                    │
              ┌─────────────────────┼─────────────────────┐
              ▼                                                 ▼
        yunzhiapi.cn (默认)                          OpenAI 兼容 API
              │
              ▼
     GET /API/deepseek.php?token=&question=&system=&type=text
              │
              ▼
     estimateTokens(prompt) + estimateTokens(answer)
              │
              ▼
     INSERT ai_qas (prompt_tokens, answer_tokens, total_tokens)
```

- 管理员通过「管理后台 → AI 配置」保存 Token 到 `admin_configs(ai.token)`
- 每次问答把 prompt/answer 的 token 估算写入 `ai_qas`
- 记录保留 7 天；启动时清理 + 每 6 小时循环清理

## 2. 数据模型

```sql
CREATE TABLE ai_qas (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id         INTEGER NOT NULL,
  problem_id      INTEGER DEFAULT 0,
  question        TEXT NOT NULL,
  answer          TEXT DEFAULT '',
  source          TEXT DEFAULT '',
  prompt_tokens   INTEGER DEFAULT 0,
  answer_tokens   INTEGER DEFAULT 0,
  total_tokens    INTEGER DEFAULT 0,
  created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

- `source`：AI 提供商名（如 `yunzhi` / `deepseek`）
- `total_tokens = prompt_tokens + answer_tokens`
- 保留期：`created_at >= datetime('now', '-7 days')`

## 3. Token 估算规则

`estimateTokens(s)` 在 `handler_leaderboard.go` 中实现：

| 字符类型 | 每字 token |
|---|---|
| CJK 表意文字（U+4E00–U+9FFF） | 1 |
| CJK 符号和标点（U+3000–U+30FF） | 1 |
| 谚文音节（U+AC00–U+D7AF） | 1 |
| CJK 扩展 A（U+3400–U+4DBF） | 1 |
| 空白字符 | 0 |
| 其他（ASCII、其他非 CJK） | 每 4 字符 1 |

对中英文混合的编程问答足够近似；精确计量由上游服务商回调提供（当前未接入）。

## 4. 文件上传规格

**允许扩展名**（26 种）：

```
.cpp .cxx .cc .c .h .hpp .hxx
.txt .in .out .md
.py .java .js .ts .go .rs .rb
.sh .sql .json .xml .yml .yaml .csv .log
```

**限制**：

| 项 | 上限 |
|---|---|
| 单文件大小 | 1 MB |
| 文件数量 | 5 个 |
| 表单总大小 | 3 MB |

**请求**（multipart）：

```
POST /api/ai/ask
Content-Type: multipart/form-data
Fields:
  question      (可选)  纯文本提问
  code          (可选)  代码文本
  problem_id    (可选)  关联题目 ID
  status        (可选)  当前判题状态
  files[]       (可选)  附件，字段名必须是 `files[]`（复数）
```

文件内容按顺序拼接进 prompt；无 `question` 但有文件时，文件内容作为提问主体。

## 5. 接口

### 5.1 `POST /api/ai/ask`

- **权限**：登录用户
- **返回**：`{ id, answer, source, prompt_tokens, answer_tokens, total_tokens }`
- **错误**：
  - 400：文件类型不支持 / 单文件过大 / 文件过多 / 问题与附件全空
  - 401：未登录
  - 413：表单过大
  - 502：AI 服务调用失败
  - 503：AI 服务未启用或 Token 未配置

### 5.2 `GET /api/ai/history`

- **权限**：登录用户
- **返回**：该用户最近 50 条、7 天内的记录

### 5.3 `GET /api/ai/stats`

- **权限**：登录用户
- **返回**：

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

### 5.4 `GET /api/admin/ai-stats`（管理员）

- **权限**：`super` / `admin`
- **返回**：

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

### 5.5 `GET /api/admin/ai-config`（管理员）

- **返回**：`{ enabled, provider, yunzhi_url, token_masked, has_token, system_prompt, default_prompt, retention_days, global_tokens, global_calls }`
- Token 掩码规则：长度 < 12 → 全部打星；否则前 4 + `****` + 后 4

### 5.6 `PUT /api/admin/ai-config`（管理员）

- **请求**：`{ token, provider, system_prompt, clear_token }`
- **语义**：
  - `token` 非空 → 保存新 token 并启用
  - `token` 空 + `clear_token=true` → 清除 token
  - `token` 空 + `clear_token=false` → 不修改（避免管理员只想更新提示词却误清 token）
  - `system_prompt` 空 → 使用内置默认提示词
- **校验**：`provider` 仅 `yunzhi` 或 `openai`；`token` ≤ 256 字；`system_prompt` ≤ 4000 字

## 6. 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `SWOJ_AI_ENABLED` | `1` | 总开关；`0` 时所有 `/api/ai/ask` 返回 503 |
| `SWOJ_AI_PROVIDER` | `yunzhi` | `yunzhi` / `openai` |
| `SWOJ_AI_URL` | `https://yunzhiapi.cn/API/deepseek.php` | yunzhi 接口 |
| `SWOJ_AI_TOKEN` | 空 | yunzhi Token（推荐由管理后台保存，覆盖此环境变量） |
| `SWOJ_AI_SYSTEM_PROMPT` | 内置 | 模型系统提示词 |
| `SWOJ_AI_OPENAI_URL` | 空 | OpenAI 兼容 URL |
| `SWOJ_AI_OPENAI_KEY` | 空 | OpenAI API Key |
| `SWOJ_AI_MODEL` | `deepseek-chat` | 模型名（仅 OpenAI 用） |
| `SWOJ_AI_MAX_TOKENS` | `4096` | 最大返回 token |
| `SWOJ_AI_TEMP` | `0.7` | Temperature |

优先级：**管理后台 admin_configs** > 环境变量 > 默认值。

## 7. 保留期清理

`bootstrap.go` 中：

```go
const aiRetentionDays = 7
const aiRetentionDuration = time.Duration(aiRetentionDays) * 24 * time.Hour

func (s *Server) aiPurgeLoop() {
    s.purgeExpiredAIQAs()
    t := time.NewTicker(6 * time.Hour)
    for range t.C { s.purgeExpiredAIQAs() }
}

func (s *Server) purgeExpiredAIQAs() {
    cutoff := time.Now().Add(-aiRetentionDuration)
    _, _ = s.db.Exec(`DELETE FROM ai_qas WHERE created_at < ?`, cutoff)
}
```

启动时立即清理一次，然后每 6 小时循环。

## 8. 前端入口

`web/src/views/AI.vue`：

- 顶部：附件上传、关联题目、当前判题状态、代码、问题
- 中间：登录用户显示个人今日/总消耗 chip
- 回答面板：显示本次消耗 token（prompt + answer）
- 管理员配置面板：全局统计、Top 用户、近 7 天趋势、Token 输入、提示词
- 右侧：历史记录，每条显示 token 数

## 9. 验证

`verify_ai.py`（116 条断言）覆盖：

- 权限（匿名、普通用户、管理员、首位 super）
- Token 掩码与不泄露完整 token
- 边界校验（token 长度、system_prompt 长度、非法 provider）
- 文件上传（.cpp/.txt/.in/.out 白名单、.exe 拒绝、> 1MB 拒绝、> 5 个拒绝）
- 7 天保留：手动插入 8 天前的记录，验证被清理
- Token 用量统计：字段存在、加总正确、管理员/普通用户权限
- 静态检查：前后端代码包含关键字段

运行：

```bash
rm -rf /tmp/swoj-ai && SWOJ_HOST_PORT=127.0.0.1:18080 \
SWOJ_DATA_DIR=/tmp/swoj-ai python3 verify_ai.py
```

## 10. 常见问题

**Q: AI 服务返回 502 是什么？**
A: 通常是 Token 无效或上游超时。`ai_client.go` 把 yunzhi 的 `500` 状态码视为超时并返回 `AI 服务调用失败`。

**Q: 为什么我的 Token 显示 `****`？**
A: 短于 12 位的 token 全部打星；更长则前 4 + `****` + 后 4。完整 token 只在 DB 中。

**Q: 为什么历史记录只有 7 天？**
A: 设计如此；`aiRetentionDays = 7`。修改需同时改 `bootstrap.go` 常量与前端文案。

**Q: Token 估算为什么和上游计费不一致？**
A: 我们用字符近似，上游按 BPE 分词；量级接近但不完全一致。精确计量需要接入上游的 usage 回调，当前未实现。
