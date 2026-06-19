#!/usr/bin/env sh
set -eu

ENV_FILE="${NIKKI_SMOKE_ENV_FILE:-.env.production}"
BASE_URL="${NIKKI_SMOKE_BASE_URL:-}"
USERNAME="${NIKKI_SMOKE_USERNAME:-}"
PASSWORD="${NIKKI_SMOKE_PASSWORD:-}"
NEW_PASSWORD="${NIKKI_SMOKE_NEW_PASSWORD:-}"
ENTRY_DATE="${NIKKI_SMOKE_ENTRY_DATE:-2099-12-31}"
ALLOW_HTTP="${NIKKI_SMOKE_ALLOW_HTTP:-false}"
CHECK_COMPOSE="${NIKKI_SMOKE_CHECK_COMPOSE:-true}"
EXPECT_SIGNUP_CLOSED="${NIKKI_SMOKE_EXPECT_SIGNUP_CLOSED:-true}"
EXPECT_SETUP_LOCKED="${NIKKI_SMOKE_EXPECT_SETUP_LOCKED:-true}"
RUN_PASSWORD_CHANGE="${NIKKI_SMOKE_RUN_PASSWORD_CHANGE:-false}"
RUN_BACKUP="${NIKKI_SMOKE_RUN_BACKUP:-false}"
KEEP_ARTIFACTS="${NIKKI_SMOKE_KEEP_ARTIFACTS:-false}"
TIMEOUT_SECONDS="${NIKKI_SMOKE_TIMEOUT_SECONDS:-20}"

WORK_DIR=""
COOKIE_JAR=""
CSRF_TOKEN=""
ENTRY_ID=""
IMAGE_ID=""
PASSWORD_STATE="original"

usage() {
  cat <<'EOF'
Run a production smoke test against a deployed Nikki instance.

Required environment:
  NIKKI_SMOKE_BASE_URL      Public app origin, for example https://diary.example.com
  NIKKI_SMOKE_USERNAME      Existing owner username for smoke verification
  NIKKI_SMOKE_PASSWORD      Existing owner password for smoke verification

Common optional environment:
  NIKKI_SMOKE_ENV_FILE              Compose env file, default .env.production
  NIKKI_SMOKE_ENTRY_DATE            Disposable entry date, default 2099-12-31
  NIKKI_SMOKE_CHECK_COMPOSE         Run production Compose config check, default true
  NIKKI_SMOKE_RUN_BACKUP            Run scripts/backup-production.sh, default false
  NIKKI_SMOKE_RUN_PASSWORD_CHANGE   Run reversible password-change smoke, default false
  NIKKI_SMOKE_NEW_PASSWORD          Temporary password required when password-change smoke is enabled
  NIKKI_SMOKE_ALLOW_HTTP            Allow http:// base URLs, default false
  NIKKI_SMOKE_KEEP_ARTIFACTS        Keep temporary response/cookie files, default false

The script creates one disposable diary entry and one tiny image, verifies the
main authenticated flows, and deletes the created smoke data before exit.
EOF
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

fail() {
  printf '%s\n' "ERROR: $1" >&2
  exit 1
}

info() {
  printf '%s\n' "==> $1"
}

pass() {
  printf '%s\n' "OK: $1"
}

skip() {
  printf '%s\n' "SKIP: $1"
}

normalize_bool() {
  name="$1"
  value="$2"
  case "${value}" in
    true | 1 | yes | on) printf '%s\n' "true" ;;
    false | 0 | no | off | "") printf '%s\n' "false" ;;
    *) fail "${name} must be true or false" ;;
  esac
}

