package stats

import (
	"time"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/moods"
)

func currentStreak(entryDates []string, today time.Time) int {
	dates := map[string]bool{}
	for _, date := range entryDates {
		dates[date] = true
	}

	cursor := today
	if !dates[cursor.Format(time.DateOnly)] {
		cursor = cursor.AddDate(0, 0, -1)
		if !dates[cursor.Format(time.DateOnly)] {
			return 0
		}
	}

	streak := 0
	for {
		if !dates[cursor.Format(time.DateOnly)] {
			break
		}
		streak++
		cursor = cursor.AddDate(0, 0, -1)
	}

	return streak
}

func moodCounts(source map[string]int) map[string]int {
	counts := map[string]int{}
	for _, spec := range moods.List() {
		counts[spec.Key] = 0
	}
	for mood, count := range source {
		counts[mood] = count
	}
	return counts
}
