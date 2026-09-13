package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Sitemap 动态生成 /sitemap.xml：首页 + 列表页 + 全部公开详情页（含 lastmod）。
// 带进程内 TTL 缓存，避免爬虫高频抓取压 DB。
func (s *Server) Sitemap(c *gin.Context) {
	key := "sitemap.xml"
	if xmlStr, ok := s.prCacheGet(key); ok {
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(xmlStr))
		return
	}
	xmlStr, err := s.buildSitemap()
	if err != nil {
		c.String(http.StatusInternalServerError, "sitemap error")
		return
	}
	s.prCachePut(key, xmlStr)
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(xmlStr))
}

func (s *Server) buildSitemap() (string, error) {
	base := s.Cfg.SiteURL
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")

	writeURL := func(loc string, lastmod time.Time) {
		b.WriteString("  <url><loc>" + xmlEscape(loc) + "</loc>")
		if !lastmod.IsZero() {
			b.WriteString("<lastmod>" + lastmod.Format("2006-01-02") + "</lastmod>")
		}
		b.WriteString("</url>\n")
	}

	writeURL(base+"/", time.Time{})
	writeURL(base+"/ask", time.Time{})
	writeURL(base+"/knowledge", time.Time{})
	writeURL(base+"/forum", time.Time{})
	writeURL(base+"/jobs", time.Time{})

	type src struct {
		query string
		path  string
	}
	sources := []src{
		{`SELECT id, COALESCE(edited_at, created_at) FROM questions ORDER BY id`, "/ask/"},
		{`SELECT id, COALESCE(edited_at, created_at) FROM articles WHERE published=1 ORDER BY id`, "/knowledge/"},
		{`SELECT id, COALESCE(edited_at, created_at) FROM forum_posts ORDER BY id`, "/forum/"},
	}
	for _, sc := range sources {
		rows, err := s.DB.Query(sc.query)
		if err != nil {
			return "", err
		}
		for rows.Next() {
			var id int64
			var lastmod time.Time
			if rows.Scan(&id, &lastmod) != nil {
				continue
			}
			writeURL(fmt.Sprintf("%s%s%d", base, sc.path, id), lastmod)
		}
		rows.Close()
	}

	b.WriteString("</urlset>\n")
	return b.String(), nil
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;").Replace(s)
}
