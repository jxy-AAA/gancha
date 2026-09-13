package tests

// 集成测试：需要本地 MySQL（docker compose up -d 启动）。
// 运行方式：cd backend && go test ./tests/ -v -count=1
// 会使用独立数据库 guangyanji_test（自动创建），结束后删除该库。

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	serverPort = "18080"
	baseURL    = "http://127.0.0.1:" + serverPort
)

// exeSuffix：Windows 下编译产物需 .exe 后缀。
var exeSuffix = func() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}()

var (
	serverCmd *exec.Cmd
	client    = &http.Client{Timeout: 10 * time.Second}
)

func TestMain(m *testing.M) {
	// 1. 准备测试数据库
	adminDSN := "guangyanji:guangyanji_dev@tcp(127.0.0.1:3306)/"
	adminDB, err := sql.Open("mysql", adminDSN)
	if err != nil {
		log.Fatalf("连接 MySQL 失败，请先 docker compose up -d: %v", err)
	}
	mustExec(adminDB, "CREATE DATABASE IF NOT EXISTS guangyanji_test CHARACTER SET utf8mb4")
	adminDB.Close()

	// 2. 启动测试服务器（先编译成二进制再运行，避免 go run 残留子进程）
	wd, _ := os.Getwd()
	backendDir := filepath.Dir(wd)
	binPath := filepath.Join(os.TempDir(), "guangyanji-server-test"+exeSuffix)
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	buildCmd.Dir = backendDir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		log.Fatalf("编译测试服务器失败: %v\n%s", err, out)
	}
	serverCmd = exec.Command(binPath)
	serverCmd.Dir = backendDir
	serverCmd.Env = append(os.Environ(),
		"PORT="+serverPort,
		"DB_NAME=guangyanji_test",
		"JWT_SECRET=test-secret-for-integration",
		"UPLOAD_DIR="+filepath.Join(os.TempDir(), "guangyanji-test-uploads"),
		// 预渲染用前端构建产物做外壳；未构建时回退简约模板
		"FRONTEND_DIST="+filepath.Join(backendDir, "..", "frontend", "dist"),
		"GIN_MODE=test",
	)
	if err := serverCmd.Start(); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
	waitHealthy()

	code := m.Run()

	serverCmd.Process.Kill()
	serverCmd.Wait()
	os.Remove(binPath)
	// 3. 清理测试库
	adminDB, _ = sql.Open("mysql", adminDSN)
	if adminDB != nil {
		mustExec(adminDB, "DROP DATABASE IF EXISTS guangyanji_test")
		adminDB.Close()
	}
	os.Exit(code)
}

func waitHealthy() {
	for i := 0; i < 60; i++ {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Fatal("服务器 30 秒内未就绪")
}

func mustExec(db *sql.DB, q string) {
	if _, err := db.Exec(q); err != nil {
		log.Fatalf("SQL 失败: %v\n%s", err, q)
	}
}

func request(method, path string, body any, token string) (*http.Response, []byte) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, baseURL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("请求失败 %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

func tokenOf(t *testing.T, resp *http.Response, data []byte) string {
	t.Helper()
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &out); err != nil || out.Token == "" {
		t.Fatalf("响应无 token: %d %s", resp.StatusCode, data)
	}
	return out.Token
}

func idOf(t *testing.T, data []byte) int64 {
	t.Helper()
	var out struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("响应无 id: %s", data)
	}
	return out.ID
}

func registerUser(t *testing.T, username, email string) string {
	t.Helper()
	resp, data := request("POST", "/api/auth/register", map[string]string{
		"username": username, "email": email, "password": "password123",
	}, "")
	if resp.StatusCode != 200 {
		t.Fatalf("注册失败: %d %s", resp.StatusCode, data)
	}
	return tokenOf(t, resp, data)
}

