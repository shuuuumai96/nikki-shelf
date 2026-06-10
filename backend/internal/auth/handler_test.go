package auth

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/httpx"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
)

func TestRequireWithoutSessionCookieReturnsUnauthorized(t *testing.T) {
	server := echo.New()
	server.GET("/protected", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, Require(nil))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	server.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != ErrUnauthorized.Error() {
		t.Fatalf("error = %q, want %q", body["error"], ErrUnauthorized.Error())
	}
}

func TestSessionCookieSecureFlag(t *testing.T) {
	session := SessionResult{
		Token:     "token-value",
		ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
	}

	cookie := sessionCookie(session, true)
	if !cookie.Secure {
		t.Fatal("Secure = false, want true")
	}
	if !cookie.HttpOnly {
		t.Fatal("HttpOnly = false, want true")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v, want %v", cookie.SameSite, http.SameSiteLaxMode)
	}

	localCookie := sessionCookie(session, false)
	if localCookie.Secure {
		t.Fatal("local Secure = true, want false")
	}
}

func TestClearSessionCookiePreservesSecureFlag(t *testing.T) {
	cookie := clearCookie(true)
	if !cookie.Secure {
		t.Fatal("Secure = false, want true")
	}
	if cookie.MaxAge != -1 {
		t.Fatalf("MaxAge = %d, want -1", cookie.MaxAge)
	}
}

func TestCSRFSafeMethodsDoNotRequireToken(t *testing.T) {
	server := echo.New()
	server.GET("/api/entries", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, CSRF(nil))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/entries", nil)
	server.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestCSRFUnsafeMethodsRequireToken(t *testing.T) {
	server := echo.New()
	server.POST("/api/entries", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, csrfWithValidator(func(_ context.Context, _ string, _ string) bool {
		return true
	}))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/entries", nil)
	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestCSRFUnsafeMethodsRejectInvalidToken(t *testing.T) {
	server := echo.New()
	server.PUT("/api/entries/1", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, csrfWithValidator(func(_ context.Context, sessionToken string, csrfToken string) bool {
		return sessionToken == "session-token" && csrfToken == "valid-csrf"
	}))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/entries/1", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-token"})
	request.Header.Set("X-CSRF-Token", "invalid-csrf")
	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestCSRFUnsafeMethodsAllowValidToken(t *testing.T) {
	server := echo.New()
	server.DELETE("/api/entries/1", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, csrfWithValidator(func(_ context.Context, sessionToken string, csrfToken string) bool {
		return sessionToken == "session-token" && csrfToken == "valid-csrf"
	}))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/entries/1", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-token"})
	request.Header.Set("X-CSRF-Token", "valid-csrf")
	server.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestLogoutRequiresCSRF(t *testing.T) {
	server := echo.New()
	server.POST("/api/auth/logout", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, csrfWithValidator(func(context.Context, string, string) bool { return false }))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-token"})
	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestLogoutAllowsValidCSRF(t *testing.T) {
	server := echo.New()
	server.POST("/api/auth/logout", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, csrfWithValidator(func(_ context.Context, sessionToken string, csrfToken string) bool {
		return sessionToken == "session-token" && csrfToken == "valid-csrf"
	}))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-token"})
	request.Header.Set("X-CSRF-Token", "valid-csrf")
	server.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestSignupClosedAfterFirstUserUnlessEnabled(t *testing.T) {
	tests := []struct {
		name                   string
		userCount              int
		allowAdditionalSignups bool
		wantClosed             bool
	}{
		{name: "first user allowed", userCount: 0, allowAdditionalSignups: false, wantClosed: false},
		{name: "additional signup closed by default", userCount: 1, allowAdditionalSignups: false, wantClosed: true},
		{name: "additional signup can be enabled", userCount: 1, allowAdditionalSignups: true, wantClosed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := signupClosed(tt.userCount, tt.allowAdditionalSignups); got != tt.wantClosed {
				t.Fatalf("signupClosed() = %v, want %v", got, tt.wantClosed)
			}
		})
	}
}

func TestSignupClosedStatus(t *testing.T) {
	if got := StatusFor(ErrSignupClosed); got != http.StatusForbidden {
		t.Fatalf("StatusFor(ErrSignupClosed) = %d, want %d", got, http.StatusForbidden)
	}
	if got := KindFor(ErrSignupClosed); got != "auth.signup_closed" {
		t.Fatalf("KindFor(ErrSignupClosed) = %q, want auth.signup_closed", got)
	}
}

func TestFirstUserSignupRequiresConfiguredBootstrapToken(t *testing.T) {
	tests := []struct {
		name            string
		configuredToken string
		headerToken     string
	}{
		{name: "no configured token"},
		{name: "missing header", configuredToken: "correct-token"},
		{name: "wrong header", configuredToken: "correct-token", headerToken: "wrong-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performAuthRequest(t, NewService(newFakeAuthRepo(0), ServiceConfig{
				FirstUserBootstrapToken: tt.configuredToken,
			}), "/api/auth/signup", tt.headerToken)

			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
			}
		})
	}
}

