package stats

import (
	"context"
	"time"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/entries"
)

type EntryStatsReader interface {
	Count(ctx context.Context, userID int64, filter entries.EntryFilter) (int, error)
	MoodCounts(ctx context.Context, userID int64) (map[string]int, error)
	LastEntryDate(ctx context.Context, userID int64) (string, error)
	EntryDatesDesc(ctx context.Context, userID int64, toDate string) ([]string, error)
}

type Service struct {
	repo EntryStatsReader
	now  func() time.Time
}

func NewService(repo EntryStatsReader) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) Get(ctx context.Context, userID int64) (Response, error) {
	total, err := s.repo.Count(ctx, userID, entries.EntryFilter{})
	if err != nil {
		return Response{}, err
	}

	counts, err := s.repo.MoodCounts(ctx, userID)
	if err != nil {
		return Response{}, err
	}

	lastDate, err := s.repo.LastEntryDate(ctx, userID)
	if err != nil {
		return Response{}, err
	}

	today := s.now()
	dates, err := s.repo.EntryDatesDesc(ctx, userID, today.Format(time.DateOnly))
	if err != nil {
		return Response{}, err
	}

	return Response{
		TotalEntries:  total,
		CurrentStreak: currentStreak(dates, today),
		MoodCounts:    moodCounts(counts),
		LastEntryDate: lastDate,
	}, nil
}
