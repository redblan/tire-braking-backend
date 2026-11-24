package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

// blacklistKey — blacklist:<хеш токена>: в Redis виден короткий ключ, а не весь JWT
func blacklistKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return blacklistPrefix + hex.EncodeToString(sum[:16])
}

// AddToBlacklist — ключ живёт ttl (оставшееся время жизни токена), значение — user_id=<id>
func (b *Blacklist) AddToBlacklist(ctx context.Context, token string, userID uint, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	return b.client.Set(ctx, blacklistKey(token), fmt.Sprintf("user_id=%d", userID), ttl).Err()
}

func (b *Blacklist) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	n, err := b.client.Exists(ctx, blacklistKey(token)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
