package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/audit"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/httpx"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
)

const SessionCookieName = "nikki_session"
const BootstrapTokenHeader = "X-Nikki-Bootstrap-Token"
const setupRestoreUploadLimitBytes int64 = 512 << 20

type Handler struct {
	service      *Service
	cookieSecure bool
	rateLimiter  *RateLimiter
	audit        *audit.Service
	clientIP     func(*http.Request) string
}

type HandlerConfig struct {
	CookieSecure bool
	RateLimiter  *RateLimiter
	Audit        *audit.Service
	ClientIP     func(*http.Request) string
}

func NewHandler(service *Service, configs ...HandlerConfig) *Handler {
	cfg := HandlerConfig{}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	if cfg.ClientIP == nil {
		cfg.ClientIP = NewClientIPExtractor("direct", nil).ClientIP
	}
	return &Handler{
		service:      service,
		cookieSecure: cfg.CookieSecure,
		rateLimiter:  cfg.RateLimiter,
		audit:        cfg.Audit,
		clientIP:     cfg.ClientIP,
	}
}

func (h *Handler) Register(api *echo.Group) {
	api.GET("/auth/config", h.config)
	api.POST("/auth/signup", h.signup)
	api.POST("/auth/login", h.login)
	api.POST("/auth/logout", h.logout, CSRF(h.service, CSRFConfig{Audit: h.audit}))
	api.GET("/auth/me", h.me)
	api.PUT("/auth/me/password", h.changePassword, Require(h.service), CSRF(h.service, CSRFConfig{Audit: h.audit}))
	api.DELETE("/auth/me", h.deleteMe, Require(h.service), CSRF(h.service, CSRFConfig{Audit: h.audit}))
	api.GET("/setup/status", h.setupStatus)
	api.POST("/setup/owner", h.setupOwner)
	api.POST("/setup/restore/verify", h.setupRestoreVerify)
	api.POST("/setup/restore", h.setupRestore)
}

func (h *Handler) config(c echo.Context) error {
	config, err := h.service.Config(c.Request().Context())
	if err != nil {
		return httpx.Internal(c, err)
	}
	return httpx.JSON(c, http.StatusOK, config)
}

func (h *Handler) signup(c echo.Context) error {
	return h.start(c, "auth.signup_succeeded", func(ctx context.Context, input Credentials) (SessionResult, error) {
		return h.service.Signup(ctx, input, c.Request().Header.Get(BootstrapTokenHeader))
	})
}

func (h *Handler) login(c echo.Context) error {
	return h.start(c, "auth.login_succeeded", h.service.Login)
}

func (h *Handler) setupStatus(c echo.Context) error {
	status, err := h.service.SetupStatus(c.Request().Context())
	if err != nil {
		return httpx.Internal(c, err)
	}
	return httpx.JSON(c, http.StatusOK, status)
}

func (h *Handler) setupOwner(c echo.Context) error {
	input := SetupOwnerInput{}
	if err := httpx.DecodeJSONWithLimit(c, &input, httpx.AuthJSONLimitBytes); err != nil {
		h.logSetupFailure(c, "invalid_input")
		if errors.Is(err, httpx.ErrRequestTooLarge) {
			return httpx.ErrorWithKind(c, http.StatusRequestEntityTooLarge, "request JSON is too large", "request.too_large")
		}
		return httpx.ErrorWithKind(c, http.StatusBadRequest, "check the request JSON", "request.invalid_json")
	}

	if h.rateLimiter != nil && !h.rateLimiter.Allow(c, input.Username) {
		h.logSetupFailure(c, "rate_limited")
		h.logRateLimited(c, input.Username, "setup.owner")
		return rateLimitError(c)
	}

	session, err := h.service.CreateFirstOwner(c.Request().Context(), input)
	if err != nil {
		if h.rateLimiter != nil {
			h.rateLimiter.RecordFailure(c, input.Username)
		}
		h.logSetupFailure(c, setupFailureReason(err))
		h.recordAudit(c, audit.Event{
			EventType:     "setup.owner_create_failed",
			Outcome:       audit.OutcomeFailed,
			ActorUsername: auditUsername(input.Username),
			ReasonKind:    setupFailureReason(err),
		})
		return setupError(c, err)
	}
	if h.rateLimiter != nil {
		h.rateLimiter.RecordSuccess(c, input.Username)
	}

	setSessionCookie(c, session, h.cookieSecure)
	logx.SetUserID(c, session.User.ID)
	logx.Event(c, "setup.owner_created",
		slog.Int64("user_id", session.User.ID),
		slog.String("username", session.User.Username),
		slog.String("remote_ip", h.remoteIP(c)),
	)
	h.recordUserAudit(c, "setup.owner_created", session.User, audit.OutcomeSucceeded, "", nil)
	return httpx.JSON(c, http.StatusOK, session.User)
}

