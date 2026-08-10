// backend/internal/auth/token_test.go
package auth

import (
	"testing"
	"time"
)

func TestSignToken_ParseToken_RoundTrip(t *testing.T) {
	secret := []byte("test-secret")

	tokenString, err := SignToken(secret, 42, "mike", "admin")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	claims, err := ParseToken(secret, tokenString)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.Username != "mike" || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	userID, err := claims.UserID()
	if err != nil {
		t.Fatalf("UserID: %v", err)
	}
	if userID != 42 {
		t.Fatalf("expected userID 42, got %d", userID)
	}
}

func TestParseToken_RejectsWrongSecret(t *testing.T) {
	tokenString, err := SignToken([]byte("secret-a"), 1, "mike", "admin")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	if _, err := ParseToken([]byte("secret-b"), tokenString); err == nil {
		t.Fatal("expected an error parsing a token with the wrong secret")
	}
}

func TestParseToken_RejectsExpiredToken(t *testing.T) {
	secret := []byte("test-secret")
	claims := Claims{
		Username: "mike",
		Role:     "admin",
	}
	claims.Subject = "1"
	claims.ExpiresAt = jwtNumericDate(time.Now().Add(-time.Hour))
	claims.IssuedAt = jwtNumericDate(time.Now().Add(-2 * time.Hour))

	tokenString, err := signClaims(secret, claims)
	if err != nil {
		t.Fatalf("signClaims: %v", err)
	}

	if _, err := ParseToken(secret, tokenString); err == nil {
		t.Fatal("expected an error parsing an expired token")
	}
}