func TestFullFlow(t *testing.T) {
	// 认证
	tokenA := registerUser(t, "光学小明", "ming@example.com")
	tokenB := registerUser(t, "光学小红", "hong@example.com")

	resp, data := request("GET", "/api/auth/me", nil, tokenA)
	if resp.StatusCode != 200 || !strings.Contains(string(data), "光学小明") {
		t.Fatalf("me 接口失败: %d %s", resp.StatusCode, data)
	}

	// 登录
	resp, data = request("POST", "/api/auth/login", map[string]string{
		"email": "ming@example.com", "password": "password123",
	}, "")
	if resp.StatusCode != 200 {
		t.Fatalf("登录失败: %d %s", resp.StatusCode, data)
	}

	// 公开分类
	resp, data = request("GET", "/api/categories", nil, "")
	if resp.StatusCode != 200 {
		t.Fatalf("分类失败: %d %s", resp.StatusCode, data)
	}
	var cats struct {
		Items []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	json.Unmarshal(data, &cats)
	if len(cats.Items) != 6 {
		t.Fatalf("应有 6 个分类，实际 %d", len(cats.Items))
	}

	// 发布问题
	resp, data = request("POST", "/api/questions", map[string]any{
		"category_id": cats.Items[0].ID,
		"title":       "如何计算 MTF 截止频率？",
		"body":        "设 $f=1/\\Phi$，请问衍射极限下的截止频率怎么算？",
		"tags":        "MTF, 像差",
	}, tokenA)
	if resp.StatusCode != 200 {
		t.Fatalf("发布问题失败: %d %s", resp.StatusCode, data)
	}
	qid := idOf(t, data)

	// 问题详情
	resp, data = request("GET", fmt.Sprintf("/api/questions/%d", qid), nil, "")
	if resp.StatusCode != 200 || !strings.Contains(string(data), "MTF") {
		t.Fatalf("问题详情失败: %d %s", resp.StatusCode, data)
	}

	// 浏览计数
	request("POST", fmt.Sprintf("/api/questions/%d/view", qid), nil, "")

	// 列表
	resp, data = request("GET", "/api/questions?page=1&page_size=10", nil, "")
	if resp.StatusCode != 200 {
		t.Fatalf("列表失败: %d %s", resp.StatusCode, data)
	}

	// 回答
	resp, data = request("POST", fmt.Sprintf("/api/questions/%d/answers", qid),
		map[string]string{"body": "衍射极限 MTF 截止频率为 $\\nu_c = 1/(\\lambda F^\\#)$"}, tokenB)
	if resp.StatusCode != 200 {
		t.Fatalf("回答失败: %d %s", resp.StatusCode, data)
	}
	aid := idOf(t, data)

	// 投票（问题与回答）
	resp, data = request("POST", "/api/votes", map[string]any{"target_type": "question", "target_id": qid}, tokenB)
	if resp.StatusCode != 200 {
		t.Fatalf("投票失败: %d %s", resp.StatusCode, data)
	}
	request("POST", "/api/votes", map[string]any{"target_type": "answer", "target_id": aid}, tokenA)

	// 采纳
	resp, data = request("POST", fmt.Sprintf("/api/answers/%d/accept", aid), nil, tokenA)
	if resp.StatusCode != 200 {
		t.Fatalf("采纳失败: %d %s", resp.StatusCode, data)
	}

	// 评论
	resp, data = request("POST", fmt.Sprintf("/api/questions/%d/comments", qid),
		map[string]string{"body": "好问题！"}, tokenB)
	if resp.StatusCode != 200 {
		t.Fatalf("评论失败: %d %s", resp.StatusCode, data)
	}

	// 书签与关注
	request("POST", fmt.Sprintf("/api/questions/%d/bookmark", qid), nil, tokenA)
	request("POST", fmt.Sprintf("/api/questions/%d/follow", qid), nil, tokenB)

	// 知识库文章
	resp, data = request("POST", "/api/articles", map[string]any{
		"title": "MTF 入门指南", "summary": "一文读懂调制传递函数", "body": "MTF 是……", "tags": "MTF",
		"published": true,
	}, tokenA)
	if resp.StatusCode != 200 {
		t.Fatalf("发布文章失败: %d %s", resp.StatusCode, data)
	}
	artID := idOf(t, data)
	resp, data = request("GET", "/api/articles", nil, "")
	if resp.StatusCode != 200 {
		t.Fatalf("文章列表失败: %d %s", resp.StatusCode, data)
	}

	// 论坛发帖与回复
	resp, data = request("GET", "/api/boards", nil, "")
	if resp.StatusCode != 200 {
		t.Fatalf("板块失败: %d %s", resp.StatusCode, data)
	}
	var boards struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	json.Unmarshal(data, &boards)
	resp, data = request("POST", "/api/forum/posts", map[string]any{
		"board_id": boards.Items[0].ID, "title": "Zemax 安装问题", "body": "求解",
	}, tokenB)
	if resp.StatusCode != 200 {
		t.Fatalf("发帖失败: %d %s", resp.StatusCode, data)
	}
	fpid := idOf(t, data)
	resp, data = request("POST", fmt.Sprintf("/api/forum/posts/%d/replies", fpid),
		map[string]string{"body": "参考官方文档"}, tokenA)
	if resp.StatusCode != 200 {
		t.Fatalf("回复失败: %d %s", resp.StatusCode, data)
	}

	// 通知（回答者应收到通知）
	resp, data = request("GET", "/api/notifications", nil, tokenA)
	if resp.StatusCode != 200 {
		t.Fatalf("通知失败: %d %s", resp.StatusCode, data)
	}

	// 上传（multipart）
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("files", "notes.txt")
	fw.Write([]byte("test file content"))
	mw.Close()
	upReq, _ := http.NewRequest("POST", baseURL+"/api/uploads", &buf)
	upReq.Header.Set("Authorization", "Bearer "+tokenA)
	upReq.Header.Set("Content-Type", mw.FormDataContentType())
	upResp, err := client.Do(upReq)
	if err != nil || upResp.StatusCode != 200 {
		t.Fatalf("上传失败: %v", err)
	}
	upData, _ := io.ReadAll(upResp.Body)
	upResp.Body.Close()
	var upOut struct {
		Files []struct {
			URL string `json:"url"`
		} `json:"files"`
	}
	json.Unmarshal(upData, &upOut)
	if len(upOut.Files) != 1 {
		t.Fatalf("应上传 1 个文件: %s", upData)
	}
	// 非法扩展名应被拒绝
	var badBuf bytes.Buffer
	mw2 := multipart.NewWriter(&badBuf)
	fw2, _ := mw2.CreateFormFile("files", "evil.exe")
	fw2.Write([]byte("x"))
	mw2.Close()
	badReq, _ := http.NewRequest("POST", baseURL+"/api/uploads", &badBuf)
	badReq.Header.Set("Authorization", "Bearer "+tokenA)
	badReq.Header.Set("Content-Type", mw2.FormDataContentType())
	badResp, _ := client.Do(badReq)
	badResp.Body.Close()
	if badResp.StatusCode != 400 {
		t.Fatalf("exe 上传应被拒绝，实际 %d", badResp.StatusCode)
	}

	// 权限：未登录发布应 401
	resp, data = request("POST", "/api/questions", map[string]any{
		"category_id": 1, "title": "x", "body": "y",
	}, "")
	if resp.StatusCode != 401 {
		t.Fatalf("未登录应 401，实际 %d", resp.StatusCode)
	}

	// 管理员提升与统计
	// 注册一个新用户并提升为 admin（直接连库操作更简单）
	testDSN := "guangyanji:guangyanji_dev@tcp(127.0.0.1:3306)/guangyanji_test"
	adminDB, _ := sql.Open("mysql", testDSN)
	defer adminDB.Close()
	mustExec(adminDB, "UPDATE users SET role='admin' WHERE email='ming@example.com'")
	resp, data = request("GET", "/api/admin/stats", nil, tokenA)
	if resp.StatusCode != 200 {
		t.Fatalf("admin stats 失败: %d %s", resp.StatusCode, data)
	}

	// 删除问题验证
	resp, data = request("DELETE", fmt.Sprintf("/api/questions/%d", qid), nil, tokenA)
	if resp.StatusCode != 200 {
		t.Fatalf("删除问题失败: %d %s", resp.StatusCode, data)
	}
	resp, data = request("GET", fmt.Sprintf("/api/questions/%d", qid), nil, "")
	if resp.StatusCode != 404 {
		t.Fatalf("删除后应 404，实际 %d", resp.StatusCode)
	}

	// 清理文章
	request("DELETE", fmt.Sprintf("/api/articles/%d", artID), nil, tokenA)
	// 清理帖子
	request("DELETE", fmt.Sprintf("/api/forum/posts/%d", fpid), nil, tokenB)

	t.Log("完整流程测试通过")
}

type jobStatsOut struct {
	Status struct {
		Active    int `json:"active"`
		Invalid   int `json:"invalid"`
		Duplicate int `json:"duplicate"`
		All       int `json:"all"`
	} `json:"status"`
	Campus struct {
		All  int `json:"all"`
		Open int `json:"open"`
	} `json:"campus"`
	Applied *struct {
		All     int `json:"all"`
		Applied int `json:"applied"`
		Not     int `json:"not"`
	} `json:"applied"`
}

type jobPageOut struct {
	Items []struct {
		ID           int64  `json:"id"`
		Company      string `json:"company"`
		Status       string `json:"status"`
		CampusStatus string `json:"campus_status"`
		MyApplied    bool   `json:"my_applied"`
	} `json:"items"`
	Total     int         `json:"total"`
	Page      int         `json:"page"`
	PageSize  int         `json:"page_size"`
	Stats     jobStatsOut `json:"stats"`
	PinnedIDs []int64     `json:"pinned_ids"`
}

func requestUA(method, path, ua, token string) (*http.Response, []byte) {
	req, _ := http.NewRequest(method, baseURL+path, nil)
	req.Header.Set("User-Agent", ua)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("请求失败 %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

// TestJobsPagination 校验就业接口分页、分面计数（各维度剔除自身条件）与置顶顺序。
func TestJobsPagination(t *testing.T) {
	tokenA := registerUser(t, "就业测试员", "jobs@example.com")

	var p1, p2 jobPageOut
	_, data := request("GET", "/api/jobs?page=1&page_size=50", nil, "")
	if err := json.Unmarshal(data, &p1); err != nil {
		t.Fatalf("解析第一页失败: %v %s", err, data)
	}
	if len(p1.Items) != 50 || p1.Total < 1000 {
		t.Fatalf("分页异常：items=%d total=%d", len(p1.Items), p1.Total)
	}
	if p1.PageSize != 50 || p1.Page != 1 {
		t.Fatalf("分页回显异常：page=%d size=%d", p1.Page, p1.PageSize)
	}
	for _, it := range p1.Items {
		if it.Status != "active" {
			t.Fatalf("默认应只返回 active，出现 %s（%s）", it.Status, it.Company)
		}
	}

	_, data = request("GET", "/api/jobs?page=2&page_size=50", nil, "")
	json.Unmarshal(data, &p2)
	if len(p2.Items) != 50 {
		t.Fatalf("第二页应有 50 条，实际 %d", len(p2.Items))
	}
	seen := map[int64]bool{}
	for _, it := range p1.Items {
		seen[it.ID] = true
	}
	for _, it := range p2.Items {
		if seen[it.ID] {
			t.Fatalf("第二页与第一页重复：id=%d", it.ID)
		}
	}

	// 分面计数：status 维度剔除自身 status 条件 → 各状态之和应等于 total（仅 status 默认过滤）
	_, data = request("GET", "/api/jobs?status=all&page=1&page_size=1", nil, "")
	var pAll jobPageOut
	json.Unmarshal(data, &pAll)
	sum := pAll.Stats.Status.Active + pAll.Stats.Status.Invalid + pAll.Stats.Status.Duplicate
	if sum != pAll.Stats.Status.All || pAll.Stats.Status.All != pAll.Total {
		t.Fatalf("status 分面计数不一致：sum=%d all=%d total=%d", sum, pAll.Stats.Status.All, pAll.Total)
	}
	// campus 维度剔除自身条件 → all 应等于 status=all 的总数
	if pAll.Stats.Campus.All != pAll.Total {
		t.Fatalf("campus 分面 all=%d 应等于 total=%d", pAll.Stats.Campus.All, pAll.Total)
	}
	// 未登录时 applied 计数必须为 null（EXISTS 恒 false，不能照发）
	if pAll.Stats.Applied != nil {
		t.Fatalf("未登录 applied 计数应为 null，实际 %+v", pAll.Stats.Applied)
	}
	// 置顶行顺序：种子数据有内推码的公司应全部置顶
	if len(pAll.PinnedIDs) < 1 {
		t.Fatalf("pinned_ids 不应为空")
	}

	// 登录后 applied 计数出现，且 applied+not=all
	_, data = request("GET", "/api/jobs?status=all&page=1&page_size=1", nil, tokenA)
	var pMine jobPageOut
	json.Unmarshal(data, &pMine)
	if pMine.Stats.Applied == nil {
		t.Fatalf("登录后 applied 计数不应为 null")
	}
	if pMine.Stats.Applied.Applied+pMine.Stats.Applied.Not != pMine.Stats.Applied.All {
		t.Fatalf("applied 计数不闭合：%+v", pMine.Stats.Applied)
	}

	// 校招状态筛选：已开启数量与分面一致
	_, data = request("GET", "/api/jobs?status=all&campus_status=%E5%B7%B2%E5%BC%80%E5%90%AF&page=1&page_size=1", nil, "")
	var pCampus jobPageOut
	json.Unmarshal(data, &pCampus)
	if pCampus.Total != pMine.Stats.Campus.Open {
		t.Fatalf("campus 筛选 total=%d 与分面 open=%d 不一致", pCampus.Total, pMine.Stats.Campus.Open)
	}

	// 城市下拉
	resp, data := request("GET", "/api/jobs/cities", nil, "")
	if resp.StatusCode != 200 {
		t.Fatalf("城市列表失败: %d %s", resp.StatusCode, data)
	}
	var cities struct {
		Items []string `json:"items"`
	}
	json.Unmarshal(data, &cities)
	if len(cities.Items) < 5 {
		t.Fatalf("城市列表异常：%v", cities.Items)
	}

	t.Logf("就业分页通过：total=%d 置顶=%d 城市=%d", pAll.Total, len(pAll.PinnedIDs), len(cities.Items))
}

// TestAttachments 校验问答/论坛的附件落库、返回与非法 URL 拦截。
func TestAttachments(t *testing.T) {
	tokenA := registerUser(t, "附件测试员", "attach@example.com")

	upload := func(name string) string {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("files", name)
		fw.Write([]byte("hello attachment"))
		mw.Close()
		req, _ := http.NewRequest("POST", baseURL+"/api/uploads", &buf)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("上传失败: %v", err)
		}
		defer resp.Body.Close()
		d, _ := io.ReadAll(resp.Body)
		var out struct {
			Files []struct {
				URL string `json:"url"`
			} `json:"files"`
		}
		json.Unmarshal(d, &out)
		if len(out.Files) != 1 {
			t.Fatalf("上传响应异常: %s", d)
		}
		return out.Files[0].URL
	}

	fileURL := upload("notes.txt")
	attach := []map[string]string{{"name": "notes.txt", "url": fileURL}}

	// 问题 + 附件
	resp, data := request("GET", "/api/categories", nil, "")
	var cats struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	json.Unmarshal(data, &cats)
	resp, data = request("POST", "/api/questions", map[string]any{
		"category_id": cats.Items[0].ID, "title": "附件测试问题", "body": "带附件的正文",
		"attachments": attach,
	}, tokenA)
	if resp.StatusCode != 200 {
		t.Fatalf("带附件发问失败: %d %s", resp.StatusCode, data)
	}
	qid := idOf(t, data)
	resp, data = request("GET", fmt.Sprintf("/api/questions/%d", qid), nil, "")
	if resp.StatusCode != 200 || !strings.Contains(string(data), fileURL) {
		t.Fatalf("问题详情未返回附件: %d %s", resp.StatusCode, data)
	}

	// 回答 + 附件
	resp, data = request("POST", fmt.Sprintf("/api/questions/%d/answers", qid), map[string]any{
		"body": "带附件的回答", "attachments": attach,
	}, tokenA)
	if resp.StatusCode != 200 {
		t.Fatalf("带附件回答失败: %d %s", resp.StatusCode, data)
	}
	resp, data = request("GET", fmt.Sprintf("/api/questions/%d/answers", qid), nil, "")
	if resp.StatusCode != 200 || !strings.Contains(string(data), fileURL) {
		t.Fatalf("回答列表未返回附件: %d %s", resp.StatusCode, data)
	}

	// 论坛帖子 + 附件 / 回复 + 附件
	resp, data = request("GET", "/api/boards", nil, "")
	var boards struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	json.Unmarshal(data, &boards)
	resp, data = request("POST", "/api/forum/posts", map[string]any{
		"board_id": boards.Items[0].ID, "title": "附件测试帖", "body": "帖子正文",
		"attachments": attach,
	}, tokenA)
	if resp.StatusCode != 200 {
		t.Fatalf("带附件发帖失败: %d %s", resp.StatusCode, data)
	}
	fpid := idOf(t, data)
	resp, data = request("POST", fmt.Sprintf("/api/forum/posts/%d/replies", fpid), map[string]any{
		"body": "带附件的回复", "attachments": attach,
	}, tokenA)
	if resp.StatusCode != 200 {
		t.Fatalf("带附件回复失败: %d %s", resp.StatusCode, data)
	}
	resp, data = request("GET", fmt.Sprintf("/api/forum/posts/%d", fpid), nil, "")
	if resp.StatusCode != 200 || strings.Count(string(data), fileURL) < 2 {
		t.Fatalf("帖子详情未同时返回帖子与回复附件: %d %s", resp.StatusCode, data)
	}

	// 非法附件 URL（目录穿越）必须被拒绝
	for _, bad := range []string{"/uploads/../etc/passwd", "https://evil.example/x.txt", "/uploads/a/b.txt"} {
		resp, data = request("POST", "/api/questions", map[string]any{
			"category_id": cats.Items[0].ID, "title": "非法附件", "body": "x",
			"attachments": []map[string]string{{"name": "x", "url": bad}},
		}, tokenA)
		if resp.StatusCode == 200 {
			t.Fatalf("非法附件 URL %q 应被拒绝，实际 %d %s", bad, resp.StatusCode, data)
		}
	}

	request("DELETE", fmt.Sprintf("/api/questions/%d", qid), nil, tokenA)
	request("DELETE", fmt.Sprintf("/api/forum/posts/%d", fpid), nil, tokenA)
	t.Log("附件流程测试通过")
}

// TestPrerenderAndSitemap 校验爬虫预渲染：爬虫拿到含正文的 HTML，普通访客仍是 SPA 外壳。
func TestPrerenderAndSitemap(t *testing.T) {
	tokenA := registerUser(t, "预渲染测试员", "prerender@example.com")
	resp, data := request("GET", "/api/categories", nil, "")
	var cats struct {
		Items []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	json.Unmarshal(data, &cats)
	_, data = request("POST", "/api/questions", map[string]any{
		"category_id": cats.Items[0].ID,
		"title":       "预渲染专属标题ABC",
		"body":        "这是预渲染正文内容XYZ",
	}, tokenA)
	qid := idOf(t, data)

	// 爬虫 UA：应拿到含标题与正文的 HTML
	resp, data = requestUA("GET", fmt.Sprintf("/ask/%d", qid), "Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)", "")
	if resp.StatusCode != 200 {
		t.Fatalf("爬虫访问详情页失败: %d", resp.StatusCode)
	}
	html := string(data)
	for _, want := range []string{"预渲染专属标题ABC", "这是预渲染正文内容XYZ", `<link rel="canonical"`, "application/ld+json", "QAPage"} {
		if !strings.Contains(html, want) {
			t.Fatalf("预渲染 HTML 缺少 %q：%s", want, firstN(html, 600))
		}
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("预渲染响应应为 no-store，实际 %q", got)
	}

	// 普通浏览器 UA：仍是 SPA 外壳（无正文）
	resp, data = requestUA("GET", fmt.Sprintf("/ask/%d", qid), "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0", "")
	if resp.StatusCode != 200 || strings.Contains(string(data), "这是预渲染正文内容XYZ") {
		t.Fatalf("普通访客不应拿到预渲染正文: %d", resp.StatusCode)
	}

	// 列表页预渲染含详情页真实链接
	_, data = requestUA("GET", "/ask?page=1", "Baiduspider", "")
	if !strings.Contains(string(data), fmt.Sprintf("/ask/%d", qid)) {
		t.Fatalf("列表页预渲染缺少详情链接")
	}

	// sitemap 覆盖详情页
	resp, data = requestUA("GET", "/sitemap.xml", "Baiduspider", "")
	if resp.StatusCode != 200 || !strings.Contains(string(data), fmt.Sprintf("/ask/%d", qid)) {
		t.Fatalf("sitemap 未包含问题详情: %d %s", resp.StatusCode, firstN(string(data), 300))
	}
	if !strings.Contains(string(data), "<urlset") {
		t.Fatalf("sitemap 格式异常: %s", firstN(string(data), 300))
	}

	request("DELETE", fmt.Sprintf("/api/questions/%d", qid), nil, tokenA)
	t.Log("预渲染与 sitemap 测试通过")
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
