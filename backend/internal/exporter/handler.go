package exporter

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/audit"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/auth"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/entries"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/httpx"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
)

type Handler struct {
	service *Service
	audit   *audit.Service
}

type HandlerConfig struct {
	Audit *audit.Service
}

func NewHandler(service *Service, configs ...HandlerConfig) *Handler {
	cfg := HandlerConfig{}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	return &Handler{service: service, audit: cfg.Audit}
}

func (h *Handler) Register(api *echo.Group) {
	api.GET("/export/:format", h.export)
	api.GET("/export/entries/:entryId/markdown", h.exportEntryMarkdown)
}

func (h *Handler) export(c echo.Context) error {
	user, ok := auth.UserFromContext(c)
	if !ok {
		return httpx.ErrorWithKind(c, http.StatusUnauthorized, auth.ErrUnauthorized.Error(), "auth.unauthorized")
	}
	userID := user.ID

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
	h.recordAudit(c, audit.Event{
		EventType:     "export.completed",
		Outcome:       audit.OutcomeSucceeded,
		ActorUserID:   audit.UserID(user.ID),
		ActorUsername: user.Username,
		ActorRole:     user.Role,
		Metadata:      audit.Metadata("format", format, "bytes_out", len(content)),
	})
	return c.Blob(http.StatusOK, exporter.ContentType(), content)
}

func (h *Handler) exportEntryMarkdown(c echo.Context) error {
	user, ok := auth.UserFromContext(c)
	if !ok {
		return httpx.ErrorWithKind(c, http.StatusUnauthorized, auth.ErrUnauthorized.Error(), "auth.unauthorized")
	}
	userID := user.ID

	entryID, err := strconv.ParseInt(c.Param("entryId"), 10, 64)
	if err != nil {
		return httpx.ErrorWithKind(c, http.StatusBadRequest, "check the ID", "request.invalid_id")
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
	h.recordAudit(c, audit.Event{
		EventType:     "export.entry_markdown.completed",
		Outcome:       audit.OutcomeSucceeded,
		ActorUserID:   audit.UserID(user.ID),
		ActorUsername: user.Username,
		ActorRole:     user.Role,
		TargetType:    "entry",
		TargetID:      strconv.FormatInt(entry.ID, 10),
		Metadata:      audit.Metadata("bytes_out", len(content)),
	})
	return c.Blob(http.StatusOK, exporter.ContentType(), content)
}

func (h *Handler) recordAudit(c echo.Context, event audit.Event) {
	if h.audit == nil {
		return
	}
	h.audit.RecordHTTP(c, event)
}
