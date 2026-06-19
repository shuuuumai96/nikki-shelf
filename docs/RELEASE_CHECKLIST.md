# Release Checklist

Release shape: the current release remains a single-tab, self-hosted diary for personal daily use by one person or a small trusted household. It preserves PC-browser writing, recoverable data, and desktop-supported safe image attachments.

## Gates

| Gate | Status | Evidence |
| --- | --- | --- |
| Frontend build passed | Pass | `docker compose -f docker-compose.check.yml run --rm checks`; local equivalent: `cd frontend && corepack pnpm install --frozen-lockfile && corepack pnpm build` |
| Backend tests passed | Pass | `docker compose -f docker-compose.check.yml run --rm checks`; local equivalent: `cd backend && go test ./...` |
| Clean restore verified | Pass | Source/restored entries `14 / 14`; source/restored images `2 / 2`; missing image count `0`; restored sample image HTTP `200` |
| Backup includes entries and images | Pass | Content backup archive contains `entries.json`, `images/`, `manifest.json`, `RESTORE.md`; operational backup archive contains `manifest.json`, `db/postgres.dump`, `uploads/uploads.tar`, and checksums |
| Cleanup dry-run works | Pass | `cleanup-images --dry-run` reports image/file mismatches without deletion |
| Destructive cleanup verified | Pass | Inspected orphan file, took backup, ran cleanup, second dry-run was clean, valid image files remained |
| Missing-image UI verified | Pass | Valid image displayed in reader and editor; missing image row displayed image ID, file name, public URL, entry date, and recovery hint; `cleanup-images --dry-run` confirmed the mismatch; existing editor delete flow removed the affected row when tested |
| Image routes are owner-checked | Pass | Normal `/api/images/<id>/content` and legacy `/uploads/<name>` routes require authentication and verify entry ownership before serving files |
| Memory shelf verified | Pass | Today-screen memory shelf returns only signed-in-user past entries, supports mood exclusions, collapses without disabling the feature, and offers a direct return to Today after opening a memory |
| Rejected DFC scope absent | Pass | Quick Capture and recoverability visibility are not active release requirements; no Settings backup dashboard, restore checklist UI links, normal restore UI, or content-backup import is claimed |
| Single-tab writing verified | Pass | 390px text writing and save status verified; desktop build verified |
| Multi-tab editing explicitly unsupported | Pass | Frozen in `docs/FROZEN_SCOPE.md`; README states single-tab writing assumption |
| Mobile image upload explicitly unsupported | Pass | Frozen in `docs/FROZEN_SCOPE.md`; README states unsupported; upload controls hidden on phone-sized viewports |
| RESTORE.md updated with isolated restore and alternate port verification | Pass | `docs/BACKUP_RESTORE.md` documents isolated volumes, alternate port `18080`, pg_restore restore, uploads tar restore, count/hash checks, sample image HTTP `200` |

## AWS Private Deployment Gate

This gate must pass before a private EC2 Docker Compose test is treated as deployment-ready.

Validation may use either local developer toolchains or the Docker-only check
runner. The Docker-only path is:

```bash
docker compose -f docker-compose.check.yml run --rm checks
```

