package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	setupLockedKey            = "setup_locked"
	setupRestoreInProgressKey = "setup_restore_in_progress"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(database *sql.DB) *Repository {
	return &Repository{db: database}
}

func (r *Repository) CountUsers(ctx context.Context) (int, error) {
	count := 0
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (r *Repository) CountEntries(ctx context.Context) (int, error) {
	count := 0
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entries`).Scan(&count)
	return count, err
}

func (r *Repository) CountImages(ctx context.Context) (int, error) {
	count := 0
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM images`).Scan(&count)
	return count, err
}

func (r *Repository) SetupLocked(ctx context.Context) (bool, error) {
	return r.settingBool(ctx, setupLockedKey)
}

func (r *Repository) SetupRestoreInProgress(ctx context.Context) (bool, error) {
	return r.settingBool(ctx, setupRestoreInProgressKey)
}

func (r *Repository) BeginSetupRestore(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `LOCK TABLE users IN EXCLUSIVE MODE`); err != nil {
		return err
	}

	setupLocked, err := setupLockedInTx(ctx, tx)
	if err != nil {
		return err
	}
	if setupLocked {
		return ErrSetupLocked
	}
	restoreInProgress, err := setupRestoreInProgressInTx(ctx, tx)
	if err != nil {
		return err
	}
	if restoreInProgress {
		return ErrRestoreInProgress
	}

	count := 0
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return ErrSetupLocked
	}

	if err := upsertSettingInTx(ctx, tx, setupRestoreInProgressKey, "true"); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ClearSetupRestoreInProgress(ctx context.Context) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO settings (key, value) VALUES ($1, 'false')
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		setupRestoreInProgressKey,
	)
	return err
}

func (r *Repository) FinishSetupRestore(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := upsertSettingInTx(ctx, tx, setupLockedKey, "true"); err != nil {
		return err
	}
	if err := upsertSettingInTx(ctx, tx, setupRestoreInProgressKey, "false"); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) CreateUser(ctx context.Context, username string, passwordHash string, now time.Time) (UserRow, error) {
	id := int64(0)
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO users (username, password_hash, role, created_at) VALUES ($1, $2, 'user', $3) RETURNING id`,
		username,
		passwordHash,
		now.Format(time.RFC3339),
	).Scan(&id)
	if isUniqueUsername(err) {
		return UserRow{}, ErrUsernameExists
	}
	if err != nil {
		return UserRow{}, err
	}

	return r.GetUserByID(ctx, id)
}

func (r *Repository) CreateFirstOwner(ctx context.Context, username string, passwordHash string, now time.Time) (UserRow, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return UserRow{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `LOCK TABLE users IN EXCLUSIVE MODE`); err != nil {
		return UserRow{}, err
	}

	setupLocked, err := setupLockedInTx(ctx, tx)
	if err != nil {
		return UserRow{}, err
	}
	if setupLocked {
		return UserRow{}, ErrSetupLocked
	}
	restoreInProgress, err := setupRestoreInProgressInTx(ctx, tx)
	if err != nil {
		return UserRow{}, err
	}
	if restoreInProgress {
		return UserRow{}, ErrRestoreInProgress
	}

	count := 0
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return UserRow{}, err
	}
	if count > 0 {
		return UserRow{}, ErrSetupLocked
	}

	row := UserRow{}
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO users (username, password_hash, role, created_at)
		 VALUES ($1, $2, 'owner', $3)
		 RETURNING id, username, password_hash, role, created_at`,
		username,
		passwordHash,
		now.Format(time.RFC3339),
	).Scan(&row.ID, &row.Username, &row.PasswordHash, &row.Role, &row.CreatedAt)
	if isUniqueUsername(err) {
		return UserRow{}, ErrUsernameExists
	}
	if err != nil {
		return UserRow{}, err
	}

	if err := upsertSettingInTx(ctx, tx, setupLockedKey, "true"); err != nil {
		return UserRow{}, err
	}

	if err := tx.Commit(); err != nil {
		return UserRow{}, err
	}
	return row, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id int64) (UserRow, error) {
	return r.scanUser(ctx, `SELECT id, username, password_hash, role, created_at FROM users WHERE id = $1`, id)
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (UserRow, error) {
	return r.scanUser(ctx, `SELECT id, username, password_hash, role, created_at FROM users WHERE username = $1`, username)
}

func (r *Repository) CreateSession(ctx context.Context, userID int64, tokenHash string, csrfHash string, expiresAt time.Time, now time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO sessions (user_id, token_hash, csrf_hash, expires_at, created_at) VALUES ($1, $2, $3, $4, $5)`,
		userID,
		tokenHash,
		csrfHash,
		expiresAt.Format(time.RFC3339),
		now.Format(time.RFC3339),
	)
	return err
}

func (r *Repository) GetSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (SessionRow, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT users.id, users.username, users.password_hash, users.role, users.created_at, sessions.csrf_hash
		 FROM sessions
		 JOIN users ON users.id = sessions.user_id
		 WHERE sessions.token_hash = $1 AND sessions.expires_at > $2`,
		tokenHash,
		now.Format(time.RFC3339),
	)

	session, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionRow{}, ErrUnauthorized
	}
	return session, err
}

