package exporter

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/auth"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/entries"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/images"
)

type fakeEntryService struct {
	items         []entries.EntryResponse
	countOverride *int
	getByID       entries.EntryResponse
	getUserID     int64
	getID         int64
	getErr        error
}

func (f *fakeEntryService) GetByID(_ context.Context, userID int64, id int64) (entries.EntryResponse, error) {
	f.getUserID = userID
	f.getID = id
	if f.getErr != nil {
		return entries.EntryResponse{}, f.getErr
	}
	return f.getByID, nil
}

func (f *fakeEntryService) Count(context.Context, int64, entries.EntryFilter) (int, error) {
	if f.countOverride != nil {
		return *f.countOverride, nil
	}
	return len(f.items), nil
}

func (f *fakeEntryService) ListForExport(context.Context, int64) ([]entries.EntryResponse, error) {
	return f.items, nil
}

func TestServiceExportSelectsMarkdownExporter(t *testing.T) {
	service := NewService(&fakeEntryService{items: []entries.EntryResponse{
		{
			EntryDate: "2026-05-18",
			Title:     "A quiet day",
			Body:      "Body text",
			Mood:      "calm",
			Tags:      []string{"life", "work"},
		},
		{
			EntryDate: "2026-05-17",
			Body:      "Untitled body",
			Mood:      "happy",
		},
	}})

	content, selected, err := service.Export(context.Background(), 42, "markdown")
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if selected.ContentType() != "text/markdown; charset=utf-8" {
		t.Fatalf("ContentType() = %q", selected.ContentType())
	}
	if selected.FileName() != "nikki-export.md" {
		t.Fatalf("FileName() = %q", selected.FileName())
	}

	want := strings.Join([]string{
		"## A quiet day",
		"",
		"- Date: 2026-05-18",
		"- Mood: calm",
		"- Tags: life, work",
		"",
		"Body text",
		"",
		"## 2026-05-17",
		"",
		"- Date: 2026-05-17",
		"- Mood: happy",
		"",
		"Untitled body",
		"",
		"",
	}, "\n")
	if string(content) != want {
		t.Fatalf("markdown export = %q, want %q", string(content), want)
	}
}

func TestServiceExportRejectsUnsupportedFormat(t *testing.T) {
	service := NewService(&fakeEntryService{})

	content, selected, err := service.Export(context.Background(), 42, "csv")
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("Export() error = %v, want %v", err, ErrUnsupportedFormat)
	}
	if content != nil {
		t.Fatalf("content = %#v, want nil", content)
	}
	if selected != nil {
		t.Fatalf("selected = %#v, want nil", selected)
	}
}

func TestServiceExportRejectsTooManyEntries(t *testing.T) {
	count := MaxAppExportEntries + 1
	service := NewService(&fakeEntryService{countOverride: &count})

	content, selected, err := service.Export(context.Background(), 42, "json")
	if !errors.Is(err, ErrExportTooLarge) {
		t.Fatalf("Export() error = %v, want %v", err, ErrExportTooLarge)
	}
	if content != nil || selected != nil {
		t.Fatalf("content/selected = %#v/%#v, want nil/nil", content, selected)
	}
}

