package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 爬虫预渲染（SEO）：搜索引擎蜘蛛拿到含正文的静态 HTML，
// 普通访客仍走 dist/index.html 的 SPA（main.go 的 NoRoute 回退）。
// 预渲染页与 SPA 使用同一份数据与同一套标题/描述，只做渲染，不做内容差异。
var crawlerUARe = regexp.MustCompile(`(?i)(baiduspider|googlebot|bingbot|sogou (web|inst) spider|360spider|haosouspider|yandex(bot|images)|bytespider|petalbot|shenma|duckduckbot|applebot|yahoo! slurp)`)

const (
	prerenderTTL      = 5 * time.Minute
	prerenderCacheMax = 500
	prerenderPageSize = 20
	prerenderJobSize  = 50
	siteName          = "棱语 OptiTalk"
)

type prLink struct {
	Text string
	URL  string
	Meta string
}

type prSection struct {
	Heading string
	HTML    template.HTML
	Links   []prLink
}

type prPage struct {
	Title       string
	Description string
	Path        string
	Heading     string
	Intro       string
	Sections    []prSection
	Pager       []prLink
	Type        string // og:type
	JSONLD      any
}

var prBodyTmpl = template.Must(template.New("prbody").Parse(`<main>
<h1>{{.Heading}}</h1>
{{if .Intro}}<p>{{.Intro}}</p>{{end}}
{{range .Sections}}<section>
{{if .Heading}}<h2>{{.Heading}}</h2>{{end}}
{{.HTML}}
{{if .Links}}<ul>
{{range .Links}}<li><a href="{{.URL}}">{{.Text}}</a>{{if .Meta}} — {{.Meta}}{{end}}</li>
{{end}}</ul>{{end}}
</section>
{{end}}{{if .Pager}}<nav>
{{range .Pager}}<a href="{{.URL}}">{{.Text}}</a> {{end}}
</nav>{{end}}
</main>`))

var (
	prTitleRe = regexp.MustCompile(`(?is)<title>.*?</title>`)
	prDescRe  = regexp.MustCompile(`(?is)<meta\s+name="description"[^>]*>`)
	prAppRe   = regexp.MustCompile(`(?is)<div id="app">\s*</div>`)
)

// SetShell 由 main 在启动时注入前端构建产物 index.html 作为预渲染外壳。
func (s *Server) SetShell(distDir string) {
	if distDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(distDir, "index.html"))
	if err != nil {
		log.Printf("预渲染外壳读取失败（将使用简约模板）: %v", err)
		return
	}
	s.prShell = string(data)
}

// Prerender 预渲染入口：挂在 /、/ask、/forum 等显式路由上，非爬虫回退 SPA 外壳。
func (s *Server) Prerender(c *gin.Context) {
	if !crawlerUARe.MatchString(c.Request.UserAgent()) {
		s.serveShell(c)
		return
	}
	key := c.Request.URL.Path + "?" + c.Request.URL.RawQuery
	if html, ok := s.prCacheGet(key); ok {
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
		return
	}
	page, err := s.prerenderFor(c)
	if errors.Is(err, sql.ErrNoRows) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("预渲染失败 %s: %v", key, err)
		s.serveShell(c)
		return
	}
	if page == nil {
		s.serveShell(c)
		return
	}
	html := s.renderPrerendered(page)
	s.prCachePut(key, html)
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (s *Server) serveShell(c *gin.Context) {
	if s.Cfg.FrontendDist != "" {
		c.File(filepath.Join(s.Cfg.FrontendDist, "index.html"))
		return
	}
	c.String(http.StatusOK, "frontend not built")
}

