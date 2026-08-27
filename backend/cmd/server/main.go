package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"guangyanji/internal/config"
	"guangyanji/internal/db"
	"guangyanji/internal/handlers"
	"guangyanji/internal/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置错误: %v", err)
	}
	conn, err := db.Open(cfg.DBDSN)
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer conn.Close()

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.Logger(), gin.Recovery())
	r.MaxMultipartMemory = 50 << 20

	h := handlers.New(conn, cfg)

	// 健康检查
	r.GET("/healthz", h.Healthz)

	api := r.Group("/api")
	{
		authPub := api.Group("/auth", middleware.RateLimit(20, time.Minute))
		{
			authPub.POST("/register", h.Register)
			authPub.POST("/login", h.Login)
			authPub.POST("/logout", h.Logout)
		}
		authPriv := api.Group("/auth", middleware.Auth(conn, cfg.JWTSecret))
		{
			authPriv.GET("/me", h.Me)
			authPriv.PUT("/me", h.UpdateMe)
			authPriv.POST("/password", h.ChangePassword)
		}

		// 公开分类
		api.GET("/categories", h.PublicCategories)

		// 问答
		api.GET("/questions", middleware.OptionalAuth(conn, cfg.JWTSecret), h.ListQuestions)
		api.POST("/questions", middleware.Auth(conn, cfg.JWTSecret), middleware.RateLimit(10, time.Minute), h.CreateQuestion)
		api.GET("/questions/:id", h.GetQuestion)
		api.PUT("/questions/:id", middleware.Auth(conn, cfg.JWTSecret), h.UpdateQuestion)
		api.DELETE("/questions/:id", middleware.Auth(conn, cfg.JWTSecret), h.DeleteQuestion)
		api.POST("/questions/:id/view", h.RegisterView)
		api.POST("/questions/:id/bookmark", middleware.Auth(conn, cfg.JWTSecret), h.ToggleBookmark)
		api.POST("/questions/:id/follow", middleware.Auth(conn, cfg.JWTSecret), h.ToggleQuestionFollow)
		api.POST("/questions/:id/comments", middleware.Auth(conn, cfg.JWTSecret), h.CreateComment)
		api.GET("/questions/:id/comments", h.ListComments)
		api.GET("/questions/:id/answers", h.ListAnswers)
		api.POST("/questions/:id/answers", middleware.Auth(conn, cfg.JWTSecret), middleware.RateLimit(10, time.Minute), h.CreateAnswer)
		api.PUT("/answers/:id", middleware.Auth(conn, cfg.JWTSecret), h.UpdateAnswer)
		api.DELETE("/answers/:id", middleware.Auth(conn, cfg.JWTSecret), h.DeleteAnswer)
		api.POST("/answers/:id/accept", middleware.Auth(conn, cfg.JWTSecret), h.AcceptAnswer)
		api.POST("/questions/:id/close", middleware.Auth(conn, cfg.JWTSecret), h.CloseQuestion)
		api.DELETE("/comments/:id", middleware.Auth(conn, cfg.JWTSecret), h.DeleteComment)

		// 书签
		api.GET("/bookmarks", middleware.Auth(conn, cfg.JWTSecret), h.ListBookmarks)

		// 投票
		api.POST("/votes", middleware.Auth(conn, cfg.JWTSecret), middleware.RateLimit(30, time.Minute), h.ToggleVote)
		api.POST("/votes/status", middleware.Auth(conn, cfg.JWTSecret), h.VoteStatus)

		// 知识库
		api.GET("/articles", h.ListArticles)
		api.POST("/articles", middleware.Auth(conn, cfg.JWTSecret), middleware.RateLimit(10, time.Minute), h.CreateArticle)
		api.GET("/articles/:id", middleware.OptionalAuth(conn, cfg.JWTSecret), h.GetArticle)
		api.PUT("/articles/:id", middleware.Auth(conn, cfg.JWTSecret), h.UpdateArticle)
		api.DELETE("/articles/:id", middleware.Auth(conn, cfg.JWTSecret), h.DeleteArticle)
		api.POST("/articles/:id/view", h.RegisterArticleView)
		api.GET("/articles/:id/comments", h.ListArticleComments)
		api.POST("/articles/:id/comments", middleware.Auth(conn, cfg.JWTSecret), middleware.RateLimit(10, time.Minute), h.CreateArticleComment)
		api.DELETE("/article-comments/:id", middleware.Auth(conn, cfg.JWTSecret), h.DeleteArticleComment)

		// 论坛
		api.GET("/boards", h.ListBoards)
		api.GET("/forum/posts", h.ListForumPosts)
		api.POST("/forum/posts", middleware.Auth(conn, cfg.JWTSecret), middleware.RateLimit(10, time.Minute), h.CreateForumPost)
		api.GET("/forum/posts/:id", h.GetForumPost)
		api.PUT("/forum/posts/:id", middleware.Auth(conn, cfg.JWTSecret), h.UpdateForumPost)
		api.DELETE("/forum/posts/:id", middleware.Auth(conn, cfg.JWTSecret), h.DeleteForumPost)
		api.POST("/forum/posts/:id/view", h.RegisterForumPostView)
		api.POST("/forum/posts/:id/replies", middleware.Auth(conn, cfg.JWTSecret), middleware.RateLimit(10, time.Minute), h.CreateForumReply)
		api.PUT("/forum/replies/:id", middleware.Auth(conn, cfg.JWTSecret), h.UpdateForumReply)
		api.DELETE("/forum/replies/:id", middleware.Auth(conn, cfg.JWTSecret), h.DeleteForumReply)

		// 就业共享表格
		api.GET("/jobs", h.ListJobs)
		api.POST("/jobs", middleware.Auth(conn, cfg.JWTSecret), middleware.RateLimit(20, time.Minute), h.CreateJob)
		api.PUT("/jobs/:id", middleware.Auth(conn, cfg.JWTSecret), middleware.RateLimit(30, time.Minute), h.UpdateJob)
		api.DELETE("/jobs/:id", middleware.Auth(conn, cfg.JWTSecret), h.DeleteJob)

		// 通知
		api.GET("/notifications", middleware.Auth(conn, cfg.JWTSecret), h.ListNotifications)
		api.POST("/notifications/read", middleware.Auth(conn, cfg.JWTSecret), h.ReadNotifications)

		// 上传
		api.POST("/uploads", middleware.Auth(conn, cfg.JWTSecret), middleware.RateLimit(20, time.Minute), h.UploadFiles)
		api.DELETE("/uploads/:filename", middleware.Auth(conn, cfg.JWTSecret), h.DeleteUpload)

		// 管理后台
		admin := api.Group("/admin", middleware.Auth(conn, cfg.JWTSecret), middleware.Admin())
		{
			admin.GET("/stats", h.AdminStats)
			admin.GET("/users", h.AdminUsers)
			admin.PUT("/users/:id", h.UpdateAdminUser)
			admin.DELETE("/users/:id", h.DeleteAdminUser)
			admin.GET("/categories", h.AdminCategories)
			admin.POST("/categories", h.CreateAdminCategory)
			admin.PUT("/categories/:id", h.UpdateAdminCategory)
			admin.DELETE("/categories/:id", h.DeleteAdminCategory)
			admin.GET("/audit", h.AdminAudit)
		}
	}

	// 公开附件静态托管
	r.StaticFS("/uploads", http.Dir(cfg.UploadDir))

	// 前端静态托管：设置了 FRONTEND_DIST 时，由后端同时提供页面与 API（单端口部署）
	if info, err := os.Stat(cfg.FrontendDist); err == nil && info.IsDir() {
		r.NoRoute(func(c *gin.Context) {
			if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
				c.Status(http.StatusNotFound)
				return
			}
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
				return
			}
			p := filepath.Join(cfg.FrontendDist, filepath.Clean(c.Request.URL.Path))
			if f, err := os.Stat(p); err == nil && !f.IsDir() {
				c.File(p)
				return
			}
			c.File(filepath.Join(cfg.FrontendDist, "index.html"))
		})
	}

	log.Printf("光研集后端启动于 :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