func TestFirstUserSignupStillRequiresBootstrapTokenWhenBrowserSetupEnabled(t *testing.T) {
	response := performAuthRequest(t, NewService(newFakeAuthRepo(0), ServiceConfig{
		AllowFirstUserSetup: true,
	}), "/api/auth/signup", "")

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestFirstUserSignupAllowsCorrectBootstrapTokenAndSetsCookie(t *testing.T) {
	response := performAuthRequest(t, NewService(newFakeAuthRepo(0), ServiceConfig{
		FirstUserBootstrapToken: "correct-token",
	}), "/api/auth/signup", "correct-token")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if cookie := response.Result().Cookies()[0]; cookie.Name != SessionCookieName || cookie.Value == "" {
		t.Fatalf("session cookie = %#v, want %s with value", cookie, SessionCookieName)
	}
}

func TestFirstUserSetupDoesNotAllowAdditionalSignup(t *testing.T) {
	response := performAuthRequest(t, NewService(newFakeAuthRepo(1), ServiceConfig{
		AllowFirstUserSetup: true,
	}), "/api/auth/signup", "")

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestBootstrapTokenDoesNotBypassClosedAdditionalSignup(t *testing.T) {
	response := performAuthRequest(t, NewService(newFakeAuthRepo(1), ServiceConfig{
		AllowAdditionalSignups:  false,
		FirstUserBootstrapToken: "correct-token",
	}), "/api/auth/signup", "correct-token")

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestAdditionalSignupWhenDisabledIsRejected(t *testing.T) {
	response := performAuthRequest(t, NewService(newFakeAuthRepo(1), ServiceConfig{
		AllowAdditionalSignups: false,
	}), "/api/auth/signup", "")

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestAdditionalSignupWhenEnabledDoesNotRequireBootstrapToken(t *testing.T) {
	response := performAuthRequest(t, NewService(newFakeAuthRepo(1), ServiceConfig{
		AllowAdditionalSignups: true,
	}), "/api/auth/signup", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
}

func TestAuthConfigReturnsSignupMode(t *testing.T) {
	tests := []struct {
		name                   string
		userCount              int
		allowFirstUserSetup    bool
		allowAdditionalSignups bool
		wantMode               string
		wantAvailable          bool
	}{
		{
			name:                "closed first user even with browser setup enabled",
			userCount:           0,
			allowFirstUserSetup: true,
			wantMode:            "closed",
		},
		{
			name:      "closed first user without setup",
			userCount: 0,
			wantMode:  "closed",
		},
		{
			name:                   "open additional signup",
			userCount:              1,
			allowAdditionalSignups: true,
			wantMode:               "open",
			wantAvailable:          true,
		},
		{
			name:      "closed additional signup",
			userCount: 1,
			wantMode:  "closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performAuthConfigRequest(NewService(newFakeAuthRepo(tt.userCount), ServiceConfig{
				AllowFirstUserSetup:    tt.allowFirstUserSetup,
				AllowAdditionalSignups: tt.allowAdditionalSignups,
			}))

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
			}

			var body ConfigResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.SignupMode != tt.wantMode {
				t.Fatalf("SignupMode = %q, want %q", body.SignupMode, tt.wantMode)
			}
			if body.SignupAvailable != tt.wantAvailable {
				t.Fatalf("SignupAvailable = %v, want %v", body.SignupAvailable, tt.wantAvailable)
			}
		})
	}
}

func TestSetupStatusEmptyUsersNeedsSetup(t *testing.T) {
	response := performSetupStatusRequest(NewService(newFakeAuthRepo(0)))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	var body SetupStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.NeedsSetup || body.SetupLocked || !body.CanCreateOwner || !body.RequiresSetupToken {
		t.Fatalf("setup status = %#v, want empty setup available with token required", body)
	}
}

func TestSetupStatusExistingUsersDoesNotNeedSetup(t *testing.T) {
	response := performSetupStatusRequest(NewService(newFakeAuthRepo(1)))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	var body SetupStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.NeedsSetup || !body.SetupLocked || body.CanCreateOwner || !body.RequiresSetupToken {
		t.Fatalf("setup status = %#v, want initialized setup locked", body)
	}
}

func TestSetupStatusLockedEmptyDatabaseCannotCreateOwner(t *testing.T) {
	repo := newFakeAuthRepo(0)
	repo.setupLocked = true
	response := performSetupStatusRequest(NewService(repo))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	var body SetupStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.NeedsSetup || !body.SetupLocked || body.CanCreateOwner {
		t.Fatalf("setup status = %#v, want locked empty database to fail safe", body)
	}
}

func TestSetupOwnerCreatesOwnerWithCorrectToken(t *testing.T) {
	repo := newFakeAuthRepo(0)
	response := performSetupOwnerRequest(NewService(repo, ServiceConfig{
		AllowAdditionalSignups:  false,
		FirstUserBootstrapToken: "correct-token",
	}), `{"setupToken":"correct-token","username":"owner","password":"long-password"}`)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if cookie := response.Result().Cookies()[0]; cookie.Name != SessionCookieName || cookie.Value == "" {
		t.Fatalf("session cookie = %#v, want %s with value", cookie, SessionCookieName)
	}
	if strings.Contains(response.Body.String(), "correct-token") || strings.Contains(response.Body.String(), "long-password") {
		t.Fatalf("response leaked setup token or password: %s", response.Body.String())
	}

	var user User
	if err := json.Unmarshal(response.Body.Bytes(), &user); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if user.Username != "owner" || user.Role != RoleOwner {
		t.Fatalf("user = %#v, want owner role", user)
	}
	if row := repo.user("owner"); row.Role != RoleOwner {
		t.Fatalf("stored role = %q, want %q", row.Role, RoleOwner)
	}
	if !repo.locked() {
		t.Fatal("setup lock = false, want true")
	}
}

func TestSetupOwnerRejectsWrongToken(t *testing.T) {
	response := performSetupOwnerRequest(NewService(newFakeAuthRepo(0), ServiceConfig{
		FirstUserBootstrapToken: "correct-token",
	}), `{"setupToken":"wrong-token","username":"owner","password":"long-password"}`)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
	assertResponseKind(t, response, "setup.invalid_token")
}

