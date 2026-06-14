package system

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const (
	UserRoleAdmin  = "admin"
	UserRoleViewer = "viewer"
)

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
	DisplayName  string
	Enabled      bool
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	const iterations = 10000
	sum := append([]byte(password), salt...)
	for range iterations {
		hash := sha256.Sum256(sum)
		sum = hash[:]
	}

	return fmt.Sprintf(
		"sha256:%d:%s:%s",
		iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, ":")
	if len(parts) != 4 || parts[0] != "sha256" {
		return false
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}

	sum := append([]byte(password), salt...)
	for range iterations {
		hash := sha256.Sum256(sum)
		sum = hash[:]
	}

	return subtle.ConstantTimeCompare(sum, want) == 1
}
