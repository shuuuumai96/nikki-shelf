#!/usr/bin/env sh
set -eu

ENV_FILE="${ENV_FILE:-.env.production}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-nikki}"
POSTGRES_DB="${POSTGRES_DB:-nikki}"
POSTGRES_USER="${POSTGRES_USER:-nikki}"
UPLOADS_VOLUME="${UPLOADS_VOLUME:-${COMPOSE_PROJECT_NAME}_nikki_uploads}"
AGE_RECIPIENT="${AGE_RECIPIENT:-}"
DELETE_PLAINTEXT_AFTER_ENCRYPT="${DELETE_PLAINTEXT_AFTER_ENCRYPT:-false}"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
run_dir="${BACKUP_DIR}/${timestamp}"
db_dump="${run_dir}/nikki-postgres-${timestamp}.sql"
uploads_archive="${run_dir}/nikki-uploads-${timestamp}.tar"
manifest="${run_dir}/manifest-${timestamp}.txt"

compose() {
  docker compose --env-file "${ENV_FILE}" -f docker-compose.yml -f docker-compose.prod.yml "$@"
}

mkdir -p "${run_dir}"
run_dir_abs="$(cd "${run_dir}" && pwd -P)"

printf '%s\n' "Creating Nikki backup ${timestamp}"
printf '%s\n' "WARNING: keep the PostgreSQL dump and uploads archive together. Both contain private diary data."

if ! docker volume inspect "${UPLOADS_VOLUME}" >/dev/null 2>&1; then
  printf '%s\n' "ERROR: uploads volume does not exist: ${UPLOADS_VOLUME}" >&2
  printf '%s\n' "Refusing to continue because Docker would otherwise create an empty volume." >&2
  exit 1
fi

compose exec -T postgres pg_dump -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" > "${db_dump}"

if [ ! -s "${db_dump}" ]; then
  printf '%s\n' "ERROR: PostgreSQL dump was not created or is empty: ${db_dump}" >&2
  exit 1
fi

MSYS_NO_PATHCONV=1 docker run --rm \
  -v "${UPLOADS_VOLUME}:/uploads:ro" \
  -v "${run_dir_abs}:/backup" \
  alpine tar -cf "/backup/nikki-uploads-${timestamp}.tar" -C /uploads .

if [ ! -s "${uploads_archive}" ]; then
  printf '%s\n' "ERROR: uploads archive was not created or is empty: ${uploads_archive}" >&2
  exit 1
fi

upload_item_count="$(tar -tf "${uploads_archive}" | wc -l | tr -d ' ')"
if [ "${upload_item_count}" -le 1 ]; then
  printf '%s\n' "WARNING: uploads archive appears empty. This is OK only if Nikki has no uploaded images yet." >&2
fi

if command -v sha256sum >/dev/null 2>&1; then
  (cd "${run_dir}" && sha256sum "$(basename "${db_dump}")" "$(basename "${uploads_archive}")" > SHA256SUMS)
fi

repo_version="$(git rev-parse --short HEAD 2>/dev/null || printf '%s' unknown)"
{
  printf 'timestamp=%s\n' "${timestamp}"
  printf 'compose_project_name=%s\n' "${COMPOSE_PROJECT_NAME}"
  printf 'repo_version=%s\n' "${repo_version}"
  printf 'database_dump=%s\n' "$(basename "${db_dump}")"
  printf 'uploads_archive=%s\n' "$(basename "${uploads_archive}")"
  printf 'checksums=SHA256SUMS\n'
  printf 'encrypted=%s\n' "$([ -n "${AGE_RECIPIENT}" ] && printf true || printf false)"
} > "${manifest}"

if [ -n "${AGE_RECIPIENT}" ]; then
  if ! command -v age >/dev/null 2>&1; then
    printf '%s\n' "ERROR: AGE_RECIPIENT is set but age is not installed." >&2
    exit 1
  fi

  age -r "${AGE_RECIPIENT}" -o "${db_dump}.age" "${db_dump}"
  age -r "${AGE_RECIPIENT}" -o "${uploads_archive}.age" "${uploads_archive}"
  age -r "${AGE_RECIPIENT}" -o "${manifest}.age" "${manifest}"
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "${run_dir}" && sha256sum "$(basename "${db_dump}").age" "$(basename "${uploads_archive}").age" "$(basename "${manifest}").age" > SHA256SUMS.age)
  fi

  if [ "${DELETE_PLAINTEXT_AFTER_ENCRYPT}" = "true" ]; then
    rm -f "${db_dump}" "${uploads_archive}" "${manifest}" "${run_dir}/SHA256SUMS"
    printf '%s\n' "Plaintext backup artifacts deleted after encryption by explicit configuration."
  fi
fi

printf '%s\n' "Backup complete:"
printf '  %s\n' "${db_dump}"
printf '  %s\n' "${uploads_archive}"
printf '  %s\n' "${manifest}"
printf '%s\n' "Restore must use both artifacts from this same timestamp."
