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

// ---- 列表 ----

type listQuestionReq struct {
	Category int64  `form:"category"`
	Search   string `form:"search"`
	Sort     string `form:"sort"` // popular | newest（默认 newest）
	Status   string `form:"status"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// ListQuestions 问题列表：分页 + 分类 + 关键词 + 排序。
func (s *Server) ListQuestions(c *gin.Context) {
	var q listQuestionReq
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	page, size := clampPage(q.Page, q.PageSize)
	where := []string{"1=1"}
	args := []interface{}{}
	if q.Category > 0 {
		where = append(where, "q.category_id=?")
		args = append(args, q.Category)
	}
	if q.Search != "" {
		where = append(where, "(q.title LIKE ? OR q.body LIKE ? OR q.tags LIKE ?)")
		like := "%" + q.Search + "%"
		args = append(args, like, like, like)
	}
	if q.Status != "" {
		where = append(where, "q.status=?")
		args = append(args, q.Status)
	}
	whereSQL := strings.Join(where, " AND ")
	order := "q.created_at DESC"
	if q.Sort == "popular" {
		order = "q.views DESC, q.id DESC"
	}
	var total int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM questions q WHERE `+whereSQL, args...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	rows, err := s.DB.Query(`SELECT q.id, q.category_id, cat.name, q.title, q.body, q.tags, q.views,
			q.status, q.accepted_answer_id, u.username, u.avatar, q.created_at, q.edited_at,
			(SELECT COUNT(*) FROM answers a WHERE a.question_id=q.id) AS answer_count,
			(SELECT COUNT(*) FROM votes v WHERE v.target_type='question' AND v.target_id=q.id) AS score,
			(SELECT COUNT(*) FROM comments cm WHERE cm.question_id=q.id) AS comment_count
		FROM questions q
		JOIN users u ON u.id=q.user_id
		LEFT JOIN categories cat ON cat.id=q.category_id
		WHERE `+whereSQL+` ORDER BY `+order+` LIMIT ? OFFSET ?`, append(args, size, (page-1)*size)...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0, size)
	for rows.Next() {
		var (
			item       gin.H
			id, catID  int64
			catName    sql.NullString
			title, tag string
			body       string
			views      int
			status     string
			accepted   sql.NullInt64
			author     string
			avatar     sql.NullString
			created    time.Time
			edited     sql.NullTime
			answers    int
			score      int
			commentCnt int
		)
		if err := rows.Scan(&id, &catID, &catName, &title, &body, &tag, &views, &status, &accepted,
			&author, &avatar, &created, &edited, &answers, &score, &commentCnt); err != nil {
			continue
		}
		item = gin.H{
			"id": id, "category_id": catID, "category_name": catName.String,
			"title": title, "body": body, "tags": tag, "views": views, "status": status,
			"accepted_answer_id": accepted.Int64, "author": author, "author_avatar": avatar.String,
			"created_at": created, "edited_at": edited.Time, "answer_count": answers, "score": score,
			"comment_count": commentCnt,
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": size})
}

// ---- 详情 ----

// GetQuestion 问题详情（含回答列表与评论数）。
func (s *Server) GetQuestion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var (
		qid, catID  int64
		catName     sql.NullString
		title, body string
		tags        string
		views       int
		status      string
		accepted    sql.NullInt64
		author      string
		avatar      sql.NullString
		created     time.Time
		edited      sql.NullTime
		score       int
	)
	err = s.DB.QueryRow(`SELECT q.id, q.category_id, cat.name, q.title, q.body, q.tags, q.views,
			q.status, q.accepted_answer_id, u.username, u.avatar, q.created_at, q.edited_at,
			(SELECT COUNT(*) FROM votes v WHERE v.target_type='question' AND v.target_id=q.id)
		FROM questions q
		JOIN users u ON u.id=q.user_id
		LEFT JOIN categories cat ON cat.id=q.category_id
		WHERE q.id=?`, id).
		Scan(&qid, &catID, &catName, &title, &body, &tags, &views, &status, &accepted,
			&author, &avatar, &created, &edited, &score)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "问题不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": qid, "category_id": catID, "category_name": catName.String,
		"title": title, "body": body, "tags": tags, "views": views, "status": status,
		"accepted_answer_id": accepted.Int64, "author": author, "author_avatar": avatar.String,
		"created_at": created, "edited_at": edited.Time, "score": score,
	})
}