ALLOW_HTTP="$(normalize_bool NIKKI_SMOKE_ALLOW_HTTP "${ALLOW_HTTP}")"
CHECK_COMPOSE="$(normalize_bool NIKKI_SMOKE_CHECK_COMPOSE "${CHECK_COMPOSE}")"
EXPECT_SIGNUP_CLOSED="$(normalize_bool NIKKI_SMOKE_EXPECT_SIGNUP_CLOSED "${EXPECT_SIGNUP_CLOSED}")"
EXPECT_SETUP_LOCKED="$(normalize_bool NIKKI_SMOKE_EXPECT_SETUP_LOCKED "${EXPECT_SETUP_LOCKED}")"
RUN_PASSWORD_CHANGE="$(normalize_bool NIKKI_SMOKE_RUN_PASSWORD_CHANGE "${RUN_PASSWORD_CHANGE}")"
RUN_BACKUP="$(normalize_bool NIKKI_SMOKE_RUN_BACKUP "${RUN_BACKUP}")"
KEEP_ARTIFACTS="$(normalize_bool NIKKI_SMOKE_KEEP_ARTIFACTS "${KEEP_ARTIFACTS}")"

[ -n "${BASE_URL}" ] || fail "NIKKI_SMOKE_BASE_URL is required"
[ -n "${USERNAME}" ] || fail "NIKKI_SMOKE_USERNAME is required"
[ -n "${PASSWORD}" ] || fail "NIKKI_SMOKE_PASSWORD is required"

BASE_URL="${BASE_URL%/}"
case "${BASE_URL}" in
  https://*) ;;
  http://*)
    [ "${ALLOW_HTTP}" = "true" ] || fail "NIKKI_SMOKE_BASE_URL must use https:// unless NIKKI_SMOKE_ALLOW_HTTP=true"
    ;;
  *) fail "NIKKI_SMOKE_BASE_URL must start with https:// or http://" ;;
esac

case "${TIMEOUT_SECONDS}" in
  "" | *[!0-9]*) fail "NIKKI_SMOKE_TIMEOUT_SECONDS must be a positive integer" ;;
  0) fail "NIKKI_SMOKE_TIMEOUT_SECONDS must be a positive integer" ;;
esac

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command is missing: $1"
}

require_command curl
require_command python3
if [ "${CHECK_COMPOSE}" = "true" ] || [ "${RUN_BACKUP}" = "true" ]; then
  require_command docker
fi

umask 077
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/nikki-smoke.XXXXXX")"
COOKIE_JAR="${WORK_DIR}/cookies.txt"
: > "${COOKIE_JAR}"

print_response_excerpt() {
  file="$1"
  if [ -s "${file}" ]; then
    printf '%s\n' "Response body excerpt:" >&2
    sed -n '1,8p' "${file}" >&2
  fi
}

expect_http() {
  label="$1"
  expected="$2"
  method="$3"
  path="$4"
  output="$5"
  shift 5

  if ! status="$(
    curl -sS \
      --connect-timeout "${TIMEOUT_SECONDS}" \
      --max-time "${TIMEOUT_SECONDS}" \
      -o "${output}" \
      -w "%{http_code}" \
      -X "${method}" \
      "$@" \
      "${BASE_URL}${path}"
  )"; then
    fail "${label} request failed"
  fi

  if [ "${status}" != "${expected}" ]; then
    printf '%s\n' "ERROR: ${label} returned HTTP ${status}, expected ${expected}" >&2
    print_response_excerpt "${output}"
    exit 1
  fi

  pass "${label}"
}

json_path() {
  file="$1"
  path="$2"
  python3 - "$file" "$path" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    value = json.load(handle)

for part in sys.argv[2].split("."):
    if part.isdigit():
        value = value[int(part)]
    else:
        value = value[part]

if isinstance(value, bool):
    print("true" if value else "false")
elif value is None:
    print("")
else:
    print(value)
PY
}

write_credentials_json() {
  output="$1"
  username="$2"
  password="$3"
  NIKKI_JSON_USERNAME="${username}" NIKKI_JSON_PASSWORD="${password}" python3 - "$output" <<'PY'
import json
import os
import sys

with open(sys.argv[1], "w", encoding="utf-8") as handle:
    json.dump(
        {
            "username": os.environ["NIKKI_JSON_USERNAME"],
            "password": os.environ["NIKKI_JSON_PASSWORD"],
        },
        handle,
        separators=(",", ":"),
    )
PY
}

