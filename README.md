# Nikki

Nikki is a self-hosted, text-first diary for daily personal records.

It is currently designed for one person or a small trusted household, with the primary writing experience in a PC browser. The project prioritizes recoverable self-hosted data, clear operational boundaries, and safe diary attachments over broad feature breadth.

Nikki is not a public SaaS product and is not intended to become a general-purpose workspace, knowledge-base, or photo-library app.

<img width="960" height="540" alt="20260521_Nikki_About" src="https://github.com/user-attachments/assets/eac189c4-1557-46a1-b69a-70abdbe04d52" />

## Project Status

Nikki is moving toward a future public self-hosted OSS direction, but the current product remains intentionally small:

- single-user or small trusted household use
- single-tab writing assumption
- PC-browser writing as the primary experience
- desktop-supported image attachments
- recoverable PostgreSQL and upload data
- operator-controlled deployment

Public exposure requires careful production configuration, access control, and backup/restore practice. See the deployment runbooks before exposing any instance outside a trusted private environment.

## What Nikki Does

- date-based diary entries
- title and body text
- tags
- moods
- basic past-entry search
- random memory shelf for revisiting older entries
- password change with full session revocation
- owner-visible security history for authentication, destructive, restore, and export events
- normal autosave for single-tab writing
- stale-version conflict fallback
- desktop-supported image attachments
- missing-image UI
- app-level backup archive
- operational backup archive and first-setup restore
- `cleanup-images` command for image/file consistency checks
- installable web app metadata, icons, and basic app-shell caching
- Docker Compose deployment

## What Nikki Does Not Do

- robust multi-tab editing
- automatic conflict merge
- mobile-first writing or image workflows
- mobile image upload, retry, or remove flows
- full mobile photo diary workflow
- rich inline image rendering or editing
- full offline-first PWA behavior
- offline writing, offline sync, or background recovery
- photo library management
- public SaaS or multi-tenant hosting
- sharing
- AI features
- statistics expansion
- broad visual redesign
- advanced Markdown/editor behavior
- advanced reflection, analytics, or recommendation features

See [docs/FROZEN_SCOPE.md](docs/FROZEN_SCOPE.md).

## Product Direction

These themes describe direction, not a commitment that the features already exist.

- **PC Browser Writing**: keep the desktop browser writing cockpit calm, fast, and reliable.
- **Archive & Retrieval**: make older records easier to read, search, revisit, and recover while keeping retrieval diary-oriented.
- **Lightweight Reflection**: support modest review and reflection through bounded diary-centered surfaces, without turning Nikki into analytics, coaching, or AI-first software.
- **Recoverable Self-hosted Data**: keep backup, restore, consistency checks, and operator clarity central.
- **Public OSS Readiness**: prepare documentation, contribution expectations, release process, and security handling for a public repository.
- **Installable Web App**: keep installability narrow: manifest metadata, app icons, standalone display, mobile web app meta tags, and basic service-worker app-shell caching. Offline writing, background sync, push notifications, and offline-first behavior remain out of scope unless separately approved.

See [docs/ROADMAP.md](docs/ROADMAP.md).

## Architecture Overview

Nikki runs as a Vue/Vite frontend, a Go/Echo backend, PostgreSQL, and local upload storage. The frontend container serves static assets with nginx. nginx proxies `/api/` and legacy `/uploads/` image requests to the backend service.

The default runtime is Docker Compose. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the detailed architecture.

## Quick Start

For local evaluation or development:

```powershell
$setupToken = python -c "import secrets; print(secrets.token_urlsafe(32))"
$env:NIKKI_FIRST_USER_BOOTSTRAP_TOKEN = $setupToken
docker compose up -d
$setupToken
```

Use the printed value as the setup token on `/setup`. Keep it private; it is
only for creating the first owner account on an empty database.

On macOS/Linux shells:

```bash
export NIKKI_FIRST_USER_BOOTSTRAP_TOKEN="$(python3 -c 'import secrets; print(secrets.token_urlsafe(32))')"
docker compose up -d
printf '%s\n' "$NIKKI_FIRST_USER_BOOTSTRAP_TOKEN"
```

Default local endpoints from [docker-compose.yml](docker-compose.yml):

