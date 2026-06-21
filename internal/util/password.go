package util

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
)

func CheckPassword(plainText, stored string) bool {
	if stored == "" {
		return false
	}

	if bcrypt.CompareHashAndPassword([]byte(stored), []byte(plainText)) == nil {
		return true
	}

	if checkPBKDF2SHA256Password(plainText, stored) {
		return true
	}

	return plainText == stored
}

func checkPBKDF2SHA256Password(plainText, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}

	salt := parts[2]
	expectedHash, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}

	derived := pbkdf2.Key([]byte(plainText), []byte(salt), iterations, len(expectedHash), sha256.New)
	return subtle.ConstantTimeCompare(derived, expectedHash) == 1
}
