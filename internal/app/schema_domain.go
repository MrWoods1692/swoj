package app

// schemaDomain 比赛、作业、训练、讨论、错题、配置与 AI。
var schemaDomain = []string{
	`CREATE TABLE IF NOT EXISTS contests (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  info TEXT DEFAULT '',
  rank INTEGER DEFAULT 1,
  visible INTEGER DEFAULT 1,
  password TEXT DEFAULT '',
  start_time DATETIME NOT NULL,
  end_time DATETIME NOT NULL,
  creator INTEGER DEFAULT 0,
  accept INTEGER DEFAULT 0,
  submit INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS contest_problems (
  contest_id INTEGER NOT NULL, problem_id INTEGER NOT NULL, order_no INTEGER DEFAULT 0,
  status INTEGER DEFAULT 0, PRIMARY KEY(contest_id, problem_id)
)`,
	`CREATE TABLE IF NOT EXISTS contest_users (
  contest_id INTEGER NOT NULL, user_id INTEGER NOT NULL,
  PRIMARY KEY(contest_id, user_id)
)`,
	`CREATE TABLE IF NOT EXISTS assignments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  info TEXT DEFAULT '',
  visible INTEGER DEFAULT 1,
  deadline DATETIME NOT NULL,
  creator INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS assignment_problems (
  assignment_id INTEGER NOT NULL, problem_id INTEGER NOT NULL, order_no INTEGER DEFAULT 0,
  status INTEGER DEFAULT 0, PRIMARY KEY(assignment_id, problem_id)
)`,
	`CREATE TABLE IF NOT EXISTS training_plans (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  info TEXT DEFAULT '',
  visible INTEGER DEFAULT 1,
  creator INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS training_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  plan_id INTEGER NOT NULL,
  problem_id INTEGER NOT NULL,
  problem_name TEXT DEFAULT '',
  sequence INTEGER DEFAULT 0,
  status INTEGER DEFAULT 0,
  accepted INTEGER DEFAULT 0,
  times INTEGER DEFAULT 0,
  UNIQUE(plan_id, problem_id)
)`,
	`CREATE TABLE IF NOT EXISTS discussions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  username TEXT DEFAULT '',
  problem_id INTEGER DEFAULT 0,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  like_count INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS discussion_replies (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  discussion_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  username TEXT DEFAULT '',
  content TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS wrong_questions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  problem_id INTEGER NOT NULL,
  problem_name TEXT DEFAULT '',
  times INTEGER DEFAULT 1,
  accepted INTEGER DEFAULT 0,
  removed INTEGER DEFAULT 0,
  last_try_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(user_id, problem_id)
)`,
	`CREATE TABLE IF NOT EXISTS admin_configs (
  key TEXT PRIMARY KEY,
  value TEXT DEFAULT ''
)`,
	`CREATE TABLE IF NOT EXISTS ai_qas (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  problem_id INTEGER DEFAULT 0,
  question TEXT NOT NULL,
  answer TEXT DEFAULT '',
  source TEXT DEFAULT '',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS recommendation (
  user_id INTEGER NOT NULL,
  problem_id INTEGER NOT NULL,
  score REAL DEFAULT 0,
  recommended_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(user_id, problem_id)
)`,
	`CREATE TABLE IF NOT EXISTS discussion_likes (
  user_id INTEGER NOT NULL, discussion_id INTEGER NOT NULL, PRIMARY KEY(user_id, discussion_id)
)`,
}

// schemaPoints 积分系统：总积分、流水、签到、在线统计、积分商城。
var schemaPoints = []string{
	`CREATE TABLE IF NOT EXISTS checkins (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  date TEXT NOT NULL,
  streak INTEGER NOT NULL DEFAULT 1,
  points INTEGER NOT NULL DEFAULT 0,
  UNIQUE(user_id, date)
)`,
	`CREATE TABLE IF NOT EXISTS points_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  delta INTEGER NOT NULL,
  category TEXT NOT NULL,
  ref_type TEXT DEFAULT '',
  ref_id INTEGER DEFAULT 0,
  remark TEXT DEFAULT '',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS online_stats (
  user_id INTEGER PRIMARY KEY,
  online_seconds INTEGER NOT NULL DEFAULT 0,
  points_awarded INTEGER NOT NULL DEFAULT 0,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS shop_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  description TEXT DEFAULT '',
  price INTEGER NOT NULL,
  stock INTEGER NOT NULL DEFAULT 0,
  status INTEGER NOT NULL DEFAULT 1,
  publisher_id INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS shop_orders (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  item_id INTEGER NOT NULL,
  item_name TEXT DEFAULT '',
  price INTEGER NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS contest_points_rules (
  contest_id INTEGER PRIMARY KEY,
  rule TEXT NOT NULL DEFAULT '{}',
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS user_achievements (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  code TEXT NOT NULL UNIQUE,
  unlocked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(user_id, code)
)`,
	`CREATE TABLE IF NOT EXISTS user_online_days (
  user_id INTEGER NOT NULL,
  date TEXT NOT NULL,
  online_seconds INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(user_id, date)
)`,
}

// schemaOps 运维侧：测评节点、操作日志。
var schemaOps = []string{
	`CREATE TABLE IF NOT EXISTS operation_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER DEFAULT 0,
  username TEXT DEFAULT '',
  action TEXT NOT NULL,
  target TEXT DEFAULT '',
  detail TEXT DEFAULT '',
  ip TEXT DEFAULT '',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS ip_blocks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ip TEXT NOT NULL UNIQUE,
  reason TEXT DEFAULT '',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  expire_at DATETIME
)`,
}
