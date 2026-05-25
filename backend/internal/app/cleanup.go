package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/db"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/images"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
)

func CleanupImages(ctx context.Context, dryRun bool) error {
	cfg := LoadConfig()
	logger := logx.New(logx.Config{Level: cfg.LogLevel, Format: cfg.LogFormat})
	slog.SetDefault(logger)

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		return err
	}

	repo := images.NewRepository(database)
	storage := images.NewLocalStorage(cfg.UploadDir, cfg.PublicUploadBase)
	service := images.NewService(repo, storage, nil)

	report, err := service.Cleanup(ctx, dryRun)
	if err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}
