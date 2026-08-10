package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenTTL is how long a session cookie stays valid — one long-lived
// token, reissued on login, no refresh flow (see the design spec's
// "single long-lived token" decision).
const TokenTTL = 14 * 24 * time.Hour

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// UserID parses the standard `sub` claim back into the numeric user id
// SignToken put there.
func (c *Claims) UserID() (int64, error) {
	return strconv.ParseInt(c.Subject, 10, 64)
}

var ErrInvalidToken = errors.New("invalid token")

// jwtNumericDate is a small local alias so token_test.go doesn't need its
// own import of the jwt package just to build an expired-token fixture.
func jwtNumericDate(t time.Time) *jwt.NumericDate {
	return jwt.NewNumericDate(t)
}

func signClaims(secret []byte, claims Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func SignToken(secret []byte, userID int64, username, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		},
	}
	return signClaims(secret, claims)
}

func ParseToken(secret []byte, tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
