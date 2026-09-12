package app

// NewServer 构建服务并初始化测评队列。
func NewServer(cfg *Config, db *DB, judgeDir string) *Server {
	judge := NewJudge(db, &cfg.Judge, cfg.Points, judgeDir)
	return &Server{
		cfg:   cfg,
		db:    db,
		AI:    &cfg.AI,
		Judge: &cfg.Judge,
		Queue: NewJudgeQueue(db, &cfg.Judge, judge),
	}
}
