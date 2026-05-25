package images

import (
	"context"
	"database/sql/driver"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupDryRunReportsMismatchesWithoutDeleting(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.jpg")
	orphan := filepath.Join(dir, "orphan.jpg")
	if err := os.WriteFile(valid, []byte("valid"), 0644); err != nil {
		t.Fatalf("write valid image: %v", err)
	}
	if err := os.WriteFile(orphan, []byte("orphan"), 0644); err != nil {
		t.Fatalf("write orphan image: %v", err)
	}

	database := openScriptDB(t, &scriptDB{
		queries: []queryResult{
			rowsResult(imageColumns, [][]driver.Value{
				imageRowValues(1, 10, valid, "/uploads/valid.jpg", "valid.jpg"),
				imageRowValues(2, 11, filepath.Join(dir, "missing.jpg"), "/uploads/missing.jpg", "missing.jpg"),
				imageRowValues(3, 12, filepath.Join(dir, "missing-entry.jpg"), "/uploads/missing-entry.jpg", "missing-entry.jpg"),
			}),
			rowsResult([]string{"exists"}, [][]driver.Value{{int64(1)}}),
			rowsResult([]string{"exists"}, [][]driver.Value{{int64(1)}}),
			rowsResult([]string{"exists"}, nil),
		},
	})
	defer database.Close()

	report, err := NewService(NewRepository(database), NewLocalStorage(dir, "/uploads"), nil).Cleanup(context.Background(), true)
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if len(report.FilesWithoutRows) != 1 || report.FilesWithoutRows[0] != orphan {
		t.Fatalf("FilesWithoutRows = %#v", report.FilesWithoutRows)
	}
	if len(report.RowsWithoutFiles) != 2 {
		t.Fatalf("RowsWithoutFiles = %#v", report.RowsWithoutFiles)
	}
	if len(report.RowsWithMissingEntries) != 1 || report.RowsWithMissingEntries[0].ID != 3 {
		t.Fatalf("RowsWithMissingEntries = %#v", report.RowsWithMissingEntries)
	}
	if _, err := os.Stat(orphan); err != nil {
		t.Fatalf("dry run deleted orphan file: %v", err)
	}
}

func TestCleanupDeletesOnlyDestructiveCandidates(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.jpg")
	orphan := filepath.Join(dir, "orphan.jpg")
	if err := os.WriteFile(valid, []byte("valid"), 0644); err != nil {
		t.Fatalf("write valid image: %v", err)
	}
	if err := os.WriteFile(orphan, []byte("orphan"), 0644); err != nil {
		t.Fatalf("write orphan image: %v", err)
	}

	database := openScriptDB(t, &scriptDB{
		queries: []queryResult{
			rowsResult(imageColumns, [][]driver.Value{
				imageRowValues(1, 10, valid, "/uploads/valid.jpg", "valid.jpg"),
				imageRowValues(2, 11, filepath.Join(dir, "missing.jpg"), "/uploads/missing.jpg", "missing.jpg"),
				imageRowValues(3, 12, filepath.Join(dir, "missing-entry.jpg"), "/uploads/missing-entry.jpg", "missing-entry.jpg"),
			}),
			rowsResult([]string{"exists"}, [][]driver.Value{{int64(1)}}),
			rowsResult([]string{"exists"}, [][]driver.Value{{int64(1)}}),
			rowsResult([]string{"exists"}, nil),
		},
		execs: []execResult{{affected: 1}},
	})
	defer database.Close()

	report, err := NewService(NewRepository(database), NewLocalStorage(dir, "/uploads"), nil).Cleanup(context.Background(), false)
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if len(report.FilesWithoutRows) != 1 || len(report.RowsWithMissingEntries) != 1 || len(report.RowsWithoutFiles) != 2 {
		t.Fatalf("report = %#v", report)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("orphan file still exists, err = %v", err)
	}
	if _, err := os.Stat(valid); err != nil {
		t.Fatalf("valid file was removed: %v", err)
	}
}
