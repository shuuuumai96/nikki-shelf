package auth

import "errors"

var (
	ErrInvalidCredentials = errors.New("check the username or password")
	ErrInvalidInput       = errors.New("check the input")
	ErrSignupClosed       = errors.New("signup is currently disabled")
	ErrUsernameExists     = errors.New("this username is already in use")
	ErrUnauthorized       = errors.New("log in to continue")
)
