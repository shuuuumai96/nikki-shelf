package images

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/auth"
)

func TestHandlerUploadAcceptsImagesField(t *testing.T) {
	server := newImageTestServer(t, &scriptDB{
		queries: []queryResult{
			rowsResult([]string{"id"}, [][]driver.Value{{int64(10)}}),
			rowsResult([]string{"count"}, [][]driver.Value{{int64(0)}}),
			rowsResult([]string{"count", "bytes"}, [][]driver.Value{{int64(0), int64(0)}}),
			rowsResult(imageColumns, [][]driver.Value{
				imageRowValues(100, 10, "/tmp/uploaded.jpg", "/uploads/uploaded.jpg", "uploaded.jpg"),
			}),
		},
	}, fakeStorage{}, true)

	response := httptest.NewRecorder()
	request := uploadRequest(t, "/api/entries/10/images", "images", "uploaded.jpg", []byte("fake image bytes"))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusCreated)
	var body []Response
	decodeResponse(t, response, &body)
	if len(body) != 1 || body[0].ID != 100 || body[0].URL != "/api/images/100/content" || body[0].FileName != "uploaded.jpg" {
		t.Fatalf("body = %#v", body)
	}
	if body[0].Size != int64(len("fake image bytes")) || body[0].MimeType != "image/jpeg" || body[0].CreatedAt == "" {
		t.Fatalf("metadata = %#v", body[0])
	}
}

func TestHandlerUploadAcceptsLegacyImageField(t *testing.T) {
	server := newImageTestServer(t, &scriptDB{
		queries: []queryResult{
			rowsResult([]string{"id"}, [][]driver.Value{{int64(10)}}),
			rowsResult([]string{"count"}, [][]driver.Value{{int64(0)}}),
			rowsResult([]string{"count", "bytes"}, [][]driver.Value{{int64(0), int64(0)}}),
			rowsResult(imageColumns, [][]driver.Value{
				imageRowValues(101, 10, "/tmp/legacy.jpg", "/uploads/legacy.jpg", "legacy.jpg"),
			}),
		},
	}, fakeStorage{}, true)

	response := httptest.NewRecorder()
	request := uploadRequest(t, "/api/entries/10/images", "image", "legacy.jpg", []byte("fake image bytes"))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusCreated)
	var body []Response
	decodeResponse(t, response, &body)
	if len(body) != 1 || body[0].ID != 101 || body[0].FileName != "legacy.jpg" {
		t.Fatalf("body = %#v", body)
	}
}

func TestHandlerUploadWithoutFilesReturnsInvalidImage(t *testing.T) {
	server := newImageTestServer(t, &scriptDB{}, nil, true)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/entries/10/images", &body)
	request.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertError(t, response, ErrInvalidImage.Error())
}

func TestHandlerUploadRejectsInvalidMIMEContent(t *testing.T) {
	server := newImageTestServer(t, &scriptDB{
		queries: []queryResult{
			rowsResult([]string{"count"}, [][]driver.Value{{int64(0)}}),
		},
	}, NewLocalStorage(t.TempDir(), "/uploads"), true)

	response := httptest.NewRecorder()
	request := uploadRequest(t, "/api/entries/10/images", "images", "note.txt", []byte("not an image"))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertError(t, response, ErrInvalidImage.Error())
}

func TestHandlerUploadRejectsOversizedFile(t *testing.T) {
	server := newImageTestServer(t, &scriptDB{
		queries: []queryResult{
			rowsResult([]string{"count"}, [][]driver.Value{{int64(0)}}),
		},
	}, fakeStorage{}, true)

	response := httptest.NewRecorder()
	request := uploadRequest(t, "/api/entries/10/images", "images", "large.jpg", bytes.Repeat([]byte{0xff}, int(MaxImageFileBytes)+1))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusRequestEntityTooLarge)
	assertError(t, response, ErrImageTooLarge.Error())
}

func TestHandlerUploadEnforcesMaxThreeImages(t *testing.T) {
	server := newImageTestServer(t, &scriptDB{
		queries: []queryResult{
			rowsResult([]string{"id"}, [][]driver.Value{{int64(10)}}),
			rowsResult([]string{"count"}, [][]driver.Value{{int64(3)}}),
		},
	}, fakeStorage{}, true)

	response := httptest.NewRecorder()
	request := uploadRequest(t, "/api/entries/10/images", "images", "too-many.jpg", []byte("fake image bytes"))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertError(t, response, ErrTooManyImages.Error())
}

