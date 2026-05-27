package entries

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/auth"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/images"
)

func TestHandlerRequiresAuth(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{}, true)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/entries", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusUnauthorized)
	assertError(t, response, auth.ErrUnauthorized.Error())
}

func TestHandlerSearchRequiresAuth(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{}, true)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/entries/search?q=tea", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusUnauthorized)
	assertError(t, response, auth.ErrUnauthorized.Error())
}

func TestHandlerListEntries(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{
		queries: []queryResult{
			rowsResult(entryColumns, [][]driver.Value{
				entryRowValues(10, "2026-05-18", "Today", "Body", "calm", `["life"]`),
			}),
		},
	}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodGet, "/api/entries?query=tea&tag=life&mood=calm&from=2026-05-01&to=2026-05-31", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusOK)
	var body EntryPageResponse
	decodeResponse(t, response, &body)
	if len(body.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(body.Items))
	}
	if body.Items[0].ID != 10 || body.Items[0].EntryDate != "2026-05-18" || body.Items[0].Tags[0] != "life" {
		t.Fatalf("items[0] = %#v", body.Items[0])
	}
	if len(body.Items[0].Images) != 1 || body.Items[0].Images[0].URL != "/api/images/100/content" {
		t.Fatalf("images = %#v", body.Items[0].Images)
	}
}

func TestHandlerListEntriesDefaultsToFiftyItems(t *testing.T) {
	rows := make([][]driver.Value, 0, DefaultEntriesPerPage+1)
	for i := 0; i < DefaultEntriesPerPage+1; i++ {
		rows = append(rows, entryRowValues(int64(100-i), "2026-05-18", "Title", "Body", "calm", `[]`))
	}
	server := newEntryTestServer(t, &scriptDB{
		queries: []queryResult{rowsResult(entryColumns, rows)},
	}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodGet, "/api/entries", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusOK)
	var body EntryPageResponse
	decodeResponse(t, response, &body)
	if len(body.Items) != DefaultEntriesPerPage {
		t.Fatalf("len(items) = %d, want %d", len(body.Items), DefaultEntriesPerPage)
	}
	if !body.HasMore || body.NextCursor == "" {
		t.Fatalf("pagination metadata = %#v, want hasMore with cursor", body)
	}
	if link := response.Header().Get("Link"); !strings.Contains(link, `rel="next"`) || !strings.Contains(link, "cursor=") {
		t.Fatalf("Link header = %q, want next cursor link", link)
	}
}

func TestHandlerListEntriesRejectsInvalidPerPage(t *testing.T) {
	for _, target := range []string{"/api/entries?per_page=0", "/api/entries?per_page=101", "/api/entries?per_page=nope"} {
		t.Run(target, func(t *testing.T) {
			server := newEntryTestServer(t, &scriptDB{}, false)

			response := httptest.NewRecorder()
			request := authedRequest(http.MethodGet, target, nil)
			server.ServeHTTP(response, request)

			assertStatus(t, response, http.StatusBadRequest)
			assertErrorKind(t, response, "entries.invalid_input")
		})
	}
}