write_password_json() {
  output="$1"
  current_password="$2"
  new_password="$3"
  NIKKI_JSON_CURRENT_PASSWORD="${current_password}" NIKKI_JSON_NEW_PASSWORD="${new_password}" python3 - "$output" <<'PY'
import json
import os
import sys

with open(sys.argv[1], "w", encoding="utf-8") as handle:
    json.dump(
        {
            "currentPassword": os.environ["NIKKI_JSON_CURRENT_PASSWORD"],
            "newPassword": os.environ["NIKKI_JSON_NEW_PASSWORD"],
        },
        handle,
        separators=(",", ":"),
    )
PY
}

write_entry_json() {
  output="$1"
  entry_date="$2"
  title="$3"
  body="$4"
  version="${5:-}"
  NIKKI_JSON_ENTRY_DATE="${entry_date}" \
    NIKKI_JSON_TITLE="${title}" \
    NIKKI_JSON_BODY="${body}" \
    NIKKI_JSON_VERSION="${version}" \
    python3 - "$output" <<'PY'
import json
import os
import sys

payload = {
    "entryDate": os.environ["NIKKI_JSON_ENTRY_DATE"],
    "title": os.environ["NIKKI_JSON_TITLE"],
    "body": os.environ["NIKKI_JSON_BODY"],
    "mood": "calm",
    "tags": ["smoke-test"],
}
version = os.environ.get("NIKKI_JSON_VERSION", "")
if version:
    payload["expectedVersion"] = int(version)

with open(sys.argv[1], "w", encoding="utf-8") as handle:
    json.dump(payload, handle, separators=(",", ":"))
PY
}

write_smoke_png() {
  output="$1"
  python3 - "$output" <<'PY'
import base64
import sys

png = (
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8"
    "/x8AAwMCAO+/p9sAAAAASUVORK5CYII="
)
with open(sys.argv[1], "wb") as handle:
    handle.write(base64.b64decode(png))
PY
}

login_as() {
  password="$1"
  label="$2"
  body="${WORK_DIR}/${label}-login.json"
  output="${WORK_DIR}/${label}-login-response.json"
  write_credentials_json "${body}" "${USERNAME}" "${password}"
  expect_http "${label} login" 200 POST "/api/auth/login" "${output}" \
    -b "${COOKIE_JAR}" \
    -c "${COOKIE_JAR}" \
    -H "Accept: application/json" \
    -H "Content-Type: application/json" \
    --data-binary "@${body}"
  CSRF_TOKEN="$(json_path "${output}" "csrfToken")"
  [ -n "${CSRF_TOKEN}" ] || fail "${label} login response did not include csrfToken"
  role="$(json_path "${output}" "role")"
  [ "${role}" = "owner" ] || fail "smoke user must be an owner to verify security history"
}

