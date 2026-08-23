package middleware

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"guangyanji/internal/auth"

	"github.com/gin-gonic/gin"
)

const CtxUserKey = "user"

type AuthUser struct {
	ID        int64
	Username  string
	Email     string
	Role      string
	Avatar    string
	TokenVer  int
	Suspended sql.NullTime
}

const CookieName = "guangyanji_session"

// Auth 必须登录。支持 Authorization: Bearer 与 HttpOnly Cookie 双通道。
func Auth(db *sql.DB, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := loadUser(c, db, secret)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}
		if u.Suspended.Valid && u.Suspended.Time.After(time.Now()) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "账号已暂停"})
			return
		}
		c.Set(CtxUserKey, u)
		c.Next()
	}
}

// OptionalAuth 可选登录：有 token 则填充用户，无 token 照常通过。
func OptionalAuth(db *sql.DB, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if u, ok := loadUser(c, db, secret); ok {
			c.Set(CtxUserKey, u)
		}
		c.Next()
	}
}

// Admin 仅管理员。
func Admin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := c.Get(CtxUserKey)
		if !ok || u.(AuthUser).Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			return
		}
		c.Next()
	}
}

func loadUser(c *gin.Context, db *sql.DB, secret string) (AuthUser, bool) {
	token := tokenFromRequest(c)
	if token == "" {
		return AuthUser{}, false
	}
	claims, err := auth.ParseJWT(secret, token)
	if err != nil {
		return AuthUser{}, false
	}
	var (
		u        AuthUser
		hash     sql.NullString
		role     string
		tokenVer int
	)
	row := db.QueryRow(`SELECT id, username, email, password_hash, role, avatar, token_version, suspended_until FROM users WHERE id=?`, claims.UID)
	if err := row.Scan(&u.ID, &u.Username, &u.Email, &hash, &role, &u.Avatar, &tokenVer, &u.Suspended); err != nil {
		return AuthUser{}, false
	}
	if claims.V != tokenVer {
		return AuthUser{}, false
	}
	u.Role = role
	u.TokenVer = claims.V
	return u, true
}

func tokenFromRequest(c *gin.Context) string {
	bearer := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if strings.Count(bearer, ".") == 2 {
		return bearer
	}
	if tok, err := c.Cookie(CookieName); err == nil && tok != "" {
		return tok
	}
	return ""
}

// CurrentUser 从上下文取当前用户（OptionalAuth 后可能为空）。
func CurrentUser(c *gin.Context) (AuthUser, bool) {
	v, ok := c.Get(CtxUserKey)
	if !ok {
		return AuthUser{}, false
	}
	u, ok := v.(AuthUser)
	return u, ok
}
