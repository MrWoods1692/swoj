# 休息提醒模块

## 设计目标

- 用户累计在线满 1 小时 → 前端弹窗提醒
- 用户点击「已休息」→ 重置计时
- 数据本地存（浏览器 localStorage）+ 后端心跳（`/api/points/online`）双写，避免本地刷新丢失

## 数据

```sql
-- 无独立表；在线时长通过 online_sessions 汇总（见 points.md）
CREATE TABLE online_sessions (
  user_id INTEGER NOT NULL,
  started_at DATETIME,
  ended_at DATETIME
);
```

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/rest-reminder/status` | pri | 在线时长与提醒状态 |
| POST | `/api/rest-reminder/ack` | pri | 确认已休息（重置计时） |

## 请求示例

```
GET  /api/rest-reminder/status
POST /api/rest-reminder/ack
```

## 响应示例

```json
{
  "online_minutes": 58,
  "should_remind": false,
  "last_ack_at": "2026-09-13 02:20:00",
  "reminder_interval_minutes": 60
}
```

`should_remind=true` 时前端弹窗；`ack` 之后 `online_minutes` 归零。

## 业务规则

- 计时基于「最后一次 ack 到当前」的时间差
- 无 ack 记录 → 从用户首次访问后端那一刻算起
- 心跳：前端每 30s 调用 `POST /api/points/online`；连续 5 分钟无心跳视为离线，不累计时长
- 计时不影响积分规则，仅用于前端提醒

## 权限

- 登录用户

## 前端

`web/src/components/RestReminder.vue` — 监听 `GET /status` 轮询（每 60s）；弹窗阻止页面操作直到用户 ack 或关闭。

## 验证

`verify_rest_reminder.py` — 54 条断言，覆盖：

- 未登录 → 401
- 首次访问 → `online_minutes=0`、`should_remind=false`
- 手动修改 DB 中 `last_ack_at` 为 59 分钟前 → `should_remind=true`
- ack 后 `online_minutes` 归零
- 匿名 GET → 401