func TestSetupOwnerRejectsMissingToken(t *testing.T) {
	response := performSetupOwnerRequest(NewService(newFakeAuthRepo(0), ServiceConfig{
		FirstUserBootstrapToken: "correct-token",
	}), `{"username":"owner","password":"long-password"}`)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
	assertResponseKind(t, response, "setup.invalid_token")
}

func TestSetupOwnerRejectsSecondCreation(t *testing.T) {
	service := NewService(newFakeAuthRepo(0), ServiceConfig{
		FirstUserBootstrapToken: "correct-token",
	})
	first := performSetupOwnerRequest(service, `{"setupToken":"correct-token","username":"owner","password":"long-password"}`)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d; body=%s", first.Code, http.StatusOK, first.Body.String())
	}

	second := performSetupOwnerRequest(service, `{"setupToken":"correct-token","username":"owner2","password":"long-password"}`)
	if second.Code != http.StatusConflict {
		t.Fatalf("second status = %d, want %d; body=%s", second.Code, http.StatusConflict, second.Body.String())
	}
	assertResponseKind(t, second, "setup.already_initialized")
}

func TestSetupOwnerRejectsInitializedInstanceBeforeInputValidation(t *testing.T) {
	response := performSetupOwnerRequest(NewService(newFakeAuthRepo(1), ServiceConfig{
		FirstUserBootstrapToken: "correct-token",
	}), `{"setupToken":"wrong-token","username":"x","password":"short"}`)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusConflict, response.Body.String())
	}
	assertResponseKind(t, response, "setup.already_initialized")
}

func TestSetupOwnerConcurrentRequestsCreateOnlyOneOwner(t *testing.T) {
	repo := newFakeAuthRepo(0)
	service := NewService(repo, ServiceConfig{FirstUserBootstrapToken: "correct-token"})
	results := make(chan error, 12)

	for i := 0; i < 12; i++ {
		i := i
		go func() {
			_, err := service.CreateFirstOwner(context.Background(), SetupOwnerInput{
				SetupToken: "correct-token",
				Username:   fmt.Sprintf("owner%d", i),
				Password:   "long-password",
			})
			results <- err
		}()
	}

	successes := 0
	conflicts := 0
	for i := 0; i < 12; i++ {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrSetupLocked):
			conflicts++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if successes != 1 || conflicts != 11 {
		t.Fatalf("successes=%d conflicts=%d, want 1 success and 11 conflicts", successes, conflicts)
	}
	if repo.countUsers() != 1 {
		t.Fatalf("user count = %d, want 1", repo.countUsers())
	}
	if repo.ownerCount() != 1 {
		t.Fatalf("owner count = %d, want 1", repo.ownerCount())
	}
}

func TestSetupOwnerWorksWhenAdditionalSignupDisabled(t *testing.T) {
	response := performSetupOwnerRequest(NewService(newFakeAuthRepo(0), ServiceConfig{
		AllowAdditionalSignups:  false,
		FirstUserBootstrapToken: "correct-token",
	}), `{"setupToken":"correct-token","username":"owner","password":"long-password"}`)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
}

func TestSetupOwnerDoesNotLeakTokenOrPasswordInResponsesOrLogs(t *testing.T) {
	const setupToken = "correct-token"
	const password = "long-password"

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	server := echo.New()
	server.Use(logx.RequestIDMiddleware())
	server.Use(logx.Middleware(logger))
	NewHandler(NewService(newFakeAuthRepo(0), ServiceConfig{
		FirstUserBootstrapToken: setupToken,
	})).Register(server.Group("/api"))

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/setup/owner",
		strings.NewReader(`{"setupToken":"wrong-token","username":"owner","password":"`+password+`"}`),
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
	for _, sensitive := range []string{setupToken, "wrong-token", password} {
		if strings.Contains(response.Body.String(), sensitive) {
			t.Fatalf("response leaked sensitive value %q: %s", sensitive, response.Body.String())
		}
		if strings.Contains(logs.String(), sensitive) {
			t.Fatalf("logs leaked sensitive value %q: %s", sensitive, logs.String())
		}
	}
	if !strings.Contains(logs.String(), "setup.owner_create_failed") || !strings.Contains(logs.String(), "invalid_token") {
		t.Fatalf("logs = %s, want setup failure event with reason", logs.String())
	}
}

func TestSetupRestoreVerifyAllowsValidBackupWhenEmpty(t *testing.T) {
	response := performSetupRestoreRequest(t, NewService(newFakeAuthRepo(0), ServiceConfig{
		FirstUserBootstrapToken: "correct-token",
	}), "/api/setup/restore/verify", "correct-token", makeOperationalBackup(t, backupOptions{
		entryCount: 123,
		imageCount: 45,
	}), "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	var body SetupRestoreVerifyResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Valid || body.EntryCount != 123 || body.ImageCount != 45 || body.NikkiVersion != "0.1.0" {
		t.Fatalf("verify response = %#v, want manifest details", body)
	}
}

func TestSetupRestoreVerifyRejectsExistingUsers(t *testing.T) {
	response := performSetupRestoreRequest(t, NewService(newFakeAuthRepo(1), ServiceConfig{
		FirstUserBootstrapToken: "correct-token",
	}), "/api/setup/restore/verify", "correct-token", makeOperationalBackup(t, backupOptions{}), "")

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusConflict, response.Body.String())
	}
	assertResponseKind(t, response, "setup.already_initialized")
}

func TestSetupRestoreVerifyRejectsMissingOrWrongToken(t *testing.T) {
	for _, token := range []string{"", "wrong-token"} {
		t.Run("token="+token, func(t *testing.T) {
			response := performSetupRestoreRequest(t, NewService(newFakeAuthRepo(0), ServiceConfig{
				FirstUserBootstrapToken: "correct-token",
			}), "/api/setup/restore/verify", token, makeOperationalBackup(t, backupOptions{}), "")

			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
			}
			assertResponseKind(t, response, "setup.invalid_token")
		})
	}
}

