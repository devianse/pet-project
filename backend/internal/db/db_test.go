package db

import "testing"

func TestOpen_InvalidDSN(t *testing.T) {
	_, err := Open("not a valid dsn ::")
	if err == nil {
		t.Fatal("expected error for invalid DSN, got nil")
	}
}
