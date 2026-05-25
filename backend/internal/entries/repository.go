package entries

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(database *sql.DB) *Repository {
	return &Repository{db: database}
}

func (r *Repository) Create(ctx context.Context, input CreateInput, now time.Time) (EntryRow, error) {
	tagsJSON, err := encodeTags(input.Tags)
	if err != nil {
		return EntryRow{}, err
	}

	timestamp := now.Format(time.RFC3339)
	id := int64(0)
	err = r.db.QueryRowContext(
		ctx,
		`INSERT INTO entries (user_id, entry_date, title, body, mood, tags_json, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id`,
		input.UserID,
		input.EntryDate,
		input.Title,
		input.Body,
		input.Mood,
		tagsJSON,
		timestamp,
		timestamp,
	).Scan(&id)
	if isUniqueEntryDate(err) {
		return EntryRow{}, ErrDateExists
	}
	if err != nil {
		return EntryRow{}, err
	}

	return r.GetByID(ctx, input.UserID, id)
}

func (r *Repository) Update(ctx context.Context, userID int64, id int64, input UpdateInput, now time.Time) (EntryRow, error) {
	tagsJSON, err := encodeTags(input.Tags)
	if err != nil {
		return EntryRow{}, err
	}

	result, err := r.db.ExecContext(
		ctx,
		`UPDATE entries
		 SET entry_date = $1, title = $2, body = $3, mood = $4, tags_json = $5, version = version + 1, updated_at = $6
		 WHERE id = $7 AND user_id = $8 AND version = $9`,
		input.EntryDate,
		input.Title,
		input.Body,
		input.Mood,
		tagsJSON,
		now.Format(time.RFC3339),
		id,
		userID,
		input.Version,
	)
	if isUniqueEntryDate(err) {
		return EntryRow{}, ErrDateExists
	}
	if err != nil {
		return EntryRow{}, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return EntryRow{}, err
	}
	if affected == 0 {
		// The version is part of the WHERE clause. Distinguish a stale write from
		// a missing row so the client can show conflict recovery instead of 404.
		exists, existsErr := r.ExistsForUser(ctx, userID, id)
		if existsErr != nil {
			return EntryRow{}, existsErr
		}
		if exists {
			return EntryRow{}, ErrStaleVersion
		}
		return EntryRow{}, ErrNotFound
	}

	return r.GetByID(ctx, userID, id)
}

func (r *Repository) GetByID(ctx context.Context, userID int64, id int64) (EntryRow, error) {
	return r.scanOne(ctx, `SELECT id, user_id, entry_date, title, body, mood, tags_json, version, created_at, updated_at FROM entries WHERE id = $1 AND user_id = $2`, id, userID)
}

func (r *Repository) GetByDate(ctx context.Context, userID int64, date string) (EntryRow, error) {
	return r.scanOne(ctx, `SELECT id, user_id, entry_date, title, body, mood, tags_json, version, created_at, updated_at FROM entries WHERE user_id = $1 AND entry_date = $2`, userID, date)
}

func (r *Repository) List(ctx context.Context, userID int64, filter EntryFilter) ([]EntryRow, error) {
	clauses, args := buildFilterClauses(userID, filter)

	query := `SELECT id, user_id, entry_date, title, body, mood, tags_json, version, created_at, updated_at FROM entries`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY entry_date DESC, id DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []EntryRow{}
	for rows.Next() {
		row, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, row)
	}

	return entries, rows.Err()
}

func (r *Repository) ListPage(ctx context.Context, userID int64, request EntryPageRequest) (EntryPage, error) {
	cursor, err := decodeCursor(request.Cursor)
	if err != nil {
		return EntryPage{}, err
	}

	clauses, args := buildFilterClauses(userID, request.Filter)
	if cursor.EntryDate != "" {
		cursorDate := appendArg(&args, cursor.EntryDate)
		cursorID := appendArg(&args, cursor.ID)
		// Keyset pagination must mirror ORDER BY entry_date DESC, id DESC. The id
		// tiebreaker keeps pagination stable for multiple entries on a date.
		clauses = append(clauses, "(entry_date < "+cursorDate+" OR (entry_date = "+cursorDate+" AND id < "+cursorID+"))")
	}

	query := `SELECT id, user_id, entry_date, title, body, mood, tags_json, version, created_at, updated_at FROM entries`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY entry_date DESC, id DESC"
	query += " LIMIT " + appendArg(&args, request.PerPage+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return EntryPage{}, err
	}
	defer rows.Close()

	items := []EntryRow{}
	for rows.Next() {
		row, err := scanEntry(rows)
		if err != nil {
			return EntryPage{}, err
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return EntryPage{}, err
	}

	page := EntryPage{Rows: items}
	if len(items) > request.PerPage {
		page.HasMore = true
		page.Rows = items[:request.PerPage]
		nextCursor, err := encodeCursor(page.Rows[len(page.Rows)-1])
		if err != nil {
			return EntryPage{}, err
		}
		page.NextCursor = nextCursor
	}
	return page, nil
}

func (r *Repository) Count(ctx context.Context, userID int64, filter EntryFilter) (int, error) {
	clauses, args := buildFilterClauses(userID, filter)
	query := `SELECT COUNT(*) FROM entries`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	count := 0
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *Repository) Tags(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT tag
		FROM entries
		CROSS JOIN LATERAL jsonb_array_elements_text(tags_json) AS entry_tags(tag)
		WHERE user_id = $1
		ORDER BY tag ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := []string{}
	for rows.Next() {
		tag := ""
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (r *Repository) MoodCounts(ctx context.Context, userID int64) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT mood, COUNT(*) FROM entries WHERE user_id = $1 GROUP BY mood`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		mood := ""
		count := 0
		if err := rows.Scan(&mood, &count); err != nil {
			return nil, err
		}
		counts[mood] = count
	}
	return counts, rows.Err()
}

func (r *Repository) LastEntryDate(ctx context.Context, userID int64) (string, error) {
	value := ""
	err := r.db.QueryRowContext(ctx, `SELECT entry_date FROM entries WHERE user_id = $1 ORDER BY entry_date DESC, id DESC LIMIT 1`, userID).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func (r *Repository) EntryDatesDesc(ctx context.Context, userID int64, toDate string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT entry_date FROM entries WHERE user_id = $1 AND entry_date <= $2 ORDER BY entry_date DESC`, userID, toDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dates := []string{}
	for rows.Next() {
		date := ""
		if err := rows.Scan(&date); err != nil {
			return nil, err
		}
		dates = append(dates, date)
	}
	return dates, rows.Err()
}

func (r *Repository) Delete(ctx context.Context, userID int64, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM entries WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *Repository) ExistsForUser(ctx context.Context, userID int64, id int64) (bool, error) {
	exists := 0
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM entries WHERE id = $1 AND user_id = $2`, id, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return exists == 1, err
}

func (r *Repository) scanOne(ctx context.Context, query string, args ...any) (EntryRow, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	entry, err := scanEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return EntryRow{}, ErrNotFound
	}
	return entry, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEntry(scanner rowScanner) (EntryRow, error) {
	row := EntryRow{}
	err := scanner.Scan(
		&row.ID,
		&row.UserID,
		&row.EntryDate,
		&row.Title,
		&row.Body,
		&row.Mood,
		&row.TagsJSON,
		&row.Version,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	return row, err
}

func encodeTags(tags []string) (string, error) {
	encoded, err := json.Marshal(tags)
	return string(encoded), err
}

func isUniqueEntryDate(err error) bool {
	pgErr := &pgconn.PgError{}
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
