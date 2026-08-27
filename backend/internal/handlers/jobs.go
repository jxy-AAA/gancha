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

var jobStatuses = map[string]bool{
	"pending":   true,
	"applied":   true,
	"test":      true,
	"interview": true,
	"offer":     true,
	"rejected":  true,
}

// ListJobs 就业共享表格：全量返回，按更新时间倒序。
func (s *Server) ListJobs(c *gin.Context) {
	rows, err := s.DB.Query(`SELECT j.id, j.user_id, j.company, j.position, j.city, j.status,
			j.url, j.note, j.created_at, j.updated_at, u.username
		FROM job_entries j
		JOIN users u ON u.id=j.user_id
		ORDER BY j.updated_at DESC, j.id DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			id, uid    int64
			company    string
			position   string
			city       string
			status     string
			url, note  string
			created    time.Time
			updated    time.Time
			author     string
		)
		if rows.Scan(&id, &uid, &company, &position, &city, &status,
			&url, &note, &created, &updated, &author) == nil {
			items = append(items, gin.H{
				"id": id, "user_id": uid, "company": company, "position": position,
				"city": city, "status": status, "url": url, "note": note,
				"created_at": created, "updated_at": updated, "author": author,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type jobReq struct {
	Company  string `json:"company"`
	Position string `json:"position"`
	City     string `json:"city"`
	Status   string `json:"status"`
	URL      string `json:"url"`
	Note     string `json:"note"`
}

func (r *jobReq) sanitize() string {
	r.Company = strings.TrimSpace(r.Company)
	r.Position = strings.TrimSpace(r.Position)
	r.City = strings.TrimSpace(r.City)
	r.URL = strings.TrimSpace(r.URL)
	r.Note = strings.TrimSpace(r.Note)
	if r.Company == "" || len([]rune(r.Company)) > 100 {
		return "公司名称不能为空且不超过 100 字"
	}
	if r.Position == "" || len([]rune(r.Position)) > 100 {
		return "岗位不能为空且不超过 100 字"
	}
	if len([]rune(r.City)) > 50 {
		return "城市不超过 50 字"
	}
	if len([]rune(r.URL)) > 500 {
		return "链接不超过 500 字"
	}
	if len([]rune(r.Note)) > 500 {
		return "备注不超过 500 字"
	}
	if r.Status == "" {
		r.Status = "pending"
	}
	if !jobStatuses[r.Status] {
		return "无效的投递状态"
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
	res, err := s.DB.Exec(`INSERT INTO job_entries (user_id, company, position, city, status, url, note)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.ID, req.Company, req.Position, req.City, req.Status, req.URL, req.Note)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// UpdateJob 编辑任意行（共享表格，登录用户均可），刷新更新时间。
func (s *Server) UpdateJob(c *gin.Context) {
	_, _ = middleware.CurrentUser(c)
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
	res, err := s.DB.Exec(`UPDATE job_entries SET company=?, position=?, city=?, status=?, url=?, note=?, updated_at=? WHERE id=?`,
		req.Company, req.Position, req.City, req.Status, req.URL, req.Note, time.Now(), id)
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