func (s *Server) prerenderFor(c *gin.Context) (*prPage, error) {
	path := c.Request.URL.Path
	switch {
	case path == "/":
		return s.prHome()
	case path == "/ask":
		return s.prQuestionList(intQuery(c, "page"), intQuery(c, "category"))
	case path == "/knowledge":
		return s.prArticleList(intQuery(c, "page"))
	case path == "/forum":
		return s.prForumList(intQuery(c, "page"), intQuery(c, "board_id"))
	case path == "/jobs":
		return s.prJobs(intQuery(c, "page"))
	case strings.HasPrefix(path, "/ask/"):
		id, err := strconv.ParseInt(strings.TrimPrefix(path, "/ask/"), 10, 64)
		if err != nil {
			return nil, nil
		}
		return s.prQuestion(id)
	case strings.HasPrefix(path, "/knowledge/"):
		id, err := strconv.ParseInt(strings.TrimPrefix(path, "/knowledge/"), 10, 64)
		if err != nil {
			return nil, nil
		}
		return s.prArticle(id)
	case strings.HasPrefix(path, "/forum/"):
		id, err := strconv.ParseInt(strings.TrimPrefix(path, "/forum/"), 10, 64)
		if err != nil {
			return nil, nil
		}
		return s.prForumPost(id)
	}
	return nil, nil
}

// ---- 页面渲染 ----

func (s *Server) renderPrerendered(p *prPage) string {
	var body strings.Builder
	_ = prBodyTmpl.Execute(&body, p)

	base := s.Cfg.SiteURL
	canonical := base + p.Path
	esc := template.HTMLEscapeString

	head := `<link rel="canonical" href="` + esc(canonical) + `" />` + "\n" +
		`<meta property="og:site_name" content="` + esc(siteName) + `" />` + "\n" +
		`<meta property="og:type" content="` + esc(ogType(p.Type)) + `" />` + "\n" +
		`<meta property="og:title" content="` + esc(p.Title) + `" />` + "\n" +
		`<meta property="og:description" content="` + esc(p.Description) + `" />` + "\n" +
		`<meta property="og:url" content="` + esc(canonical) + `" />` + "\n"
	if p.JSONLD != nil {
		if b, err := json.Marshal(p.JSONLD); err == nil {
			// json.Marshal 会把 < > & 转成 < 等，脚本上下文内安全
			head += `<script type="application/ld+json">` + string(b) + `</script>` + "\n"
		}
	}

	shell := s.prShell
	if shell == "" {
		shell = `<html lang="zh-CN"><head><meta charset="utf-8" />` +
			`<meta name="viewport" content="width=device-width, initial-scale=1" />` +
			`<title>` + esc(p.Title) + `</title>` +
			`<meta name="description" content="` + esc(p.Description) + `" />` +
			`</head><body><div id="app"></div></body></html>`
	}
	if prTitleRe.MatchString(shell) {
		shell = prTitleRe.ReplaceAllString(shell, "<title>"+esc(p.Title)+"</title>")
	}
	if prDescRe.MatchString(shell) {
		shell = prDescRe.ReplaceAllString(shell, `<meta name="description" content="`+esc(p.Description)+`" />`)
	}
	shell = strings.Replace(shell, "</head>", head+"</head>", 1)
	if prAppRe.MatchString(shell) {
		shell = prAppRe.ReplaceAllString(shell, `<div id="app">`+body.String()+`</div>`)
	}
	return shell
}

func ogType(t string) string {
	if t == "" {
		return "website"
	}
	return t
}

// ---- 各页面 ----

