package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("auth: invalid jwt token")
	ErrTokenExpired = errors.New("auth: jwt token expired")
)

var globalSecret []byte

// Init 设置全局 JWT 密钥（HMAC-SHA256）
func Init(secret string) {
	globalSecret = []byte(secret)
}

// Claims 自定义载荷
type Claims struct {
	UserId     int `json:"user_id"`
	AuthUserId int `json:"uid"`
	Type       int `json:"type"` // 1=user 2=service 3=corp
	jwt.RegisteredClaims
}

// Encode 生成 JWT token，有效期 days 天
func Encode(userId, authUserId, tokenType int, days int) (string, error) {
	if len(globalSecret) == 0 {
		return "", errors.New("auth: jwt secret not initialized, call auth.Init first")
	}
	if days <= 0 {
		days = 365
	}
	claims := Claims{
		UserId:     userId,
		AuthUserId: authUserId,
		Type:       tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(days) * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(globalSecret)
}

// Decode 解析并校验 JWT token
func Decode(tokenString string) (*Claims, error) {
	if len(globalSecret) == 0 {
		return nil, errors.New("auth: jwt secret not initialized, call auth.Init first")
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method: %v", t.Header["alg"])
		}
		return globalSecret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// ParseUserId 从 token 中解析 userId / authUserId
func ParseUserId(tokenString string) (userId int, authUserId int, err error) {
	claims, err := Decode(tokenString)
	if err != nil {
		return 0, 0, err
	}
	switch claims.Type {
	case 1:
		return claims.UserId, claims.AuthUserId, nil
	case 2:
		return claims.AuthUserId, 0, nil
	case 3:
		return claims.UserId, claims.AuthUserId, nil
	default:
		return 0, 0, fmt.Errorf("auth: unknown token type %d", claims.Type)
	}
}
