package entries

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeCreateDefaultsAndTrims(t *testing.T) {
	input := CreateInput{
		EntryDate: " ",
		Title:     "  A quiet day  ",
		Body:      "  Body text  ",
		Mood:      "",
		Tags:      []string{"work", " work ", "", "life", "work"},
	}

	got, err := normalizeCreate(input, func() string { return "2026-05-18" })
	if err != nil {
		t.Fatalf("normalizeCreate() error = %v", err)
	}

	want := CreateInput{
		EntryDate: "2026-05-18",
		Title:     "A quiet day",
		Body:      "Body text",
		Mood:      "calm",
		Tags:      []string{"work", "life"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeCreate() = %#v, want %#v", got, want)
	}
}

func TestNormalizeUpdateValidation(t *testing.T) {
	tests := []struct {
		name  string
		input UpdateInput
	}{
		{
			name:  "invalid date",
			input: UpdateInput{EntryDate: "2026/05/18", Mood: "calm"},
		},
		{
			name:  "invalid mood",
			input: UpdateInput{EntryDate: "2026-05-18", Mood: "sleepy"},
		},
		{
			name:  "missing expected version",
			input: UpdateInput{EntryDate: "2026-05-18", Mood: "calm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := normalizeUpdate(tt.input, func() string { return "2026-05-18" }); err != ErrInvalidInput {
				t.Fatalf("normalizeUpdate() error = %v, want %v", err, ErrInvalidInput)
			}
		})
	}
}

func TestNormalizeTagsDeduplicatesAfterTrimming(t *testing.T) {
	got := normalizeTags([]string{" diary ", "diary", "", "travel", " travel"})
	want := []string{"diary", "travel"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeTags() = %#v, want %#v", got, want)
	}
}

