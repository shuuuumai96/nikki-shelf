package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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

func (r *Repository) CreateUser(ctx context.Context, username string, passwordHash string, now time.Time) (UserRow, error) {
	id := int64(0)
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO users (username, password_hash, created_at) VALUES ($1, $2, $3) RETURNING id`,
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

func (r *Repository) CreateFirstUser(ctx context.Context, username string, passwordHash string, now time.Time) (UserRow, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return UserRow{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `LOCK TABLE users IN EXCLUSIVE MODE`); err != nil {
		return UserRow{}, err
	}

	count := 0
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return UserRow{}, err
	}
	if count > 0 {
		return UserRow{}, ErrSignupClosed
	}

	row := UserRow{}
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO users (username, password_hash, created_at) VALUES ($1, $2, $3) RETURNING id, username, password_hash, created_at`,
		username,
		passwordHash,
		now.Format(time.RFC3339),
	).Scan(&row.ID, &row.Username, &row.PasswordHash, &row.CreatedAt)
	if isUniqueUsername(err) {
		return UserRow{}, ErrUsernameExists
	}
	if err != nil {
		return UserRow{}, err
	}

	if err := tx.Commit(); err != nil {
		return UserRow{}, err
	}
	return row, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id int64) (UserRow, error) {
	return r.scanUser(ctx, `SELECT id, username, password_hash, created_at FROM users WHERE id = $1`, id)
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (UserRow, error) {
	return r.scanUser(ctx, `SELECT id, username, password_hash, created_at FROM users WHERE username = $1`, username)
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
		`SELECT users.id, users.username, users.password_hash, users.created_at, sessions.csrf_hash
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

func (r *Repository) ClaimLegacyEntries(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE entries SET user_id = $1 WHERE user_id IS NULL`, userID)
	return err
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
	err := scanner.Scan(&row.User.ID, &row.User.Username, &row.User.PasswordHash, &row.User.CreatedAt, &row.CSRFHash)
	return row, err
}

func scanUser(scanner rowScanner) (UserRow, error) {
	row := UserRow{}
	err := scanner.Scan(&row.ID, &row.Username, &row.PasswordHash, &row.CreatedAt)
	return row, err
}

func isUniqueUsername(err error) bool {
	pgErr := &pgconn.PgError{}
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