func (s *Server) prHome() (*prPage, error) {
	p := &prPage{
		Title:       siteName + "｜光学问答与知识社区",
		Description: "棱语 OptiTalk 是面向光学学习者与工程师的问答和知识社区，从基础概念到工程实践，沉淀可靠答案与真实经验。",
		Path:        "/",
		Heading:     siteName,
		Intro:       "把光学问题，讲清楚、做出来。面向光学学习者与工程师的问答社区：从基础概念到工程实践，找到可靠答案，也留下你的经验。",
	}
	base := s.Cfg.SiteURL

	if links, err := s.prQueryLinks(`SELECT q.id, q.title, q.created_at, u.username
			FROM questions q JOIN users u ON u.id=q.user_id
			ORDER BY q.created_at DESC LIMIT 5`); err == nil {
		for i := range links {
			links[i].URL = base + "/ask/" + links[i].URL
		}
		p.Sections = append(p.Sections, prSection{
			Heading: "最新投稿",
			Links:   links,
			HTML:    template.HTML(`<p><a href="` + base + `/ask">全部投稿</a></p>`),
		})
	}

	if links, err := s.prQueryLinks(`SELECT a.id, a.title, a.created_at, u.username
			FROM articles a JOIN users u ON u.id=a.user_id WHERE a.published=1
			ORDER BY a.created_at DESC LIMIT 4`); err == nil {
		for i := range links {
			links[i].URL = base + "/knowledge/" + links[i].URL
		}
		p.Sections = append(p.Sections, prSection{
			Heading: "知识库精选",
			Links:   links,
			HTML:    template.HTML(`<p><a href="` + base + `/knowledge">全部文章</a></p>`),
		})
	}

	if links, err := s.prQueryLinks(`SELECT p.id, p.title, p.created_at,
			CASE WHEN p.is_anonymous=1 THEN '匿名' ELSE u.username END
			FROM forum_posts p JOIN users u ON u.id=p.user_id
			ORDER BY p.created_at DESC LIMIT 4`); err == nil {
		for i := range links {
			links[i].URL = base + "/forum/" + links[i].URL
		}
		p.Sections = append(p.Sections, prSection{
			Heading: "论坛新帖",
			Links:   links,
			HTML:    template.HTML(`<p><a href="` + base + `/forum">进入论坛</a></p>`),
		})
	}

	var jobCount int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM job_entries WHERE status='active'`).Scan(&jobCount)
	if jobCount > 0 {
		p.Sections = append(p.Sections, prSection{
			Heading: "就业信息",
			HTML: template.HTML(fmt.Sprintf(
				`<p>2027 届光学公司校招共享数据库，目前收录 %d 家公司，人人可查看、可编辑，含真实评价与版本记录。</p><p><a href="%s/jobs">进入就业信息表</a></p>`,
				jobCount, base)),
		})
	}

	p.JSONLD = []any{
		map[string]any{
			"@context": "https://schema.org",
			"@type":    "WebSite",
			"name":     siteName,
			"url":      base,
			"inLanguage": "zh-CN",
		},
		map[string]any{
			"@context": "https://schema.org",
			"@type":    "Organization",
			"name":     siteName,
			"url":      base,
		},
	}
	return p, nil
}

func (s *Server) prQuestionList(page, categoryID int) (*prPage, error) {
	if page < 1 {
		page = 1
	}
	where, args := "1=1", []any{}
	if categoryID > 0 {
		where = "q.category_id=?"
		args = append(args, categoryID)
	}
	var total int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM questions q WHERE `+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	args = append(args, prerenderPageSize, (page-1)*prerenderPageSize)
	links, err := s.prQueryLinks(`SELECT q.id, q.title, q.created_at, u.username
		FROM questions q JOIN users u ON u.id=q.user_id
		WHERE `+where+` ORDER BY q.created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	base := s.Cfg.SiteURL
	for i := range links {
		links[i].URL = base + "/ask/" + links[i].URL
	}
	p := &prPage{
		Title:       pageTitle("问题投稿", page) + "｜" + siteName,
		Description: "棱语 OptiTalk 问答社区的问题投稿：光学设计、光学考研、Zemax 使用等方向的提问与解答。",
		Path:        "/ask",
		Heading:     "问题投稿",
		Intro:       "提出你在光学学习与工程实践中的问题，社区一起解答。",
		Sections:    []prSection{{Heading: "问题列表", Links: links}},
		Type:        "website",
	}
	p.Pager = listPager(base+"/ask", page, total, prerenderPageSize)
	p.JSONLD = listJSONLD(base+"/ask", "问题投稿", links)
	return p, nil
}

func (s *Server) prQuestion(id int64) (*prPage, error) {
	var (
		title, body, tags, author string
		created                   time.Time
		views                     int
		filesRaw                  string
	)
	err := s.DB.QueryRow(`SELECT q.title, q.body, q.tags, q.views, q.created_at, q.attachments, u.username
		FROM questions q JOIN users u ON u.id=q.user_id WHERE q.id=?`, id).
		Scan(&title, &body, &tags, &views, &created, &filesRaw, &author)
	if err != nil {
		return nil, err
	}
	base := s.Cfg.SiteURL
	url := fmt.Sprintf("%s/ask/%d", base, id)
	p := &prPage{
		Title:       title + "｜" + siteName,
		Description: plainSummary(body, 120),
		Path:        fmt.Sprintf("/ask/%d", id),
		Heading:     title,
		Intro:       fmt.Sprintf("%s 提问于 %s · 浏览 %d · 回答见下方", author, created.Format("2006-01-02"), views),
		Sections: []prSection{
			{HTML: renderMarkdown(body)},
			{Heading: "附件", Links: attachmentLinks(base, filesRaw)},
		},
		Type: "article",
	}

	rows, err := s.DB.Query(`SELECT a.body, a.created_at, a.attachments, u.username
		FROM answers a JOIN users u ON u.id=a.user_id
		WHERE a.question_id=? ORDER BY a.created_at ASC LIMIT 20`, id)
	if err == nil {
		defer rows.Close()
		answerBodies := []string{}
		for rows.Next() {
			var abody, aauthor, afiles string
			var acreated time.Time
			if rows.Scan(&abody, &acreated, &afiles, &aauthor) != nil {
				continue
			}
			answerBodies = append(answerBodies, string(renderMarkdown(abody)))
		}
		if len(answerBodies) > 0 {
			var html strings.Builder
			for i, b := range answerBodies {
				fmt.Fprintf(&html, "<h3>回答 %d</h3>%s", i+1, b)
			}
			p.Sections = append(p.Sections, prSection{Heading: "回答", HTML: template.HTML(html.String())})
		}
	}

	answersForLD := make([]any, 0, 3)
	for _, b := range sectionTexts(p) {
		answersForLD = append(answersForLD, map[string]any{
			"@type": "Answer",
			"text":  b,
		})
	}
	p.JSONLD = map[string]any{
		"@context": "https://schema.org",
		"@type":    "QAPage",
		"mainEntity": map[string]any{
			"@type":         "Question",
			"name":          title,
			"text":          plainSummary(body, 500),
			"url":           url,
			"dateCreated":   created.Format("2006-01-02"),
			"answerCount":   len(answersForLD),
			"suggestedAnswer": answersForLD,
		},
	}
	return p, nil
}

func (s *Server) prArticleList(page int) (*prPage, error) {
	if page < 1 {
		page = 1
	}
	var total int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM articles WHERE published=1`).Scan(&total); err != nil {
		return nil, err
	}
	links, err := s.prQueryLinks(`SELECT a.id, a.title, a.created_at, u.username
		FROM articles a JOIN users u ON u.id=a.user_id WHERE a.published=1
		ORDER BY a.created_at DESC LIMIT ? OFFSET ?`, prerenderPageSize, (page-1)*prerenderPageSize)
	if err != nil {
		return nil, err
	}
	base := s.Cfg.SiteURL
	for i := range links {
		links[i].URL = base + "/knowledge/" + links[i].URL
	}
	p := &prPage{
		Title:       pageTitle("知识库", page) + "｜" + siteName,
		Description: "棱语 OptiTalk 知识库：光学基础概念与工程实践文章，按需检索、随手分享。",
		Path:        "/knowledge",
		Heading:     "知识库",
		Intro:       "沉淀可靠的光学知识文章：从基础概念到工程经验，按需检索。",
		Sections:    []prSection{{Heading: "文章列表", Links: links}},
	}
	p.Pager = listPager(base+"/knowledge", page, total, prerenderPageSize)
	p.JSONLD = listJSONLD(base+"/knowledge", "知识库", links)
	return p, nil
}

