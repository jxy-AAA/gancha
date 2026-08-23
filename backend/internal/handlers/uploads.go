package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"guangyanji/internal/middleware"

	"github.com/gin-gonic/gin"
)

var allowedExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true,
	".pdf": true, ".txt": true, ".csv": true, ".zip": true, ".zmx": true, ".zar": true,
}

const (
	maxFileSize = 10 << 20 // 10 MB
	maxFiles    = 5
)

// UploadFiles multipart 附件上传：字段名 files，最多 5 个，每个 ≤10MB，扩展名白名单。
func (s *Server) UploadFiles(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	if err := c.Request.ParseMultipartForm(maxFileSize * maxFiles); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "上传大小超出限制"})
		return
	}
	files := c.Request.MultipartForm.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未选择文件"})
		return
	}
	if len(files) > maxFiles {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("最多上传 %d 个文件", maxFiles)})
		return
	}
	results := make([]gin.H, 0, len(files))
	for _, fh := range files {
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if !allowedExt[ext] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的文件类型: " + fh.Filename})
			return
		}
		if fh.Size > maxFileSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("文件 %s 超过 10MB 限制", fh.Filename)})
			return
		}
		src, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
			return
		}
		name := fmt.Sprintf("%d_%s", u.ID, sanitizeName(fh.Filename))
		dst, err := os.Create(filepath.Join(s.Cfg.UploadDir, name))
		if err != nil {
			src.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
			return
		}
		_, cpErr := io.Copy(dst, src)
		src.Close()
		dst.Close()
		if cpErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
			return
		}
		_, _ = s.DB.Exec(`INSERT INTO uploads_meta (filename, user_id) VALUES (?, ?)`, name, u.ID)
		results = append(results, gin.H{"name": fh.Filename, "url": "/uploads/" + name})
	}
	c.JSON(http.StatusOK, gin.H{"files": results})
}

// DeleteUpload 删除附件（上传者或管理员）。
func (s *Server) DeleteUpload(c *gin.Context) {
	u, _ := middleware.CurrentUser(c)
	name := filepath.Base(c.Param("filename"))
	if name == "." || name == "/" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var owner int64
	err := s.DB.QueryRow(`SELECT user_id FROM uploads_meta WHERE filename=?`, name).Scan(&owner)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	if owner != u.ID && u.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除"})
		return
	}
	_ = os.Remove(filepath.Join(s.Cfg.UploadDir, name))
	_, _ = s.DB.Exec(`DELETE FROM uploads_meta WHERE filename=?`, name)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "..", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	if len(name) > 100 {
		name = name[len(name)-100:]
	}
	return name
}
