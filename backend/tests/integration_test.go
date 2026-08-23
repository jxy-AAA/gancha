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
