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

The app-level JSON/Markdown export and content backup archive are bounded to avoid unbounded in-process memory growth. App-level JSON/Markdown export is limited to 5,000 entries. App-level backup archive creation is limited to 5,000 entries and 10,000 images. Larger installations should use the operational PostgreSQL dump plus matching uploads backup described below.

## Operational Backup

Create a PostgreSQL dump:

```bash
docker compose exec postgres pg_dump -U nikki -d nikki > nikki.sql
```

Back up the upload volume or directory at the same time. In the default Docker Compose setup, uploaded images are stored in the `nikki_uploads` Docker volume.

For production Compose, prefer the backup script:

```bash
ENV_FILE=.env.production ./scripts/backup-production.sh
```

It creates a timestamped PostgreSQL dump, uploads tar archive, manifest, and checksums. If `AGE_RECIPIENT` is set, it also creates encrypted `.age` artifacts and fails if `age` is not installed. Backups contain private diary text and images; encrypted artifacts are recommended before copying backups to S3 or any external storage.

The app-level backup archive and the operational backup set are different things. The archive is a readable export package. Operational recovery needs the database dump and the matching uploads backup restored together.

## Operational Restore

1. Stop Nikki.
2. Restore the PostgreSQL data from the matching dump.
3. Restore the uploads volume or directory from the matching image backup.
4. Start Nikki.
5. Sign in and verify entries, dates, moods, tags, and images.

Do not restore a database dump from one point in time with an uploads backup from another point in time. That can create missing image files or orphan files.

## Isolated Restore Verification

Use an isolated restore when verifying a backup without touching the live Nikki volumes.

Create the operational backup:

```bash
docker compose exec -T postgres pg_dump -U nikki -d nikki > nikki.sql
docker run --rm -v nikki_nikki_uploads:/uploads -v "$PWD":/backup alpine tar -cf /backup/nikki-uploads.tar -C /uploads .
```

Start a separate PostgreSQL volume under another Compose project name:

```bash
docker compose -p nikki_restore up -d postgres
```

Restore the database:

```bash
cat nikki.sql | docker compose -p nikki_restore exec -T postgres psql -U nikki -d nikki
```

Restore uploads into the isolated uploads volume:

```bash
docker run --rm -v nikki_restore_nikki_uploads:/uploads -v "$PWD":/backup alpine sh -c 'cd /uploads && tar -xf /backup/nikki-uploads.tar'
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

- Entries open by date.
- Entry body text is present.
- Mood and tags are present.
- Attached images load.
- `GET /api/export/backup` produces a zip with `entries.json`, `images/`, `manifest.json`, and `RESTORE.md` when the app-level archive limits are not exceeded.

## Cleanup and Repair

Use cleanup dry-run before any destructive cleanup:

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

Deletes orphan files from `filesWithoutRows` and deletes DB rows from `rowsWithMissingEntries`.

It intentionally does not delete `rowsWithoutFiles`. A DB row with a missing file can represent recoverable metadata if the operator still has the missing uploads backup. To repair it:

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