func (r *Repository) GetUserByTokenHash(ctx context.Context, tokenHash string, now time.Time) (UserRow, error) {
	session, err := r.GetSessionByTokenHash(ctx, tokenHash, now)
	return session.User, err
}

func (r *Repository) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *Repository) UpdateSessionCSRF(ctx context.Context, tokenHash string, csrfHash string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET csrf_hash = $1 WHERE token_hash = $2`, csrfHash, tokenHash)
	return err
}

func (r *Repository) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= $1`, now.Format(time.RFC3339))
	return err
}

func (r *Repository) UpdatePasswordAndDeleteSessions(ctx context.Context, userID int64, currentPasswordHash string, nextPasswordHash string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	storedHash := ""
	if err := tx.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&storedHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUnauthorized
		}
		return err
	}
	if storedHash != currentPasswordHash {
		return ErrInvalidCredentials
	}

	result, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, nextPasswordHash, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrUnauthorized
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ClaimLegacyEntries(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE entries SET user_id = $1 WHERE user_id IS NULL`, userID)
	return err
}

// DeleteAccount removes the user-owned database rows and returns file paths
// that need storage cleanup after commit. The users lock keeps owner deletion
// checks and final-user setup reopening consistent with concurrent requests.
func (r *Repository) DeleteAccount(ctx context.Context, userID int64) (AccountDeletionResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AccountDeletionResult{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `LOCK TABLE users IN EXCLUSIVE MODE`); err != nil {
		return AccountDeletionResult{}, err
	}

	row, err := scanUser(tx.QueryRowContext(ctx, `SELECT id, username, password_hash, role, created_at FROM users WHERE id = $1 FOR UPDATE`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return AccountDeletionResult{}, ErrUnauthorized
	}
	if err != nil {
		return AccountDeletionResult{}, err
	}

	userCount := 0
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		return AccountDeletionResult{}, err
	}
	if row.Role == RoleOwner && userCount > 1 {
		return AccountDeletionResult{}, ErrOwnerAccountRequired
	}

	// Cascading deletes remove image metadata, so capture storage paths first.
	paths, err := accountImageFilePaths(ctx, tx, userID)
	if err != nil {
		return AccountDeletionResult{}, err
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return AccountDeletionResult{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return AccountDeletionResult{}, err
	}
	if affected == 0 {
		return AccountDeletionResult{}, ErrUnauthorized
	}

	remaining := 0
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&remaining); err != nil {
		return AccountDeletionResult{}, err
	}
	if remaining == 0 {
		if err := upsertSettingInTx(ctx, tx, setupLockedKey, "false"); err != nil {
			return AccountDeletionResult{}, err
		}
		if err := upsertSettingInTx(ctx, tx, setupRestoreInProgressKey, "false"); err != nil {
			return AccountDeletionResult{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return AccountDeletionResult{}, err
	}
	return AccountDeletionResult{ImageFilePaths: paths, RemainingUsers: remaining}, nil
}

func (r *Repository) settingBool(ctx context.Context, key string) (bool, error) {
	value := ""
	err := r.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return settingBool(value), nil
}

func (r *Repository) scanUser(ctx context.Context, query string, args ...any) (UserRow, error) {
	user, err := scanUser(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return UserRow{}, ErrInvalidCredentials
	}
	return user, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

type SessionRow struct {
	User     UserRow
	CSRFHash string
}

func scanSession(scanner rowScanner) (SessionRow, error) {
	row := SessionRow{}
	err := scanner.Scan(&row.User.ID, &row.User.Username, &row.User.PasswordHash, &row.User.Role, &row.User.CreatedAt, &row.CSRFHash)
	return row, err
}

func scanUser(scanner rowScanner) (UserRow, error) {
	row := UserRow{}
	err := scanner.Scan(&row.ID, &row.Username, &row.PasswordHash, &row.Role, &row.CreatedAt)
	return row, err
}

func isUniqueUsername(err error) bool {
	pgErr := &pgconn.PgError{}
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type txQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type txExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type txQueryExecutor interface {
	txQueryer
	txExecutor
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func accountImageFilePaths(ctx context.Context, tx txQueryExecutor, userID int64) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT i.file_path
		FROM images i
		JOIN entries e ON e.id = i.entry_id
		WHERE e.user_id = $1
		ORDER BY i.id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	paths := []string{}
	for rows.Next() {
		path := ""
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

func setupLockedInTx(ctx context.Context, tx txQueryer) (bool, error) {
	return settingBoolInTx(ctx, tx, setupLockedKey)
}

func setupRestoreInProgressInTx(ctx context.Context, tx txQueryer) (bool, error) {
	return settingBoolInTx(ctx, tx, setupRestoreInProgressKey)
}

func settingBoolInTx(ctx context.Context, tx txQueryer, key string) (bool, error) {
	value := ""
	err := tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = $1 FOR UPDATE`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return settingBool(value), nil
}

func upsertSettingInTx(ctx context.Context, tx txExecutor, key string, value string) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key,
		value,
	)
	return err
}

func settingBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
