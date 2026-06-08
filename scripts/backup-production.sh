#!/usr/bin/env sh
set -eu

ENV_FILE="${ENV_FILE:-.env.production}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
POSTGRES_DB="${POSTGRES_DB:-nikki}"
POSTGRES_USER="${POSTGRES_USER:-nikki}"
UPLOADS_VOLUME="${UPLOADS_VOLUME:-}"
AGE_RECIPIENT="${AGE_RECIPIENT:-}"
DELETE_PLAINTEXT_AFTER_ENCRYPT="${DELETE_PLAINTEXT_AFTER_ENCRYPT:-false}"

timestamp="$(date -u +%Y%m%d-%H%M%S)"
created_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
run_dir="${BACKUP_DIR}/${timestamp}"
staging_dir="${run_dir}/archive"
archive_name="nikki-operational-backup-${timestamp}.tar.gz"
archive="${run_dir}/${archive_name}"
db_dump="${staging_dir}/db/postgres.dump"
uploads_archive="${staging_dir}/uploads/uploads.tar"
manifest="${staging_dir}/manifest.json"

compose() {
  docker compose --env-file "${ENV_FILE}" -f docker-compose.yml -f docker-compose.prod.yml "$@"
}

detect_uploads_volume() {
  backend_container="$(compose ps -q backend 2>/dev/null || true)"
  if [ -z "${backend_container}" ]; then
    printf '%s\n' "ERROR: could not find the backend container for this Compose project." >&2
    printf '%s\n' "Start the production stack or set UPLOADS_VOLUME explicitly." >&2
    exit 1
  fi

  detected_volume="$(
    docker inspect \
      --format '{{range .Mounts}}{{if and (eq .Destination "/uploads") (eq .Type "volume")}}{{.Name}}{{end}}{{end}}' \
      "${backend_container}"
  )"
  if [ -z "${detected_volume}" ]; then
    printf '%s\n' "ERROR: could not detect a Docker volume mounted at /uploads on backend container: ${backend_container}" >&2
    printf '%s\n' "Set UPLOADS_VOLUME explicitly if uploads are mounted differently." >&2
    exit 1
  fi

  printf '%s\n' "${detected_volume}"
}

mkdir -p "${staging_dir}/db" "${staging_dir}/uploads"
run_dir_abs="$(cd "${run_dir}" && pwd -P)"

printf '%s\n' "Creating Nikki operational backup ${timestamp}"
printf '%s\n' "WARNING: operational backups contain private diary data and password hashes."

if [ -z "${UPLOADS_VOLUME}" ]; then
  UPLOADS_VOLUME="$(detect_uploads_volume)"
fi
printf '%s\n' "Using uploads volume: ${UPLOADS_VOLUME}"

if ! docker volume inspect "${UPLOADS_VOLUME}" >/dev/null 2>&1; then
  printf '%s\n' "ERROR: uploads volume does not exist: ${UPLOADS_VOLUME}" >&2
  printf '%s\n' "Refusing to continue because Docker would otherwise create an empty volume." >&2
  exit 1
fi

compose exec -T postgres pg_dump -Fc -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" > "${db_dump}"

if [ ! -s "${db_dump}" ]; then
  printf '%s\n' "ERROR: PostgreSQL custom-format dump was not created or is empty." >&2
  exit 1
fi

MSYS_NO_PATHCONV=1 docker run --rm \
  -v "${UPLOADS_VOLUME}:/uploads:ro" \
  -v "${run_dir_abs}:/backup" \
  alpine tar -cf "/backup/archive/uploads/uploads.tar" -C /uploads .

if [ ! -s "${uploads_archive}" ]; then
  printf '%s\n' "ERROR: uploads archive was not created or is empty." >&2
  exit 1
fi

upload_item_count="$(tar -tf "${uploads_archive}" | wc -l | tr -d ' ')"
if [ "${upload_item_count}" -le 1 ]; then
  printf '%s\n' "WARNING: uploads archive appears empty. This is OK only if Nikki has no uploaded images yet." >&2
fi

entry_count="$(compose exec -T postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -Atc 'SELECT COUNT(*) FROM entries;' | tr -d '\r\n ')"
image_count="$(compose exec -T postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -Atc 'SELECT COUNT(*) FROM images;' | tr -d '\r\n ')"
repo_version="$(git rev-parse --short HEAD 2>/dev/null || printf '%s' unknown)"

cat > "${manifest}" <<EOF
{
  "format": "nikki-operational-backup-v1",
  "backupCreatedAt": "${created_at}",
  "nikkiVersion": "${repo_version}",
  "schemaVersion": "1",
  "entryCount": ${entry_count:-0},
  "imageCount": ${image_count:-0}
}
EOF

if command -v sha256sum >/dev/null 2>&1; then
  (
    cd "${staging_dir}"
    sha256sum manifest.json db/postgres.dump uploads/uploads.tar > SHA256SUMS
  )
else
  printf '%s\n' "WARNING: sha256sum is unavailable; archive will not include SHA256SUMS." >&2
fi

(
  cd "${staging_dir}"
  if [ -f SHA256SUMS ]; then
    tar -czf "../${archive_name}" manifest.json db/postgres.dump uploads/uploads.tar SHA256SUMS
  else
    tar -czf "../${archive_name}" manifest.json db/postgres.dump uploads/uploads.tar
  fi
)

if [ ! -s "${archive}" ]; then
  printf '%s\n' "ERROR: operational backup archive was not created or is empty." >&2
  exit 1
fi

rm -rf "${staging_dir}"

if [ -n "${AGE_RECIPIENT}" ]; then
  if ! command -v age >/dev/null 2>&1; then
    printf '%s\n' "ERROR: AGE_RECIPIENT is set but age is not installed." >&2
    exit 1
  fi

  age -r "${AGE_RECIPIENT}" -o "${archive}.age" "${archive}"
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "${run_dir}" && sha256sum "${archive_name}.age" > SHA256SUMS.age)
  fi

  if [ "${DELETE_PLAINTEXT_AFTER_ENCRYPT}" = "true" ]; then
    rm -f "${archive}"
    printf '%s\n' "Plaintext operational backup archive deleted after encryption by explicit configuration."
  fi
fi

printf '%s\n' "Backup complete:"
if [ -f "${archive}" ]; then
  printf '  %s\n' "${archive}"
fi
if [ -f "${archive}.age" ]; then
  printf '  %s\n' "${archive}.age"
fi
printf '%s\n' "Use the plaintext .tar.gz archive for first setup restore, or decrypt the .age copy first."
