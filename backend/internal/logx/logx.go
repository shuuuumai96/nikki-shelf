package logx

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

const (
	requestIDHeader    = "X-Request-ID"
	maxRequestIDLength = 128

	errorKey     = "log.error"
	errorKindKey = "log.error_kind"
	loggerKey    = "log.logger"
	requestIDKey = "log.request_id"
	userIDKey    = "log.user_id"
)

const internalMessage = "something went wrong on the server"

type Config struct {
	Level  string
	Format string
}

func New(cfg Config) *slog.Logger {
	level := ParseLevel(cfg.Level)
	options := &slog.HandlerOptions{Level: level}

	if strings.EqualFold(strings.TrimSpace(cfg.Format), "text") {
		return slog.New(slog.NewTextHandler(os.Stdout, options))
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, options))
}

func ParseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func RequestIDMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestID := strings.TrimSpace(c.Request().Header.Get(requestIDHeader))
			requestID = normalizeRequestID(requestID)
			if requestID == "" {
				requestID = newRequestID()
			}

			c.Set(requestIDKey, requestID)
			c.Response().Header().Set(requestIDHeader, requestID)
			return next(c)
		}
	}
}

func Middleware(logger *slog.Logger) echo.MiddlewareFunc {
	return MiddlewareWithRemoteIP(logger, directRemoteIP)
}

func MiddlewareWithRemoteIP(logger *slog.Logger, remoteIP func(*http.Request) string) echo.MiddlewareFunc {
	if logger == nil {
		logger = slog.Default()
	}
	if remoteIP == nil {
		remoteIP = directRemoteIP
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(loggerKey, logger)
			startedAt := time.Now()

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			// Echo handlers may write directly and return nil. Treat an unset
			// status as OK so request logs do not emit status 0.
			status := c.Response().Status
			if status == 0 {
				status = http.StatusOK
			}

			duration := time.Since(startedAt)
			attrs := requestAttrs(c, status, duration, remoteIP)
			attrs = append(attrs, ErrorAttrs(c)...)
			if status >= http.StatusInternalServerError && err != nil && Error(c) == nil {
				attrs = append(attrs, slog.String("error", err.Error()))
			}

			logger.LogAttrs(c.Request().Context(), requestLevel(c, status, duration), "http request", attrs...)
			return nil
		}
	}
}

func Recover(logger *slog.Logger) echo.MiddlewareFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}

				panicErr := panicError(recovered)
				SetError(c, "server.panic", panicErr)
				logger.LogAttrs(c.Request().Context(), slog.LevelError, "panic recovered",
					slog.String("request_id", RequestID(c)),
					slog.String("method", c.Request().Method),
					slog.String("route", route(c)),
					slog.String("error", panicErr.Error()),
					slog.String("stack", string(debug.Stack())),
				)

				if !c.Response().Committed {
					_ = c.JSON(http.StatusInternalServerError, map[string]string{
						"error": internalMessage,
						"kind":  "server.panic",
					})
				}
				// Echo's outer error handler has already been bypassed by the
				// recovered panic; returning nil prevents a duplicate response.
				err = nil
			}()

			return next(c)
		}
	}
}

func SetError(c echo.Context, kind string, err error) {
	if strings.TrimSpace(kind) != "" {
		c.Set(errorKindKey, kind)
	}
	if err != nil {
		c.Set(errorKey, err)
	}
}

func ErrorKind(c echo.Context) string {
	kind, _ := c.Get(errorKindKey).(string)
	return kind
}

func Error(c echo.Context) error {
	err, _ := c.Get(errorKey).(error)
	return err
}

func ErrorAttrs(c echo.Context) []slog.Attr {
	attrs := []slog.Attr{}
	if kind := ErrorKind(c); kind != "" {
		attrs = append(attrs, slog.String("error_kind", kind))
	}
	if err := Error(c); err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	return attrs
}

func SetUserID(c echo.Context, userID int64) {
	c.Set(userIDKey, userID)
}

func UserID(c echo.Context) (int64, bool) {
	userID, ok := c.Get(userIDKey).(int64)
	return userID, ok
}

func RequestID(c echo.Context) string {
	requestID, _ := c.Get(requestIDKey).(string)
	return requestID
}

func Event(c echo.Context, event string, attrs ...slog.Attr) {
	logger, _ := c.Get(loggerKey).(*slog.Logger)
	if logger == nil {
		return
	}

	eventAttrs := make([]slog.Attr, 0, len(attrs)+1)
	if requestID := RequestID(c); requestID != "" {
		eventAttrs = append(eventAttrs, slog.String("request_id", requestID))
	}
	eventAttrs = append(eventAttrs, attrs...)

	logger.LogAttrs(c.Request().Context(), slog.LevelInfo, event, eventAttrs...)
}

func requestAttrs(c echo.Context, status int, duration time.Duration, remoteIP func(*http.Request) string) []slog.Attr {
	request := c.Request()
	attrs := []slog.Attr{
		slog.String("request_id", RequestID(c)),
		slog.String("method", request.Method),
		slog.String("route", route(c)),
		slog.Int("status", status),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Int64("bytes_out", c.Response().Size),
		slog.String("remote_ip", remoteIP(request)),
	}

	if request.ContentLength >= 0 {
		attrs = append(attrs, slog.Int64("bytes_in", request.ContentLength))
	}
	if userID, ok := UserID(c); ok {
		attrs = append(attrs, slog.Int64("user_id", userID))
	}

	return attrs
}

func directRemoteIP(request *http.Request) string {
	if request == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(request.RemoteAddr)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "unknown"
	}
	return ip.String()
}

func requestLevel(c echo.Context, status int, duration time.Duration) slog.Level {
	if status >= http.StatusInternalServerError {
		return slog.LevelError
	}
	if route(c) == "/api/health" {
		return slog.LevelDebug
	}
	if duration >= time.Second || status >= http.StatusBadRequest {
		return slog.LevelWarn
	}
	return slog.LevelInfo
}

func route(c echo.Context) string {
	if path := c.Path(); path != "" {
		return path
	}
	return c.Request().URL.Path
}

func normalizeRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxRequestIDLength {
		return ""
	}
	for _, char := range value {
		if char < 33 || char > 126 {
			return ""
		}
	}
	return value
}

func newRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err == nil {
		return hex.EncodeToString(bytes)
	}
	// Logging should keep working even if the CSPRNG fails; this ID is for
	// correlation, not authentication.
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func panicError(value any) error {
	if err, ok := value.(error); ok {
		return err
	}
	return fmt.Errorf("%v", value)
}