func TestHandlerListEntriesCursorPaginationReturnsNextRows(t *testing.T) {
	firstPageRows := [][]driver.Value{
		entryRowValues(9, "2026-05-19", "Newest", "Body", "calm", `[]`),
		entryRowValues(8, "2026-05-18", "Middle", "Body", "calm", `[]`),
		entryRowValues(7, "2026-05-18", "Extra", "Body", "calm", `[]`),
	}
	script := &scriptDB{
		queries: []queryResult{rowsResult(entryColumns, firstPageRows)},
	}
	server := newEntryTestServer(t, script, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodGet, "/api/entries?per_page=2", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusOK)
	var first EntryPageResponse
	decodeResponse(t, response, &first)
	if len(first.Items) != 2 || !first.HasMore || first.NextCursor == "" {
		t.Fatalf("first page = %#v, want two items with cursor", first)
	}
	if first.Items[0].ID != 9 || first.Items[1].ID != 8 {
		t.Fatalf("first page order = %#v", first.Items)
	}

	secondScript := &scriptDB{
		queries: []queryResult{rowsResult(entryColumns, [][]driver.Value{
			entryRowValues(7, "2026-05-18", "Extra", "Body", "calm", `[]`),
			entryRowValues(6, "2026-05-17", "Older", "Body", "calm", `[]`),
		})},
	}
	server = newEntryTestServer(t, secondScript, false)
	response = httptest.NewRecorder()
	request = authedRequest(http.MethodGet, "/api/entries?per_page=2&cursor="+first.NextCursor, nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusOK)
	var second EntryPageResponse
	decodeResponse(t, response, &second)
	if len(second.Items) != 2 || second.HasMore || second.NextCursor != "" {
		t.Fatalf("second page = %#v, want final two items", second)
	}
	if second.Items[0].ID != 7 || second.Items[1].ID != 6 {
		t.Fatalf("second page order = %#v", second.Items)
	}

	args := secondScript.queryArgs(0)
	if len(args) < 4 || args[0] != int64(42) || args[1] != "2026-05-18" || args[2] != int64(8) {
		t.Fatalf("cursor query args = %#v, want scoped user and cursor values", args)
	}
}

func TestHandlerListEntriesRejectsInvalidCursor(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodGet, "/api/entries?cursor=not-a-cursor", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertErrorKind(t, response, "entries.invalid_cursor")
}

func TestHandlerCreateEntry(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{
		queries: []queryResult{
			rowsResult([]string{"id"}, [][]driver.Value{{int64(11)}}),
			rowsResult(entryColumns, [][]driver.Value{
				entryRowValues(11, "2026-05-18", "Created", "Body", "calm", `["life"]`),
			}),
		},
	}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodPost, "/api/entries", strings.NewReader(`{
		"entryDate":"2026-05-18",
		"title":"Created",
		"body":"Body",
		"mood":"calm",
		"tags":["life"]
	}`))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusCreated)
	var body EntryResponse
	decodeResponse(t, response, &body)
	if body.ID != 11 || body.Title != "Created" || body.Images[0].FileName != "11.jpg" {
		t.Fatalf("body = %#v", body)
	}
}

func TestHandlerCreateEntryValidationError(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodPost, "/api/entries", strings.NewReader(`{
		"entryDate":"2026/05/18",
		"title":"Bad date",
		"body":"Body",
		"mood":"calm",
		"tags":[]
	}`))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertError(t, response, ErrInvalidInput.Error())
}

func TestHandlerCreateEntryRejectsOversizedJSON(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodPost, "/api/entries", strings.NewReader(`{
		"entryDate":"2026-05-18",
		"title":"Oversized",
		"body":"`+strings.Repeat("x", (1<<20)+1)+`",
		"mood":"calm",
		"tags":[]
	}`))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusRequestEntityTooLarge)
	assertErrorKind(t, response, "request.too_large")
}

func TestHandlerCreateEntryRejectsTitleOverLimit(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodPost, "/api/entries", strings.NewReader(validCreateJSON(strings.Repeat("あ", MaxTitleRunes+1), "Body", []string{"life"})))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertError(t, response, ErrInvalidInput.Error())
}

func TestHandlerCreateEntryRejectsBodyOverLimit(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodPost, "/api/entries", strings.NewReader(validCreateJSON("Title", strings.Repeat("あ", MaxBodyRunes+1), []string{"life"})))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertError(t, response, ErrInvalidInput.Error())
}

func TestHandlerCreateEntryRejectsTooManyTags(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{}, false)
	tags := make([]string, MaxTags+1)
	for i := range tags {
		tags[i] = "tag-" + strconv.Itoa(i)
	}

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodPost, "/api/entries", strings.NewReader(validCreateJSON("Title", "Body", tags)))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertError(t, response, ErrInvalidInput.Error())
}

func TestHandlerCreateEntryRejectsTagOverLimit(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodPost, "/api/entries", strings.NewReader(validCreateJSON("Title", "Body", []string{strings.Repeat("あ", MaxTagRunes+1)})))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertError(t, response, ErrInvalidInput.Error())
}

