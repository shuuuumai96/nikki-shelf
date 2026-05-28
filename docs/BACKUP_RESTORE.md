# Backup and Restore

Nikki data lives in two places:

- PostgreSQL: users, sessions, diary entries, tags, moods, image metadata.
- Upload storage: image files referenced by image metadata.

Back up both at the same point in time.

## Content Backup Archive

Nikki can generate a portable archive:

```text
nikki-backup.zip
├── entries.json
├── images/
├── manifest.json
└── RESTORE.md
```

Generate it from Settings with `Backup`, or call:

```bash
GET /api/export/backup
```

This archive is useful for export and inspection. It is not an automated database restore or import path, and Nikki does not currently provide a one-click import or restore action for it.

The app-level JSON/Markdown export and content backup archive are bounded to avoid unbounded in-process memory growth. App-level JSON/Markdown export is limited to 5,000 entries. App-level backup archive creation is limited to 5,000 entries and 10,000 images. Larger installations should use the operational backup archive described below.

## Operational Backup Archive

Operational restore uses a Nikki operational backup archive, not the content backup zip:

```text
nikki-operational-backup-YYYYmmdd-HHMMSS.tar.gz
├── manifest.json
├── db/
│   └── postgres.dump
├── uploads/
│   └── uploads.tar
└── SHA256SUMS
```

`db/postgres.dump` is a PostgreSQL custom-format dump created with `pg_dump -Fc`. Restore uses `pg_restore`. `uploads/uploads.tar` contains the matching upload storage contents from the same backup run.

For production Compose, use the backup script:

```bash
ENV_FILE=.env.production ./scripts/backup-production.sh
```

It creates the timestamped operational `.tar.gz` archive. If `AGE_RECIPIENT` is set, it also creates an encrypted `.age` copy and fails if `age` is not installed. Backups contain private diary text, images, password hashes, and operational metadata; encrypted artifacts are recommended before copying backups to S3 or any external storage.

The app-level backup archive and the operational backup archive are different things. The content archive is a readable export package. Operational recovery needs the PostgreSQL custom dump and the matching uploads archive restored together.

## First Setup Restore

On a new empty instance, open `/setup` and choose **Restore from backup**. The restore path is available only when the users table is empty and requires `NIKKI_FIRST_USER_BOOTSTRAP_TOKEN`. The backend verifies the token, archive layout, optional SHA256 checksums, manifest, PostgreSQL dump, and uploads tar before running restore. After successful restore, setup is locked and the app returns to the login screen.

This restore path intentionally does not import `nikki-backup.zip` content exports and is not available from normal Settings.

Restore is rejected when:

- the database already has any user
- the setup token is missing or incorrect
- the archive is not a valid Nikki operational backup archive
- the uploads tar contains an absolute path, `..`, or another unsafe path
- another setup restore is already in progress

Do not restore a database dump from one point in time with an uploads backup from another point in time. That can create missing image files or orphan files.

## Isolated Restore Verification

Use an isolated restore when verifying a backup without touching the live Nikki volumes.

Create the operational backup archive:

```bash
ENV_FILE=.env.production ./scripts/backup-production.sh
```

Start a separate PostgreSQL volume under another Compose project name:

```bash
docker compose -p nikki_restore up -d postgres
```

Restore the database:

```bash
mkdir -p /tmp/nikki-restore
tar -xzf backups/<timestamp>/nikki-operational-backup-<timestamp>.tar.gz -C /tmp/nikki-restore
cat /tmp/nikki-restore/db/postgres.dump | docker compose -p nikki_restore exec -T postgres pg_restore --data-only --no-owner --disable-triggers -U nikki -d nikki
```

Restore uploads into the isolated uploads volume:

```bash
docker run --rm \
  -v nikki_restore_nikki_uploads:/uploads \
  -v /tmp/nikki-restore:/backup:ro \
  alpine sh -c 'cd /uploads && tar -xf /backup/uploads/uploads.tar'
```

Avoid port collisions with the live app. Either use a temporary override file for alternate ports, or start the restored backend directly on an alternate port such as `18080`:

```bash
docker run --rm --name nikki_restore_backend \
  --network nikki_restore_default \
  -p 18080:8080 \
  -e NIKKI_ADDR=:8080 \
  -e NIKKI_DATABASE_URL='postgres://nikki:nikki@postgres:5432/nikki?sslmode=disable' \
  -e NIKKI_UPLOAD_DIR=/uploads \
  -e NIKKI_PUBLIC_UPLOAD_BASE=/uploads \
  -v nikki_restore_nikki_uploads:/uploads \
  nikki_restore-backend:latest
```

