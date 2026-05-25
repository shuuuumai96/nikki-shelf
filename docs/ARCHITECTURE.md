# Nikki Architecture

## 1. Architecture Summary

Nikki is a self-hosted, text-first diary for daily personal records. It is designed for one person or a small trusted household, with PC-browser writing as the primary experience, recoverable self-hosted data, and desktop-supported safe image attachments.

The current release is intentionally small. It supports daily text writing, tags, moods, normal single-tab autosave, desktop image attachments, app-level backup export, operational restore documentation, and image consistency cleanup.

## 2. Runtime Components

- `frontend/`: Vue 3 + Vite application. The production container builds static assets and serves them with nginx.
- `frontend/nginx/default.conf`: nginx configuration. It serves the frontend and proxies `/api/` and legacy `/uploads/` image requests to the backend service.
- `backend/`: Go application using Echo and pgx-compatible PostgreSQL access through `database/sql`.
- PostgreSQL: stores users, sessions, diary entries, image metadata, and settings.
- Uploads storage: stores image files in `NIKKI_UPLOAD_DIR`; Docker Compose mounts this as the `nikki_uploads` volume.
- `docker-compose.yml`: starts PostgreSQL, backend, and frontend containers.

Default exposed ports in `docker-compose.yml`:

- frontend/nginx: `8089:80`
- backend: `8080:8080`

## 3. Repository Structure

```text
.
├── backend/
│   ├── cmd/server/                  # server entry point and cleanup-images subcommand
│   ├── internal/app/                # runtime setup, config, cleanup command
│   ├── internal/auth/               # users and sessions
│   ├── internal/db/                 # database open, migration, schema.sql
│   ├── internal/entries/            # diary entry API, service, repository
│   ├── internal/exporter/           # backup/export archive generation
│   ├── internal/images/             # image upload, storage, cleanup
│   ├── internal/logx/               # logging middleware and helpers
│   └── internal/stats/              # current statistics support
├── docs/
├── frontend/
│   ├── nginx/default.conf           # production nginx config
│   └── src/
│       ├── features/auth/
│       ├── features/calendar/
│       ├── features/entries/
│       ├── features/settings/
│       └── shared/
├── docker-compose.yml
└── README.md
```

Important paths:

- backend command: `backend/cmd/server/main.go`
- database schema: `backend/internal/db/schema.sql`
- exporter: `backend/internal/exporter/`
- image handling: `backend/internal/images/`
- entry UI: `frontend/src/features/entries/`
- shared UI components: `frontend/src/shared/components/`

## 4. Data Model

Core entities are defined in `backend/internal/db/schema.sql`.

- `users`: application users. Each user has a unique username and password hash.
- `sessions`: login sessions. Each session belongs to a user and is removed when the user is deleted.
- `entries`: diary entries. Each entry belongs to a user, has a unique `(user_id, entry_date)` pair, stores title, body, mood, tags JSON, and a `version` for stale update detection.
- `images`: image attachment metadata. Each image belongs to an entry, stores the disk file path, public URL, original/safe file name, size, MIME type, and creation time.
- `settings`: key/value settings table.

Images are related to entries through `images.entry_id`. Entries are related to users through `entries.user_id`. Sessions are related to users through `sessions.user_id`.

## 5. Entry Lifecycle

Entry creation inserts a row into `entries` with normalized date, title, body, mood, tags JSON, and timestamps. The `(user_id, entry_date)` constraint prevents duplicate entries for the same date.

Entry update sends the current client `version`. The repository updates only when `id`, `user_id`, and `version` match, then increments `version`. If no row is updated but the entry exists, the backend returns a stale-version conflict.

Autosave uses the normal update path. Nikki assumes one active writing tab.

Entry deletion removes the entry row. The database cascades image metadata deletion through the foreign key, and the entry service asks the image service to delete attached image files.

## 6. Image Lifecycle

Image upload is handled by the backend image service.

1. The request targets an existing entry owned by the user.
2. The service rejects requests that would exceed the per-entry attachment limit.
3. Each file is checked against the 8 MiB size limit.
4. Storage detects content type and allows JPEG, PNG, GIF, and WebP.
5. The backend generates a random stored file name.
6. The file is written to the upload directory.
7. The backend opens a database transaction, locks the parent entry row with `FOR UPDATE`, re-checks entry ownership, the current per-entry image count, per-user byte/count quotas, and the optional global byte quota.
8. Image metadata rows are inserted and committed. If the transaction fails after files were written, newly written files are deleted best-effort.
9. The response returns image metadata for the entry.