func TestHandlerCreateEntryDuplicateDate(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{
		queries: []queryResult{{err: &pgconn.PgError{Code: "23505"}}},
	}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodPost, "/api/entries", strings.NewReader(`{
		"entryDate":"2026-05-18",
		"title":"Duplicate",
		"body":"Body",
		"mood":"calm",
		"tags":[]
	}`))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusConflict)
	assertError(t, response, ErrDateExists.Error())
}

func TestHandlerGetEntryByID(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{
		queries: []queryResult{
			rowsResult(entryColumns, [][]driver.Value{
				entryRowValues(12, "2026-05-18", "Found", "Body", "happy", `[]`),
			}),
		},
	}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodGet, "/api/entries/12", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusOK)
	var body EntryResponse
	decodeResponse(t, response, &body)
	if body.ID != 12 || body.Mood != "happy" {
		t.Fatalf("body = %#v", body)
	}
}

func TestHandlerGetEntryInvalidID(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodGet, "/api/entries/not-a-number", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertError(t, response, "check the ID")
}

func TestHandlerGetEntryNotFound(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{
		queries: []queryResult{rowsResult(entryColumns, nil)},
	}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodGet, "/api/entries/99", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusNotFound)
	assertError(t, response, ErrNotFound.Error())
}

func TestHandlerUpdateEntry(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{
		execs: []execResult{{affected: 1}},
		queries: []queryResult{
			rowsResult(entryColumns, [][]driver.Value{
				entryRowValues(13, "2026-05-18", "Updated", "Body", "calm", `["life"]`),
			}),
		},
	}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodPut, "/api/entries/13", strings.NewReader(`{
		"entryDate":"2026-05-18",
		"title":"Updated",
		"body":"Body",
		"mood":"calm",
		"tags":["life"],
		"expectedVersion":1
	}`))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusOK)
	var body EntryResponse
	decodeResponse(t, response, &body)
	if body.ID != 13 || body.Title != "Updated" {
		t.Fatalf("body = %#v", body)
	}
}

func TestHandlerUpdateEntryRejectsOversizedJSON(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodPut, "/api/entries/13", strings.NewReader(`{
		"entryDate":"2026-05-18",
		"title":"Oversized",
		"body":"`+strings.Repeat("x", (1<<20)+1)+`",
		"mood":"calm",
		"tags":[],
		"expectedVersion":1
	}`))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusRequestEntityTooLarge)
	assertErrorKind(t, response, "request.too_large")
}

func TestHandlerUpdateEntryRejectsContentOverLimitBeforeDatabase(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodPut, "/api/entries/13", strings.NewReader(validUpdateJSON(strings.Repeat("あ", MaxTitleRunes+1), "Body", []string{"life"})))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertError(t, response, ErrInvalidInput.Error())
}

func TestHandlerUpdateEntryNotFound(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{
		execs:   []execResult{{affected: 0}},
		queries: []queryResult{rowsResult([]string{"exists"}, nil)},
	}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodPut, "/api/entries/13", strings.NewReader(`{
		"entryDate":"2026-05-18",
		"title":"Missing",
		"body":"Body",
		"mood":"calm",
		"tags":[],
		"expectedVersion":1
	}`))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusNotFound)
	assertError(t, response, ErrNotFound.Error())
}

func TestHandlerUpdateEntryStaleVersion(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{
		execs: []execResult{{affected: 0}},
		queries: []queryResult{
			rowsResult([]string{"exists"}, [][]driver.Value{{int64(1)}}),
		},
	}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodPut, "/api/entries/13", strings.NewReader(`{
		"entryDate":"2026-05-18",
		"title":"Stale",
		"body":"Old body",
		"mood":"calm",
		"tags":[],
		"expectedVersion":1
	}`))
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusConflict)
	assertError(t, response, ErrStaleVersion.Error())
	assertErrorKind(t, response, "entries.stale_version")
}

func TestHandlerDeleteEntry(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{
		execs: []execResult{{affected: 1}},
	}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodDelete, "/api/entries/14", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusNoContent)
}

func TestHandlerDeleteEntryNotFound(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{
		execs: []execResult{{affected: 0}},
	}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodDelete, "/api/entries/14", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusNotFound)
	assertError(t, response, ErrNotFound.Error())
}

