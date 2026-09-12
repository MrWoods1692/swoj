package app

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB 封装 SQLite 连接与建表。
type DB struct {
	conn *sql.DB
}

// OpenDB 打开数据库并按需创建表结构，同时写入内置测评节点与题目。
// 不种子任何账号：首位完成校园墙授权的用户自动成为管理员。
func OpenDB(dataDir string, cfg *Config) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	path := filepath.Join(dataDir, "swoj.db")
	// PRAGMA 是连接级的，多连接下必须随 DSN 下发，否则只有建库那条连接生效。
	conn, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// WAL 允许读写并发；保留多个连接是为了让事务可用：
	// 若只开 1 条连接，Begin 会占住唯一连接，同连接上的后续查询必然死锁，
	// 导致写事务内的语句全部静默失败（点赞计数、删题、积分入账都受影响）。
	conn.SetMaxOpenConns(3)
	conn.SetMaxIdleConns(3)
	if _, err := conn.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`); err != nil {
		return nil, fmt.Errorf("pragma: %w", err)
	}
	if err := migrate(conn); err != nil {
		return nil, err
	}
	if err := seed(conn); err != nil {
		return nil, fmt.Errorf("seed: %w", err)
	}
	return &DB{conn: conn}, nil
}

// Exec 执行非查询语句。
func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	return d.conn.Exec(query, args...)
}

// QueryRow 查询单行。
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	return d.conn.QueryRow(query, args...)
}

// Query 查询多行。
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return d.conn.Query(query, args...)
}

// Begin 开启事务，用于计数更新与去重表的原子联动。
func (d *DB) Begin() (*sql.Tx, error) {
	return d.conn.Begin()
}

// Close 关闭连接。
func (d *DB) Close() error {
	return d.conn.Close()
}
