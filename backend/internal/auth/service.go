package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const SessionTTL = 30 * 24 * time.Hour
const RoleOwner = "owner"
const RoleUser = "user"

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,39}$`)

type Service struct {
	repo                    serviceRepository
	allowAdditionalSignups  bool
	allowFirstUserSetup     bool
	firstUserBootstrapToken string
	restorePostgres         func(context.Context, string) error
	uploadDir               string
	accountFiles            accountFileDeleter
	now                     func() time.Time
}

type serviceRepository interface {
	CountUsers(ctx context.Context) (int, error)
	CountEntries(ctx context.Context) (int, error)
	CountImages(ctx context.Context) (int, error)
	SetupLocked(ctx context.Context) (bool, error)
	SetupRestoreInProgress(ctx context.Context) (bool, error)
	BeginSetupRestore(ctx context.Context) error
	ClearSetupRestoreInProgress(ctx context.Context) error
	FinishSetupRestore(ctx context.Context) error
	CreateUser(ctx context.Context, username string, passwordHash string, now time.Time) (UserRow, error)
	CreateFirstOwner(ctx context.Context, username string, passwordHash string, now time.Time) (UserRow, error)
	GetUserByUsername(ctx context.Context, username string) (UserRow, error)
	GetUserByID(ctx context.Context, id int64) (UserRow, error)
	CreateSession(ctx context.Context, userID int64, tokenHash string, csrfHash string, expiresAt time.Time, now time.Time) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (SessionRow, error)
	UpdateSessionCSRF(ctx context.Context, tokenHash string, csrfHash string) error
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) error
	ClaimLegacyEntries(ctx context.Context, userID int64) error
	DeleteAccount(ctx context.Context, userID int64) (AccountDeletionResult, error)
}

type accountFileDeleter interface {
	Delete(ctx context.Context, path string) error
}

type ServiceConfig struct {
	AllowAdditionalSignups  bool
	AllowFirstUserSetup     bool
	FirstUserBootstrapToken string
	DatabaseURL             string
	UploadDir               string
	PgRestorePath           string
	AccountFiles            accountFileDeleter
}

func NewService(repo serviceRepository, configs ...ServiceConfig) *Service {
	cfg := ServiceConfig{}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	return &Service{
		repo:                    repo,
		allowAdditionalSignups:  cfg.AllowAdditionalSignups,
		allowFirstUserSetup:     cfg.AllowFirstUserSetup,
		firstUserBootstrapToken: strings.TrimSpace(cfg.FirstUserBootstrapToken),
		restorePostgres:         pgRestoreRunner(cfg.DatabaseURL, cfg.PgRestorePath),
		uploadDir:               cfg.UploadDir,
		accountFiles:            cfg.AccountFiles,
		now:                     time.Now,
	}
}

func (s *Service) Signup(ctx context.Context, input Credentials, bootstrapToken string) (SessionResult, error) {
	if inProgress, err := s.SetupRestoreInProgress(ctx); err != nil {
		return SessionResult{}, err
	} else if inProgress {
		return SessionResult{}, ErrRestoreInProgress
	}

	username, password, err := normalizeCredentials(input)
	if err != nil {
		return SessionResult{}, err
	}

	userCount, err := s.repo.CountUsers(ctx)
	if err != nil {
		return SessionResult{}, err
	}
	firstUserSignup := userCount == 0
	if firstUserSignup && !s.validFirstUserBootstrapToken(bootstrapToken) {
		return SessionResult{}, ErrSignupClosed
	}
	if signupClosed(userCount, s.allowAdditionalSignups) {
		return SessionResult{}, ErrSignupClosed
	}

	if firstUserSignup {
		return s.createFirstOwner(ctx, username, password)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return SessionResult{}, err
	}

	row, err := s.repo.CreateUser(ctx, username, string(hash), s.now())
	if err != nil {
		return SessionResult{}, err
	}

	return s.startSession(ctx, row)
}

func (s *Service) Config(ctx context.Context) (ConfigResponse, error) {
	userCount, err := s.repo.CountUsers(ctx)
	if err != nil {
		return ConfigResponse{}, err
	}

	mode := signupMode(userCount, s.allowFirstUserSetup, s.allowAdditionalSignups)
	return ConfigResponse{
		SignupMode:      mode,
		SignupAvailable: mode != "closed",
	}, nil
}

func (s *Service) SetupStatus(ctx context.Context) (SetupStatusResponse, error) {
	userCount, err := s.repo.CountUsers(ctx)
	if err != nil {
		return SetupStatusResponse{}, err
	}

	locked, err := s.repo.SetupLocked(ctx)
	if err != nil {
		return SetupStatusResponse{}, err
	}

	restoreInProgress, err := s.repo.SetupRestoreInProgress(ctx)
	if err != nil {
		return SetupStatusResponse{}, err
	}

	needsSetup := userCount == 0
	setupLocked := userCount > 0 || locked || restoreInProgress
	return SetupStatusResponse{
		NeedsSetup:         needsSetup,
		SetupLocked:        setupLocked,
		CanCreateOwner:     needsSetup && !setupLocked,
		CanRestoreBackup:   needsSetup && !setupLocked,
		RequiresSetupToken: true,
		RestoreInProgress:  restoreInProgress,
	}, nil
}

func (s *Service) CreateFirstOwner(ctx context.Context, input SetupOwnerInput) (SessionResult, error) {
	status, err := s.SetupStatus(ctx)
	if err != nil {
		return SessionResult{}, err
	}
	if status.RestoreInProgress {
		return SessionResult{}, ErrRestoreInProgress
	}
	if !status.CanCreateOwner {
		return SessionResult{}, ErrSetupLocked
	}

	username, password, err := normalizeCredentials(Credentials{
		Username: input.Username,
		Password: input.Password,
	})
	if err != nil {
		return SessionResult{}, err
	}

	if !s.validFirstUserBootstrapToken(input.SetupToken) {
		return SessionResult{}, ErrInvalidSetupToken
	}

	return s.createFirstOwner(ctx, username, password)
}

func (s *Service) VerifySetupRestore(ctx context.Context, input SetupRestoreVerifyInput) (SetupRestoreVerifyResponse, error) {
	status, err := s.SetupStatus(ctx)
	if err != nil {
		return SetupRestoreVerifyResponse{}, err
	}
	if status.RestoreInProgress {
		return SetupRestoreVerifyResponse{}, ErrRestoreInProgress
	}
	if !status.CanRestoreBackup {
		return SetupRestoreVerifyResponse{}, ErrSetupLocked
	}
	if !s.validFirstUserBootstrapToken(input.SetupToken) {
		return SetupRestoreVerifyResponse{}, ErrInvalidSetupToken
	}

	backup, err := prepareOperationalBackup(input.ArchivePath, input.ArchiveSize)
	if err != nil {
		return SetupRestoreVerifyResponse{}, err
	}
	defer backup.cleanup()

	return backup.verifyResponse(), nil
}

func (s *Service) RestoreSetupBackup(ctx context.Context, input SetupRestoreInput) (SetupRestoreResponse, error) {
	if !input.ConfirmRestore {
		return SetupRestoreResponse{}, ErrRestoreConfirmationMissing
	}

	status, err := s.SetupStatus(ctx)
	if err != nil {
		return SetupRestoreResponse{}, err
	}
	if status.RestoreInProgress {
		return SetupRestoreResponse{}, ErrRestoreInProgress
	}
	if !status.CanRestoreBackup {
		return SetupRestoreResponse{}, ErrSetupLocked
	}
	if !s.validFirstUserBootstrapToken(input.SetupToken) {
		return SetupRestoreResponse{}, ErrInvalidSetupToken
	}

	if err := s.repo.BeginSetupRestore(ctx); err != nil {
		return SetupRestoreResponse{}, err
	}
	restoreSucceeded := false
	defer func() {
		if !restoreSucceeded {
			_ = s.repo.ClearSetupRestoreInProgress(context.Background())
		}
	}()

	backup, err := prepareOperationalBackup(input.ArchivePath, input.ArchiveSize)
	if err != nil {
		return SetupRestoreResponse{}, err
	}
	defer backup.cleanup()

	if err := s.restorePostgres(ctx, backup.postgresDumpPath); err != nil {
		return SetupRestoreResponse{}, ErrRestoreFailed
	}

	if err := extractUploadsTar(backup.uploadsTarPath, s.uploadDir); err != nil {
		return SetupRestoreResponse{}, err
	}

	entryCount, err := s.repo.CountEntries(ctx)
	if err != nil {
		return SetupRestoreResponse{}, err
	}
	imageCount, err := s.repo.CountImages(ctx)
	if err != nil {
		return SetupRestoreResponse{}, err
	}
	userCount, err := s.repo.CountUsers(ctx)
	if err != nil {
		return SetupRestoreResponse{}, err
	}
	if userCount == 0 {
		return SetupRestoreResponse{}, ErrRestoreCountMismatch
	}
	if backup.manifest.EntryCount != nil && entryCount != *backup.manifest.EntryCount {
		return SetupRestoreResponse{}, ErrRestoreCountMismatch
	}
	if backup.manifest.ImageCount != nil && imageCount != *backup.manifest.ImageCount {
		return SetupRestoreResponse{}, ErrRestoreCountMismatch
	}

	if err := s.repo.FinishSetupRestore(ctx); err != nil {
		return SetupRestoreResponse{}, err
	}
	restoreSucceeded = true

	return SetupRestoreResponse{
		Restored:   true,
		EntryCount: entryCount,
		ImageCount: imageCount,
	}, nil
}

func (s *Service) SetupRestoreInProgress(ctx context.Context) (bool, error) {
	return s.repo.SetupRestoreInProgress(ctx)
}

func (s *Service) validFirstUserBootstrapToken(token string) bool {
	expected := strings.TrimSpace(s.firstUserBootstrapToken)
	provided := strings.TrimSpace(token)
	if expected == "" || provided == "" {
		return false
	}
	expectedHash := hashToken(expected)
	providedHash := hashToken(provided)
	// Compare fixed-length digests so bootstrap token checks do not leak prefix
	// matches or token length through timing.
	return subtle.ConstantTimeCompare([]byte(expectedHash), []byte(providedHash)) == 1
}

func (s *Service) createFirstOwner(ctx context.Context, username string, password string) (SessionResult, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return SessionResult{}, err
	}

	row, err := s.repo.CreateFirstOwner(ctx, username, string(hash), s.now())
	if err != nil {
		return SessionResult{}, err
	}

	// Imported pre-auth entries belong to the first real account so upgrades
	// from the single-user prototype do not orphan diary data.
	if err := s.repo.ClaimLegacyEntries(ctx, row.ID); err != nil {
		return SessionResult{}, err
	}

	return s.startSession(ctx, row)
}

func (s *Service) Login(ctx context.Context, input Credentials) (SessionResult, error) {
	if inProgress, err := s.SetupRestoreInProgress(ctx); err != nil {
		return SessionResult{}, err
	} else if inProgress {
		return SessionResult{}, ErrRestoreInProgress
	}

	username, password, err := normalizeCredentials(input)
	if err != nil {
		return SessionResult{}, ErrInvalidCredentials
	}

	row, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return SessionResult{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(row.PasswordHash), []byte(password)); err != nil {
		return SessionResult{}, ErrInvalidCredentials
	}

	return s.startSession(ctx, row)
}

func (s *Service) DeleteCurrentAccount(ctx context.Context, userID int64, input DeleteAccountInput) (AccountDeletionResult, error) {
	if inProgress, err := s.SetupRestoreInProgress(ctx); err != nil {
		return AccountDeletionResult{}, err
	} else if inProgress {
		return AccountDeletionResult{}, ErrRestoreInProgress
	}

	username, password, err := normalizeCredentials(Credentials{
		Username: input.Username,
		Password: input.Password,
	})
	if err != nil {
		return AccountDeletionResult{}, err
	}

	row, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return AccountDeletionResult{}, err
	}
	if username != row.Username {
		return AccountDeletionResult{}, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(row.PasswordHash), []byte(password)); err != nil {
		return AccountDeletionResult{}, ErrInvalidCredentials
	}

	result, err := s.repo.DeleteAccount(ctx, userID)
	if err != nil {
		return AccountDeletionResult{}, err
	}
	if s.accountFiles != nil {
		for _, path := range result.ImageFilePaths {
			if err := s.accountFiles.Delete(ctx, path); err != nil {
				slog.ErrorContext(ctx, "account image file deletion failed", slog.Int64("user_id", userID), slog.String("file_path", path), slog.String("error", err.Error()))
			}
		}
	}
	return result, nil
}

func (s *Service) UserByToken(ctx context.Context, token string) (User, error) {
	if inProgress, err := s.SetupRestoreInProgress(ctx); err != nil {
		return User{}, err
	} else if inProgress {
		return User{}, ErrRestoreInProgress
	}

	if strings.TrimSpace(token) == "" {
		return User{}, ErrUnauthorized
	}

	session, err := s.repo.GetSessionByTokenHash(ctx, hashToken(token), s.now())
	if err != nil {
		return User{}, err
	}

	return responseUser(session.User), nil
}

func (s *Service) UserWithCSRFByToken(ctx context.Context, token string) (User, error) {
	if inProgress, err := s.SetupRestoreInProgress(ctx); err != nil {
		return User{}, err
	} else if inProgress {
		return User{}, ErrRestoreInProgress
	}

	if strings.TrimSpace(token) == "" {
		return User{}, ErrUnauthorized
	}

	session, err := s.repo.GetSessionByTokenHash(ctx, hashToken(token), s.now())
	if err != nil {
		return User{}, err
	}

	user := responseUser(session.User)
	csrfToken, err := randomToken()
	if err != nil {
		return User{}, err
	}
	user.CSRFToken = csrfToken

	// Store only the token hash. The raw CSRF token exists in the response body
	// and the frontend keeps it in memory, not in a readable cookie.
	if err := s.repo.UpdateSessionCSRF(ctx, hashToken(token), hashToken(csrfToken)); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Service) ValidateCSRF(ctx context.Context, sessionToken string, csrfToken string) bool {
	if strings.TrimSpace(sessionToken) == "" || strings.TrimSpace(csrfToken) == "" {
		return false
	}
	session, err := s.repo.GetSessionByTokenHash(ctx, hashToken(sessionToken), s.now())
	if err != nil || strings.TrimSpace(session.CSRFHash) == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(session.CSRFHash), []byte(hashToken(csrfToken))) == 1
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return s.repo.DeleteSession(ctx, hashToken(token))
}

func (s *Service) startSession(ctx context.Context, row UserRow) (SessionResult, error) {
	if err := s.repo.DeleteExpiredSessions(ctx, s.now()); err != nil {
		return SessionResult{}, err
	}

	token, err := randomToken()
	if err != nil {
		return SessionResult{}, err
	}
	csrfToken, err := randomToken()
	if err != nil {
		return SessionResult{}, err
	}

	expiresAt := s.now().Add(SessionTTL)
	// Persist token hashes only; the cookie carries the raw session token once.
	if err := s.repo.CreateSession(ctx, row.ID, hashToken(token), hashToken(csrfToken), expiresAt, s.now()); err != nil {
		return SessionResult{}, err
	}

	return SessionResult{
		User:      userWithCSRF(responseUser(row), csrfToken),
		Token:     token,
		CSRFToken: csrfToken,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}, nil
}

func normalizeCredentials(input Credentials) (string, string, error) {
	username := strings.ToLower(strings.TrimSpace(input.Username))
	password := strings.TrimSpace(input.Password)

	validators := []func() error{
		func() error {
			if !usernamePattern.MatchString(username) {
				return ErrInvalidInput
			}
			return nil
		},
		func() error {
			if len(password) < 8 || len(password) > 200 {
				return ErrInvalidInput
			}
			return nil
		},
	}

	for _, validator := range validators {
		if err := validator(); err != nil {
			return "", "", err
		}
	}

	return username, password, nil
}

func randomToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func responseUser(row UserRow) User {
	role := row.Role
	if strings.TrimSpace(role) == "" {
		role = RoleUser
	}
	return User{ID: row.ID, Username: row.Username, Role: role}
}

func userWithCSRF(user User, token string) User {
	user.CSRFToken = token
	return user
}

var errorSpecs = []struct {
	target error
	status int
	kind   string
}{
	{ErrInvalidInput, 400, "auth.invalid_input"},
	{ErrInvalidBackup, 400, "setup.invalid_backup"},
	{ErrRestoreConfirmationMissing, 400, "setup.invalid_input"},
	{ErrInvalidSetupToken, 403, "setup.invalid_token"},
	{ErrOwnerAccountRequired, 409, "auth.owner_account_required"},
	{ErrRestoreInProgress, 503, "setup.restore_in_progress"},
	{ErrRestoreCountMismatch, 500, "setup.restore_failed"},
	{ErrRestoreFailed, 500, "setup.restore_failed"},
	{ErrSetupLocked, 409, "setup.already_initialized"},
	{ErrInvalidCredentials, 401, "auth.invalid_credentials"},
	{ErrUnauthorized, 401, "auth.unauthorized"},
	{ErrSignupClosed, 403, "auth.signup_closed"},
	{ErrUsernameExists, 409, "auth.username_exists"},
}

func StatusFor(err error) int {
	for _, item := range errorSpecs {
		if errors.Is(err, item.target) {
			return item.status
		}
	}

	return 500
}

func KindFor(err error) string {
	for _, item := range errorSpecs {
		if errors.Is(err, item.target) {
			return item.kind
		}
	}

	return "server.internal"
}

func signupClosed(userCount int, allowAdditionalSignups bool) bool {
	return userCount > 0 && !allowAdditionalSignups
}

func signupMode(userCount int, _ bool, allowAdditionalSignups bool) string {
	if userCount > 0 && allowAdditionalSignups {
		return "open"
	}
	return "closed"
}

func pgRestoreRunner(databaseURL string, path string) func(context.Context, string) error {
	binary := strings.TrimSpace(path)
	if binary == "" {
		binary = "pg_restore"
	}
	database := strings.TrimSpace(databaseURL)
	return func(ctx context.Context, dumpPath string) error {
		if database == "" {
			return ErrRestoreFailed
		}

		cmd := exec.CommandContext(
			ctx,
			binary,
			"--data-only",
			"--no-owner",
			"--disable-triggers",
			"--dbname",
			database,
			dumpPath,
		)
		if err := cmd.Run(); err != nil {
			return ErrRestoreFailed
		}
		return nil
	}
}