// RegisterView 问题浏览计数（按 IP+日期去重，简化实现只累加）。
func (s *Server) RegisterView(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	_, _ = s.DB.Exec(`UPDATE questions SET views=views+1 WHERE id=?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---- 发布 / 编辑 / 删除 ----

type questionReq struct {
	CategoryID int64  `json:"category_id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Tags       string `json:"tags"`
}

func validateQuestion(req *questionReq) (string, bool) {
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
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
	if len([]rune(req.Tags)) > 250 {
		return "标签过长", false
	}
	return "", true
}

// CreateQuestion 发布问题。
func (s *Server) CreateQuestion(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	var req questionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if msg, ok := validateQuestion(&req); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	var catName string
	if err := s.DB.QueryRow(`SELECT name FROM categories WHERE id=? AND active=1`, req.CategoryID).Scan(&catName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分类不存在"})
		return
	}
	res, err := s.DB.Exec(`INSERT INTO questions (user_id, category_id, title, body, tags) VALUES (?, ?, ?, ?, ?)`,
		u.ID, req.CategoryID, req.Title, req.Body, req.Tags)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// UpdateQuestion 编辑问题（作者或管理员）。
func (s *Server) UpdateQuestion(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var req questionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if msg, ok := validateQuestion(&req); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM questions WHERE id=?`, id).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "问题不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权编辑"})
		return
	}
	_, err = s.DB.Exec(`UPDATE questions SET category_id=?, title=?, body=?, tags=?, edited_at=? WHERE id=?`,
		req.CategoryID, req.Title, req.Body, req.Tags, time.Now(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteQuestion 删除问题（作者或管理员）。
func (s *Server) DeleteQuestion(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM questions WHERE id=?`, id).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "问题不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除"})
		return
	}
	_, _ = s.DB.Exec(`DELETE FROM questions WHERE id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM answers WHERE question_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM comments WHERE question_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM votes WHERE target_type='question' AND target_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM votes WHERE target_type='answer' AND target_id IN (SELECT id FROM answers WHERE question_id=?)`, id)
	_, _ = s.DB.Exec(`DELETE FROM bookmarks WHERE question_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM question_follows WHERE question_id=?`, id)
	if u.Role == "admin" {
		s.audit(c, "delete_question", "question", &id, "管理员删除问题")
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---- 回答 ----

type answerReq struct {
	Body string `json:"body"`
}

// ListAnswers 问题下的回答列表。
func (s *Server) ListAnswers(c *gin.Context) {
	qid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var accepted sql.NullInt64
	_ = s.DB.QueryRow(`SELECT accepted_answer_id FROM questions WHERE id=?`, qid).Scan(&accepted)
	rows, err := s.DB.Query(`SELECT a.id, a.user_id, a.body, a.created_at, a.edited_at, u.username, u.role, u.avatar,
			(SELECT COUNT(*) FROM votes v WHERE v.target_type='answer' AND v.target_id=a.id) AS score
		FROM answers a JOIN users u ON u.id=a.user_id
		WHERE a.question_id=? ORDER BY a.created_at ASC`, qid)
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
			edited  sql.NullTime
			author  string
			role    string
			avatar  string
			score   int
		)
		if err := rows.Scan(&id, &uid, &body, &created, &edited, &author, &role, &avatar, &score); err != nil {
			continue
		}
		items = append(items, gin.H{
			"id": id, "user_id": uid, "body": body, "author": author, "author_role": role,
			"avatar": avatar, "score": score, "accepted": accepted.Valid && accepted.Int64 == id,
			"created_at": created, "edited_at": edited.Time,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "accepted_answer_id": accepted.Int64})
}

