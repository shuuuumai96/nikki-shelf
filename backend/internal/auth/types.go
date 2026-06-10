package auth

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CSRFToken string `json:"csrfToken,omitempty"`
}

type ConfigResponse struct {
	SignupMode      string `json:"signupMode"`
	SignupAvailable bool   `json:"signupAvailable"`
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type DeleteAccountInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AccountDeletionResult struct {
	// ImageFilePaths are deleted after the account rows are no longer reachable.
	ImageFilePaths []string
	// RemainingUsers lets callers distinguish ordinary logout from setup reopen.
	RemainingUsers int
}

type SetupStatusResponse struct {
	NeedsSetup         bool `json:"needsSetup"`
	SetupLocked        bool `json:"setupLocked"`
	CanCreateOwner     bool `json:"canCreateOwner"`
	CanRestoreBackup   bool `json:"canRestoreBackup"`
	RequiresSetupToken bool `json:"requiresSetupToken"`
	RestoreInProgress  bool `json:"restoreInProgress"`
}

type SetupOwnerInput struct {
	SetupToken string `json:"setupToken"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

type SetupRestoreVerifyInput struct {
	SetupToken  string
	ArchivePath string
	ArchiveSize int64
}

type SetupRestoreInput struct {
	SetupToken     string
	ArchivePath    string
	ArchiveSize    int64
	ConfirmRestore bool
}

type SetupRestoreVerifyResponse struct {
	Valid           bool     `json:"valid"`
	BackupCreatedAt string   `json:"backupCreatedAt"`
	NikkiVersion    string   `json:"nikkiVersion"`
	SchemaVersion   string   `json:"schemaVersion"`
	EntryCount      int      `json:"entryCount"`
	ImageCount      int      `json:"imageCount"`
	BackupSizeBytes int64    `json:"backupSizeBytes"`
	Warnings        []string `json:"warnings"`
}

type SetupRestoreResponse struct {
	Restored   bool `json:"restored"`
	EntryCount int  `json:"entryCount"`
	ImageCount int  `json:"imageCount"`
}

type UserRow struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
	CreatedAt    string
}

type SessionResult struct {
	User      User
	Token     string
	CSRFToken string
	ExpiresAt string
}
