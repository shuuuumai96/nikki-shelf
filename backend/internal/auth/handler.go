package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/httpx"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
)

const SessionCookieName = "nikki_session"
const BootstrapTokenHeader = "X-Nikki-Bootstrap-Token"

type Handler struct {
	service      *Service
	cookieSecure bool
	rateLimiter  *RateLimiter
}

type HandlerConfig struct {
	CookieSecure bool
	RateLimiter  *RateLimiter
}

func NewHandler(service *Service, configs ...HandlerConfig) *Handler {
	cfg := HandlerConfig{}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	return &Handler{
		service:      service,
		cookieSecure: cfg.CookieSecure,
		rateLimiter:  cfg.RateLimiter,
	}
}

func (h *Handler) Register(api *echo.Group) {
	api.GET("/auth/config", h.config)
	api.POST("/auth/signup", h.signup)
	api.POST("/auth/login", h.login)
	api.POST("/auth/logout", h.logout, CSRF(h.service))
	api.GET("/auth/me", h.me)
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

func (h *Handler) start(c echo.Context, event string, start func(context.Context, Credentials) (SessionResult, error)) error {
	input := Credentials{}
	if err := httpx.DecodeJSONWithLimit(c, &input, httpx.AuthJSONLimitBytes); err != nil {
		if errors.Is(err, httpx.ErrRequestTooLarge) {
			return httpx.ErrorWithKind(c, http.StatusRequestEntityTooLarge, "JSONが大きすぎます", "request.too_large")
		}
		return httpx.ErrorWithKind(c, http.StatusBadRequest, "JSONを確認してください", "request.invalid_json")
	}

	if h.rateLimiter != nil && !h.rateLimiter.Allow(c, input.Username) {
		return rateLimitError(c)
	}

	session, err := start(c.Request().Context(), input)
	if err != nil {
		if h.rateLimiter != nil {
			h.rateLimiter.RecordFailure(c, input.Username)
		}
		return authError(c, err)
	}
	if h.rateLimiter != nil {
		h.rateLimiter.RecordSuccess(c, input.Username)
	}

	setSessionCookie(c, session, h.cookieSecure)
	logx.SetUserID(c, session.User.ID)
	logx.Event(c, event, slog.Int64("user_id", session.User.ID))
	return httpx.JSON(c, http.StatusOK, session.User)
}

func (h *Handler) me(c echo.Context) error {
	token, err := sessionToken(c)
	if err != nil {
		return httpx.Error(c, http.StatusUnauthorized, ErrUnauthorized.Error())
	}

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

func CSRF(service *Service) echo.MiddlewareFunc {
	return csrfWithValidator(func(ctx context.Context, sessionToken string, csrfToken string) bool {
		return service != nil && service.ValidateCSRF(ctx, sessionToken, csrfToken)
	})
}

func csrfWithValidator(validate func(context.Context, string, string) bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			switch c.Request().Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				return next(c)
			}

			token, err := sessionToken(c)
			if err != nil || !validate(c.Request().Context(), token, c.Request().Header.Get("X-CSRF-Token")) {
				return httpx.ErrorWithKind(c, http.StatusForbidden, "リクエストを確認してください", "auth.csrf")
			}
			return next(c)
		}
	}
}

func authError(c echo.Context, err error) error {
	status := StatusFor(err)
	if status >= http.StatusInternalServerError {
		return httpx.Internal(c, err)
	}
	return httpx.ErrorWithKind(c, status, err.Error(), KindFor(err))
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