func (h *Handler) setupRestoreVerify(c echo.Context) error {
	upload, err := parseSetupRestoreUpload(c)
	if err != nil {
		h.logSetupRestoreFailure(c, "invalid_input")
		return setupRestoreUploadError(c, err)
	}
	defer upload.cleanup()

	if h.rateLimiter != nil && !h.rateLimiter.Allow(c, "setup-restore") {
		h.logSetupRestoreFailure(c, "rate_limited")
		h.logRateLimited(c, "setup-restore", "setup.restore")
		return rateLimitError(c)
	}

	result, err := h.service.VerifySetupRestore(c.Request().Context(), SetupRestoreVerifyInput{
		SetupToken:  upload.setupToken,
		ArchivePath: upload.path,
		ArchiveSize: upload.size,
	})
	if err != nil {
		if h.rateLimiter != nil {
			h.rateLimiter.RecordFailure(c, "setup-restore")
		}
		h.logSetupRestoreFailure(c, setupRestoreFailureReason(err))
		h.recordSetupRestoreAudit(c, audit.OutcomeFailed, setupRestoreFailureReason(err), nil)
		return setupError(c, err)
	}
	if h.rateLimiter != nil {
		h.rateLimiter.RecordSuccess(c, "setup-restore")
	}

	h.recordAudit(c, audit.Event{
		EventType: "setup.restore_verified",
		Outcome:   audit.OutcomeSucceeded,
		Metadata: audit.Metadata(
			"entry_count", result.EntryCount,
			"image_count", result.ImageCount,
			"backup_size_bytes", result.BackupSizeBytes,
		),
	})
	return httpx.JSON(c, http.StatusOK, result)
}

func (h *Handler) setupRestore(c echo.Context) error {
	upload, err := parseSetupRestoreUpload(c)
	if err != nil {
		h.logSetupRestoreFailure(c, "invalid_input")
		return setupRestoreUploadError(c, err)
	}
	defer upload.cleanup()

	if h.rateLimiter != nil && !h.rateLimiter.Allow(c, "setup-restore") {
		h.logSetupRestoreFailure(c, "rate_limited")
		h.logRateLimited(c, "setup-restore", "setup.restore")
		return rateLimitError(c)
	}

	result, err := h.service.RestoreSetupBackup(c.Request().Context(), SetupRestoreInput{
		SetupToken:     upload.setupToken,
		ArchivePath:    upload.path,
		ArchiveSize:    upload.size,
		ConfirmRestore: strings.EqualFold(upload.confirmRestore, "true"),
	})
	if err != nil {
		if h.rateLimiter != nil {
			h.rateLimiter.RecordFailure(c, "setup-restore")
		}
		h.logSetupRestoreFailure(c, setupRestoreFailureReason(err))
		h.recordSetupRestoreAudit(c, audit.OutcomeFailed, setupRestoreFailureReason(err), nil)
		return setupError(c, err)
	}
	if h.rateLimiter != nil {
		h.rateLimiter.RecordSuccess(c, "setup-restore")
	}

	logx.Event(c, "setup.restore_completed",
		slog.Int("entry_count", result.EntryCount),
		slog.Int("image_count", result.ImageCount),
		slog.String("remote_ip", h.remoteIP(c)),
	)
	h.recordSetupRestoreAudit(c, audit.OutcomeSucceeded, "", audit.Metadata(
		"entry_count", result.EntryCount,
		"image_count", result.ImageCount,
	))
	return httpx.JSON(c, http.StatusOK, result)
}

