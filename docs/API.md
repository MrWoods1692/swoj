# SWOJ API 文档

本文件按业务模块列出 SWOJ 后端全部 112 条对外 API。**权限**列取值：

- `pub`：公开，无需登录
- `pri`：登录用户（JWT + Cookie）；handler 内部可进一步用 `requireTeacherClaims` / `requireAdminClaims` 收敛到教师或管理员

所有请求与响应默认使用 JSON（`Content-Type: application/json`）；仅 `POST /api/ai/ask` 支持 multipart。所有 API 均被 `accessLogMiddleware` 记录到 `operation_logs` 表（含匿名请求，`user_id=0`）。

路由真源：`internal/app/router.go`。

## 认证

```
pub  GET  /auth/campux                    跳转校园墙 OAuth
pub  GET  /auth/campux/callback           OAuth 回调（oauthCallbackPath）
pub  POST /api/auth/logout                退出登录
pub  GET  /api/csrf                       获取匿名 CSRF Token
pri  GET  /api/auth/me                    当前用户信息
pri  PUT  /api/auth/me                    修改本人资料
```

登录后 Cookie 下发 `swoj_token`（JWT，HttpOnly）与 `swoj_csrf`。所有写方法必须带 `X-CSRF-Token` 头（值等于 cookie 里的 `swoj_csrf`）。CSRF 检查在 auth 之前，因此未登录的写方法返回 403 而非 401。

## 服务条款

```
pub  GET  /api/terms                      服务条款详情
pri  POST /api/terms/accept               用户接受条款（首次登录弹窗）
```

## 题库（problems）

```
pub  GET  /api/problems                   列表（分页/搜索/难度/标签）
pub  GET  /api/problems/{id}              详情（含测试点数、样例、讨论数）
pri  POST /api/admin/problems             创建题目（admin）
pri  POST /api/admin/problems/{id}        更新题目（admin）
pri  DELETE /api/admin/problems/{id}      删除题目（admin）
```

## 提交与测评（submissions / judge）

```
pri  POST   /api/submissions              提交代码
pri  GET    /api/submissions              个人提交列表（可按 problem_id/status 过滤）
pri  GET    /api/submissions/{id}         提交详情（含 judge 输出、stderr）
pub  GET    /api/status                   服务状态（后端/前端/judge 版本）
pub  GET    /api/judge/info               Judge 引擎信息
pri  GET    /api/admin/judge/config       读取 judge 配置
pri  PUT    /api/admin/judge/config       保存 judge 配置（admin）
```

仅支持 C++17；未配置 `SWOJ_JUDGE_BIN` 时提交返回 503。

## 训练 / 比赛 / 作业

```
pub  GET  /api/training-plans                  训练计划列表
pub  GET  /api/training-plans/{id}             训练计划详情（含题目组）
pri  POST /api/training-plans                  创建训练计划（admin）

pub  GET  /api/contests                        比赛列表
pub  GET  /api/contests/{id}                   比赛详情
pub  GET  /api/contests/{id}/rank              比赛排行榜
pri  POST /api/contests                        创建比赛（admin）
pri  POST /api/contests/{id}/enroll            报名参加比赛

pub  GET  /api/assignments                     作业列表
pub  GET  /api/assignments/{id}                作业详情
pri  POST /api/assignments                     创建作业（teacher/admin）
```

## 讨论

```
pub  GET  /api/discussions                 列表（可按 problem_id 过滤）
pub  GET  /api/discussions/{id}            详情（含回复）
pri  POST /api/discussions                 创建讨论
pri  POST /api/discussions/{id}/reply      回复
pri  POST /api/discussions/{id}/like       点赞/取消
```

## 收藏 / 错题本 / 笔记

```
pri  POST   /api/favorites/{type}/{id}     切换收藏（type: problem/discussion/...）
pri  GET    /api/favorites/{type}          收藏列表
pri  GET    /api/wrong-questions           错题本
pri  DELETE /api/wrong-questions/{id}      从错题本移除
pri  GET    /api/notes                     我的笔记列表（置顶优先，snippet 截断 120 字）
pri  POST   /api/notes                     新建笔记
pri  GET    /api/notes/{id}                笔记详情（完整正文）
pri  PUT    /api/notes/{id}                更新笔记（含 pinned 切换）
pri  DELETE /api/notes/{id}                删除笔记（硬删除）
```

## 学生出题（问题提议）

```
pri  POST   /api/proposals                          新建提议（pri，作者）
pri  GET    /api/proposals                          我的提议列表（pri，作者）
pri  GET    /api/proposals/{id}                     提议详情（pri，作者或审核员）
pri  PUT    /api/proposals/{id}/withdraw            作者撤回（仅 pending/rejected）
pri  DELETE /api/proposals/{id}                     作者删除（仅 withdrawn/rejected）
pri  GET    /api/admin/proposals                    待审核列表（teacher+）
pri  POST   /api/admin/proposals/{id}/review        审核：通过/拒绝 + 奖励积分（teacher+）
```

## 积分 / 签到 / 在线

```
pub  GET  /api/points/rules                积分规则（公开）
pub  GET  /api/points/rank                 积分排行榜（公开）
pri  GET  /api/points                      我的积分详情
pri  GET  /api/points/log                  积分收支流水
pri  GET  /api/points/checkin              签到状态
pri  POST /api/points/checkin              签到
pri  POST /api/points/online               在线心跳（前端定时上报）
pri  GET  /api/points/online               在线时长汇总
```

## 等级 / 成就 / 商城

```
pub  GET  /api/levels                      等级表
pri  GET  /api/achievements                成就列表
pri  GET  /api/levels/me                   我的等级
pri  POST /api/levels/buy                  购买等级装扮
pub  GET  /api/shop                        商品列表
pri  POST /api/shop/redeem                 下单兑换
pri  GET  /api/shop/orders                 我的订单
```

