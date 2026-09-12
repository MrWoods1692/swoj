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

// OpenDB 打开数据库并按需创建表结构，同时写入默认管理员与内置题目。
func OpenDB(dataDir string, cfg *Config) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	path := filepath.Join(dataDir, "swoj.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// WAL 允许读写并发；busy_timeout 降低多 goroutine 竞争时的锁等待失败。
	conn.SetMaxOpenConns(1)
	if _, err := conn.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`); err != nil {
		return nil, fmt.Errorf("pragma: %w", err)
	}
	if err := migrate(conn); err != nil {
		return nil, err
	}
	if err := seed(conn, cfg); err != nil {
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

// Close 关闭连接。
func (d *DB) Close() error {
	return d.conn.Close()
}
