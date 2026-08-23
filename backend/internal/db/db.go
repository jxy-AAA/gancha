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
	for _, stmt := range Seed {
		if _, err := conn.Exec(stmt); err != nil {
			return nil, err
		}
	}
	return conn, nil
}
