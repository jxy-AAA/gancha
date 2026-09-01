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

// ListBoards 论坛板块列表（含帖子数）。
func (s *Server) ListBoards(c *gin.Context) {
	rows, err := s.DB.Query(`SELECT b.id, b.name, b.description, b.position,
			(SELECT COUNT(*) FROM forum_posts p WHERE p.board_id=b.id) AS post_count
		FROM forum_boards b ORDER BY b.position ASC, b.id ASC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			id          int64
			name        string
			desc        string
			position    int
			postCount   int
		)
		if rows.Scan(&id, &name, &desc, &position, &postCount) == nil {
			items = append(items, gin.H{"id": id, "name": name, "description": desc,
				"position": position, "post_count": postCount})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type listForumReq struct {
	BoardID  int64  `form:"board_id"`
	Search   string `form:"search"`
	Sort     string `form:"sort"` // latest | updated | popular（默认 latest）
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// ListForumPosts 帖子列表：板块筛选 + 关键词 + 排序 + 分页，置顶优先。
func (s *Server) ListForumPosts(c *gin.Context) {
	var q listForumReq
	_ = c.ShouldBindQuery(&q)
	page, size := clampPage(q.Page, q.PageSize)
	where := []string{"1=1"}
	args := []interface{}{}
	if q.BoardID > 0 {
		where = append(where, "p.board_id=?")
		args = append(args, q.BoardID)
	}
	if q.Search != "" {
		where = append(where, "(p.title LIKE ? OR p.body LIKE ?)")
		like := "%" + q.Search + "%"
		args = append(args, like, like)
	}
	whereSQL := strings.Join(where, " AND ")
	order := "p.created_at DESC"
	switch q.Sort {
	case "updated":
		order = "COALESCE((SELECT MAX(r.created_at) FROM forum_replies r WHERE r.post_id=p.id), p.created_at) DESC, p.id DESC"
	case "popular":
		order = "p.views DESC, p.id DESC"
	}
	var total int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM forum_posts p WHERE `+whereSQL, args...).Scan(&total)
	rows, err := s.DB.Query(`SELECT p.id, p.board_id, b.name, p.title, p.body, p.views, p.created_at, p.edited_at,
			p.is_pinned, p.is_solved, p.tags,
			CASE WHEN p.is_anonymous=1 THEN '匿名' ELSE u.username END,
			CASE WHEN p.is_anonymous=1 THEN '' ELSE u.avatar END,
			(SELECT COUNT(*) FROM forum_replies r WHERE r.post_id=p.id) AS reply_count,
			(SELECT MAX(r.created_at) FROM forum_replies r WHERE r.post_id=p.id) AS last_reply_at,
			(SELECT u2.username FROM forum_replies r JOIN users u2 ON u2.id=r.user_id
				WHERE r.post_id=p.id ORDER BY r.created_at DESC LIMIT 1) AS last_reply_author
		FROM forum_posts p
		JOIN users u ON u.id=p.user_id
		LEFT JOIN forum_boards b ON b.id=p.board_id
		WHERE `+whereSQL+` ORDER BY p.is_pinned DESC, `+order+` LIMIT ? OFFSET ?`, append(args, size, (page-1)*size)...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0, size)
	for rows.Next() {
		var (
			id, bid       int64
			bName         sql.NullString
			title         string
			body          string
			views         int
			created       time.Time
			edited        sql.NullTime
			pinned        bool
			solved        bool
			tags          string
			author        string
			avatar        string
			replyCnt      int
			lastReplyAt   sql.NullTime
			lastReplyAuth sql.NullString
		)
		if rows.Scan(&id, &bid, &bName, &title, &body, &views, &created, &edited,
			&pinned, &solved, &tags, &author, &avatar, &replyCnt,
			&lastReplyAt, &lastReplyAuth) == nil {
			items = append(items, gin.H{
				"id": id, "board_id": bid, "board_name": bName.String, "title": title, "body": body,
				"views": views, "author": author, "author_avatar": avatar, "reply_count": replyCnt,
				"is_pinned": pinned, "is_solved": solved, "tags": tags,
				"last_reply_at": lastReplyAt.Time, "last_reply_author": lastReplyAuth.String,
				"created_at": created, "edited_at": edited.Time,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": size})
}

// GetForumPost 帖子详情（含回复）。
func (s *Server) GetForumPost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	viewerID := int64(0)
	isAdmin := false
	if u, ok := middleware.CurrentUser(c); ok {
		viewerID = u.ID
		isAdmin = u.Role == "admin"
	}
	var (
		bid          int64
		bName        sql.NullString
		title        string
		body         string
		views        int
		author       string
		avatar       string
		created      time.Time
		edited       sql.NullTime
		replyCnt     int
		score        int
		userID       int64
		isAnonymous  bool
		isPinned     bool
		isSolved     bool
		tags         string
		lastReplyAt  sql.NullTime
	)
	err = s.DB.QueryRow(`SELECT p.board_id, b.name, p.title, p.body, p.views, u.username, u.avatar,
			p.created_at, p.edited_at, p.user_id, p.is_anonymous, p.is_pinned, p.is_solved, p.tags,
			(SELECT COUNT(*) FROM forum_replies r WHERE r.post_id=p.id),
			(SELECT COUNT(*) FROM votes v WHERE v.target_type='forum_post' AND v.target_id=p.id),
			(SELECT MAX(r.created_at) FROM forum_replies r WHERE r.post_id=p.id)
		FROM forum_posts p
		JOIN users u ON u.id=p.user_id
		LEFT JOIN forum_boards b ON b.id=p.board_id
		WHERE p.id=?`, id).
		Scan(&bid, &bName, &title, &body, &views, &author, &avatar, &created, &edited, &userID,
			&isAnonymous, &isPinned, &isSolved, &tags, &replyCnt, &score, &lastReplyAt)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "帖子不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if isAnonymous {
		author = "匿名"
		avatar = ""
		if viewerID != userID && !isAdmin {
			userID = 0
		}
	}
	replies, _ := s.listReplies(id)
	c.JSON(http.StatusOK, gin.H{
		"id": id, "board_id": bid, "board_name": bName.String, "title": title, "body": body,
		"views": views, "author": author, "author_avatar": avatar, "user_id": userID,
		"is_pinned": isPinned, "is_solved": isSolved, "tags": tags,
		"last_reply_at": lastReplyAt.Time,
		"created_at": created, "edited_at": edited.Time, "reply_count": replyCnt, "score": score,
		"replies": replies,
	})
}

func (s *Server) listReplies(postID int64) ([]gin.H, error) {
	rows, err := s.DB.Query(`SELECT r.id, r.user_id, r.body, r.created_at, u.username, u.avatar,
			(SELECT COUNT(*) FROM votes v WHERE v.target_type='forum_reply' AND v.target_id=r.id)
		FROM forum_replies r JOIN users u ON u.id=r.user_id
		WHERE r.post_id=? ORDER BY r.created_at ASC`, postID)
	if err != nil {
		return nil, err
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
			score   int
		)
		if rows.Scan(&id, &uid, &body, &created, &author, &avatar, &score) == nil {
			items = append(items, gin.H{"id": id, "user_id": uid, "body": body, "author": author,
				"avatar": avatar, "created_at": created, "score": score})
		}
	}
	return items, nil
}

type forumPostReq struct {
	BoardID     int64  `json:"board_id"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Tags        string `json:"tags"`
	IsAnonymous bool   `json:"is_anonymous"`
	IsSolved    bool   `json:"is_solved"`
}

// CreateForumPost 发帖。
func (s *Server) CreateForumPost(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	var req forumPostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	req.Tags = strings.TrimSpace(req.Tags)
	if req.Title == "" || len([]rune(req.Title)) > 160 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题不能为空且不超过 160 字"})
		return
	}
	if req.Body == "" || len([]rune(req.Body)) > 20000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容不能为空且不超过 20000 字"})
		return
	}
	if len([]rune(req.Tags)) > 250 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标签不超过 250 字"})
		return
	}
	var exists int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM forum_boards WHERE id=?`, req.BoardID).Scan(&exists)
	if exists == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "板块不存在"})
		return
	}
	res, err := s.DB.Exec(`INSERT INTO forum_posts (board_id, user_id, title, body, tags, is_anonymous) VALUES (?, ?, ?, ?, ?, ?)`,
		req.BoardID, u.ID, req.Title, req.Body, req.Tags, req.IsAnonymous)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// UpdateForumPost 编辑帖子（作者或管理员）。
