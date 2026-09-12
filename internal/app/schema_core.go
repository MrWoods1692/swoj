package app

import (
	"database/sql"
	"fmt"
)

// migrate 创建全部数据表。按业务域拆分，便于独立演进。
// 项目未部署，不存在历史库：schema 变更直接改 CREATE，不做 ALTER 兼容。
func migrate(conn *sql.DB) error {
	schema := append([]string{}, schemaCore...)
	schema = append(schema, schemaDomain...)
	schema = append(schema, schemaOps...)
	schema = append(schema, schemaPoints...)
	for _, t := range schema {
		if _, err := conn.Exec(t); err != nil {
			return fmt.Errorf("create table: %w", err)
		}
	}
	return nil
}

// schemaCore 用户、题目、提交、测评。
var schemaCore = []string{
	`CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  email TEXT DEFAULT '',
  realname TEXT DEFAULT '',
  role TEXT DEFAULT 'user',
  school TEXT DEFAULT '',
  avatar TEXT DEFAULT '',
  signature TEXT DEFAULT '',
  oauth_provider TEXT NOT NULL DEFAULT '',
  oauth_id TEXT NOT NULL DEFAULT '',
  oauth_name TEXT DEFAULT '',
  website TEXT DEFAULT '',
  background TEXT DEFAULT '',
  qq TEXT DEFAULT '',
  problem_count INTEGER DEFAULT 0,
  rank_no INTEGER DEFAULT 0,
  can_submit INTEGER DEFAULT 0,
  points INTEGER NOT NULL DEFAULT 0,
  level INTEGER NOT NULL DEFAULT 1,
  terms_accepted_at TEXT DEFAULT '',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  last_login_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(oauth_provider, oauth_id)
)`,
	`CREATE TABLE IF NOT EXISTS problems (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  difficulty TEXT DEFAULT 'Easy',
  problem_type TEXT DEFAULT 'OJ',
  time_limit INTEGER DEFAULT 1000,
  mem_limit INTEGER DEFAULT 256,
  file_limit INTEGER DEFAULT 64,
  stack_limit INTEGER DEFAULT 64,
  open_data INTEGER DEFAULT 0,
  show_tag INTEGER DEFAULT 1,
  show_code INTEGER DEFAULT 1,
  invisible INTEGER DEFAULT 0,
  accept INTEGER DEFAULT 0,
  submit INTEGER DEFAULT 0,
  favorite_count INTEGER DEFAULT 0,
  content TEXT DEFAULT '',
  hint TEXT DEFAULT '',
  hint_time INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS cases (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  problem_id INTEGER NOT NULL,
  index_no INTEGER NOT NULL,
  input TEXT NOT NULL,
  output TEXT NOT NULL,
  UNIQUE(problem_id, index_no)
)`,
	`CREATE TABLE IF NOT EXISTS tags (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE)`,
	`CREATE TABLE IF NOT EXISTS problem_tags (
  problem_id INTEGER NOT NULL, tag_id INTEGER NOT NULL, PRIMARY KEY(problem_id, tag_id)
)`,
	`CREATE TABLE IF NOT EXISTS submissions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  username TEXT DEFAULT '',
  problem_id INTEGER NOT NULL,
  problem_name TEXT DEFAULT '',
  code TEXT NOT NULL,
  mode TEXT DEFAULT 'cpp',
  contest_id INTEGER DEFAULT 0,
  assignment_id INTEGER DEFAULT 0,
  status INTEGER DEFAULT 0,
  msg TEXT DEFAULT '',
  time_used INTEGER DEFAULT 0,
  mem_used INTEGER DEFAULT 0,
  error TEXT DEFAULT '',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS judge_nodes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  status INTEGER DEFAULT 0,
  total_count INTEGER DEFAULT 0,
  accept_count INTEGER DEFAULT 0,
  running INTEGER DEFAULT 0,
  judge_type TEXT DEFAULT 'cpp',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  last_seen DATETIME DEFAULT CURRENT_TIMESTAMP
)`,
}
