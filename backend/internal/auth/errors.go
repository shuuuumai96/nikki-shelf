package auth

import "errors"

var (
	ErrInvalidBackup              = errors.New("check the backup archive")
	ErrInvalidCredentials         = errors.New("check the username or password")
	ErrInvalidInput               = errors.New("check the input")
	ErrInvalidSetupToken          = errors.New("check the setup token")
	ErrRestoreConfirmationMissing = errors.New("confirm restore before continuing")
	ErrRestoreCountMismatch       = errors.New("restored backup counts do not match")
	ErrRestoreFailed              = errors.New("restore failed")
	ErrRestoreInProgress          = errors.New("setup restore is in progress")
	ErrSetupLocked                = errors.New("setup is already complete")
	ErrSignupClosed               = errors.New("signup is currently disabled")
	ErrUsernameExists             = errors.New("this username is already in use")
	ErrUnauthorized               = errors.New("log in to continue")
)
