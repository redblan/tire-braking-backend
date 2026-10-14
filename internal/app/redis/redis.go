package redis

import (
	"context"
	"net"
	"time"

	"tire-braking/internal/app/config"

	goredis "github.com/redis/go-redis/v9"
)

// New — создаёт клиент Redis и проверяет соединение через Ping
func New(cfg *config.Config) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr: net.JoinHostPort(cfg.RedisHost, cfg.RedisPort),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}
