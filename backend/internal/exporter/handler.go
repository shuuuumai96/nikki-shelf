package exporter

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/auth"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/entries"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/httpx"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(api *echo.Group) {
	api.GET("/export/:format", h.export)
	api.GET("/export/entries/:entryId/markdown", h.exportEntryMarkdown)
}

func (h *Handler) export(c echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return httpx.ErrorWithKind(c, http.StatusUnauthorized, auth.ErrUnauthorized.Error(), "auth.unauthorized")
	}

	format := c.Param("format")
	content, exporter, err := h.service.Export(c.Request().Context(), userID, format)
	if errors.Is(err, ErrUnsupportedFormat) {
		return httpx.ErrorWithKind(c, http.StatusBadRequest, err.Error(), "export.unsupported_format")
	}
	if errors.Is(err, ErrExportTooLarge) {
		return httpx.ErrorWithKind(c, http.StatusRequestEntityTooLarge, err.Error(), "export.too_large")
	}
	if err != nil {
		return httpx.Internal(c, err)
	}

	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+exporter.FileName()+`"`)
	logx.Event(c, "export.completed",
		slog.Int64("user_id", userID),
		slog.String("format", format),
		slog.Int("bytes_out", len(content)),
	)
	return c.Blob(http.StatusOK, exporter.ContentType(), content)
}

func (h *Handler) exportEntryMarkdown(c echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return httpx.ErrorWithKind(c, http.StatusUnauthorized, auth.ErrUnauthorized.Error(), "auth.unauthorized")
	}

	entryID, err := strconv.ParseInt(c.Param("entryId"), 10, 64)
	if err != nil {
		return httpx.ErrorWithKind(c, http.StatusBadRequest, "IDを確認してください", "request.invalid_id")
	}

	content, exporter, entry, err := h.service.ExportEntryMarkdown(c.Request().Context(), userID, entryID)
	if errors.Is(err, entries.ErrNotFound) {
		return httpx.ErrorWithKind(c, http.StatusNotFound, entries.ErrNotFound.Error(), entries.KindFor(err))
	}
	if err != nil {
		return httpx.Internal(c, err)
	}

	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+exporter.FileName()+`"`)
	logx.Event(c, "export.entry_markdown.completed",
		slog.Int64("user_id", userID),
		slog.Int64("entry_id", entry.ID),
		slog.String("entry_date", entry.EntryDate),
		slog.Int("bytes_out", len(content)),
	)
	return c.Blob(http.StatusOK, exporter.ContentType(), content)
}