func TestSetupRestoreVerifyRejectsInvalidArchives(t *testing.T) {
	tests := []struct {
		name    string
		archive []byte
	}{
		{name: "invalid tar gz", archive: []byte("not a tar.gz")},
		{name: "manifest missing", archive: makeOperationalBackup(t, backupOptions{omitManifest: true})},
		{name: "db dump missing", archive: makeOperationalBackup(t, backupOptions{omitDBDump: true})},
		{name: "uploads missing", archive: makeOperationalBackup(t, backupOptions{omitUploads: true})},
		{name: "uploads path traversal", archive: makeOperationalBackup(t, backupOptions{uploadName: "../evil.jpg"})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performSetupRestoreRequest(t, NewService(newFakeAuthRepo(0), ServiceConfig{
				FirstUserBootstrapToken: "correct-token",
			}), "/api/setup/restore/verify", "correct-token", tt.archive, "")

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			assertResponseKind(t, response, "setup.invalid_backup")
		})
	}
}

func TestSetupRestoreSucceedsAndLocksSetup(t *testing.T) {
	repo := newFakeAuthRepo(0)
	service := NewService(repo, ServiceConfig{
		FirstUserBootstrapToken: "correct-token",
		UploadDir:               t.TempDir(),
	})
	service.restorePostgres = func(context.Context, string) error {
		repo.restoreFixture(123, 45)
		return nil
	}

	response := performSetupRestoreRequest(t, service, "/api/setup/restore", "correct-token", makeOperationalBackup(t, backupOptions{
		entryCount: 123,
		imageCount: 45,
	}), "true")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	var body SetupRestoreResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Restored || body.EntryCount != 123 || body.ImageCount != 45 {
		t.Fatalf("restore response = %#v, want restored counts", body)
	}
	if repo.countUsers() != 1 || !repo.locked() {
		t.Fatalf("users=%d locked=%v, want restored user and setup lock", repo.countUsers(), repo.locked())
	}

	status := performSetupStatusRequest(service)
	var setup SetupStatusResponse
	if err := json.Unmarshal(status.Body.Bytes(), &setup); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if setup.NeedsSetup || !setup.SetupLocked {
		t.Fatalf("setup status = %#v, want initialized", setup)
	}
}

func TestSetupRestoreRejectsMissingConfirmation(t *testing.T) {
	repo := newFakeAuthRepo(0)
	service := NewService(repo, ServiceConfig{
		FirstUserBootstrapToken: "correct-token",
		UploadDir:               t.TempDir(),
	})

	response := performSetupRestoreRequest(t, service, "/api/setup/restore", "correct-token", makeOperationalBackup(t, backupOptions{}), "")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	assertResponseKind(t, response, "setup.invalid_input")
	if repo.locked() {
		t.Fatal("setup lock = true, want false after rejected restore")
	}
}

func TestSetupRestoreRejectsSecondAttempt(t *testing.T) {
	repo := newFakeAuthRepo(0)
	service := NewService(repo, ServiceConfig{
		FirstUserBootstrapToken: "correct-token",
		UploadDir:               t.TempDir(),
	})
	service.restorePostgres = func(context.Context, string) error {
		repo.restoreFixture(1, 1)
		return nil
	}

	first := performSetupRestoreRequest(t, service, "/api/setup/restore", "correct-token", makeOperationalBackup(t, backupOptions{
		entryCount: 1,
		imageCount: 1,
	}), "true")
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d; body=%s", first.Code, http.StatusOK, first.Body.String())
	}

	second := performSetupRestoreRequest(t, service, "/api/setup/restore", "correct-token", makeOperationalBackup(t, backupOptions{
		entryCount: 1,
		imageCount: 1,
	}), "true")
	if second.Code != http.StatusConflict {
		t.Fatalf("second status = %d, want %d; body=%s", second.Code, http.StatusConflict, second.Body.String())
	}
	assertResponseKind(t, second, "setup.already_initialized")
}

func TestSetupRestoreDoesNotLeakSecretsInResponsesOrLogs(t *testing.T) {
	const setupToken = "correct-token"
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	server := echo.New()
	server.Use(logx.RequestIDMiddleware())
	server.Use(logx.Middleware(logger))
	NewHandler(NewService(newFakeAuthRepo(0), ServiceConfig{
		FirstUserBootstrapToken: setupToken,
		DatabaseURL:             "postgres://nikki:secret-db-password@example.invalid/nikki",
		UploadDir:               t.TempDir(),
	})).Register(server.Group("/api"))

	body, contentType := multipartSetupRestoreBody(t, "wrong-token", makeOperationalBackup(t, backupOptions{}), "")
	request := httptest.NewRequest(http.MethodPost, "/api/setup/restore/verify", body)
	request.Header.Set(echo.HeaderContentType, contentType)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
	for _, sensitive := range []string{setupToken, "wrong-token", "secret-db-password"} {
		if strings.Contains(response.Body.String(), sensitive) {
			t.Fatalf("response leaked sensitive value %q: %s", sensitive, response.Body.String())
		}
		if strings.Contains(logs.String(), sensitive) {
			t.Fatalf("logs leaked sensitive value %q: %s", sensitive, logs.String())
		}
	}
}

