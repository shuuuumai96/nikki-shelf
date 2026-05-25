package entries

import (
	"fmt"
	"strings"
)

type filterBuilder func(*[]string, *[]any, string)

var filterOrder = []string{"query", "tag", "mood", "from", "to"}

var filterBuilders = map[string]filterBuilder{
	"query": addQueryFilter,
	"tag":   addTagFilter,
	"mood":  addMoodFilter,
	"from":  addFromFilter,
	"to":    addToFilter,
}

func buildFilterClauses(userID int64, filter EntryFilter) ([]string, []any) {
	clauses := []string{"user_id = $1"}
	args := []any{userID}
	values := filterValues(filter)

	for _, key := range filterOrder {
		filterBuilders[key](&clauses, &args, values[key])
	}

	return clauses, args
}

func addQueryFilter(clauses *[]string, args *[]any, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}

	pattern := "%" + strings.ToLower(value) + "%"
	left := appendArg(args, pattern)
	right := appendArg(args, pattern)
	*clauses = append(*clauses, "(LOWER(title) LIKE "+left+" OR LOWER(body) LIKE "+right+")")
}

func addTagFilter(clauses *[]string, args *[]any, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}

	// Tag filters match one exact tag inside the JSONB array. Entries may have
	// multiple tags; a single tag filter matches when any stored tag equals it.
	*clauses = append(*clauses, "tags_json ? "+appendArg(args, value))
}

func addMoodFilter(clauses *[]string, args *[]any, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}

	*clauses = append(*clauses, "mood = "+appendArg(args, value))
}

func addFromFilter(clauses *[]string, args *[]any, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}

	*clauses = append(*clauses, "entry_date >= "+appendArg(args, value))
}

func addToFilter(clauses *[]string, args *[]any, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}

	*clauses = append(*clauses, "entry_date <= "+appendArg(args, value))
}

func filterValues(filter EntryFilter) map[string]string {
	return map[string]string{
		"query": filter.Query,
		"tag":   filter.Tag,
		"mood":  filter.Mood,
		"from":  filter.From,
		"to":    filter.To,
	}
}

func appendArg(args *[]any, value any) string {
	*args = append(*args, value)
	return fmt.Sprintf("$%d", len(*args))
}