func (s *Server) UpdateForumPost(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var req forumPostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	req.Tags = strings.TrimSpace(req.Tags)
	if req.Title == "" || len([]rune(req.Title)) > 160 || req.Body == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题与内容不能为空"})
		return
	}
	if len([]rune(req.Tags)) > 250 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标签不超过 250 字"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM forum_posts WHERE id=?`, id).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "帖子不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权编辑"})
		return
	}
	_, _ = s.DB.Exec(`UPDATE forum_posts SET board_id=?, title=?, body=?, tags=?, is_solved=?, edited_at=? WHERE id=?`,
		req.BoardID, req.Title, req.Body, req.Tags, req.IsSolved, time.Now(), id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// PinForumPost 管理员置顶 / 取消置顶帖子。
func (s *Server) PinForumPost(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以置顶"})
		return
	}
	var req struct {
		Pinned bool `json:"pinned"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	res, err := s.DB.Exec(`UPDATE forum_posts SET is_pinned=? WHERE id=?`, req.Pinned, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "帖子不存在"})
		return
	}
	s.audit(c, "pin_forum_post", "forum_post", &id, "置顶="+strconv.FormatBool(req.Pinned))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteForumPost 删除帖子（作者或管理员）。
func (s *Server) DeleteForumPost(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM forum_posts WHERE id=?`, id).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "帖子不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除"})
		return
	}
	_, _ = s.DB.Exec(`DELETE FROM forum_posts WHERE id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM forum_replies WHERE post_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM votes WHERE target_type='forum_post' AND target_id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM votes WHERE target_type='forum_reply' AND target_id IN (SELECT id FROM forum_replies WHERE post_id=?)`, id)
	if u.Role == "admin" {
		s.audit(c, "delete_forum_post", "forum_post", &id, "管理员删除帖子")
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// CreateForumReply 回复帖子。
func (s *Server) CreateForumReply(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	pid, err := strconv.ParseInt(c.Param("id"), 10, 64)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "回复内容不能为空且不超过 12000 字"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM forum_posts WHERE id=?`, pid).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "帖子不存在"})
		return
	}
	res, err := s.DB.Exec(`INSERT INTO forum_replies (post_id, user_id, body) VALUES (?, ?, ?)`, pid, u.ID, req.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	rid, _ := res.LastInsertId()
	if owner != u.ID {
		pidP := pid
		s.notify(owner, "reply", "你的论坛帖子收到了新回复", &pidP)
	}
	c.JSON(http.StatusOK, gin.H{"id": rid})
}

// UpdateForumReply 编辑回复（作者或管理员）。
func (s *Server) UpdateForumReply(c *gin.Context) {
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
	if req.Body == "" || len([]rune(req.Body)) > 12000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "回复内容不合法"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM forum_replies WHERE id=?`, id).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "回复不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权编辑"})
		return
	}
	_, _ = s.DB.Exec(`UPDATE forum_replies SET body=? WHERE id=?`, req.Body, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteForumReply 删除回复（作者或管理员）。
func (s *Server) DeleteForumReply(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var owner int64
	if err := s.DB.QueryRow(`SELECT user_id FROM forum_replies WHERE id=?`, id).Scan(&owner); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "回复不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除"})
		return
	}
	_, _ = s.DB.Exec(`DELETE FROM forum_replies WHERE id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM votes WHERE target_type='forum_reply' AND target_id=?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RegisterForumPostView 帖子浏览计数。
func (s *Server) RegisterForumPostView(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	_, _ = s.DB.Exec(`UPDATE forum_posts SET views=views+1 WHERE id=?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
