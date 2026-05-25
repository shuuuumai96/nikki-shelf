package images

import (
	"context"
	"errors"
	"log/slog"
	"mime/multipart"
	"path/filepath"
	"time"
)

const (
	MaxImageFileBytes     int64 = 8 << 20
	MaxImagesPerEntry           = 3
	DefaultUserQuotaBytes int64 = 1 << 30
	DefaultUserQuotaCount       = 1000
)

type QuotaConfig struct {
	UserBytes  int64
	UserCount  int
	TotalBytes int64
}

type ServiceConfig struct {
	Quota QuotaConfig
}

type EntryReader interface {
	ExistsForUser(ctx context.Context, userID int64, entryID int64) (bool, error)
}

type Service struct {
	repo    *Repository
	storage Storage
	entries EntryReader
	now     func() time.Time
	quota   QuotaConfig
}

func NewService(repo *Repository, storage Storage, entries EntryReader, configs ...ServiceConfig) *Service {
	config := ServiceConfig{Quota: defaultQuotaConfig()}
	if len(configs) > 0 {
		config = configs[0]
	}
	return &Service{repo: repo, storage: storage, entries: entries, now: time.Now, quota: config.Quota}
}

func defaultQuotaConfig() QuotaConfig {
	return QuotaConfig{
		UserBytes:  DefaultUserQuotaBytes,
		UserCount:  DefaultUserQuotaCount,
		TotalBytes: 0,
	}
}

func (s *Service) SaveMany(ctx context.Context, userID int64, entryID int64, headers []*multipart.FileHeader) ([]Response, error) {
	if len(headers) == 0 {
		return nil, ErrInvalidImage
	}
	if len(headers) > MaxImagesPerEntry {
		return nil, ErrTooManyImages
	}

	if err := s.ensureEntry(ctx, userID, entryID); err != nil {
		return nil, err
	}

	storedFiles := make([]StoredFile, 0, len(headers))
	for _, header := range headers {
		if header.Size > MaxImageFileBytes {
			s.deleteStoredFiles(ctx, storedFiles)
			return nil, ErrImageTooLarge
		}

		stored, err := s.storage.Save(ctx, entryID, header)
		if err != nil {
			s.deleteStoredFiles(ctx, storedFiles)
			return nil, err
		}
		storedFiles = append(storedFiles, stored)
	}

	rows, err := s.repo.CreateForUser(ctx, userID, entryID, storedFiles, s.now(), s.quota)
	if err != nil {
		s.deleteStoredFiles(ctx, storedFiles)
		return nil, err
	}

	responses := make([]Response, 0, len(rows))
	for _, row := range rows {
		responses = append(responses, response(row))
	}

	return responses, nil
}

func (s *Service) Delete(ctx context.Context, userID int64, id int64) error {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.ensureEntry(ctx, userID, row.EntryID); err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	if err := s.storage.Delete(ctx, row.FilePath); err != nil {
		slog.ErrorContext(ctx, "image file deletion failed", slog.Int64("image_id", row.ID), slog.String("file_path", row.FilePath), slog.String("error", err.Error()))
		return err
	}
	return nil
}

func (s *Service) Content(ctx context.Context, userID int64, id int64) (Row, error) {
	return s.repo.GetOwnedByID(ctx, id, userID)
}

func (s *Service) ContentByPublicURL(ctx context.Context, userID int64, publicURL string) (Row, error) {
	return s.repo.GetOwnedByPublicURL(ctx, publicURL, userID)
}

func (s *Service) ListByEntryID(ctx context.Context, entryID int64) ([]Row, error) {
	return s.repo.ListByEntryID(ctx, entryID)
}

func (s *Service) DeleteFiles(ctx context.Context, rows []Row) {
	for _, row := range rows {
		if err := s.storage.Delete(ctx, row.FilePath); err != nil {
			slog.ErrorContext(ctx, "image file deletion failed", slog.Int64("image_id", row.ID), slog.String("file_path", row.FilePath), slog.String("error", err.Error()))
		}
	}
}

func (s *Service) deleteStoredFiles(ctx context.Context, files []StoredFile) {
	for _, file := range files {
		if err := s.storage.Delete(ctx, file.FilePath); err != nil {
			slog.ErrorContext(ctx, "uploaded image cleanup failed", slog.String("error", err.Error()))
		}
	}
}

type CleanupReport struct {
	FilesWithoutRows       []string `json:"filesWithoutRows"`
	RowsWithoutFiles       []Row    `json:"rowsWithoutFiles"`
	RowsWithMissingEntries []Row    `json:"rowsWithMissingEntries"`
}

func (s *Service) Cleanup(ctx context.Context, dryRun bool) (CleanupReport, error) {
	report := CleanupReport{}
	rows, err := s.repo.ListAll(ctx)
	if err != nil {
		return report, err
	}

	fileRows := map[string]Row{}
	for _, row := range rows {
		fileRows[filepath.Clean(row.FilePath)] = row
		if local, ok := s.storage.(*LocalStorage); ok && !local.Exists(row.FilePath) {
			report.RowsWithoutFiles = append(report.RowsWithoutFiles, row)
		}
		exists, err := s.repo.ExistsEntry(ctx, row.EntryID)
		if err != nil {
			return report, err
		}
		if !exists {
			report.RowsWithMissingEntries = append(report.RowsWithMissingEntries, row)
		}
	}

	if local, ok := s.storage.(*LocalStorage); ok {
		files, err := local.ListFiles()
		if err != nil {
			return report, err
		}
		for _, file := range files {
			if _, ok := fileRows[filepath.Clean(file)]; !ok {
				report.FilesWithoutRows = append(report.FilesWithoutRows, file)
				if !dryRun {
					if err := s.storage.Delete(ctx, file); err != nil {
						slog.ErrorContext(ctx, "orphan image file deletion failed", slog.String("file_path", file), slog.String("error", err.Error()))
					}
				}
			}
		}
	}

	if !dryRun {
		for _, row := range report.RowsWithMissingEntries {
			if err := s.repo.Delete(ctx, row.ID); err != nil {
				slog.ErrorContext(ctx, "orphan image row deletion failed", slog.Int64("image_id", row.ID), slog.String("error", err.Error()))
			}
		}
	}

	return report, nil
}

func (s *Service) ensureEntry(ctx context.Context, userID int64, entryID int64) error {
	exists, err := s.entries.ExistsForUser(ctx, userID, entryID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrImageNotFound
	}
	return nil
}

func response(row Row) Response {
	return Response{
		ID:        row.ID,
		EntryID:   row.EntryID,
		URL:       ContentURL(row.ID),
		FileName:  row.FileName,
		Size:      row.Size,
		MimeType:  row.MimeType,
		CreatedAt: row.CreatedAt,
	}
}

var errorSpecs = []struct {
	target error
	status int
	kind   string
}{
	{ErrInvalidImage, 400, "images.invalid_image"},
	{ErrImageTooLarge, 413, "images.too_large"},
	{ErrImageQuotaExceeded, 413, "images.quota_exceeded"},
	{ErrTooManyImages, 400, "images.too_many"},
	{ErrImageNotFound, 404, "images.not_found"},
}

func StatusFor(err error) int {
	for _, item := range errorSpecs {
		if errors.Is(err, item.target) {
			return item.status
		}
	}

	return 500
}

func KindFor(err error) string {
	for _, item := range errorSpecs {
		if errors.Is(err, item.target) {
			return item.kind
		}
	}

	return "server.internal"
}
