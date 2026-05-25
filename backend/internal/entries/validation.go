package entries

import "github.com/shuuuumai96/nikki-shelf/backend/internal/moods"

const (
	MaxTitleRunes = 200
	MaxBodyRunes  = 100000
	MaxTags       = 20
	MaxTagRunes   = 64
)

func validateEntry(date string, mood string) error {
	validators := []func() error{
		func() error {
			if !isDate(date) {
				return ErrInvalidInput
			}
			return nil
		},
		func() error {
			if !moods.IsValid(mood) {
				return ErrInvalidInput
			}
			return nil
		},
	}

	for _, validator := range validators {
		if err := validator(); err != nil {
			return err
		}
	}

	return nil
}

func validateEntryContent(title string, body string, tags []string) error {
	validators := []func() error{
		func() error {
			if runeCount(title) > MaxTitleRunes {
				return ErrInvalidInput
			}
			return nil
		},
		func() error {
			if runeCount(body) > MaxBodyRunes {
				return ErrInvalidInput
			}
			return nil
		},
		func() error {
			if len(tags) > MaxTags {
				return ErrInvalidInput
			}
			return nil
		},
		func() error {
			for _, tag := range tags {
				if runeCount(tag) > MaxTagRunes {
					return ErrInvalidInput
				}
			}
			return nil
		},
	}

	for _, validator := range validators {
		if err := validator(); err != nil {
			return err
		}
	}

	return nil
}

func validateExpectedVersion(version int64) error {
	if version < 1 {
		return ErrInvalidInput
	}
	return nil
}

func runeCount(value string) int {
	count := 0
	for range value {
		count++
	}
	return count
}
