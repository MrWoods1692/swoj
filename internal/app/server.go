package app

// Server 持有进程内的可变运行状态，供 HTTP 路由与队列消费者共享。
type Server struct {
	cfg *Config
	db  *DB

	RateLimiter *RateLimiter

	AI    *AIConfig
	Judge *JudgeConfig
	Queue *JudgeQueue
}
