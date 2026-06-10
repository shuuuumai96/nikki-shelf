package entries

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
)

const (
	DefaultEntriesPerPage = 50
	MaxEntriesPerPage     = 100
)

type entryCursor struct {
	// These compact JSON keys are an API compatibility boundary. The fields
	// mirror ListPage's ORDER BY entry_date DESC, id DESC.
	EntryDate string `json:"d"`
	ID        int64  `json:"id"`
}

func encodeCursor(row EntryRow) (string, error) {
	encoded, err := json.Marshal(entryCursor{EntryDate: row.EntryDate, ID: row.ID})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeCursor(value string) (entryCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return entryCursor{}, nil
	}
	content, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return entryCursor{}, ErrInvalidCursor
	}
	cursor := entryCursor{}
	if err := json.Unmarshal(content, &cursor); err != nil {
		return entryCursor{}, ErrInvalidCursor
	}
	if !isDate(cursor.EntryDate) || cursor.ID < 1 {
		return entryCursor{}, ErrInvalidCursor
	}
	return cursor, nil
}

func normalizePerPage(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultEntriesPerPage, nil
	}
	perPage, err := strconv.Atoi(value)
	if err != nil || perPage <= 0 || perPage > MaxEntriesPerPage {
		return 0, ErrInvalidInput
	}
	return perPage, nil
}