func TestHandlerUploadRejectsMoreThanThreeFilesInSingleRequest(t *testing.T) {
	storage := &recordingStorage{}
	server := newImageTestServer(t, &scriptDB{}, storage, true)

	response := httptest.NewRecorder()
	request := multiUploadRequest(t, "/api/entries/10/images", 4, []byte("fake image bytes"))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertError(t, response, ErrTooManyImages.Error())
	if len(storage.saved) != 0 {
		t.Fatalf("saved files = %#v, want none", storage.saved)
	}
}

func TestHandlerUploadSucceedsBelowUserByteQuota(t *testing.T) {
	server := newImageTestServerWithConfig(t, &scriptDB{
		queries: []queryResult{
			rowsResult([]string{"id"}, [][]driver.Value{{int64(10)}}),
			rowsResult([]string{"count"}, [][]driver.Value{{int64(0)}}),
			rowsResult([]string{"count", "bytes"}, [][]driver.Value{{int64(1), int64(10)}}),
			rowsResult(imageColumns, [][]driver.Value{
				imageRowValues(100, 10, "/tmp/small.jpg", "/uploads/small.jpg", "small.jpg"),
			}),
		},
	}, &recordingStorage{}, true, ServiceConfig{Quota: QuotaConfig{UserBytes: 20, UserCount: 100}})

	response := httptest.NewRecorder()
	request := uploadRequest(t, "/api/entries/10/images", "images", "small.jpg", []byte("12345"))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusCreated)
}

func TestHandlerUploadRejectsUserByteQuotaExceeded(t *testing.T) {
	storage := &recordingStorage{}
	server := newImageTestServerWithConfig(t, &scriptDB{
		queries: []queryResult{
			rowsResult([]string{"id"}, [][]driver.Value{{int64(10)}}),
			rowsResult([]string{"count"}, [][]driver.Value{{int64(0)}}),
			rowsResult([]string{"count", "bytes"}, [][]driver.Value{{int64(1), int64(10)}}),
		},
	}, storage, true, ServiceConfig{Quota: QuotaConfig{UserBytes: 20, UserCount: 100}})

	response := httptest.NewRecorder()
	request := uploadRequest(t, "/api/entries/10/images", "images", "quota.jpg", []byte("12345678901"))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusRequestEntityTooLarge)
	assertError(t, response, ErrImageQuotaExceeded.Error())
	assertKind(t, response, "images.quota_exceeded")
	if len(storage.deleted) != 1 {
		t.Fatalf("deleted files = %#v, want one cleanup", storage.deleted)
	}
}

func TestHandlerUploadRejectsUserCountQuotaExceeded(t *testing.T) {
	storage := &recordingStorage{}
	server := newImageTestServerWithConfig(t, &scriptDB{
		queries: []queryResult{
			rowsResult([]string{"id"}, [][]driver.Value{{int64(10)}}),
			rowsResult([]string{"count"}, [][]driver.Value{{int64(0)}}),
			rowsResult([]string{"count", "bytes"}, [][]driver.Value{{int64(2), int64(10)}}),
		},
	}, storage, true, ServiceConfig{Quota: QuotaConfig{UserBytes: 100, UserCount: 2}})

	response := httptest.NewRecorder()
	request := uploadRequest(t, "/api/entries/10/images", "images", "quota.jpg", []byte("1"))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusRequestEntityTooLarge)
	assertKind(t, response, "images.quota_exceeded")
	if len(storage.deleted) != 1 {
		t.Fatalf("deleted files = %#v, want one cleanup", storage.deleted)
	}
}

func TestHandlerUploadUserQuotaIsScopedToAuthenticatedUser(t *testing.T) {
	script := &scriptDB{
		queries: []queryResult{
			rowsResult([]string{"id"}, [][]driver.Value{{int64(10)}}),
			rowsResult([]string{"count"}, [][]driver.Value{{int64(0)}}),
			rowsResult([]string{"count", "bytes"}, [][]driver.Value{{int64(0), int64(0)}}),
			rowsResult(imageColumns, [][]driver.Value{
				imageRowValues(100, 10, "/tmp/user.jpg", "/uploads/user.jpg", "user.jpg"),
			}),
		},
	}
	server := newImageTestServerWithConfig(t, script, &recordingStorage{}, true, ServiceConfig{Quota: QuotaConfig{UserBytes: 5, UserCount: 1}})

	response := httptest.NewRecorder()
	request := uploadRequest(t, "/api/entries/10/images", "images", "user.jpg", []byte("12345"))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusCreated)
	if len(script.records) < 3 || !strings.Contains(script.records[2].query, "WHERE e.user_id = $1") {
		t.Fatalf("user quota query = %#v", script.records)
	}
	if got := script.records[2].args[0].Value; got != int64(42) {
		t.Fatalf("user quota arg = %#v, want authenticated user 42", got)
	}
}

