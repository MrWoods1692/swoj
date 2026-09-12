# SWOJ API 文档

本文件按业务模块列出 SWOJ 后端全部对外 API。**权限**列取值：

- `pub`：公开，无需登录
- `login`：登录用户（JWT + Cookie）
- `teacher`：教师或管理员
- `admin`：管理员或超级管理员

完整路由注册见 `internal/app/router.go`（112 条）。所有请求与响应默认使用 JSON（Content-Type: application/json）；仅 `POST /api/ai/ask` 支持 multipart。

所有 API 均被 `accessLogMiddleware` 记录到 `operation_logs` 表（含匿名请求，`user_id=0`）。

## 认证

```
GET  /auth/campux                    pub     跳转校园墙 OAuth
GET  /auth/campux/callback           pub     OAuth 回调
POST /api/auth/logout                pub     退出登录
GET  /api/csrf                       pub     获取匿名 CSRF Token
```

登录完成后 Cookie 会同时下发 `swoj_token`（JWT，HttpOnly）与 `swoj_csrf`。所有写方法必须带 `X-CSRF-Token` 头（值等于 cookie 里的 `swoj_csrf`）。

## 题库（problems）

```
GET    /api/problems                        pub     列表（分页/搜索/难度/标签）
GET    /api/problems/{id}                   pub     详情
POST   /api/problems                        admin   创建
PUT    /api/problems/{id}                   admin   更新
DELETE /api/problems/{id}                   admin   删除
POST   /api/problems/{id}/seed              admin   批量插入测试点
GET    /api/problems/{id}/recommendations   login   相关推荐
```

## 提交与测评（submissions / judge）

```
POST   /api/submissions                     login   提交代码
GET    /api/submissions                     login   个人提交列表
GET    /api/submissions/{id}                login   提交详情（含 judge 输出）
GET    /api/judge/status                    pub     测评服务状态
GET    /api/judge/queue                     pub     当前测评队列
POST   /api/judge/retry/{id}                login   重试提交（限个人/超管）
```

仅支持 C++17；未配置 `SWOJ_JUDGE_BIN` 时提交返回 503。

## AI 问答

```
POST   /api/ai/ask                          login   提问（JSON 或 multipart 上传）
GET    /api/ai/history                      login   个人历史（7 天内）
GET    /api/ai/stats                        login   个人用量统计
GET    /api/admin/ai-config                 admin   读取配置
PUT    /api/admin/ai-config                 admin   保存配置
GET    /api/admin/ai-stats                  admin   全局用量统计
```

详细规格见 `docs/AI.md`。

## 训练 / 比赛 / 作业

```
GET    /api/training-plans                  pub
GET    /api/training-plans/{id}             pub
POST   /api/training-plans                  admin
PUT    /api/training-plans/{id}             admin
DELETE /api/training-plans/{id}             admin
POST   /api/training-plans/{id}/problems    admin   批量加入题目

GET    /api/contests                        pub
GET    /api/contests/{id}                   pub
POST   /api/contests                        admin
PUT    /api/contests/{id}                   admin
DELETE /api/contests/{id}                   admin

GET    /api/assignments                     pub
GET    /api/assignments/{id}                pub
POST   /api/assignments                     teacher
PUT    /api/assignments/{id}                teacher
DELETE /api/assignments/{id}                teacher
```

## 讨论

```
GET    /api/discussions                     pub     列表
GET    /api/discussions/{id}                pub     详情
POST   /api/discussions                     login   创建
DELETE /api/discussions/{id}                teacher 删除（含作者权限）
POST   /api/discussions/{id}/like           login   点赞/取消
POST   /api/discussions/{id}/pin            teacher 置顶/取消
```

## 收藏 / 错题本 / 推荐

```
POST   /api/favorites/{problem_id}          login   收藏
DELETE /api/favorites/{problem_id}          login   取消
GET    /api/favorites                       login   收藏列表

GET    /api/wrong-questions                 login   错题本
DELETE /api/wrong-questions/{id}            login   移除

GET    /api/recommendations                 login   首页推荐
```

## 积分 / 等级 / 商城

```
GET    /api/points/balance                  login   余额
GET    /api/points/records                  login   收支流水
GET    /api/achievements                    pub     成就列表
GET    /api/levels                          pub     等级表

GET    /api/shop                            pub     商品列表
POST   /api/shop/orders                     login   下单兑换

POST   /api/admin/points/adjust             admin   调整余额
POST   /api/admin/points/grant              admin   发放积分
POST   /api/admin/shop/items                admin   管理商品
PUT    /api/admin/shop/items/{id}           admin   更新商品
DELETE /api/admin/shop/items/{id}           admin   删除商品
```

## 排行榜 / 用户

```
GET    /api/leaderboard                     pub     排行榜
GET    /api/users                           admin   用户列表（admin）
GET    /api/users/{id}                      login   用户详情
GET    /api/users/{id}/homepage             pub     个人主页聚合数据
PUT    /api/users/{id}/profile              login   修改本人资料
PUT    /api/admin/users/{id}/role           admin   修改角色
DELETE /api/admin/users/{id}                admin   禁用用户
GET    /api/stats/rank                     pub     全站统计
```

## 日志系统

```
GET    /api/logs                            login   个人日志（含 filter 参数）
GET    /api/admin/logs                      admin   全局日志（可按 user_id/action/date 过滤）
GET    /api/logs/actions                    login   语义动作字典（用于筛选下拉）
GET    /api/logs/daily                      login   每日汇总
```

日志两种记录共存：

| 来源 | action 特征 | path | status_code |
|---|---|---|---|
| `accessLogMiddleware` | `api:<METHOD> <PATH>` | 请求路径 | HTTP 状态码 |
| `logOp`（业务） | 语义如 `discussion_create` | 空 | 0 |

## 公告 / 更新日志 / 服务条款 / 资料

```
GET    /api/notices                         pub
POST   /api/notices                         admin
PUT    /api/notices/{id}                    admin
DELETE /api/notices/{id}                    admin

GET    /api/changelog                       pub
POST   /api/changelog                       admin
PUT    /api/changelog/{id}                  admin
DELETE /api/changelog/{id}                  admin

GET    /api/terms                           pub
PUT    /api/admin/terms                     admin

GET    /api/materials                       pub
POST   /api/materials                       teacher
PUT    /api/materials/{id}                  teacher
DELETE /api/materials/{id}                  teacher
```

## 休息提醒

```
GET    /api/rest-reminder/status            login   在线时长与提醒状态
POST   /api/rest-reminder/ack               login   确认已休息
```

累计在线满 1 小时前端弹出提醒，`ack` 重置计时。

## 通用错误码

| HTTP | 语义 |
|---|---|
| 400 | 请求参数错误（字段校验失败） |
| 401 | 未登录（GET） |
| 403 | 未登录（POST/PUT/DELETE，CSRF 检查在 auth 之前） |
| 404 | 资源不存在 |
| 409 | 冲突（如重复提交） |
| 413 | 请求体过大（文件上传超限） |
| 502 | 上游服务调用失败（如 AI 提供商） |
| 503 | 服务未启用（如未配置 judge 或 AI Token） |

## OpenAPI 说明

项目未提供 OpenAPI/Swagger 自动生成；本文档由人工维护，新增路由后请同步更新。
