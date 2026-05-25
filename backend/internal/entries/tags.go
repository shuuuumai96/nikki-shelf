package entries

import (
	"encoding/json"
	"slices"
	"strings"
)

func normalizeTags(tags []string) []string {
	seen := map[string]bool{}
	result := []string{}

	for _, tag := range tags {
		value := strings.TrimSpace(tag)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}

	return result
}

func decodeTags(value string) []string {
	tags := []string{}
	if err := json.Unmarshal([]byte(value), &tags); err != nil {
		return []string{}
	}
	return tags
}

func uniqueSortedTags(rows []EntryRow) []string {
	seen := map[string]bool{}
	tags := []string{}
	for _, row := range rows {
		for _, tag := range decodeTags(row.TagsJSON) {
			if seen[tag] {
				continue
			}
			seen[tag] = true
			tags = append(tags, tag)
		}
	}

	slices.Sort(tags)
	return tags
}
