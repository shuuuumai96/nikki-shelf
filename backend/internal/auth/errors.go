package auth

import "errors"

var (
	ErrInvalidCredentials = errors.New("ユーザー名またはパスワードを確認してください")
	ErrInvalidInput       = errors.New("入力内容を確認してください")
	ErrSignupClosed       = errors.New("サインアップは現在停止されています")
	ErrUsernameExists     = errors.New("このユーザー名はすでに使われています")
	ErrUnauthorized       = errors.New("ログインしてください")
)
