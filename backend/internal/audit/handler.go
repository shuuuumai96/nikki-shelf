package audit

import (
	"context"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/httpx"
)

type listService interface {
	List(ctx context.Context, limit int) ([]Event, error)
}

type Handler struct {
	service listService
}

func NewHandler(service listService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(api *echo.Group) {
	api.GET("/events", h.list)
}

func (h *Handler) list(c echo.Context) error {
	events, err := h.service.List(c.Request().Context(), queryLimit(c.QueryParam("limit")))
	if err != nil {
		return httpx.Internal(c, err)
	}
	return httpx.JSON(c, http.StatusOK, ListResponse{Items: events})
}

func queryLimit(value string) int {
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0
		}
	}
	limit, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return limit
}