func TestSetupRestoreInProgressBlocksProtectedRequests(t *testing.T) {
	repo := newFakeAuthRepo(1)
	repo.restoreInProgress = true
	repo.sessions[hashToken("session-token")] = SessionRow{
		User: UserRow{ID: 1, Username: "owner", PasswordHash: "hash", Role: RoleOwner, CreatedAt: time.Now().Format(time.RFC3339)},
	}
	server := echo.New()
	server.GET("/api/entries", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, Require(NewService(repo)))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/entries", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-token"})
	server.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusServiceUnavailable, response.Body.String())
	}
	assertResponseKind(t, response, "setup.restore_in_progress")
}

func TestDeleteAccountAllowsLastOwnerAndUnlocksSetup(t *testing.T) {
	repo := newFakeAuthRepo(0)
	repo.setupLocked = true
	owner := repo.addUser(t, "owner", "password123", RoleOwner)
	repo.accountImageFilePaths = []string{"/uploads/one.jpg", "/uploads/two.jpg"}
	deleter := &fakeAccountFileDeleter{}

	result, err := NewService(repo, ServiceConfig{AccountFiles: deleter}).DeleteCurrentAccount(context.Background(), owner.ID, DeleteAccountInput{
		Username: "owner",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("DeleteCurrentAccount() error = %v", err)
	}

	if result.RemainingUsers != 0 || repo.countUsers() != 0 {
		t.Fatalf("remaining users = %d / %d, want 0", result.RemainingUsers, repo.countUsers())
	}
	if repo.locked() {
		t.Fatal("setupLocked = true, want false after deleting last user")
	}
	if len(deleter.deleted) != 2 || deleter.deleted[0] != "/uploads/one.jpg" || deleter.deleted[1] != "/uploads/two.jpg" {
		t.Fatalf("deleted files = %#v, want account image files", deleter.deleted)
	}
}

func TestDeleteAccountRejectsOwnerWhenOtherUsersRemain(t *testing.T) {
	repo := newFakeAuthRepo(0)
	owner := repo.addUser(t, "owner", "password123", RoleOwner)
	repo.addUser(t, "tester", "password123", RoleUser)
	repo.accountImageFilePaths = []string{"/uploads/owner.jpg"}
	deleter := &fakeAccountFileDeleter{}

	_, err := NewService(repo, ServiceConfig{AccountFiles: deleter}).DeleteCurrentAccount(context.Background(), owner.ID, DeleteAccountInput{
		Username: "owner",
		Password: "password123",
	})
	if !errors.Is(err, ErrOwnerAccountRequired) {
		t.Fatalf("DeleteCurrentAccount() error = %v, want %v", err, ErrOwnerAccountRequired)
	}
	if repo.countUsers() != 2 {
		t.Fatalf("user count = %d, want 2", repo.countUsers())
	}
	if len(deleter.deleted) != 0 {
		t.Fatalf("deleted files = %#v, want none", deleter.deleted)
	}
}

func TestDeleteAccountAllowsRegularUserWhenOwnerRemains(t *testing.T) {
	repo := newFakeAuthRepo(0)
	repo.setupLocked = true
	repo.addUser(t, "owner", "password123", RoleOwner)
	user := repo.addUser(t, "tester", "password123", RoleUser)

	result, err := NewService(repo).DeleteCurrentAccount(context.Background(), user.ID, DeleteAccountInput{
		Username: "tester",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("DeleteCurrentAccount() error = %v", err)
	}
	if result.RemainingUsers != 1 || repo.countUsers() != 1 {
		t.Fatalf("remaining users = %d / %d, want 1", result.RemainingUsers, repo.countUsers())
	}
	if !repo.locked() {
		t.Fatal("setupLocked = false, want existing setup lock preserved")
	}
	if _, ok := repo.users["tester"]; ok {
		t.Fatal("tester still exists after deletion")
	}
}

func TestDeleteAccountRejectsWrongConfirmation(t *testing.T) {
	repo := newFakeAuthRepo(0)
	user := repo.addUser(t, "tester", "password123", RoleUser)

	_, err := NewService(repo).DeleteCurrentAccount(context.Background(), user.ID, DeleteAccountInput{
		Username: "wrong-user",
		Password: "password123",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("DeleteCurrentAccount() error = %v, want %v", err, ErrInvalidCredentials)
	}
	if repo.countUsers() != 1 {
		t.Fatalf("user count = %d, want 1", repo.countUsers())
	}
}

func TestDeleteAccountEndpointClearsSessionCookie(t *testing.T) {
	repo := newFakeAuthRepo(0)
	user := repo.addUser(t, "tester", "password123", RoleUser)
	repo.sessions[hashToken("session-token")] = SessionRow{
		User:     user,
		CSRFHash: hashToken("csrf-token"),
	}

	response := performDeleteAccountRequest(NewService(repo), `{"username":"tester","password":"password123"}`)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNoContent, response.Body.String())
	}

	cookies := response.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != SessionCookieName || cookies[0].MaxAge != -1 {
		t.Fatalf("cookies = %#v, want cleared session cookie", cookies)
	}
}

func TestLoginDoesNotRequireBootstrapToken(t *testing.T) {
	repo := newFakeAuthRepo(1)
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	repo.users["tester"] = UserRow{ID: 1, Username: "tester", PasswordHash: string(hash), CreatedAt: time.Now().Format(time.RFC3339)}

	response := performAuthRequest(t, NewService(repo, ServiceConfig{
		FirstUserBootstrapToken: "correct-token",
	}), "/api/auth/login", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
}

func TestLoginRejectsOversizedJSON(t *testing.T) {
	response := performAuthRequestWithBody(
		NewService(newFakeAuthRepo(1)),
		"/api/auth/login",
		strings.NewReader(`{"username":"tester","password":"`+strings.Repeat("x", int(httpx.AuthJSONLimitBytes)+1)+`"}`),
		"",
	)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusRequestEntityTooLarge, response.Body.String())
	}
}

