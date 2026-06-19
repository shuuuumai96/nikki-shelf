package logx_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/httpx"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  slog.Level
	}{
		{name: "debug", value: "debug", want: slog.LevelDebug},
		{name: "info default", value: "", want: slog.LevelInfo},
		{name: "warn alias", value: "warning", want: slog.LevelWarn},
		{name: "error", value: "error", want: slog.LevelError},
		{name: "unknown default", value: "verbose", want: slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := logx.ParseLevel(tt.value); got != tt.want {
				t.Fatalf("ParseLevel(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestMiddlewareLogsRequestFields(t *testing.T) {
	buffer := bytes.Buffer{}
	server := newServer(&buffer)
	server.GET("/things/:id", func(c echo.Context) error {
		logx.SetUserID(c, 42)
		return httpx.JSON(c, http.StatusOK, map[string]string{"ok": "true"})
	})

	request := httptest.NewRequest(http.MethodGet, "/things/7?query=private", nil)
	request.Header.Set("X-Request-ID", "req-123")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	record := decodeLog(t, buffer.String())
	assertField(t, record, "msg", "http request")
	assertField(t, record, "request_id", "req-123")
	assertField(t, record, "method", http.MethodGet)
	assertField(t, record, "route", "/things/:id")
	assertField(t, record, "status", float64(http.StatusOK))
	assertField(t, record, "user_id", float64(42))

	if strings.Contains(buffer.String(), "private") {
		t.Fatal("request query value leaked into logs")
	}
}

func TestRequestIDMiddlewareRejectsOversizedHeader(t *testing.T) {
	buffer := bytes.Buffer{}
	server := newServer(&buffer)
	server.GET("/things", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	oversized := strings.Repeat("a", 256)
	request := httptest.NewRequest(http.MethodGet, "/things", nil)
	request.Header.Set("X-Request-ID", oversized)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Header().Get("X-Request-ID") == oversized {
		t.Fatal("oversized request id was echoed back")
	}
	record := decodeLog(t, buffer.String())
	if got := record["request_id"]; got == oversized || got == "" {
		t.Fatalf("request_id = %#v, want generated bounded id", got)
	}
}

func TestMiddlewareIgnoresForwardedHeaderByDefault(t *testing.T) {
	buffer := bytes.Buffer{}
	server := newServer(&buffer)
	server.GET("/things", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/things", nil)
	request.RemoteAddr = "198.51.100.10:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.99")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	record := decodeLog(t, buffer.String())
	assertField(t, record, "remote_ip", "198.51.100.10")
}

func TestMiddlewareUsesConfiguredRemoteIP(t *testing.T) {
	buffer := bytes.Buffer{}
	server := echo.New()
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	server.Use(logx.RequestIDMiddleware())
	server.Use(logx.MiddlewareWithRemoteIP(logger, func(*http.Request) string {
		return "203.0.113.20"
	}))
	server.GET("/things", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/things", nil)
	request.RemoteAddr = "198.51.100.10:1234"
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	record := decodeLog(t, buffer.String())
	assertField(t, record, "remote_ip", "203.0.113.20")
}

func TestInternalErrorLogsOriginalErrorButHidesResponse(t *testing.T) {
	buffer := bytes.Buffer{}
	server := newServer(&buffer)
	server.GET("/boom", func(c echo.Context) error {
		return httpx.Internal(c, errors.New("database unavailable"))
	})

	request := httptest.NewRequest(http.MethodGet, "/boom?query=private", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if strings.Contains(response.Body.String(), "database unavailable") {
		t.Fatal("internal error leaked into response body")
	}

	record := decodeLog(t, buffer.String())
	assertField(t, record, "status", float64(http.StatusInternalServerError))
	assertField(t, record, "error_kind", "server.internal")
	assertField(t, record, "error", "database unavailable")

	if strings.Contains(buffer.String(), "private") {
		t.Fatal("request query value leaked into logs")
	}
}

func newServer(buffer *bytes.Buffer) *echo.Echo {
	server := echo.New()
	logger := slog.New(slog.NewJSONHandler(buffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	server.Use(logx.RequestIDMiddleware())
	server.Use(logx.Middleware(logger))
	return server
}

func decodeLog(t *testing.T, line string) map[string]any {
	t.Helper()

	decoder := json.NewDecoder(strings.NewReader(line))
	record := map[string]any{}
	if err := decoder.Decode(&record); err != nil {
		t.Fatalf("decode log: %v\nlog: %s", err, line)
	}
	return record
}

func assertField(t *testing.T, record map[string]any, key string, want any) {
	t.Helper()

	if got := record[key]; got != want {
		t.Fatalf("%s = %#v, want %#v", key, got, want)
	}
}