func (h *Handler) start(c echo.Context, event string, start func(context.Context, Credentials) (SessionResult, error)) error {
	input := Credentials{}
	if err := httpx.DecodeJSONWithLimit(c, &input, httpx.AuthJSONLimitBytes); err != nil {
		if errors.Is(err, httpx.ErrRequestTooLarge) {
			return httpx.ErrorWithKind(c, http.StatusRequestEntityTooLarge, "request JSON is too large", "request.too_large")
		}
		return httpx.ErrorWithKind(c, http.StatusBadRequest, "check the request JSON", "request.invalid_json")
	}

	if h.rateLimiter != nil && !h.rateLimiter.Allow(c, input.Username) {
		h.logRateLimited(c, input.Username, event)
		return rateLimitError(c)
	}

	session, err := start(c.Request().Context(), input)
	if err != nil {
		if h.rateLimiter != nil {
			h.rateLimiter.RecordFailure(c, input.Username)
		}
		h.recordAudit(c, audit.Event{
			EventType:     authFailureEvent(event),
			Outcome:       audit.OutcomeFailed,
			ActorUsername: auditUsername(input.Username),
			ReasonKind:    KindFor(err),
		})
		return authError(c, err)
	}
	if h.rateLimiter != nil {
		h.rateLimiter.RecordSuccess(c, input.Username)
	}

	setSessionCookie(c, session, h.cookieSecure)
	logx.SetUserID(c, session.User.ID)
	logx.Event(c, event, slog.Int64("user_id", session.User.ID))
	h.recordUserAudit(c, event, session.User, audit.OutcomeSucceeded, "", nil)
	return httpx.JSON(c, http.StatusOK, session.User)
}

func (h *Handler) me(c echo.Context) error {
	token, err := sessionToken(c)
	if err != nil {
		return httpx.ErrorWithKind(c, http.StatusUnauthorized, ErrUnauthorized.Error(), "auth.unauthorized")
	}

	// /me is the refresh point for the in-memory frontend CSRF token. Rotate the
	// token here while keeping the session cookie stable.
	user, err := h.service.UserWithCSRFByToken(c.Request().Context(), token)
	if err != nil {
		return authError(c, err)
	}

	SetUser(c, user)
	return httpx.JSON(c, http.StatusOK, user)
}

func (h *Handler) logout(c echo.Context) error {
	token, _ := sessionToken(c)
	if err := h.service.Logout(c.Request().Context(), token); err != nil {
		return httpx.Internal(c, err)
	}

	clearSessionCookie(c, h.cookieSecure)
	logx.Event(c, "auth.logout_succeeded")
	h.recordAudit(c, audit.Event{EventType: "auth.logout_succeeded", Outcome: audit.OutcomeSucceeded})
	return httpx.NoContent(c)
}

func (h *Handler) changePassword(c echo.Context) error {
	user, ok := UserFromContext(c)
	if !ok {
		return httpx.ErrorWithKind(c, http.StatusUnauthorized, ErrUnauthorized.Error(), "auth.unauthorized")
	}

	input := ChangePasswordInput{}
	if err := httpx.DecodeJSONWithLimit(c, &input, httpx.AuthJSONLimitBytes); err != nil {
		if errors.Is(err, httpx.ErrRequestTooLarge) {
			return httpx.ErrorWithKind(c, http.StatusRequestEntityTooLarge, "request JSON is too large", "request.too_large")
		}
		return httpx.ErrorWithKind(c, http.StatusBadRequest, "check the request JSON", "request.invalid_json")
	}

	rateKey := user.Username
	if h.rateLimiter != nil && !h.rateLimiter.Allow(c, rateKey) {
		h.logRateLimited(c, rateKey, "auth.password_changed")
		return rateLimitError(c)
	}

	if err := h.service.ChangePassword(c.Request().Context(), user.ID, input); err != nil {
		if h.rateLimiter != nil {
			h.rateLimiter.RecordFailure(c, rateKey)
		}
		h.recordUserAudit(c, "auth.password_change_failed", user, audit.OutcomeFailed, KindFor(err), nil)
		return authError(c, err)
	}
	if h.rateLimiter != nil {
		h.rateLimiter.RecordSuccess(c, rateKey)
	}

	clearSessionCookie(c, h.cookieSecure)
	logx.Event(c, "auth.password_changed", slog.Int64("user_id", user.ID))
	h.recordUserAudit(c, "auth.password_changed", user, audit.OutcomeSucceeded, "", nil)
	return httpx.NoContent(c)
}