| Gate | Required Evidence |
| --- | --- |
| Frontend build passes | Local `cd frontend && corepack pnpm install --frozen-lockfile && corepack pnpm build`, or Docker-only `docker compose -f docker-compose.check.yml run --rm checks` |
| Backend tests pass | Local `cd backend && go test ./...`, or Docker-only `docker compose -f docker-compose.check.yml run --rm checks` |
| Docker Compose build passes | `docker compose build` |
| `.env.production` is required for production Compose | `docker compose -f docker-compose.yml -f docker-compose.prod.yml config` fails when `.env.production` or required variables are missing |
| Production Compose config passes | `docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env.production config` |
| Production config check passes | `docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env.production config \| scripts/check-production-config.sh` |
| Backend port is not published in production Compose | Production config shows backend `expose: 8080` and no backend `ports` entry |
| Secure cookie mode is available | `NIKKI_COOKIE_SECURE=true` sets the session cookie `Secure` attribute |
| CORS is restricted in production | `.env.production` sets `NIKKI_CORS_ALLOWED_ORIGINS` to the production HTTPS origin, not `*` |
| Signup is not open in production | `NIKKI_SIGNUP_ENABLED=false` and `NIKKI_FIRST_USER_SETUP_ENABLED=false`; first setup uses `/setup` or setup APIs with the operator-controlled bootstrap token; additional public signup is blocked |
| First-user bootstrap token is configured | `NIKKI_FIRST_USER_BOOTSTRAP_TOKEN` is set to a long random secret and is not a placeholder |
| CSRF protects mutating authenticated requests | Authenticated `POST`, `PUT`, and `DELETE` requests require `X-CSRF-Token`; `GET` export/download routes still work without it |
| Trusted proxy behavior is explicit | `NIKKI_IP_EXTRACTOR_MODE` and `NIKKI_TRUSTED_PROXY_CIDRS` match the deployed reverse proxy chain |
| Auth rate limiting exists | `NIKKI_AUTH_RATE_LIMIT_IP_ATTEMPTS`, `NIKKI_AUTH_RATE_LIMIT_ACCOUNT_ATTEMPTS`, and `NIKKI_AUTH_RATE_LIMIT_WINDOW` protect login, signup, password change, account deletion, setup owner creation, and setup restore APIs |
| Audit retention is configured | `NIKKI_AUDIT_RETENTION_DAYS` is present and positive; owner Settings can load recent security history |
| Production smoke script is available | `sh ./scripts/smoke-production.sh` verifies the non-destructive production HTTP flows when `NIKKI_SMOKE_BASE_URL`, `NIKKI_SMOKE_USERNAME`, and `NIKKI_SMOKE_PASSWORD` are set in a private shell; password-change and backup checks are opt-in |
| Backup command exists | `scripts/backup-production.sh` creates a timestamped Nikki operational backup archive and detects the backend `/uploads` Docker volume when `UPLOADS_VOLUME` is not set |
| Backup refuses missing uploads volume | `UPLOADS_VOLUME=missing-volume scripts/backup-production.sh` fails before running `tar` |
| Backup artifacts are usable as one set | Backup creates a non-empty operational `.tar.gz` containing DB dump, uploads archive, manifest, and checksums from the same timestamp, or clearly warns if uploads are empty |
| Encrypted backup path is available | `AGE_RECIPIENT=... scripts/backup-production.sh` creates `.age` artifacts; missing `age` fails |
| S3 upload is opt-in and encrypted-first | `scripts/upload-backup-s3.sh` requires bucket/prefix/region and encrypted artifacts by default |
| Image metadata stripping is configured | `NIKKI_STRIP_IMAGE_METADATA=true` is set in production and JPEG/PNG stripping limitations are documented |
| Restore test procedure exists | `docs/BACKUP_RESTORE.md` documents isolated restore verification |
| No real secrets are committed | Secret search has no real credential or private-key hits |

## Public Production Release Gate

This gate is mandatory before opening Nikki to the public internet on the single-instance EC2 deployment.