func (s *Server) prArticle(id int64) (*prPage, error) {
	var (
		title, body, summary, author string
		created, edited              sql.NullTime
		views                        int
	)
	err := s.DB.QueryRow(`SELECT a.title, a.body, a.summary, a.views, a.created_at, a.edited_at, u.username
		FROM articles a JOIN users u ON u.id=a.user_id WHERE a.id=? AND a.published=1`, id).
		Scan(&title, &body, &summary, &views, &created, &edited, &author)
	if err != nil {
		return nil, err
	}
	base := s.Cfg.SiteURL
	if summary == "" {
		summary = plainSummary(body, 120)
	}
	p := &prPage{
		Title:       title + "｜" + siteName,
		Description: summary,
		Path:        fmt.Sprintf("/knowledge/%d", id),
		Heading:     title,
		Intro:       fmt.Sprintf("%s 发布于 %s · 浏览 %d", author, created.Time.Format("2006-01-02"), views),
		Sections:    []prSection{{HTML: renderMarkdown(body)}},
		Type:        "article",
	}
	p.JSONLD = map[string]any{
		"@context":      "https://schema.org",
		"@type":         "Article",
		"headline":      title,
		"description":   summary,
		"url":           fmt.Sprintf("%s/knowledge/%d", base, id),
		"datePublished": created.Time.Format("2006-01-02"),
		"author":        map[string]any{"@type": "Person", "name": author},
	}
	return p, nil
}

