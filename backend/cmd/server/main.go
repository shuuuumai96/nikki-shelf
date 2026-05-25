package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/app"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "cleanup-images" {
		dryRun := len(os.Args) >= 3 && os.Args[2] == "--dry-run"
		if err := app.CleanupImages(context.Background(), dryRun); err != nil {
			slog.Error("cleanup failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "seed-dev-data" {
		if err := app.SeedDevData(context.Background(), os.Args[2:]); err != nil {
			slog.Error("seed dev data failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil {
		slog.Error("server stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