Verify counts:

```bash
docker compose exec -T postgres psql -U nikki -d nikki -t -A -F ',' \
  -c "SELECT (SELECT count(*) FROM entries), (SELECT count(*) FROM images);"

docker compose -p nikki_restore exec -T postgres psql -U nikki -d nikki -t -A -F ',' \
  -c "SELECT (SELECT count(*) FROM entries), (SELECT count(*) FROM images);"
```

Verify content equivalence with a hash over dates, titles, bodies, moods, and tags:

```bash
docker compose exec -T postgres psql -U nikki -d nikki -t -A \
  -c "SELECT md5(string_agg(entry_date || '|' || title || '|' || body || '|' || mood || '|' || tags_json::text, E'\n' ORDER BY id)) FROM entries;"

docker compose -p nikki_restore exec -T postgres psql -U nikki -d nikki -t -A \
  -c "SELECT md5(string_agg(entry_date || '|' || title || '|' || body || '|' || mood || '|' || tags_json::text, E'\n' ORDER BY id)) FROM entries;"
```

Verify a sample image returns HTTP 200 from the restored backend:

```bash
curl -i -b restored-cookie.txt http://localhost:18080/api/images/<image-id>/content
```

The restored entry count must match the source entry count, the restored image count must match the source image count, the content hashes must match, and a restored sample image must return HTTP 200.

## Verification Checklist

- `/setup` restores a valid operational archive only on an empty database with the correct setup token.
- Setup is locked after restore and a second restore attempt returns a conflict.
- Entries open by date.
- Entry body text is present.
- Mood and tags are present.
- Attached images load.
- `GET /api/export/backup` produces a zip with `entries.json`, `images/`, `manifest.json`, and `RESTORE.md` when the app-level archive limits are not exceeded.

## Cleanup and Repair

Treat destructive cleanup as a maintenance operation. Do not run it while users are actively uploading images, editing entries, restoring data, or deploying a release. Always take a current operational backup first, and always run dry-run before any destructive cleanup:

```bash
./nikki cleanup-images --dry-run
```

The report has three sections:

- `filesWithoutRows`: image files on disk with no DB metadata row.
- `rowsWithoutFiles`: DB metadata rows whose image file is missing.
- `rowsWithMissingEntries`: image metadata rows linked to an entry that no longer exists.

The destructive command:

```bash
./nikki cleanup-images
```

Deletes in this intended order:

1. Delete orphan files reported in `filesWithoutRows`.
2. Delete DB rows reported in `rowsWithMissingEntries`.

It intentionally does not delete `rowsWithoutFiles`. A DB row with a missing file can represent recoverable metadata if the operator still has the missing uploads backup.

DB rows and filesystem files are not protected by a single distributed transaction. If destructive cleanup partially fails, use the logs to identify which step failed, preserve the current backup, rerun `./nikki cleanup-images --dry-run`, and compare the new report with the pre-cleanup dry-run before taking further action. Expected recovery behavior is:

- If orphan file deletion fails, the database should still retain its existing image metadata; rerun dry-run and delete only the remaining orphan files after correcting the filesystem problem.
- If DB row deletion fails after orphan files were deleted, rerun dry-run and remove only the remaining `rowsWithMissingEntries` after correcting the database problem.
- If the operator is unsure which deletions completed, stop and verify from the backup, logs, database counts, and a fresh dry-run before retrying.

To repair `rowsWithoutFiles`:

1. Prefer restoring the missing file from the matching uploads backup.
2. If the file is permanently lost, inspect the affected row and entry first.
3. Delete only the specific unrecoverable image row:

```sql
DELETE FROM images WHERE id = <image_id>;
```

After repair, run `./nikki cleanup-images --dry-run` again and confirm the row no longer appears.

## Missing Image UI

If an entry has image metadata but the file is missing from upload storage, the reader and editor attachment grids show a missing-image state instead of relying on the browser's broken image icon. This is display-layer behavior only; it does not rewrite diary body content and does not repair or import files.

The state includes the available recovery details returned by the API:

- image ID
- file name, when present
- public URL
- entry date

Use that information with `./nikki cleanup-images --dry-run` to confirm whether the database row points to a missing file. Prefer restoring the file from the matching uploads backup before deleting metadata. If the file is permanently lost, the existing editor delete action can delete the affected image row through the normal image delete API; do not treat that as automated cleanup or restore.