func (s *Server) prForumList(page, boardID int) (*prPage, error) {
	if page < 1 {
		page = 1
	}
	where, args := "1=1", []any{}
	if boardID > 0 {
		where = "p.board_id=?"
		args = append(args, boardID)
	}
	var total int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM forum_posts p WHERE `+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	args = append(args, prerenderPageSize, (page-1)*prerenderPageSize)
	links, err := s.prQueryLinks(`SELECT p.id, p.title, p.created_at,
			CASE WHEN p.is_anonymous=1 THEN '匿名' ELSE u.username END
		FROM forum_posts p JOIN users u ON u.id=p.user_id
		WHERE `+where+` ORDER BY p.is_pinned DESC, p.created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	base := s.Cfg.SiteURL
	for i := range links {
		links[i].URL = base + "/forum/" + links[i].URL
	}
	p := &prPage{
		Title:       pageTitle("论坛交流", page) + "｜" + siteName,
		Description: "棱语 OptiTalk 论坛：光学行业动态、学术讨论、学习资源与经验交流。",
		Path:        "/forum",
		Heading:     "论坛交流",
		Intro:       "自由讨论光学相关话题：行业动态、学术前沿、学习资源与经验交流。",
		Sections:    []prSection{{Heading: "帖子列表", Links: links}},
	}
	p.Pager = listPager(base+"/forum", page, total, prerenderPageSize)
	p.JSONLD = listJSONLD(base+"/forum", "论坛交流", links)
	return p, nil
}

func (s *Server) prForumPost(id int64) (*prPage, error) {
	var (
		title, body, tags, author string
		created                   time.Time
		views                     int
		filesRaw                  string
	)
	err := s.DB.QueryRow(`SELECT p.title, p.body, p.tags, p.views, p.created_at, p.attachments,
			CASE WHEN p.is_anonymous=1 THEN '匿名' ELSE u.username END
		FROM forum_posts p JOIN users u ON u.id=p.user_id WHERE p.id=?`, id).
		Scan(&title, &body, &tags, &views, &created, &filesRaw, &author)
	if err != nil {
		return nil, err
	}
	base := s.Cfg.SiteURL
	p := &prPage{
		Title:       title + "｜" + siteName,
		Description: plainSummary(body, 120),
		Path:        fmt.Sprintf("/forum/%d", id),
		Heading:     title,
		Intro:       fmt.Sprintf("%s 发表于 %s · 浏览 %d", author, created.Format("2006-01-02"), views),
		Sections: []prSection{
			{HTML: renderMarkdown(body)},
			{Heading: "附件", Links: attachmentLinks(base, filesRaw)},
		},
		Type: "article",
	}

	rows, err := s.DB.Query(`SELECT r.body, r.created_at, r.attachments, u.username
		FROM forum_replies r JOIN users u ON u.id=r.user_id
		WHERE r.post_id=? ORDER BY r.created_at ASC LIMIT 30`, id)
	if err == nil {
		defer rows.Close()
		var html strings.Builder
		n := 0
		for rows.Next() {
			var rbody, rauthor, rfiles string
			var rcreated time.Time
			if rows.Scan(&rbody, &rcreated, &rfiles, &rauthor) != nil {
				continue
			}
			n++
			fmt.Fprintf(&html, "<h3>回复 %d · %s</h3>%s", n, template.HTMLEscapeString(rauthor), renderMarkdown(rbody))
		}
		if n > 0 {
			p.Sections = append(p.Sections, prSection{Heading: "回复", HTML: template.HTML(html.String())})
		}
	}
	p.JSONLD = map[string]any{
		"@context":         "https://schema.org",
		"@type":            "DiscussionForumPosting",
		"headline":         title,
		"text":             plainSummary(body, 500),
		"url":              fmt.Sprintf("%s/forum/%d", base, id),
		"datePublished":    created.Format("2006-01-02"),
		"author":           map[string]any{"@type": "Person", "name": author},
		"interactionStatistic": map[string]any{
			"@type":                "InteractionCounter",
			"interactionType":      "https://schema.org/ViewAction",
			"userInteractionCount": views,
		},
	}
	return p, nil
}

