package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/db"
	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
)

const devSeedImagePrefix = "nikki-dev-seed-"

type seedDevOptions struct {
	reset       bool
	entries     int
	months      int
	user        string
	password    string
	withImages  bool
	confirmed   bool
	allowDevEnv bool
}

type seedDevSummary struct {
	User            string `json:"user"`
	EntriesInserted int    `json:"entriesInserted"`
	DateFrom        string `json:"dateFrom"`
	DateTo          string `json:"dateTo"`
	ImagesInserted  int    `json:"imagesInserted"`
	Reset           bool   `json:"reset"`
}

type seedEntry struct {
	date  time.Time
	title string
	body  string
	mood  string
	tags  []string
}

func SeedDevData(ctx context.Context, args []string) error {
	options, err := parseSeedDevOptions(args)
	if err != nil {
		return err
	}
	if err := validateSeedDevOptions(options); err != nil {
		return err
	}

	cfg := LoadConfig()
	logger := logx.New(logx.Config{Level: cfg.LogLevel, Format: cfg.LogFormat})
	slog.SetDefault(logger)

	if err := guardDevSeedEnvironment(cfg, options.allowDevEnv); err != nil {
		return err
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		return err
	}

	summary, err := seedDevData(ctx, database, cfg, options)
	if err != nil {
		return err
	}

	fmt.Printf("Development seed data created:\n")
	fmt.Printf("  user: %s\n", summary.User)
	fmt.Printf("  entries inserted: %d\n", summary.EntriesInserted)
	fmt.Printf("  date range: %s to %s\n", summary.DateFrom, summary.DateTo)
	fmt.Printf("  images inserted: %d\n", summary.ImagesInserted)
	fmt.Printf("  reset: %t\n", summary.Reset)
	return nil
}

