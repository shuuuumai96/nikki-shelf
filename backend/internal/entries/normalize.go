package entries

import (
	"strings"
	"time"
)

func normalizeCreateInput(input CreateInput, today func() string) (CreateInput, error) {
	input.EntryDate = normalizeDate(input.EntryDate, today)
	input.Title = strings.TrimSpace(input.Title)
	input.Body = strings.TrimSpace(input.Body)
	input.Mood = normalizeMood(input.Mood)
	input.Tags = normalizeTags(input.Tags)

	if err := validateEntry(input.EntryDate, input.Mood); err != nil {
		return input, err
	}
	return input, validateEntryContent(input.Title, input.Body, input.Tags)
}

func normalizeCreate(input CreateInput, today func() string) (CreateInput, error) {
	return normalizeCreateInput(input, today)
}

func normalizeUpdateInput(input UpdateInput, today func() string) (UpdateInput, error) {
	input.EntryDate = normalizeDate(input.EntryDate, today)
	input.Title = strings.TrimSpace(input.Title)
	input.Body = strings.TrimSpace(input.Body)
	input.Mood = normalizeMood(input.Mood)
	input.Tags = normalizeTags(input.Tags)

	if err := validateEntry(input.EntryDate, input.Mood); err != nil {
		return input, err
	}
	if err := validateEntryContent(input.Title, input.Body, input.Tags); err != nil {
		return input, err
	}
	return input, validateExpectedVersion(input.Version)
}

func normalizeUpdate(input UpdateInput, today func() string) (UpdateInput, error) {
	return normalizeUpdateInput(input, today)
}

func normalizeDate(date string, today func() string) string {
	value := strings.TrimSpace(date)
	if value != "" {
		return value
	}

	// Diary dates are local calendar labels, not instants. Keep them as
	// YYYY-MM-DD text so existing entries keep their user-facing day.
	return today()
}

func normalizeMood(mood string) string {
	value := strings.TrimSpace(mood)
	if value != "" {
		return value
	}
	return "calm"
}

func isDate(value string) bool {
	_, err := time.Parse(time.DateOnly, value)
	return err == nil
}