func (h *Handler) deleteMe(c echo.Context) error {
	user, ok := UserFromContext(c)
	if !ok {
		return httpx.ErrorWithKind(c, http.StatusUnauthorized, ErrUnauthorized.Error(), "auth.unauthorized")
	}

	input := DeleteAccountInput{}
	if err := httpx.DecodeJSONWithLimit(c, &input, httpx.AuthJSONLimitBytes); err != nil {
		if errors.Is(err, httpx.ErrRequestTooLarge) {
			return httpx.ErrorWithKind(c, http.StatusRequestEntityTooLarge, "request JSON is too large", "request.too_large")
		}
		return httpx.ErrorWithKind(c, http.StatusBadRequest, "check the request JSON", "request.invalid_json")
	}

	rateKey := input.Username
	if strings.TrimSpace(rateKey) == "" {
		rateKey = user.Username
	}
	if h.rateLimiter != nil && !h.rateLimiter.Allow(c, rateKey) {
		h.logRateLimited(c, rateKey, "auth.account_deleted")
		return rateLimitError(c)
	}

	result, err := h.service.DeleteCurrentAccount(c.Request().Context(), user.ID, input)
	if err != nil {
		if h.rateLimiter != nil {
			h.rateLimiter.RecordFailure(c, rateKey)
		}
		h.recordUserAudit(c, "auth.account_delete_failed", user, audit.OutcomeFailed, KindFor(err), nil)
		return authError(c, err)
	}
	if h.rateLimiter != nil {
		h.rateLimiter.RecordSuccess(c, rateKey)
	}

	clearSessionCookie(c, h.cookieSecure)
	logx.Event(c, "auth.account_deleted",
		slog.Int64("user_id", user.ID),
		slog.Int("remaining_users", result.RemainingUsers),
	)
	h.recordUserAudit(c, "auth.account_deleted", user, audit.OutcomeSucceeded, "", audit.Metadata("remaining_users", result.RemainingUsers))
	return httpx.NoContent(c)
}

func Require(service *Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token, err := sessionToken(c)
			if err != nil {
				return httpx.ErrorWithKind(c, http.StatusUnauthorized, ErrUnauthorized.Error(), "auth.unauthorized")
			}

			user, err := service.UserByToken(c.Request().Context(), token)
			if err != nil {
				return authError(c, err)
			}

			SetUser(c, user)
			return next(c)
		}
	}
}

type CSRFConfig struct {
	Audit *audit.Service
}

func CSRF(service *Service, configs ...CSRFConfig) echo.MiddlewareFunc {
	cfg := CSRFConfig{}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	return csrfWithValidator(func(ctx context.Context, sessionToken string, csrfToken string) bool {
		return service != nil && service.ValidateCSRF(ctx, sessionToken, csrfToken)
	}, cfg)
}

func csrfWithValidator(validate func(context.Context, string, string) bool, configs ...CSRFConfig) echo.MiddlewareFunc {
	cfg := CSRFConfig{}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Reads are authenticated by the HttpOnly session cookie; mutating
			// requests also need the per-session CSRF token returned by /auth/me.
			switch c.Request().Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				return next(c)
			}

			token, err := sessionToken(c)
			if err != nil || !validate(c.Request().Context(), token, c.Request().Header.Get("X-CSRF-Token")) {
				recordCSRFFailure(c, cfg.Audit)
				return httpx.ErrorWithKind(c, http.StatusForbidden, "check the request", "auth.csrf")
			}
			return next(c)
		}
	}
}

func authError(c echo.Context, err error) error {
	if errors.Is(err, ErrRestoreInProgress) {
		return httpx.ErrorWithKind(c, http.StatusServiceUnavailable, ErrRestoreInProgress.Error(), "setup.restore_in_progress")
	}
	status := StatusFor(err)
	if status >= http.StatusInternalServerError {
		return httpx.Internal(c, err)
	}
	return httpx.ErrorWithKind(c, status, err.Error(), KindFor(err))
}

