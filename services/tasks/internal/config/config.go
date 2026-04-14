package config

import "time"

type Config struct {
	RedisAddr      string
	CacheTTL       time.Duration
	CacheTTLJitter time.Duration
}

func New() Config {
	return Config{
		RedisAddr:      "redis:6379", // имя сервиса в docker-compose
		CacheTTL:       120 * time.Second,
		CacheTTLJitter: 30 * time.Second,
	}
}