func parseSeedDevOptions(args []string) (seedDevOptions, error) {
	options := seedDevOptions{
		entries:  420,
		months:   18,
		user:     "dev",
		password: "devpassword123",
	}

	flags := flag.NewFlagSet("seed-dev-data", flag.ContinueOnError)
	flags.BoolVar(&options.reset, "reset", false, "delete and recreate development diary data for the selected user")
	flags.IntVar(&options.entries, "entries", options.entries, "number of diary entries to generate")
	flags.IntVar(&options.months, "months", options.months, "number of months to spread diary entries across")
	flags.StringVar(&options.user, "user", options.user, "development username")
	flags.StringVar(&options.password, "password", options.password, "development password")
	flags.BoolVar(&options.withImages, "with-images", false, "create tiny valid image files and matching image rows")
	flags.BoolVar(&options.confirmed, "i-understand-this-deletes-dev-data", false, "required destructive confirmation")
	flags.BoolVar(&options.allowDevEnv, "allow-dev-environment-override", false, "allow running despite production-looking environment")
	if err := flags.Parse(args); err != nil {
		return seedDevOptions{}, err
	}
	if flags.NArg() > 0 {
		return seedDevOptions{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	return options, nil
}

func validateSeedDevOptions(options seedDevOptions) error {
	if !options.confirmed {
		return errors.New("refusing to run without --i-understand-this-deletes-dev-data")
	}
	if options.entries < 1 {
		return errors.New("--entries must be at least 1")
	}
	if options.months < 1 {
		return errors.New("--months must be at least 1")
	}
	if strings.TrimSpace(options.user) == "" {
		return errors.New("--user is required")
	}
	if len(strings.TrimSpace(options.password)) < 8 {
		return errors.New("--password must be at least 8 characters")
	}
	return nil
}

func guardDevSeedEnvironment(cfg Config, allowOverride bool) error {
	if allowOverride || strings.EqualFold(strings.TrimSpace(os.Getenv("NIKKI_ALLOW_DEV_SEED")), "true") {
		return nil
	}
	if cfg.CookieSecure {
		return errors.New("refusing to seed because NIKKI_COOKIE_SECURE=true; set NIKKI_ALLOW_DEV_SEED=true only for local development")
	}
	for _, origin := range cfg.CORSAllowedOrigins {
		if isPublicHTTPSOrigin(origin) {
			return fmt.Errorf("refusing to seed because NIKKI_CORS_ALLOWED_ORIGINS contains public https origin %q; set NIKKI_ALLOW_DEV_SEED=true only for local development", origin)
		}
	}
	return nil
}

func isPublicHTTPSOrigin(origin string) bool {
	normalized := strings.ToLower(strings.TrimSpace(origin))
	if !strings.HasPrefix(normalized, "https://") {
		return false
	}
	return !strings.Contains(normalized, "localhost") && !strings.Contains(normalized, "127.0.0.1") && !strings.Contains(normalized, "[::1]")
}

func seedDevData(ctx context.Context, database *sql.DB, cfg Config, options seedDevOptions) (seedDevSummary, error) {
	now := time.Now().UTC().Truncate(time.Second)
	entries := buildSeedEntries(options.entries, options.months, now)
	if len(entries) == 0 {
		return seedDevSummary{}, errors.New("no seed entries generated")
	}

	if options.reset {
		if err := deleteSeedImageFiles(cfg.UploadDir); err != nil {
			return seedDevSummary{}, err
		}
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return seedDevSummary{}, err
	}
	defer tx.Rollback()

	userID, err := ensureSeedUser(ctx, tx, options.user, options.password, now)
	if err != nil {
		return seedDevSummary{}, err
	}
	if options.reset {
		if err := resetSeedUserData(ctx, tx, userID); err != nil {
			return seedDevSummary{}, err
		}
	}

	imagesCreated := []string{}
	inserted := 0
	imagesInserted := 0
	for index, entry := range entries {
		entryID, ok, err := insertSeedEntry(ctx, tx, userID, entry, now)
		if err != nil {
			cleanupFiles(imagesCreated)
			return seedDevSummary{}, err
		}
		if !ok {
			continue
		}
		inserted += 1

		if options.withImages && index%11 == 0 {
			path, publicURL, size, err := writeSeedImageFile(cfg.UploadDir, cfg.PublicUploadBase, entryID, index)
			if err != nil {
				cleanupFiles(imagesCreated)
				return seedDevSummary{}, err
			}
			imagesCreated = append(imagesCreated, path)
			if err := insertSeedImage(ctx, tx, entryID, path, publicURL, size, now); err != nil {
				cleanupFiles(imagesCreated)
				return seedDevSummary{}, err
			}
			imagesInserted += 1
		}
	}

	if err := tx.Commit(); err != nil {
		cleanupFiles(imagesCreated)
		return seedDevSummary{}, err
	}

	return seedDevSummary{
		User:            options.user,
		EntriesInserted: inserted,
		DateFrom:        entries[len(entries)-1].date.Format(time.DateOnly),
		DateTo:          entries[0].date.Format(time.DateOnly),
		ImagesInserted:  imagesInserted,
		Reset:           options.reset,
	}, nil
}

func ensureSeedUser(ctx context.Context, tx *sql.Tx, username string, password string, now time.Time) (int64, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(password)), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	id := int64(0)
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO users (username, password_hash, created_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (username) DO UPDATE SET password_hash = EXCLUDED.password_hash
		 RETURNING id`,
		strings.ToLower(strings.TrimSpace(username)),
		string(hash),
		now.Format(time.RFC3339),
	).Scan(&id)
	return id, err
}

func resetSeedUserData(ctx context.Context, tx *sql.Tx, userID int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM images WHERE entry_id IN (SELECT id FROM entries WHERE user_id = $1)`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM entries WHERE user_id = $1`, userID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

func insertSeedEntry(ctx context.Context, tx *sql.Tx, userID int64, entry seedEntry, now time.Time) (int64, bool, error) {
	tagsJSON, err := json.Marshal(entry.tags)
	if err != nil {
		return 0, false, err
	}

	id := int64(0)
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO entries (user_id, entry_date, title, body, mood, tags_json, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		 ON CONFLICT (user_id, entry_date) DO NOTHING
		 RETURNING id`,
		userID,
		entry.date.Format(time.DateOnly),
		entry.title,
		entry.body,
		entry.mood,
		string(tagsJSON),
		now.Format(time.RFC3339),
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return id, true, err
}

func insertSeedImage(ctx context.Context, tx *sql.Tx, entryID int64, path string, publicURL string, size int64, now time.Time) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO images (entry_id, file_path, public_url, file_name, size_bytes, mime_type, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entryID,
		path,
		publicURL,
		filepath.Base(path),
		size,
		"image/png",
		now.Format(time.RFC3339),
	)
	return err
}

func buildSeedEntries(count int, months int, now time.Time) []seedEntry {
	latest := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)
	start := latest.AddDate(0, -months+1, 0)
	availableDays := int(latest.Sub(start).Hours()/24) + 1
	if count > availableDays {
		count = availableDays
	}

	entries := make([]seedEntry, 0, count)
	used := map[string]bool{}
	for i := 0; len(entries) < count && i < count*2; i++ {
		offset := 0
		if count > 1 {
			offset = int(math.Round(float64(i) * float64(availableDays-1) / float64(count-1)))
		}
		date := latest.AddDate(0, 0, -offset)
		key := date.Format(time.DateOnly)
		if used[key] {
			date = latest.AddDate(0, 0, -len(entries))
			key = date.Format(time.DateOnly)
		}
		if date.Before(start) || used[key] {
			continue
		}
		used[key] = true
		entries = append(entries, seedEntryFor(date, len(entries)))
	}
	return entries
}

