# 服务状态与测评队列模块

## 数据表

```sql
CREATE TABLE nodes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  host TEXT NOT NULL,
  port INTEGER DEFAULT 9801,
  enabled INTEGER DEFAULT 1,
  load INTEGER DEFAULT 0,          -- 当前负载（队列数）
  last_heartbeat DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 队列无持久表；内存维护，见 queue.go
```

## 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/status` | pub | 服务状态（后端版本、judge 状态、时间） |
| GET | `/api/judge/info` | pub | Judge 引擎信息（版本、算法） |
| GET | `/api/admin/queue` | admin | 当前测评队列 |
| GET | `/api/admin/stats` | admin | 后台统计（含按日趋势） |
| GET | `/api/admin/nodes` | admin | 节点列表 |
| POST | `/api/admin/nodes` | admin | 创建节点 |
| PUT | `/api/admin/nodes/{id}` | admin | 更新节点 |
| DELETE | `/api/admin/nodes/{id}` | admin | 删除节点 |
| GET | `/api/admin/judge/config` | pri | 读取 judge 配置 |
| PUT | `/api/admin/judge/config` | admin | 保存 judge 配置 |

## 请求示例

```
GET /api/status
```

响应：

```json
{
  "backend_version": "0.1.0",
  "judge_bin": "/usr/local/bin/go-judge",
  "judge_available": true,
  "workers": 4,
  "queue_size": 12,
  "queue_max": 64,
  "server_time": "2026-09-13T03:20:00Z",
  "uptime_seconds": 7200
}
```

## 队列机制

`queue.go` 实现：

```go
type queue struct {
    ch chan *Submission
    workers int
}

func (q *queue) run(s *Server) {
    for i := 0; i < q.workers; i++ {
        go func() {
            for sub := range q.ch {
                s.runJudge(sub)
            }
        }()
    }
}
```

- 并发 worker 数：`SWOJ_WORKERS`（默认 4）
- 队列容量：`SWOJ_QUEUE_SIZE`（默认 64）
- 队列满：返回 503「系统繁忙」
- 单个测评超时：5s（写死在 judge.go）

## 节点

节点是分布测评的扩展点；当前实现是「单 judge 二进制 + 内存队列」，多节点仅做节点注册，实际调度未实现。

- 节点通过 `last_heartbeat` 判断存活（< 60s 视为在线）
- 关闭节点不参与调度；删除节点不影响历史提交

## 业务规则

- 未配置 `SWOJ_JUDGE_BIN`：`judge_available=false`；提交返回 503
- Judge 版本通过 `go-judge --version` 获取，每次启动时读取一次
- 队列数据仅存于内存；服务重启会丢失「测评中」状态（提交状态保持为 pending，可重试）

## 权限

- 公开：`/api/status`、`/api/judge/info`
- 登录用户：读 judge 配置
- 管理员：保存 judge 配置、读队列/统计、管理节点

## 前端

`web/src/views/Status.vue` — 展示后端状态与队列实时负载；`web/src/views/Admin.vue` 内嵌节点管理与队列监控面板。

## 验证

暂无独立 verify；`verify_logs.py` 覆盖 `api:GET /api/status` 中间件日志。
