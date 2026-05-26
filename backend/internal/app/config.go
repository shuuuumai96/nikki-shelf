package app

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/images"
)

type Config struct {
	Addr                         string
	DatabaseURL                  string
	LogFormat                    string
	LogLevel                     string
	UploadDir                    string
	PublicUploadBase             string
	CookieSecure                 bool
	CORSAllowedOrigins           []string
	AllowAdditionalSignups       bool
	AllowFirstUserSetup          bool
	FirstUserBootstrapToken      string
	AuthRateLimitIPAttempts      int
	AuthRateLimitAccountAttempts int
	AuthRateLimitWindow          time.Duration
	AuthRateLimitLockout         time.Duration
	AuthRateLimitMaxEntries      int
	IPExtractorMode              string
	TrustedProxyCIDRs            []string
	StripImageMetadata           bool
	ImageUserQuotaBytes          int64
	ImageUserQuotaCount          int
	ImageTotalQuotaBytes         int64
}

func LoadConfig() Config {
	return Config{
		Addr:             env("NIKKI_ADDR", ":8080"),
		DatabaseURL:      env("NIKKI_DATABASE_URL", "postgres://nikki:nikki@localhost:5432/nikki?sslmode=disable"),
		LogFormat:        env("NIKKI_LOG_FORMAT", "json"),
		LogLevel:         env("NIKKI_LOG_LEVEL", "info"),
		UploadDir:        env("NIKKI_UPLOAD_DIR", "./uploads"),
		PublicUploadBase: env("NIKKI_PUBLIC_UPLOAD_BASE", "/uploads"),
		CookieSecure:     boolEnv("NIKKI_COOKIE_SECURE", false),
		CORSAllowedOrigins: listEnv("NIKKI_CORS_ALLOWED_ORIGINS", []string{
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			"http://localhost:8089",
			"http://127.0.0.1:8089",
		}),
		AllowAdditionalSignups:       boolEnv("NIKKI_SIGNUP_ENABLED", false),
		AllowFirstUserSetup:          boolEnv("NIKKI_FIRST_USER_SETUP_ENABLED", false),
		FirstUserBootstrapToken:      env("NIKKI_FIRST_USER_BOOTSTRAP_TOKEN", ""),
		AuthRateLimitIPAttempts:      intEnv("NIKKI_AUTH_RATE_LIMIT_IP_ATTEMPTS", intEnv("NIKKI_AUTH_RATE_LIMIT_MAX", 10)),
		AuthRateLimitAccountAttempts: intEnv("NIKKI_AUTH_RATE_LIMIT_ACCOUNT_ATTEMPTS", 5),
		AuthRateLimitWindow:          durationEnv("NIKKI_AUTH_RATE_LIMIT_WINDOW", 5*time.Minute),
		AuthRateLimitLockout:         durationEnv("NIKKI_AUTH_RATE_LIMIT_LOCKOUT", 15*time.Minute),
		AuthRateLimitMaxEntries:      intEnv("NIKKI_AUTH_RATE_LIMIT_MAX_ENTRIES", 2048),
		IPExtractorMode:              env("NIKKI_IP_EXTRACTOR_MODE", "direct"),
		TrustedProxyCIDRs:            listEnv("NIKKI_TRUSTED_PROXY_CIDRS", nil),
		StripImageMetadata:           boolEnv("NIKKI_STRIP_IMAGE_METADATA", false),
		ImageUserQuotaBytes:          nonNegativeInt64Env("NIKKI_IMAGE_USER_QUOTA_BYTES", images.DefaultUserQuotaBytes),
		ImageUserQuotaCount:          nonNegativeIntEnv("NIKKI_IMAGE_USER_QUOTA_COUNT", images.DefaultUserQuotaCount),
		ImageTotalQuotaBytes:         nonNegativeInt64Env("NIKKI_IMAGE_TOTAL_QUOTA_BYTES", 0),
	}
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return fallback
}

func boolEnv(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func intEnv(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func nonNegativeIntEnv(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	// Quota settings are safety limits. Reject malformed explicit values instead
	// of silently falling back to a weaker default.
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		panic("invalid " + key + ": must be a non-negative integer")
	}
	if parsed < 0 {
		panic("invalid " + key + ": must not be negative")
	}
	return parsed
}

func nonNegativeInt64Env(key string, fallback int64) int64 {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	// Quota settings are safety limits. Reject malformed explicit values instead
	// of silently falling back to a weaker default.
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		panic("invalid " + key + ": must be a non-negative integer")
	}
	if parsed < 0 {
		panic("invalid " + key + ": must not be negative")
	}
	return parsed
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func listEnv(key string, fallback []string) []string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	items := []string{}
	for _, part := range strings.Split(value, ",") {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}
