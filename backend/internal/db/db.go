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
	// 现有环境的 forum_posts.is_anonymous 列升级（幂等）
	var hasAnon int
	_ = conn.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='forum_posts' AND COLUMN_NAME='is_anonymous'`).Scan(&hasAnon)
	if hasAnon == 0 {
		if _, err := conn.Exec(`ALTER TABLE forum_posts ADD COLUMN is_anonymous TINYINT(1) NOT NULL DEFAULT 0`); err != nil {
			return nil, err
		}
	}
	// 现有环境的 job_entries 升级为“2027届公司招聘信息”共享表格列（幂等：逐列检查后 ALTER）
	for name, ddl := range map[string]string{
		"industry":      "VARCHAR(200) NOT NULL DEFAULT ''",
		"positions_27":  "VARCHAR(500) NOT NULL DEFAULT ''",
		"confirm_level": "VARCHAR(300) NOT NULL DEFAULT ''",
		"strength":      "VARCHAR(10) NOT NULL DEFAULT ''",
		"current_status": "VARCHAR(200) NOT NULL DEFAULT ''",
		"links":         "VARCHAR(1000) NOT NULL DEFAULT ''",
		"verified_at":   "VARCHAR(10) NOT NULL DEFAULT ''",
	} {
		var n int
		_ = conn.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='job_entries' AND COLUMN_NAME=?`, name).Scan(&n)
		if n == 0 {
			if _, err := conn.Exec(`ALTER TABLE job_entries ADD COLUMN ` + name + ` ` + ddl); err != nil {
				return nil, err
			}
		}
	}
	// 旧版就业表字段（position/status/url/note）已废弃：NOT NULL 无默认值会阻塞新插入，幂等删除
	for _, col := range []string{"position", "status", "url", "note"} {
		var n int
		_ = conn.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='job_entries' AND COLUMN_NAME=?`, col).Scan(&n)
		if n > 0 {
			if _, err := conn.Exec(`ALTER TABLE job_entries DROP COLUMN ` + col); err != nil {
				return nil, err
			}
		}
	}
	// 旧版 city 为 VARCHAR(50)，新结构上限 100 字，幂等加宽
	var cityLen int
	_ = conn.QueryRow(`SELECT CHARACTER_MAXIMUM_LENGTH FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='job_entries' AND COLUMN_NAME='city'`).Scan(&cityLen)
	if cityLen > 0 && cityLen < 100 {
		if _, err := conn.Exec(`ALTER TABLE job_entries MODIFY COLUMN city VARCHAR(100) NOT NULL DEFAULT ''`); err != nil {
			return nil, err
		}
	}
	for _, stmt := range Seed {
		if _, err := conn.Exec(stmt); err != nil {
			return nil, err
		}
	}
	if err := seedJobEntries(conn); err != nil {
		return nil, err
	}
	return conn, nil
}
