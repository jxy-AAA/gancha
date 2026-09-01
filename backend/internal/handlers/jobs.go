package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"guangyanji/internal/middleware"

	"github.com/gin-gonic/gin"
)

// ListJobs 就业共享表格（2027 届公司招聘信息，社区协作数据库）。
// 支持地点（city LIKE）、方向（industry LIKE）、状态筛选；置顶优先，新建在前。
func (s *Server) ListJobs(c *gin.Context) {
	where := []string{"1=1"}
	args := []interface{}{}
	status := strings.TrimSpace(c.Query("status"))
	if status == "" || status == "active" {
		where = append(where, "j.status='active'")
	} else if status != "all" {
		where = append(where, "j.status=?")
		args = append(args, status)
	}
	if city := strings.TrimSpace(c.Query("city")); city != "" {
		where = append(where, "j.city LIKE ?")
		args = append(args, "%"+city+"%")
	}
	if industry := strings.TrimSpace(c.Query("industry")); industry != "" {
		where = append(where, "(j.industry LIKE ? OR j.company LIKE ? OR j.positions_27 LIKE ?)")
		args = append(args, "%"+industry+"%", "%"+industry+"%", "%"+industry+"%")
	}
	whereSQL := strings.Join(where, " AND ")
	rows, err := s.DB.Query(`SELECT j.id, j.user_id, j.company, j.industry, j.positions_27,
			j.confirm_level, j.strength, j.city, j.current_status, j.links, j.verified_at,
			j.status, j.is_pinned, j.edit_reason,
			j.created_at, j.updated_at,
			COALESCE(u.username, '官方'), COALESCE(le.username, u.username, '官方'),
			(SELECT COUNT(*) FROM job_reviews r WHERE r.job_id = j.id),
			(SELECT COUNT(*) FROM job_entry_versions v WHERE v.job_id = j.id)
		FROM job_entries j
		LEFT JOIN users u ON u.id=j.user_id
		LEFT JOIN users le ON le.id=j.last_editor_id
		WHERE `+whereSQL+`
		ORDER BY j.is_pinned DESC, j.created_at DESC, j.id DESC`, args...)
	if err != nil {
		log.Printf("ListJobs query err: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			id, uid   int64
			company   string
			industry  string
			positions string
			confirm   string
			strength  string
			city      string
			curStatus string
			links     string
			verified  string
			status    string
			pinned    bool
			reason    string
			created   time.Time
			updated   time.Time
			author    string
			updater   string
			reviewCnt int
			editCnt   int
		)
		if rows.Scan(&id, &uid, &company, &industry, &positions,
			&confirm, &strength, &city, &curStatus, &links, &verified,
			&status, &pinned, &reason,
			&created, &updated, &author, &updater, &reviewCnt, &editCnt) == nil {
			items = append(items, gin.H{
				"id": id, "user_id": uid, "company": company, "industry": industry,
				"positions_27": positions, "confirm_level": confirm, "strength": strength,
				"city": city, "current_status": curStatus, "links": links,
				"verified_at": verified, "status": status, "is_pinned": pinned,
				"edit_reason": reason, "created_at": created, "updated_at": updated,
				"author": author, "updater": updater, "review_count": reviewCnt,
				"edit_count": editCnt,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type jobReq struct {
	Company       string `json:"company"`
	Industry      string `json:"industry"`
	Positions27   string `json:"positions_27"`
	ConfirmLevel  string `json:"confirm_level"`
	Strength      string `json:"strength"`
	City          string `json:"city"`
	CurrentStatus string `json:"current_status"`
	Links         string `json:"links"`
	VerifiedAt    string `json:"verified_at"`
	EditReason    string `json:"edit_reason"`
}

func (r *jobReq) sanitize() string {
	r.Company = strings.TrimSpace(r.Company)
	r.Industry = strings.TrimSpace(r.Industry)
	r.Positions27 = strings.TrimSpace(r.Positions27)
	r.ConfirmLevel = strings.TrimSpace(r.ConfirmLevel)
	r.Strength = strings.TrimSpace(r.Strength)
	r.City = strings.TrimSpace(r.City)
	r.CurrentStatus = strings.TrimSpace(r.CurrentStatus)
	r.Links = strings.TrimSpace(r.Links)
	r.VerifiedAt = strings.TrimSpace(r.VerifiedAt)
	r.EditReason = strings.TrimSpace(r.EditReason)
	if r.Company == "" || len([]rune(r.Company)) > 100 {
		return "公司名称不能为空且不超过 100 字"
	}
	if len([]rune(r.Industry)) > 200 {
		return "产业链/光学方向不超过 200 字"
	}
	if len([]rune(r.Positions27)) > 500 {
		return "27届岗位不超过 500 字"
	}
	if len([]rune(r.ConfirmLevel)) > 300 {
		return "岗位确认度不超过 300 字"
	}
	if len([]rune(r.Strength)) > 10 {
		return "证据强度不超过 10 字"
	}
	if len([]rune(r.City)) > 100 {
		return "地点不超过 100 字"
	}
	if len([]rune(r.CurrentStatus)) > 200 {
		return "当前状态不超过 200 字"
	}
	if len([]rune(r.Links)) > 1000 {
		return "证据链接不超过 1000 字"
	}
	if len([]rune(r.VerifiedAt)) > 10 {
		return "核验日期格式不正确"
	}
	if len([]rune(r.EditReason)) > 200 {
		return "修改原因不超过 200 字"
	}
	return ""
}

// recordJobVersion 把一条记录的快照写入版本历史（用于修改、标记失效/重复、恢复等）。
func (s *Server) recordJobVersion(jobID, editorID int64, reason, company, industry, positions,
	confirm, strength, city, curStatus, links, verified string) {
	_, _ = s.DB.Exec(`INSERT INTO job_entry_versions
		(job_id, editor_id, reason, company, industry, positions_27, confirm_level, strength, city, current_status, links, verified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		jobID, editorID, reason, company, industry, positions,
		confirm, strength, city, curStatus, links, verified)
}

// CreateJob 新增一行（登录用户均可），同时记录初始版本。
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
	res, err := s.DB.Exec(`INSERT INTO job_entries (user_id, last_editor_id, company, industry, positions_27,
			confirm_level, strength, city, current_status, links, verified_at, status, is_pinned, edit_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', 0, ?)`,
		u.ID, u.ID, req.Company, req.Industry, req.Positions27,
		req.ConfirmLevel, req.Strength, req.City, req.CurrentStatus, req.Links, req.VerifiedAt, req.EditReason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	id, _ := res.LastInsertId()
	reason := req.EditReason
	if reason == "" {
		reason = "创建记录"
	}
	s.recordJobVersion(id, u.ID, reason, req.Company, req.Industry, req.Positions27,
		req.ConfirmLevel, req.Strength, req.City, req.CurrentStatus, req.Links, req.VerifiedAt)
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// UpdateJob 编辑任意行（共享表格，登录用户均可）：必须填写修改原因，自动记录版本、修改人、修改原因。
func (s *Server) UpdateJob(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
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
	if req.EditReason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "修改原因不能为空"})
		return
	}
	var (
		oldCompany, oldIndustry, oldPositions, oldConfirm, oldStrength string
		oldCity, oldStatus, oldLinks, oldVerified                     string
	)
	err = s.DB.QueryRow(`SELECT company, industry, positions_27, confirm_level, strength,
			city, current_status, links, verified_at FROM job_entries WHERE id=?`, id).
		Scan(&oldCompany, &oldIndustry, &oldPositions, &oldConfirm, &oldStrength,
			&oldCity, &oldStatus, &oldLinks, &oldVerified)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	res, err := s.DB.Exec(`UPDATE job_entries SET company=?, industry=?, positions_27=?, confirm_level=?,
			strength=?, city=?, current_status=?, links=?, verified_at=?, status='active',
			last_editor_id=?, edit_reason=?, updated_at=?
		WHERE id=?`,
		req.Company, req.Industry, req.Positions27, req.ConfirmLevel,
		req.Strength, req.City, req.CurrentStatus, req.Links, req.VerifiedAt,
		u.ID, req.EditReason, time.Now(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	s.recordJobVersion(id, u.ID, req.EditReason, oldCompany, oldIndustry, oldPositions,
		oldConfirm, oldStrength, oldCity, oldStatus, oldLinks, oldVerified)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// FlagJob 提交“失效/重复”标记（登录用户均可），替代删除；记录版本与原因。
func (s *Server) FlagJob(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var req struct {
		Flag   string `json:"flag"` // invalid | duplicate
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if req.Flag != "invalid" && req.Flag != "duplicate" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的标记类型"})
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" || len([]rune(req.Reason)) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写标记原因（不超过 200 字）"})
		return
	}
	var (
		company, industry, positions, confirm, strength string
		city, curStatus, links, verified                string
	)
	err = s.DB.QueryRow(`SELECT company, industry, positions_27, confirm_level, strength,
			city, current_status, links, verified_at FROM job_entries WHERE id=?`, id).
		Scan(&company, &industry, &positions, &confirm, &strength,
			&city, &curStatus, &links, &verified)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	_, err = s.DB.Exec(`UPDATE job_entries SET status=?, last_editor_id=?, edit_reason=?, updated_at=? WHERE id=?`,
		req.Flag, u.ID, req.Reason, time.Now(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	s.recordJobVersion(id, u.ID, "标记为"+
		map[string]string{"invalid": "失效", "duplicate": "重复"}[req.Flag]+"："+req.Reason,
		company, industry, positions, confirm, strength, city, curStatus, links, verified)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RestoreJob 恢复被标记的记录（登录用户均可）。
func (s *Server) RestoreJob(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var (
		company, industry, positions, confirm, strength string
		city, curStatus, links, verified, status        string
	)
	err = s.DB.QueryRow(`SELECT company, industry, positions_27, confirm_level, strength,
			city, current_status, links, verified_at, status FROM job_entries WHERE id=?`, id).
		Scan(&company, &industry, &positions, &confirm, &strength,
			&city, &curStatus, &links, &verified, &status)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if status == "active" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	_, err = s.DB.Exec(`UPDATE job_entries SET status='active', last_editor_id=?, edit_reason='恢复为有效记录', updated_at=? WHERE id=?`,
		u.ID, time.Now(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	s.recordJobVersion(id, u.ID, "恢复为有效记录",
		company, industry, positions, confirm, strength, city, curStatus, links, verified)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListJobVersions 记录的版本历史（含修改人、修改原因）。
func (s *Server) ListJobVersions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	rows, err := s.DB.Query(`SELECT v.id, v.editor_id, v.reason, v.company, v.industry, v.positions_27,
			v.confirm_level, v.strength, v.city, v.current_status, v.links, v.verified_at, v.created_at,
			COALESCE(u.username, '官方')
		FROM job_entry_versions v LEFT JOIN users u ON u.id=v.editor_id
		WHERE v.job_id=? ORDER BY v.id DESC`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			vid, eid  int64
			reason    string
			company   string
			industry  string
			positions string
			confirm   string
			strength  string
			city      string
			curStatus string
			links     string
			verified  string
			created   time.Time
			editor    string
		)
		if rows.Scan(&vid, &eid, &reason, &company, &industry, &positions,
			&confirm, &strength, &city, &curStatus, &links, &verified, &created, &editor) == nil {
			items = append(items, gin.H{
				"id": vid, "editor_id": eid, "editor": editor, "reason": reason,
				"company": company, "industry": industry, "positions_27": positions,
				"confirm_level": confirm, "strength": strength, "city": city,
				"current_status": curStatus, "links": links, "verified_at": verified,
				"created_at": created,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// RevertJobVersion 管理员恢复到任意历史版本（处理恶意修改与争议）。
func (s *Server) RevertJobVersion(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以恢复版本"})
		return
	}
	var req struct {
		VersionID int64 `json:"version_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	var (
		company, industry, positions, confirm, strength string
		city, curStatus, links, verified, reason        string
		editorID                                        int64
	)
	err = s.DB.QueryRow(`SELECT editor_id, reason, company, industry, positions_27, confirm_level,
			strength, city, current_status, links, verified_at
		FROM job_entry_versions WHERE id=? AND job_id=?`, req.VersionID, id).
		Scan(&editorID, &reason, &company, &industry, &positions, &confirm,
			&strength, &city, &curStatus, &links, &verified)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "版本不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	_, err = s.DB.Exec(`UPDATE job_entries SET company=?, industry=?, positions_27=?, confirm_level=?,
			strength=?, city=?, current_status=?, links=?, verified_at=?, status='active',
			last_editor_id=?, edit_reason=?, updated_at=?
		WHERE id=?`,
		company, industry, positions, confirm,
		strength, city, curStatus, links, verified,
		u.ID, "管理员恢复版本 #"+strconv.FormatInt(req.VersionID, 10)+"（"+reason+"）", time.Now(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	s.audit(c, "revert_job_version", "job", &id, "恢复版本 #"+strconv.FormatInt(req.VersionID, 10))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// PinJob 管理员置顶 / 取消置顶公司。
func (s *Server) PinJob(c *gin.Context) {
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
	res, err := s.DB.Exec(`UPDATE job_entries SET is_pinned=? WHERE id=?`, req.Pinned, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	s.audit(c, "pin_job", "job", &id, "置顶="+strconv.FormatBool(req.Pinned))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteJob 硬删除（仅管理员，用于清理）。
func (s *Server) DeleteJob(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	if u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以删除记录，普通用户请使用「标记失效/重复」"})
		return
	}
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
	_, _ = s.DB.Exec(`DELETE FROM job_entries WHERE id=?`, id)
	_, _ = s.DB.Exec(`DELETE FROM job_entry_versions WHERE job_id=?`, id)
	s.audit(c, "delete_job", "job", &id, "管理员删除记录")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