func (s *Server) prJobs(page int) (*prPage, error) {
	if page < 1 {
		page = 1
	}
	var total int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM job_entries WHERE status='active'`).Scan(&total); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`SELECT company, industry, city, apply_link, campus_status, updated_at
		FROM job_entries WHERE status='active'
		ORDER BY is_pinned DESC, pin_order ASC, updated_at DESC, id DESC LIMIT ? OFFSET ?`,
		prerenderJobSize, (page-1)*prerenderJobSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var b strings.Builder
	b.WriteString("<ul>\n")
	for rows.Next() {
		var company, industry, city, apply, campus string
		var updated time.Time
		if rows.Scan(&company, &industry, &city, &apply, &campus, &updated) != nil {
			continue
		}
		fmt.Fprintf(&b, "<li><b>%s</b>", template.HTMLEscapeString(company))
		if industry != "" {
			fmt.Fprintf(&b, " — %s", template.HTMLEscapeString(industry))
		}
		if city != "" {
			fmt.Fprintf(&b, " — %s", template.HTMLEscapeString(city))
		}
		fmt.Fprintf(&b, " — 校招状态：%s", template.HTMLEscapeString(campus))
		for l := range strings.SplitSeq(apply, "\n") {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			if u, ok := safeExternalURL(l); ok {
				fmt.Fprintf(&b, ` — <a href="%s" rel="nofollow noopener">投递链接</a>`, template.HTMLEscapeString(u))
			}
		}
		b.WriteString("</li>\n")
	}
	b.WriteString("</ul>\n")

	base := s.Cfg.SiteURL
	p := &prPage{
		Title:       pageTitle("就业信息", page) + "｜2027 届光学公司校招共享数据库｜" + siteName,
		Description: fmt.Sprintf("棱语 OptiTalk 就业信息：2027 届光学公司校招共享数据库，目前收录 %d 家公司，含方向、地点、内推码与投递链接，人人可查看、可编辑。", total),
		Path:        "/jobs",
		Heading:     "就业信息（2027 届光学公司校招共享数据库）",
		Intro:       fmt.Sprintf("社区协作数据库，目前收录 %d 家公司：人人可查看，登录后即可新增、编辑、评价；每次修改都会记录版本、修改人与修改原因。", total),
		Sections:    []prSection{{Heading: "公司列表", HTML: template.HTML(b.String())}},
	}
	p.Pager = listPager(base+"/jobs", page, total, prerenderJobSize)
	p.JSONLD = map[string]any{
		"@context":    "https://schema.org",
		"@type":       "CollectionPage",
		"name":        "2027 届光学公司校招共享数据库",
		"url":         base + "/jobs",
		"description": p.Description,
	}
	return p, nil
}

// ---- 辅助 ----

func intQuery(c *gin.Context, name string) int {
	n, _ := strconv.Atoi(c.Query(name))
	return n
}

func pageTitle(name string, page int) string {
	if page > 1 {
		return fmt.Sprintf("%s（第 %d 页）", name, page)
	}
	return name
}

func listPager(baseURL string, page, total, size int) []prLink {
	pages := (total + size - 1) / size
	if pages <= 1 {
		return nil
	}
	links := make([]prLink, 0, pages+2)
	for i := 1; i <= pages && i <= 10; i++ {
		if i == page {
			continue
		}
		links = append(links, prLink{Text: fmt.Sprintf("第 %d 页", i), URL: fmt.Sprintf("%s?page=%d", baseURL, i)})
	}
	if page < pages {
		links = append(links, prLink{Text: "下一页", URL: fmt.Sprintf("%s?page=%d", baseURL, page+1)})
	}
	return links
}

func listJSONLD(url, name string, links []prLink) any {
	items := make([]any, 0, len(links))
	for i, l := range links {
		items = append(items, map[string]any{
			"@type":    "ListItem",
			"position": i + 1,
			"url":      l.URL,
			"name":     l.Text,
		})
	}
	return map[string]any{
		"@context":        "https://schema.org",
		"@type":           "CollectionPage",
		"name":            name,
		"url":             url,
		"mainEntity":      map[string]any{"@type": "ItemList", "itemListElement": items},
	}
}

func (s *Server) prQueryLinks(query string, args ...any) ([]prLink, error) {
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := make([]prLink, 0)
	for rows.Next() {
		var id int64
		var title, author string
		var created time.Time
		if err := rows.Scan(&id, &title, &created, &author); err != nil {
			return nil, err
		}
		links = append(links, prLink{
			Text: title,
			URL:  strconv.FormatInt(id, 10),
			Meta: fmt.Sprintf("%s · %s", author, created.Format("2006-01-02")),
		})
	}
	return links, nil
}

func attachmentLinks(base, filesRaw string) []prLink {
	files := attachmentsJSON(filesRaw)
	links := make([]prLink, 0, len(files))
	for _, f := range files {
		links = append(links, prLink{Text: f.Name, URL: base + f.URL})
	}
	return links
}

func sectionTexts(p *prPage) []string {
	out := []string{}
	for _, sec := range p.Sections {
		if sec.Heading != "回答" {
			continue
		}
		out = append(out, strings.TrimSpace(stripTags(string(sec.HTML))))
	}
	return out
}

var tagRe = regexp.MustCompile(`(?s)<[^>]*>`)

func stripTags(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(tagRe.ReplaceAllString(s, " ")), " "))
}

// plainSummary 生成 meta description：去掉常见 Markdown 记号后截断。
func plainSummary(src string, n int) string {
	s := strings.NewReplacer("#", "", "*", "", "`", "", ">", "", "$", "").Replace(src)
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > n {
		s = string([]rune(s)[:n]) + "…"
	}
	if s == "" {
		s = siteName
	}
	return s
}

