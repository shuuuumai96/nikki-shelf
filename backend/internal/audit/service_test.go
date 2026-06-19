package audit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestRecordNormalizesAuditEvent(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo)
	service.now = func() time.Time {
		return time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	}

	if err := service.Record(context.Background(), Event{EventType: "auth.login_succeeded"}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if len(repo.inserted) != 1 {
		t.Fatalf("inserted = %d, want 1", len(repo.inserted))
	}
	event := repo.inserted[0]
	if event.Outcome != OutcomeSucceeded {
		t.Fatalf("Outcome = %q, want %q", event.Outcome, OutcomeSucceeded)
	}
	if event.CreatedAt != "2026-06-13T10:00:00Z" {
		t.Fatalf("CreatedAt = %q", event.CreatedAt)
	}
	if event.Metadata == nil {
		t.Fatal("Metadata = nil, want empty map")
	}
}

func TestListClampsLimit(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo)

	if _, err := service.List(context.Background(), 999); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if repo.listLimit != MaxListLimit {
		t.Fatalf("limit = %d, want %d", repo.listLimit, MaxListLimit)
	}
}

func TestPruneExpiredUsesRetentionDays(t *testing.T) {
	repo := &fakeRepo{deleted: 2}
	service := NewService(repo, Config{RetentionDays: 90})
	service.now = func() time.Time {
		return time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	}

	deleted, err := service.PruneExpired(context.Background())
	if err != nil {
		t.Fatalf("PruneExpired() error = %v", err)
	}

	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	want := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	if !repo.cutoff.Equal(want) {
		t.Fatalf("cutoff = %s, want %s", repo.cutoff, want)
	}
}

func TestRecordHTTPDefaultsToDirectRemoteIP(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo)
	server := echo.New()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	request.RemoteAddr = "198.51.100.10:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.99")
	context := server.NewContext(request, httptest.NewRecorder())

	service.RecordHTTP(context, Event{EventType: "auth.login_failed"})

	if len(repo.inserted) != 1 {
		t.Fatalf("inserted = %d, want 1", len(repo.inserted))
	}
	if repo.inserted[0].RemoteIP != "198.51.100.10" {
		t.Fatalf("RemoteIP = %q, want direct remote addr", repo.inserted[0].RemoteIP)
	}
}

func TestRecordHTTPUsesConfiguredRemoteIP(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo, Config{
		RemoteIP: func(*http.Request) string {
			return "203.0.113.20"
		},
	})
	server := echo.New()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	request.RemoteAddr = "198.51.100.10:1234"
	context := server.NewContext(request, httptest.NewRecorder())

	service.RecordHTTP(context, Event{EventType: "auth.login_failed"})

	if len(repo.inserted) != 1 {
		t.Fatalf("inserted = %d, want 1", len(repo.inserted))
	}
	if repo.inserted[0].RemoteIP != "203.0.113.20" {
		t.Fatalf("RemoteIP = %q, want configured IP", repo.inserted[0].RemoteIP)
	}
}

type fakeRepo struct {
	inserted  []Event
	listLimit int
	cutoff    time.Time
	deleted   int64
}

func (r *fakeRepo) Insert(_ context.Context, event Event) error {
	r.inserted = append(r.inserted, event)
	return nil
}

func (r *fakeRepo) List(_ context.Context, limit int) ([]Event, error) {
	r.listLimit = limit
	return nil, nil
}

func (r *fakeRepo) DeleteOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	r.cutoff = cutoff
	return r.deleted, nil
}