restore_password_if_needed() {
  [ "${PASSWORD_STATE}" = "temporary" ] || return 0
  printf '%s\n' "WARN: attempting to restore smoke password before exit" >&2
  set +e

  restore_cookie="${WORK_DIR}/restore-cookies.txt"
  restore_login_body="${WORK_DIR}/restore-login.json"
  restore_login_response="${WORK_DIR}/restore-login-response.json"
  restore_password_body="${WORK_DIR}/restore-password.json"
  restore_response="${WORK_DIR}/restore-password-response.json"
  write_credentials_json "${restore_login_body}" "${USERNAME}" "${NEW_PASSWORD}"
  login_status="$(curl -sS --connect-timeout "${TIMEOUT_SECONDS}" --max-time "${TIMEOUT_SECONDS}" -o "${restore_login_response}" -w "%{http_code}" -X POST -b "${restore_cookie}" -c "${restore_cookie}" -H "Content-Type: application/json" --data-binary "@${restore_login_body}" "${BASE_URL}/api/auth/login")"
  if [ "${login_status}" = "200" ]; then
    restore_csrf="$(json_path "${restore_login_response}" "csrfToken" 2>/dev/null)"
    write_password_json "${restore_password_body}" "${NEW_PASSWORD}" "${PASSWORD}"
    restore_status="$(curl -sS --connect-timeout "${TIMEOUT_SECONDS}" --max-time "${TIMEOUT_SECONDS}" -o "${restore_response}" -w "%{http_code}" -X PUT -b "${restore_cookie}" -c "${restore_cookie}" -H "Content-Type: application/json" -H "X-CSRF-Token: ${restore_csrf}" --data-binary "@${restore_password_body}" "${BASE_URL}/api/auth/me/password")"
    if [ "${restore_status}" = "204" ]; then
      PASSWORD_STATE="original"
      printf '%s\n' "WARN: smoke password was restored" >&2
    else
      printf '%s\n' "WARN: password restore returned HTTP ${restore_status}; the smoke password may still be NIKKI_SMOKE_NEW_PASSWORD" >&2
    fi
  else
    printf '%s\n' "WARN: could not log in with NIKKI_SMOKE_NEW_PASSWORD during cleanup; password state is uncertain" >&2
  fi

  set -e
}

cleanup_smoke_data() {
  set +e
  if [ -n "${ENTRY_ID}" ] && [ -n "${CSRF_TOKEN}" ]; then
    curl -sS \
      --connect-timeout "${TIMEOUT_SECONDS}" \
      --max-time "${TIMEOUT_SECONDS}" \
      -o /dev/null \
      -w "%{http_code}" \
      -X DELETE \
      -b "${COOKIE_JAR}" \
      -c "${COOKIE_JAR}" \
      -H "X-CSRF-Token: ${CSRF_TOKEN}" \
      "${BASE_URL}/api/entries/${ENTRY_ID}" >/dev/null 2>&1
  fi
  set -e
}

cleanup() {
  status=$?
  restore_password_if_needed
  cleanup_smoke_data
  if [ "${KEEP_ARTIFACTS}" = "true" ]; then
    printf '%s\n' "Temporary smoke artifacts kept at: ${WORK_DIR}"
  else
    rm -rf "${WORK_DIR}"
  fi
  exit "${status}"
}
trap cleanup EXIT INT TERM

if [ "${CHECK_COMPOSE}" = "true" ]; then
  info "checking production Compose configuration"
  config_output="${WORK_DIR}/compose-config.yml"
  docker compose --env-file "${ENV_FILE}" -f docker-compose.yml -f docker-compose.prod.yml config > "${config_output}"
  sh ./scripts/check-production-config.sh < "${config_output}" >/dev/null
  pass "production Compose config"
else
  skip "production Compose config"
fi

info "checking public HTTP endpoints"
expect_http "health endpoint" 200 GET "/api/health" "${WORK_DIR}/health.json" \
  -H "Accept: application/json"

expect_http "auth config" 200 GET "/api/auth/config" "${WORK_DIR}/auth-config.json" \
  -H "Accept: application/json"
if [ "${EXPECT_SIGNUP_CLOSED}" = "true" ]; then
  signup_available="$(json_path "${WORK_DIR}/auth-config.json" "signupAvailable")"
  signup_mode="$(json_path "${WORK_DIR}/auth-config.json" "signupMode")"
  [ "${signup_available}" = "false" ] || fail "signupAvailable must be false in production smoke"
  [ "${signup_mode}" = "closed" ] || fail "signupMode must be closed in production smoke"
  pass "signup is closed"
else
  skip "signup closed assertion"
fi

expect_http "setup status" 200 GET "/api/setup/status" "${WORK_DIR}/setup-status.json" \
  -H "Accept: application/json"
