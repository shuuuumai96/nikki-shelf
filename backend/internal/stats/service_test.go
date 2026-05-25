package stats

import (
	"context"
	"testing"
	"time"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/entries"
)

type fakeStatsReader struct {
	total      int
	moodCounts map[string]int
	lastDate   string
	dates      []string
	countCalls int
}

func (f *fakeStatsReader) Count(context.Context, int64, entries.EntryFilter) (int, error) {
	f.countCalls++
	return f.total, nil
}

func (f *fakeStatsReader) MoodCounts(context.Context, int64) (map[string]int, error) {
	return f.moodCounts, nil
}

func (f *fakeStatsReader) LastEntryDate(context.Context, int64) (string, error) {
	return f.lastDate, nil
}

func (f *fakeStatsReader) EntryDatesDesc(context.Context, int64, string) ([]string, error) {
	return f.dates, nil
}

func TestServiceGetCalculatesStatsWithoutFullEntryList(t *testing.T) {
	reader := &fakeStatsReader{
		total:      3,
		moodCounts: map[string]int{"calm": 2, "happy": 1},
		lastDate:   "2026-05-18",
		dates:      []string{"2026-05-18", "2026-05-17", "2026-05-15"},
	}
	service := NewService(reader)
	service.now = func() time.Time {
		return time.Date(2026, 5, 18, 12, 0, 0, 0, time.Local)
	}

	got, err := service.Get(context.Background(), 42)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if reader.countCalls != 1 {
		t.Fatalf("Count calls = %d, want 1", reader.countCalls)
	}
	if got.TotalEntries != 3 {
		t.Fatalf("TotalEntries = %d, want 3", got.TotalEntries)
	}
	if got.CurrentStreak != 2 {
		t.Fatalf("CurrentStreak = %d, want 2", got.CurrentStreak)
	}
	if got.LastEntryDate != "2026-05-18" {
		t.Fatalf("LastEntryDate = %q, want 2026-05-18", got.LastEntryDate)
	}
	if got.MoodCounts["calm"] != 2 || got.MoodCounts["happy"] != 1 {
		t.Fatalf("MoodCounts = %#v", got.MoodCounts)
	}
	if got.MoodCounts["tired"] != 0 || got.MoodCounts["sad"] != 0 || got.MoodCounts["excited"] != 0 {
		t.Fatalf("MoodCounts should include zero values for known moods: %#v", got.MoodCounts)
	}
}

func TestServiceGetHandlesEmptyEntries(t *testing.T) {
	service := NewService(&fakeStatsReader{})
	service.now = func() time.Time {
		return time.Date(2026, 5, 18, 12, 0, 0, 0, time.Local)
	}

	got, err := service.Get(context.Background(), 42)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.TotalEntries != 0 {
		t.Fatalf("TotalEntries = %d, want 0", got.TotalEntries)
	}
	if got.CurrentStreak != 0 {
		t.Fatalf("CurrentStreak = %d, want 0", got.CurrentStreak)
	}
	if got.LastEntryDate != "" {
		t.Fatalf("LastEntryDate = %q, want empty", got.LastEntryDate)
	}
}

func TestCurrentStreakWithOneDayGracePeriod(t *testing.T) {
	tests := []struct {
		name  string
		today time.Time
		dates []string
		want  int
	}{
		{
			name:  "empty entries",
			today: localDate(2026, time.May, 18),
			want:  0,
		},
		{
			name:  "only today",
			today: localDate(2026, time.May, 18),
			dates: []string{"2026-05-18"},
			want:  1,
		},
		{
			name:  "today and yesterday",
			today: localDate(2026, time.May, 18),
			dates: []string{"2026-05-18", "2026-05-17"},
			want:  2,
		},
		{
			name:  "yesterday only",
			today: localDate(2026, time.May, 18),
			dates: []string{"2026-05-17"},
			want:  1,
		},
		{
			name:  "yesterday and day before",
			today: localDate(2026, time.May, 18),
			dates: []string{"2026-05-17", "2026-05-16"},
			want:  2,
		},
		{
			name:  "gap before yesterday",
			today: localDate(2026, time.May, 18),
			dates: []string{"2026-05-16", "2026-05-15"},
			want:  0,
		},
		{
			name:  "month boundary non leap year",
			today: localDate(2026, time.March, 1),
			dates: []string{"2026-02-28", "2026-02-27"},
			want:  2,
		},
		{
			name:  "month boundary leap year",
			today: localDate(2024, time.March, 1),
			dates: []string{"2024-02-29", "2024-02-28"},
			want:  2,
		},
		{
			name:  "year boundary",
			today: localDate(2026, time.January, 1),
			dates: []string{"2025-12-31", "2025-12-30"},
			want:  2,
		},
		{
			name:  "unsorted input",
			today: localDate(2026, time.May, 18),
			dates: []string{"2026-05-15", "2026-05-17", "2026-05-16"},
			want:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := currentStreak(tt.dates, tt.today); got != tt.want {
				t.Fatalf("currentStreak() = %d, want %d", got, tt.want)
			}
		})
	}
}

func localDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, time.Local)
}
