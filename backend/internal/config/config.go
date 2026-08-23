package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port           string
	DBDSN          string
	JWTSecret      string
	UploadDir      string
	RegisterEnable bool
}

// Load 读取 backend/ 目录下的 .env（若存在），再叠加进程环境变量。
func Load() (*Config, error) {
	loadDotEnv()
	cfg := &Config{
		Port:           getenv("PORT", "8080"),
		JWTSecret:      getenv("JWT_SECRET", ""),
		UploadDir:      getenv("UPLOAD_DIR", "uploads"),
		RegisterEnable: getenv("REGISTER_ENABLED", "true") != "false",
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET 未配置（可复制 .env.example 为 .env 后填写）")
	}
	cfg.DBDSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		getenv("DB_USER", "guangyanji"),
		getenv("DB_PASSWORD", "guangyanji_dev"),
		getenv("DB_HOST", "127.0.0.1"),
		getenv("DB_PORT", "3306"),
		getenv("DB_NAME", "guangyanji"),
	)
	if dir, err := os.Getwd(); err == nil {
		cfg.UploadDir = absPath(dir, cfg.UploadDir)
	}
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func absPath(base, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}

// loadDotEnv 解析 .env 文件（KEY=VALUE，忽略注释与空行），不覆盖已有环境变量。
func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func ParseBool(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}
