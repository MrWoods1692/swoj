# SWOJ 文档

本目录收录 SWOJ（School Wall Online Judge，简称 swoj）的全部模块文档。项目根目录的 [`README.md`](../README.md) 是入口，负责快速开始、部署与目录结构。

## 文档列表

| 文档 | 内容 |
|---|---|
| [`README.md`](../README.md) | 项目简介、快速开始、环境配置、权限模型、部署指南、验证脚本一览 |
| [`ARCHITECTURE.md`](ARCHITECTURE.md) | 系统架构：中间件链、分层、权限模型、数据一致性口径 |
| [`API.md`](API.md) | 全部对外 API 一览，按业务模块分组，标注权限 |
| [`AI.md`](AI.md) | AI 问答模块：接口规格、Token 估算、文件上传、7 天保留 |
| [`AI_STATS.md`](AI_STATS.md) | AI Token 用量统计：字段、估算规则、查询端点、前端展示 |
| [`DEV.md`](DEV.md) | 开发指南：本地启动、schema 变更、加路由/handler/页面/验证脚本 |
| [`DEPLOY.md`](DEPLOY.md) | 部署指南：systemd、nginx、备份、升级、健康检查 |
| [`schema.md`](schema.md) | 数据库表结构总览：SQLite、四份 schema 文件的表清单 |
| [`frontend.md`](frontend.md) | 前端架构：Vue 3 + Vite、目录结构、状态管理、路由、i18n、构建 |
| [`judge.md`](judge.md) | Judge 集成：go-judge 子进程、C++17、状态码、队列 |
| [`auth.md`](auth.md) | 认证与用户：校园墙 OAuth、users 表、角色、Cookie |
| [`problems.md`](problems.md) | 题库模块：problems 表、多语言题干、CRUD |
| [`submissions.md`](submissions.md) | 提交/测评：submissions 表、状态码、队列、go-judge 集成 |
| [`discussions.md`](discussions.md) | 讨论/评论/点赞：discussions 表 |
| [`contests.md`](contests.md) | 训练/比赛/作业：training_plans、contests、assignments |
| [`points.md`](points.md) | 积分/等级/商城：point_records、achievements、shop_items |
| [`logs.md`](logs.md) | 日志系统：operation_logs、action 字典、聚合查询、保留策略 |
| [`admin.md`](admin.md) | 管理后台：requireAdminClaims、教师/管理员/super 权限区分 |
| [`leaderboard.md`](leaderboard.md) | 排行榜：/api/leaderboard、/api/contests/{id}/rank |
| [`favorites.md`](favorites.md) | 收藏与错题本：favorites、wrong_questions |
| [`shop.md`](shop.md) | 商城：shop_items、shop_orders |
| [`notices.md`](notices.md) | 公告：notices、pinned/top |
| [`materials.md`](materials.md) | 资料：materials、category 字典、downloads |
| [`changelog.md`](changelog.md) | 更新日志：changelogs 表与 kinds 字典 |
| [`terms.md`](terms.md) | 服务条款：terms 表、users.terms_accepted_at |
| [`rest_reminder.md`](rest_reminder.md) | 休息提醒：localStorage + /api/points/online 双写 |
| [`status.md`](status.md) | 服务状态与队列：nodes、/api/status、/api/judge/info |
| [`nodes.md`](nodes.md) | Judge 节点：nodes 表与 /api/admin/nodes CRUD |
| [`ip_blocks.md`](ip_blocks.md) | IP 黑名单：ip_blocks 表与 /api/admin/ip-blocks |
| [`config.md`](config.md) | 全局配置：admin_configs 键值表与 /api/admin/config |
| [`notes.md`](notes.md) | 个人笔记：仅本人可见的私密备忘 |
| [`proposals.md`](proposals.md) | 学生出题：题目草稿 → 审核 → 入题库 + 发积分 |
| [`export.md`](export.md) | 比赛成绩表导出：csv/tsv/json/markdown/html/xlsx 六种格式 |

## 相关资源

- 源码：`internal/app/router.go` 是全部路由的单一真源；`docs/API.md` 由它派生
- 数据库 schema：`internal/app/schema_core.go` / `schema_domain.go` / `schema_ops.go` / `schema_points.go`
- 验证脚本：`verify_*.py`，每份脚本对应一个业务模块（详见 `README.md` 的「验证脚本」章节）

## 文档规范

- 文档以 Markdown 编写，中文为主；技术术语保留英文原形
- 接口文档统一格式：`METHOD /path 权限 说明`
- 权限取值：`pub` / `pri` / `teacher` / `admin`
- 新增模块时必须同步更新本文档索引
- 新增路由必须同步更新 `docs/API.md`；新增 AI 相关行为必须同步更新 `docs/AI.md`