func TestHandlerGetEntryByDate(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{
		queries: []queryResult{
			rowsResult(entryColumns, [][]driver.Value{
				entryRowValues(15, "2026-05-18", "By date", "Body", "sad", `[]`),
			}),
		},
	}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodGet, "/api/entries/date/2026-05-18", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusOK)
	var body EntryDateLookupResponse
	decodeResponse(t, response, &body)
	if !body.Exists || body.Date != "2026-05-18" || body.Entry == nil {
		t.Fatalf("body = %#v", body)
	}
	if body.Entry.EntryDate != "2026-05-18" || body.Entry.Title != "By date" {
		t.Fatalf("body = %#v", body)
	}
}

func TestHandlerGetEntryByDateEmptyDate(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{
		queries: []queryResult{rowsResult(entryColumns, nil)},
	}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodGet, "/api/entries/date/2026-05-19", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusOK)
	var body EntryDateLookupResponse
	decodeResponse(t, response, &body)
	if body.Exists || body.Date != "2026-05-19" || body.Entry != nil {
		t.Fatalf("body = %#v", body)
	}
}

func TestHandlerGetEntryByDateValidationError(t *testing.T) {
	server := newEntryTestServer(t, &scriptDB{}, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodGet, "/api/entries/date/2026-05-99", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusBadRequest)
	assertError(t, response, ErrInvalidInput.Error())
}

func TestHandlerTags(t *testing.T) {
	script := &scriptDB{
		queries: []queryResult{
			rowsResult([]string{"tag"}, [][]driver.Value{
				{"a"},
				{"m"},
				{"z"},
			}),
		},
	}
	server := newEntryTestServer(t, script, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodGet, "/api/tags", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusOK)
	var body []string
	decodeResponse(t, response, &body)
	if strings.Join(body, ",") != "a,m,z" {
		t.Fatalf("tags = %#v", body)
	}
	if query := script.queryLog()[0].query; strings.Contains(query, "body") {
		t.Fatalf("tags query loads body content: %s", query)
	}
}

func TestHandlerSearchEntries(t *testing.T) {
	script := &scriptDB{
		queries: []queryResult{
			rowsResult(searchColumns, [][]driver.Value{
				searchRowValues(20, "2026-05-18", "Tea", "Quiet tea with the cat after work", "calm", `["home","cat"]`, 2),
			}),
		},
	}
	server := newEntryTestServer(t, script, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodGet, "/api/entries/search?q=cat&from=2026-05-01&to=2026-05-31&mood=calm&tag=home&hasImage=true&limit=500&offset=2", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusOK)
	var body SearchResponse
	decodeResponse(t, response, &body)
	if len(body.Results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(body.Results))
	}
	result := body.Results[0]
	if result.ID != 20 || result.EntryDate != "2026-05-18" || result.Preview == "" {
		t.Fatalf("result = %#v", result)
	}
	if !result.HasImage || result.ImageCount != 2 || result.Tags[1] != "cat" {
		t.Fatalf("result metadata = %#v", result)
	}

	args := script.queryArgs(0)
	if len(args) == 0 || args[0] != int64(42) {
		t.Fatalf("first query arg = %#v, want user id 42", args)
	}
	if args[len(args)-2] != int64(maxSearchLimit) {
		t.Fatalf("limit arg = %#v, want cap %d", args[len(args)-2], maxSearchLimit)
	}
}

func TestHandlerSearchNoActiveFilterReturnsEmptyWithoutQuery(t *testing.T) {
	script := &scriptDB{}
	server := newEntryTestServer(t, script, false)

	response := httptest.NewRecorder()
	request := authedRequest(http.MethodGet, "/api/entries/search", nil)
	server.ServeHTTP(response, request)

	assertStatus(t, response, http.StatusOK)
	var body SearchResponse
	decodeResponse(t, response, &body)
	if len(body.Results) != 0 {
		t.Fatalf("results = %#v, want empty", body.Results)
	}
	if len(script.queryLog()) != 0 {
		t.Fatalf("query count = %d, want 0", len(script.queryLog()))
	}
}