func setupError(c echo.Context, err error) error {
	if errors.Is(err, ErrInvalidInput) {
		return httpx.ErrorWithKind(c, http.StatusBadRequest, ErrInvalidInput.Error(), "setup.invalid_input")
	}
	if errors.Is(err, ErrUsernameExists) {
		return httpx.ErrorWithKind(c, http.StatusConflict, ErrUsernameExists.Error(), "setup.username_exists")
	}
	if errors.Is(err, ErrInvalidBackup) {
		return httpx.ErrorWithKind(c, http.StatusBadRequest, ErrInvalidBackup.Error(), "setup.invalid_backup")
	}
	if errors.Is(err, ErrRestoreInProgress) {
		return httpx.ErrorWithKind(c, http.StatusServiceUnavailable, ErrRestoreInProgress.Error(), "setup.restore_in_progress")
	}
	if errors.Is(err, ErrRestoreFailed) || errors.Is(err, ErrRestoreCountMismatch) {
		return httpx.ErrorWithKind(c, http.StatusInternalServerError, ErrRestoreFailed.Error(), "setup.restore_failed")
	}

	status := StatusFor(err)
	if status >= http.StatusInternalServerError {
		return httpx.Internal(c, err)
	}
	return httpx.ErrorWithKind(c, status, err.Error(), KindFor(err))
}

func setupFailureReason(err error) string {
	switch {
	case errors.Is(err, ErrInvalidSetupToken):
		return "invalid_token"
	case errors.Is(err, ErrSetupLocked):
		return "already_initialized"
	case errors.Is(err, ErrRestoreInProgress):
		return "restore_in_progress"
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrUsernameExists):
		return "invalid_input"
	default:
		return "server_error"
	}
}

func (h *Handler) logSetupFailure(c echo.Context, reason string) {
	logx.Event(c, "setup.owner_create_failed",
		slog.String("reason_kind", reason),
		slog.String("remote_ip", h.remoteIP(c)),
	)
}

func setupRestoreFailureReason(err error) string {
	switch {
	case errors.Is(err, ErrInvalidSetupToken):
		return "invalid_token"
	case errors.Is(err, ErrSetupLocked):
		return "already_initialized"
	case errors.Is(err, ErrRestoreInProgress):
		return "restore_in_progress"
	case errors.Is(err, ErrInvalidBackup), errors.Is(err, ErrInvalidInput), errors.Is(err, ErrRestoreConfirmationMissing):
		return "invalid_input"
	case errors.Is(err, ErrRestoreFailed), errors.Is(err, ErrRestoreCountMismatch):
		return "restore_failed"
	default:
		return "server_error"
	}
}

func (h *Handler) logSetupRestoreFailure(c echo.Context, reason string) {
	logx.Event(c, "setup.restore_failed",
		slog.String("reason_kind", reason),
		slog.String("remote_ip", h.remoteIP(c)),
	)
}

func (h *Handler) recordAudit(c echo.Context, event audit.Event) {
	if h.audit == nil {
		return
	}
	h.audit.RecordHTTP(c, event)
}

func (h *Handler) recordUserAudit(c echo.Context, eventType string, user User, outcome string, reasonKind string, metadata map[string]string) {
	h.recordAudit(c, audit.Event{
		EventType:     eventType,
		Outcome:       outcome,
		ActorUserID:   audit.UserID(user.ID),
		ActorUsername: user.Username,
		ActorRole:     user.Role,
		ReasonKind:    reasonKind,
		Metadata:      metadata,
	})
}

func (h *Handler) recordSetupRestoreAudit(c echo.Context, outcome string, reasonKind string, metadata map[string]string) {
	eventType := "setup.restore_completed"
	if outcome != audit.OutcomeSucceeded {
		eventType = "setup.restore_failed"
	}
	h.recordAudit(c, audit.Event{
		EventType:  eventType,
		Outcome:    outcome,
		ReasonKind: reasonKind,
		Metadata:   metadata,
	})
}

func (h *Handler) logRateLimited(c echo.Context, username string, flow string) {
	logx.Event(c, "auth.rate_limited",
		slog.String("flow", flow),
		slog.String("username", auditUsername(username)),
		slog.String("remote_ip", h.remoteIP(c)),
	)
}

