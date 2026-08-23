package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type ipBucket struct {
	count int
	reset time.Time
}

// RateLimit 简单的每 IP 滑动窗口限流。
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	var (
		mu    sync.Mutex
		table = make(map[string]*ipBucket)
	)
	go func() {
		for range time.Tick(window) {
			mu.Lock()
			for k, b := range table {
				if time.Now().After(b.reset) {
					delete(table, k)
				}
			}
			mu.Unlock()
		}
	}()
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		mu.Lock()
		b, ok := table[ip]
		if !ok || now.After(b.reset) {
			table[ip] = &ipBucket{count: 1, reset: now.Add(window)}
			b = table[ip]
		} else {
			b.count++
		}
		over := b.count > limit
		mu.Unlock()
		if over {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "请求过于频繁，请稍后再试"})
			return
		}
		c.Next()
	}
}
