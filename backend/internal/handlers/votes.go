package handlers

import (
	"net/http"

	"guangyanji/internal/middleware"

	"github.com/gin-gonic/gin"
)

type voteReq struct {
	TargetType string `json:"target_type"` // question | answer | forum_post | forum_reply
	TargetID   int64  `json:"target_id"`
}

// ToggleVote 点赞/取消点赞（同一目标每个用户只能一票）。
func (s *Server) ToggleVote(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	var req voteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	switch req.TargetType {
	case "question", "answer", "forum_post", "forum_reply":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的点赞目标类型"})
		return
	}
	if req.TargetID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var table string
	switch req.TargetType {
	case "question":
		table = "questions"
	case "answer":
		table = "answers"
	case "forum_post":
		table = "forum_posts"
	case "forum_reply":
		table = "forum_replies"
	}
	var exists int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE id=?`, req.TargetID).Scan(&exists)
	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "目标不存在"})
		return
	}
	var voted int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM votes WHERE user_id=? AND target_type=? AND target_id=?`,
		u.ID, req.TargetType, req.TargetID).Scan(&voted)
	if voted > 0 {
		_, _ = s.DB.Exec(`DELETE FROM votes WHERE user_id=? AND target_type=? AND target_id=?`,
			u.ID, req.TargetType, req.TargetID)
		c.JSON(http.StatusOK, gin.H{"voted": false})
		return
	}
	_, _ = s.DB.Exec(`INSERT INTO votes (user_id, target_type, target_id) VALUES (?, ?, ?)`,
		u.ID, req.TargetType, req.TargetID)
	c.JSON(http.StatusOK, gin.H{"voted": true})
}

// VoteStatus 批量查询当前用户对目标的点赞状态与计数。
func (s *Server) VoteStatus(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	var req voteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	var voted bool
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM votes WHERE user_id=? AND target_type=? AND target_id=?`,
		u.ID, req.TargetType, req.TargetID).Scan(&voted)
	_ = err
	var count int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM votes WHERE target_type=? AND target_id=?`,
		req.TargetType, req.TargetID).Scan(&count)
	c.JSON(http.StatusOK, gin.H{"voted": voted, "count": count})
}