func (h *Handler) remoteIP(c echo.Context) string {
	if h.clientIP == nil {
		return NewClientIPExtractor("direct", nil).ClientIP(c.Request())
	}
	return h.clientIP(c.Request())
}

func recordCSRFFailure(c echo.Context, auditService *audit.Service) {
	if auditService == nil {
		return
	}
	user, ok := UserFromContext(c)
	if !ok {
		return
	}
	event := audit.Event{
		EventType:     "auth.csrf_failed",
		Outcome:       audit.OutcomeFailed,
		ReasonKind:    "auth.csrf",
		ActorUserID:   audit.UserID(user.ID),
		ActorUsername: user.Username,
		ActorRole:     user.Role,
	}
	auditService.RecordHTTP(c, event)
}

func authFailureEvent(event string) string {
	switch event {
	case "auth.login_succeeded":
		return "auth.login_failed"
	case "auth.signup_succeeded":
		return "auth.signup_failed"
	default:
		return event + "_failed"
	}
}

func auditUsername(username string) string {
	value := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(username, "\n", " "), "\r", " ")))
	runes := []rune(value)
	if len(runes) > 64 {
		return string(runes[:64])
	}
	return value
}

type setupRestoreUpload struct {
	path           string
	setupToken     string
	confirmRestore string
	size           int64
}

func (u setupRestoreUpload) cleanup() {
	if u.path != "" {
		_ = os.Remove(u.path)
	}
}

func parseSetupRestoreUpload(c echo.Context) (setupRestoreUpload, error) {
	request := c.Request()
	request.Body = http.MaxBytesReader(c.Response().Writer, request.Body, setupRestoreUploadLimitBytes)
	if err := request.ParseMultipartForm(32 << 20); err != nil {
		if httpx.IsRequestTooLarge(err) {
			return setupRestoreUpload{}, httpx.ErrRequestTooLarge
		}
		return setupRestoreUpload{}, ErrInvalidInput
	}

	file, _, err := request.FormFile("backupFile")
	if err != nil {
		return setupRestoreUpload{}, ErrInvalidInput
	}
	defer file.Close()

	tempFile, err := os.CreateTemp("", "nikki-setup-upload-*.tar.gz")
	if err != nil {
		return setupRestoreUpload{}, err
	}
	upload := setupRestoreUpload{path: tempFile.Name()}

	written, copyErr := io.Copy(tempFile, file)
	closeErr := tempFile.Close()
	if copyErr != nil {
		upload.cleanup()
		if httpx.IsRequestTooLarge(copyErr) {
			return setupRestoreUpload{}, httpx.ErrRequestTooLarge
		}
		return setupRestoreUpload{}, ErrInvalidInput
	}
	if closeErr != nil {
		upload.cleanup()
		return setupRestoreUpload{}, closeErr
	}
	upload.setupToken = request.FormValue("setupToken")
	upload.confirmRestore = request.FormValue("confirmRestore")
	upload.size = written
	if upload.size <= 0 {
		upload.cleanup()
		return setupRestoreUpload{}, ErrInvalidInput
	}
	return upload, nil
}

func setupRestoreUploadError(c echo.Context, err error) error {
	if errors.Is(err, httpx.ErrRequestTooLarge) {
		return httpx.ErrorWithKind(c, http.StatusRequestEntityTooLarge, "request body is too large", "request.too_large")
	}
	if errors.Is(err, ErrInvalidInput) {
		return httpx.ErrorWithKind(c, http.StatusBadRequest, ErrInvalidInput.Error(), "setup.invalid_input")
	}
	return httpx.Internal(c, err)
}

func sessionToken(c echo.Context) (string, error) {
	cookie, err := c.Cookie(SessionCookieName)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func setSessionCookie(c echo.Context, session SessionResult, secure bool) {
	c.SetCookie(sessionCookie(session, secure))
}

func sessionCookie(session SessionResult, secure bool) *http.Cookie {
	expiresAt, err := time.Parse(time.RFC3339, session.ExpiresAt)
	if err != nil {
		expiresAt = time.Now().Add(SessionTTL)
	}

	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    session.Token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func clearSessionCookie(c echo.Context, secure bool) {
	c.SetCookie(clearCookie(secure))
}

func clearCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}
