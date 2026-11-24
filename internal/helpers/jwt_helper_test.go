package helpers

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
)

func TestTokenTypeClaim(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	access, err := GenerateToken(1, "a@x.com", TokenTypeAccess, key, "kid", 1)
	if err != nil {
		t.Fatal(err)
	}
	refresh, err := GenerateToken(1, "a@x.com", TokenTypeRefresh, key, "kid", 1)
	if err != nil {
		t.Fatal(err)
	}

	ac, err := ValidateToken(access, &key.PublicKey)
	if err != nil || ac.TokenType != TokenTypeAccess {
		t.Fatalf("access token: typ=%q err=%v", ac.TokenType, err)
	}
	rc, err := ValidateToken(refresh, &key.PublicKey)
	if err != nil || rc.TokenType != TokenTypeRefresh {
		t.Fatalf("refresh token: typ=%q err=%v", rc.TokenType, err)
	}
	if ac.ID == rc.ID {
		t.Fatal("access and refresh tokens must have distinct jti")
	}
}
