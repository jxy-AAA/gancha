package handlers

import (
	"database/sql"
	"net/http"
	"regexp"
	"strings"
	"time"

	"guangyanji/internal/auth"
	"guangyanji/internal/middleware"

	"github.com/gin-gonic/gin"
)

var (
	emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	nameRe  = regexp.MustCompile(`^[\p{L}\p{N}_\-\. ]{2,30}$`)
)

func userJSON(u middleware.AuthUser) gin.H {
	return gin.H{
		"id":       u.ID,
		"username": u.Username,
		"email":    u.Email,
		"role":     u.Role,
		"avatar":   u.Avatar,
	}
}

type registerReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register 注册并直接登录。
func (s *Server) Register(c *gin.Context) {
	if !s.Cfg.RegisterEnable {
		c.JSON(http.StatusForbidden, gin.H{"error": "注册已关闭"})
		return
	}
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	switch {
	case !nameRe.MatchString(req.Username):
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名需为 2-30 位中文/字母/数字"})
		return
	case !emailRe.MatchString(req.Email):
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱格式不正确"})
		return
	case len(req.Password) < 8 || len(req.Password) > 72:
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码长度需在 8-72 位之间"})
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	res, err := s.DB.Exec(`INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)`,
		req.Username, req.Email, hash)
	if err != nil {
		if isDuplicate(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "用户名或邮箱已被注册"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	uid, _ := res.LastInsertId()
	token, _ := auth.SignJWT(s.Cfg.JWTSecret, uid, 0)
	setSessionCookie(c, token)
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  gin.H{"id": uid, "username": req.Username, "email": req.Email, "role": "new_user", "avatar": ""},
	})
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login 邮箱 + 密码登录。
func (s *Server) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	var (
		uid       int64
		username  string
		hash      string
		role      string
		avatar    string
		suspended sql.NullTime
		tokenVer  int
	)
	err := s.DB.QueryRow(`SELECT id, username, password_hash, role, avatar, token_version, suspended_until
		FROM users WHERE email=?`, req.Email).
		Scan(&uid, &username, &hash, &role, &avatar, &tokenVer, &suspended)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if !auth.CheckPassword(hash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}
	if suspended.Valid && !suspended.Time.IsZero() && suspended.Time.After(time.Now()) {
		c.JSON(http.StatusForbidden, gin.H{"error": "账号已暂停至 " + suspended.Time.Format("2006-01-02 15:04")})
		return
	}
	token, _ := auth.SignJWT(s.Cfg.JWTSecret, uid, tokenVer)
	setSessionCookie(c, token)
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  gin.H{"id": uid, "username": username, "email": req.Email, "role": role, "avatar": avatar},
	})
}

func (s *Server) Logout(c *gin.Context) {
	c.SetCookie(middleware.CookieName, "", -1, "/", "", c.Request.TLS != nil, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Me 当前用户信息（含个人资料字段与社区统计）。
func (s *Server) Me(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	var bio, expertise string
	_ = s.DB.QueryRow(`SELECT bio, expertise FROM users WHERE id=?`, u.ID).Scan(&bio, &expertise)

	var following, followers, receivedLikes, posts, comments int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM user_follows WHERE follower_id=?`, u.ID).Scan(&following)
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM user_follows WHERE followed_id=?`, u.ID).Scan(&followers)
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM votes WHERE
		(target_type='question' AND target_id IN (SELECT id FROM questions WHERE user_id=?)) OR
		(target_type='answer' AND target_id IN (SELECT id FROM answers WHERE user_id=?)) OR
		(target_type='forum_post' AND target_id IN (SELECT id FROM forum_posts WHERE user_id=?)) OR
		(target_type='forum_reply' AND target_id IN (SELECT id FROM forum_replies WHERE user_id=?))`,
		u.ID, u.ID, u.ID, u.ID).Scan(&receivedLikes)
	_ = s.DB.QueryRow(`SELECT
		(SELECT COUNT(*) FROM questions WHERE user_id=?) + (SELECT COUNT(*) FROM forum_posts WHERE user_id=?)`,
		u.ID, u.ID).Scan(&posts)
	_ = s.DB.QueryRow(`SELECT
		(SELECT COUNT(*) FROM answers WHERE user_id=?) + (SELECT COUNT(*) FROM forum_replies WHERE user_id=?) +
		(SELECT COUNT(*) FROM comments WHERE user_id=?)`,
		u.ID, u.ID, u.ID).Scan(&comments)

	c.JSON(http.StatusOK, gin.H{
		"id": u.ID, "username": u.Username, "email": u.Email, "role": u.Role, "avatar": u.Avatar,
		"bio": bio, "expertise": expertise,
		"stats": gin.H{
			"following": following, "followers": followers, "received_likes": receivedLikes,
			"posts": posts, "comments": comments,
		},
	})
}

type updateMeReq struct {
	Username  string `json:"username"`
	Bio       string `json:"bio"`
	Expertise string `json:"expertise"`
	Avatar    string `json:"avatar"`
}

// UpdateMe 更新个人资料。
func (s *Server) UpdateMe(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	var req updateMeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username != "" && req.Username != u.Username {
		if !nameRe.MatchString(req.Username) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "用户名需为 2-30 位中文/字母/数字"})
			return
		}
	}
	if len(req.Bio) > 160 || len(req.Expertise) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "简介或擅长方向过长"})
		return
	}
	_, err := s.DB.Exec(`UPDATE users SET username=?, bio=?, expertise=?, avatar=? WHERE id=?`,
		req.Username, req.Bio, req.Expertise, req.Avatar, u.ID)
	if err != nil {
		if isDuplicate(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "用户名已被占用"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type changePasswordReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword 修改密码，成功后使旧 token 全部失效（token_version+1）。
func (s *Server) ChangePassword(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if len(req.NewPassword) < 8 || len(req.NewPassword) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码长度需在 8-72 位之间"})
		return
	}
	var hash string
	if err := s.DB.QueryRow(`SELECT password_hash FROM users WHERE id=?`, u.ID).Scan(&hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if !auth.CheckPassword(hash, req.OldPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "原密码错误"})
		return
	}
	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if _, err := s.DB.Exec(`UPDATE users SET password_hash=?, token_version=token_version+1 WHERE id=?`, newHash, u.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	// 重新签发 token（版本号已 +1）
	var ver int
	_ = s.DB.QueryRow(`SELECT token_version FROM users WHERE id=?`, u.ID).Scan(&ver)
	token, _ := auth.SignJWT(s.Cfg.JWTSecret, u.ID, ver)
	setSessionCookie(c, token)
	c.JSON(http.StatusOK, gin.H{"ok": true, "token": token})
}

func setSessionCookie(c *gin.Context, token string) {
	secure := c.Request.TLS != nil || strings.HasPrefix(c.Request.Header.Get("X-Forwarded-Proto"), "https")
	c.SetCookie(middleware.CookieName, token, 604800, "/", "", secure, true)
}

func isDuplicate(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Duplicate entry")
}