func TestFilterBuildersPreserveCurrentSQLFragments(t *testing.T) {
	filter := EntryFilter{
		Query: " tea ",
		Tag:   "life",
		Mood:  "calm",
		From:  "2026-05-01",
		To:    "2026-05-31",
	}
	clauses := []string{}
	args := []any{}
	values := filterValues(filter)

	for _, key := range filterOrder {
		filterBuilders[key](&clauses, &args, values[key])
	}

	wantClauses := []string{
		"(LOWER(title) LIKE $1 OR LOWER(body) LIKE $2)",
		"tags_json ? $3",
		"mood = $4",
		"entry_date >= $5",
		"entry_date <= $6",
	}
	wantArgs := []any{"%tea%", "%tea%", "life", "calm", "2026-05-01", "2026-05-31"}

	if !reflect.DeepEqual(clauses, wantClauses) {
		t.Fatalf("clauses = %#v, want %#v", clauses, wantClauses)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestTagFilterUsesExactJSONBMembership(t *testing.T) {
	clauses := []string{}
	args := []any{}

	addTagFilter(&clauses, &args, " cat ")

	wantClauses := []string{"tags_json ? $1"}
	wantArgs := []any{"cat"}

	if !reflect.DeepEqual(clauses, wantClauses) {
		t.Fatalf("clauses = %#v, want %#v", clauses, wantClauses)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestTagFilterMatchesAnyExactTagInJSONBArray(t *testing.T) {
	clauses, args := buildFilterClauses(42, EntryFilter{Tag: "cat"})

	wantClauses := []string{
		"user_id = $1",
		"tags_json ? $2",
	}
	wantArgs := []any{int64(42), "cat"}

	if !reflect.DeepEqual(clauses, wantClauses) {
		t.Fatalf("clauses = %#v, want %#v", clauses, wantClauses)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestFilterBuildersIgnoreBlankValues(t *testing.T) {
	clauses := []string{}
	args := []any{}
	values := filterValues(EntryFilter{})

	for _, key := range filterOrder {
		filterBuilders[key](&clauses, &args, values[key])
	}

	if len(clauses) != 0 {
		t.Fatalf("clauses = %#v, want empty", clauses)
	}
	if len(args) != 0 {
		t.Fatalf("args = %#v, want empty", args)
	}
}

func TestSearchClausesIncludeSupportedFieldsAndFilters(t *testing.T) {
	filter, err := normalizeSearchFilter(SearchFilter{
		Query:    " cat ",
		Tag:      "home",
		Mood:     "calm",
		From:     "2026-05-01",
		To:       "2026-05-31",
		HasImage: "true",
		Limit:    500,
		Offset:   -1,
	})
	if err != nil {
		t.Fatalf("normalizeSearchFilter() error = %v", err)
	}

	clauses, args := buildSearchClauses(42, filter)
	wantClauses := []string{
		"e.user_id = $1",
		"(e.title ILIKE $2 ESCAPE '\\' OR e.body ILIKE $2 ESCAPE '\\' OR e.mood ILIKE $2 ESCAPE '\\' OR e.tags_json::text ILIKE $2 ESCAPE '\\')",
		"e.entry_date >= $3",
		"e.entry_date <= $4",
		"e.mood = $5",
		"e.tags_json ? $6",
		"EXISTS (SELECT 1 FROM images search_images WHERE search_images.entry_id = e.id)",
	}
	wantArgs := []any{int64(42), "%cat%", "2026-05-01", "2026-05-31", "calm", "home"}

	if !reflect.DeepEqual(clauses, wantClauses) {
		t.Fatalf("clauses = %#v, want %#v", clauses, wantClauses)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
	if filter.Limit != maxSearchLimit || filter.Offset != 0 {
		t.Fatalf("limit/offset = %d/%d, want %d/0", filter.Limit, filter.Offset, maxSearchLimit)
	}
}

func TestSearchWildcardCharactersAreEscaped(t *testing.T) {
	got := ilikePattern(`100%_cat\day`)
	want := `%100\%\_cat\\day%`

	if got != want {
		t.Fatalf("ilikePattern() = %q, want %q", got, want)
	}
}

func TestSearchPreviewCentersQuery(t *testing.T) {
	body := "This morning was ordinary. Then I wrote a long note about the train station, a blue notebook, and the cat sitting near the window after work."
	got := previewText(body, "", "cat")

	if !strings.Contains(got, "cat sitting") {
		t.Fatalf("preview = %q, want query context", got)
	}
	if len([]rune(got)) > previewMaxRunes+6 {
		t.Fatalf("preview length = %d, want compact", len([]rune(got)))
	}
}

func TestMemoryFilterValidationAndClauses(t *testing.T) {
	filter, err := normalizeMemoryFilter(MemoryFilter{
		Date:         " 2026-06-10 ",
		ExcludeMoods: []string{"sad", " sad ", "", "tired"},
		Limit:        500,
	})
	if err != nil {
		t.Fatalf("normalizeMemoryFilter() error = %v", err)
	}

	clauses, args := buildMemoryClauses(42, filter)
	wantClauses := []string{
		"e.user_id = $1",
		"e.entry_date < $2",
		"(NULLIF(BTRIM(e.title), '') IS NOT NULL OR NULLIF(BTRIM(e.body), '') IS NOT NULL OR EXISTS (SELECT 1 FROM images memory_images WHERE memory_images.entry_id = e.id))",
		"e.mood NOT IN ($3, $4)",
	}
	wantArgs := []any{int64(42), "2026-06-10", "sad", "tired"}

	if !reflect.DeepEqual(clauses, wantClauses) {
		t.Fatalf("clauses = %#v, want %#v", clauses, wantClauses)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
	if filter.Limit != maxMemoryLimit {
		t.Fatalf("limit = %d, want %d", filter.Limit, maxMemoryLimit)
	}
}

func TestMemoryFilterRejectsInvalidMood(t *testing.T) {
	_, err := normalizeMemoryFilter(MemoryFilter{
		Date:         "2026-06-10",
		ExcludeMoods: []string{"sleepy"},
	})
	if err != ErrInvalidInput {
		t.Fatalf("normalizeMemoryFilter() error = %v, want %v", err, ErrInvalidInput)
	}
}
