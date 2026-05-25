package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestDecodeJSONWithLimitRejectsTrailingTokens(t *testing.T) {
	server := echo.New()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"first"} {"name":"second"}`))
	response := httptest.NewRecorder()
	context := server.NewContext(request, response)

	input := struct {
		Name string `json:"name"`
	}{}
	if err := DecodeJSONWithLimit(context, &input, GenericJSONLimitBytes); err == nil {
		t.Fatal("DecodeJSONWithLimit returned nil, want trailing token error")
	}
}

func TestDecodeJSONWithLimitReturnsRequestTooLarge(t *testing.T) {
	server := echo.New()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"body":"`+strings.Repeat("x", 128)+`"}`))
	response := httptest.NewRecorder()
	context := server.NewContext(request, response)

	input := struct {
		Body string `json:"body"`
	}{}
	if err := DecodeJSONWithLimit(context, &input, 16); !errors.Is(err, ErrRequestTooLarge) {
		t.Fatalf("error = %v, want ErrRequestTooLarge", err)
	}
}
