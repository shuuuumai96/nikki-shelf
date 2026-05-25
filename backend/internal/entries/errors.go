package entries

import "errors"

var (
	ErrInvalidCursor = errors.New("ページ指定を確認してください")
	ErrDateExists    = errors.New("その日の日記はすでにあります")
	ErrInvalidInput  = errors.New("日記の入力内容を確認してください")
	ErrNotFound      = errors.New("日記が見つかりません")
	ErrStaleVersion  = errors.New("日記が別の画面で更新されています")
)
