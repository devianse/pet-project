package auth

import "testing"

func TestHashPassword_VerifyPassword_RoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("expected a non-empty hash")
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("expected VerifyPassword to accept the original password")
	}
}

func TestVerifyPassword_RejectsWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("expected VerifyPassword to reject an incorrect password")
	}
}
