package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/httpx"
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

func TestFirstUserSignupAllowsBrowserSetupWhenEnabled(t *testing.T) {
	response := performAuthRequest(t, NewService(newFakeAuthRepo(0), ServiceConfig{
		AllowFirstUserSetup: true,
	}), "/api/auth/signup", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if cookie := response.Result().Cookies()[0]; cookie.Name != SessionCookieName || cookie.Value == "" {
		t.Fatalf("session cookie = %#v, want %s with value", cookie, SessionCookieName)
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
			name:                "setup",
			userCount:           0,
			allowFirstUserSetup: true,
			wantMode:            "setup",
			wantAvailable:       true,
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

type fakeAuthRepo struct {
	count       int
	nextID      int64
	users       map[string]UserRow
	sessions    map[string]SessionRow
	claimedUser int64
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
	return r.count, nil
}

func (r *fakeAuthRepo) CreateUser(_ context.Context, username string, passwordHash string, now time.Time) (UserRow, error) {
	if _, ok := r.users[username]; ok {
		return UserRow{}, ErrUsernameExists
	}
	row := UserRow{ID: r.nextID, Username: username, PasswordHash: passwordHash, CreatedAt: now.Format(time.RFC3339)}
	r.nextID++
	r.count++
	r.users[username] = row
	return row, nil
}

func (r *fakeAuthRepo) CreateFirstUser(ctx context.Context, username string, passwordHash string, now time.Time) (UserRow, error) {
	if r.count > 0 {
		return UserRow{}, ErrSignupClosed
	}
	return r.CreateUser(ctx, username, passwordHash, now)
}

func (r *fakeAuthRepo) GetUserByUsername(_ context.Context, username string) (UserRow, error) {
	row, ok := r.users[username]
	if !ok {
		return UserRow{}, ErrInvalidCredentials
	}
	return row, nil
}

func (r *fakeAuthRepo) CreateSession(_ context.Context, userID int64, tokenHash string, csrfHash string, _ time.Time, _ time.Time) error {
	for _, row := range r.users {
		if row.ID == userID {
			r.sessions[tokenHash] = SessionRow{User: row, CSRFHash: csrfHash}
			return nil
		}
	}
	return ErrUnauthorized
}

func (r *fakeAuthRepo) GetSessionByTokenHash(_ context.Context, tokenHash string, _ time.Time) (SessionRow, error) {
	session, ok := r.sessions[tokenHash]
	if !ok {
		return SessionRow{}, ErrUnauthorized
	}
	return session, nil
}

func (r *fakeAuthRepo) UpdateSessionCSRF(_ context.Context, tokenHash string, csrfHash string) error {
	session, ok := r.sessions[tokenHash]
	if !ok {
		return ErrUnauthorized
	}
	session.CSRFHash = csrfHash
	r.sessions[tokenHash] = session
	return nil
}

func (r *fakeAuthRepo) DeleteSession(_ context.Context, tokenHash string) error {
	delete(r.sessions, tokenHash)
	return nil
}

func (r *fakeAuthRepo) DeleteExpiredSessions(context.Context, time.Time) error {
	return nil
}

func (r *fakeAuthRepo) ClaimLegacyEntries(_ context.Context, userID int64) error {
	r.claimedUser = userID
	return nil
}
