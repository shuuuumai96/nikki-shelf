# Configuration

This page is the production-oriented reference for Nikki environment variables. Keep secrets out of git, tickets, screenshots, shared shell logs, and chat. Production Compose reads `.env.production`; copy `.env.production.example`, replace placeholders, and keep the file private.

## Database

| Variable | Purpose | Production expectation | Safe default where known | Misconfiguration risk |
| --- | --- | --- | --- | --- |
| `POSTGRES_DB` | PostgreSQL database name created and used by the Compose `postgres` service. | Use the same production database name in `NIKKI_DATABASE_URL`. | `.env.production.example` uses `nikki`. | If it differs from `NIKKI_DATABASE_URL`, the backend may fail to start or connect to the wrong database. |
| `POSTGRES_USER` | PostgreSQL role used by the backend. | Use a dedicated Nikki database user. | `.env.production.example` uses `nikki`. | If it differs from `NIKKI_DATABASE_URL`, login to PostgreSQL fails; over-privileged users increase blast radius. |
| `POSTGRES_PASSWORD` | PostgreSQL password for `POSTGRES_USER`. | Use a long random secret and keep it private. | No safe shared default; replace the placeholder. | Weak, leaked, or mismatched values can expose diary data or prevent startup. |
| `NIKKI_DATABASE_URL` | Backend PostgreSQL connection string. | Use the production DB values and `postgres:5432` inside Compose, with `sslmode=disable` for the private Docker network. | Local fallback in code is `postgres://nikki:nikki@localhost:5432/nikki?sslmode=disable`. | Wrong host, database, user, or password can connect to the wrong data set or break the app. |

## Authentication and Browser Access

| Variable | Purpose | Production expectation | Safe default where known | Misconfiguration risk |
| --- | --- | --- | --- | --- |
| `NIKKI_COOKIE_SECURE` | Marks session cookies as HTTPS-only. | Set `true` for HTTPS production. | Code default is `false` for local development. | `false` on public HTTPS weakens session-cookie protection. |
| `NIKKI_CORS_ALLOWED_ORIGINS` | Comma-separated browser origins allowed for cross-origin API requests. | Use exact HTTPS origins only when cross-origin access is needed; do not use `*`. | Local development origins are allowed by code when unset. | Broad origins can allow unintended browser contexts to interact with the API. |
| `NIKKI_SIGNUP_ENABLED` | Allows additional signups after the first user exists. | Set `false` for public production unless intentionally opening registration. | Code default is `false`. | `true` can allow unwanted account creation on a public instance. |
| `NIKKI_FIRST_USER_SETUP_ENABLED` | Legacy first-user setup switch retained for configuration compatibility. It does not remove the setup-token requirement. | Keep `false` for public production. | Code default is `false`. | Treating this as a security boundary can create operator confusion; the token is the required gate. |
| `NIKKI_FIRST_USER_BOOTSTRAP_TOKEN` | Secret token accepted by `/setup` first-owner creation, `/setup` operational backup restore, and the legacy `X-Nikki-Bootstrap-Token` first-user path on an empty database. | Set a long random secret before exposing an empty database. | Empty by code default; production must replace the placeholder. | Missing, weak, or leaked tokens can block safe bootstrap or let an attacker create the owner account or restore data into an empty instance. |

When `NIKKI_SIGNUP_ENABLED=false`, first-owner creation is still possible only on an empty database with a matching `NIKKI_FIRST_USER_BOOTSTRAP_TOKEN`. The browser setup screen at `/setup` is shown only while the users table is empty. It can create the first user with the `owner` role or restore a Nikki operational backup archive, and both actions require the token. Owner creation or restore locks setup after success. If a user already exists, setup execution returns a conflict and additional signup stays closed unless the operator enables it.

Do not paste real setup tokens, passwords, cookies, `.env.production`, database URLs, or operational backup archives into tickets, chat, screenshots, or shared logs. If an empty instance must be reachable from the internet before setup is complete, prefer temporarily restricting access with a Security Group, VPN, SSM port forwarding, Tailscale, or another operator-only path.

## Images and Uploads

