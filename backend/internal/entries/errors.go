package entries

import "errors"

var (
	ErrInvalidCursor = errors.New("check the page cursor")
	ErrDateExists    = errors.New("an entry already exists for that day")
	ErrInvalidInput  = errors.New("check the diary input")
	ErrNotFound      = errors.New("entry not found")
	ErrStaleVersion  = errors.New("entry was updated in another session")
)
