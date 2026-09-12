package app

import (
"context"
"sync"
"time"
)

// JudgeTask 一次待测提交。
type JudgeTask struct {
SubID int64
UserID int64
ProblemID int64
Code    string
}

// JudgeQueue 并发测评队列：固定容量无缓冲派发通道 + 独立 worker 池。
// 提交入队后立即返回，测评在后台进行，避免 HTTP 请求被编译耗时阻塞。
type JudgeQueue struct {
queue chan JudgeTask
db    *DB
cfg   *JudgeConfig
judge *Judge

mu      sync.Mutex
running int
total   int
done    int
stopped bool
}

// NewJudgeQueue 创建队列并启动 worker。
func NewJudgeQueue(db *DB, cfg *JudgeConfig, judge *Judge) *JudgeQueue {
q := &JudgeQueue{
queue: make(chan JudgeTask, cfg.QueueSize),
db:    db,
cfg:   cfg,
judge: judge,
}
for i := 0; i < cfg.Workers; i++ {
go q.worker(i)
}
return q
}

func (q *JudgeQueue) worker(id int) {
for t := range q.queue {
q.mu.Lock()
q.running++
q.total++
q.mu.Unlock()

err := q.judge.Run(t)
if err != nil {
q.fail(t.SubID, "judge_error", err.Error())
}

q.mu.Lock()
q.running--
q.done++
q.mu.Unlock()
}
}

// Enqueue 投递一个待测任务。队列满时返回错误，由调用方决定降级策略。
func (q *JudgeQueue) Enqueue(t JudgeTask) error {
select {
case q.queue <- t:
return nil
default:
return ErrQueueFull
}
}

func (q *JudgeQueue) fail(subID int64, status, msg string) {
_, _ = q.db.Exec(`UPDATE submissions SET status=?, msg=?, error=? WHERE id=?`, status, msg, msg, subID)
}

// Stats 当前队列水位，供服务状态页展示。
func (q *JudgeQueue) Stats() QueueStats {
q.mu.Lock()
defer q.mu.Unlock()
return QueueStats{
Running: q.running, Total: q.total, Done: q.done,
Pending: len(q.queue), Capacity: cap(q.queue),
Workers: q.cfg.Workers,
}
}

// QueueStats 队列监控数据。
type QueueStats struct {
Running  int `json:"running"`
Pending  int `json:"pending"`
Capacity int `json:"capacity"`
Total    int `json:"total"`
Done     int `json:"done"`
Workers  int `json:"workers"`
}

// ErrQueueFull 队列已满。
var ErrQueueFull = &ErrText{msg: "测评队列已满，请稍后再试"}

// ErrText 业务错误载体。
type ErrText struct{ msg string }

func (e *ErrText) Error() string { return e.msg }

// Shutdown 停止队列：关闭派发通道并等待 worker 排空。
func (q *JudgeQueue) Shutdown(ctx context.Context) {
close(q.queue)
_ = ctx
time.Sleep(0)
}
