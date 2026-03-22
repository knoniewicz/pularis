package config

import (
	"os"
	"runtime"
	"strconv"
)

type Config struct {
	RedisAddr  string
	ConfigsDir string
	MaxWorkers int
}

func LoadConfig() Config {
	return Config{
		RedisAddr:  getEnv("REDIS_ADDR", ""),
		ConfigsDir: getEnv("CONFIGS_DIR", "configs"),
		MaxWorkers: getEnvInt("MAX_WORKERS", runtime.NumCPU()),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}