type fakeImageReader struct{}

func (fakeImageReader) ListByEntryID(_ context.Context, entryID int64) ([]images.Row, error) {
	name := strconv.FormatInt(entryID, 10) + ".jpg"
	return []images.Row{{ID: entryID * 10, PublicURL: "/uploads/" + name, FileName: name}}, nil
}

func newEntryTestServer(t *testing.T, script *scriptDB, requireAuthMiddleware bool) *echo.Echo {
	t.Helper()

	database := openScriptDB(t, script)
	t.Cleanup(func() { _ = database.Close() })

	handler := NewHandler(NewService(NewRepository(database), fakeImageReader{}))
	server := echo.New()
	group := server.Group("/api")
	if !requireAuthMiddleware {
		group.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				auth.SetUser(c, auth.User{ID: 42, Username: "tester"})
				return next(c)
			}
		})
	}
	handler.Register(group)
	return server
}

func authedRequest(method string, target string, body io.Reader) *http.Request {
	request := httptest.NewRequest(method, target, body)
	if body != nil {
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
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

func assertErrorKind(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body map[string]string
	decodeResponse(t, response, &body)
	if body["kind"] != want {
		t.Fatalf("kind = %q, want %q", body["kind"], want)
	}
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, response.Body.String())
	}
}

var entryColumns = []string{"id", "user_id", "entry_date", "title", "body", "mood", "tags_json", "version", "created_at", "updated_at"}
var searchColumns = []string{"id", "entry_date", "title", "body", "mood", "tags_json", "image_count", "updated_at"}

func entryRowValues(id int64, date string, title string, body string, mood string, tags string) []driver.Value {
	return []driver.Value{id, int64(42), date, title, body, mood, tags, int64(1), "2026-05-18T10:00:00Z", "2026-05-18T10:00:00Z"}
}

func searchRowValues(id int64, date string, title string, body string, mood string, tags string, imageCount int64) []driver.Value {
	return []driver.Value{id, date, title, body, mood, tags, imageCount, "2026-05-18T10:00:00Z"}
}

func validCreateJSON(title string, body string, tags []string) string {
	encoded, err := json.Marshal(CreateInput{
		EntryDate: "2026-05-18",
		Title:     title,
		Body:      body,
		Mood:      "calm",
		Tags:      tags,
	})
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func validUpdateJSON(title string, body string, tags []string) string {
	encoded, err := json.Marshal(UpdateInput{
		EntryDate: "2026-05-18",
		Title:     title,
		Body:      body,
		Mood:      "calm",
		Tags:      tags,
		Version:   1,
	})
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

type scriptDB struct {
	mu            sync.Mutex
	queries       []queryResult
	execs         []execResult
	loggedQueries []loggedQuery
}

type loggedQuery struct {
	query string
	args  []any
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

func (s *scriptDB) logQuery(query string, args []driver.NamedValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := make([]any, 0, len(args))
	for _, arg := range args {
		values = append(values, arg.Value)
	}
	s.loggedQueries = append(s.loggedQueries, loggedQuery{query: query, args: values})
}

func (s *scriptDB) queryLog() []loggedQuery {
	s.mu.Lock()
	defer s.mu.Unlock()
	logged := make([]loggedQuery, len(s.loggedQueries))
	copy(logged, s.loggedQueries)
	return logged
}

func (s *scriptDB) queryArgs(index int) []any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.loggedQueries) {
		return nil
	}
	args := make([]any, len(s.loggedQueries[index].args))
	copy(args, s.loggedQueries[index].args)
	return args
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

var (
	scriptDriverOnce sync.Once
	scriptDriverMu   sync.Mutex
	scriptDrivers    = map[string]*scriptDB{}
)

func openScriptDB(t *testing.T, script *scriptDB) *sql.DB {
	t.Helper()
	scriptDriverOnce.Do(func() {
		sql.Register("entries-handler-script", scriptDriver{})
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

	database, err := sql.Open("entries-handler-script", name)
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
	return nil, errors.New("transactions are not supported")
}

func (c scriptConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.script.logQuery(query, args)
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
