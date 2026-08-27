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

type listArticleReq struct {
	Search   string `form:"search"`
	Tag      string `form:"tag"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// ListArticles 知识库文章列表（仅 published）。
func (s *Server) ListArticles(c *gin.Context) {
	var q listArticleReq
	_ = c.ShouldBindQuery(&q)
	page, size := clampPage(q.Page, q.PageSize)
	where := []string{"a.published=1"}
	args := []interface{}{}
	if q.Search != "" {
		where = append(where, "(a.title LIKE ? OR a.body LIKE ? OR a.tags LIKE ?)")
		like := "%" + q.Search + "%"
		args = append(args, like, like, like)
	}
	if q.Tag != "" {
		where = append(where, "a.tags LIKE ?")
		args = append(args, "%"+q.Tag+"%")
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM articles a WHERE `+whereSQL, args...).Scan(&total)
	rows, err := s.DB.Query(`SELECT a.id, a.title, a.summary, a.body, a.tags, a.views, a.created_at, a.edited_at,
			u.username, u.role, u.avatar,
			(SELECT COUNT(*) FROM votes v WHERE v.target_type='article' AND v.target_id=a.id) AS score,
			(SELECT COUNT(*) FROM article_comments ac WHERE ac.article_id=a.id) AS comment_count
		FROM articles a JOIN users u ON u.id=a.user_id
		WHERE `+whereSQL+` ORDER BY a.created_at DESC LIMIT ? OFFSET ?`, append(args, size, (page-1)*size)...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0, size)
	for rows.Next() {
		var (
			id           int64
			title        string
			summary      string
			body         string
			tags         string
			views        int
			created      time.Time
			edited       sql.NullTime
			author       string
			role         string
			avatar       string
			score        int
			commentCount int
		)
		if rows.Scan(&id, &title, &summary, &body, &tags, &views, &created, &edited,
			&author, &role, &avatar, &score, &commentCount) == nil {
			items = append(items, gin.H{
				"id": id, "title": title, "summary": summary, "body": body, "tags": tags,
				"views": views, "author": author, "author_role": role, "author_avatar": avatar,
				"created_at": created, "edited_at": edited.Time,
				"score": score, "comment_count": commentCount,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": size})
}

// GetArticle 文章详情。
func (s *Server) GetArticle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var (
		title, summary, body, tags string
		views                      int
		published                  bool
		author, role, avatar       string
		created                    time.Time
		edited                     sql.NullTime
		score, commentCount        int
	)
	err = s.DB.QueryRow(`SELECT a.title, a.summary, a.body, a.tags, a.views, a.published,
			u.username, u.role, u.avatar, a.created_at, a.edited_at,
			(SELECT COUNT(*) FROM votes v WHERE v.target_type='article' AND v.target_id=a.id) AS score,
			(SELECT COUNT(*) FROM article_comments ac WHERE ac.article_id=a.id) AS comment_count
		FROM articles a JOIN users u ON u.id=a.user_id WHERE a.id=?`, id).
		Scan(&title, &summary, &body, &tags, &views, &published, &author, &role, &avatar, &created, &edited, &score, &commentCount)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if !published {
		// 未发布仅作者与管理员可见
		u, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
			return
		}
		var owner int64
		_ = s.DB.QueryRow(`SELECT user_id FROM articles WHERE id=?`, id).Scan(&owner)
		if u.Role != "admin" && u.ID != owner {
			c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"id": id, "title": title, "summary": summary, "body": body, "tags": tags,
		"views": views, "published": published, "author": author, "author_role": role,
		"author_avatar": avatar, "created_at": created, "edited_at": edited.Time,
		"score": score, "comment_count": commentCount,
	})
}

// ListArticleComments 文章评论列表。
func (s *Server) ListArticleComments(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	rows, err := s.DB.Query(`SELECT ac.id, ac.user_id, ac.body, ac.created_at, u.username, u.avatar
		FROM article_comments ac JOIN users u ON u.id=ac.user_id
		WHERE ac.article_id=? ORDER BY ac.created_at ASC`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			acid, uid int64
			body      string
			created   time.Time
			author    string
			avatar    string
		)
		if rows.Scan(&acid, &uid, &body, &created, &author, &avatar) == nil {
			items = append(items, gin.H{"id": acid, "user_id": uid, "body": body,
				"author": author, "avatar": avatar, "created_at": created})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// CreateArticleComment 发表文章评论。
func (s *Server) CreateArticleComment(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
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
	var exists int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM articles WHERE id=?`, id).Scan(&exists)
	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	res, err := s.DB.Exec(`INSERT INTO article_comments (article_id, user_id, body) VALUES (?, ?, ?)`,
		id, u.ID, req.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	cid, _ := res.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"id": cid})
}

// DeleteArticleComment 删除文章评论（作者或管理员）。
func (s *Server) DeleteArticleComment(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM article_comments WHERE id=?`, id).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "评论不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除"})
		return
	}
	_, _ = s.DB.Exec(`DELETE FROM article_comments WHERE id=?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type articleReq struct {
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	Body      string `json:"body"`
	Tags      string `json:"tags"`
	Published bool   `json:"published"`
}

func validateArticle(req *articleReq) (string, bool) {
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	req.Summary = strings.TrimSpace(req.Summary)
	req.Tags = strings.TrimSpace(req.Tags)
	if req.Title == "" || len([]rune(req.Title)) > 160 {
		return "标题不能为空且不超过 160 字", false
	}
	if req.Body == "" {
		return "内容不能为空", false
	}
	if len([]rune(req.Body)) > 20000 {
		return "内容过长（最多 20000 字）", false
	}
	if len([]rune(req.Summary)) > 300 {
		return "摘要过长（最多 300 字）", false
	}
	if len([]rune(req.Tags)) > 250 {
		return "标签过长", false
	}
	return "", true
}

// CreateArticle 发布知识文章。
func (s *Server) CreateArticle(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	var req articleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if msg, ok := validateArticle(&req); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	res, err := s.DB.Exec(`INSERT INTO articles (user_id, title, summary, body, tags, published) VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, req.Title, req.Summary, req.Body, req.Tags, req.Published)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// UpdateArticle 编辑文章（作者或管理员）。
func (s *Server) UpdateArticle(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var req articleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if msg, ok := validateArticle(&req); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM articles WHERE id=?`, id).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权编辑"})
		return
	}
	_, _ = s.DB.Exec(`UPDATE articles SET title=?, summary=?, body=?, tags=?, published=?, edited_at=? WHERE id=?`,
		req.Title, req.Summary, req.Body, req.Tags, req.Published, time.Now(), id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteArticle 删除文章（作者或管理员）。
func (s *Server) DeleteArticle(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM articles WHERE id=?`, id).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除"})
		return
	}
	_, _ = s.DB.Exec(`DELETE FROM articles WHERE id=?`, id)
	if u.Role == "admin" {
		s.audit(c, "delete_article", "article", &id, "管理员删除文章")
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RegisterArticleView 文章浏览计数。
func (s *Server) RegisterArticleView(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	_, _ = s.DB.Exec(`UPDATE articles SET views=views+1 WHERE id=?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
