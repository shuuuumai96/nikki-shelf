package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/auth"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/db"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/entries"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/exporter"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/images"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/stats"
)

func Run(ctx context.Context) error {
	cfg := LoadConfig()
	logger := logx.New(logx.Config{Level: cfg.LogLevel, Format: cfg.LogFormat})
	slog.SetDefault(logger)

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		logger.ErrorContext(ctx, "database connect failed", slog.String("component", "db"), slog.String("operation", "connect"), slog.String("error", err.Error()))
		return err
	}
	defer database.Close()
	logger.InfoContext(ctx, "database connected", slog.String("component", "db"))

	if err := db.Migrate(database); err != nil {
		logger.ErrorContext(ctx, "database migration failed", slog.String("component", "db"), slog.String("operation", "migrate"), slog.String("error", err.Error()))
		return err
	}
	logger.InfoContext(ctx, "database migrated", slog.String("component", "db"))

	authRepo := auth.NewRepository(database)
	entryRepo := entries.NewRepository(database)
	imageRepo := images.NewRepository(database)
	imageStorage := images.NewLocalStorage(cfg.UploadDir, cfg.PublicUploadBase, images.StorageConfig{
		StripMetadata: cfg.StripImageMetadata,
	})

	authService := auth.NewService(authRepo, auth.ServiceConfig{
		AllowAdditionalSignups:  cfg.AllowAdditionalSignups,
		AllowFirstUserSetup:     cfg.AllowFirstUserSetup,
		FirstUserBootstrapToken: cfg.FirstUserBootstrapToken,
		DatabaseURL:             cfg.DatabaseURL,
		UploadDir:               cfg.UploadDir,
		AccountFiles:            imageStorage,
	})
	imageService := images.NewService(imageRepo, imageStorage, entryRepo, images.ServiceConfig{
		Quota: images.QuotaConfig{
			UserBytes:  cfg.ImageUserQuotaBytes,
			UserCount:  cfg.ImageUserQuotaCount,
			TotalBytes: cfg.ImageTotalQuotaBytes,
		},
	})
	entryService := entries.NewService(entryRepo, imageService, imageService)
	statsService := stats.NewService(entryRepo)
	exportService := exporter.NewService(entryService, imageRepo)

	server := echo.New()
	server.HideBanner = true
	server.HidePort = true
	server.Use(logx.RequestIDMiddleware())
	server.Use(logx.Middleware(logger))
	server.Use(logx.Recover(logger))
	if len(cfg.CORSAllowedOrigins) > 0 {
		server.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: cfg.CORSAllowedOrigins,
			AllowHeaders: []string{echo.HeaderContentType, "X-CSRF-Token"},
			AllowMethods: []string{
				http.MethodGet,
				http.MethodPost,
				http.MethodPut,
				http.MethodDelete,
				http.MethodOptions,
			},
		}))
	}

	api := server.Group("/api")
	api.GET("/health", health)
	auth.NewHandler(authService, auth.HandlerConfig{
		CookieSecure: cfg.CookieSecure,
		RateLimiter: auth.NewRateLimiter(auth.RateLimiterConfig{
			IPAttempts:      cfg.AuthRateLimitIPAttempts,
			AccountAttempts: cfg.AuthRateLimitAccountAttempts,
			Window:          cfg.AuthRateLimitWindow,
			Lockout:         cfg.AuthRateLimitLockout,
			MaxEntries:      cfg.AuthRateLimitMaxEntries,
			Extractor:       auth.NewClientIPExtractor(cfg.IPExtractorMode, cfg.TrustedProxyCIDRs),
		}),
	}).Register(api)

	protected := api.Group("")
	protected.Use(auth.Require(authService))
	protected.Use(auth.CSRF(authService))
	imageHandler := images.NewHandler(imageService)
	// Legacy /uploads URLs stay behind auth; the handler resolves each name
	// through stored image metadata so ownership checks remain centralized.
	uploads := server.Group("/uploads", auth.Require(authService))
	imageHandler.RegisterUploads(uploads)

	entries.NewHandler(entryService).Register(protected)
	imageHandler.Register(protected)
	stats.NewHandler(statsService).Register(protected)
	exporter.NewHandler(exportService).Register(protected)

	logger.InfoContext(ctx, "server starting", slog.String("addr", cfg.Addr), slog.String("upload_dir", cfg.UploadDir))
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.ErrorContext(shutdownCtx, "server shutdown failed", slog.String("error", err.Error()))
		}
	}()

	err = server.Start(cfg.Addr)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
