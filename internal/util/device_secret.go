package util

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/fernet/fernet-go"
)

func EncryptDeviceSecret(secretKey, plainText string) (string, error) {
	if plainText == "" {
		return "", nil
	}
	keys, err := decodeDeviceSecretKeys(secretKey)
	if err != nil {
		return "", err
	}
	token, err := fernet.EncryptAndSign([]byte(plainText), keys[0])
	if err != nil {
		return "", fmt.Errorf("encrypt device secret: %w", err)
	}
	return string(token), nil
}

func DecryptDeviceSecret(secretKey, encrypted string) (string, error) {
	trimmedEncrypted := strings.TrimSpace(encrypted)
	if trimmedEncrypted == "" {
		return "", nil
	}
	keys, err := decodeDeviceSecretKeys(secretKey)
	if err != nil {
		return "", err
	}
	plainText := fernet.VerifyAndDecrypt([]byte(trimmedEncrypted), 0*time.Second, keys)
	if plainText == nil {
		return "", fmt.Errorf("invalid encrypted device secret")
	}
	return string(plainText), nil
}

func ResolveDeviceSecret(secretKey, stored string) (string, error) {
	if strings.TrimSpace(stored) == "" {
		return "", nil
	}
	plainText, err := DecryptDeviceSecret(secretKey, stored)
	if err == nil {
		return plainText, nil
	}
	if err.Error() == "invalid encrypted device secret" {
		return stored, nil
	}
	return "", err
}

func decodeDeviceSecretKeys(secretKey string) ([]*fernet.Key, error) {
	trimmedSecret := strings.TrimSpace(secretKey)
	if trimmedSecret == "" {
		return nil, fmt.Errorf("DEVICE_SECRET_KEY is empty")
	}

	digest := sha256.Sum256([]byte(trimmedSecret))
	fernetKey := base64.URLEncoding.EncodeToString(digest[:])
	keys, err := fernet.DecodeKeys(fernetKey)
	if err != nil {
		return nil, fmt.Errorf("decode fernet key: %w", err)
	}
	return keys, nil
}