- Frontend: `http://localhost:8089`
- Backend API: `http://localhost:8080`
- Health check: `http://localhost:8080/api/health`

On an empty database, Nikki redirects unauthenticated browsers to `/setup`.
That screen can create the first owner account or restore a Nikki operational
backup archive. Both paths require the setup token. After the owner account
exists or restore succeeds, setup is locked and additional signup remains closed
unless `NIKKI_SIGNUP_ENABLED` is explicitly enabled.

Persistent Docker volumes:

- `nikki_postgres_data`: PostgreSQL data
- `nikki_uploads`: uploaded image files

## Development

Developers can choose either a local toolchain or Docker-only checks. Keep both
paths working so contributors do not need to install every runtime locally just
to validate the repository.

Docker-only validation:

```bash
docker compose -f docker-compose.check.yml run --rm checks
```

This runs frontend install, format check, and production build, then backend
tests, backend `goimports` check, and `git --no-pager diff --check`. It uses
Docker named volumes for dependency caches, `frontend/node_modules`, and
`frontend/dist`, so the check should not leave generated files in the working
tree.

Local validation remains supported for developers who prefer local tools.

Frontend install and build:

```bash
cd frontend
corepack pnpm install --frozen-lockfile
corepack pnpm build
```

Frontend formatting:

```bash
cd frontend
corepack pnpm format
corepack pnpm format:check
```

Backend tests:

```bash
cd backend
go test ./...
```

The backend Go module lives under `backend/` and uses the public module path `github.com/shuuuumai96/nikki-shelf/backend`. Backend formatting uses `goimports` with this module path as the local import group.

Repository formatting:

```bash
python3 scripts/format.py
python3 scripts/format.py --check
```

On Windows, if `python3` is not available on PATH:

```powershell
python .\scripts\format.py
python .\scripts\format.py --check
```

Repository checks and Docker commands:

```bash
git --no-pager diff --check
docker compose build
docker compose up -d
```

## Data, Backup, and Restore

Diary data is personal and not reproducible if lost. Nikki data lives in two places:

- PostgreSQL: users, sessions, audit events, diary entries, tags, moods, image metadata, and settings
- Upload storage: image files referenced by database metadata

Back up the database and uploads from the same point in time. The app-level backup archive is useful for export and inspection, but it is not an automated database restore or import path.

Operational backups use `nikki-operational-backup-YYYYmmdd-HHMMSS.tar.gz`, which contains `manifest.json`, a PostgreSQL custom-format `db/postgres.dump`, `uploads/uploads.tar`, and optional `SHA256SUMS`. On a new empty instance, `/setup` can verify and restore this archive when the setup token matches `NIKKI_FIRST_USER_BOOTSTRAP_TOKEN`.

Restore verification should use isolated volumes and non-conflicting ports, not live data volumes. For production-style backups:

```bash
ENV_FILE=.env.production ./scripts/backup-production.sh
```

Backups can contain private diary text, images, password hashes, and other operational data. Encrypted backup artifacts are recommended before copying backups to external storage.

See [docs/BACKUP_RESTORE.md](docs/BACKUP_RESTORE.md).

## Search and Memory Shelf

Nikki includes basic search for past entries and a small random memory shelf on
the Today screen. The memory shelf selects earlier entries for the signed-in
user only, never today's entry, and only entries that have a title, body, or
attached image.

The memory shelf is not a photo library, recommendation system, statistics
surface, or AI feature. It is a lightweight diary review aid. Users can collapse
the shelf in the Today view without disabling the feature. The Settings screen
can disable the shelf and can hide selected moods from memory results. The
default hidden moods are `tired` and `sad`; this preference is stored in the
browser's local storage.

## Image Attachments and Cleanup

Images are diary entry attachments, not photo library items. Desktop-supported image attachments are in current scope. Mobile image upload flows are not release-supported.

The backend accepts detected JPEG, PNG, GIF, and WebP images. Each image is limited to 8 MiB, and each entry can have up to 3 images. The per-entry image limit is enforced inside a database transaction.

Uploaded diary images are served through authenticated, owner-checked endpoints. Normal image display uses `/api/images/<id>/content`; legacy `/uploads/<name>` requests are also checked against image metadata and entry ownership before any file is served.

