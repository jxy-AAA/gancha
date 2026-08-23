package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"guangyanji/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Audit 记录管理操作。
func (s *Server) audit(c *gin.Context, action, targetType string, targetID *int64, details string) {
	u, _ := middleware.CurrentUser(c)
	_, _ = s.DB.Exec(`INSERT INTO admin_audit (actor_id, action, target_type, target_id, details) VALUES (?, ?, ?, ?, ?)`,
		u.ID, action, targetType, targetID, details)
}

// AdminStats 站内统计。
func (s *Server) AdminStats(c *gin.Context) {
	count := func(table string) int {
		var n int
		_ = s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n)
		return n
	}
	c.JSON(http.StatusOK, gin.H{
		"users":         count("users"),
		"questions":     count("questions"),
		"answers":       count("answers"),
		"articles":      count("articles"),
		"forum_posts":   count("forum_posts"),
		"forum_replies": count("forum_replies"),
		"uploads":       count("uploads_meta"),
		"notifications": count("notifications"),
	})
}

// AdminUsers 用户列表（分页）。
func (s *Server) AdminUsers(c *gin.Context) {
	page, size := clampPage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	search := strings.TrimSpace(c.Query("search"))
	where := "1=1"
	args := []interface{}{}
	if search != "" {
		where = "(username LIKE ? OR email LIKE ?)"
		like := "%" + search + "%"
		args = append(args, like, like)
	}
	var total int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE `+where, args...).Scan(&total)
	rows, err := s.DB.Query(`SELECT id, username, email, role, avatar, bio, expertise, created_at, suspended_until
		FROM users WHERE `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, append(args, size, (page-1)*size)...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			id       int64
			username string
			email    string
			role     string
			avatar   string
			bio      string
			expert   string
			created  time.Time
			susp     sql.NullString
		)
		if rows.Scan(&id, &username, &email, &role, &avatar, &bio, &expert, &created, &susp) == nil {
			items = append(items, gin.H{
				"id": id, "username": username, "email": email, "role": role, "avatar": avatar,
				"bio": bio, "expertise": expert, "created_at": created,
				"suspended_until": susp.String,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

type adminUserUpdate struct {
	Role           string `json:"role"`
	SuspendedUntil string `json:"suspended_until"` // "2026-09-01T00:00:00Z" 或 ""
}

// UpdateAdminUser 修改用户角色 / 暂停 / 解封（不能改自己）。
func (s *Server) UpdateAdminUser(c *gin.Context) {
	actor, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if id == actor.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能修改自己的账号"})
		return
	}
	var req adminUserUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	var exists int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE id=?`, id).Scan(&exists); err != nil || exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if req.Role != "" {
		switch req.Role {
		case "new_user", "user", "admin":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效角色"})
			return
		}
	}
	var susp interface{}
	if req.SuspendedUntil == "" {
		susp = nil
	} else {
		t, err := time.Parse(time.RFC3339, req.SuspendedUntil)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "暂停时间格式错误"})
			return
		}
		susp = t
	}
	_, err = s.DB.Exec(`UPDATE users SET role=?, suspended_until=? WHERE id=?`, req.Role, susp, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	s.audit(c, "update_user", "user", &id, "角色="+req.Role+" 暂停="+req.SuspendedUntil)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteAdminUser 删除用户及其内容。
func (s *Server) DeleteAdminUser(c *gin.Context) {
	actor, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if id == actor.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除自己的账号"})
		return
	}
	var exists int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE id=?`, id).Scan(&exists); err != nil || exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	_, _ = s.DB.Exec(`DELETE FROM questions WHERE user_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM answers WHERE user_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM articles WHERE user_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM forum_posts WHERE user_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM forum_replies WHERE user_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM comments WHERE user_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM votes WHERE user_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM bookmarks WHERE user_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM question_follows WHERE user_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM user_follows WHERE follower_id=? OR followed_id=?`, id, id)
	_, _ = s.DB.Exec(`DELETE FROM users WHERE id=?`, id)
	s.audit(c, "delete_user", "user", &id, "删除用户")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// PublicCategories 公开分类列表（仅 active）。
func (s *Server) PublicCategories(c *gin.Context) {
	rows, err := s.DB.Query(`SELECT id, name, position FROM categories WHERE active=1 ORDER BY position ASC, id ASC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			id       int64
			name     string
			position int
		)
		if rows.Scan(&id, &name, &position) == nil {
			items = append(items, gin.H{"id": id, "name": name, "position": position})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// AdminCategories 分类管理。
func (s *Server) AdminCategories(c *gin.Context) {
	rows, err := s.DB.Query(`SELECT id, name, active, position FROM categories ORDER BY position ASC, id ASC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			id       int64
			name     string
			active   bool
			position int
		)
		if rows.Scan(&id, &name, &active, &position) == nil {
			items = append(items, gin.H{"id": id, "name": name, "active": active, "position": position})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type categoryReq struct {
	Name     string `json:"name"`
	Active   bool   `json:"active"`
	Position int    `json:"position"`
}

// CreateAdminCategory 新增分类。
func (s *Server) CreateAdminCategory(c *gin.Context) {
	var req categoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len([]rune(req.Name)) > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分类名不合法"})
		return
	}
	res, err := s.DB.Exec(`INSERT INTO categories (name, active, position) VALUES (?, ?, ?)`,
		req.Name, req.Active, req.Position)
	if err != nil {
		if isDuplicate(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "分类已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	id, _ := res.LastInsertId()
	s.audit(c, "create_category", "category", &id, req.Name)
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// UpdateAdminCategory 编辑分类。
func (s *Server) UpdateAdminCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var req categoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分类名不能为空"})
		return
	}
	_, err = s.DB.Exec(`UPDATE categories SET name=?, active=?, position=? WHERE id=?`,
		req.Name, req.Active, req.Position, id)
	if err != nil {
		if isDuplicate(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "分类已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	s.audit(c, "update_category", "category", &id, req.Name)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteAdminCategory 删除分类（存在关联问题时返回错误）。
func (s *Server) DeleteAdminCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var refs int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM questions WHERE category_id=?`, id).Scan(&refs)
	if refs > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该分类下仍有问题，无法删除"})
		return
	}
	if _, err := s.DB.Exec(`DELETE FROM categories WHERE id=?`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	s.audit(c, "delete_category", "category", &id, "删除分类")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// AdminAudit 审计日志（分页）。
func (s *Server) AdminAudit(c *gin.Context) {
	page, size := clampPage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	rows, err := s.DB.Query(`SELECT a.id, a.actor_id, u.username, a.action, a.target_type, a.target_id, a.details, a.created_at
		FROM admin_audit a LEFT JOIN users u ON u.id=a.actor_id
		ORDER BY a.id DESC LIMIT ? OFFSET ?`, size, (page-1)*size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			id       int64
			actorID  int64
			actor    sql.NullString
			action   string
			tType    string
			tID      sql.NullInt64
			details  string
			created  time.Time
		)
		if rows.Scan(&id, &actorID, &actor, &action, &tType, &tID, &details, &created) == nil {
			items = append(items, gin.H{
				"id": id, "actor_id": actorID, "actor_name": actor.String, "action": action,
				"target_type": tType, "target_id": tID.Int64, "details": details, "created_at": created,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// ListNotifications 当前用户的通知。
func (s *Server) ListNotifications(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	rows, err := s.DB.Query(`SELECT id, type, message, question_id, is_read, created_at
		FROM notifications WHERE user_id=? ORDER BY id DESC LIMIT 50`, u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			id        int64
			typ       string
			msg       string
			qid       sql.NullInt64
			isRead    bool
			created   time.Time
		)
		if rows.Scan(&id, &typ, &msg, &qid, &isRead, &created) == nil {
			items = append(items, gin.H{"id": id, "type": typ, "message": msg,
				"question_id": qid.Int64, "read": isRead, "created_at": created})
		}
	}
	var unread int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id=? AND is_read=0`, u.ID).Scan(&unread)
	c.JSON(http.StatusOK, gin.H{"items": items, "unread": unread})
}

// ReadNotifications 全部标记已读。
func (s *Server) ReadNotifications(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	_, _ = s.DB.Exec(`UPDATE notifications SET is_read=1 WHERE user_id=? AND is_read=0`, u.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
