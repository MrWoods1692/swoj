package app

import "time"

// NewServer 构建服务并初始化测评队列与限流器。
func NewServer(cfg *Config, db *DB, judgeDir string) *Server {
	judge := NewJudge(db, &cfg.Judge, judgeDir)
	return &Server{
		cfg:         cfg,
		db:          db,
		RateLimiter: NewRateLimiter("login", 10, time.Minute),
		AI:          &cfg.AI,
		Judge:       &cfg.Judge,
		Queue:       NewJudgeQueue(db, &cfg.Judge, judge),
	}
}