func TestHandlerUploadRejectsGlobalByteQuotaExceeded(t *testing.T) {
	storage := &recordingStorage{}
	server := newImageTestServerWithConfig(t, &scriptDB{
		queries: []queryResult{
			rowsResult([]string{"id"}, [][]driver.Value{{int64(10)}}),
			rowsResult([]string{"count"}, [][]driver.Value{{int64(0)}}),
			rowsResult([]string{"count", "bytes"}, [][]driver.Value{{int64(0), int64(0)}}),
			rowsResult([]string{"count", "bytes"}, [][]driver.Value{{int64(7), int64(19)}}),
		},
	}, storage, true, ServiceConfig{Quota: QuotaConfig{UserBytes: 100, UserCount: 100, TotalBytes: 20}})

	response := httptest.NewRecorder()
	request := uploadRequest(t, "/api/entries/10/images", "images", "global.jpg", []byte("12"))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusRequestEntityTooLarge)
	assertKind(t, response, "images.quota_exceeded")
	if len(storage.deleted) != 1 {
		t.Fatalf("deleted files = %#v, want one cleanup", storage.deleted)
	}
}

func TestHandlerUploadCleansFilesWhenTransactionFails(t *testing.T) {
	storage := &recordingStorage{}
	server := newImageTestServerWithConfig(t, &scriptDB{
		queries: []queryResult{
			rowsResult([]string{"id"}, nil),
		},
	}, storage, true, ServiceConfig{Quota: QuotaConfig{UserBytes: 100, UserCount: 100}})

	response := httptest.NewRecorder()
	request := uploadRequest(t, "/api/entries/10/images", "images", "cleanup.jpg", []byte("1"))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusNotFound)
	if len(storage.deleted) != 1 {
		t.Fatalf("deleted files = %#v, want one cleanup", storage.deleted)
	}
}

