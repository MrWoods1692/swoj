package app

import (
	"database/sql"
	"fmt"
	"strings"
)

// migrate 创建全部数据表。按业务域拆分，便于独立演进。
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
	return ensureColumns(conn)
}

// ensureColumns 给已存在的旧表补上新增列。SQLite 不支持 IF NOT EXISTS，需先查列。
func ensureColumns(conn *sql.DB) error {
	cols := map[string]string{
		"users": "points INTEGER NOT NULL DEFAULT 0",
	}
	for table, spec := range cols {
		if hasColumn(conn, table, spec) {
			continue
		}
		if _, err := conn.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + spec); err != nil {
			return fmt.Errorf("alter %s: %w", table, err)
		}
	}
	return nil
}

// hasColumn 判断表中是否已有该列。从 ALTER 语句里取列名。
func hasColumn(conn *sql.DB, table, addCol string) bool {
	name := strings.Fields(addCol)[0]
	var got string
	rows, err := conn.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		_ = rows.Scan(&got)
		if got == name {
			return true
		}
	}
	return false
}

// schemaCore 用户、题目、提交、测评。
var schemaCore = []string{
	`CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  password TEXT NOT NULL,
  email TEXT DEFAULT '',
  realname TEXT DEFAULT '',
  role TEXT DEFAULT 'guest',
  school TEXT DEFAULT '',
  avatar TEXT DEFAULT '',
  signature TEXT DEFAULT '',
  problem_count INTEGER DEFAULT 0,
  rank_no INTEGER DEFAULT 0,
  can_submit INTEGER DEFAULT 1,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  last_login_at DATETIME DEFAULT CURRENT_TIMESTAMP
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
