#!/usr/bin/env sh
set -eu

ENV_FILE="${ENV_FILE:-.env.production}"

if [ -t 0 ]; then
  config="$(docker compose --env-file "${ENV_FILE}" -f docker-compose.yml -f docker-compose.prod.yml config)"
else
  config="$(cat)"
fi

fail() {
  printf '%s\n' "production config check failed: $1" >&2
  exit 1
}

contains() {
  printf '%s\n' "${config}" | grep -Fq -- "$1"
}

contains "CHANGE_ME" && fail "placeholder CHANGE_ME is present"
contains "replace-with-your-domain.example" && fail "placeholder domain is present"
contains "replace-with-long-random-bootstrap-token" && fail "bootstrap token placeholder is present"
contains "nikki:nikki" && fail "default development database credentials are present"
contains "POSTGRES_PASSWORD=nikki" && fail "default development POSTGRES_PASSWORD is present"
contains "8080:8080" && fail "backend development port mapping is present"
contains "published: \"8080\"" && fail "backend port 8080 is published"
contains "published: 8080" && fail "backend port 8080 is published"

service_block() {
  service="$1"
  printf '%s\n' "${config}" | awk -v service="${service}" '
    $0 == "  " service ":" { in_service=1; print; next }
    /^  [a-zA-Z0-9_-]+:/ && in_service { in_service=0 }
    in_service { print }
  '
}

env_value() {
  block="$1"
  key="$2"
  printf '%s\n' "${block}" | awk -v key="${key}" '
    $1 == key ":" {
      sub("^[[:space:]]*" key ":[[:space:]]*", "")
      gsub(/^"|"$/, "")
      print
      exit
    }
  '
}

backend_block="$(printf '%s\n' "${config}" | awk '
  /^  backend:/ { in_backend=1; print; next }
  /^  [a-zA-Z0-9_-]+:/ && in_backend { in_backend=0 }
  in_backend { print }
')"
postgres_block="$(service_block postgres)"
frontend_block="$(service_block frontend)"

printf '%s\n' "${backend_block}" | grep -Eq '^    ports:' && fail "backend service has published ports"
printf '%s\n' "${backend_block}" | grep -Eq '^    expose:' || fail "backend service does not expose its Docker-network port"
printf '%s\n' "${backend_block}" | grep -Fq -- '--spider' && fail "backend healthcheck uses HEAD/spider instead of GET"
printf '%s\n' "${backend_block}" | grep -Fq -- 'wget -q -O - http://127.0.0.1:8080/api/health' || fail "backend healthcheck does not use GET /api/health"
printf '%s\n' "${postgres_block}" | grep -Eq '^    ports:' && fail "postgres service has published ports"
printf '%s\n' "${frontend_block}" | grep -Eq 'host_ip: 127\.0\.0\.1' || fail "frontend is not bound to 127.0.0.1"

[ "$(env_value "${backend_block}" NIKKI_COOKIE_SECURE)" = "true" ] || fail "NIKKI_COOKIE_SECURE must be true"
[ "$(env_value "${backend_block}" NIKKI_SIGNUP_ENABLED)" = "false" ] || fail "NIKKI_SIGNUP_ENABLED must be false"
[ "$(env_value "${backend_block}" NIKKI_FIRST_USER_SETUP_ENABLED)" = "false" ] || fail "NIKKI_FIRST_USER_SETUP_ENABLED must be false"
[ "$(env_value "${backend_block}" NIKKI_STRIP_IMAGE_METADATA)" = "true" ] || fail "NIKKI_STRIP_IMAGE_METADATA must be true"
[ "$(env_value "${backend_block}" NIKKI_AUTH_RATE_LIMIT_IP_ATTEMPTS)" != "" ] || fail "NIKKI_AUTH_RATE_LIMIT_IP_ATTEMPTS is missing"
[ "$(env_value "${backend_block}" NIKKI_AUTH_RATE_LIMIT_ACCOUNT_ATTEMPTS)" != "" ] || fail "NIKKI_AUTH_RATE_LIMIT_ACCOUNT_ATTEMPTS is missing"
[ "$(env_value "${backend_block}" NIKKI_AUTH_RATE_LIMIT_WINDOW)" != "" ] || fail "NIKKI_AUTH_RATE_LIMIT_WINDOW is missing"

cors_origins="$(env_value "${backend_block}" NIKKI_CORS_ALLOWED_ORIGINS)"
[ "${cors_origins}" != "" ] || fail "NIKKI_CORS_ALLOWED_ORIGINS is missing"
[ "${cors_origins}" != "*" ] || fail "NIKKI_CORS_ALLOWED_ORIGINS must not be wildcard"
printf '%s\n' "${cors_origins}" | grep -Fq ',' && fail "NIKKI_CORS_ALLOWED_ORIGINS must be one exact origin"
printf '%s\n' "${cors_origins}" | grep -Eq '^https://[^[:space:]]+$' || fail "NIKKI_CORS_ALLOWED_ORIGINS must be an exact HTTPS origin"

bootstrap_token="$(env_value "${backend_block}" NIKKI_FIRST_USER_BOOTSTRAP_TOKEN)"
[ "${bootstrap_token}" != "" ] || fail "NIKKI_FIRST_USER_BOOTSTRAP_TOKEN is missing"

printf '%s\n' "production config check passed"
