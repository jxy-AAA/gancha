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
// 支持地点（city LIKE）、方向（industry LIKE）、状态筛选；置顶优先（按 pin_order 手动排序），
// 其余按最近更新时间在前：有人编辑/恢复后该行自动上浮到非置顶区第一位。
func (s *Server) ListJobs(c *gin.Context) {
	// 登录用户附带其“是否投递”状态；匿名（OptionalAuth 未命中）ID 为 0，EXISTS 恒为 false
	u, _ := middleware.CurrentUser(c)
	where := []string{"1=1"}
	args := []interface{}{u.ID}
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
		where = append(where, "(j.industry LIKE ? OR j.company LIKE ?)")
		args = append(args, "%"+industry+"%", "%"+industry+"%")
	}
	whereSQL := strings.Join(where, " AND ")
	rows, err := s.DB.Query(`SELECT j.id, j.user_id, j.company, j.industry,
			j.city, j.apply_link, j.referral_code, j.verified_at, j.campus_status,
			j.status, j.is_pinned, j.edit_reason,
			j.created_at, j.updated_at,
			COALESCE(u.username, '官方'), COALESCE(le.username, u.username, '官方'),
			(SELECT COUNT(*) FROM job_reviews r WHERE r.job_id = j.id),
			(SELECT COUNT(*) FROM job_entry_versions v WHERE v.job_id = j.id),
			EXISTS(SELECT 1 FROM job_applications a WHERE a.job_id=j.id AND a.user_id=?) AS my_applied
		FROM job_entries j
		LEFT JOIN users u ON u.id=j.user_id
		LEFT JOIN users le ON le.id=j.last_editor_id
		WHERE `+whereSQL+`
		ORDER BY j.is_pinned DESC, j.pin_order ASC, j.updated_at DESC, j.id DESC`, args...)
	if err != nil {
		log.Printf("ListJobs query err: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			id, uid  int64
			company  string
			industry string
			city     string
			apply    string
			referral string
			verified string
			campus   string
			status   string
			pinned   bool
			reason   string
			created  time.Time
			updated  time.Time
			author   string
			updater  string
			reviewCnt int
			editCnt   int
			applied   bool
		)
		if rows.Scan(&id, &uid, &company, &industry,
			&city, &apply, &referral, &verified, &campus,
			&status, &pinned, &reason,
			&created, &updated, &author, &updater, &reviewCnt, &editCnt, &applied) == nil {
			items = append(items, gin.H{
				"id": id, "user_id": uid, "company": company, "industry": industry,
				"city": city, "apply_link": apply, "referral_code": referral,
				"verified_at": verified, "campus_status": campus, "status": status,
				"is_pinned": pinned, "edit_reason": reason,
				"created_at": created, "updated_at": updated,
				"author": author, "updater": updater, "review_count": reviewCnt,
				"edit_count": editCnt, "my_applied": applied,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type jobReq struct {
	Company      string `json:"company"`
	Industry     string `json:"industry"`
	City         string `json:"city"`
	ApplyLink    string `json:"apply_link"`
	ReferralCode string `json:"referral_code"`
	VerifiedAt   string `json:"verified_at"`
	CampusStatus string `json:"campus_status"`
	EditReason   string `json:"edit_reason"`
}

func (r *jobReq) sanitize() string {
	r.Company = strings.TrimSpace(r.Company)
	r.Industry = strings.TrimSpace(r.Industry)
	r.City = strings.TrimSpace(r.City)
	r.ApplyLink = strings.TrimSpace(r.ApplyLink)
	r.ReferralCode = strings.TrimSpace(r.ReferralCode)
	r.VerifiedAt = strings.TrimSpace(r.VerifiedAt)
	r.CampusStatus = strings.TrimSpace(r.CampusStatus)
	r.EditReason = strings.TrimSpace(r.EditReason)
	if r.Company == "" || len([]rune(r.Company)) > 100 {
		return "公司名称不能为空且不超过 100 字"
	}
	if len([]rune(r.Industry)) > 200 {
		return "产业链/光学方向不超过 200 字"
	}
	if len([]rune(r.City)) > 100 {
		return "地点不超过 100 字"
	}
	if len([]rune(r.ApplyLink)) > 1000 {
		return "投递链接不超过 1000 字"
	}
	if len([]rune(r.ReferralCode)) > 50 {
		return "内推码不超过 50 字"
	}
	if len([]rune(r.VerifiedAt)) > 10 {
		return "核验日期格式不正确"
	}
	if r.CampusStatus != "" && r.CampusStatus != "已开启" && r.CampusStatus != "待核验" {
		return "校招状态仅支持：已开启 / 待核验"
	}
	if len([]rune(r.EditReason)) > 200 {
		return "修改原因不超过 200 字"
	}
	return ""
}

// recordJobVersion 把一条记录的快照写入版本历史（用于修改、标记失效/重复、恢复等）。
func (s *Server) recordJobVersion(jobID, editorID int64, reason, company, industry,
	city, applyLink, referralCode, verified, campus string) {
	_, _ = s.DB.Exec(`INSERT INTO job_entry_versions
		(job_id, editor_id, reason, company, industry, city, apply_link, referral_code, verified_at, campus_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		jobID, editorID, reason, company, industry,
		city, applyLink, referralCode, verified, campus)
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
	// 填了内推码就自动置顶（蓝色效果，pin_manual=0 表示由内推码驱动），置顶序号排到最后
	pinned := 0
	pinOrder := 0
	if req.ReferralCode != "" {
		pinned = 1
		if err := s.DB.QueryRow(`SELECT COALESCE(MAX(pin_order),0)+1 FROM job_entries`).Scan(&pinOrder); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
			return
		}
	}
	// 校招状态仅管理员可指定（默认待核验，需核验后才可标已开启）
	campus := "待核验"
	if u.Role == "admin" && req.CampusStatus != "" {
		campus = req.CampusStatus
	}
	res, err := s.DB.Exec(`INSERT INTO job_entries (user_id, last_editor_id, company, industry,
			city, apply_link, referral_code, verified_at, campus_status, status, is_pinned, pin_manual, pin_order, edit_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, 0, ?, ?)`,
		u.ID, u.ID, req.Company, req.Industry,
		req.City, req.ApplyLink, req.ReferralCode, req.VerifiedAt, campus, pinned, pinOrder, req.EditReason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	id, _ := res.LastInsertId()
	reason := req.EditReason
	if reason == "" {
		reason = "创建记录"
	}
	s.recordJobVersion(id, u.ID, reason, req.Company, req.Industry,
		req.City, req.ApplyLink, req.ReferralCode, req.VerifiedAt, campus)
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
		oldCompany, oldIndustry string
		oldCity, oldApply       string
		oldReferral, oldVerified string
		oldCampus               string
		oldPinned               bool
		oldPinOrder             int
		oldPinManual            bool
	)
	err = s.DB.QueryRow(`SELECT company, industry,
			city, apply_link, referral_code, verified_at, campus_status,
			is_pinned, pin_order, pin_manual FROM job_entries WHERE id=?`, id).
		Scan(&oldCompany, &oldIndustry,
			&oldCity, &oldApply, &oldReferral, &oldVerified, &oldCampus,
			&oldPinned, &oldPinOrder, &oldPinManual)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	// 置顶规则：有内推码自动置顶（删码即取消）；管理员手动置顶（pin_manual=1）保留，不被用户编辑清除
	pinned := req.ReferralCode != "" || oldPinManual
	// 置顶序号：保持置顶时不动；新置顶（补内推码）排到置顶区最后；取消置顶时归零
	pinOrder := oldPinOrder
	if pinned && !oldPinned {
		if err := s.DB.QueryRow(`SELECT COALESCE(MAX(pin_order),0)+1 FROM job_entries`).Scan(&pinOrder); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
			return
		}
	} else if !pinned {
		pinOrder = 0
	}
	// 校招状态仅管理员可改，非管理员编辑时保持原值
	newCampus := oldCampus
	if u.Role == "admin" && req.CampusStatus != "" {
		newCampus = req.CampusStatus
	}
	res, err := s.DB.Exec(`UPDATE job_entries SET company=?, industry=?, city=?, apply_link=?,
			referral_code=?, verified_at=?, campus_status=?, status='active', is_pinned=?, pin_manual=?,
			pin_order=?, last_editor_id=?, edit_reason=?, updated_at=?
		WHERE id=?`,
		req.Company, req.Industry, req.City, req.ApplyLink,
		req.ReferralCode, req.VerifiedAt, newCampus, pinned, oldPinManual, pinOrder,
		u.ID, req.EditReason, time.Now(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	s.recordJobVersion(id, u.ID, req.EditReason, oldCompany, oldIndustry,
		oldCity, oldApply, oldReferral, oldVerified, oldCampus)
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
		company, industry string
		city, apply       string
		referral, verified, campus string
	)
	err = s.DB.QueryRow(`SELECT company, industry,
			city, apply_link, referral_code, verified_at, campus_status FROM job_entries WHERE id=?`, id).
		Scan(&company, &industry,
			&city, &apply, &referral, &verified, &campus)
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
		company, industry, city, apply, referral, verified, campus)
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
		company, industry string
		city, apply       string
		referral, verified, campus, status string
	)
	err = s.DB.QueryRow(`SELECT company, industry,
			city, apply_link, referral_code, verified_at, campus_status, status FROM job_entries WHERE id=?`, id).
		Scan(&company, &industry,
			&city, &apply, &referral, &verified, &campus, &status)
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
		company, industry, city, apply, referral, verified, campus)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// SetJobApplied 登录用户勾选/取消自己的“是否投递”状态。仅记录在 job_applications 表，
// 不触碰 job_entries，因此不影响列表排序与置顶。
func (s *Server) SetJobApplied(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var req struct {
		Applied bool `json:"applied"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM job_entries WHERE id=?`, id).Scan(&n); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	if req.Applied {
		_, err = s.DB.Exec(`INSERT IGNORE INTO job_applications (user_id, job_id) VALUES (?, ?)`, u.ID, id)
	} else {
		_, err = s.DB.Exec(`DELETE FROM job_applications WHERE user_id=? AND job_id=?`, u.ID, id)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListJobVersions 记录的版本历史（含修改人、修改原因）。
func (s *Server) ListJobVersions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	rows, err := s.DB.Query(`SELECT v.id, v.editor_id, v.reason, v.company, v.industry,
			v.city, v.apply_link, v.referral_code, v.verified_at, v.created_at,
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
			vid, eid int64
			reason   string
			company  string
			industry string
			city     string
			apply    string
			referral string
			verified string
			created  time.Time
			editor   string
		)
		if rows.Scan(&vid, &eid, &reason, &company, &industry,
			&city, &apply, &referral, &verified, &created, &editor) == nil {
			items = append(items, gin.H{
				"id": vid, "editor_id": eid, "editor": editor, "reason": reason,
				"company": company, "industry": industry, "city": city,
				"apply_link": apply, "referral_code": referral, "verified_at": verified,
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
		company, industry string
		city, apply       string
		referral, verified, campus, reason string
		editorID          int64
	)
	err = s.DB.QueryRow(`SELECT editor_id, reason, company, industry,
			city, apply_link, referral_code, verified_at, campus_status
		FROM job_entry_versions WHERE id=? AND job_id=?`, req.VersionID, id).
		Scan(&editorID, &reason, &company, &industry,
			&city, &apply, &referral, &verified, &campus)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "版本不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	_, err = s.DB.Exec(`UPDATE job_entries SET company=?, industry=?, city=?, apply_link=?,
			referral_code=?, verified_at=?, campus_status=?, status='active',
			last_editor_id=?, edit_reason=?, updated_at=?
		WHERE id=?`,
		company, industry, city, apply,
		referral, verified, campus,
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
	var oldPinned bool
	var oldPinOrder int
	err = s.DB.QueryRow(`SELECT is_pinned, pin_order FROM job_entries WHERE id=?`, id).
		Scan(&oldPinned, &oldPinOrder)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	pinManual := 0
	pinOrder := 0
	if req.Pinned {
		pinManual = 1
		if !oldPinned {
			// 新置顶排到置顶区最后；已置顶则保持原序号
			if err := s.DB.QueryRow(`SELECT COALESCE(MAX(pin_order),0)+1 FROM job_entries`).Scan(&pinOrder); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
				return
			}
		} else {
			pinOrder = oldPinOrder
		}
	}
	res, err := s.DB.Exec(`UPDATE job_entries SET is_pinned=?, pin_manual=?, pin_order=? WHERE id=?`,
		req.Pinned, pinManual, pinOrder, id)
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

// MoveJob 管理员在置顶区上移/下移记录（仅 active 置顶行参与相邻判断）。
func (s *Server) MoveJob(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可以调整置顶顺序"})
		return
	}
	var req struct {
		Direction string `json:"direction"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if req.Direction != "up" && req.Direction != "down" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "方向仅支持 up / down"})
		return
	}
	var pinned bool
	err = s.DB.QueryRow(`SELECT is_pinned FROM job_entries WHERE id=?`, id).Scan(&pinned)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if !pinned {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该记录未置顶，无需移动"})
		return
	}
	tx, err := s.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	rows, err := tx.Query(`SELECT id FROM job_entries
		WHERE is_pinned=1 AND status='active'
		ORDER BY pin_order ASC, created_at DESC, id DESC`)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	ids := make([]int64, 0)
	for rows.Next() {
		var pid int64
		if err := rows.Scan(&pid); err != nil {
			rows.Close()
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
			return
		}
		ids = append(ids, pid)
	}
	rows.Close()
	idx := -1
	for i, pid := range ids {
		if pid == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "该记录已失效或不在置顶区"})
		return
	}
	if (req.Direction == "up" && idx == 0) || (req.Direction == "down" && idx == len(ids)-1) {
		tx.Rollback()
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	// 先按当前展示序整体重编号，再与相邻行交换位置
	for i, pid := range ids {
		if _, err := tx.Exec(`UPDATE job_entries SET pin_order=? WHERE id=?`, i+1, pid); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
			return
		}
	}
	neighbor := idx - 1
	if req.Direction == "down" {
		neighbor = idx + 1
	}
	if _, err := tx.Exec(`UPDATE job_entries SET pin_order=? WHERE id=?`, neighbor+1, ids[idx]); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if _, err := tx.Exec(`UPDATE job_entries SET pin_order=? WHERE id=?`, idx+1, ids[neighbor]); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器错误"})
		return
	}
	s.audit(c, "move_job", "job", &id, req.Direction)
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
