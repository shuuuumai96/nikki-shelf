package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/auth"
)

func TestRequireOwnerRejectsMissingUser(t *testing.T) {
	response := performOwnerRequest(nil)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestRequireOwnerRejectsNonOwner(t *testing.T) {
	user := auth.User{ID: 2, Username: "viewer", Role: auth.RoleUser}
	response := performOwnerRequest(&user)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if !strings.Contains(response.Body.String(), "auth.owner_required") {
		t.Fatalf("body = %s, want owner_required kind", response.Body.String())
	}
}

func TestRequireOwnerAllowsOwner(t *testing.T) {
	user := auth.User{ID: 1, Username: "owner", Role: auth.RoleOwner}
	response := performOwnerRequest(&user)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func performOwnerRequest(user *auth.User) *httptest.ResponseRecorder {
	server := echo.New()
	server.GET("/api/audit/events", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if user != nil {
				auth.SetUser(c, *user)
			}
			return requireOwner(next)(c)
		}
	})

	request := httptest.NewRequest(http.MethodGet, "/api/audit/events", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}