## 排行榜 / 用户

```
pub  GET  /api/leaderboard                 排行榜（可按 problem 过滤）
pri  GET  /api/auth/me/stats               个人统计（提交/AC/排名）
pub  GET  /api/users/{id}/homepage         用户主页聚合数据（公开）
```

## 服务状态 / 站点统计

```
pub  GET  /api/stats/site                  全站统计（题目/用户/提交数）
pri  GET  /api/admin/stats                 后台统计（含按日趋势）
pri  GET  /api/admin/queue                 当前测评队列（admin）
```

## AI 问答

```
pri  POST /api/ai/ask                      提问（JSON 或 multipart 上传）
pri  GET  /api/ai/history                  个人历史（保留 7 天）
pri  GET  /api/ai/stats                    个人用量统计
pri  GET  /api/admin/ai-config             读取 AI 配置（admin）
pri  PUT  /api/admin/ai-config             保存 AI 配置（admin）
pri  GET  /api/admin/ai-stats              全局用量统计（admin）
```

详细规格见 [`AI.md`](AI.md)，Token 用量统计方案见 [`AI_STATS.md`](AI_STATS.md)。

## 日志系统

```
pri  GET  /api/admin/logs                  全局操作日志（admin，可过滤）
pri  GET  /api/admin/logs/users            日志中的用户 ID → 用户映射（admin）
pri  GET  /api/logs                        个人操作日志
pri  GET  /api/logs/actions                语义动作字典（下拉筛选用）
pri  GET  /api/logs/daily                  按日汇总
```

日志两种记录共存：

| 来源 | action 特征 | path | status_code |
|---|---|---|---|
| `accessLogMiddleware` | `api:<METHOD> <PATH>` | 请求路径 | HTTP 状态码 |
| `logOp`（业务） | 语义如 `discussion_create` | 空 | 0 |

## 公告 / 更新日志 / 资料

```
pub  GET  /api/notices                     公告列表
pub  GET  /api/notices/{id}                公告详情
pri  GET  /api/admin/notices               全部公告（admin）
pri  POST /api/admin/notices               创建公告（admin）
pri  PUT  /api/admin/notices/{id}          更新公告（admin）
pri  DELETE /api/admin/notices/{id}        删除公告（admin）

pub  GET  /api/changelog                   更新日志列表
pub  GET  /api/changelog/kinds             更新日志分类字典
pub  GET  /api/changelog/{id}              更新日志详情
pri  GET  /api/admin/changelog             全部更新日志（admin）
pri  POST /api/admin/changelog             创建（admin）
pri  PUT  /api/admin/changelog/{id}        更新（admin）
pri  DELETE /api/admin/changelog/{id}      删除（admin）

pub  GET  /api/materials/meta              资料分类字典
pub  GET  /api/materials                   资料列表
pub  GET  /api/materials/{id}              资料详情
pri  GET  /api/admin/materials             全部资料（teacher/admin）
pri  POST /api/admin/materials             创建（teacher/admin）
pri  PUT  /api/admin/materials/{id}        更新（teacher/admin）
pri  DELETE /api/admin/materials/{id}      删除（teacher/admin）
```

## 管理后台：用户 / 节点 / IP 黑名单 / 配置

```
pri  GET    /api/admin/users               用户列表（admin）
pri  PUT    /api/admin/users/{id}          修改用户角色/禁用（admin）
pri  GET    /api/admin/nodes               节点列表（admin）
pri  POST   /api/admin/nodes               创建节点
pri  PUT    /api/admin/nodes/{id}          更新节点
pri  DELETE /api/admin/nodes/{id}          删除节点
pri  GET    /api/admin/ip-blocks           IP 黑名单列表
pri  POST   /api/admin/ip-blocks           加入 IP 黑名单
pri  DELETE /api/admin/ip-blocks/{id}      移除
pri  GET    /api/admin/config              全局配置（键值）
pri  POST   /api/admin/config              保存配置
```

## 管理后台：积分 / 商城

```
pri  POST   /api/admin/points/adjust                    调整用户积分
pri  POST   /api/admin/points/grant                     发放积分（附原因）
pri  POST   /api/admin/points/contest/{id}/rule        设置比赛积分规则
pri  GET    /api/admin/points/contest/{id}/rule        读取比赛积分规则
pri  POST   /api/admin/points/contest/{id}/apply       应用比赛积分规则
pri  GET    /api/admin/shop                            商品列表（admin）
pri  POST   /api/admin/shop                            创建商品（admin）
pri  PUT    /api/admin/shop/{id}                       更新商品（admin）
pri  DELETE /api/admin/shop/{id}                       删除商品（admin）
```

## 通用错误码

| HTTP | 语义 |
|---|---|
| 400 | 请求参数错误（字段校验失败） |
| 401 | 未登录（GET 请求，缺 token） |
| 403 | 未登录（写方法，CSRF 拦截）或权限不足 |
| 404 | 资源不存在 |
| 409 | 冲突（如重复提交） |
| 413 | 请求体过大（文件上传超限） |
| 502 | 上游服务调用失败（如 AI 提供商） |
| 503 | 服务未启用（未配置 judge 或 AI Token） |

## 请求 / 响应格式

**成功**：

```json
{ "code": 0, "data": { ... } }
```

**失败**：

```json
{ "code": 400, "msg": "错误描述" }
```

**分页**：所有列表接口支持 `?page=1&page_size=20`；响应含 `total`。

**排序**：`?sort=asc|desc` 与 `?order_by=created_at|score|...`。

## OpenAPI 说明

项目未提供 OpenAPI/Swagger 自动生成；本文档由人工维护，新增路由后请同步更新。
