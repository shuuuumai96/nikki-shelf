package stats

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/auth"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/httpx"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(api *echo.Group) {
	api.GET("/stats", h.get)
}

func (h *Handler) get(c echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return httpx.ErrorWithKind(c, http.StatusUnauthorized, auth.ErrUnauthorized.Error(), "auth.unauthorized")
	}

	stats, err := h.service.Get(c.Request().Context(), userID)
	if err != nil {
		return httpx.Internal(c, err)
	}

	return httpx.JSON(c, http.StatusOK, stats)
}
