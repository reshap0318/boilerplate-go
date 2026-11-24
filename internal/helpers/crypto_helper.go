package helpers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// GenerateRandomString generates a cryptographically secure random string.
func GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// HashToken returns a deterministic SHA-256 hex digest of a high-entropy token
// (reset tokens, API keys). Use it when the hash must be looked up with
// WHERE token = ?. Never use it for passwords — use HashString (bcrypt) there.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// HashString hashes a string using bcrypt (salted, non-deterministic).
// Suitable for passwords only — the output differs on every call, so it can
// never be used as a lookup key.
func HashString(str string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(str), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// VerifyString verifies a string against its hash.
func VerifyString(str, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(str))
}
