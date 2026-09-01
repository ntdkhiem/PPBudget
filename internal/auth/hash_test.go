package auth

import (
	"testing"
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "superSecretPassword123!"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if hash == "" {
		t.Fatalf("expected non-empty hash")
	}

	if !CheckPasswordHash(password, hash) {
		t.Fatalf("expected password to match hash")
	}

	if CheckPasswordHash("wrongPassword", hash) {
		t.Fatalf("expected wrong password to fail check")
	}
}