func seedEntryFor(date time.Time, index int) seedEntry {
	moods := []string{"happy", "calm", "tired", "sad", "excited"}
	tagSets := [][]string{
		{"work", "coffee"},
		{"暮らし", "散歩"},
		{"idea", "reading"},
		{"家族", "週末"},
		{"health", "sleep"},
		{"料理", "買い物"},
		{"travel", "写真"},
		{"study", "notes"},
	}
	titles := []string{
		"Morning notes",
		"静かな一日",
		"Small wins",
		"",
		"After work",
		"週末の記録",
		"Long walk",
		"雨の日メモ",
	}
	bodyParts := []string{
		"朝は少し早く起きて、窓を開けたら空気がひんやりしていた。コーヒーを淹れてから今日やることを小さく整理した。",
		"仕事の合間に短い散歩をした。派手なことはなかったけれど、こういう普通の日をちゃんと覚えておきたい。",
		"夕方に考えていたことが少しまとまった。明日は続きを試してみる。焦らず、でも手は止めない。",
		"今日は短め。眠いので早く寝る。",
		"買い物帰りに見た夕焼けがきれいだった。写真を撮るほどではないけれど、心に残る色だった。",
		"Longer preview check: I wrote a few paragraphs about the day so the list can show a realistic two-line body preview. The details are ordinary, which is exactly what makes diary data useful for layout testing.",
	}

	body := bodyParts[index%len(bodyParts)]
	if index%9 == 0 {
		body = body + "\n\n" + bodyParts[(index+2)%len(bodyParts)] + "\n\n" + bodyParts[(index+4)%len(bodyParts)]
	}

	return seedEntry{
		date:  date,
		title: titles[index%len(titles)],
		body:  body,
		mood:  moods[index%len(moods)],
		tags:  tagSets[index%len(tagSets)],
	}
}

func writeSeedImageFile(uploadDir string, publicBase string, entryID int64, index int) (string, string, int64, error) {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", "", 0, err
	}
	name := fmt.Sprintf("%s%d-%03d.png", devSeedImagePrefix, entryID, index)
	path := filepath.Join(uploadDir, name)
	if err := os.WriteFile(path, tinySeedPNG(), 0644); err != nil {
		return "", "", 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", "", 0, err
	}
	return path, strings.TrimRight(publicBase, "/") + "/" + name, info.Size(), nil
}

func tinySeedPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0xf0,
		0x1f, 0x00, 0x05, 0x00, 0x01, 0xff, 0x89, 0x99,
		0x3d, 0x1d, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
		0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

func deleteSeedImageFiles(uploadDir string) error {
	items, err := os.ReadDir(uploadDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.IsDir() || !strings.HasPrefix(item.Name(), devSeedImagePrefix) {
			continue
		}
		if err := os.Remove(filepath.Join(uploadDir, item.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func cleanupFiles(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}
