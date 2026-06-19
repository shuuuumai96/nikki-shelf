package audit

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
)

const (
	DefaultRetentionDays = 180
	DefaultListLimit     = 100
	MaxListLimit         = 200
)

type repository interface {
	Insert(ctx context.Context, event Event) error
	List(ctx context.Context, limit int) ([]Event, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type Config struct {
	RetentionDays int
	RemoteIP      func(*http.Request) string
}

type Service struct {
	repo          repository
	retentionDays int
	now           func() time.Time
	remoteIP      func(*http.Request) string
}

func NewService(repo repository, configs ...Config) *Service {
	cfg := Config{RetentionDays: DefaultRetentionDays}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = DefaultRetentionDays
	}
	if cfg.RemoteIP == nil {
		cfg.RemoteIP = directRemoteIP
	}

	return &Service{
		repo:          repo,
		retentionDays: cfg.RetentionDays,
		now:           time.Now,
		remoteIP:      cfg.RemoteIP,
	}
}

func (s *Service) Record(ctx context.Context, event Event) error {
	if s == nil || s.repo == nil {
		return nil
	}
	event = normalizeEvent(event, s.now)
	if event.EventType == "" {
		return nil
	}
	return s.repo.Insert(ctx, event)
}

func (s *Service) RecordHTTP(c echo.Context, event Event) {
	if s == nil {
		return
	}
	if event.RequestID == "" {
		event.RequestID = logx.RequestID(c)
	}
	if event.RemoteIP == "" {
		event.RemoteIP = s.remoteIP(c.Request())
	}
	if err := s.Record(c.Request().Context(), event); err != nil {
		slog.ErrorContext(c.Request().Context(), "audit event write failed",
			slog.String("event_type", event.EventType),
			slog.String("error", err.Error()),
		)
	}
}

func (s *Service) List(ctx context.Context, limit int) ([]Event, error) {
	if s == nil || s.repo == nil {
		return []Event{}, nil
	}
	return s.repo.List(ctx, normalizeLimit(limit))
}

func (s *Service) PruneExpired(ctx context.Context) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	cutoff := s.now().UTC().AddDate(0, 0, -s.retentionDays)
	return s.repo.DeleteOlderThan(ctx, cutoff)
}

func UserID(id int64) *int64 {
	return &id
}

func Metadata(items ...any) map[string]string {
	metadata := map[string]string{}
	for i := 0; i+1 < len(items); i += 2 {
		key, ok := items[i].(string)
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		metadata[key] = stringify(items[i+1])
	}
	return metadata
}

func normalizeEvent(event Event, now func() time.Time) Event {
	event.EventType = strings.TrimSpace(event.EventType)
	if strings.TrimSpace(event.Outcome) == "" {
		event.Outcome = OutcomeSucceeded
	}
	if event.Metadata == nil {
		event.Metadata = map[string]string{}
	}
	if strings.TrimSpace(event.CreatedAt) == "" {
		event.CreatedAt = now().UTC().Format(time.RFC3339)
	}
	return event
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultListLimit
	}
	if limit > MaxListLimit {
		return MaxListLimit
	}
	return limit
}

func directRemoteIP(request *http.Request) string {
	if request == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(request.RemoteAddr)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "unknown"
	}
	return ip.String()
}

func stringify(value any) string {
	switch item := value.(type) {
	case string:
		return item
	case int:
		return strconv.Itoa(item)
	case int64:
		return strconv.FormatInt(item, 10)
	case bool:
		return strconv.FormatBool(item)
	default:
		return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(fmt.Sprint(item), "\n", " "), "\r", " "))
	}
}
