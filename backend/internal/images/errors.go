package images

import "errors"

var (
	ErrImageNotFound      = errors.New("画像が見つかりません")
	ErrInvalidImage       = errors.New("画像ファイルを確認してください")
	ErrImageTooLarge      = errors.New("画像は8MB以下にしてください")
	ErrTooManyImages      = errors.New("画像は1つの日記につき3枚までです")
	ErrImageQuotaExceeded = errors.New("画像の保存容量を超えています")
)