var boldRe = regexp.MustCompile(`\*\*([^*]+)\*\*`)

// renderMarkdown 把 Markdown 粗略转为安全的 HTML（整体转义后再识别标题/列表/加粗），
// 用于预渲染页正文；公式等复杂语法保持原文，文字内容与 SPA 一致。
func renderMarkdown(src string) template.HTML {
	lines := strings.Split(template.HTMLEscapeString(src), "\n")
	var b strings.Builder
	inList := false
	var para []string
	closeList := func() {
		if inList {
			b.WriteString("</ul>\n")
			inList = false
		}
	}
	flushPara := func() {
		if len(para) > 0 {
			b.WriteString("<p>" + strings.Join(para, "<br>") + "</p>\n")
			para = nil
		}
	}
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		switch {
		case t == "":
			closeList()
			flushPara()
		case strings.HasPrefix(t, "#"):
			closeList()
			flushPara()
			if text := strings.TrimSpace(strings.TrimLeft(t, "#")); text != "" {
				b.WriteString("<h3>" + boldText(text) + "</h3>\n")
			}
		case strings.HasPrefix(t, "- "), strings.HasPrefix(t, "* "):
			flushPara()
			if !inList {
				b.WriteString("<ul>\n")
				inList = true
			}
			b.WriteString("<li>" + boldText(t[2:]) + "</li>\n")
		default:
			closeList()
			para = append(para, boldText(t))
		}
	}
	closeList()
	flushPara()
	return template.HTML(b.String())
}

func boldText(s string) string {
	return boldRe.ReplaceAllString(s, "<strong>$1</strong>")
}

// safeExternalURL 只放行 http/https 的外链（投递链接来自用户输入）。
func safeExternalURL(raw string) (string, bool) {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		if strings.ContainsAny(raw, " \"'<>") {
			return "", false
		}
		return raw, true
	}
	return "", false
}
