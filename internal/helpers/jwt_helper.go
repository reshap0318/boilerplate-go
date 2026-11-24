package helpers

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Token types stored in the "typ" claim. Access and refresh tokens are signed
// with the same key, so the type claim is what keeps them from being swapped.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// JWTClaims represents custom claims for JWT tokens
type JWTClaims struct {
	UserID    uint   `json:"user_id"`
	Email     string `json:"email"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

// GenerateKeyID generates a unique key ID from public key fingerprint
func GenerateKeyID(publicKey *rsa.PublicKey) string {
	// Get public key bytes
	pubBytes := publicKey.N.Bytes()

	// Hash with SHA256
	hash := sha256.Sum256(pubBytes)

	// Return first 8 chars of hex hash
	return hex.EncodeToString(hash[:])[:8]
}

// LoadPrivateKey loads and decrypts private key from PEM file
func LoadPrivateKey(path string, passphrase string) (*rsa.PrivateKey, error) {
	// Read private key file
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	// Decrypt and parse private key
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEMWithPassword(keyData, passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return privateKey, nil
}

// LoadPublicKey loads public key from PEM file
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	// Read public key file
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	// Parse public key
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return publicKey, nil
}

// GenerateToken creates a signed RS256 JWT. tokenType is TokenTypeAccess or
// TokenTypeRefresh; it is stored in the "typ" claim so the two can't be swapped.
func GenerateToken(userID uint, email, tokenType string, privateKey *rsa.PrivateKey, kid string, expirationHours int) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		UserID:    userID,
		Email:     email,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expirationHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign %s token: %w", tokenType, err)
	}

	return tokenString, nil
}

// LoadPassphrase reads the JWT passphrase from a file.
func LoadPassphrase(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read passphrase file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// ValidateToken validates and parses a JWT token, returns claims
func ValidateToken(tokenString string, publicKey *rsa.PublicKey) (*JWTClaims, error) {
	claims := &JWTClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
