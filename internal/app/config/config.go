package config

import (
	"os"
	"time"
)

type Config struct {
	JWTSecret string
	JWTTTL    time.Duration
	RedisHost string
	RedisPort string
}

// Load — читает настройки из переменных окружения, если не заданы — значения для локального запуска
func Load() *Config {
	ttl, err := time.ParseDuration(getEnv("JWT_TTL", "1h"))
	if err != nil {
		ttl = time.Hour
	}

	return &Config{
		JWTSecret: getEnv("JWT_SECRET", "local-dev-secret"),
		JWTTTL:    ttl,
		RedisHost: getEnv("REDIS_HOST", "localhost"),
		RedisPort: getEnv("REDIS_PORT", "6379"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
