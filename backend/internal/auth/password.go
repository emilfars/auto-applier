// Package auth implements Auto Applier accounts: registration, email
// verification, login/logout sessions, password reset, and auth rate limiting.
//
// Password hashing uses PBKDF2-HMAC-SHA256 (stdlib crypto/pbkdf2), storing a
// self-describing string: pbkdf2_sha256$<iter>$<salt_b64>$<hash_b64>.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	pbkdf2Iterations = 210_000
	saltLen          = 16
	keyLen           = 32
	tokenBytes       = 32
)

var b64 = base64.RawStdEncoding

// HashPassword returns an encoded PBKDF2-SHA256 hash of password.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("auth: password must not be empty")
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: read salt: %w", err)
	}
	dk, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, keyLen)
	if err != nil {
		return "", fmt.Errorf("auth: derive key: %w", err)
	}
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s",
		pbkdf2Iterations, b64.EncodeToString(salt), b64.EncodeToString(dk)), nil
}

// VerifyPassword reports whether password matches the encoded hash, using a
// constant-time comparison.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err := b64.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := b64.DecodeString(parts[3])
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// NewToken returns a URL-safe, cryptographically-random opaque token suitable
// for verification links, sessions, and password resets.
func NewToken() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: read token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