| Gate | Required Evidence |
| --- | --- |
| Production config check passes | `docker compose --env-file .env.production -f docker-compose.yml -f docker-compose.prod.yml config \| ./scripts/check-production-config.sh` |
| Containers are healthy | `docker compose --env-file .env.production -f docker-compose.yml -f docker-compose.prod.yml ps` shows healthy `postgres`, `backend`, and `frontend` |
| Public health endpoint returns 200 | `curl -fsS https://your-real-domain.example/api/health` |
| First setup succeeds using the setup-token flow | Empty DB `/api/setup/status` returns `needsSetup=true`; `/api/setup/owner` and `/api/setup/restore/verify` without or with an incorrect setup token return `403`; `/api/setup/owner` with the correct `NIKKI_FIRST_USER_BOOTSTRAP_TOKEN` returns `200` and creates the owner user, or `/setup` restores a valid operational backup archive with the correct token; a second setup attempt returns `409`; then keep `NIKKI_SIGNUP_ENABLED=false` and `NIKKI_FIRST_USER_SETUP_ENABLED=false` |
| Login works | Browser login succeeds at the exact production HTTPS origin |
| Logout works | Browser logout clears the session |
| Re-login works | Existing user can log in again after logout |
| Password change works | Existing user can change their password from Settings; current and other active sessions are invalidated and the user can log in again with the new password |
| Security history works | Owner Settings shows recent audit events for login failure/success, password change, account deletion, setup restore, export, entry deletion, and image deletion without diary content or secrets |
| Automated production smoke passes | `sh ./scripts/smoke-production.sh` passes with `NIKKI_SMOKE_BASE_URL`, `NIKKI_SMOKE_USERNAME`, and `NIKKI_SMOKE_PASSWORD` set in a private shell; set `NIKKI_SMOKE_RUN_PASSWORD_CHANGE=true` and `NIKKI_SMOKE_RUN_BACKUP=true` only when the operator accepts those side effects |
| Entry create/edit works | Create a diary entry, edit it, refresh, and confirm persisted content |
| Memory shelf works | Confirm Today shows past entries for the signed-in user, `tired` and `sad` are hidden by default for new browser preferences, collapse/expand does not disable the feature, and opening a memory shows a direct return to Today |
| Image upload/display/delete works | Upload an image, confirm it displays in reader and editor, delete it through the editor, and confirm it no longer displays |
| Missing image state works | Create a disposable missing-file condition, confirm reader and editor show the missing-image placeholder with available recovery details, run `cleanup-images --dry-run` to confirm the mismatch, and verify existing editor delete behavior if deletion is used |
| Unauthenticated API access returns expected 401 | `curl -i https://your-real-domain.example/api/entries` returns `401` |
| Image routes reject unauthenticated access | Unauthenticated requests to `/api/images/<known-image-id>/content` and legacy `/uploads/<known-stored-name>` do not serve image bytes |
| Service worker cache stays static-only | Production `frontend/public/sw.js` excludes `/api` and `/uploads`; authenticated API responses, uploads, and diary data are not cached by the service worker |
| Backup command succeeds | `ENV_FILE=.env.production ./scripts/backup-production.sh` detects the production backend `/uploads` Docker volume and exits successfully |
| Backup artifacts are complete | Operational backup archive includes `manifest.json`, `db/postgres.dump`, `uploads/uploads.tar`, and `SHA256SUMS` when checksum tooling is available |
| Backup/export copy is accurate | Documentation states that app exports can contain private diary text/images and are not automated database restore |
| Isolated restore succeeds | Restore test uses isolated volumes and alternate ports, not production volumes |
| Restored DB count matches source | Restore evidence records matching source/restored entry and image counts |
| Restored upload hash check passes | Restore evidence records matching upload checksums or hash manifest validation |
| Sample restored image returns 200 when authenticated | Authenticated request to a restored sample image returns HTTP `200` |
| Public network exposure is limited | Security Group keeps `22/tcp`, `8080/tcp`, and `5432/tcp` closed; only `443/tcp` is required and `80/tcp` is temporary/optional |
| Public Caddy proxy is correct | Caddy proxies to `127.0.0.1:8089`, uses public ACME TLS, enables compression, and allows setup restore uploads up to the backend restore upload limit |
| Production safety env is set | `NIKKI_COOKIE_SECURE=true`, exact `NIKKI_CORS_ALLOWED_ORIGINS`, `NIKKI_SIGNUP_ENABLED=false`, `NIKKI_FIRST_USER_SETUP_ENABLED=false`, `NIKKI_FIRST_USER_BOOTSTRAP_TOKEN` is a long random secret, `NIKKI_AUDIT_RETENTION_DAYS` is positive, and `NIKKI_STRIP_IMAGE_METADATA=true` |
| Schema-changing release has backup first | Automatic idempotent schema setup is understood; do not add a migration framework unless an explicit task requests it |

## Unsupported In This Release

- robust multi-tab editing
- automatic merge/conflict resolution
- mobile image upload support
- full mobile photo diary workflow
- inline rich image placement
- full offline-first PWA behavior
- offline writing, offline sync, background recovery, or authenticated diary-data caching
- photo library management
- advanced reflection, analytics, or recommendation features
