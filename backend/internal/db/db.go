package db

import (
	"database/sql"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Open 建立 MySQL 连接并执行建表与种子数据。
func Open(dsn string) (*sql.DB, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(20)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(time.Hour)
	if err := conn.Ping(); err != nil {
		return nil, err
	}
	for _, stmt := range strings.Split(Schema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := conn.Exec(stmt); err != nil {
			return nil, err
		}
	}
	// 现有环境的 votes.target_type ENUM 升级（幂等：仅当缺 'article' 时 ALTER）
	var colType string
	_ = conn.QueryRow(`SELECT COLUMN_TYPE FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='votes' AND COLUMN_NAME='target_type'`).Scan(&colType)
	if colType != "" && !strings.Contains(colType, "'article'") {
		if _, err := conn.Exec(`ALTER TABLE votes MODIFY COLUMN target_type
			ENUM('question','answer','forum_post','forum_reply','article') NOT NULL`); err != nil {
			return nil, err
		}
	}
	for _, stmt := range Seed {
		if _, err := conn.Exec(stmt); err != nil {
			return nil, err
		}
	}
	return conn, nil
}