if [ "${EXPECT_SETUP_LOCKED}" = "true" ]; then
  needs_setup="$(json_path "${WORK_DIR}/setup-status.json" "needsSetup")"
  setup_locked="$(json_path "${WORK_DIR}/setup-status.json" "setupLocked")"
  [ "${needs_setup}" = "false" ] || fail "needsSetup must be false after production setup"
  [ "${setup_locked}" = "true" ] || fail "setupLocked must be true after production setup"
  pass "setup is locked"
else
  skip "setup locked assertion"
fi

expect_http "unauthenticated entries are rejected" 401 GET "/api/entries" "${WORK_DIR}/entries-unauth.json" \
  -H "Accept: application/json"

info "checking authenticated account and CSRF flows"
login_as "${PASSWORD}" "owner"

expect_http "auth me" 200 GET "/api/auth/me" "${WORK_DIR}/me.json" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "Accept: application/json"
CSRF_TOKEN="$(json_path "${WORK_DIR}/me.json" "csrfToken")"
[ -n "${CSRF_TOKEN}" ] || fail "/api/auth/me response did not include csrfToken"

expect_http "authenticated mutation without CSRF is rejected" 403 POST "/api/auth/logout" "${WORK_DIR}/csrf-response.json" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "Accept: application/json"

if [ "${RUN_PASSWORD_CHANGE}" = "true" ]; then
  [ -n "${NEW_PASSWORD}" ] || fail "NIKKI_SMOKE_NEW_PASSWORD is required when NIKKI_SMOKE_RUN_PASSWORD_CHANGE=true"
  [ "${NEW_PASSWORD}" != "${PASSWORD}" ] || fail "NIKKI_SMOKE_NEW_PASSWORD must differ from NIKKI_SMOKE_PASSWORD"
  info "checking reversible password change"
  change_body="${WORK_DIR}/password-change.json"
  write_password_json "${change_body}" "${PASSWORD}" "${NEW_PASSWORD}"
  expect_http "password change to temporary password" 204 PUT "/api/auth/me/password" "${WORK_DIR}/password-change-response.txt" \
    -b "${COOKIE_JAR}" \
    -c "${COOKIE_JAR}" \
    -H "Accept: application/json" \
    -H "Content-Type: application/json" \
    -H "X-CSRF-Token: ${CSRF_TOKEN}" \
    --data-binary "@${change_body}"
  PASSWORD_STATE="temporary"
  expect_http "old session is revoked after password change" 401 GET "/api/auth/me" "${WORK_DIR}/me-after-password-change.json" \
    -b "${COOKIE_JAR}" \
    -c "${COOKIE_JAR}" \
    -H "Accept: application/json"
  : > "${COOKIE_JAR}"
  login_as "${NEW_PASSWORD}" "temporary-password"
  revert_body="${WORK_DIR}/password-revert.json"
  write_password_json "${revert_body}" "${NEW_PASSWORD}" "${PASSWORD}"
  expect_http "password change back to original password" 204 PUT "/api/auth/me/password" "${WORK_DIR}/password-revert-response.txt" \
    -b "${COOKIE_JAR}" \
    -c "${COOKIE_JAR}" \
    -H "Accept: application/json" \
    -H "Content-Type: application/json" \
    -H "X-CSRF-Token: ${CSRF_TOKEN}" \
    --data-binary "@${revert_body}"
  PASSWORD_STATE="original"
  expect_http "temporary session is revoked after password restore" 401 GET "/api/auth/me" "${WORK_DIR}/me-after-password-restore.json" \
    -b "${COOKIE_JAR}" \
    -c "${COOKIE_JAR}" \
    -H "Accept: application/json"
  : > "${COOKIE_JAR}"
  login_as "${PASSWORD}" "restored-password"
else
  skip "password change smoke"
fi

info "checking diary, image, search, and audit flows"
expect_http "disposable entry date lookup" 200 GET "/api/entries/date/${ENTRY_DATE}" "${WORK_DIR}/entry-date.json" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "Accept: application/json"
date_exists="$(json_path "${WORK_DIR}/entry-date.json" "exists")"
[ "${date_exists}" = "false" ] || fail "NIKKI_SMOKE_ENTRY_DATE already exists; choose an unused date"