| Variable | Purpose | Production expectation | Safe default where known | Misconfiguration risk |
| --- | --- | --- | --- | --- |
| `NIKKI_STRIP_IMAGE_METADATA` | Removes metadata from uploaded images when supported. | Set `true` for production. | Code default is `false`; `.env.production.example` sets `true`. | `false` may preserve sensitive camera, device, or location metadata in uploads. |
| `NIKKI_IMAGE_USER_QUOTA_BYTES` | Per-user image storage byte quota. | Keep a finite quota unless an operator has planned storage monitoring. | Code default is 1 GiB. `0` disables the per-user byte quota. | Too high or disabled can allow unexpected disk growth; too low can block legitimate uploads. |
| `NIKKI_IMAGE_USER_QUOTA_COUNT` | Per-user image count quota. | Keep a finite quota unless an operator has planned storage monitoring. | Code default is 1,000 images. `0` disables the per-user count quota. | Too high or disabled can allow excessive metadata and file growth; too low can block legitimate uploads. |
| `NIKKI_IMAGE_TOTAL_QUOTA_BYTES` | Optional total image storage byte quota across all users. | Set when the instance needs a global disk guardrail. | Code default is `0`, disabled. | Disabled or oversized values can allow the uploads volume to fill the host; too low can block uploads for all users. |

Uploads are user-owned diary data. Do not mount the uploads volume into a public webroot. Requests for `/api/images/<id>/content` and legacy `/uploads/<name>` must go through the backend path that performs metadata lookup and ownership verification.

## Proxy and Client IP Handling

| Variable | Purpose | Production expectation | Safe default where known | Misconfiguration risk |
| --- | --- | --- | --- | --- |
| `NIKKI_IP_EXTRACTOR_MODE` | Selects how the backend derives the client IP for rate limiting, request logs, and audit events. | Use `direct` for the initial Caddy-to-frontend-to-backend chain. Use proxy-header modes only with verified trusted proxy CIDRs. | Code default is `direct`. | Trusting unverified proxy headers can let clients spoof IPs and weaken rate limiting or log integrity. |
| `NIKKI_TRUSTED_PROXY_CIDRS` | Comma-separated CIDRs allowed to supply trusted proxy headers. | Leave empty with `direct`; otherwise set only the verified frontend/proxy network CIDR. | Empty by default. | Broad public CIDRs or guessed Docker ranges can make IP spoofing possible. |

For the single-host deployment documented in this repository, Caddy should proxy only to the frontend container endpoint. Frontend nginx then proxies `/api/` and `/uploads/` to the backend according to `frontend/nginx/default.conf`.

The auth rate limiter covers login, signup, `/api/setup/owner`, and setup restore verification/execution. Rate-limit responses intentionally do not reveal setup-token configuration or token validity.

## Logging and Audit History

| Variable | Purpose | Production expectation | Safe default where known | Misconfiguration risk |
| --- | --- | --- | --- | --- |
| `NIKKI_LOG_FORMAT` | Backend stdout log format. | Use `json` in containers. | Code default is `json`. | Text logs are harder to search after incidents. |
| `NIKKI_LOG_LEVEL` | Backend stdout log level. | Keep `info` unless diagnosing a specific issue. | Code default is `info`. | Too noisy can hide important events; too quiet can obscure failures. |
| `NIKKI_AUDIT_RETENTION_DAYS` | Retention window for database-backed security history. | Keep a bounded positive value such as `180`. | Code and production example default to `180`. | Too short can erase useful investigation history; too long retains operational metadata longer than needed. |

Audit history is stored in `audit_events` and is visible only to owner accounts. It records authentication successes/failures, CSRF failures, password changes, account deletion attempts, setup restore events, exports, entry deletion, and image deletion. Audit remote IP extraction uses `NIKKI_IP_EXTRACTOR_MODE` and `NIKKI_TRUSTED_PROXY_CIDRS`, the same trust boundary as auth rate limiting. It must not contain diary title/body/tags, passwords, cookies, CSRF tokens, session tokens, request bodies, upload file paths, or SQL arguments. Rate-limit denials are structured stdout logs only, not persisted audit rows, to avoid database write amplification during attacks.
