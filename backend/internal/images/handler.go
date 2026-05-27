package images

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/auth"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/httpx"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
)

const maxImageRequestBytes = MaxImageFileBytes + (1 << 20)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(api *echo.Group) {
	api.POST("/entries/:id/images", h.upload)
	api.GET("/images/:id/content", h.content)
	api.DELETE("/images/:id", h.delete)
}

func (h *Handler) RegisterUploads(uploads *echo.Group) {
	uploads.GET("/:name", h.legacyContent)
}

func (h *Handler) upload(c echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return httpx.ErrorWithKind(c, http.StatusUnauthorized, auth.ErrUnauthorized.Error(), "auth.unauthorized")
	}

	entryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return httpx.ErrorWithKind(c, http.StatusBadRequest, "check the ID", "request.invalid_id")
	}

	request := c.Request()
	// Keep the transport limit slightly above one image to account for multipart
	// overhead while still failing oversized bodies before parsing them.
	request.Body = http.MaxBytesReader(c.Response().Writer, request.Body, maxImageRequestBytes)
	if err := request.ParseMultipartForm(12 << 20); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return httpx.ErrorWithKind(c, http.StatusRequestEntityTooLarge, ErrImageTooLarge.Error(), "images.too_large")
		}
		return httpx.ErrorWithKind(c, http.StatusBadRequest, ErrInvalidImage.Error(), "images.invalid_image")
	}

	files := request.MultipartForm.File["images"]
	if len(files) == 0 {
		files = request.MultipartForm.File["image"]
	}

	images, err := h.service.SaveMany(request.Context(), userID, entryID, files)
	if err != nil {
		return imageError(c, err)
	}

	logx.Event(c, "images.uploaded",
		slog.Int64("user_id", userID),
		slog.Int64("entry_id", entryID),
		slog.Int("count", len(images)),
	)
	return httpx.JSON(c, http.StatusCreated, images)
}

func (h *Handler) delete(c echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return httpx.ErrorWithKind(c, http.StatusUnauthorized, auth.ErrUnauthorized.Error(), "auth.unauthorized")
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return httpx.ErrorWithKind(c, http.StatusBadRequest, "check the ID", "request.invalid_id")
	}

	if err := h.service.Delete(c.Request().Context(), userID, id); err != nil {
		return imageError(c, err)
	}

	logx.Event(c, "images.deleted", slog.Int64("user_id", userID), slog.Int64("image_id", id))
	return httpx.NoContent(c)
}

func (h *Handler) content(c echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return httpx.ErrorWithKind(c, http.StatusUnauthorized, auth.ErrUnauthorized.Error(), "auth.unauthorized")
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		return httpx.ErrorWithKind(c, http.StatusBadRequest, "check the image ID", "images.invalid_image_id")
	}

	row, err := h.service.Content(c.Request().Context(), userID, id)
	if err != nil {
		return imageError(c, err)
	}
	return serveContent(c, row)
}

func (h *Handler) legacyContent(c echo.Context) error {
	userID, ok := auth.UserID(c)
	if !ok {
		return httpx.ErrorWithKind(c, http.StatusUnauthorized, auth.ErrUnauthorized.Error(), "auth.unauthorized")
	}

	name := filepath.Base(c.Param("name"))
	// Legacy /uploads URLs are still authenticated and resolved through the DB;
	// reject path tricks before translating the name to a public_url lookup.
	if name == "." || name != c.Param("name") || strings.ContainsAny(name, `/\`) {
		return echo.ErrNotFound
	}

	row, err := h.service.ContentByPublicURL(c.Request().Context(), userID, "/uploads/"+name)
	if err != nil {
		return imageError(c, err)
	}
	return serveContent(c, row)
}

func serveContent(c echo.Context, row Row) error {
	file, err := os.Open(row.FilePath)
	if errors.Is(err, os.ErrNotExist) {
		return imageError(c, ErrImageNotFound)
	}
	if err != nil {
		return httpx.Internal(c, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if errors.Is(err, os.ErrNotExist) {
		return imageError(c, ErrImageNotFound)
	}
	if err != nil {
		return httpx.Internal(c, err)
	}
	if info.IsDir() {
		return imageError(c, ErrImageNotFound)
	}

	response := c.Response()
	response.Header().Set(echo.HeaderContentType, row.MimeType)
	response.Header().Set(echo.HeaderCacheControl, "private, max-age=3600")
	http.ServeContent(response, c.Request(), row.FileName, info.ModTime(), file)
	return nil
}

func imageError(c echo.Context, err error) error {
	status := StatusFor(err)
	if status >= http.StatusInternalServerError {
		return httpx.Internal(c, err)
	}
	return httpx.ErrorWithKind(c, status, err.Error(), KindFor(err))
}