// CreateAnswer 回答问题。
func (s *Server) CreateAnswer(c *gin.Context) {
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
	if req.Body == "" || len([]rune(req.Body)) > 12000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "回答内容不能为空且不超过 12000 字"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM questions WHERE id=?`, qid).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "问题不存在"})
		return
	}
	res, err := s.DB.Exec(`INSERT INTO answers (question_id, user_id, body) VALUES (?, ?, ?)`, qid, u.ID, req.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	aid, _ := res.LastInsertId()
	if owner != u.ID {
		qidP := qid
		s.notify(owner, "answer", "你的问题收到了新的回答", &qidP)
	}
	c.JSON(http.StatusOK, gin.H{"id": aid})
}

// UpdateAnswer 编辑回答（作者或管理员）。
func (s *Server) UpdateAnswer(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	aid, err := strconv.ParseInt(c.Param("id"), 10, 64)
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
	if req.Body == "" || len([]rune(req.Body)) > 12000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "回答内容不合法"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM answers WHERE id=?`, aid).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "回答不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权编辑"})
		return
	}
	_, _ = s.DB.Exec(`UPDATE answers SET body=?, edited_at=? WHERE id=?`, req.Body, time.Now(), aid)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteAnswer 删除回答（作者或管理员）。
func (s *Server) DeleteAnswer(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	aid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM answers WHERE id=?`, aid).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "回答不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除"})
		return
	}
	_, _ = s.DB.Exec(`DELETE FROM answers WHERE id=?`, aid)
	_, _ = s.DB.Exec(`DELETE FROM votes WHERE target_type='answer' AND target_id=?`, aid)
	_, _ = s.DB.Exec(`UPDATE questions SET accepted_answer_id=NULL WHERE accepted_answer_id=?`, aid)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// AcceptAnswer 采纳回答（仅问题作者）。
func (s *Server) AcceptAnswer(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	aid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var qid, owner int64
	if err := s.DB.QueryRow(`SELECT question_id, user_id FROM answers WHERE id=?`, aid).Scan(&qid, &owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "回答不存在"})
		return
	}
	var qOwner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM questions WHERE id=?`, qid).Scan(&qOwner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "问题不存在"})
		return
	}
	if qOwner != u.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅问题作者可以采纳回答"})
		return
	}
	_, _ = s.DB.Exec(`UPDATE questions SET accepted_answer_id=?, status='solved' WHERE id=?`, aid, qid)
	if owner != u.ID {
		qidP := qid
		s.notify(owner, "accept", "你的回答被采纳了", &qidP)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// CloseQuestion 关闭问题（作者或管理员）。
func (s *Server) CloseQuestion(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM questions WHERE id=?`, id).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "问题不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权操作"})
		return
	}
	_, _ = s.DB.Exec(`UPDATE questions SET status='closed' WHERE id=?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---- 书签 / 关注 ----

func (s *Server) ToggleBookmark(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	qid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var exists int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM bookmarks WHERE user_id=? AND question_id=?`, u.ID, qid).Scan(&exists)
	if exists > 0 {
		_, _ = s.DB.Exec(`DELETE FROM bookmarks WHERE user_id=? AND question_id=?`, u.ID, qid)
		c.JSON(http.StatusOK, gin.H{"bookmarked": false})
		return
	}
	_, _ = s.DB.Exec(`INSERT INTO bookmarks (user_id, question_id) VALUES (?, ?)`, u.ID, qid)
	c.JSON(http.StatusOK, gin.H{"bookmarked": true})
}

func (s *Server) ListBookmarks(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	rows, err := s.DB.Query(`SELECT q.id, q.title FROM bookmarks b JOIN questions q ON q.id=b.question_id
		WHERE b.user_id=? ORDER BY b.created_at DESC`, u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var title string
		if rows.Scan(&id, &title) == nil {
			items = append(items, gin.H{"id": id, "title": title})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (s *Server) ToggleQuestionFollow(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	qid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var exists int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM question_follows WHERE user_id=? AND question_id=?`, u.ID, qid).Scan(&exists)
	if exists > 0 {
		_, _ = s.DB.Exec(`DELETE FROM question_follows WHERE user_id=? AND question_id=?`, u.ID, qid)
		c.JSON(http.StatusOK, gin.H{"following": false})
		return
	}
	_, _ = s.DB.Exec(`INSERT INTO question_follows (user_id, question_id) VALUES (?, ?)`, u.ID, qid)
	c.JSON(http.StatusOK, gin.H{"following": true})
}

func clampPage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if size > 50 {
		size = 50
	}
	return page, size
}
