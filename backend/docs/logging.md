# Backend Logging Design

## Goals

- Make backend behavior observable from Docker logs without adding an external logging service.
- Use structured logs so request, user, route, and error context can be searched later.
- Persist a bounded security history for events that must be reviewed after Docker logs have rotated.
- Keep private diary data out of logs.
- Add logging through shared HTTP and app boundaries first, instead of scattering ad-hoc prints through handlers.

## Logger

- Use the standard library `log/slog`.
- Write to stdout. Docker Compose can collect it with `docker compose logs -f backend`.
- Default level: `info`.
- Default format: JSON in containers, text is acceptable for local development.
- Configure with environment variables:
  - `NIKKI_LOG_LEVEL`: `debug`, `info`, `warn`, `error`.
  - `NIKKI_LOG_FORMAT`: `json`, `text`.

Example startup fields:

```json
{"level":"INFO","msg":"server starting","addr":":8080","upload_dir":"/uploads"}
{"level":"INFO","msg":"database connected"}
{"level":"INFO","msg":"database migrated"}
```

## Request Logging

Add one Echo middleware near server creation, after request id setup and before routes.

Log exactly one record per request after the handler finishes.

Fields:

- `request_id`: from `X-Request-ID` when it is a bounded printable value, generated when absent or invalid.
- `method`: HTTP method.
- `route`: Echo route pattern, for example `/api/entries/:id`.
- `status`: response status code.
- `duration_ms`: request duration in milliseconds.
- `bytes_in`: request content length when known.
- `bytes_out`: response size.
- `remote_ip`: client IP from the configured trusted-proxy extractor.
- `user_id`: authenticated user id when available.
- `error_kind`: stable app error code when available.

Levels:

- `debug`: health checks.
- `info`: successful requests and expected redirects/downloads.
- `warn`: 4xx client errors except very noisy unauthenticated checks if they become spammy.
- `error`: 5xx responses and panics.

Do not log raw URL query values by default. Route and status are usually enough, and search terms/tags may contain personal diary content.

## Error Logging

Keep client responses and internal logs separate.

Current handlers call `httpx.Error(c, status, err.Error())`, including for some 500 errors. Replace that pattern in two steps:

1. Keep `httpx.Error(c, status, publicMessage)` for expected 4xx responses.
2. Add `httpx.Internal(c, err)` for unexpected failures.

`httpx.Internal` should:

- Set response status to 500.
- Return a generic client message.
- Store the original error on the Echo context for request logging.
- Avoid logging secrets, request bodies, cookies, passwords, tokens, diary body, tags, or uploaded file paths.

For app errors, attach a stable `error_kind` instead of relying on localized messages:

| Area | Error | Status | `error_kind` |
| --- | --- | ---: | --- |
| auth | invalid credentials | 401 | `auth.invalid_credentials` |
| auth | unauthorized | 401 | `auth.unauthorized` |
| auth | username exists | 409 | `auth.username_exists` |
| entries | invalid input | 400 | `entries.invalid_input` |
| entries | not found | 404 | `entries.not_found` |
| entries | date exists | 409 | `entries.date_exists` |
| images | invalid image | 400 | `images.invalid_image` |
| images | too many images | 400 | `images.too_many` |
| images | not found | 404 | `images.not_found` |
| exporter | unsupported format | 400 | `export.unsupported_format` |

## Panic Recovery

Use Echo recover middleware or a small custom middleware that:

- Converts panics to 500 responses.
- Logs `request_id`, `route`, `method`, and stack trace.
- Does not include request body or cookies.

Panic records must be `error` level.

## Domain Events

Add event logs only where they answer an operational question that request logs cannot answer.

Recommended events:

- `auth.signup_succeeded`: `user_id`.
- `auth.login_succeeded`: `user_id`.
- `auth.logout_succeeded`: `user_id` when known.
- `entries.created`: `user_id`, `entry_id`, `entry_date`.
- `entries.updated`: `user_id`, `entry_id`, `entry_date`.
- `entries.deleted`: `user_id`, `entry_id`.
- `images.uploaded`: `user_id`, `entry_id`, `count`.
- `images.deleted`: `user_id`, `image_id`.
- `export.completed`: `user_id`, `format`, `bytes_out`.

Avoid logging entry title, body, markdown, tags, mood notes, image original filename, generated storage path, passwords, session tokens, cookies, and full SQL arguments.

## Persistent Audit Events

`audit_events` stores the subset of events that need later investigation from the owner Settings screen:

- authentication success/failure, logout, password change, account deletion, and CSRF failure
- setup owner creation and setup restore verification/completion/failure
- export completion, entry deletion, and image deletion

Audit rows include event type, outcome, actor id/name/role when known, target type/id, reason code, request id, remote IP, small operational metadata, and creation time. Audit remote IP extraction follows the same trusted-proxy configuration used by auth rate limiting. Rows must not include diary title/body/markdown/tags, passwords, cookies, CSRF tokens, session tokens, request bodies, generated upload paths, or raw SQL arguments.

Rate-limit denials are written to structured stdout logs but are intentionally not persisted to `audit_events`. This avoids turning an active brute-force or spray attempt into unbounded database write amplification; the authentication failures before lockout remain persisted.

`NIKKI_AUDIT_RETENTION_DAYS` controls retention and defaults to 180 days. The backend prunes old audit rows on startup after schema migration.

## Database and Storage Failures

Do not log every SQL query by default. Start with unexpected error logs at the request boundary, then add targeted logs only when diagnosing a recurring issue.

Useful operation fields for unexpected failures:

- `component`: `db`, `storage`, `auth`, `entries`, `images`, `exporter`.
- `operation`: `connect`, `migrate`, `entry.create`, `image.save`, etc.
- `error`: original error string for server logs only.

Slow requests should be logged at `warn` when `duration_ms >= 1000`. If DB latency becomes a problem, add per-repository slow operation logging later with query names, not raw SQL with arguments.

## Implementation Shape

Add package `internal/logx`:

- `New(cfg Config) *slog.Logger`
- `ParseLevel(value string) slog.Level`
- `RequestID(c echo.Context) string`
- `Middleware(logger *slog.Logger) echo.MiddlewareFunc`
- `Recover(logger *slog.Logger) echo.MiddlewareFunc`
- `SetError(c echo.Context, kind string, err error)`
- `ErrorAttrs(c echo.Context) []slog.Attr`

Update `internal/app/config.go`:

- Add `LogLevel string`.
- Add `LogFormat string`.

Update `internal/app/server.go`:

- Create the logger at startup.
- Replace `log.Printf` with `logger.Info`.
- Add request id, request logging, and recover middleware before CORS/routes.
- Log startup steps: database connect, migration, server start.

Update `cmd/server/main.go`:

- Use `slog` for fatal startup errors.

Update `internal/httpx`:

- Keep current response helpers.
- Add error metadata helpers.
- Add an internal error helper for 500 responses.

## Tests

Add focused tests for:

- Log level parsing.
- Request middleware logs route, status, request id, duration, and user id when present.
- 500 responses include original error in logs but not in response body.
- Sensitive values are absent from logs for auth and entry requests.

## Rollout

1. Add `slog` logger, config, request id, request logging, and recover middleware.
2. Convert 500 paths to `httpx.Internal`.
3. Add stable `error_kind` metadata for known app errors.
4. Add domain event logs for auth, entries, images, and export.
5. Persist the minimal owner-visible audit event subset with bounded retention.
6. Tune noisy 4xx logs after observing real Docker logs.
