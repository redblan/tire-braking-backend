package auth

import (
	"errors"
	"time"

	"tire-braking/internal/app/config"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	UserID      uint `json:"user_id"`
	IsModerator bool `json:"is_moderator"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTManager(cfg *config.Config) *JWTManager {
	return &JWTManager{secret: []byte(cfg.JWTSecret), ttl: cfg.JWTTTL}
}

// GenerateToken — HS256-токен с user_id и is_moderator, живёт JWT_TTL
func (m *JWTManager) GenerateToken(userID uint, isModerator bool) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:      userID,
		IsModerator: isModerator,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// ParseToken — проверяет подпись и срок действия; любая проблема -> ErrInvalidToken
func (m *JWTManager) ParseToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
