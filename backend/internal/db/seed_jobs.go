package db

import (
	"database/sql"
	_ "embed"
	"encoding/json"
)

//go:embed jobs_seed.json
var jobsSeedJSON []byte

type jobSeedRow struct {
	Company      string `json:"company"`
	Industry     string `json:"industry"`
	City         string `json:"city"`
	ApplyLink    string `json:"apply_link"`
	ReferralCode string `json:"referral_code"`
	VerifiedAt   string `json:"verified_at"`
}

// seedJobEntries 首次启动（表中尚无 Excel 格式数据）时导入 2027 届光学公司招聘信息。
func seedJobEntries(conn *sql.DB) error {
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM job_entries WHERE industry <> ''`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	var rows []jobSeedRow
	if err := json.Unmarshal(jobsSeedJSON, &rows); err != nil {
		return err
	}
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO job_entries (user_id, last_editor_id, company, industry,
		city, apply_link, referral_code, verified_at)
		VALUES (0, 0, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.Company, r.Industry,
			r.City, r.ApplyLink, r.ReferralCode, r.VerifiedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}
