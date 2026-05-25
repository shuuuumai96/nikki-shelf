package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const SessionTTL = 30 * 24 * time.Hour

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,39}$`)

type Service struct {
	repo                    serviceRepository
	allowAdditionalSignups  bool
	allowFirstUserSetup     bool
	firstUserBootstrapToken string
	now                     func() time.Time
}

type serviceRepository interface {
	CountUsers(ctx context.Context) (int, error)
	CreateUser(ctx context.Context, username string, passwordHash string, now time.Time) (UserRow, error)
	CreateFirstUser(ctx context.Context, username string, passwordHash string, now time.Time) (UserRow, error)
	GetUserByUsername(ctx context.Context, username string) (UserRow, error)
	CreateSession(ctx context.Context, userID int64, tokenHash string, csrfHash string, expiresAt time.Time, now time.Time) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (SessionRow, error)
	UpdateSessionCSRF(ctx context.Context, tokenHash string, csrfHash string) error
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) error
	ClaimLegacyEntries(ctx context.Context, userID int64) error
}

type ServiceConfig struct {
	AllowAdditionalSignups  bool
	AllowFirstUserSetup     bool
	FirstUserBootstrapToken string
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
		now:                     time.Now,
	}
}

func (s *Service) Signup(ctx context.Context, input Credentials, bootstrapToken string) (SessionResult, error) {
	username, password, err := normalizeCredentials(input)
	if err != nil {
		return SessionResult{}, err
	}

	userCount, err := s.repo.CountUsers(ctx)
	if err != nil {
		return SessionResult{}, err
	}
	firstUserSignup := userCount == 0
	if firstUserSignup && !s.validFirstUserSignup(bootstrapToken) {
		return SessionResult{}, ErrSignupClosed
	}
	if signupClosed(userCount, s.allowAdditionalSignups) {
		return SessionResult{}, ErrSignupClosed
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return SessionResult{}, err
	}

	var row UserRow
	if firstUserSignup {
		row, err = s.repo.CreateFirstUser(ctx, username, string(hash), s.now())
	} else {
		row, err = s.repo.CreateUser(ctx, username, string(hash), s.now())
	}
	if err != nil {
		return SessionResult{}, err
	}

	if userCount == 0 {
		if err := s.repo.ClaimLegacyEntries(ctx, row.ID); err != nil {
			return SessionResult{}, err
		}
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

func (s *Service) validFirstUserSignup(bootstrapToken string) bool {
	return s.validFirstUserBootstrapToken(bootstrapToken) || s.allowFirstUserSetup
}

func (s *Service) validFirstUserBootstrapToken(token string) bool {
	expected := strings.TrimSpace(s.firstUserBootstrapToken)
	provided := strings.TrimSpace(token)
	if expected == "" || provided == "" {
		return false
	}
	expectedHash := hashToken(expected)
	providedHash := hashToken(provided)
	return subtle.ConstantTimeCompare([]byte(expectedHash), []byte(providedHash)) == 1
}

func (s *Service) Login(ctx context.Context, input Credentials) (SessionResult, error) {
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

func (s *Service) UserByToken(ctx context.Context, token string) (User, error) {
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
	return User{ID: row.ID, Username: row.Username}
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

func signupMode(userCount int, allowFirstUserSetup bool, allowAdditionalSignups bool) string {
	if userCount == 0 && allowFirstUserSetup {
		return "setup"
	}
	if userCount > 0 && allowAdditionalSignups {
		return "open"
	}
	return "closed"
}
