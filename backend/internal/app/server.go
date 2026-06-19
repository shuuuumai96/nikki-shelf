package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/audit"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/auth"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/db"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/entries"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/exporter"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/httpx"
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

	clientIPExtractor := auth.NewClientIPExtractor(cfg.IPExtractorMode, cfg.TrustedProxyCIDRs)
	auditRepo := audit.NewRepository(database)
	auditService := audit.NewService(auditRepo, audit.Config{
		RetentionDays: cfg.AuditRetentionDays,
		RemoteIP:      clientIPExtractor.ClientIP,
	})
	if deleted, err := auditService.PruneExpired(ctx); err != nil {
		logger.ErrorContext(ctx, "audit retention cleanup failed", slog.String("component", "audit"), slog.String("operation", "prune"), slog.String("error", err.Error()))
	} else if deleted > 0 {
		logger.InfoContext(ctx, "audit retention cleanup completed", slog.String("component", "audit"), slog.Int64("deleted", deleted))
	}

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
	server.Use(logx.MiddlewareWithRemoteIP(logger, clientIPExtractor.ClientIP))
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
		Audit:        auditService,
		ClientIP:     clientIPExtractor.ClientIP,
		RateLimiter: auth.NewRateLimiter(auth.RateLimiterConfig{
			IPAttempts:      cfg.AuthRateLimitIPAttempts,
			AccountAttempts: cfg.AuthRateLimitAccountAttempts,
			Window:          cfg.AuthRateLimitWindow,
			Lockout:         cfg.AuthRateLimitLockout,
			MaxEntries:      cfg.AuthRateLimitMaxEntries,
			Extractor:       clientIPExtractor,
		}),
	}).Register(api)

	protected := api.Group("")
	protected.Use(auth.Require(authService))
	protected.Use(auth.CSRF(authService, auth.CSRFConfig{Audit: auditService}))
	imageHandler := images.NewHandler(imageService, images.HandlerConfig{Audit: auditService})
	// Legacy /uploads URLs stay behind auth; the handler resolves each name
	// through stored image metadata so ownership checks remain centralized.
	uploads := server.Group("/uploads", auth.Require(authService))
	imageHandler.RegisterUploads(uploads)

	audit.NewHandler(auditService).Register(protected.Group("/audit", requireOwner))
	entries.NewHandler(entryService, entries.HandlerConfig{Audit: auditService}).Register(protected)
	imageHandler.Register(protected)
	stats.NewHandler(statsService).Register(protected)
	exporter.NewHandler(exportService, exporter.HandlerConfig{Audit: auditService}).Register(protected)

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

func requireOwner(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, ok := auth.UserFromContext(c)
		if !ok {
			return httpx.ErrorWithKind(c, http.StatusUnauthorized, auth.ErrUnauthorized.Error(), "auth.unauthorized")
		}
		if user.Role != auth.RoleOwner {
			return httpx.ErrorWithKind(c, http.StatusForbidden, "owner account required", "auth.owner_required")
		}
		return next(c)
	}
}
