package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"guangyanji/internal/middleware"

	"github.com/gin-gonic/gin"
)

// ListJobs 就业共享表格（2027 届公司招聘信息）：全量返回，按更新时间倒序。
func (s *Server) ListJobs(c *gin.Context) {
	rows, err := s.DB.Query(`SELECT j.id, j.user_id, j.company, j.industry, j.positions_27,
			j.confirm_level, j.strength, j.city, j.current_status, j.links, j.verified_at,
			j.created_at, j.updated_at,
			COALESCE(u.username, '官方'), COALESCE(le.username, u.username, '官方'),
			(SELECT COUNT(*) FROM job_reviews r WHERE r.job_id = j.id)
		FROM job_entries j
		LEFT JOIN users u ON u.id=j.user_id
		LEFT JOIN users le ON le.id=j.last_editor_id
		ORDER BY j.updated_at DESC, j.id ASC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			id, uid   int64
			company   string
			industry  string
			positions string
			confirm   string
			strength  string
			city      string
			curStatus string
			links     string
			verified  string
			created   time.Time
			updated   time.Time
			author    string
			updater   string
			reviewCnt int
		)
		if rows.Scan(&id, &uid, &company, &industry, &positions,
			&confirm, &strength, &city, &curStatus, &links, &verified,
			&created, &updated, &author, &updater, &reviewCnt) == nil {
			items = append(items, gin.H{
				"id": id, "user_id": uid, "company": company, "industry": industry,
				"positions_27": positions, "confirm_level": confirm, "strength": strength,
				"city": city, "current_status": curStatus, "links": links,
				"verified_at": verified, "created_at": created, "updated_at": updated,
				"author": author, "updater": updater, "review_count": reviewCnt,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type jobReq struct {
	Company      string `json:"company"`
	Industry     string `json:"industry"`
	Positions27  string `json:"positions_27"`
	ConfirmLevel string `json:"confirm_level"`
	Strength     string `json:"strength"`
	City         string `json:"city"`
	CurrentStatus string `json:"current_status"`
	Links        string `json:"links"`
	VerifiedAt   string `json:"verified_at"`
}

func (r *jobReq) sanitize() string {
	r.Company = strings.TrimSpace(r.Company)
	r.Industry = strings.TrimSpace(r.Industry)
	r.Positions27 = strings.TrimSpace(r.Positions27)
	r.ConfirmLevel = strings.TrimSpace(r.ConfirmLevel)
	r.Strength = strings.TrimSpace(r.Strength)
	r.City = strings.TrimSpace(r.City)
	r.CurrentStatus = strings.TrimSpace(r.CurrentStatus)
	r.Links = strings.TrimSpace(r.Links)
	r.VerifiedAt = strings.TrimSpace(r.VerifiedAt)
	if r.Company == "" || len([]rune(r.Company)) > 100 {
		return "公司名称不能为空且不超过 100 字"
	}
	if len([]rune(r.Industry)) > 200 {
		return "产业链/光学方向不超过 200 字"
	}
	if len([]rune(r.Positions27)) > 500 {
		return "27届岗位不超过 500 字"
	}
	if len([]rune(r.ConfirmLevel)) > 300 {
		return "岗位确认度不超过 300 字"
	}
	if len([]rune(r.Strength)) > 10 {
		return "证据强度不超过 10 字"
	}
	if len([]rune(r.City)) > 100 {
		return "地点不超过 100 字"
	}
	if len([]rune(r.CurrentStatus)) > 200 {
		return "当前状态不超过 200 字"
	}
	if len([]rune(r.Links)) > 1000 {
		return "证据链接不超过 1000 字"
	}
	if len([]rune(r.VerifiedAt)) > 10 {
		return "核验日期格式不正确"
	}
	return ""
}

// CreateJob 新增一行（登录用户均可）。
func (s *Server) CreateJob(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	var req jobReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if msg := req.sanitize(); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	res, err := s.DB.Exec(`INSERT INTO job_entries (user_id, last_editor_id, company, industry, positions_27,
			confirm_level, strength, city, current_status, links, verified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.ID, req.Company, req.Industry, req.Positions27,
		req.ConfirmLevel, req.Strength, req.City, req.CurrentStatus, req.Links, req.VerifiedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// UpdateJob 编辑任意行（共享表格，登录用户均可），刷新更新时间与最后编辑人。
func (s *Server) UpdateJob(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var req jobReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if msg := req.sanitize(); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	res, err := s.DB.Exec(`UPDATE job_entries SET company=?, industry=?, positions_27=?, confirm_level=?,
			strength=?, city=?, current_status=?, links=?, verified_at=?, last_editor_id=?, updated_at=?
		WHERE id=?`,
		req.Company, req.Industry, req.Positions27, req.ConfirmLevel,
		req.Strength, req.City, req.CurrentStatus, req.Links, req.VerifiedAt, u.ID, time.Now(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var exists int
		_ = s.DB.QueryRow(`SELECT COUNT(*) FROM job_entries WHERE id=?`, id).Scan(&exists)
		if exists == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteJob 删除行（仅作者或管理员）。
func (s *Server) DeleteJob(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM job_entries WHERE id=?`, id).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该行"})
		return
	}
	_, _ = s.DB.Exec(`DELETE FROM job_entries WHERE id=?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
