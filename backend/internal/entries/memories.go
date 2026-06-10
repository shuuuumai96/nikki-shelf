package entries

import (
	"context"
	"strings"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/moods"
)

const (
	defaultMemoryLimit = 3
	maxMemoryLimit     = 12
)

func normalizeMemoryFilter(filter MemoryFilter) (MemoryFilter, error) {
	filter.Date = strings.TrimSpace(filter.Date)
	if !isDate(filter.Date) {
		return filter, ErrInvalidInput
	}

	if filter.Limit <= 0 {
		filter.Limit = defaultMemoryLimit
	}
	if filter.Limit > maxMemoryLimit {
		filter.Limit = maxMemoryLimit
	}

	filter.ExcludeMoods = normalizeMemoryMoods(filter.ExcludeMoods)
	for _, mood := range filter.ExcludeMoods {
		if !moods.IsValid(mood) {
			return filter, ErrInvalidInput
		}
	}

	return filter, nil
}

func normalizeMemoryMoods(source []string) []string {
	seen := map[string]bool{}
	moods := []string{}
	for _, value := range source {
		mood := strings.TrimSpace(value)
		if mood == "" || seen[mood] {
			continue
		}
		seen[mood] = true
		moods = append(moods, mood)
	}
	return moods
}

func memoryItem(row searchRow) MemoryItem {
	imageCount := row.ImageCount
	return MemoryItem{
		ID:         row.ID,
		EntryDate:  row.EntryDate,
		Title:      row.Title,
		Preview:    previewText(row.Body, row.Title, ""),
		Mood:       row.Mood,
		Tags:       decodeTags(row.TagsJSON),
		HasImage:   imageCount > 0,
		ImageCount: imageCount,
		UpdatedAt:  row.UpdatedAt,
	}
}

func (r *Repository) Memories(ctx context.Context, userID int64, filter MemoryFilter) ([]searchRow, error) {
	clauses, args := buildMemoryClauses(userID, filter)

	query := `SELECT e.id, e.entry_date, e.title, e.body, e.mood, e.tags_json, COUNT(i.id) AS image_count, e.updated_at
		FROM entries e
		LEFT JOIN images i ON i.entry_id = e.id`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " GROUP BY e.id, e.entry_date, e.title, e.body, e.mood, e.tags_json, e.updated_at"
	seed := appendArg(&args, filter.Date)
	// The selected date seeds a stable pseudo-random order for one day's memory
	// shelf, while letting the shelf change naturally from day to day.
	query += " ORDER BY md5(e.id::text || ':' || " + seed + ")"
	query += " LIMIT " + appendArg(&args, filter.Limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []searchRow{}
	for rows.Next() {
		row, err := scanSearchRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

func buildMemoryClauses(userID int64, filter MemoryFilter) ([]string, []any) {
	clauses := []string{
		"e.user_id = $1",
		"e.entry_date < $2",
		"(NULLIF(BTRIM(e.title), '') IS NOT NULL OR NULLIF(BTRIM(e.body), '') IS NOT NULL OR EXISTS (SELECT 1 FROM images memory_images WHERE memory_images.entry_id = e.id))",
	}
	args := []any{userID, filter.Date}

	if len(filter.ExcludeMoods) > 0 {
		placeholders := make([]string, 0, len(filter.ExcludeMoods))
		for _, mood := range filter.ExcludeMoods {
			placeholders = append(placeholders, appendArg(&args, mood))
		}
		clauses = append(clauses, "e.mood NOT IN ("+strings.Join(placeholders, ", ")+")")
	}

	return clauses, args
}