func TestHandlerContentServesOwnedImage(t *testing.T) {
	path := writeTestImageFile(t, "owned image bytes")
	server := newImageTestServer(t, &scriptDB{
		queries: []queryResult{
			rowsResult(imageColumns, [][]driver.Value{
				imageRowValues(100, 10, path, "/uploads/owned.jpg", "owned.jpg"),
			}),
		},
	}, fakeStorage{}, true)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/images/100/content", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusOK)
	if got := response.Body.String(); got != "owned image bytes" {
		t.Fatalf("body = %q", got)
	}
	if got := response.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("Content-Type = %q, want image/jpeg", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "private, max-age=3600" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if strings.Contains(response.Body.String(), path) {
		t.Fatalf("response leaked file path %q", path)
	}
}

func TestHandlerContentRejectsCrossUserImageAsNotFound(t *testing.T) {
	server := newImageTestServerForUser(t, &scriptDB{
		queries: []queryResult{rowsResult(imageColumns, nil)},
	}, fakeStorage{}, true, 7, ServiceConfig{Quota: defaultQuotaConfig()})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/images/100/content", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusNotFound)
	assertKind(t, response, "images.not_found")
}

func TestHandlerContentMissingImageReturnsNotFound(t *testing.T) {
	server := newImageTestServer(t, &scriptDB{
		queries: []queryResult{rowsResult(imageColumns, nil)},
	}, fakeStorage{}, true)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/images/999/content", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusNotFound)
	assertKind(t, response, "images.not_found")
}

func TestHandlerContentInvalidImageIDReturnsBadRequest(t *testing.T) {
	server := newImageTestServer(t, &scriptDB{}, fakeStorage{}, true)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/images/not-a-number/content", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertKind(t, response, "images.invalid_image_id")
}

func TestHandlerContentMissingFileReturnsNotFound(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.jpg")
	server := newImageTestServer(t, &scriptDB{
		queries: []queryResult{
			rowsResult(imageColumns, [][]driver.Value{
				imageRowValues(100, 10, missing, "/uploads/missing.jpg", "missing.jpg"),
			}),
		},
	}, fakeStorage{}, true)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/images/100/content", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusNotFound)
	assertKind(t, response, "images.not_found")
}

func TestHandlerLegacyUploadRouteIsOwnerChecked(t *testing.T) {
	server := newImageTestServerForUser(t, &scriptDB{
		queries: []queryResult{rowsResult(imageColumns, nil)},
	}, fakeStorage{}, true, 7, ServiceConfig{Quota: defaultQuotaConfig()})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/uploads/owned.jpg", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusNotFound)
	assertKind(t, response, "images.not_found")
}

func TestHandlerLegacyUploadRouteServesOwnedImageByStoredName(t *testing.T) {
	path := writeTestImageFile(t, "legacy image bytes")
	server := newImageTestServer(t, &scriptDB{
		queries: []queryResult{
			rowsResult(imageColumns, [][]driver.Value{
				imageRowValues(100, 10, path, "/uploads/legacy.jpg", "legacy.jpg"),
			}),
		},
	}, fakeStorage{}, true)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/uploads/legacy.jpg", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusOK)
	if got := response.Body.String(); got != "legacy image bytes" {
		t.Fatalf("body = %q", got)
	}
}

func TestHandlerDeleteImageNotFound(t *testing.T) {
	server := newImageTestServer(t, &scriptDB{
		queries: []queryResult{rowsResult(imageColumns, nil)},
	}, fakeStorage{}, true)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/images/99", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusNotFound)
	assertError(t, response, ErrImageNotFound.Error())
}

func TestHandlerDeleteImageRemovesRecordAndStorage(t *testing.T) {
	storage := &trackingStorage{}
	server := newImageTestServer(t, &scriptDB{
		queries: []queryResult{
			rowsResult(imageColumns, [][]driver.Value{
				imageRowValues(99, 10, "/tmp/delete.jpg", "/uploads/delete.jpg", "delete.jpg"),
			}),
		},
		execs: []execResult{{affected: 1}},
	}, storage, true)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/images/99", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusNoContent)
	if storage.deleted != "/tmp/delete.jpg" {
		t.Fatalf("deleted path = %q", storage.deleted)
	}
}

type fakeEntryReader bool

func (f fakeEntryReader) ExistsForUser(context.Context, int64, int64) (bool, error) {
	return bool(f), nil
}

type fakeStorage struct{}

func (fakeStorage) Save(_ context.Context, _ int64, file *multipart.FileHeader) (StoredFile, error) {
	return StoredFile{
		FilePath:  "/tmp/" + file.Filename,
		PublicURL: "/uploads/" + file.Filename,
		FileName:  file.Filename,
		Size:      file.Size,
		MimeType:  "image/jpeg",
	}, nil
}

func (fakeStorage) Delete(context.Context, string) error {
	return nil
}

type recordingStorage struct {
	mu      sync.Mutex
	saved   []string
	deleted []string
}

func (s *recordingStorage) Save(_ context.Context, _ int64, file *multipart.FileHeader) (StoredFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := "/tmp/" + file.Filename
	s.saved = append(s.saved, path)
	return StoredFile{
		FilePath:  path,
		PublicURL: "/uploads/" + file.Filename,
		FileName:  file.Filename,
		Size:      file.Size,
		MimeType:  "image/jpeg",
	}, nil
}

func (s *recordingStorage) Delete(_ context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, path)
	return nil
}

type trackingStorage struct {
	deleted string
}

func (trackingStorage) Save(context.Context, int64, *multipart.FileHeader) (StoredFile, error) {
	return StoredFile{}, nil
}

func (s *trackingStorage) Delete(_ context.Context, path string) error {
	s.deleted = path
	return nil
}

func newImageTestServer(t *testing.T, script *scriptDB, storage Storage, entryExists bool) *echo.Echo {
	return newImageTestServerWithConfig(t, script, storage, entryExists, ServiceConfig{Quota: defaultQuotaConfig()})
}

func newImageTestServerWithConfig(t *testing.T, script *scriptDB, storage Storage, entryExists bool, config ServiceConfig) *echo.Echo {
	return newImageTestServerForUser(t, script, storage, entryExists, 42, config)
}

func newImageTestServerForUser(t *testing.T, script *scriptDB, storage Storage, entryExists bool, userID int64, config ServiceConfig) *echo.Echo {
	t.Helper()

	database := openScriptDB(t, script)
	t.Cleanup(func() { _ = database.Close() })

	server := echo.New()
	withUser := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth.SetUser(c, auth.User{ID: userID, Username: "tester"})
			return next(c)
		}
	}
	handler := NewHandler(NewService(NewRepository(database), storage, fakeEntryReader(entryExists), config))
	group := server.Group("/api")
	group.Use(withUser)
	handler.Register(group)
	uploads := server.Group("/uploads")
	uploads.Use(withUser)
	handler.RegisterUploads(uploads)
	return server
}

func uploadRequest(t *testing.T, target string, field string, name string, content []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, name)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, target, &body)
	request.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	return request
}

func writeTestImageFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "image.jpg")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write test image: %v", err)
	}
	return path
}

func multiUploadRequest(t *testing.T, target string, count int, content []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for i := range count {
		part, err := writer.CreateFormFile("images", "image-"+strconv.Itoa(i)+".jpg")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("write form file: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, target, &body)
	request.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	return request
}

func assertStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, want, response.Body.String())
	}
}

func assertError(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body map[string]string
	decodeResponse(t, response, &body)
	if body["error"] != want {
		t.Fatalf("error = %q, want %q", body["error"], want)
	}
}

func assertKind(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body map[string]string
	decodeResponse(t, response, &body)
	if body["kind"] != want {
		t.Fatalf("kind = %q, want %q; body = %s", body["kind"], want, response.Body.String())
	}
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, response.Body.String())
	}
}

var imageColumns = []string{"id", "entry_id", "file_path", "public_url", "file_name", "size_bytes", "mime_type", "created_at"}

func imageRowValues(id int64, entryID int64, filePath string, publicURL string, fileName string) []driver.Value {
	return []driver.Value{id, entryID, filePath, publicURL, fileName, int64(16), "image/jpeg", "2026-05-18T10:00:00Z"}
}

type scriptDB struct {
	mu      sync.Mutex
	queries []queryResult
	execs   []execResult
	records []queryRecord
}

type queryRecord struct {
	query string
	args  []driver.NamedValue
}

type queryResult struct {
	columns []string
	rows    [][]driver.Value
	err     error
}

type execResult struct {
	affected int64
	err      error
}

func rowsResult(columns []string, rows [][]driver.Value) queryResult {
	return queryResult{columns: columns, rows: rows}
}

func (s *scriptDB) popQuery() queryResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queries) == 0 {
		return queryResult{err: errors.New("unexpected query")}
	}
	result := s.queries[0]
	s.queries = s.queries[1:]
	return result
}

func (s *scriptDB) popExec() execResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.execs) == 0 {
		return execResult{err: errors.New("unexpected exec")}
	}
	result := s.execs[0]
	s.execs = s.execs[1:]
	return result
}

func (s *scriptDB) recordQuery(query string, args []driver.NamedValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := append([]driver.NamedValue(nil), args...)
	s.records = append(s.records, queryRecord{query: query, args: copied})
}

var (
	scriptDriverOnce sync.Once
	scriptDriverMu   sync.Mutex
	scriptDrivers    = map[string]*scriptDB{}
)

func openScriptDB(t *testing.T, script *scriptDB) *sql.DB {
	t.Helper()
	scriptDriverOnce.Do(func() {
		sql.Register("images-handler-script", scriptDriver{})
	})

	name := t.Name()
	scriptDriverMu.Lock()
	scriptDrivers[name] = script
	scriptDriverMu.Unlock()
	t.Cleanup(func() {
		scriptDriverMu.Lock()
		delete(scriptDrivers, name)
		scriptDriverMu.Unlock()
	})

	database, err := sql.Open("images-handler-script", name)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	return database
}

type scriptDriver struct{}

func (scriptDriver) Open(name string) (driver.Conn, error) {
	scriptDriverMu.Lock()
	script := scriptDrivers[name]
	scriptDriverMu.Unlock()
	if script == nil {
		return nil, errors.New("missing script db")
	}
	return scriptConn{script: script}, nil
}

type scriptConn struct {
	script *scriptDB
}

func (scriptConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}

func (scriptConn) Close() error {
	return nil
}

func (scriptConn) Begin() (driver.Tx, error) {
	return scriptTx{}, nil
}

func (c scriptConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.script.recordQuery(query, args)
	result := c.script.popQuery()
	if result.err != nil {
		return nil, result.err
	}
	return &scriptRows{columns: result.columns, rows: result.rows}, nil
}

func (c scriptConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	result := c.script.popExec()
	if result.err != nil {
		return nil, result.err
	}
	return scriptResult(result.affected), nil
}

type scriptRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *scriptRows) Columns() []string {
	return r.columns
}

func (r *scriptRows) Close() error {
	return nil
}

func (r *scriptRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

type scriptResult int64

func (scriptResult) LastInsertId() (int64, error) {
	return 0, errors.New("last insert id is not supported")
}

func (r scriptResult) RowsAffected() (int64, error) {
	return int64(r), nil
}

type scriptTx struct{}

func (scriptTx) Commit() error {
	return nil
}

func (scriptTx) Rollback() error {
	return nil
}
