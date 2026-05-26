package images

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(database *sql.DB) *Repository {
	return &Repository{db: database}
}

func (r *Repository) Create(ctx context.Context, entryID int64, file StoredFile, now time.Time) (Row, error) {
	id := int64(0)
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO images (entry_id, file_path, public_url, file_name, size_bytes, mime_type, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		entryID,
		file.FilePath,
		file.PublicURL,
		file.FileName,
		file.Size,
		file.MimeType,
		now.Format(time.RFC3339),
	).Scan(&id)
	if err != nil {
		return Row{}, err
	}

	return r.GetByID(ctx, id)
}

func (r *Repository) CreateForUser(ctx context.Context, userID int64, entryID int64, files []StoredFile, now time.Time, quota QuotaConfig) ([]Row, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := lockEntryForUser(ctx, tx, userID, entryID); err != nil {
		return nil, err
	}

	// Locking the entry serializes concurrent uploads for the same diary entry,
	// so the per-entry image cap and quota checks stay consistent.
	entryCount, err := countImagesByEntryID(ctx, tx, entryID)
	if err != nil {
		return nil, err
	}
	if entryCount+len(files) > MaxImagesPerEntry {
		return nil, ErrTooManyImages
	}

	userUsage, err := imageUsageForUser(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	incoming := usageForStoredFiles(files)
	if quota.UserBytes > 0 && userUsage.Bytes+incoming.Bytes > quota.UserBytes {
		return nil, ErrImageQuotaExceeded
	}
	if quota.UserCount > 0 && userUsage.Count+incoming.Count > quota.UserCount {
		return nil, ErrImageQuotaExceeded
	}

	if quota.TotalBytes > 0 {
		totalUsage, err := totalImageUsage(ctx, tx)
		if err != nil {
			return nil, err
		}
		if totalUsage.Bytes+incoming.Bytes > quota.TotalBytes {
			return nil, ErrImageQuotaExceeded
		}
	}

	rows := make([]Row, 0, len(files))
	for _, file := range files {
		row, err := insertImage(ctx, tx, entryID, file, now)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return rows, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (Row, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, entry_id, file_path, public_url, file_name, size_bytes, mime_type, created_at FROM images WHERE id = $1`,
		id,
	)
	image, err := scanImage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Row{}, ErrImageNotFound
	}
	return image, err
}

func (r *Repository) GetOwnedByID(ctx context.Context, id int64, userID int64) (Row, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT images.id, images.entry_id, images.file_path, images.public_url, images.file_name, images.size_bytes, images.mime_type, images.created_at
		FROM images
		JOIN entries ON entries.id = images.entry_id
		WHERE images.id = $1 AND entries.user_id = $2`, id, userID)
	image, err := scanImage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Row{}, ErrImageNotFound
	}
	return image, err
}

func (r *Repository) GetOwnedByPublicURL(ctx context.Context, publicURL string, userID int64) (Row, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT images.id, images.entry_id, images.file_path, images.public_url, images.file_name, images.size_bytes, images.mime_type, images.created_at
		FROM images
		JOIN entries ON entries.id = images.entry_id
		WHERE images.public_url = $1 AND entries.user_id = $2`, publicURL, userID)
	image, err := scanImage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Row{}, ErrImageNotFound
	}
	return image, err
}

func (r *Repository) ListByEntryID(ctx context.Context, entryID int64) ([]Row, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, entry_id, file_path, public_url, file_name, size_bytes, mime_type, created_at FROM images WHERE entry_id = $1 ORDER BY id ASC`,
		entryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := []Row{}
	for rows.Next() {
		image, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}

	return images, rows.Err()
}

func (r *Repository) ListAll(ctx context.Context) ([]Row, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, entry_id, file_path, public_url, file_name, size_bytes, mime_type, created_at FROM images ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := []Row{}
	for rows.Next() {
		image, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}

	return images, rows.Err()
}

func (r *Repository) CountByEntryID(ctx context.Context, entryID int64) (int, error) {
	count := 0
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM images WHERE entry_id = $1`, entryID).Scan(&count)
	return count, err
}

func (r *Repository) UsageForUser(ctx context.Context, userID int64) (Usage, error) {
	return imageUsageForUser(ctx, r.db, userID)
}

func (r *Repository) TotalUsage(ctx context.Context) (Usage, error) {
	return totalImageUsage(ctx, r.db)
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM images WHERE id = $1`, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrImageNotFound
	}

	return nil
}

func (r *Repository) DeleteByEntryID(ctx context.Context, entryID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM images WHERE entry_id = $1`, entryID)
	return err
}

func (r *Repository) ExistsEntry(ctx context.Context, entryID int64) (bool, error) {
	exists := 0
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM entries WHERE id = $1`, entryID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return exists == 1, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func lockEntryForUser(ctx context.Context, tx *sql.Tx, userID int64, entryID int64) error {
	id := int64(0)
	// The parent entry is the quota lock target. It also verifies ownership
	// before any image rows are inserted.
	err := tx.QueryRowContext(ctx, `SELECT id FROM entries WHERE id = $1 AND user_id = $2 FOR UPDATE`, entryID, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrImageNotFound
	}
	return err
}

func countImagesByEntryID(ctx context.Context, q queryer, entryID int64) (int, error) {
	count := 0
	err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM images WHERE entry_id = $1`, entryID).Scan(&count)
	return count, err
}

func imageUsageForUser(ctx context.Context, q queryer, userID int64) (Usage, error) {
	usage := Usage{}
	err := q.QueryRowContext(ctx, `
		SELECT COUNT(i.id), COALESCE(SUM(i.size_bytes), 0)
		FROM images i
		JOIN entries e ON e.id = i.entry_id
		WHERE e.user_id = $1`, userID).Scan(&usage.Count, &usage.Bytes)
	return usage, err
}

func totalImageUsage(ctx context.Context, q queryer) (Usage, error) {
	usage := Usage{}
	err := q.QueryRowContext(ctx, `SELECT COUNT(id), COALESCE(SUM(size_bytes), 0) FROM images`).Scan(&usage.Count, &usage.Bytes)
	return usage, err
}

func insertImage(ctx context.Context, tx *sql.Tx, entryID int64, file StoredFile, now time.Time) (Row, error) {
	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO images (entry_id, file_path, public_url, file_name, size_bytes, mime_type, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, entry_id, file_path, public_url, file_name, size_bytes, mime_type, created_at`,
		entryID,
		file.FilePath,
		file.PublicURL,
		file.FileName,
		file.Size,
		file.MimeType,
		now.Format(time.RFC3339),
	)
	return scanImage(row)
}

func usageForStoredFiles(files []StoredFile) Usage {
	usage := Usage{Count: len(files)}
	for _, file := range files {
		usage.Bytes += file.Size
	}
	return usage
}

func scanImage(scanner rowScanner) (Row, error) {
	row := Row{}
	err := scanner.Scan(
		&row.ID,
		&row.EntryID,
		&row.FilePath,
		&row.PublicURL,
		&row.FileName,
		&row.Size,
		&row.MimeType,
		&row.CreatedAt,
	)
	return row, err
}
