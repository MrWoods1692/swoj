# 积分 / 等级 / 商城模块

## 数据表

```sql
CREATE TABLE point_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  delta INTEGER NOT NULL,        -- + 或 -
  reason TEXT NOT NULL,          -- 如 "ac_submission", "admin_grant"
  ref_type TEXT DEFAULT '',
  ref_id INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE achievements (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT UNIQUE,              -- 如 "first_ac", "level_10"
  name TEXT NOT NULL,
  description TEXT,
  points INTEGER DEFAULT 0,
  condition TEXT,                -- JSON 或表达式
  icon TEXT
);

CREATE TABLE user_achievements (
  user_id INTEGER NOT NULL,
  achievement_id INTEGER NOT NULL,
  unlocked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(user_id, achievement_id)
);

CREATE TABLE shop_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  description TEXT,
  icon TEXT,
  price INTEGER NOT NULL,
  category TEXT,
  active INTEGER DEFAULT 1,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE shop_orders (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  item_id INTEGER NOT NULL,
  price INTEGER NOT NULL,
  status INTEGER DEFAULT 0,      -- 0=paid
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE checkins (
  user_id INTEGER NOT NULL,
  date TEXT NOT NULL,            -- YYYY-MM-DD
  points INTEGER DEFAULT 1,
  PRIMARY KEY(user_id, date)
);

CREATE TABLE online_sessions (
  user_id INTEGER NOT NULL,
  started_at DATETIME,
  ended_at DATETIME
);
```

## 接口

```
pub  GET  /api/points/rules                 积分规则字典（公开）
pub  GET  /api/points/rank                  积分排行榜
pri  GET  /api/points                       我的积分详情
pri  GET  /api/points/log                   积分收支流水
pri  GET  /api/points/checkin               签到状态
pri  POST /api/points/checkin               签到（每日一次）
pri  POST /api/points/online                在线心跳（前端每 30s 上报）
pri  GET  /api/points/online                在线时长汇总
```

```
pub  GET  /api/levels                       等级表
pri  GET  /api/achievements                 成就列表（含我的解锁状态）
pri  GET  /api/levels/me                    我的等级
pri  POST /api/levels/buy                   购买装扮（扣积分）
```

```
pub  GET  /api/shop                         商品列表
pri  POST /api/shop/redeem                  下单兑换
pri  GET  /api/shop/orders                  我的订单
```

```
pri  POST /api/admin/points/adjust          调整用户积分（admin）
pri  POST /api/admin/points/grant           发放积分（附原因）
pri  POST /api/admin/points/contest/{id}/rule   设置比赛积分规则
pri  GET  /api/admin/points/contest/{id}/rule   读取比赛积分规则
pri  POST /api/admin/points/contest/{id}/apply  应用比赛积分规则
pri  GET  /api/admin/shop                   商品列表（admin）
pri  POST /api/admin/shop                   创建商品（admin）
pri  PUT  /api/admin/shop/{id}              更新商品（admin）
pri  DELETE /api/admin/shop/{id}            删除商品（admin）
```

## 积分规则

`/api/points/rules` 返回：

| 行为 | 积分 |
|---|---|
| 提交 | +1 |
| AC 提交 | +10 |
| 每日签到 | +1 |
| 累计在线 1 小时 | +2 |
| 讨论点赞收到 | +2 |
| 管理员发放 | 指定值 |
| 购买装扮 | -price |

## 请求示例

### 签到

```
POST /api/points/checkin
```

响应：`{ "date": "2026-09-13", "points": 1, "total": 42 }`

### 购买装扮

```json
POST /api/levels/buy
{ "item_id": 5 }
```

响应：`{ "level_id": 5, "remaining": 38 }`

### 下单兑换

```json
POST /api/shop/redeem
{ "item_id": 3 }
```

## 业务规则

- 每日签到只能一次；重复返回当前状态，不重复加分
- 在线时长通过 `POST /api/points/online` 每 30s 上报一次；连续 5 分钟无心跳则视为离线
- 积分余额 < price → 400「积分不足」
- 管理员发放积分：写一条 `reason="admin_grant"` 的流水
- 管理员调整积分：写一条 `reason="admin_adjust"` 的流水
- 交易事务性：下单 + 扣积分在同一事务

## 权限

- 公开：读规则、读排行榜、读商品列表
- 登录用户：所有写操作
- 管理员：调整积分、发放积分、管理商品、设置比赛规则

## 前端

`web/src/views/Points.vue`、`Shop.vue`、`Levels.vue`、`Achievements.vue`；用户头像旁显示当前等级徽章。

## 验证

暂无独立 `verify_points.py`；`verify_logs.py` 覆盖 `points_adjust` / `shop_redeem` 语义日志。
