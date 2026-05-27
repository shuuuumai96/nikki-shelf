package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
)

const (
	AuthJSONLimitBytes    int64 = 16 << 10
	GenericJSONLimitBytes int64 = 64 << 10
	EntryJSONLimitBytes   int64 = 1 << 20
)

var ErrRequestTooLarge = errors.New("request body is too large")

func DecodeJSON(c echo.Context, out any) error {
	return DecodeJSONWithLimit(c, out, GenericJSONLimitBytes)
}

func DecodeJSONWithLimit(c echo.Context, out any, limitBytes int64) error {
	request := c.Request()
	if limitBytes > 0 {
		request.Body = http.MaxBytesReader(c.Response().Writer, request.Body, limitBytes)
	}

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		if isMaxBytesError(err) {
			return ErrRequestTooLarge
		}
		return err
	}

	// Reject concatenated JSON values. Without this, a valid first object could
	// hide trailing junk from handlers.
	if err := decoder.Decode(&struct{}{}); err == nil {
		return errors.New("request body must contain a single JSON value")
	} else if !errors.Is(err, io.EOF) {
		if isMaxBytesError(err) {
			return ErrRequestTooLarge
		}
		return err
	}

	return nil
}

func isMaxBytesError(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}

func JSON(c echo.Context, status int, value any) error {
	return c.JSON(status, value)
}

func NoContent(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

func Error(c echo.Context, status int, message string) error {
	return c.JSON(status, map[string]string{"error": message})
}

func ErrorWithKind(c echo.Context, status int, message string, kind string) error {
	logx.SetError(c, kind, nil)
	return c.JSON(status, map[string]string{"error": message, "kind": kind})
}

func Internal(c echo.Context, err error) error {
	logx.SetError(c, "server.internal", err)
	return ErrorWithKind(c, http.StatusInternalServerError, "something went wrong on the server", "server.internal")
}
