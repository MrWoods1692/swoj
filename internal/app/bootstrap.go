package app

import "time"

// NewServer 构建服务并初始化测评队列。
//
// 初始化顺序：judge → AI 配置（从 admin_configs 覆盖 token/systemPrompt）→
// 启动 ai_qas 定期清理（超过 7 天的记录删除，满足合规要求）。
func NewServer(cfg *Config, db *DB, judgeDir string) *Server {
	judge := NewJudge(db, &cfg.Judge, cfg.Points, judgeDir)
	s := &Server{
		cfg:   cfg,
		db:    db,
		AI:    &cfg.AI,
		Judge: &cfg.Judge,
		Queue: NewJudgeQueue(db, &cfg.Judge, judge),
	}
	applyAIDBConfig(s)
	purgeExpiredAIQAs(s.db)
	go s.aiPurgeLoop()
	return s
}

// applyAIDBConfig 把 admin_configs 中的 AI 配置读入内存。
// 数据库里的 token/system_prompt 优先级高于环境变量，便于管理员在后台直接配置。
func applyAIDBConfig(s *Server) {
	loadAIConfig(s.db, s.AI)
}

// loadAIConfig 从 admin_configs 表把 AI 相关 key 写入 cfg。
// key: ai.token / ai.provider / ai.system_prompt
func loadAIConfig(db *DB, c *AIConfig) {
	rows, err := db.Query(`SELECT key, value FROM admin_configs WHERE key IN ('ai.token','ai.provider','ai.system_prompt')`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		switch k {
		case "ai.token":
			if v != "" {
				c.YunzhiToken = v
				// 只要 token 有值就视作启用，避免管理员忘改 ENABLED 却配好了 token
				c.Enabled = true
			}
		case "ai.provider":
			if v == "yunzhi" || v == "openai" {
				c.Provider = v
			}
		case "ai.system_prompt":
			if len(v) <= 4000 {
				c.SystemPrompt = v
			}
		}
	}
}

// aiRetentionDays 对话记录的保留期（天）。
const aiRetentionDays = 7

// aiRetentionDuration 记录保留的时长。
var aiRetentionDuration = 24 * time.Hour * aiRetentionDays

// purgeExpiredAIQAs 删除 ai_qas 中超过保留期的记录；启动时立即执行一次。
func purgeExpiredAIQAs(db *DB) {
	_, _ = db.Exec(`DELETE FROM ai_qas WHERE created_at < ?`, time.Now().Add(-aiRetentionDuration))
}

// aiPurgeLoop 每 6 小时清理一次过期 AI 问答记录。
func (s *Server) aiPurgeLoop() {
	t := time.NewTicker(6 * time.Hour)
	defer t.Stop()
	for range t.C {
		purgeExpiredAIQAs(s.db)
	}
}