entry_body="${WORK_DIR}/entry-create.json"
write_entry_json "${entry_body}" "${ENTRY_DATE}" "Nikki smoke test" "Disposable production smoke test entry."
expect_http "entry create" 201 POST "/api/entries" "${WORK_DIR}/entry-create-response.json" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "Accept: application/json" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}" \
  --data-binary "@${entry_body}"
ENTRY_ID="$(json_path "${WORK_DIR}/entry-create-response.json" "id")"
entry_version="$(json_path "${WORK_DIR}/entry-create-response.json" "version")"
[ -n "${ENTRY_ID}" ] || fail "entry create response did not include id"

expect_http "entry read" 200 GET "/api/entries/${ENTRY_ID}" "${WORK_DIR}/entry-read.json" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "Accept: application/json"

entry_update_body="${WORK_DIR}/entry-update.json"
write_entry_json "${entry_update_body}" "${ENTRY_DATE}" "Nikki smoke test updated" "Disposable production smoke test entry, updated." "${entry_version}"
expect_http "entry update" 200 PUT "/api/entries/${ENTRY_ID}" "${WORK_DIR}/entry-update-response.json" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "Accept: application/json" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}" \
  --data-binary "@${entry_update_body}"

expect_http "entry search" 200 GET "/api/entries/search?q=smoke%20test&limit=5" "${WORK_DIR}/entry-search.json" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "Accept: application/json"

expect_http "memory shelf endpoint" 200 GET "/api/entries/memories?date=${ENTRY_DATE}&limit=1" "${WORK_DIR}/memories.json" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "Accept: application/json"

png_file="${WORK_DIR}/smoke.png"
write_smoke_png "${png_file}"
expect_http "image upload" 201 POST "/api/entries/${ENTRY_ID}/images" "${WORK_DIR}/image-upload-response.json" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "Accept: application/json" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}" \
  -F "images=@${png_file};type=image/png"
IMAGE_ID="$(json_path "${WORK_DIR}/image-upload-response.json" "0.id")"
[ -n "${IMAGE_ID}" ] || fail "image upload response did not include id"

expect_http "authenticated image content" 200 GET "/api/images/${IMAGE_ID}/content" "${WORK_DIR}/image-content.png" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}"
[ -s "${WORK_DIR}/image-content.png" ] || fail "authenticated image content response was empty"

expect_http "unauthenticated image content is rejected" 401 GET "/api/images/${IMAGE_ID}/content" "${WORK_DIR}/image-content-unauth.txt"

expect_http "image delete" 204 DELETE "/api/images/${IMAGE_ID}" "${WORK_DIR}/image-delete-response.txt" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}"
IMAGE_ID=""

expect_http "entry delete" 204 DELETE "/api/entries/${ENTRY_ID}" "${WORK_DIR}/entry-delete-response.txt" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}"
ENTRY_ID=""

expect_http "security history" 200 GET "/api/audit/events?limit=20" "${WORK_DIR}/audit-events.json" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "Accept: application/json"

if [ "${RUN_BACKUP}" = "true" ]; then
  info "checking operational backup command"
  ENV_FILE="${ENV_FILE}" sh ./scripts/backup-production.sh
  pass "operational backup command"
else
  skip "operational backup command"
fi

expect_http "logout" 204 POST "/api/auth/logout" "${WORK_DIR}/logout-response.txt" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "X-CSRF-Token: ${CSRF_TOKEN}"

expect_http "session is cleared after logout" 401 GET "/api/auth/me" "${WORK_DIR}/me-after-logout.json" \
  -b "${COOKIE_JAR}" \
  -c "${COOKIE_JAR}" \
  -H "Accept: application/json"

printf '%s\n' "Production smoke test passed"
