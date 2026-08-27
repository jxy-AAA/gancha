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

// ListJobReviews 公司评价列表（公开，匿名评价显示为「匿名用户」）。
func (s *Server) ListJobReviews(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var exists int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM job_entries WHERE id=?`, id).Scan(&exists)
	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	rows, err := s.DB.Query(`SELECT r.id, r.user_id, r.body, r.is_anonymous, r.created_at,
			COALESCE(u.username, '')
		FROM job_reviews r
		LEFT JOIN users u ON u.id=r.user_id
		WHERE r.job_id=?
		ORDER BY r.created_at DESC, r.id DESC`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			rid, uid  int64
			body      string
			anon      bool
			created   time.Time
			username  string
		)
		if rows.Scan(&rid, &uid, &body, &anon, &created, &username) == nil {
			author := username
			if anon || author == "" {
				author = "匿名用户"
			}
			items = append(items, gin.H{
				"id": rid, "user_id": uid, "body": body, "is_anonymous": anon,
				"created_at": created, "author": author,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type jobReviewReq struct {
	Body      string `json:"body"`
	Anonymous bool   `json:"anonymous"`
}

// CreateJobReview 对某家公司填写评价（登录用户均可，可选择匿名）。
func (s *Server) CreateJobReview(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var req jobReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" || len([]rune(req.Body)) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评价不能为空且不超过 500 字"})
		return
	}
	var exists int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM job_entries WHERE id=?`, id).Scan(&exists)
	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	if _, err := s.DB.Exec(`INSERT INTO job_reviews (job_id, user_id, body, is_anonymous) VALUES (?, ?, ?, ?)`,
		id, u.ID, req.Body, req.Anonymous); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteJobReview 删除评价（仅评价作者或管理员）。
func (s *Server) DeleteJobReview(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM job_reviews WHERE id=?`, id).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "评价不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该评价"})
		return
	}
	_, _ = s.DB.Exec(`DELETE FROM job_reviews WHERE id=?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
