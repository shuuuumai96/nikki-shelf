package entries

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/auth"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/httpx"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/moods"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(api *echo.Group) {
	api.GET("/entries", h.list)
	api.GET("/entries/search", h.search)
	api.POST("/entries", h.create)
	api.GET("/entries/:id", h.get)
	api.PUT("/entries/:id", h.update)
	api.DELETE("/entries/:id", h.delete)
	api.GET("/entries/date/:date", h.getByDate)
	api.GET("/tags", h.tags)
	api.GET("/moods", h.moods)
}

func (h *Handler) list(c echo.Context) error {
	userID, ok := userID(c)
	if !ok {
		return unauthorized(c)
	}

	perPage, err := normalizePerPage(c.QueryParam("per_page"))
	if err != nil {
		return entryError(c, err)
	}

	page, err := h.service.ListPage(c.Request().Context(), userID, EntryPageRequest{
		Filter: EntryFilter{
			Query: c.QueryParam("query"),
			Tag:   c.QueryParam("tag"),
			Mood:  c.QueryParam("mood"),
			From:  c.QueryParam("from"),
			To:    c.QueryParam("to"),
		},
		PerPage: perPage,
		Cursor:  c.QueryParam("cursor"),
	})
	if err != nil {
		return entryError(c, err)
	}
	if page.HasMore {
		c.Response().Header().Set("Link", nextLink(c, page.NextCursor, perPage))
	}

	return httpx.JSON(c, http.StatusOK, page)
}

func (h *Handler) search(c echo.Context) error {
	userID, ok := userID(c)
	if !ok {
		return unauthorized(c)
	}

	results, err := h.service.Search(c.Request().Context(), userID, SearchFilter{
		Query:    c.QueryParam("q"),
		From:     c.QueryParam("from"),
		To:       c.QueryParam("to"),
		Mood:     c.QueryParam("mood"),
		Tag:      c.QueryParam("tag"),
		HasImage: c.QueryParam("hasImage"),
		Limit:    queryInt(c.QueryParam("limit"), 0),
		Offset:   queryInt(c.QueryParam("offset"), 0),
	})
	if err != nil {
		return entryError(c, err)
	}

	return httpx.JSON(c, http.StatusOK, results)
}

func (h *Handler) create(c echo.Context) error {
	userID, ok := userID(c)
	if !ok {
		return unauthorized(c)
	}

	input := CreateInput{}
	if err := decodeEntryJSON(c, &input); err != nil {
		return entryJSONError(c, err)
	}

	entry, err := h.service.Create(c.Request().Context(), userID, input)
	if err != nil {
		return entryError(c, err)
	}

	logx.Event(c, "entries.created",
		slog.Int64("user_id", userID),
		slog.Int64("entry_id", entry.ID),
		slog.String("entry_date", entry.EntryDate),
	)
	return httpx.JSON(c, http.StatusCreated, entry)
}

func (h *Handler) get(c echo.Context) error {
	userID, ok := userID(c)
	if !ok {
		return unauthorized(c)
	}

	id, ok := entryID(c)
	if !ok {
		return badID(c)
	}

	entry, err := h.service.GetByID(c.Request().Context(), userID, id)
	if err != nil {
		return entryError(c, err)
	}

	return httpx.JSON(c, http.StatusOK, entry)
}

func (h *Handler) getByDate(c echo.Context) error {
	userID, ok := userID(c)
	if !ok {
		return unauthorized(c)
	}

	date := c.Param("date")
	entry, err := h.service.GetByDate(c.Request().Context(), userID, date)
	if errors.Is(err, ErrNotFound) {
		return httpx.JSON(c, http.StatusOK, EntryDateLookupResponse{
			Entry:  nil,
			Date:   date,
			Exists: false,
		})
	}
	if err != nil {
		return entryError(c, err)
	}

	return httpx.JSON(c, http.StatusOK, EntryDateLookupResponse{
		Entry:  &entry,
		Date:   entry.EntryDate,
		Exists: true,
	})
}

func (h *Handler) update(c echo.Context) error {
	userID, ok := userID(c)
	if !ok {
		return unauthorized(c)
	}

	id, ok := entryID(c)
	if !ok {
		return badID(c)
	}

	input := UpdateInput{}
	if err := decodeEntryJSON(c, &input); err != nil {
		return entryJSONError(c, err)
	}

	entry, err := h.service.Update(c.Request().Context(), userID, id, input)
	if err != nil {
		return entryError(c, err)
	}

	logx.Event(c, "entries.updated",
		slog.Int64("user_id", userID),
		slog.Int64("entry_id", entry.ID),
		slog.String("entry_date", entry.EntryDate),
	)
	return httpx.JSON(c, http.StatusOK, entry)
}

func (h *Handler) delete(c echo.Context) error {
	userID, ok := userID(c)
	if !ok {
		return unauthorized(c)
	}

	id, ok := entryID(c)
	if !ok {
		return badID(c)
	}

	if err := h.service.Delete(c.Request().Context(), userID, id); err != nil {
		return entryError(c, err)
	}

	logx.Event(c, "entries.deleted", slog.Int64("user_id", userID), slog.Int64("entry_id", id))
	return httpx.NoContent(c)
}

func (h *Handler) tags(c echo.Context) error {
	userID, ok := userID(c)
	if !ok {
		return unauthorized(c)
	}

	tags, err := h.service.Tags(c.Request().Context(), userID)
	if err != nil {
		return entryError(c, err)
	}

	return httpx.JSON(c, http.StatusOK, tags)
}

func (h *Handler) moods(c echo.Context) error {
	return httpx.JSON(c, http.StatusOK, moods.List())
}

func parseID(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

func queryInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func nextLink(c echo.Context, cursor string, perPage int) string {
	values := c.QueryParams()
	values.Set("per_page", strconv.Itoa(perPage))
	values.Set("cursor", cursor)
	return "<" + c.Request().URL.Path + "?" + values.Encode() + `>; rel="next"`
}

func userID(c echo.Context) (int64, bool) {
	userID, ok := auth.UserID(c)
	if !ok {
		return 0, false
	}
	return userID, true
}

func entryID(c echo.Context) (int64, bool) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		return 0, false
	}
	return id, true
}

func decodeEntryJSON(c echo.Context, input any) error {
	return httpx.DecodeJSONWithLimit(c, input, httpx.EntryJSONLimitBytes)
}

func entryError(c echo.Context, err error) error {
	status := StatusFor(err)
	if status >= http.StatusInternalServerError {
		return httpx.Internal(c, err)
	}
	return httpx.ErrorWithKind(c, status, err.Error(), KindFor(err))
}

func unauthorized(c echo.Context) error {
	return httpx.ErrorWithKind(c, http.StatusUnauthorized, auth.ErrUnauthorized.Error(), "auth.unauthorized")
}

func badID(c echo.Context) error {
	return httpx.ErrorWithKind(c, http.StatusBadRequest, "IDを確認してください", "request.invalid_id")
}

func badJSON(c echo.Context) error {
	return httpx.ErrorWithKind(c, http.StatusBadRequest, "JSONを確認してください", "request.invalid_json")
}

func entryJSONError(c echo.Context, err error) error {
	if errors.Is(err, httpx.ErrRequestTooLarge) {
		return httpx.ErrorWithKind(c, http.StatusRequestEntityTooLarge, "JSONが大きすぎます", "request.too_large")
	}
	return badJSON(c)
}
