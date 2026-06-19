package app

import (
	"reflect"
	"testing"
	"time"
)

func TestLoadConfigSecurityOptions(t *testing.T) {
	t.Setenv("NIKKI_COOKIE_SECURE", "true")
	t.Setenv("NIKKI_CORS_ALLOWED_ORIGINS", "https://diary.example.com, https://second.example.com ")
	t.Setenv("NIKKI_SIGNUP_ENABLED", "true")
	t.Setenv("NIKKI_FIRST_USER_SETUP_ENABLED", "true")
	t.Setenv("NIKKI_FIRST_USER_BOOTSTRAP_TOKEN", "bootstrap-token")
	t.Setenv("NIKKI_AUTH_RATE_LIMIT_IP_ATTEMPTS", "7")
	t.Setenv("NIKKI_AUTH_RATE_LIMIT_ACCOUNT_ATTEMPTS", "4")
	t.Setenv("NIKKI_AUTH_RATE_LIMIT_WINDOW", "2m")
	t.Setenv("NIKKI_AUTH_RATE_LIMIT_LOCKOUT", "10m")
	t.Setenv("NIKKI_AUDIT_RETENTION_DAYS", "365")
	t.Setenv("NIKKI_IP_EXTRACTOR_MODE", "x-real-ip")
	t.Setenv("NIKKI_TRUSTED_PROXY_CIDRS", "127.0.0.1/32")
	t.Setenv("NIKKI_STRIP_IMAGE_METADATA", "true")
	t.Setenv("NIKKI_IMAGE_USER_QUOTA_BYTES", "2048")
	t.Setenv("NIKKI_IMAGE_USER_QUOTA_COUNT", "12")
	t.Setenv("NIKKI_IMAGE_TOTAL_QUOTA_BYTES", "4096")

	cfg := LoadConfig()
	if !cfg.CookieSecure {
		t.Fatal("CookieSecure = false, want true")
	}
	wantOrigins := []string{"https://diary.example.com", "https://second.example.com"}
	if !reflect.DeepEqual(cfg.CORSAllowedOrigins, wantOrigins) {
		t.Fatalf("CORSAllowedOrigins = %#v, want %#v", cfg.CORSAllowedOrigins, wantOrigins)
	}
	if !cfg.AllowAdditionalSignups {
		t.Fatal("AllowAdditionalSignups = false, want true")
	}
	if !cfg.AllowFirstUserSetup {
		t.Fatal("AllowFirstUserSetup = false, want true")
	}
	if cfg.FirstUserBootstrapToken != "bootstrap-token" {
		t.Fatalf("FirstUserBootstrapToken = %q, want bootstrap-token", cfg.FirstUserBootstrapToken)
	}
	if cfg.AuthRateLimitIPAttempts != 7 {
		t.Fatalf("AuthRateLimitIPAttempts = %d, want 7", cfg.AuthRateLimitIPAttempts)
	}
	if cfg.AuthRateLimitAccountAttempts != 4 {
		t.Fatalf("AuthRateLimitAccountAttempts = %d, want 4", cfg.AuthRateLimitAccountAttempts)
	}
	if cfg.AuthRateLimitWindow != 2*time.Minute {
		t.Fatalf("AuthRateLimitWindow = %s, want 2m", cfg.AuthRateLimitWindow)
	}
	if cfg.AuthRateLimitLockout != 10*time.Minute {
		t.Fatalf("AuthRateLimitLockout = %s, want 10m", cfg.AuthRateLimitLockout)
	}
	if cfg.AuditRetentionDays != 365 {
		t.Fatalf("AuditRetentionDays = %d, want 365", cfg.AuditRetentionDays)
	}
	if cfg.IPExtractorMode != "x-real-ip" {
		t.Fatalf("IPExtractorMode = %q, want x-real-ip", cfg.IPExtractorMode)
	}
	if !reflect.DeepEqual(cfg.TrustedProxyCIDRs, []string{"127.0.0.1/32"}) {
		t.Fatalf("TrustedProxyCIDRs = %#v", cfg.TrustedProxyCIDRs)
	}
	if !cfg.StripImageMetadata {
		t.Fatal("StripImageMetadata = false, want true")
	}
	if cfg.ImageUserQuotaBytes != 2048 {
		t.Fatalf("ImageUserQuotaBytes = %d, want 2048", cfg.ImageUserQuotaBytes)
	}
	if cfg.ImageUserQuotaCount != 12 {
		t.Fatalf("ImageUserQuotaCount = %d, want 12", cfg.ImageUserQuotaCount)
	}
	if cfg.ImageTotalQuotaBytes != 4096 {
		t.Fatalf("ImageTotalQuotaBytes = %d, want 4096", cfg.ImageTotalQuotaBytes)
	}
}

func TestLoadConfigCanDisableCORSMiddleware(t *testing.T) {
	t.Setenv("NIKKI_CORS_ALLOWED_ORIGINS", "")

	cfg := LoadConfig()
	if len(cfg.CORSAllowedOrigins) != 0 {
		t.Fatalf("CORSAllowedOrigins = %#v, want empty", cfg.CORSAllowedOrigins)
	}
}

func TestLoadConfigImageQuotaDefaults(t *testing.T) {
	t.Setenv("NIKKI_IMAGE_USER_QUOTA_BYTES", "")
	t.Setenv("NIKKI_IMAGE_USER_QUOTA_COUNT", "")
	t.Setenv("NIKKI_IMAGE_TOTAL_QUOTA_BYTES", "")

	cfg := LoadConfig()
	if cfg.ImageUserQuotaBytes != 1<<30 {
		t.Fatalf("ImageUserQuotaBytes = %d, want 1GiB", cfg.ImageUserQuotaBytes)
	}
	if cfg.ImageUserQuotaCount != 1000 {
		t.Fatalf("ImageUserQuotaCount = %d, want 1000", cfg.ImageUserQuotaCount)
	}
	if cfg.ImageTotalQuotaBytes != 0 {
		t.Fatalf("ImageTotalQuotaBytes = %d, want disabled", cfg.ImageTotalQuotaBytes)
	}
}

func TestLoadConfigRejectsInvalidImageQuota(t *testing.T) {
	t.Setenv("NIKKI_IMAGE_USER_QUOTA_BYTES", "-1")

	defer func() {
		if recover() == nil {
			t.Fatal("LoadConfig did not panic")
		}
	}()
	_ = LoadConfig()
}

func TestLoadConfigRejectsNonNumericImageQuota(t *testing.T) {
	t.Setenv("NIKKI_IMAGE_USER_QUOTA_COUNT", "many")

	defer func() {
		if recover() == nil {
			t.Fatal("LoadConfig did not panic")
		}
	}()
	_ = LoadConfig()
}

func TestLoadConfigRejectsInvalidAuditRetention(t *testing.T) {
	t.Setenv("NIKKI_AUDIT_RETENTION_DAYS", "0")

	defer func() {
		if recover() == nil {
			t.Fatal("LoadConfig did not panic")
		}
	}()
	_ = LoadConfig()
}
