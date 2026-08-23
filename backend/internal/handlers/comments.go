package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"guangyanji/internal/middleware"

	"github.com/gin-gonic/gin"
)

// ListComments 问题下的评论。
func (s *Server) ListComments(c *gin.Context) {
	qid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	rows, err := s.DB.Query(`SELECT cm.id, cm.user_id, cm.body, cm.created_at, u.username, u.avatar
		FROM comments cm JOIN users u ON u.id=cm.user_id
		WHERE cm.question_id=? ORDER BY cm.created_at ASC`, qid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			id, uid int64
			body    string
			created time.Time
			author  string
			avatar  string
		)
		if rows.Scan(&id, &uid, &body, &created, &author, &avatar) == nil {
			items = append(items, gin.H{"id": id, "user_id": uid, "body": body,
				"author": author, "avatar": avatar, "created_at": created})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// CreateComment 发表评论。
func (s *Server) CreateComment(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	qid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var req answerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" || len([]rune(req.Body)) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论不能为空且不超过 500 字"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM questions WHERE id=?`, qid).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "问题不存在"})
		return
	}
	res, err := s.DB.Exec(`INSERT INTO comments (question_id, user_id, body) VALUES (?, ?, ?)`, qid, u.ID, req.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// DeleteComment 删除评论（作者或管理员）。
func (s *Server) DeleteComment(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM comments WHERE id=?`, id).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "评论不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除"})
		return
	}
	_, _ = s.DB.Exec(`DELETE FROM comments WHERE id=?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