func TestSignupRejectsOversizedJSON(t *testing.T) {
	response := performAuthRequestWithBody(
		NewService(newFakeAuthRepo(0), ServiceConfig{FirstUserBootstrapToken: "correct-token"}),
		"/api/auth/signup",
		strings.NewReader(`{"username":"tester","password":"`+strings.Repeat("x", int(httpx.AuthJSONLimitBytes)+1)+`"}`),
		"correct-token",
	)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusRequestEntityTooLarge, response.Body.String())
	}
}

func TestRateLimiterBlocksAfterLimitAndResets(t *testing.T) {
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	limiter := NewRateLimiter(RateLimiterConfig{
		IPAttempts:      2,
		AccountAttempts: 2,
		Window:          time.Minute,
		Lockout:         time.Minute,
		Extractor:       NewClientIPExtractor("direct", nil),
	})
	limiter.now = func() time.Time {
		return now
	}

	server := echo.New()
	server.POST("/api/auth/login", func(c echo.Context) error {
		if !limiter.Allow(c, "tester") {
			return rateLimitError(c)
		}
		limiter.RecordFailure(c, "tester")
		return c.NoContent(http.StatusUnauthorized)
	})

	for i := 0; i < 2; i++ {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		request.RemoteAddr = "203.0.113.10:1234"
		server.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d", i+1, response.Code, http.StatusUnauthorized)
		}
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	request.RemoteAddr = "203.0.113.10:1234"
	server.ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("limited status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}

	now = now.Add(time.Minute)
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	request.RemoteAddr = "203.0.113.10:1234"
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("reset status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestRateLimiterBoundsMapSize(t *testing.T) {
	limiter := NewRateLimiter(RateLimiterConfig{
		IPAttempts:      100,
		AccountAttempts: 100,
		Window:          time.Minute,
		MaxEntries:      3,
		Extractor:       NewClientIPExtractor("direct", nil),
	})
	server := echo.New()
	for i := 0; i < 10; i++ {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		request.RemoteAddr = fmt.Sprintf("203.0.113.%d:1234", i+1)
		c := server.NewContext(request, httptest.NewRecorder())
		limiter.RecordFailure(c, "user")
	}
	if len(limiter.ipAttempts) > 3 {
		t.Fatalf("ipAttempts size = %d, want <= 3", len(limiter.ipAttempts))
	}
}

func TestRateLimiterUsesTrustedExtractor(t *testing.T) {
	limiter := NewRateLimiter(RateLimiterConfig{
		IPAttempts:      2,
		AccountAttempts: 10,
		Window:          time.Minute,
		Extractor:       NewClientIPExtractor("x-real-ip", []string{"198.51.100.0/24"}),
	})
	server := echo.New()

	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		request.RemoteAddr = "198.51.100.10:443"
		request.Header.Set("X-Real-IP", "203.0.113.50")
		c := server.NewContext(request, httptest.NewRecorder())
		if !limiter.Allow(c, "user") {
			t.Fatalf("attempt %d unexpectedly limited", i+1)
		}
		limiter.RecordFailure(c, "user")
	}

	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("X-Real-IP", "203.0.113.50")
	c := server.NewContext(request, httptest.NewRecorder())
	if limiter.Allow(c, "user") {
		t.Fatal("Allow returned true after trusted extracted IP reached limit")
	}
}

func TestClientIPExtractorTrustsConfiguredProxyOnly(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("X-Forwarded-For", "203.0.113.40, 198.51.100.10")
	if got := NewClientIPExtractor("x-forwarded-for", nil).ClientIP(request); got != "198.51.100.10" {
		t.Fatalf("untrusted proxy ip = %q, want direct", got)
	}
	if got := NewClientIPExtractor("x-forwarded-for", []string{"198.51.100.0/24"}).ClientIP(request); got != "203.0.113.40" {
		t.Fatalf("trusted proxy ip = %q, want forwarded client", got)
	}
}

func TestClientIPExtractorXRealIPTrustsConfiguredProxyOnly(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("X-Real-IP", "203.0.113.41")
	if got := NewClientIPExtractor("x-real-ip", []string{"192.0.2.0/24"}).ClientIP(request); got != "198.51.100.10" {
		t.Fatalf("untrusted x-real-ip = %q, want direct", got)
	}
	if got := NewClientIPExtractor("x-real-ip", []string{"198.51.100.0/24"}).ClientIP(request); got != "203.0.113.41" {
		t.Fatalf("trusted x-real-ip = %q, want header client", got)
	}
}

func TestClientIPExtractorMalformedHeadersFallBackSafely(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("X-Forwarded-For", "not-an-ip")
	if got := NewClientIPExtractor("x-forwarded-for", []string{"198.51.100.0/24"}).ClientIP(request); got != "198.51.100.10" {
		t.Fatalf("malformed x-forwarded-for = %q, want direct", got)
	}
}

func performAuthRequest(t *testing.T, service *Service, path string, bootstrapToken string) *httptest.ResponseRecorder {
	t.Helper()

	return performAuthRequestWithBody(service, path, strings.NewReader(`{"username":"tester","password":"password123"}`), bootstrapToken)
}

func performAuthRequestWithBody(service *Service, path string, body *strings.Reader, bootstrapToken string) *httptest.ResponseRecorder {
	server := echo.New()
	api := server.Group("/api")
	NewHandler(service).Register(api)

	request := httptest.NewRequest(http.MethodPost, path, body)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if bootstrapToken != "" {
		request.Header.Set(BootstrapTokenHeader, bootstrapToken)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func performAuthConfigRequest(service *Service) *httptest.ResponseRecorder {
	server := echo.New()
	api := server.Group("/api")
	NewHandler(service).Register(api)

	request := httptest.NewRequest(http.MethodGet, "/api/auth/config", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func performSetupStatusRequest(service *Service) *httptest.ResponseRecorder {
	server := echo.New()
	api := server.Group("/api")
	NewHandler(service).Register(api)

	request := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func performSetupOwnerRequest(service *Service, body string) *httptest.ResponseRecorder {
	server := echo.New()
	api := server.Group("/api")
	NewHandler(service).Register(api)

	request := httptest.NewRequest(http.MethodPost, "/api/setup/owner", strings.NewReader(body))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func performDeleteAccountRequest(service *Service, body string) *httptest.ResponseRecorder {
	server := echo.New()
	api := server.Group("/api")
	NewHandler(service).Register(api)

	request := httptest.NewRequest(http.MethodDelete, "/api/auth/me", strings.NewReader(body))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set("X-CSRF-Token", "csrf-token")
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-token"})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func performSetupRestoreRequest(t *testing.T, service *Service, path string, token string, archive []byte, confirmRestore string) *httptest.ResponseRecorder {
	t.Helper()

	server := echo.New()
	api := server.Group("/api")
	NewHandler(service).Register(api)

	body, contentType := multipartSetupRestoreBody(t, token, archive, confirmRestore)
	request := httptest.NewRequest(http.MethodPost, path, body)
	request.Header.Set(echo.HeaderContentType, contentType)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func multipartSetupRestoreBody(t *testing.T, token string, archive []byte, confirmRestore string) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if token != "" {
		if err := writer.WriteField("setupToken", token); err != nil {
			t.Fatalf("write token field: %v", err)
		}
	}
	if confirmRestore != "" {
		if err := writer.WriteField("confirmRestore", confirmRestore); err != nil {
			t.Fatalf("write confirm field: %v", err)
		}
	}
	file, err := writer.CreateFormFile("backupFile", "nikki-operational-backup.tar.gz")
	if err != nil {
		t.Fatalf("create file field: %v", err)
	}
	if _, err := file.Write(archive); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body, writer.FormDataContentType()
}

func assertResponseKind(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()

	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["kind"] != want {
		t.Fatalf("kind = %q, want %q; body=%s", body["kind"], want, response.Body.String())
	}
}

type backupOptions struct {
	omitManifest bool
	omitDBDump   bool
	omitUploads  bool
	uploadName   string
	entryCount   int
	imageCount   int
}

func makeOperationalBackup(t *testing.T, opts backupOptions) []byte {
	t.Helper()

	manifest := []byte(fmt.Sprintf(`{
  "backupCreatedAt": "2026-05-28T10:00:00Z",
  "nikkiVersion": "0.1.0",
  "schemaVersion": "1",
  "entryCount": %d,
  "imageCount": %d
}`, opts.entryCount, opts.imageCount))
	dump := []byte("custom dump placeholder")
	uploads := makeUploadsTar(t, opts.uploadName)

	files := map[string][]byte{}
	if !opts.omitManifest {
		files[operationalManifestPath] = manifest
	}
	if !opts.omitDBDump {
		files[operationalDumpPath] = dump
	}
	if !opts.omitUploads {
		files[operationalUploadsPath] = uploads
	}
	files[operationalSumsPath] = sha256Sums(files)

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, name := range []string{operationalManifestPath, operationalDumpPath, operationalUploadsPath, operationalSumsPath} {
		content, ok := files[name]
		if !ok {
			continue
		}
		if err := tarWriter.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o600,
			Size: int64(len(content)),
		}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tarWriter.Write(content); err != nil {
			t.Fatalf("write tar content: %v", err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buffer.Bytes()
}

func makeUploadsTar(t *testing.T, uploadName string) []byte {
	t.Helper()

	if uploadName == "" {
		uploadName = "images/photo.jpg"
	}
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	content := []byte("image")
	if err := writer.WriteHeader(&tar.Header{
		Name: uploadName,
		Mode: 0o600,
		Size: int64(len(content)),
	}); err != nil {
		t.Fatalf("write uploads tar header: %v", err)
	}
	if _, err := writer.Write(content); err != nil {
		t.Fatalf("write uploads tar content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close uploads tar: %v", err)
	}
	return buffer.Bytes()
}

func sha256Sums(files map[string][]byte) []byte {
	var builder strings.Builder
	for _, name := range []string{operationalManifestPath, operationalDumpPath, operationalUploadsPath} {
		content, ok := files[name]
		if !ok {
			continue
		}
		sum := sha256.Sum256(content)
		builder.WriteString(hex.EncodeToString(sum[:]))
		builder.WriteString("  ")
		builder.WriteString(name)
		builder.WriteString("\n")
	}
	return []byte(builder.String())
}

type fakeAuthRepo struct {
	mu                    sync.Mutex
	count                 int
	entryCount            int
	imageCount            int
	nextID                int64
	users                 map[string]UserRow
	sessions              map[string]SessionRow
	accountImageFilePaths []string
	claimedUser           int64
	setupLocked           bool
	restoreInProgress     bool
}

func newFakeAuthRepo(count int) *fakeAuthRepo {
	return &fakeAuthRepo{
		count:    count,
		nextID:   int64(count + 1),
		users:    map[string]UserRow{},
		sessions: map[string]SessionRow{},
	}
}

func (r *fakeAuthRepo) CountUsers(context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count, nil
}

func (r *fakeAuthRepo) CountEntries(context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.entryCount, nil
}

func (r *fakeAuthRepo) CountImages(context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.imageCount, nil
}

func (r *fakeAuthRepo) SetupLocked(context.Context) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.setupLocked, nil
}

func (r *fakeAuthRepo) SetupRestoreInProgress(context.Context) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.restoreInProgress, nil
}

func (r *fakeAuthRepo) BeginSetupRestore(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.setupLocked || r.count > 0 {
		return ErrSetupLocked
	}
	if r.restoreInProgress {
		return ErrRestoreInProgress
	}
	r.restoreInProgress = true
	return nil
}

func (r *fakeAuthRepo) ClearSetupRestoreInProgress(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.restoreInProgress = false
	return nil
}

func (r *fakeAuthRepo) FinishSetupRestore(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.setupLocked = true
	r.restoreInProgress = false
	return nil
}

func (r *fakeAuthRepo) CreateUser(_ context.Context, username string, passwordHash string, now time.Time) (UserRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.createUserLocked(username, passwordHash, RoleUser, now)
}

func (r *fakeAuthRepo) createUserLocked(username string, passwordHash string, role string, now time.Time) (UserRow, error) {
	if _, ok := r.users[username]; ok {
		return UserRow{}, ErrUsernameExists
	}
	row := UserRow{ID: r.nextID, Username: username, PasswordHash: passwordHash, Role: role, CreatedAt: now.Format(time.RFC3339)}
	r.nextID++
	r.count++
	r.users[username] = row
	return row, nil
}

func (r *fakeAuthRepo) addUser(t *testing.T, username string, password string, role string) UserRow {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	row, err := r.createUserLocked(username, string(hash), role, time.Now())
	if err != nil {
		t.Fatalf("add user: %v", err)
	}
	return row
}

func (r *fakeAuthRepo) CreateFirstOwner(_ context.Context, username string, passwordHash string, now time.Time) (UserRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.setupLocked || r.count > 0 {
		return UserRow{}, ErrSetupLocked
	}
	if r.restoreInProgress {
		return UserRow{}, ErrRestoreInProgress
	}
	row, err := r.createUserLocked(username, passwordHash, RoleOwner, now)
	if err != nil {
		return UserRow{}, err
	}
	r.setupLocked = true
	return row, nil
}

func (r *fakeAuthRepo) GetUserByUsername(_ context.Context, username string) (UserRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	row, ok := r.users[username]
	if !ok {
		return UserRow{}, ErrInvalidCredentials
	}
	return row, nil
}

func (r *fakeAuthRepo) GetUserByID(_ context.Context, id int64) (UserRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, row := range r.users {
		if row.ID == id {
			return row, nil
		}
	}
	return UserRow{}, ErrInvalidCredentials
}

func (r *fakeAuthRepo) CreateSession(_ context.Context, userID int64, tokenHash string, csrfHash string, _ time.Time, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, row := range r.users {
		if row.ID == userID {
			r.sessions[tokenHash] = SessionRow{User: row, CSRFHash: csrfHash}
			return nil
		}
	}
	return ErrUnauthorized
}

func (r *fakeAuthRepo) GetSessionByTokenHash(_ context.Context, tokenHash string, _ time.Time) (SessionRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[tokenHash]
	if !ok {
		return SessionRow{}, ErrUnauthorized
	}
	return session, nil
}

func (r *fakeAuthRepo) UpdateSessionCSRF(_ context.Context, tokenHash string, csrfHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[tokenHash]
	if !ok {
		return ErrUnauthorized
	}
	session.CSRFHash = csrfHash
	r.sessions[tokenHash] = session
	return nil
}

func (r *fakeAuthRepo) DeleteSession(_ context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, tokenHash)
	return nil
}

func (r *fakeAuthRepo) DeleteExpiredSessions(context.Context, time.Time) error {
	return nil
}

func (r *fakeAuthRepo) ClaimLegacyEntries(_ context.Context, userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claimedUser = userID
	return nil
}

func (r *fakeAuthRepo) DeleteAccount(_ context.Context, userID int64) (AccountDeletionResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var username string
	var user UserRow
	for candidate, row := range r.users {
		if row.ID == userID {
			username = candidate
			user = row
			break
		}
	}
	if username == "" {
		return AccountDeletionResult{}, ErrUnauthorized
	}
	if user.Role == RoleOwner && r.count > 1 {
		return AccountDeletionResult{}, ErrOwnerAccountRequired
	}

	delete(r.users, username)
	for tokenHash, session := range r.sessions {
		if session.User.ID == userID {
			delete(r.sessions, tokenHash)
		}
	}
	if r.count > 0 {
		r.count--
	}
	if r.count == 0 {
		r.setupLocked = false
		r.restoreInProgress = false
	}
	return AccountDeletionResult{
		ImageFilePaths: append([]string{}, r.accountImageFilePaths...),
		RemainingUsers: r.count,
	}, nil
}

func (r *fakeAuthRepo) user(username string) UserRow {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.users[username]
}

func (r *fakeAuthRepo) locked() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.setupLocked
}

func (r *fakeAuthRepo) countUsers() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func (r *fakeAuthRepo) ownerCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, user := range r.users {
		if user.Role == RoleOwner {
			count++
		}
	}
	return count
}

func (r *fakeAuthRepo) restoreFixture(entryCount int, imageCount int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.count = 1
	r.entryCount = entryCount
	r.imageCount = imageCount
	r.users["owner"] = UserRow{
		ID:           1,
		Username:     "owner",
		PasswordHash: "restored-password-hash",
		Role:         RoleOwner,
		CreatedAt:    time.Now().Format(time.RFC3339),
	}
	if r.nextID < 2 {
		r.nextID = 2
	}
}

type fakeAccountFileDeleter struct {
	deleted []string
}

func (d *fakeAccountFileDeleter) Delete(_ context.Context, path string) error {
	d.deleted = append(d.deleted, path)
	return nil
}
