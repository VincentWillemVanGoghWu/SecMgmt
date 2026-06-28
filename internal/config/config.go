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
	FFmpegPath             string
	LiveHLSSegmentSeconds  int
	LiveHLSListSize        int
	LiveHLSStartTimeout    int
	LiveHLSSessionTTL      int
	LiveHLSMaxSessions     int
	LiveHLSTranscode       bool
	AICallbackSecret       string
	PushHTTPTimeoutSeconds int
}

func Load(rootDir string) (*Config, error) {
	envPath := filepath.Join(rootDir, ".env")
	if err := godotenv.Load(envPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

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
		FFmpegPath:             readString("FFMPEG_PATH", "ffmpeg"),
		LiveHLSSegmentSeconds:  readInt("LIVE_HLS_SEGMENT_SECONDS", 2),
		LiveHLSListSize:        readInt("LIVE_HLS_LIST_SIZE", 6),
		LiveHLSStartTimeout:    readInt("LIVE_HLS_START_TIMEOUT_SECONDS", 30),
		LiveHLSSessionTTL:      readInt("LIVE_HLS_SESSION_TTL_SECONDS", 300),
		LiveHLSMaxSessions:     readInt("LIVE_HLS_MAX_SESSIONS", 16),
		LiveHLSTranscode:       readBool("LIVE_HLS_TRANSCODE", true),
		AICallbackSecret:       readString("AI_CALLBACK_SECRET", "change-ai-signature-secret"),
		PushHTTPTimeoutSeconds: readInt("PUSH_HTTP_TIMEOUT_SECONDS", 10),
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

func readBool(key string, fallback bool) bool {
	if !viper.IsSet(key) {
		return fallback
	}
	return viper.GetBool(key)
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
