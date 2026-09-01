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
	// 现有环境的 job_entries 证据链列 links 更名为投递链接 apply_link（保留已有数据，幂等）
	var hasApplyLink int
	_ = conn.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='job_entries' AND COLUMN_NAME='apply_link'`).Scan(&hasApplyLink)
	if hasApplyLink == 0 {
		var hasLinks int
		_ = conn.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='job_entries' AND COLUMN_NAME='links'`).Scan(&hasLinks)
		if hasLinks > 0 {
			if _, err := conn.Exec(`ALTER TABLE job_entries CHANGE COLUMN links apply_link VARCHAR(1000) NOT NULL DEFAULT ''`); err != nil {
				return nil, err
			}
		}
	}
	// 现有环境的 job_entries 升级为共享表格列（幂等：逐列检查后 ALTER）
	for name, ddl := range map[string]string{
		"industry":      "VARCHAR(200) NOT NULL DEFAULT ''",
		"apply_link":    "VARCHAR(1000) NOT NULL DEFAULT ''",
		"referral_code": "VARCHAR(50) NOT NULL DEFAULT ''",
		"verified_at":   "VARCHAR(10) NOT NULL DEFAULT ''",
		"status":        "VARCHAR(20) NOT NULL DEFAULT 'active'",
		"is_pinned":     "TINYINT(1) NOT NULL DEFAULT 0",
		"edit_reason":   "VARCHAR(200) NOT NULL DEFAULT ''",
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
	// job_entries 已废弃字段（27届项目与光学岗位/岗位确认度/证据强度/当前状态）幂等删除
	for _, col := range []string{"positions_27", "confirm_level", "strength", "current_status"} {
		var n int
		_ = conn.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='job_entries' AND COLUMN_NAME=?`, col).Scan(&n)
		if n > 0 {
			if _, err := conn.Exec(`ALTER TABLE job_entries DROP COLUMN ` + col); err != nil {
				return nil, err
			}
		}
	}
	// job_entry_versions 同步迁移：links 更名、新增内推码、删除废弃列（幂等）
	var verHasApply int
	_ = conn.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='job_entry_versions' AND COLUMN_NAME='apply_link'`).Scan(&verHasApply)
	if verHasApply == 0 {
		var verHasLinks int
		_ = conn.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='job_entry_versions' AND COLUMN_NAME='links'`).Scan(&verHasLinks)
		if verHasLinks > 0 {
			if _, err := conn.Exec(`ALTER TABLE job_entry_versions CHANGE COLUMN links apply_link VARCHAR(1000) NOT NULL DEFAULT ''`); err != nil {
				return nil, err
			}
		}
	}
	var verHasReferral int
	_ = conn.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='job_entry_versions' AND COLUMN_NAME='referral_code'`).Scan(&verHasReferral)
	if verHasReferral == 0 {
		if _, err := conn.Exec(`ALTER TABLE job_entry_versions ADD COLUMN referral_code VARCHAR(50) NOT NULL DEFAULT ''`); err != nil {
			return nil, err
		}
	}
	for _, col := range []string{"positions_27", "confirm_level", "strength", "current_status"} {
		var n int
		_ = conn.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='job_entry_versions' AND COLUMN_NAME=?`, col).Scan(&n)
		if n > 0 {
			if _, err := conn.Exec(`ALTER TABLE job_entry_versions DROP COLUMN ` + col); err != nil {
				return nil, err
			}
		}
	}
	// 现有环境的 forum_posts 升级为“排序/置顶/解决/标签”增强列（幂等）
	for name, ddl := range map[string]string{
		"is_pinned": "TINYINT(1) NOT NULL DEFAULT 0",
		"is_solved": "TINYINT(1) NOT NULL DEFAULT 0",
		"tags":      "VARCHAR(250) NOT NULL DEFAULT ''",
	} {
		var n int
		_ = conn.QueryRow(`SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='forum_posts' AND COLUMN_NAME=?`, name).Scan(&n)
		if n == 0 {
			if _, err := conn.Exec(`ALTER TABLE forum_posts ADD COLUMN ` + name + ` ` + ddl); err != nil {
				return nil, err
			}
		}
	}
	// 旧版就业表字段（position/url/note）已废弃：NOT NULL 无默认值会阻塞新插入，幂等删除。
	// 注意：status 现在被复用为“失效/重复”标记列，不能删除。
	for _, col := range []string{"position", "url", "note"} {
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