Image display uses DB-backed owner verification. API image metadata exposes `/api/images/<id>/content` as the normal content URL. That endpoint requires an authenticated session, joins `images` to `entries`, verifies `entries.user_id`, and then serves the file path from the authorized DB row. Legacy `/uploads/<stored-name>` requests are retained for compatibility but are also resolved through image metadata and entry ownership before serving content.

Authenticated image responses use `Cache-Control: private, max-age=3600`.

Default image quotas are 1 GiB per user and 1,000 images per user. `NIKKI_IMAGE_USER_QUOTA_BYTES=0` disables the per-user byte quota, `NIKKI_IMAGE_USER_QUOTA_COUNT=0` disables the per-user count quota, and `NIKKI_IMAGE_TOTAL_QUOTA_BYTES=0` disables the optional global byte quota.

Image deletion deletes the database row, then deletes the file. If file deletion fails, the backend logs and returns the error.

Entry deletion asks image storage to delete files for images that were attached to the entry.

Missing-image detection is surfaced in the frontend when image metadata exists but the served image cannot be loaded. `cleanup-images` compares database image rows, existing entries, and files in the upload directory.

## 7. Backup and Restore Architecture

The app-level backup archive is generated by `backend/internal/exporter/`. It contains entries, images, a manifest, and restore notes.

`GET /api/entries` is cursor-paginated. It returns `{ items, nextCursor, hasMore }`, defaults to `per_page=50`, and rejects `per_page` values above 100. Cursors are opaque keyset cursors over `entry_date DESC, id DESC`.

App-level JSON/Markdown export and app-level backup archive generation have guardrails to avoid unbounded memory growth: JSON/Markdown export is limited to 5,000 entries, and backup archive creation is limited to 5,000 entries and 10,000 images. Operational backup remains the PostgreSQL dump plus matching uploads backup.

Operational backup has two parts:

- PostgreSQL dump from the `postgres` service.
- Uploads archive or volume backup from the `nikki_uploads` volume.

The manifest describes exported content. The uploads archive preserves files referenced by image metadata.

Restore verification should use isolated volumes and alternate ports so live data is not affected. Verification should compare entry/image counts, content hashes where practical, and sample image HTTP responses.

See [BACKUP_RESTORE.md](BACKUP_RESTORE.md).

## 8. Error and Failure Handling

- `409` conflict: duplicate date or stale entry version. Stale version is the conflict fallback for unsupported multi-tab editing.
- Missing image: the UI displays missing image details instead of hiding the mismatch.
- Upload validation failure: unsupported image content type, too-large image, too many images, or missing entry returns an API error.
- Cleanup dry-run: reports mismatches without deleting files or rows.
- Cleanup destructive mode: deletes orphan files and image rows linked to missing entries after operator review.
- Restore verification failures: count, hash, or sample image mismatches indicate the database dump and uploads backup do not match or restore was incomplete.

## 9. Operational Commands

Frontend build:

```bash
cd frontend
corepack pnpm build
```

Backend tests:

```bash
cd backend
go test ./...
```

Whitespace check:

```bash
git diff --check
```

Docker Compose startup:

```bash
docker compose up -d
```

Operational database backup:

```bash
docker compose exec postgres pg_dump -U nikki -d nikki > nikki.sql
```

Backup archive API:

```text
GET /api/export/backup
```

Cleanup dry-run:

```bash
cd backend
go run ./cmd/server cleanup-images --dry-run
```

Cleanup destructive mode:

```bash
cd backend
go run ./cmd/server cleanup-images
```

Docker cleanup invocation:

```bash
docker compose exec backend /app/nikki cleanup-images --dry-run
docker compose exec backend /app/nikki cleanup-images
```

Restore verification commands are documented in [BACKUP_RESTORE.md](BACKUP_RESTORE.md) because they require environment-specific volume names and ports.

## 10. Release Boundaries

Unsupported in this release:

- robust multi-tab editing
- automatic conflict merge
- mobile image upload support
- full mobile photo diary workflow
- rich inline image placement
- drag-to-line image insertion
- full offline-first PWA behavior
- service workers or offline writing
- photo library management
- sharing
- AI features
- statistics expansion

See [FROZEN_SCOPE.md](FROZEN_SCOPE.md).

## 11. Architecture Decision Records

- [adr/0001-release-scope-feature-cuts.md](adr/0001-release-scope-feature-cuts.md)
- [adr/0002-single-tab-writing-assumption.md](adr/0002-single-tab-writing-assumption.md)
- [adr/0003-desktop-supported-image-attachments.md](adr/0003-desktop-supported-image-attachments.md)
- [adr/0004-backup-restore-as-core-requirement.md](adr/0004-backup-restore-as-core-requirement.md)
