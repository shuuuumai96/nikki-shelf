package entries

import (
	"context"
	"database/sql"
	"strings"
	"unicode/utf8"
)

const (
	defaultSearchLimit = 50
	maxSearchLimit     = 100
	previewMaxRunes    = 160
)

type searchRow struct {
	ID         int64
	EntryDate  string
	Title      string
	Body       string
	Mood       string
	TagsJSON   string
	ImageCount int
	UpdatedAt  string
}

func normalizeSearchFilter(filter SearchFilter) (SearchFilter, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.From = strings.TrimSpace(filter.From)
	filter.To = strings.TrimSpace(filter.To)
	filter.Mood = strings.TrimSpace(filter.Mood)
	filter.Tag = strings.TrimSpace(filter.Tag)
	filter.HasImage = strings.TrimSpace(filter.HasImage)

	if filter.From != "" && !isDate(filter.From) {
		return filter, ErrInvalidInput
	}
	if filter.To != "" && !isDate(filter.To) {
		return filter, ErrInvalidInput
	}
	if filter.Limit <= 0 {
		filter.Limit = defaultSearchLimit
	}
	if filter.Limit > maxSearchLimit {
		filter.Limit = maxSearchLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	return filter, nil
}

func searchHasActiveFilter(filter SearchFilter) bool {
	return filter.Query != "" || filter.From != "" || filter.To != "" || filter.Mood != "" || filter.Tag != "" || filter.HasImage != ""
}

func searchResult(row searchRow, query string) SearchResult {
	imageCount := row.ImageCount
	return SearchResult{
		ID:         row.ID,
		EntryDate:  row.EntryDate,
		Title:      row.Title,
		Preview:    previewText(row.Body, row.Title, query),
		Mood:       row.Mood,
		Tags:       decodeTags(row.TagsJSON),
		HasImage:   imageCount > 0,
		ImageCount: imageCount,
		UpdatedAt:  row.UpdatedAt,
	}
}

func (r *Repository) Search(ctx context.Context, userID int64, filter SearchFilter) ([]searchRow, error) {
	clauses, args := buildSearchClauses(userID, filter)

	query := `SELECT e.id, e.entry_date, e.title, e.body, e.mood, e.tags_json, COUNT(i.id) AS image_count, e.updated_at
		FROM entries e
		LEFT JOIN images i ON i.entry_id = e.id`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " GROUP BY e.id, e.entry_date, e.title, e.body, e.mood, e.tags_json, e.updated_at"
	query += " ORDER BY e.entry_date DESC, e.id DESC"
	query += " LIMIT " + appendArg(&args, filter.Limit)
	query += " OFFSET " + appendArg(&args, filter.Offset)

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

func buildSearchClauses(userID int64, filter SearchFilter) ([]string, []any) {
	clauses := []string{"e.user_id = $1"}
	args := []any{userID}

	if filter.Query != "" {
		pattern := ilikePattern(filter.Query)
		placeholder := appendArg(&args, pattern)
		// Keep the broad search as ILIKE for predictable small-instance behavior;
		// exact filters below narrow the result set when precision matters.
		clauses = append(clauses, `(e.title ILIKE `+placeholder+` ESCAPE '\' OR e.body ILIKE `+placeholder+` ESCAPE '\' OR e.mood ILIKE `+placeholder+` ESCAPE '\' OR e.tags_json::text ILIKE `+placeholder+` ESCAPE '\')`)
	}
	if filter.From != "" {
		clauses = append(clauses, "e.entry_date >= "+appendArg(&args, filter.From))
	}
	if filter.To != "" {
		clauses = append(clauses, "e.entry_date <= "+appendArg(&args, filter.To))
	}
	if filter.Mood != "" {
		clauses = append(clauses, "e.mood = "+appendArg(&args, filter.Mood))
	}
	if filter.Tag != "" {
		// Exact tag filtering is separate from the broader text query above.
		// Entries with multiple tags match when any stored tag equals filter.Tag.
		clauses = append(clauses, "e.tags_json ? "+appendArg(&args, filter.Tag))
	}
	if hasImageValue, ok := parseSearchBool(filter.HasImage); ok {
		exists := "EXISTS"
		if !hasImageValue {
			exists = "NOT EXISTS"
		}
		clauses = append(clauses, exists+" (SELECT 1 FROM images search_images WHERE search_images.entry_id = e.id)")
	}

	return clauses, args
}

func scanSearchRow(scanner rowScanner) (searchRow, error) {
	row := searchRow{}
	err := scanner.Scan(
		&row.ID,
		&row.EntryDate,
		&row.Title,
		&row.Body,
		&row.Mood,
		&row.TagsJSON,
		&row.ImageCount,
		&row.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return searchRow{}, err
	}
	return row, err
}

func ilikePattern(value string) string {
	return "%" + escapeILike(strings.TrimSpace(value)) + "%"
}

func escapeILike(value string) string {
	// User text should not turn into SQL wildcards. The query clauses use
	// ESCAPE '\' so literal %, _, and backslash are searchable.
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func parseSearchBool(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "y", "on":
		return true, true
	case "false", "0", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func previewText(body string, title string, query string) string {
	source := compactText(body)
	if source == "" {
		source = compactText(title)
	}
	if source == "" {
		return ""
	}

	start := previewStart(source, query)
	runes := []rune(source)
	if start >= len(runes) {
		start = 0
	}

	end := start + previewMaxRunes
	if end > len(runes) {
		end = len(runes)
	}
	preview := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		preview = "..." + preview
	}
	if end < len(runes) {
		preview += "..."
	}
	return preview
}

func compactText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func previewStart(source string, query string) int {
	query = compactText(query)
	if query == "" {
		return 0
	}

	sourceLower := strings.ToLower(source)
	queryLower := strings.ToLower(query)
	byteIndex := strings.Index(sourceLower, queryLower)
	if byteIndex < 0 {
		return 0
	}

	matchRune := utf8.RuneCountInString(source[:byteIndex])
	if matchRune <= 40 {
		return 0
	}
	// Center the preview near the first query hit without splitting multibyte
	// text; byteIndex came from strings.Index, but slicing uses rune offsets.
	return matchRune - 40
}
