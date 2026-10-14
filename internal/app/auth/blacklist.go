package auth

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const blacklistPrefix = "blacklist:"

type Blacklist struct {
	client *goredis.Client
}

func NewBlacklist(client *goredis.Client) *Blacklist {
	return &Blacklist{client: client}
}

// AddToBlacklist — ключ blacklist:<token> живёт ttl (оставшееся время жизни токена)
func (b *Blacklist) AddToBlacklist(ctx context.Context, token string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	return b.client.Set(ctx, blacklistPrefix+token, 1, ttl).Err()
}

func (b *Blacklist) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	n, err := b.client.Exists(ctx, blacklistPrefix+token).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
