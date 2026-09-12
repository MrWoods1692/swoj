# 数据库 Schema

## 设计原则

- **纯 SQLite**：`modernc.org/sqlite` 纯 Go 驱动，无 CGO 依赖
- **无历史迁移**：项目未部署，schema 变更直接改 `CREATE TABLE`（见 `schema_core.go:9` 注释）
- **按业务域拆分**：`schema_core.go` / `schema_domain.go` / `schema_ops.go` / `schema_points.go`
- **启动时 migrate**：`bootstrap.go` 中 `migrate` 幂等执行所有 CREATE TABLE

## 表清单

### 核心（`schema_core.go`）

| 表 | 说明 |
|---|---|
| `users` | 用户（见 [auth.md](auth.md)） |
| `problems` | 题目（见 [problems.md](problems.md)） |
| `submissions` | 提交（见 [submissions.md](submissions.md)） |
| `judge_logs` | 每测试点结果（见 [submissions.md](submissions.md)） |

### 业务域（`schema_domain.go`）

| 表 | 说明 |
|---|---|
| `operation_logs` | 操作日志（见 [logs.md](logs.md)） |
| `admin_configs` | 键值配置（见 [admin.md](admin.md)） |
| `ai_qas` | AI 问答（见 [AI.md](AI.md)、[AI_STATS.md](AI_STATS.md)） |
| `discussions` | 讨论（见 [discussions.md](discussions.md)） |
| `discussion_replies` | 讨论回复 |
| `discussion_likes` | 讨论点赞 |
| `favorites` | 收藏（见 [favorites.md](favorites.md)） |
| `wrong_questions` | 错题本（见 [favorites.md](favorites.md)） |
| `recommendation` | 推荐缓存 |

### 运营（`schema_ops.go`）

| 表 | 说明 |
|---|---|
| `training_plans` | 训练计划（见 [contests.md](contests.md)） |
| `training_plan_problems` | 训练计划题目关联 |
| `contests` | 比赛（见 [contests.md](contests.md)） |
| `contest_enrolls` | 比赛报名 |
| `assignments` | 作业（见 [contests.md](contests.md)） |
| `notices` | 公告（见 [notices.md](notices.md)） |
| `materials` | 资料（见 [materials.md](materials.md)） |
| `changelogs` | 更新日志（见 [changelog.md](changelog.md)） |
| `nodes` | Judge 节点（见 [status.md](status.md)） |
| `ip_blocks` | IP 黑名单（见 [admin.md](admin.md)） |
| `terms` | 服务条款（见 [terms.md](terms.md)） |

### 积分（`schema_points.go`）

| 表 | 说明 |
|---|---|
| `point_records` | 积分流水（见 [points.md](points.md)） |
| `achievements` | 成就定义 |
| `user_achievements` | 用户成就解锁 |
| `shop_items` | 商城商品（见 [shop.md](shop.md)） |
| `shop_orders` | 商城订单 |
| `checkins` | 签到记录 |
| `online_sessions` | 在线会话 |

## 变更流程

1. 修改 `schema_*.go` 中的 `CREATE TABLE`
2. 重启服务 → `migrate` 自动执行
3. 本地有测试库 → `rm -rf $SWOJ_DATA_DIR` 后重启

## 索引规范

- 每个 `user_id` 外键字段都有索引（如 `idx_logs_user`）
- 时间戳字段用于过滤的加索引（如 `idx_logs_created`）
- 主键字段不加额外索引（SQLite 已建）
- 复合唯一键用 `UNIQUE(...)`；`PRIMARY KEY(...)` 仅用于真正的主键

## 事务边界

- AI 问答：写 `ai_qas` + 写 `point_records`（如触发积分）→ 同一事务
- 商城下单：扣积分 + 写 `shop_orders` + 写 `point_records` → 同一事务
- 提交：入队列（内存）→ judge 完成后再写 `submissions` + `point_records`（异步事务）

## 备份

SQLite 文件即 `/opt/swoj/data/swoj.db`；在线备份：

```bash
sqlite3 /opt/swoj/data/swoj.db ".backup '/opt/swoj/backups/swoj-$(date +%F).db'"
```
