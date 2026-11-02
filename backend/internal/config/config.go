package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DeepSeekAPIKey string
	DeepSeekBase   string
	DeepSeekModel  string

	ZhipuAPIKey string
	ZhipuBase   string
	ZhipuModel  string
	EmbedDim    int

	MilvusAddr       string
	MilvusCollection string

	DatabaseURL    string
	ServerAddr     string
	ExtractWorkers int
	UploadDir      string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // 找不到 .env 时忽略

	return &Config{
		DeepSeekAPIKey: getenv("DEEPSEEK_API_KEY", ""),
		DeepSeekBase:   getenv("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
		DeepSeekModel:  getenv("DEEPSEEK_MODEL", "deepseek-chat"),

		ZhipuAPIKey: getenv("ZHIPU_API_KEY", ""),
		ZhipuBase:   getenv("ZHIPU_BASE_URL", "https://open.bigmodel.cn/api/paas/v4"),
		ZhipuModel:  getenv("ZHIPU_MODEL", "embedding-3"),
		EmbedDim:    getenvInt("EMBED_DIM", 1024),

		MilvusAddr:       getenv("MILVUS_ADDR", "localhost:19530"),
		MilvusCollection: getenv("MILVUS_COLLECTION", "user_profiles"),

		DatabaseURL: getenv("DATABASE_URL", "postgres://ntmu:ntmupass@localhost:5433/ntmu"),
		ServerAddr:  getenv("SERVER_ADDR", ":8080"),

		ExtractWorkers: getenvInt("EXTRACT_WORKERS", 4),
		UploadDir:      getenv("UPLOAD_DIR", "uploads"),
	}, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