func TestJSONExporterPreservesResponseShape(t *testing.T) {
	content, err := JSONExporter{}.Export([]entries.EntryResponse{
		{
			ID:        7,
			EntryDate: "2026-05-18",
			Title:     "A quiet day",
			Body:      "Body text",
			Mood:      "calm",
			Tags:      []string{"life"},
			Images: []entries.EntryImage{
				{ID: 3, URL: "/api/images/3/content", FileName: "photo.jpg"},
			},
			CreatedAt: "2026-05-18T01:02:03Z",
			UpdatedAt: "2026-05-18T04:05:06Z",
		},
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	var decoded []map[string]any
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	item := decoded[0]
	if item["entryDate"] != "2026-05-18" {
		t.Fatalf("entryDate = %v", item["entryDate"])
	}
	images := item["images"].([]any)
	image := images[0].(map[string]any)
	if image["fileName"] != "photo.jpg" {
		t.Fatalf("fileName = %v", image["fileName"])
	}
}

type fakeImageService struct {
	rows          map[int64][]images.Row
	countOverride *int
}

func (f fakeImageService) CountByEntryID(_ context.Context, entryID int64) (int, error) {
	if f.countOverride != nil {
		return *f.countOverride, nil
	}
	return len(f.rows[entryID]), nil
}

func (f fakeImageService) ListByEntryID(_ context.Context, entryID int64) ([]images.Row, error) {
	return f.rows[entryID], nil
}

func TestBackupExporterRejectsTooManyEntries(t *testing.T) {
	count := MaxBackupEntries + 1
	service := NewService(&fakeEntryService{countOverride: &count}, fakeImageService{})

	content, _, err := service.Export(context.Background(), 42, "backup")
	if !errors.Is(err, ErrExportTooLarge) {
		t.Fatalf("Export() error = %v, want %v", err, ErrExportTooLarge)
	}
	if content != nil {
		t.Fatalf("content = %#v, want nil", content)
	}
}

func TestBackupExporterRejectsTooManyImages(t *testing.T) {
	imageCount := MaxBackupImages + 1
	service := NewService(
		&fakeEntryService{items: []entries.EntryResponse{{ID: 7, EntryDate: "2026-05-18"}}},
		fakeImageService{countOverride: &imageCount},
	)

	content, _, err := service.Export(context.Background(), 42, "backup")
	if !errors.Is(err, ErrExportTooLarge) {
		t.Fatalf("Export() error = %v, want %v", err, ErrExportTooLarge)
	}
	if content != nil {
		t.Fatalf("content = %#v, want nil", content)
	}
}

func TestBackupExporterIncludesEntriesImagesManifestAndRestoreDoc(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(imagePath, []byte("image bytes"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	service := NewService(
		&fakeEntryService{items: []entries.EntryResponse{
			{
				ID:        7,
				EntryDate: "2026-05-18",
				Title:     "A quiet day",
				Body:      "Body text",
				Mood:      "calm",
				Tags:      []string{"life"},
				Images: []entries.EntryImage{
					{ID: 3, EntryID: 7, URL: "/api/images/3/content", FileName: "photo.jpg"},
				},
				Version:   1,
				CreatedAt: "2026-05-18T01:02:03Z",
				UpdatedAt: "2026-05-18T04:05:06Z",
			},
		}},
		fakeImageService{rows: map[int64][]images.Row{
			7: {{ID: 3, EntryID: 7, FilePath: imagePath, PublicURL: "/uploads/photo.jpg", FileName: "photo.jpg"}},
		}},
	)

	content, selected, err := service.Export(context.Background(), 42, "backup")
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if selected.FileName() != "nikki-backup.zip" {
		t.Fatalf("FileName() = %q", selected.FileName())
	}

	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	for _, name := range []string{"entries.json", "images/photo.jpg", "manifest.json", "RESTORE.md"} {
		if !names[name] {
			t.Fatalf("backup missing %s; names = %#v", name, names)
		}
	}
}

func TestEntryMarkdownExporterIncludesOnlyPublicImageReferences(t *testing.T) {
	content, err := EntryMarkdownExporter{EntryDate: "2026-05-18"}.Export([]entries.EntryResponse{
		{
			EntryDate: "2026-05-18",
			Title:     "A quiet day",
			Body:      "Body text",
			Mood:      "calm",
			Tags:      []string{"life", "work"},
			Images: []entries.EntryImage{
				{URL: "/api/images/3/content", FileName: "private-name.jpg"},
				{URL: `C:\uploads\secret.jpg`, FileName: "secret.jpg"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	got := string(content)
	for _, want := range []string{
		"# A quiet day",
		"Date: 2026-05-18",
		"Mood: calm",
		"Tags: life, work",
		"Body text",
		"## Images",
		"![Image 1](/api/images/3/content)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown missing %q:\n%s", want, got)
		}
	}
	for _, blocked := range []string{"C:\\uploads", "secret.jpg", "private-name.jpg"} {
		if strings.Contains(got, blocked) {
			t.Fatalf("markdown leaked %q:\n%s", blocked, got)
		}
	}
}

func TestHandlerExportEntryMarkdown(t *testing.T) {
	entriesService := &fakeEntryService{getByID: entries.EntryResponse{
		ID:        7,
		EntryDate: "2026-05-18",
		Title:     "A quiet day",
		Body:      "Body text",
		Mood:      "calm",
		Tags:      []string{"life"},
		Images: []entries.EntryImage{
			{URL: "/api/images/3/content"},
		},
	}}
	server := newExportTestServer(entriesService)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/export/entries/7/markdown", nil)
	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if entriesService.getUserID != 42 || entriesService.getID != 7 {
		t.Fatalf("GetByID user/id = %d/%d, want 42/7", entriesService.getUserID, entriesService.getID)
	}
	if got := response.Header().Get("Content-Type"); got != "text/markdown; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := response.Header().Get("Content-Disposition"); got != `attachment; filename="nikki-entry-2026-05-18.md"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	body := response.Body.String()
	if strings.Count(body, "# A quiet day") != 1 || !strings.Contains(body, "Body text") || !strings.Contains(body, "![Image 1](/api/images/3/content)") {
		t.Fatalf("markdown body = %q", body)
	}
}

func TestHandlerExportEntryMarkdownMissingEntryReturnsNotFound(t *testing.T) {
	server := newExportTestServer(&fakeEntryService{getErr: entries.ErrNotFound})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/export/entries/99/markdown", nil)
	server.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
	}
}

func TestHandlerExportTooLargeReturnsStableError(t *testing.T) {
	count := MaxAppExportEntries + 1
	server := newExportTestServer(&fakeEntryService{countOverride: &count})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/export/json", nil)
	server.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusRequestEntityTooLarge, response.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["kind"] != "export.too_large" {
		t.Fatalf("kind = %q, want export.too_large", body["kind"])
	}
	if strings.Contains(response.Body.String(), "Body text") {
		t.Fatalf("error response leaked diary content: %s", response.Body.String())
	}
}

func TestHandlerExportEntryMarkdownRejectsCrossUserEntry(t *testing.T) {
	entriesService := &fakeEntryService{getErr: entries.ErrNotFound}
	server := newExportTestServer(entriesService)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/export/entries/88/markdown", nil)
	server.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
	}
	if entriesService.getUserID != 42 || entriesService.getID != 88 {
		t.Fatalf("GetByID user/id = %d/%d, want 42/88", entriesService.getUserID, entriesService.getID)
	}
}

func newExportTestServer(entriesService *fakeEntryService) *echo.Echo {
	server := echo.New()
	group := server.Group("/api")
	group.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth.SetUser(c, auth.User{ID: 42, Username: "tester"})
			return next(c)
		}
	})
	NewHandler(NewService(entriesService)).Register(group)
	return server
}
