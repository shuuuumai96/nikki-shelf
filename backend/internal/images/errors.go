package images

import "errors"

var (
	ErrImageNotFound      = errors.New("image not found")
	ErrInvalidImage       = errors.New("check the image file")
	ErrImageTooLarge      = errors.New("images must be 8 MB or smaller")
	ErrTooManyImages      = errors.New("each entry can have up to 3 images")
	ErrImageQuotaExceeded = errors.New("image storage quota exceeded")
)
