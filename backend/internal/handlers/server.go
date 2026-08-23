package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"guangyanji/internal/config"

	"github.com/gin-gonic/gin"
)

type Server struct {
	DB     *sql.DB
	Cfg    *config.Config
}

func New(db *sql.DB, cfg *config.Config) *Server {
	return &Server{DB: db, Cfg: cfg}
}

func (s *Server) Healthz(c *gin.Context) {
	if err := s.DB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// notify 给指定用户插入一条站内通知（忽略失败，不阻塞主流程）。
func (s *Server) notify(userID int64, typ, message string, questionID *int64) {
	_, err := s.DB.Exec(`INSERT INTO notifications (user_id, type, message, question_id) VALUES (?, ?, ?, ?)`,
		userID, typ, message, questionID)
	if err != nil {
		log.Printf("notify failed: %v", err)
	}
}
