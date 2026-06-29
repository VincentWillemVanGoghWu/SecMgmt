package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const (
	DefaultPushEmailSMTPHost = "smtp.163.com"
	DefaultPushEmailSMTPPort = 465
	DefaultPushEmailUsername = "18017751995@163.com"
	DefaultPushEmailPassword = "MLY39eMywrC3KLG8"
	DefaultPushEmailFrom     = "18017751995@163.com"
	DefaultPushEmailFromName = "SMARTLINK(7*24\u5c0f\u65f6\u6316\u6398\u673a)"
)

type Config struct {
	AppName                string
	AppEnv                 string
	HTTPPort               int
	MySQLDSN               string
	RedisAddr              string
	RedisDB                int
	JWTSecretKey           string
	DeviceSecretKey        string
	JWTExpireMinutes       int
	HikvisionSDKPath       string
	MediaRootDir           string
	MediaMountPath         string
	BackendPublicBaseURL   string
	AICallbackSecret       string
	PushHTTPTimeoutSeconds int
	PushEmailSMTPHost      string
	PushEmailSMTPPort      int
	PushEmailUsername      string
	PushEmailPassword      string
	PushEmailFrom          string
	PushEmailFromName      string
}

func Load(rootDir string) (*Config, error) {
	envPath := filepath.Join(rootDir, ".env")
	_ = godotenv.Load(envPath)

	viper.AutomaticEnv()

	cfg := &Config{
		AppName:                readString("APP_NAME", "secmgmt-go"),
		AppEnv:                 readString("APP_ENV", "development"),
		HTTPPort:               readInt("HTTP_PORT", 8000),
		MySQLDSN:               readString("MYSQL_DSN", ""),
		RedisAddr:              readString("REDIS_ADDR", "127.0.0.1:6379"),
		RedisDB:                readInt("REDIS_DB", 0),
		JWTSecretKey:           readString("JWT_SECRET_KEY", "change-me"),
		DeviceSecretKey:        readString("DEVICE_SECRET_KEY", ""),
		JWTExpireMinutes:       readInt("JWT_EXPIRE_MINUTES", 1440),
		HikvisionSDKPath:       readString("HIKVISION_SDK_PATH", defaultSDKPath(rootDir)),
		MediaRootDir:           readString("MEDIA_ROOT_DIR", filepath.Join(rootDir, "media")),
		MediaMountPath:         normalizeMountPath(readString("MEDIA_MOUNT_PATH", "/media")),
		BackendPublicBaseURL:   readString("BACKEND_PUBLIC_BASE_URL", "http://127.0.0.1:8000"),
		AICallbackSecret:       readString("AI_CALLBACK_SECRET", "change-ai-signature-secret"),
		PushHTTPTimeoutSeconds: readInt("PUSH_HTTP_TIMEOUT_SECONDS", 10),
		PushEmailSMTPHost:      readString("PUSH_EMAIL_SMTP_HOST", DefaultPushEmailSMTPHost),
		PushEmailSMTPPort:      readInt("PUSH_EMAIL_SMTP_PORT", DefaultPushEmailSMTPPort),
		PushEmailUsername:      readString("PUSH_EMAIL_SMTP_USERNAME", DefaultPushEmailUsername),
		PushEmailPassword:      readString("PUSH_EMAIL_SMTP_PASSWORD", DefaultPushEmailPassword),
		PushEmailFrom:          readString("PUSH_EMAIL_FROM", DefaultPushEmailFrom),
		PushEmailFromName:      readString("PUSH_EMAIL_FROM_NAME", DefaultPushEmailFromName),
	}

	if strings.TrimSpace(cfg.MySQLDSN) == "" {
		return nil, fmt.Errorf("MYSQL_DSN is required")
	}
	if err := os.MkdirAll(cfg.MediaRootDir, 0o755); err != nil {
		return nil, fmt.Errorf("create media root dir: %w", err)
	}

	return cfg, nil
}

func (c *Config) HTTPAddr() string {
	return fmt.Sprintf(":%d", c.HTTPPort)
}

func readString(key, fallback string) string {
	value := strings.TrimSpace(viper.GetString(key))
	if value == "" {
		return fallback
	}
	return value
}

func readInt(key string, fallback int) int {
	if !viper.IsSet(key) {
		return fallback
	}
	return viper.GetInt(key)
}

func defaultSDKPath(rootDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(rootDir, "third_party", "HCNetSDK_Win64")
	}
	return filepath.Join(rootDir, "third_party", "HCNetSDK_Linux64")
}

func normalizeMountPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return "/media"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return strings.TrimRight(value, "/")
}
