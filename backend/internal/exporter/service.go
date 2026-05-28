package exporter

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/entries"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/images"
)

const (
	MaxAppExportEntries = 5000
	MaxBackupEntries    = 5000
	MaxBackupImages     = 10000
)

var (
	ErrUnsupportedFormat = errors.New("check the export format")
	ErrExportTooLarge    = errors.New("too many items to export")
)

type EntryService interface {
	GetByID(ctx context.Context, userID int64, id int64) (entries.EntryResponse, error)
	Count(ctx context.Context, userID int64, filter entries.EntryFilter) (int, error)
	ListForExport(ctx context.Context, userID int64) ([]entries.EntryResponse, error)
}

type ImageReader interface {
	CountByEntryID(ctx context.Context, entryID int64) (int, error)
	ListByEntryID(ctx context.Context, entryID int64) ([]images.Row, error)
}

type Service struct {
	entries EntryService
	images  ImageReader
}

func NewService(entries EntryService, imageReaders ...ImageReader) *Service {
	var reader ImageReader
	if len(imageReaders) > 0 {
		reader = imageReaders[0]
	}
	return &Service{entries: entries, images: reader}
}

func (s *Service) Export(ctx context.Context, userID int64, format string) ([]byte, Exporter, error) {
	if format == "backup" {
		if s.images == nil {
			return nil, nil, ErrUnsupportedFormat
		}
		content, err := s.backup(ctx, userID)
		return content, BackupExporter{}, err
	}

	exporter, ok := Exporters[format]
	if !ok {
		return nil, nil, ErrUnsupportedFormat
	}

	count, err := s.entries.Count(ctx, userID, entries.EntryFilter{})
	if err != nil {
		return nil, nil, err
	}
	if count > MaxAppExportEntries {
		return nil, nil, ErrExportTooLarge
	}

	items, err := s.entries.ListForExport(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	content, err := exporter.Export(items)
	return content, exporter, err
}

func (s *Service) ExportEntryMarkdown(ctx context.Context, userID int64, entryID int64) ([]byte, Exporter, entries.EntryResponse, error) {
	item, err := s.entries.GetByID(ctx, userID, entryID)
	if err != nil {
		return nil, nil, entries.EntryResponse{}, err
	}

	exporter := EntryMarkdownExporter{EntryDate: item.EntryDate}
	content, err := exporter.Export([]entries.EntryResponse{item})
	return content, exporter, item, err
}

type BackupManifest struct {
	CreatedAt string `json:"createdAt"`
	Format    string `json:"format"`
	Entries   int    `json:"entries"`
	Images    int    `json:"images"`
}

func (s *Service) backup(ctx context.Context, userID int64) ([]byte, error) {
	count, err := s.entries.Count(ctx, userID, entries.EntryFilter{})
	if err != nil {
		return nil, err
	}
	if count > MaxBackupEntries {
		return nil, ErrExportTooLarge
	}

	items, err := s.entries.ListForExport(ctx, userID)
	if err != nil {
		return nil, err
	}

	imageCount, err := s.countBackupImages(ctx, items)
	if err != nil {
		return nil, err
	}
	// Count images before opening the zip so oversized backups fail without
	// partially streaming or reading image files.
	if imageCount > MaxBackupImages {
		return nil, ErrExportTooLarge
	}

	buffer := bytes.NewBuffer(nil)
	archive := zip.NewWriter(buffer)

	if err := writeJSONFile(archive, "entries.json", items); err != nil {
		_ = archive.Close()
		return nil, err
	}

	for _, item := range items {
		rows, err := s.images.ListByEntryID(ctx, item.ID)
		if err != nil {
			_ = archive.Close()
			return nil, err
		}
		for _, row := range rows {
			if err := writeImageFile(archive, row); err != nil {
				_ = archive.Close()
				return nil, err
			}
		}
	}

	manifest := BackupManifest{
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Format:    "nikki-backup-v1",
		Entries:   len(items),
		Images:    imageCount,
	}
	if err := writeJSONFile(archive, "manifest.json", manifest); err != nil {
		_ = archive.Close()
		return nil, err
	}
	if err := writeTextFile(archive, "RESTORE.md", restoreInstructions); err != nil {
		_ = archive.Close()
		return nil, err
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (s *Service) countBackupImages(ctx context.Context, items []entries.EntryResponse) (int, error) {
	total := 0
	for _, item := range items {
		count, err := s.images.CountByEntryID(ctx, item.ID)
		if err != nil {
			return 0, err
		}
		total += count
		if total > MaxBackupImages {
			return total, nil
		}
	}
	return total, nil
}

func writeJSONFile(archive *zip.Writer, name string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeTextFile(archive, name, string(encoded)+"\n")
}

func writeTextFile(archive *zip.Writer, name string, value string) error {
	file, err := archive.Create(name)
	if err != nil {
		return err
	}
	_, err = file.Write([]byte(value))
	return err
}

func writeImageFile(archive *zip.Writer, row images.Row) error {
	content, err := os.ReadFile(row.FilePath)
	if err != nil {
		return fmt.Errorf("read image %d: %w", row.ID, err)
	}
	// Store images by the generated basename only; absolute upload paths stay out
	// of portable backup archives.
	name := filepath.Base(row.FilePath)
	file, err := archive.Create("images/" + name)
	if err != nil {
		return err
	}
	_, err = file.Write(content)
	return err
}

const restoreInstructions = `# Restore Nikki Backup

This archive is a portable content backup. It contains diary entries, image metadata embedded in entries.json, image files, and a manifest.

For operational recovery of a self-hosted Nikki instance, use a Nikki operational backup archive created from the PostgreSQL custom-format dump and matching uploads archive:

1. Start a new empty Nikki instance with a configured setup token.
2. Open /setup and choose Restore from backup.
3. Upload the trusted operational backup archive.
4. Confirm entries, dates, moods, tags, and images are visible after login.

The entries.json file is intended for inspection and future import tooling. Direct database restore from this archive is not yet automated.
`