Image storage quotas are configurable. By default, each user can store up to 1 GiB and 1,000 images. `NIKKI_IMAGE_USER_QUOTA_BYTES=0` disables the per-user byte quota, `NIKKI_IMAGE_USER_QUOTA_COUNT=0` disables the per-user count quota, and `NIKKI_IMAGE_TOTAL_QUOTA_BYTES` is disabled by default when set to `0`.

If image metadata exists but the referenced upload cannot be loaded, the reader and editor attachment grids show a missing-image state with available recovery details such as image ID, file name, URL, and entry date. This is a visibility and recovery aid only; it does not repair files or rewrite diary content.

Run cleanup in dry-run mode first:

```bash
cd backend
go run ./cmd/server cleanup-images --dry-run
```

Inside Docker Compose:

```bash
docker compose exec backend /app/nikki cleanup-images --dry-run
```

Destructive cleanup requires operator review and a current backup. `cleanup-images` reports orphan files, image rows whose files are missing, and image rows linked to missing entries. Destructive cleanup deletes orphan files and image rows linked to missing entries; it does not automatically delete rows whose files are missing.

See [docs/BACKUP_RESTORE.md](docs/BACKUP_RESTORE.md) for cleanup and repair details.

## Deployment Notes

Nikki can be operated in local, private, or public single-instance environments. Public exposure requires the operator to apply the production configuration, access controls, and backup/restore practices documented in this repository. The README is not a deployment runbook; use the linked operational docs for host-specific steps.

Production Compose uses `.env.production`. Do not commit `.env.production`. For HTTPS production, use secure cookies, exact CORS origins, disabled public signup, and a long random `NIKKI_FIRST_USER_BOOTSTRAP_TOKEN`. The `/setup` screen is available only while the database has no users, and owner creation or operational backup restore through it still requires the token. If possible, keep the instance behind a Security Group, VPN, SSM tunnel, or Tailscale-style private path until setup is complete. Backend and PostgreSQL ports must not be publicly exposed. See [docs/CONFIGURATION.md](docs/CONFIGURATION.md) for production-relevant environment variables.

Security history is stored in PostgreSQL as bounded audit events and is visible only to owner accounts. It records event names, outcome, actor metadata, target IDs, reason codes, request IDs, remote IPs, and small operational metadata; it does not store diary title/body/tags, passwords, cookies, CSRF tokens, session tokens, request bodies, upload file paths, or SQL arguments. `NIKKI_AUDIT_RETENTION_DAYS` controls retention and defaults to 180 days.

Uploaded images are served from local upload storage in the current single-host design. S3, if used, is for encrypted backup artifacts only, not as the image-serving backend.

Deployment and release documents:

- [docs/AWS_EC2_DEPLOYMENT.md](docs/AWS_EC2_DEPLOYMENT.md)
- [docs/AWS_EC2_PRIVATE_TEST_RUNBOOK.md](docs/AWS_EC2_PRIVATE_TEST_RUNBOOK.md)
- [docs/RELEASE_CHECKLIST.md](docs/RELEASE_CHECKLIST.md)

## Documentation

- [docs/DESIGN.md](docs/DESIGN.md)
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- [docs/CONFIGURATION.md](docs/CONFIGURATION.md)
- [docs/FROZEN_SCOPE.md](docs/FROZEN_SCOPE.md)
- [docs/ROADMAP.md](docs/ROADMAP.md)
- [docs/OSS_READINESS.md](docs/OSS_READINESS.md)
- [docs/BACKUP_RESTORE.md](docs/BACKUP_RESTORE.md)
- [docs/RELEASE_CHECKLIST.md](docs/RELEASE_CHECKLIST.md)
- [docs/AWS_EC2_DEPLOYMENT.md](docs/AWS_EC2_DEPLOYMENT.md)
- [docs/AWS_EC2_PRIVATE_TEST_RUNBOOK.md](docs/AWS_EC2_PRIVATE_TEST_RUNBOOK.md)
- [CHANGELOG.md](CHANGELOG.md)
- [SECURITY.md](SECURITY.md)
- [CONTRIBUTING.md](CONTRIBUTING.md)

## License

Nikki is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).

Project notice information is in [NOTICE](NOTICE). Third-party dependency notices are summarized in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
